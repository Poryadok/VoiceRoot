package spacemedia

import (
	"context"
	"encoding/json"

	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/spacelifecycle"
)

type nativeRetirementAuthorization struct {
	proof    retirementproof.ProofRef
	work     retirementproof.OwnerWork
	workHash retirementproof.Digest
}

func authorizeNativeRetirement(t spacelifecycle.CapturedTarget, verified retirementproof.VerifiedRetirement, work retirementproof.OwnerWork) (*nativeRetirementAuthorization, error) {
	if verified.ValidateFor(work) != nil || work.SourceKind != "NATIVE_SPACE" || work.Ref.OwnerProtocol != spacelifecycle.NativeOwnerProtocol || work.SpaceID.String() != t.SpaceID {
		return nil, ErrAdmissionConflict
	}
	var targets []struct {
		ID, Kind string
		Data     json.RawMessage
	}
	if json.Unmarshal(work.SealedTargetManifestBytes, &targets) != nil {
		return nil, ErrAdmissionConflict
	}
	for _, target := range targets {
		if target.ID != "native:"+t.OperationID || target.Kind != "native" {
			continue
		}
		var saved spacelifecycle.CapturedTarget
		if json.Unmarshal(target.Data, &saved) != nil {
			return nil, ErrAdmissionConflict
		}
		if saved.SpaceID == "" {
			saved.SpaceID = work.SpaceID.String()
		}
		if saved.OperationID == t.OperationID && saved.Generation == t.Generation && saved.AccountID == t.AccountID && saved.ProfileID == t.ProfileID && saved.SpaceID == t.SpaceID && saved.RoomID == t.RoomID && saved.VoiceRoomID == t.VoiceRoomID && saved.RoomGeneration == t.RoomGeneration && saved.Identity == t.Identity {
			return &nativeRetirementAuthorization{verified.ProofRef(), work, verified.WorkHash()}, nil
		}
		return nil, ErrAdmissionConflict
	}
	return nil, ErrAdmissionConflict
}

// CompleteRetiredCapturedDrain derives authority from a trusted, accepted proof
// for this exact sealed target. Missing/another fence may already release the old
// owner; it never authorizes deletion of that newer owner or grant history.
func (s *PostgresAdmissionStore) CompleteRetiredCapturedDrain(ctx context.Context, t spacelifecycle.CapturedTarget, verified retirementproof.VerifiedRetirement, work retirementproof.OwnerWork) error {
	if !s.usable() {
		return ErrAdmissionUnavailable
	}
	authorization, err := authorizeNativeRetirement(t, verified, work)
	if err != nil {
		return err
	}
	return s.completeCapturedDrain(ctx, t, authorization)
}

type retiredAdmissionCompletion struct {
	AdmissionRecoveryStore
	verified retirementproof.VerifiedRetirement
	work     retirementproof.OwnerWork
}

func (a *retiredAdmissionCompletion) CompleteCapturedDrain(ctx context.Context, t spacelifecycle.CapturedTarget) error {
	store, ok := a.AdmissionRecoveryStore.(interface {
		CompleteRetiredCapturedDrain(context.Context, spacelifecycle.CapturedTarget, retirementproof.VerifiedRetirement, retirementproof.OwnerWork) error
	})
	if !ok {
		return ErrAdmissionUnavailable
	}
	return store.CompleteRetiredCapturedDrain(ctx, t, a.verified, a.work)
}

func (c *Coordinator) ResumeRetiredCapturedDrain(ctx context.Context, t spacelifecycle.CapturedTarget, verified retirementproof.VerifiedRetirement, work retirementproof.OwnerWork) error {
	if c == nil || c.Calls == nil || c.Store == nil || c.Admissions == nil {
		return ErrAdmissionUnavailable
	}
	if _, err := authorizeNativeRetirement(t, verified, work); err != nil {
		return err
	}
	// Construct only the exact cleanup view; never copy reconcileMu/readiness
	// from the serving coordinator or publish any ordinary forward capability.
	cleanup := &Coordinator{Calls: c.Calls, Store: c.Store, Admissions: &retiredAdmissionCompletion{c.Admissions, verified, work}}
	return cleanup.resumeCapturedDrain(ctx, t, true)
}
