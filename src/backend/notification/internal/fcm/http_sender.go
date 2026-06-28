package fcm

import (
	"context"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"google.golang.org/api/option"

	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type messagingClient interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
}

// HTTPSender delivers notifications via Firebase Cloud Messaging HTTP v1.
type HTTPSender struct {
	client messagingClient
}

// NewHTTPSenderForTest constructs an HTTPSender with a custom client (tests only).
func NewHTTPSenderForTest(client messagingClient) *HTTPSender {
	return &HTTPSender{client: client}
}

// NewHTTPSender builds an FCM sender from service account credentials.
func NewHTTPSender(cfg Config) (*HTTPSender, error) {
	app, err := firebase.NewApp(context.Background(), nil, option.WithAuthCredentialsJSON(option.ServiceAccount, cfg.CredentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("fcm firebase app: %w", err)
	}
	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("fcm messaging client: %w", err)
	}
	return &HTTPSender{client: client}, nil
}

func (s *HTTPSender) Send(ctx context.Context, profileID uuid.UUID, device store.DeviceToken, p push.Payload) error {
	_ = profileID
	if s == nil || s.client == nil {
		return fmt.Errorf("fcm: sender unavailable")
	}
	msg, err := BuildFCMMessage(device.Token, p)
	if err != nil {
		return err
	}
	_, err = s.client.Send(ctx, msg)
	if err != nil {
		if isInvalidFCMToken(err) {
			return ErrInvalidToken
		}
		return err
	}
	return nil
}

func isInvalidFCMToken(err error) bool {
	if err == nil {
		return false
	}
	if messaging.IsUnregistered(err) {
		return true
	}
	if messaging.IsInvalidArgument(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "requested entity was not found") ||
		strings.Contains(msg, "registration-token-not-registered")
}
