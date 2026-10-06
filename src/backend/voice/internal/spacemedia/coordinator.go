package spacemedia

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	Store   Store
	Access  AccessResolver
	Grants  GrantResolver
	Media   MediaLifecycle
	Every   time.Duration
	OnError func(error)
}

func (c *Coordinator) Observe(ctx context.Context, notice AuthorityNotice) error {
	if c == nil || c.Store == nil || c.Access == nil || c.Grants == nil || c.Media == nil ||
		!canonicalID(notice.SpaceID) || notice.Epoch == 0 ||
		(notice.Kind != store.SpaceAccessEpoch && notice.Kind != store.RolePolicyEpoch) {
		return store.ErrInvalidState
	}
	if _, err := c.Store.RaiseSpaceMediaEpochFloor(ctx, notice.SpaceID, notice.Kind, notice.Epoch); err != nil {
		return err
	}
	return c.ReconcileSpace(ctx, notice.SpaceID)
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

func (c *Coordinator) reconcileParticipant(ctx context.Context, call store.Call, participant store.SpaceMediaParticipant) error {
	// A revocation begun for this immutable LiveKit incarnation must finish even
	// if authority has since been restored. Rejoin receives a new generation.
	if participant.Revoking {
		return c.eject(ctx, call, participant)
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
	_, matched, err := c.Store.BeginSpaceMediaRevocation(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation)
	if err != nil || !matched {
		return err
	}
	if err := c.Media.RemoveParticipant(ctx, call.LivekitRoomName, participant.Identity); err != nil {
		return err
	}
	_, _, err = c.Store.CompleteSpaceMediaRevocation(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation)
	return err
}

func (c *Coordinator) ReconcileAll(ctx context.Context) error {
	if c == nil || c.Store == nil {
		return store.ErrInvalidState
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
