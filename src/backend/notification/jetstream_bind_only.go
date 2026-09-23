package main

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

type jetStreamSubscribeFunc func(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error)

// bindPreprovisionedConsumer connects a handler only to a durable created by
// the central NATS bootstrap. Notification must not create or alter JetStream
// consumers while serving traffic.
func bindPreprovisionedConsumer(js nats.JetStreamContext, stream, durable, subject, target string, handler nats.MsgHandler, options ...nats.SubOpt) (*nats.Subscription, error) {
	info, err := js.ConsumerInfo(stream, durable)
	if err != nil {
		return nil, fmt.Errorf("inspect pre-provisioned consumer %s/%s: %w", stream, durable, err)
	}
	if err := validateNotificationConsumerFilter(info, stream, durable, subject, target); err != nil {
		return nil, err
	}
	return bindPreprovisionedSubscribe(js.Subscribe, stream, durable, subject, handler, options...)
}

func validateNotificationConsumerFilter(info *nats.ConsumerInfo, stream, durable, subject, target string) error {
	if info == nil || info.Stream != stream || info.Name != durable ||
		info.Config.FilterSubject != subject || len(info.Config.FilterSubjects) != 0 ||
		info.Config.DeliverSubject != target || info.Config.AckPolicy != nats.AckExplicitPolicy ||
		info.Config.DeliverPolicy != nats.DeliverNewPolicy {
		return fmt.Errorf("pre-provisioned consumer %s/%s must have exact filter %q, target %q, explicit ack, and deliver-new policy", stream, durable, subject, target)
	}
	return nil
}

func bindPreprovisionedSubscribe(subscribe jetStreamSubscribeFunc, stream, durable, subject string, handler nats.MsgHandler, options ...nats.SubOpt) (*nats.Subscription, error) {
	options = append([]nats.SubOpt{nats.Bind(stream, durable)}, options...)
	return subscribe(subject, handler, options...)
}
