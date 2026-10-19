package spacemedia

import (
	"context"
	"errors"
	"fmt"
	"math"
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

var ErrReconciliationPending = errors.New("space media authority changed during reconciliation")

type AccessResolver interface {
	ResolveVoiceRoomAccess(context.Context, string, string, string) (grpcsvc.CanonicalVoiceRoomAccess, error)
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
	BeginOwnedRoomLeave(context.Context, store.SpaceAdmissionOwnership, uuid.UUID, string) error
	CompleteOwnedRoomLeave(context.Context, store.SpaceAdmissionOwnership, store.SpaceMediaAdmission) error
	BeginRestrictiveRoomLeave(context.Context, uuid.UUID, string) (store.SpaceAdmissionRevocation, error)
	CompleteRestrictiveRoomLeave(context.Context, store.SpaceAdmissionRevocation) error
	FenceOwnedRoomLeave(context.Context, store.SpaceAdmissionOwnership, uuid.UUID, store.SpaceAdmissionProjectionGates) (store.SpaceProjectionTicket, error)
	FenceRestrictiveRoomLeave(context.Context, store.SpaceAdmissionRevocation, store.SpaceAdmissionProjectionGates) (store.SpaceProjectionTicket, error)
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
	CurrentFloor(context.Context, string) (int64, error)
}

type accountFloorResult struct {
	epoch int64
	err   error
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
	ProjectionGates     store.SpaceProjectionGates
	ResumeP3            func(context.Context) error
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
	return c.reconcileSpace(ctx, spaceID, make(map[string]accountFloorResult))
}

func (c *Coordinator) reconcileSpace(ctx context.Context, spaceID string, accountFloors map[string]accountFloorResult) error {
	if c == nil || c.Store == nil || c.Access == nil || c.Grants == nil || c.Media == nil || !canonicalID(spaceID) {
		return store.ErrInvalidState
	}
	for pass := 0; pass < reconcilePasses; pass++ {
		if pass > 0 {
			// Discard all values, including those read for earlier Spaces in a
			// full sweep, when a watermark change requires fresh authority.
			clear(accountFloors)
		}
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
				if err := c.reconcileParticipant(ctx, call, participant, accountFloors); err != nil {
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
			if err := c.reconcileOwnedAdmission(ctx, admission); err != nil {
				return err
			}
		}
	}
	// Rebuild creators first, with the same durable ownership arbitration as
	// unfinished operations. Another healthy owner can finish its own projection.
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
				if err := c.reconcileOwnedAdmission(ctx, admission); err != nil {
					return err
				}
				after = admission.OperationID
			}
		}
	}
	return nil
}

func (c *Coordinator) reconcileOwnedAdmission(ctx context.Context, candidate store.SpaceMediaAdmission) error {
	ownership, ok := c.Admissions.(interface {
		ClaimAdmissionRecovery(context.Context, store.SpaceMediaAdmission) (store.SpaceMediaAdmission, bool, error)
		HoldAdmissionOwnership(context.Context, uuid.UUID) (context.Context, func(), error)
	})
	if !ok {
		return ErrAdmissionUnavailable
	}
	admission, claimed, err := ownership.ClaimAdmissionRecovery(ctx, candidate)
	if err != nil || !claimed {
		return err
	}
	ctx = store.WithSpaceAdmissionOwnership(ctx, admission.Ownership)
	ctx, stop, err := ownership.HoldAdmissionOwnership(ctx, admission.OperationID)
	if err != nil {
		return err
	}
	defer stop()
	if admission.State == AdmissionCommitted && admission.ProjectionApplied && admission.ParticipantState == "REVOKING" {
		return c.recoverRevokingAdmission(ctx, admission)
	}
	if admission.State == AdmissionAborting || admission.State == AdmissionPrepared || admission.State == AdmissionFenced {
		return c.abortAdmission(ctx, admission)
	}
	if admission.State != AdmissionCommitted || admission.ProjectionApplied {
		if admission.State == AdmissionCommitted && admission.ProjectionApplied && admission.ParticipantState == "ACTIVE" {
			return c.recoverCommittedAdmission(ctx, admission)
		}
		return nil
	}
	return c.recoverCommittedAdmission(ctx, admission)
}

func (c *Coordinator) recoverRevokingAdmission(ctx context.Context, admission store.SpaceMediaAdmission) error {
	call, err := c.Calls.GetCall(ctx, admission.RoomID)
	if err == nil {
		if participant, ok := call.SpaceMedia[admission.ProfileID.String()]; ok {
			if participant.AdmissionOperationID != admission.OperationID.String() || participant.Generation != admission.Generation || participant.Identity != admission.Identity {
				return ErrAdmissionConflict
			}
			_, completed, err := c.ejectOwnedParticipant(ctx, admission.Ownership, call, participant)
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
	gates, ok := c.ProjectionGates.(store.SpaceAdmissionProjectionGates)
	if !ok {
		return ErrAdmissionUnavailable
	}
	if err := c.Admissions.BeginOwnedRoomLeave(ctx, admission.Ownership, admission.OperationID, admission.Generation); err != nil {
		return err
	}
	if _, err := c.Admissions.FenceOwnedRoomLeave(ctx, admission.Ownership, admission.OperationID, gates); err != nil {
		return err
	}
	// The database may be ahead of a lost/removed Redis projection. Remove the
	// same immutable LiveKit identity before releasing its fence or ending the
	// room generation; a failed removal keeps readiness closed for retry.
	if err := c.Media.RemoveParticipant(ctx, "voice-room-"+admission.VoiceRoomID, admission.Identity); err != nil {
		return err
	}
	return c.Admissions.CompleteOwnedRoomLeave(ctx, admission.Ownership, admission)
}

func (c *Coordinator) abortAdmission(ctx context.Context, admission store.SpaceMediaAdmission) error {
	owner, ok := store.SpaceAdmissionOwnershipFromContext(ctx, admission.OperationID)
	if !ok {
		return ErrAdmissionConflict
	}
	admission.Ownership = owner
	if admission.State != AdmissionAborting {
		if err := c.Admissions.MarkAborting(ctx, admission.OperationID, admission.Generation); err != nil {
			return err
		}
	}
	fencer, ok := c.Admissions.(interface {
		FenceAbortedAdmission(context.Context, uuid.UUID, store.SpaceAdmissionProjectionGates) error
	})
	gates, gatesOK := c.ProjectionGates.(store.SpaceAdmissionProjectionGates)
	if !ok || !gatesOK {
		return ErrAdmissionUnavailable
	}
	if err := fencer.FenceAbortedAdmission(ctx, admission.OperationID, gates); err != nil {
		return err
	}
	admission.State = AdmissionAborting
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
				if _, completed, err := c.ejectOwnedParticipant(ctx, admission.Ownership, call, participant); err != nil || !completed {
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
	ticket, err := c.prepareProjection(ctx, admission)
	if err != nil {
		return err
	}

	if err := c.Admissions.ConfirmRoomHeadOpen(ctx, admission); err != nil {
		return err
	}
	access, err := c.Access.ResolveVoiceRoomAccess(ctx, admission.SpaceID.String(), admission.VoiceRoomID, admission.ProfileID.String())
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if err != nil && status.Code(err) != codes.NotFound {
		return err
	}
	if err == nil && !access.CommunicationStateResolved {
		return status.Error(codes.Unavailable, "Space communication authority incomplete")
	}
	if err != nil || access.SpaceID != admission.SpaceID.String() || !access.Active || !access.Member || access.TimedOut || access.AccessEpoch != admission.AccessEpoch {
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
		// A creator may have committed before its first Redis dispatch. Its
		// immutable journal is then its own creator witness; requiring an applied
		// marker would make this crash boundary permanently unrecoverable.
		creator := admission
		if !admission.CreatedRoom {
			var creatorErr error
			creator, creatorErr = c.Admissions.RoomCreator(ctx, admission.VoiceRoomID, admission.RoomGeneration)
			if creatorErr != nil {
				return creatorErr
			}
		}
		call = store.Call{SpaceProjectionTicket: ticket, RoomID: admission.RoomID, SpaceLifecycleGeneration: admission.LifecycleGeneration, LivekitRoomName: "voice-room-" + admission.VoiceRoomID,
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
			ProjectionTicket: ticket, LifecycleGeneration: admission.LifecycleGeneration, RoomGeneration: admission.RoomGeneration, CreatedRoom: admission.CreatedRoom,
			ProfileID: admission.ProfileID.String(), Identity: admission.Identity, Generation: admission.Generation,
			Issued: store.SpaceMediaGrant{SessionEpoch: admission.SessionEpoch, AccessEpoch: admission.AccessEpoch, PolicyEpoch: admission.PolicyEpoch,
				CanJoin: admission.CanJoin, CanPublishAudio: admission.CanPublishAudio, CanSubscribe: admission.CanSubscribe},
		}, admission.MaxParticipants); err != nil {
			if errors.Is(err, store.ErrActiveCall) || errors.Is(err, store.ErrRoomFull) {
				// Another operation won the active-call slot or the room reached its
				// participant cap. This operation never became public; abort its
				// exact fence and hidden shell, leaving its outbox unclaimable.
				return c.abortAdmission(ctx, admission)
			}
			return err
		}
	}
	return c.Admissions.MarkProjectionApplied(ctx, admission.OperationID, admission.Generation)
}

func (c *Coordinator) reconcileParticipant(ctx context.Context, call store.Call, participant store.SpaceMediaParticipant, accountFloors map[string]accountFloorResult) error {
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
	if c.SessionEpochChecker == nil {
		return status.Error(codes.Unavailable, "Auth session epoch floor unavailable")
	}
	if !canonicalID(participant.AccountID) || participant.Issued.SessionEpoch == 0 {
		return c.eject(ctx, call, participant)
	}
	if participant.Issued.SessionEpoch > math.MaxInt64 {
		return status.Error(codes.Unavailable, "invalid persisted Auth session epoch")
	}
	floor, found := accountFloors[participant.AccountID]
	if !found {
		floor.epoch, floor.err = c.SessionEpochChecker.CurrentFloor(ctx, participant.AccountID)
		accountFloors[participant.AccountID] = floor
	}
	if floor.err != nil {
		if status.Code(floor.err) == codes.Unauthenticated || status.Code(floor.err) == codes.NotFound {
			return c.eject(ctx, call, participant)
		}
		return floor.err
	}
	if floor.epoch <= 0 {
		return status.Error(codes.Unavailable, "Auth session epoch floor unavailable")
	}
	if participant.Issued.SessionEpoch < uint64(floor.epoch) {
		return c.eject(ctx, call, participant)
	}

	access, accessErr := c.Access.ResolveVoiceRoomAccess(ctx, call.SpaceID, call.VoiceRoomID, participant.ProfileID)
	if accessErr != nil && status.Code(accessErr) != codes.NotFound {
		return accessErr
	}
	if accessErr == nil {
		if access.SpaceID != call.SpaceID || access.AccessEpoch == 0 || !access.CommunicationStateResolved {
			return fmt.Errorf("incomplete canonical Space voice authority")
		}
		if _, err := c.Store.RaiseSpaceMediaEpochFloor(ctx, call.SpaceID, store.SpaceAccessEpoch, access.AccessEpoch); err != nil {
			return err
		}
	}
	if accessErr != nil || !access.Active || !access.Member || access.TimedOut {
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
	projectionCtx := ctx
	if participant.AdmissionOperationID != "" {
		op, err := uuid.Parse(participant.AdmissionOperationID)
		if err != nil {
			return err
		}
		ticket, err := c.prepareProjection(ctx, store.SpaceMediaAdmission{OperationID: op, Generation: participant.Generation, SpaceID: uuid.MustParse(call.SpaceID)})
		if err != nil {
			return err
		}
		projectionCtx = store.WithSpaceProjectionTicket(ctx, ticket)
	}

	_, _, err = c.Store.ReconcileSpaceMediaParticipant(projectionCtx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation, grant)
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
	op, err := uuid.Parse(participant.AdmissionOperationID)
	if err != nil || c.Admissions == nil {
		return call, false, store.ErrInvalidState
	}
	target, err := c.Admissions.BeginRestrictiveRoomLeave(ctx, op, participant.Generation)
	if err != nil {
		return call, false, err
	}
	gates, ok := c.ProjectionGates.(store.SpaceAdmissionProjectionGates)
	if !ok {
		return call, false, ErrAdmissionUnavailable
	}
	ticket, err := c.Admissions.FenceRestrictiveRoomLeave(ctx, target, gates)
	if err != nil {
		return call, false, err
	}
	return c.drainParticipant(ctx, call, participant, ticket, func(ctx context.Context) error { return c.Admissions.CompleteRestrictiveRoomLeave(ctx, target) })
}

func (c *Coordinator) ejectOwnedParticipant(ctx context.Context, owner store.SpaceAdmissionOwnership, call store.Call, participant store.SpaceMediaParticipant) (store.Call, bool, error) {
	op, err := uuid.Parse(participant.AdmissionOperationID)
	if err != nil || c.Admissions == nil {
		return call, false, store.ErrInvalidState
	}
	if err := c.Admissions.BeginOwnedRoomLeave(ctx, owner, op, participant.Generation); err != nil {
		return call, false, err
	}
	gates, ok := c.ProjectionGates.(store.SpaceAdmissionProjectionGates)
	if !ok {
		return call, false, ErrAdmissionUnavailable
	}
	ticket, err := c.Admissions.FenceOwnedRoomLeave(ctx, owner, op, gates)
	if err != nil {
		return call, false, err
	}
	a := store.SpaceMediaAdmission{OperationID: op, Generation: participant.Generation, AccountID: uuid.MustParse(participant.AccountID), ProfileID: uuid.MustParse(participant.ProfileID), SpaceID: uuid.MustParse(call.SpaceID), RoomID: call.RoomID}
	return c.drainParticipant(ctx, call, participant, ticket, func(ctx context.Context) error { return c.Admissions.CompleteOwnedRoomLeave(ctx, owner, a) })
}

func (c *Coordinator) drainParticipant(ctx context.Context, call store.Call, participant store.SpaceMediaParticipant, ticket store.SpaceProjectionTicket, complete func(context.Context) error) (store.Call, bool, error) {
	ticket.SpaceID = call.SpaceID
	ticket.RoomID = call.RoomID
	ticket.VoiceRoomID = call.VoiceRoomID
	ticket.RoomGeneration = participant.RoomGeneration
	ticket.OperationID = participant.AdmissionOperationID
	ticket.AdmissionGeneration = participant.Generation
	ticket.AccountID = participant.AccountID
	ticket.ProfileID = participant.ProfileID
	ticket.Identity = participant.Identity
	ctx = store.WithSpaceCleanupTarget(ctx, ticket)
	_, matched, err := c.Store.BeginSpaceMediaRevocation(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation)
	if err != nil || !matched {
		return call, false, err
	}
	if err := c.Media.RemoveParticipant(ctx, call.LivekitRoomName, participant.Identity); err != nil {
		return call, false, err
	}
	updated, done, err := c.Store.CompleteSpaceMediaRevocation(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation)
	if err != nil || !done {
		return updated, done, err
	}
	if err := complete(ctx); err != nil {
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
	// Retain established readiness across a successful periodic refresh; each RPC
	// still checks current authority and journal state. Startup stays closed, and
	// a failed refresh closes serving below.
	if !c.ready {
		c.setReady(false)
	}
	converged := false
	defer func() {
		if !converged && c.ready {
			c.setReady(false)
		}
	}()
	if c.Admissions != nil {
		if c.ProjectionGates == nil {
			return ErrAdmissionUnavailable
		}
		authority, ok := c.Admissions.(interface {
			RebuildLifecycleWatermarks(context.Context, store.SpaceProjectionGates) error
		})
		if !ok {
			return ErrAdmissionUnavailable
		}
		if err := authority.RebuildLifecycleWatermarks(ctx, c.ProjectionGates); err != nil {
			return err
		}
	}

	if c.ResumeP3 != nil {
		if err := c.ResumeP3(ctx); err != nil && c.OnError != nil {
			c.OnError(err)
		}
	}
	if err := c.reconcileAdmissions(ctx); err != nil {
		return err
	}
	spaces, err := c.Store.ListActiveSpaceIDs(ctx)
	if err != nil {
		return err
	}
	// Profiles in different Spaces may share an account and carry different
	// epochs. Cache the authority value, never a profile decision, for this sweep.
	accountFloors := make(map[string]accountFloorResult)
	for _, spaceID := range spaces {
		if err := c.reconcileSpace(ctx, spaceID, accountFloors); err != nil {
			return err
		}
	}
	if !c.ready {
		c.setReady(true)
	}
	converged = true
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

func (c *Coordinator) prepareProjection(ctx context.Context, a store.SpaceMediaAdmission) (store.SpaceProjectionTicket, error) {
	if c.ProjectionGates == nil {
		return store.SpaceProjectionTicket{}, ErrAdmissionUnavailable
	}
	authority, ok := c.Admissions.(interface {
		ReadSpaceLifecycle(context.Context, string) (int64, bool, error)
		ReadAdmissionDispatch(context.Context, string) (int64, bool, error)
		AuthorizeSpaceProjection(context.Context, uuid.UUID, string, store.SpaceProjectionTicket) (store.SpaceProjectionTicket, error)
	})
	if !ok {
		return store.SpaceProjectionTicket{}, ErrAdmissionUnavailable
	}
	if err := c.ProjectionGates.EnsureSpaceLifecycle(ctx, a.SpaceID.String(), authority.ReadSpaceLifecycle); err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	observed, err := c.ProjectionGates.ReadReadySpaceGate(ctx, a.SpaceID.String())
	if err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	gates, ok := c.ProjectionGates.(store.SpaceAdmissionProjectionGates)
	if !ok {
		return store.SpaceProjectionTicket{}, ErrAdmissionUnavailable
	}
	dispatch, err := gates.EnsureAdmissionDispatch(ctx, a.OperationID.String(), authority.ReadAdmissionDispatch)
	if err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	observed.AdmissionIncarnation = dispatch.AdmissionIncarnation
	observed.AdmissionOwnerEpoch = dispatch.AdmissionOwnerEpoch
	observed.AdmissionLive = dispatch.AdmissionLive
	return authority.AuthorizeSpaceProjection(ctx, a.OperationID, a.Generation, observed)
}
