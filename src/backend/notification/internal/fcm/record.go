package fcm

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"voice/backend/notification/internal/store"
)

// RecordedPush is a captured FCM delivery attempt for opt-in E2E verification.
type RecordedPush struct {
	ProfileID uuid.UUID
	Token     string
	Type      string
	Title     string
	Body      string
}

// PushRecorder stores recent push attempts (dev/staging only).
type PushRecorder struct {
	mu      sync.Mutex
	records []RecordedPush
}

// GlobalPushRecorder is set when NOTIFICATION_RECORD_PUSHES=true.
var GlobalPushRecorder = &PushRecorder{}

func (r *PushRecorder) Record(profileID uuid.UUID, token store.DeviceToken, payload PushPayload) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, RecordedPush{
		ProfileID: profileID,
		Token:     token.Token,
		Type:      payload.Data["type"],
		Title:     payload.Title,
		Body:      payload.Body,
	})
	if len(r.records) > 200 {
		r.records = r.records[len(r.records)-200:]
	}
}

// LastForProfile returns the most recent push for a profile (if any).
func (r *PushRecorder) LastForProfile(profileID uuid.UUID) (RecordedPush, bool) {
	if r == nil {
		return RecordedPush{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.records) - 1; i >= 0; i-- {
		if r.records[i].ProfileID == profileID {
			return r.records[i], true
		}
	}
	return RecordedPush{}, false
}

// RecordSender wraps a sender and records deliveries.
type RecordSender struct {
	Inner Sender
}

func (s *RecordSender) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload PushPayload) error {
	GlobalPushRecorder.Record(profileID, token, payload)
	if s == nil || s.Inner == nil {
		return nil
	}
	return s.Inner.Send(ctx, profileID, token, payload)
}
