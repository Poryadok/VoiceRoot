package grpcsvc

import "context"

// phase13EventsRecorder captures user.events payloads for Phase 13 profile lifecycle tests.
type phase13EventsRecorder struct {
	profileCreated  int
	profileSwitched []profileSwitchedEvent
}

type profileSwitchedEvent struct {
	accountID     string
	oldProfileID  string
	newProfileID  string
}

func (r *phase13EventsRecorder) PublishProfileCreated(_ context.Context, _, _ string) error {
	r.profileCreated++
	return nil
}

func (r *phase13EventsRecorder) PublishProfileUpdated(_ context.Context, _, _, _ string) error {
	return nil
}

func (r *phase13EventsRecorder) PublishProfileSwitched(_ context.Context, accountID, oldProfileID, newProfileID string) error {
	r.profileSwitched = append(r.profileSwitched, profileSwitchedEvent{
		accountID:    accountID,
		oldProfileID: oldProfileID,
		newProfileID: newProfileID,
	})
	return nil
}

func (r *phase13EventsRecorder) PublishVerified(_ context.Context, _, _, _ string) error {
	return nil
}
