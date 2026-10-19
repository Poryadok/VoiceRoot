package main

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"voice/backend/notification/internal/chatquota"
)

// Configuration, not Redis runtime health, controls activation. Non-chat paths
// gain no runtime dependency on ordinary admission.
func newChatPushAdmission(lookup func(string) (string, bool), reg prometheus.Registerer) (*chatquota.Redis, error) {
	cfg, err := chatquota.ConfigFromEnv(lookup)
	if err != nil {
		return nil, err
	}
	nats, _ := lookup("NATS_URL")
	if strings.TrimSpace(nats) == "" {
		return nil, nil
	}
	addr, _ := lookup("NOTIFICATION_REDIS_ADDR")
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("NOTIFICATION_REDIS_ADDR required for message consumption")
	}
	client := redis.NewClient(&redis.Options{Addr: strings.TrimSpace(addr)})
	q, err := chatquota.New(client, cfg)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	if err = q.RegisterMetrics(reg); err != nil {
		_ = client.Close()
		return nil, err
	}
	return q, nil
}
