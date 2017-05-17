package queue

import (
	"context"
	"encoding/base64"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"

	"cred-alert/crypto"
	"cred-alert/metrics"
)

type sigCheck struct {
	child    PubSubProcessor
	verifier crypto.Verifier

	failures metrics.Counter
}

func NewSignatureCheck(verify crypto.Verifier, emitter metrics.Emitter, processor PubSubProcessor) PubSubProcessor {
	return &sigCheck{
		child:    processor,
		verifier: verify,

		failures: emitter.Counter("queue.verification_failures"),
	}
}

func (s *sigCheck) Process(ctx context.Context, logger lager.Logger, message *pubsub.Message) (bool, error) {
	signature := message.Attributes["signature"]
	sigLog := logger.WithData(lager.Data{
		"signature": signature,
	})

	decodedSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		sigLog.Error("signature-malformed", err)
		s.failures.Inc(sigLog)

		return false, err
	}

	err = s.verifier.Verify(message.Data, decodedSignature)
	if err != nil {
		sigLog.Error("signature-invalid", err)
		s.failures.Inc(sigLog)

		return false, err
	}

	return s.child.Process(ctx, logger, message)
}
