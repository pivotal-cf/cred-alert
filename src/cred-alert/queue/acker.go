package queue

import "cloud.google.com/go/pubsub"

//go:generate counterfeiter . Acker

type Acker interface {
	Ack(*pubsub.Message, bool)
}

type acker struct {
}

func NewAcker() Acker {
	return &acker{}
}

func (a *acker) Ack(msg *pubsub.Message, ack bool) {
	msg.Done(ack)
}
