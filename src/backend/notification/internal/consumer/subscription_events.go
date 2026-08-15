package consumer

import (
	"context"

	eventsv1 "voice.app/voice/events/v1"
)

// GraceReminderResult is a stub record for subscription.grace_reminder handling (SUB-04).
type GraceReminderResult struct {
	Handled   bool
	Kind      string
	AccountID string
	Plan      string
	Day       int32
}

// SubscriptionEventHandler maps subscription.events to notification stubs.
type SubscriptionEventHandler struct{}

// HandleGraceReminder records a grace reminder for push/email delivery stubs.
func (h *SubscriptionEventHandler) HandleGraceReminder(ctx context.Context, ev *eventsv1.GraceReminder) GraceReminderResult {
	_ = ctx
	_ = h
	if ev == nil || ev.GetAccountId() == "" {
		return GraceReminderResult{}
	}
	day := ev.GetDay()
	if day != 1 && day != 3 && day != 7 {
		return GraceReminderResult{}
	}
	return GraceReminderResult{
		Handled:   true,
		Kind:      "grace_reminder",
		AccountID: ev.GetAccountId(),
		Plan:      ev.GetPlan(),
		Day:       day,
	}
}
