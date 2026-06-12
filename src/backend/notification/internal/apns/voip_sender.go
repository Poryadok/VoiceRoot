package apns

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"

	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

// VoIPSender delivers VoIP push notifications via Apple Push Notification service.
type VoIPSender interface {
	Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload push.Payload) error
}

// HTTPVoIPSender delivers VoIP pushes via Apple's HTTP/2 API.
type HTTPVoIPSender struct {
	client   pushClient
	bundleID string
}

// NewHTTPVoIPSenderForTest constructs an HTTPVoIPSender with a custom client (tests only).
func NewHTTPVoIPSenderForTest(client pushClient, bundleID string) *HTTPVoIPSender {
	return &HTTPVoIPSender{client: client, bundleID: bundleID}
}

// NewHTTPVoIPSender builds a VoIP APNs HTTP/2 sender from config.
func NewHTTPVoIPSender(cfg Config) (*HTTPVoIPSender, error) {
	authKey, err := token.AuthKeyFromBytes([]byte(cfg.AuthKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("apns voip auth key: %w", err)
	}
	tok := &token.Token{
		AuthKey: authKey,
		KeyID:   cfg.KeyID,
		TeamID:  cfg.TeamID,
	}
	client := apns2.NewTokenClient(tok)
	if !cfg.Production {
		client = client.Development()
	}
	return &HTTPVoIPSender{client: client, bundleID: cfg.BundleID}, nil
}

func (s *HTTPVoIPSender) Send(ctx context.Context, profileID uuid.UUID, device store.DeviceToken, p push.Payload) error {
	_ = profileID
	if s == nil || s.client == nil {
		return fmt.Errorf("apns voip: sender unavailable")
	}
	n, err := BuildVoIPNotification(s.bundleID, device.Token, p)
	if err != nil {
		return err
	}
	resp, err := s.client.PushWithContext(ctx, n)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}
	if !resp.Sent() {
		if isInvalidTokenReason(resp.Reason) {
			return ErrInvalidToken
		}
		return fmt.Errorf("apns voip: push rejected: %s", resp.Reason)
	}
	return nil
}

// VoIPNoopSender logs VoIP push attempts when credentials are not configured.
type VoIPNoopSender struct {
	Logger interface {
		Debug(msg string, args ...any)
	}
}

func (n *VoIPNoopSender) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, p push.Payload) error {
	_ = ctx
	_ = profileID
	_ = token
	_ = p
	return nil
}
