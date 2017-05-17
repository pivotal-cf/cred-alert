package queue

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/trace"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"cred-alert/crypto"
	"cred-alert/metrics"
	"cred-alert/revok"
)

type pushEventProcessor struct {
	changeFetcher revok.ChangeFetcher
	verifier      crypto.Verifier
	clock         clock.Clock
	traceClient   *trace.Client

	verifyFailedCounter metrics.Counter
	endToEndGauge       metrics.Gauge
}

func NewPushEventProcessor(
	changeFetcher revok.ChangeFetcher,
	verifier crypto.Verifier,
	emitter metrics.Emitter,
	clock clock.Clock,
	traceClient *trace.Client,
) *pushEventProcessor {
	return &pushEventProcessor{
		changeFetcher: changeFetcher,
		verifier:      verifier,
		clock:         clock,
		traceClient:   traceClient,

		verifyFailedCounter: emitter.Counter("queue.push_event_processor.verify.failed"),
		endToEndGauge:       emitter.Gauge("queue.end-to-end.duration"),
	}
}

func (proc *pushEventProcessor) Process(ctx context.Context, logger lager.Logger, message *pubsub.Message) (bool, error) {
	logger = logger.Session("processing-push-event")

	span := proc.traceClient.NewSpan("io.pivotal.red.revok/CodePush")
	defer span.Finish()
	ctx = trace.NewContext(ctx, span)

	if err := proc.checkSignature(ctx, logger, message); err != nil {
		return false, err
	}

	p, err := proc.decodeMessage(ctx, logger, message)
	if err != nil {
		return false, err
	}

	logger = logger.WithData(lager.Data{
		"owner":      p.Owner,
		"repository": p.Repository,
	})
	span.SetLabel("owner", p.Owner)
	span.SetLabel("repository", p.Repository)

	err = proc.changeFetcher.Fetch(ctx, logger, p.Owner, p.Repository, true)
	if err != nil {
		return true, err
	}

	duration := proc.clock.Since(p.PushTime).Seconds()
	proc.endToEndGauge.Update(logger, float32(duration))

	return false, nil
}

func (proc *pushEventProcessor) checkSignature(ctx context.Context, logger lager.Logger, message *pubsub.Message) error {
	decodedSignature, err := base64.StdEncoding.DecodeString(message.Attributes["signature"])
	if err != nil {
		logger.Error("signature-malformed", err, lager.Data{
			"signature": message.Attributes["signature"],
		})
		return err
	}

	err = proc.verifier.Verify(message.Data, decodedSignature)
	if err != nil {
		logger.Error("signature-invalid", err, lager.Data{
			"signature": message.Attributes["signature"],
		})
		proc.verifyFailedCounter.Inc(logger)
		return err
	}

	return nil
}

func (proc *pushEventProcessor) decodeMessage(ctx context.Context, logger lager.Logger, message *pubsub.Message) (PushEventPlan, error) {
	buffer := bytes.NewBuffer(message.Data)

	var p PushEventPlan
	if err := json.NewDecoder(buffer).Decode(&p); err != nil {
		logger.Error("payload-malformed", err)
		return p, err
	}

	if p.Owner == "" || p.Repository == "" {
		err := errors.New("invalid payload: missing owner or repository")
		logger.Error("payload-incomplete", err)
		return p, err
	}

	return p, nil
}
