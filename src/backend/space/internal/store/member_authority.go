package store

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

var ErrMemberAuthorityUnavailable = errors.New("member authority unavailable")

// ReadMemberAdmissionAuthority is an own-DB snapshot under the native Space
// fence. It neither calls Role nor issues a membership or permission capability.
func (s *SpaceStore) ReadMemberAdmissionAuthority(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
	if s == nil || s.Pool == nil || s.tx != nil || req == nil || req.GetProtocolVersion() != 1 || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err := validateBotMessageSelector(req); err != nil {
		return nil, err
	}
	var botDeadline time.Time
	if req.GetBotMessageSendBinding() != nil {
		botDeadline = time.Now().Add(5 * time.Second)
		if earlier, ok := ctx.Deadline(); ok && earlier.Before(botDeadline) {
			botDeadline = earlier
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, botDeadline)
		defer cancel()
	}
	space, err := canonicalLifecycleUUID(req.GetSpaceId())
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	profile, err := canonicalLifecycleUUID(req.GetProfileId())
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if (req.ExpectedGeneration == nil) != (req.ExpectedAdmissionOperationId == nil) || req.GetExpectedGeneration() > math.MaxInt64 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if req.ExpectedAdmissionOperationId != nil {
		if _, err = canonicalLifecycleUUID(*req.ExpectedAdmissionOperationId); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	if req.ChatId != nil {
		if _, err = canonicalLifecycleUUID(*req.ChatId); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockLifecycleSpace(ctx, tx, space); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var schema int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&schema); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if schema == 44 || schema == 45 {
		// Catalog authentication above is necessary but not per-Space cutover.
		// The same fenced snapshot must prove COMPLETE and an actual serving
		// LOGIN before any positive source facts leave this protected route.
		if _, err = tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, space); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	var sampled time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	revision, err := spaceSourceRevision(ctx, tx, space.String())
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	response := &spacev1.GetMemberAdmissionAuthorityResponse{ProtocolVersion: 1, SpaceId: space.String(), ProfileId: profile.String(), SpaceAuthorityRevision: revision, SampledAt: timestamppb.New(sampled), ValidityRemaining: durationpb.New(5 * time.Second)}
	response.Lifecycle, err = readMemberLifecycle(ctx, tx, space)
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	response.Ownership, err = readMemberOwnership(ctx, tx, space)
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	response.Ordinary, err = readOrdinaryMemberAuthority(ctx, tx, space, profile)
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	response.Community, err = readCommunityMemberAuthority(ctx, tx, space, profile, sampled)
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if response.Community.GetState() == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
		if validity := response.Community.GetValidityRemaining().AsDuration(); validity < response.ValidityRemaining.AsDuration() {
			response.ValidityRemaining = durationpb.New(validity)
		}
	}
	if req.ChatId != nil {
		var present bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_tree_nodes WHERE space_id=$1 AND chat_id=$2 AND kind='text_chat')`, space, *req.ChatId).Scan(&present); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		response.TreeRelation = &spacev1.MemberAdmissionTreeRelationAuthority{ChatId: *req.ChatId, Present: present, Revision: revision}
	}
	// Detect a noncanonical unscoped writer rather than returning mixed snapshots.
	if binding := req.GetBotMessageSendBinding(); binding != nil {
		if err = qualifyBotMessageAuthority(ctx, tx, binding, response); err != nil {
			return nil, err
		}
	}
	last, err := spaceSourceRevision(ctx, tx, space.String())
	if err != nil || last != revision {
		return nil, ErrMemberAuthorityUnavailable
	}
	var finished time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	remaining := response.ValidityRemaining.AsDuration() - finished.Sub(sampled)
	if !botDeadline.IsZero() && time.Until(botDeadline) < remaining {
		remaining = time.Until(botDeadline)
	}
	if remaining <= 0 || remaining > 5*time.Second {
		return nil, ErrMemberAuthorityUnavailable
	}
	response.ValidityRemaining = durationpb.New(remaining)
	if err = tx.Commit(ctx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if !botDeadline.IsZero() {
		remaining = time.Until(botDeadline)
		if ctx.Err() != nil || remaining <= 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
		if remaining < response.ValidityRemaining.AsDuration() {
			response.ValidityRemaining = durationpb.New(remaining)
		}
	}
	return response, nil
}

func readMemberLifecycle(ctx context.Context, tx pgx.Tx, space uuid.UUID) (*spacev1.MemberAdmissionLifecycleAuthority, error) {
	// A permanent terminal fence dominates even a recreated Space/aggregate.
	// Read it independently; an existing LIVE aggregate is not proof of reuse.
	var purgedGeneration uint64
	err := tx.QueryRow(ctx, `SELECT purge_generation FROM space_permanent_purge_fences WHERE space_id=$1`, space).Scan(&purgedGeneration)
	if err == nil {
		if purgedGeneration == 0 || purgedGeneration > math.MaxInt64 {
			return nil, ErrMemberAuthorityUnavailable
		}
		return &spacev1.MemberAdmissionLifecycleAuthority{Generation: purgedGeneration, State: spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_RETIRED}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMemberAuthorityUnavailable
	}
	var phase string
	var generation uint64
	err = tx.QueryRow(ctx, `SELECT phase,generation FROM space_lifecycle_aggregates WHERE space_id=$1`, space).Scan(&phase, &generation)
	if errors.Is(err, pgx.ErrNoRows) {
		var initial bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1)
		 AND NOT EXISTS(SELECT 1 FROM space_lifecycle_operations WHERE space_id=$1)
		 AND NOT EXISTS(SELECT 1 FROM space_lifecycle_participants WHERE space_id=$1)
		 AND NOT EXISTS(SELECT 1 FROM space_lifecycle_completions WHERE space_id=$1)
		 AND NOT EXISTS(SELECT 1 FROM space_lifecycle_outbox WHERE space_id=$1)
		 AND NOT EXISTS(SELECT 1 FROM space_deletion_tombstones WHERE space_id=$1)
		 AND NOT EXISTS(SELECT 1 FROM space_permanent_purge_fences WHERE space_id=$1)
		 AND NOT EXISTS(SELECT 1 FROM space_membership_ownership_heads WHERE space_id=$1 AND state='RETIRED')`, space).Scan(&initial)
		if err != nil || !initial {
			return nil, ErrMemberAuthorityUnavailable
		}
		return &spacev1.MemberAdmissionLifecycleAuthority{State: spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE}, nil
	}
	if err != nil || generation == 0 || generation > math.MaxInt64 {
		return nil, ErrMemberAuthorityUnavailable
	}
	state := spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN
	switch phase {
	case "LIVE":
		state = spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE
	case "SCHEDULE_PENDING", "FREEZE_PENDING", "SCHEDULED", "RESTORE_DECIDED":
	case "PURGE_DECIDED", "PURGING":
		state = spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_PURGE_DECIDED
	case "PURGED":
		state = spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_RETIRED
	default:
		return nil, ErrMemberAuthorityUnavailable
	}
	return &spacev1.MemberAdmissionLifecycleAuthority{Generation: generation, State: state}, nil
}

func readMemberOwnership(ctx context.Context, tx pgx.Tx, space uuid.UUID) (*spacev1.MemberAdmissionOwnershipAuthority, error) {
	var generation uint64
	var owner, current *uuid.UUID
	var state string
	var pending bool
	var discordant bool
	err := tx.QueryRow(ctx, `SELECT h.generation,h.owner_profile_id,h.state,s.owner_profile_id,
	 EXISTS(SELECT 1 FROM ownership_journal j WHERE j.space_id=h.space_id AND j.state NOT IN ('completed','aborted'))
	 OR EXISTS(SELECT 1 FROM community_owner_authority a WHERE a.space_id=h.space_id AND a.status<>'active'),
	 EXISTS(SELECT 1 FROM community_owner_authority a WHERE a.space_id=h.space_id AND a.status='active' AND a.owner_profile_id IS DISTINCT FROM s.owner_profile_id)
	 FROM space_membership_ownership_heads h LEFT JOIN spaces s ON s.id=h.space_id WHERE h.space_id=$1`, space).Scan(&generation, &owner, &state, &current, &pending, &discordant)
	if err != nil || generation == 0 || generation > math.MaxInt64 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if discordant {
		return nil, ErrMemberAuthorityUnavailable
	}
	if state == "RETIRED" && owner == nil && current == nil {
		var terminal bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_permanent_purge_fences WHERE space_id=$1)`, space).Scan(&terminal); err != nil || !terminal {
			return nil, ErrMemberAuthorityUnavailable
		}
		return &spacev1.MemberAdmissionOwnershipAuthority{Generation: generation, State: spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_RETIRED}, nil
	}
	if state != "ACTIVE" || owner == nil || current == nil || *owner != *current || *owner == uuid.Nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	result := spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE
	if pending {
		result = spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED
	}
	return &spacev1.MemberAdmissionOwnershipAuthority{Generation: generation, State: result, CurrentOwnerProfileId: owner.String()}, nil
}

func ordinaryMembershipBinding(g uint64, operation uuid.UUID, legacy bool) *rolev1.MembershipBinding {
	return &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: g, PublicationIdentity: operation.String(), Legacy: legacy}}}
}

func readOrdinaryMemberAuthority(ctx context.Context, tx pgx.Tx, space, profile uuid.UUID) (*spacev1.MemberAdmissionOrdinaryAuthority, error) {
	var row ordinaryAuthorityRow
	err := tx.QueryRow(ctx, `SELECT generation,retired_through,publication_id,state,legacy,role_receipt_sha256,admission_binding_sha256,publication_kind,native_publication_sha256,account_id
	 FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space, profile).Scan(&row.generation, &row.retiredThrough, &row.operation, &row.state, &row.legacy, &row.receipt, &row.binding, &row.kind, &row.native, &row.account)
	if errors.Is(err, pgx.ErrNoRows) {
		return ordinaryAuthorityFromRow(ctx, tx, space, profile, row, nil)
	}
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	row.found = true
	return ordinaryAuthorityFromRow(ctx, tx, space, profile, row, nil)
}

func readCommunityMemberAuthority(ctx context.Context, tx pgx.Tx, space, profile uuid.UUID, sampled time.Time) (*spacev1.MemberAdmissionCommunityAuthority, error) {
	var row communityAuthorityRow
	err := tx.QueryRow(ctx, `SELECT generation,retired_through,owner_generation,episode_id,state FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2`, space, profile).Scan(&row.generation, &row.retiredThrough, &row.ownerGeneration, &row.episode, &row.state)
	if errors.Is(err, pgx.ErrNoRows) {
		return communityAuthorityFromRow(ctx, tx, space, profile, sampled, row, nil)
	}
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	row.found = true
	return communityAuthorityFromRow(ctx, tx, space, profile, sampled, row, nil)
}
