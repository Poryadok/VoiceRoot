package apns

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

// Sender delivers push notifications via Apple Push Notification service.
type Sender interface {
	Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload push.Payload) error
}

// ErrInvalidToken indicates the token should be removed from notification_db.
var ErrInvalidToken = errInvalidToken{}

type errInvalidToken struct{}

func (errInvalidToken) Error() string { return "apns: invalid token" }
