package fcm

import (
	"context"

	"github.com/google/uuid"
	"voice/backend/notification/internal/store"
)

// PushPayload is the FCM data message envelope.
type PushPayload struct {
	Title       string
	Body        string
	CollapseTag string
	Counter     int
	Data        map[string]string
}

// Sender delivers push notifications via FCM.
type Sender interface {
	Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload PushPayload) error
}

// StubSender is a Phase-6 placeholder that always reports not implemented.
type StubSender struct{}

func (StubSender) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload PushPayload) error {
	_ = ctx
	_ = profileID
	_ = token
	_ = payload
	return ErrNotImplemented
}

// ErrNotImplemented marks unimplemented FCM delivery.
var ErrNotImplemented = errNotImplemented{}

type errNotImplemented struct{}

func (errNotImplemented) Error() string { return "fcm sender: not implemented" }

// ErrInvalidToken indicates the token should be removed from notification_db.
var ErrInvalidToken = errInvalidToken{}

type errInvalidToken struct{}

func (errInvalidToken) Error() string { return "fcm: invalid token" }
