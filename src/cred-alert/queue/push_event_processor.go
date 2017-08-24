package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/trace"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"cred-alert/crypto"
	"cred-alert/lgctx"
	"cred-alert/metrics"
)

//go:generate counterfeiter . ChangeFetcher

type ChangeFetcher interface {
	Fetch(ctx context.Context, logger lager.Logger, owner, name string, reenable bool) error
}

type pushEventProcessor struct {
	changeFetcher ChangeFetcher
	verifier      crypto.Verifier
	clock         clock.Clock
	traceClient   *trace.Client

	endToEndGauge metrics.Gauge
}

func NewPushEventProcessor(
	changeFetcher ChangeFetcher,
	emitter metrics.Emitter,
	clock clock.Clock,
	traceClient *trace.Client,
) *pushEventProcessor {
	return &pushEventProcessor{
		changeFetcher: changeFetcher,
		clock:         clock,
		traceClient:   traceClient,

		endToEndGauge: emitter.Gauge("queue.end-to-end.duration"),
	}
}

func (proc *pushEventProcessor) Process(ctx context.Context, message *pubsub.Message) (bool, error) {
	logger := lgctx.WithSession(ctx, "processing-push-event")

	span := proc.traceClient.NewSpan("io.pivotal.red.revok/CodePush")
	defer span.Finish()
	ctx = trace.NewContext(ctx, span)

	p, err := proc.decodeMessage(logger, message)
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

func (proc *pushEventProcessor) decodeMessage(logger lager.Logger, message *pubsub.Message) (PushEventPlan, error) {
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
