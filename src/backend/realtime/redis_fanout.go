package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisProfilePrefix = "voice:rt:prof:"
	defaultRedisFanoutChannel = "voice:rt:fanout"
)

type redisFanoutPayload struct {
	ChatID      string `json:"chat_id"`
	ProfileID   string `json:"profile_id"`
	Kind        string `json:"kind"`
	SrcInstance string `json:"src_instance"`
	SrcConn     string `json:"src_conn"`
}

type redisFanoutConfig struct {
	Client          *redis.Client
	Hub             *wsHub
	InstanceID      string
	KeyPrefix       string
	FanoutChannel   string
	ProfileRegistry *redisProfileRegistry // optional override; nil uses Client + KeyPrefix
}

type redisFanout struct {
	rdb             *redis.Client
	hub             *wsHub
	instanceID      string
	fanoutChannel   string
	profileRegistry *redisProfileRegistry
}

func newRedisFanout(cfg redisFanoutConfig) *redisFanout {
	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = defaultRedisProfilePrefix
	}
	ch := strings.TrimSpace(cfg.FanoutChannel)
	if ch == "" {
		ch = defaultRedisFanoutChannel
	}
	reg := cfg.ProfileRegistry
	if reg == nil && cfg.Client != nil {
		reg = newRedisProfileRegistry(cfg.Client, prefix)
	}
	return &redisFanout{
		rdb:             cfg.Client,
		hub:             cfg.Hub,
		instanceID:      cfg.InstanceID,
		fanoutChannel:   ch,
		profileRegistry: reg,
	}
}

func (f *redisFanout) Register(ctx context.Context, profileID, connID string) error {
	if f == nil || f.profileRegistry == nil {
		return nil
	}
	return f.profileRegistry.Register(ctx, profileID, f.instanceID, connID)
}

func (f *redisFanout) Unregister(ctx context.Context, profileID, connID string) error {
	if f == nil || f.profileRegistry == nil {
		return nil
	}
	return f.profileRegistry.Unregister(ctx, profileID, f.instanceID, connID)
}

func (f *redisFanout) PublishTyping(ctx context.Context, chatID, profileID, kind, srcConn string) error {
	if f == nil || f.rdb == nil {
		return nil
	}
	msg := redisFanoutPayload{
		ChatID:      chatID,
		ProfileID:   profileID,
		Kind:        kind,
		SrcInstance: f.instanceID,
		SrcConn:     srcConn,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return f.rdb.Publish(ctx, f.fanoutChannel, string(b)).Err()
}

func (f *redisFanout) runSubscriber(ctx context.Context) error {
	if f == nil || f.rdb == nil || f.hub == nil {
		return nil
	}
	sub := f.rdb.Subscribe(ctx, f.fanoutChannel)
	defer func() { _ = sub.Close() }()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if msg == nil {
				continue
			}
			var p redisFanoutPayload
			if err := json.Unmarshal([]byte(msg.Payload), &p); err != nil {
				log.Printf("realtime redis fanout: bad payload: %v", err)
				continue
			}
			d, err := json.Marshal(map[string]any{
				"chat_id":    p.ChatID,
				"profile_id": p.ProfileID,
				"kind":       p.Kind,
			})
			if err != nil {
				continue
			}
			f.hub.broadcastTypingExcept(p.ChatID, p.SrcInstance, p.SrcConn, d)
		}
	}
}
