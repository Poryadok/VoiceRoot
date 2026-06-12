package apns

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"

	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

// Config holds APNs HTTP/2 sender credentials.
type Config struct {
	KeyID      string
	TeamID     string
	BundleID   string
	AuthKeyPEM string
	Production bool
}

// ConfigFromEnv reads APNs credentials from environment variables.
func ConfigFromEnv() (Config, bool) {
	keyID := strings.TrimSpace(os.Getenv("APNS_KEY_ID"))
	teamID := strings.TrimSpace(os.Getenv("APNS_TEAM_ID"))
	bundleID := strings.TrimSpace(os.Getenv("APNS_BUNDLE_ID"))
	authKey := strings.TrimSpace(os.Getenv("APNS_AUTH_KEY"))
	if authKey == "" {
		if path := strings.TrimSpace(os.Getenv("APNS_AUTH_KEY_PATH")); path != "" {
			b, err := os.ReadFile(path)
			if err == nil {
				authKey = string(b)
			}
		}
	}
	if keyID == "" || teamID == "" || bundleID == "" || authKey == "" {
		return Config{}, false
	}
	production := strings.TrimSpace(os.Getenv("APNS_PRODUCTION")) != "false"
	return Config{
		KeyID:      keyID,
		TeamID:     teamID,
		BundleID:   bundleID,
		AuthKeyPEM: authKey,
		Production: production,
	}, true
}

// HTTPSender delivers notifications via Apple's HTTP/2 API.
type HTTPSender struct {
	client   pushClient
	bundleID string
}

type pushClient interface {
	PushWithContext(ctx apns2.Context, n *apns2.Notification) (*apns2.Response, error)
}

// NewHTTPSenderForTest constructs an HTTPSender with a custom client (tests only).
func NewHTTPSenderForTest(client pushClient, bundleID string) *HTTPSender {
	return &HTTPSender{client: client, bundleID: bundleID}
}

// NewHTTPSender builds an APNs HTTP/2 sender from config.
func NewHTTPSender(cfg Config) (*HTTPSender, error) {
	authKey, err := token.AuthKeyFromBytes([]byte(cfg.AuthKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("apns auth key: %w", err)
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
	return &HTTPSender{client: client, bundleID: cfg.BundleID}, nil
}

func (s *HTTPSender) Send(ctx context.Context, profileID uuid.UUID, device store.DeviceToken, p push.Payload) error {
	_ = profileID
	if s == nil || s.client == nil {
		return fmt.Errorf("apns: sender unavailable")
	}
	n, err := BuildNotification(s.bundleID, device.Token, p)
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
		return fmt.Errorf("apns: push rejected: %s", resp.Reason)
	}
	return nil
}

func isInvalidTokenReason(reason string) bool {
	switch reason {
	case apns2.ReasonBadDeviceToken,
		apns2.ReasonDeviceTokenNotForTopic,
		apns2.ReasonUnregistered,
		apns2.ReasonExpiredToken:
		return true
	default:
		return false
	}
}
