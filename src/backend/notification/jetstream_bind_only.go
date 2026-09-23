package main

import "github.com/nats-io/nats.go"

type jetStreamSubscribeFunc func(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error)

// bindPreprovisionedConsumer connects a handler only to a durable created by
// the central NATS bootstrap. Notification must not create or alter JetStream
// consumers while serving traffic.
func bindPreprovisionedConsumer(js nats.JetStreamContext, stream, durable string, handler nats.MsgHandler, options ...nats.SubOpt) (*nats.Subscription, error) {
	return bindPreprovisionedSubscribe(js.Subscribe, stream, durable, handler, options...)
}

func bindPreprovisionedSubscribe(subscribe jetStreamSubscribeFunc, stream, durable string, handler nats.MsgHandler, options ...nats.SubOpt) (*nats.Subscription, error) {
	options = append([]nats.SubOpt{nats.Bind(stream, durable)}, options...)
	return subscribe("", handler, options...)
}
