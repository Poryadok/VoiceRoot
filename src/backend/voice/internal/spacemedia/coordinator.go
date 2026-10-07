package spacemedia

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/store"
)

const (
	ReconcileInterval = 5 * time.Second
	reconcilePasses   = 3
)

var ErrReconciliationPending = errors.New("Space media authority changed during reconciliation")

type AccessResolver interface {
	ResolveVoiceRoomAccess(context.Context, string, string) (grpcsvc.CanonicalVoiceRoomAccess, error)
}

type GrantResolver interface {
	ResolveVoiceRoomGrants(context.Context, string, string, string) (grpcsvc.CanonicalVoiceRoomGrants, error)
}

type MediaLifecycle interface {
	RemoveParticipant(context.Context, string, string) error
}

type AdmissionRecoveryStore interface {
	RecoverOrphanRoomHeads(context.Context) error
	ConfirmProjection(context.Context, uuid.UUID, string) error
	ConfirmRoomHeadOpen(context.Context, store.SpaceMediaAdmission) error
	BeginRoomLeave(context.Context, uuid.UUID, string) error
	CompleteRoomLeave(context.Context, store.SpaceMediaAdmission) error
	RecoveryRows(context.Context, int) ([]store.SpaceMediaAdmission, error)
	ProjectionRows(context.Context, uuid.UUID, int, bool) ([]store.SpaceMediaAdmission, error)
	RoomCreator(context.Context, string, uint64) (store.SpaceMediaAdmission, error)
	MarkAborting(context.Context, uuid.UUID, string) error
	ReleaseFence(context.Context, store.SpaceMediaAdmission) error
	MarkCleanupCompleted(context.Context, uuid.UUID, string) error
	MarkProjectionApplied(context.Context, uuid.UUID, string) error
}

type SessionEpochChecker interface {
	RequireCurrent(context.Context, string, int64) error
}

type Store interface {
	ListActiveSpaceIDs(context.Context) ([]string, error)
	ListActiveSpaceVoiceCalls(context.Context, string) ([]store.Call, error)
	RaiseSpaceMediaEpochFloor(context.Context, string, store.SpaceMediaEpochKind, uint64) (store.SpaceMediaEpochFloors, error)
	GetSpaceMediaEpochProgress(context.Context, string) (store.SpaceMediaEpochProgress, error)
	MarkSpaceMediaEpochReconciled(context.Context, string, store.SpaceMediaEpochKind, uint64) (store.SpaceMediaEpochProgress, error)
	BeginSpaceMediaRevocation(context.Context, string, string, string, string) (store.Call, bool, error)
	CompleteSpaceMediaRevocation(context.Context, string, string, string, string) (store.Call, bool, error)
	ReconcileSpaceMediaParticipant(context.Context, string, string, string, string, store.SpaceMediaGrant) (store.Call, bool, error)
}

// AuthorityNotice contains only the durable authority snapshot. Its optional
// scope is intentionally ignored: a scoped event cannot prove that omitted
// participants were covered by a later event.
type AuthorityNotice struct {
	SpaceID     string
	Kind        store.SpaceMediaEpochKind
	Epoch       uint64
	VoiceRoomID string
	ProfileID   string
}

type Coordinator struct {
	reconcileMu         sync.Mutex
	ready               bool
	Store               Store
	Calls               store.CallStore
	Admissions          AdmissionRecoveryStore
	SessionEpochChecker SessionEpochChecker
	Access              AccessResolver
	Grants              GrantResolver
	Media               MediaLifecycle
	Every               time.Duration
	OnError             func(error)
	OnReadiness         func(bool)
}

func (c *Coordinator) setReady(ready bool) {
	c.ready = ready
	if c.OnReadiness != nil {
		c.OnReadiness(ready)
	}
}

func (c *Coordinator) Observe(ctx context.Context, notice AuthorityNotice) error {
	if c == nil || c.Store == nil || c.Access == nil || c.Grants == nil || c.Media == nil ||
		!canonicalID(notice.SpaceID) || notice.Epoch == 0 ||
		(notice.Kind != store.SpaceAccessEpoch && notice.Kind != store.RolePolicyEpoch) {
		return store.ErrInvalidState
	}
	c.reconcileMu.Lock()
	defer c.reconcileMu.Unlock()
	wasReady := c.ready
	c.setReady(false)
	if _, err := c.Store.RaiseSpaceMediaEpochFloor(ctx, notice.SpaceID, notice.Kind, notice.Epoch); err != nil {
		return err
	}
	if err := c.ReconcileSpace(ctx, notice.SpaceID); err != nil {
		return err
	}
	c.setReady(wasReady)
	return nil
}

// ReconcileSpace raises authority floors before selecting targets, then scans
// every indexed active room and every Space-media participant. A changed
// observed watermark restarts the scan; watermarks advance only after the
// complete stable pass succeeds.
func (c *Coordinator) ReconcileSpace(ctx context.Context, spaceID string) error {
	if c == nil || c.Store == nil || c.Access == nil || c.Grants == nil || c.Media == nil || !canonicalID(spaceID) {
		return store.ErrInvalidState
	}
	for pass := 0; pass < reconcilePasses; pass++ {
		before, err := c.Store.GetSpaceMediaEpochProgress(ctx, spaceID)
		if err != nil {
			return err
		}
		calls, err := c.Store.ListActiveSpaceVoiceCalls(ctx, spaceID)
		if err != nil {
			return err
		}
		sort.Slice(calls, func(i, j int) bool { return calls[i].RoomID < calls[j].RoomID })
		for _, call := range calls {
			if call.SpaceID != spaceID || !call.IsVoiceRoom() || call.LivekitRoomName == "" {
				return fmt.Errorf("invalid indexed Space voice room")
			}
			profiles := make([]string, 0, len(call.SpaceMedia))
			for profileID := range call.SpaceMedia {
				profiles = append(profiles, profileID)
			}
			sort.Strings(profiles)
			for _, profileID := range profiles {
				participant := call.SpaceMedia[profileID]
				if !canonicalID(profileID) || participant.ProfileID != profileID || participant.Identity == "" || participant.Generation == "" {
					return fmt.Errorf("invalid indexed Space media participant")
				}
				if err := c.reconcileParticipant(ctx, call, participant); err != nil {
					return err
				}
			}
		}
		after, err := c.Store.GetSpaceMediaEpochProgress(ctx, spaceID)
		if err != nil {
			return err
		}
		if after.Observed != before.Observed {
			continue
		}
		if before.Observed.AccessEpoch > 0 {
			if _, err := c.Store.MarkSpaceMediaEpochReconciled(ctx, spaceID, store.SpaceAccessEpoch, before.Observed.AccessEpoch); err != nil {
				return err
			}
		}
		if before.Observed.PolicyEpoch > 0 {
			if _, err := c.Store.MarkSpaceMediaEpochReconciled(ctx, spaceID, store.RolePolicyEpoch, before.Observed.PolicyEpoch); err != nil {
				return err
			}
		}
		final, err := c.Store.GetSpaceMediaEpochProgress(ctx, spaceID)
		if err != nil {
			return err
		}
		if final.Observed == final.Reconciled {
			return nil
		}
	}
	return ErrReconciliationPending
}

func (c *Coordinator) reconcileAdmissions(ctx context.Context) error {
	if c == nil || c.Admissions == nil {
		return nil
	}
	if c.Calls == nil || c.Store == nil || c.Access == nil || c.Grants == nil || c.SessionEpochChecker == nil {
		return store.ErrInvalidState
	}
	if err := c.Admissions.RecoverOrphanRoomHeads(ctx); err != nil {
		return err
	}
	for {
		rows, err := c.Admissions.RecoveryRows(ctx, 256)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, admission := range rows {
			if admission.State == AdmissionCommitted && admission.ProjectionApplied && admission.ParticipantState == "REVOKING" {
				if err := c.recoverRevokingAdmission(ctx, admission); err != nil {
					return err
				}
				continue
			}
			if admission.State == AdmissionAborting || admission.State == AdmissionPrepared || admission.State == AdmissionFenced {
				if err := c.abortAdmission(ctx, admission); err != nil {
					return err
				}
				continue
			}
			if admission.State != AdmissionCommitted || admission.ProjectionApplied {
				continue
			}
			if err := c.recoverCommittedAdmission(ctx, admission); err != nil {
				return err
			}
		}
	}
	// PostgreSQL is the lifecycle source of truth. Redis call/participant
	// projections may be lost independently, so rebuild every currently active
	// committed admission from its immutable journal row before opening serving.
	// Rebuild creator rows first so a lost call shell exists before joiners.
	for _, creators := range []bool{true, false} {
		after := uuid.Nil
		for {
			rows, err := c.Admissions.ProjectionRows(ctx, after, 256, creators)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				break
			}
			for _, admission := range rows {
				if admission.State != AdmissionCommitted || !admission.ProjectionApplied || admission.ParticipantState != "ACTIVE" || admission.CreatedRoom != creators {
					return ErrAdmissionConflict
				}
				if err := c.recoverCommittedAdmission(ctx, admission); err != nil {
					return err
				}
				after = admission.OperationID
			}
		}
	}
	return nil
}

func (c *Coordinator) recoverRevokingAdmission(ctx context.Context, admission store.SpaceMediaAdmission) error {
	call, err := c.Calls.GetCall(ctx, admission.RoomID)
	if err == nil {
		if participant, ok := call.SpaceMedia[admission.ProfileID.String()]; ok {
			if participant.AdmissionOperationID != admission.OperationID.String() || participant.Generation != admission.Generation || participant.Identity != admission.Identity {
				return ErrAdmissionConflict
			}
			_, completed, err := c.ejectParticipant(ctx, call, participant)
			if err != nil {
				return err
			}
			if !completed {
				return ErrAdmissionConflict
			}
			return nil
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	// The database may be ahead of a lost/removed Redis projection. Remove the
	// same immutable LiveKit identity before releasing its fence or ending the
	// room generation; a failed removal keeps readiness closed for retry.
	if err := c.Media.RemoveParticipant(ctx, "voice-room-"+admission.VoiceRoomID, admission.Identity); err != nil {
		return err
	}
	return c.Admissions.CompleteRoomLeave(ctx, admission)
}

func (c *Coordinator) abortAdmission(ctx context.Context, admission store.SpaceMediaAdmission) error {
	if admission.State != AdmissionAborting {
		if err := c.Admissions.MarkAborting(ctx, admission.OperationID, admission.Generation); err != nil {
			return err
		}
	}
	// An ABORTING row may be a retry after the process failed partway through
	// draining a committed projection. Re-inspect the exact operation on every
	// retry before releasing its fence; state alone does not prove cleanup ran.
	if admission.State == AdmissionCommitted || admission.State == AdmissionAborting {
		call, err := c.Calls.GetCall(ctx, admission.RoomID)
		if err == nil {
			if participant, ok := call.SpaceMedia[admission.ProfileID.String()]; ok {
				if participant.AdmissionOperationID != admission.OperationID.String() || participant.Generation != admission.Generation || participant.Identity != admission.Identity {
					return ErrAdmissionConflict
				}
				if _, completed, err := c.ejectParticipant(ctx, call, participant); err != nil || !completed {
					if err != nil {
						return err
					}
					return ErrAdmissionConflict
				}
			} else if admission.CreatedRoom && call.Status == callsv1.CallStatus_CALL_STATUS_UNSPECIFIED {
				if _, err := c.Calls.SetStatus(ctx, call.RoomID, callsv1.CallStatus_CALL_STATUS_ENDED, time.Now().UTC()); err != nil {
					return err
				}
			}
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	if admission.State != AdmissionPrepared {
		if err := c.Admissions.ReleaseFence(ctx, admission); err != nil && !errors.Is(err, ErrAdmissionConflict) {
			return err
		}
	}
	return c.Admissions.MarkCleanupCompleted(ctx, admission.OperationID, admission.Generation)
}

func (c *Coordinator) recoverCommittedAdmission(ctx context.Context, admission store.SpaceMediaAdmission) error {
	if err := c.Admissions.ConfirmRoomHeadOpen(ctx, admission); err != nil {
		return err
	}
	access, err := c.Access.ResolveVoiceRoomAccess(ctx, admission.VoiceRoomID, admission.ProfileID.String())
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if err != nil && status.Code(err) != codes.NotFound {
		return err
	}
	if err != nil || access.SpaceID != admission.SpaceID.String() || !access.Active || !access.Member || access.AccessEpoch != admission.AccessEpoch {
		return c.abortAdmission(ctx, admission)
	}
	grants, err := c.Grants.ResolveVoiceRoomGrants(ctx, admission.SpaceID.String(), admission.VoiceRoomID, admission.ProfileID.String())
	if err != nil && status.Code(err) != codes.NotFound && status.Code(err) != codes.PermissionDenied {
		return err
	}
	if err != nil || grants.PolicyEpoch != admission.PolicyEpoch || !grants.CanJoin || !grants.CanSubscribe || grants.CanPublishAudio != admission.CanPublishAudio {
		return c.abortAdmission(ctx, admission)
	}
	if err := c.SessionEpochChecker.RequireCurrent(ctx, admission.AccountID.String(), int64(admission.SessionEpoch)); err != nil {
		if status.Code(err) == codes.Unauthenticated || status.Code(err) == codes.NotFound {
			return c.abortAdmission(ctx, admission)
		}
		return err
	}
	accessFloors, err := c.Store.RaiseSpaceMediaEpochFloor(ctx, admission.SpaceID.String(), store.SpaceAccessEpoch, access.AccessEpoch)
	if err != nil {
		return err
	}
	policyFloors, err := c.Store.RaiseSpaceMediaEpochFloor(ctx, admission.SpaceID.String(), store.RolePolicyEpoch, grants.PolicyEpoch)
	if err != nil {
		return err
	}
	if accessFloors.AccessEpoch > admission.AccessEpoch || policyFloors.PolicyEpoch > admission.PolicyEpoch {
		return c.abortAdmission(ctx, admission)
	}
	call, err := c.Calls.GetCall(ctx, admission.RoomID)
	if errors.Is(err, store.ErrNotFound) {
		creator, creatorErr := c.Admissions.RoomCreator(ctx, admission.VoiceRoomID, admission.RoomGeneration)
		if creatorErr != nil {
			return creatorErr
		}
		call = store.Call{RoomID: admission.RoomID, LivekitRoomName: "voice-room-" + admission.VoiceRoomID,
			VoiceRoomID: admission.VoiceRoomID, SpaceID: admission.SpaceID.String(),
			SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
			InitiatorProfileID: creator.ProfileID.String(), MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
			Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED, StartedAt: creator.CallStartedAt,
			States: map[string]store.ParticipantState{}}
		call, err = c.Calls.CreateCall(ctx, call)
	}
	if err != nil {
		return err
	}
	if call.SpaceID != admission.SpaceID.String() || call.VoiceRoomID != admission.VoiceRoomID || !call.IsVoiceRoom() {
		return c.abortAdmission(ctx, admission)
	}
	if (admission.CreatedRoom && call.Status != callsv1.CallStatus_CALL_STATUS_UNSPECIFIED && call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE) ||
		(!admission.CreatedRoom && call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE) {
		return c.abortAdmission(ctx, admission)
	}
	if participant, ok := call.SpaceMedia[admission.ProfileID.String()]; ok {
		if participant.Generation != admission.Generation || participant.Identity != admission.Identity || participant.AdmissionOperationID != admission.OperationID.String() || participant.AccountID != admission.AccountID.String() {
			return ErrAdmissionConflict
		}
	} else {
		if _, err := c.Calls.AdmitSpaceMediaParticipant(ctx, call.RoomID, store.SpaceMediaParticipant{
			AccountID: admission.AccountID.String(), AdmissionOperationID: admission.OperationID.String(),
			RoomGeneration: admission.RoomGeneration, CreatedRoom: admission.CreatedRoom,
			ProfileID: admission.ProfileID.String(), Identity: admission.Identity, Generation: admission.Generation,
			Issued: store.SpaceMediaGrant{SessionEpoch: admission.SessionEpoch, AccessEpoch: admission.AccessEpoch, PolicyEpoch: admission.PolicyEpoch,
				CanJoin: admission.CanJoin, CanPublishAudio: admission.CanPublishAudio, CanSubscribe: admission.CanSubscribe},
		}, admission.MaxParticipants); err != nil {
			if errors.Is(err, store.ErrActiveCall) {
				// Another operation won the single active-call slot for this
				// voice_room_id. This operation never became public; abort its
				// exact fence and hidden shell, leaving its outbox unclaimable.
				return c.abortAdmission(ctx, admission)
			}
			return err
		}
	}
	return c.Admissions.MarkProjectionApplied(ctx, admission.OperationID, admission.Generation)
}

func (c *Coordinator) reconcileParticipant(ctx context.Context, call store.Call, participant store.SpaceMediaParticipant) error {
	// A revocation begun for this immutable LiveKit incarnation must finish even
	// if authority has since been restored. Rejoin receives a new generation.
	if participant.Revoking {
		return c.eject(ctx, call, participant)
	}
	if participant.AdmissionOperationID != "" {
		operationID, err := uuid.Parse(participant.AdmissionOperationID)
		if err != nil || c.Admissions == nil || c.Admissions.ConfirmProjection(ctx, operationID, participant.Generation) != nil {
			return ErrAdmissionConflict
		}
	}
	if c.SessionEpochChecker == nil || participant.AccountID == "" || participant.Issued.SessionEpoch == 0 {
		return c.eject(ctx, call, participant)
	}
	if err := c.SessionEpochChecker.RequireCurrent(ctx, participant.AccountID, int64(participant.Issued.SessionEpoch)); err != nil {
		if status.Code(err) == codes.Unauthenticated || status.Code(err) == codes.NotFound {
			return c.eject(ctx, call, participant)
		}
		return err
	}

	access, accessErr := c.Access.ResolveVoiceRoomAccess(ctx, call.VoiceRoomID, participant.ProfileID)
	if accessErr != nil && status.Code(accessErr) != codes.NotFound {
		return accessErr
	}
	if accessErr == nil {
		if access.SpaceID != call.SpaceID || access.AccessEpoch == 0 {
			return fmt.Errorf("incomplete canonical Space voice authority")
		}
		if _, err := c.Store.RaiseSpaceMediaEpochFloor(ctx, call.SpaceID, store.SpaceAccessEpoch, access.AccessEpoch); err != nil {
			return err
		}
	}
	if accessErr != nil || !access.Active || !access.Member {
		return c.eject(ctx, call, participant)
	}

	grants, err := c.Grants.ResolveVoiceRoomGrants(ctx, call.SpaceID, call.VoiceRoomID, participant.ProfileID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return c.eject(ctx, call, participant)
		}
		return err
	}
	if grants.PolicyEpoch == 0 {
		return fmt.Errorf("incomplete canonical Role voice authority")
	}
	if _, err := c.Store.RaiseSpaceMediaEpochFloor(ctx, call.SpaceID, store.RolePolicyEpoch, grants.PolicyEpoch); err != nil {
		return err
	}
	if !grants.CanJoin || !grants.CanSubscribe {
		return c.eject(ctx, call, participant)
	}
	grant := store.SpaceMediaGrant{
		SessionEpoch: participant.Issued.SessionEpoch,
		AccessEpoch:  access.AccessEpoch, PolicyEpoch: grants.PolicyEpoch,
		CanJoin: grants.CanJoin, CanPublishAudio: grants.CanPublishAudio, CanSubscribe: grants.CanSubscribe,
	}
	_, _, err = c.Store.ReconcileSpaceMediaParticipant(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation, grant)
	if errors.Is(err, store.ErrSpaceMediaStaleGrant) {
		return ErrReconciliationPending
	}
	return err
}

func (c *Coordinator) eject(ctx context.Context, call store.Call, participant store.SpaceMediaParticipant) error {
	_, _, err := c.ejectParticipant(ctx, call, participant)
	return err
}

// RevokeSpaceMediaParticipant removes only the exact incarnation captured by
// the caller's current call snapshot. The revoking CAS remains durable if the
// LiveKit removal fails, so a client leave cannot erase a pending target.
func (c *Coordinator) RevokeSpaceMediaParticipant(ctx context.Context, call store.Call, participant store.SpaceMediaParticipant) (store.Call, bool, error) {
	if c == nil || c.Store == nil || c.Media == nil || call.SpaceID == "" || participant.ProfileID == "" || participant.Identity == "" || participant.Generation == "" {
		return store.Call{}, false, store.ErrInvalidState
	}
	current, ok := call.SpaceMedia[participant.ProfileID]
	if !ok || current.Identity != participant.Identity || current.Generation != participant.Generation {
		return call, false, nil
	}
	return c.ejectParticipant(ctx, call, current)
}

func (c *Coordinator) ejectParticipant(ctx context.Context, call store.Call, participant store.SpaceMediaParticipant) (store.Call, bool, error) {
	if c.Admissions == nil {
		return call, false, store.ErrInvalidState
	}
	operationID, opErr := uuid.Parse(participant.AdmissionOperationID)
	if opErr != nil {
		return call, false, store.ErrInvalidState
	}
	if err := c.Admissions.BeginRoomLeave(ctx, operationID, participant.Generation); err != nil {
		return call, false, err
	}
	_, matched, err := c.Store.BeginSpaceMediaRevocation(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation)
	if err != nil || !matched {
		return call, false, err
	}
	if err := c.Media.RemoveParticipant(ctx, call.LivekitRoomName, participant.Identity); err != nil {
		return call, false, err
	}
	updated, completed, err := c.Store.CompleteSpaceMediaRevocation(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation)
	if err != nil || !completed {
		return updated, completed, err
	}
	accountID, accountErr := uuid.Parse(participant.AccountID)
	profileID, profileErr := uuid.Parse(participant.ProfileID)
	spaceID, spaceErr := uuid.Parse(call.SpaceID)
	if accountErr != nil || profileErr != nil || spaceErr != nil {
		return updated, false, store.ErrInvalidState
	}
	if err := c.Admissions.CompleteRoomLeave(ctx, store.SpaceMediaAdmission{
		OperationID: operationID, Generation: participant.Generation, AccountID: accountID,
		ProfileID: profileID, SpaceID: spaceID, RoomID: call.RoomID,
	}); err != nil {
		return updated, false, err
	}
	return updated, true, nil
}

func (c *Coordinator) ReconcileAll(ctx context.Context) error {
	if c == nil || c.Store == nil {
		return store.ErrInvalidState
	}
	c.reconcileMu.Lock()
	defer c.reconcileMu.Unlock()
	c.setReady(false)
	if err := c.reconcileAdmissions(ctx); err != nil {
		return err
	}
	spaces, err := c.Store.ListActiveSpaceIDs(ctx)
	if err != nil {
		return err
	}
	for _, spaceID := range spaces {
		if err := c.ReconcileSpace(ctx, spaceID); err != nil {
			return err
		}
	}
	c.setReady(true)
	return nil
}

// Run starts with an immediate full indexed sweep and repeats at a bounded
// interval. A failed pass leaves its durable reconciled watermark behind.
func (c *Coordinator) Run(ctx context.Context) error {
	if c == nil || c.Store == nil {
		return store.ErrInvalidState
	}
	every := c.Every
	if every <= 0 || every > ReconcileInterval {
		every = ReconcileInterval
	}
	if err := c.ReconcileAll(ctx); err != nil && c.OnError != nil {
		c.OnError(err)
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.ReconcileAll(ctx); err != nil && c.OnError != nil {
				c.OnError(err)
			}
		}
	}
}

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

var _ MediaLifecycle = (*livekit.RoomLifecycle)(nil)
