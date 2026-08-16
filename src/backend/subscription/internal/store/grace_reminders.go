package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// GraceReminderCandidate is a grace_period subscription that may need a D1/D3/D7 reminder.
type GraceReminderCandidate struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	Plan           string
	GracePeriodEnd time.Time
	RemindersSent  []int32
}

const gracePeriodDuration = 7 * 24 * time.Hour

var graceReminderDays = []int32{1, 3, 7}

// GraceDay returns 1..7 for the current day within a grace window ending at graceEnd.
// Returns 0 when now is outside the window.
func GraceDay(now, graceEnd time.Time) int32 {
	now = now.UTC()
	graceEnd = graceEnd.UTC()
	graceStart := graceEnd.Add(-gracePeriodDuration)
	if now.Before(graceStart) || now.After(graceEnd) {
		return 0
	}
	day := int32(now.Sub(graceStart)/(24*time.Hour)) + 1
	if day < 1 {
		return 0
	}
	if day > 7 {
		day = 7
	}
	return day
}

// ListGraceReminderCandidates returns active grace_period rows still inside the window.
func (s *SubscriptionStore) ListGraceReminderCandidates(ctx context.Context, now time.Time) ([]GraceReminderCandidate, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, account_id, plan, grace_period_end, COALESCE(grace_reminders_sent, '{}')
FROM subscriptions
WHERE status = 'grace_period'
  AND grace_period_end IS NOT NULL
  AND grace_period_end >= $1`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GraceReminderCandidate
	for rows.Next() {
		var item GraceReminderCandidate
		if err := rows.Scan(&item.ID, &item.AccountID, &item.Plan, &item.GracePeriodEnd, &item.RemindersSent); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// MarkGraceReminderSent appends day to grace_reminders_sent when not already present.
func (s *SubscriptionStore) MarkGraceReminderSent(ctx context.Context, subID uuid.UUID, day int32) error {
	_, err := s.Pool.Exec(ctx, `
UPDATE subscriptions
SET grace_reminders_sent = CASE
	WHEN $2 = ANY(grace_reminders_sent) THEN grace_reminders_sent
	ELSE array_append(grace_reminders_sent, $2::int)
END,
updated_at = now()
WHERE id = $1`, subID, day)
	return err
}

// ShouldEmitGraceReminder reports whether day is a reminder day and not yet sent.
func ShouldEmitGraceReminder(day int32, already []int32) bool {
	ok := false
	for _, d := range graceReminderDays {
		if d == day {
			ok = true
			break
		}
	}
	if !ok {
		return false
	}
	for _, d := range already {
		if d == day {
			return false
		}
	}
	return true
}
