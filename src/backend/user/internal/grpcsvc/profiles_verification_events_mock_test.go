package grpcsvc

import "context"

// profilesVerificationEventsRecorder captures user.events payloads for multi-profile/verification (docs/features/multi-profile.md) profile lifecycle tests.
type profilesVerificationEventsRecorder struct {
	profileCreated  int
	profileSwitched []profileSwitchedEvent
}

type profileSwitchedEvent struct {
	accountID     string
	oldProfileID  string
	newProfileID  string
}

func (r *profilesVerificationEventsRecorder) PublishProfileCreated(_ context.Context, _, _ string) error {
	r.profileCreated++
	return nil
}

func (r *profilesVerificationEventsRecorder) PublishProfileUpdated(_ context.Context, _, _, _ string) error {
	return nil
}

func (r *profilesVerificationEventsRecorder) PublishProfileSwitched(_ context.Context, accountID, oldProfileID, newProfileID string) error {
	r.profileSwitched = append(r.profileSwitched, profileSwitchedEvent{
		accountID:    accountID,
		oldProfileID: oldProfileID,
		newProfileID: newProfileID,
	})
	return nil
}

func (r *profilesVerificationEventsRecorder) PublishVerified(_ context.Context, _, _, _ string) error {
	return nil
}
