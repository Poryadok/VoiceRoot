package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
)

var ErrMemberAuthorityUnavailable = errors.New("member authority unavailable")

// ReadMemberAdmissionAuthority is an own-DB snapshot under the native Space
// fence. It neither calls Role nor issues a membership or permission capability.
func (s *SpaceStore) ReadMemberAdmissionAuthority(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
	if s == nil || s.Pool == nil || s.tx != nil || req == nil || req.GetProtocolVersion() != 1 || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrMemberAuthorityUnavailable
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
	last, err := spaceSourceRevision(ctx, tx, space.String())
	if err != nil || last != revision {
		return nil, ErrMemberAuthorityUnavailable
	}
	var finished time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	remaining := response.ValidityRemaining.AsDuration() - finished.Sub(sampled)
	if remaining <= 0 || remaining > 5*time.Second {
		return nil, ErrMemberAuthorityUnavailable
	}
	response.ValidityRemaining = durationpb.New(remaining)
	if err = tx.Commit(ctx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
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
	r := &spacev1.MemberAdmissionOrdinaryAuthority{State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE}
	var operation uuid.UUID
	var state string
	var receipt, binding, native []byte
	var kind int32
	var account *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT generation,retired_through,publication_id,state,legacy,role_receipt_sha256,admission_binding_sha256,publication_kind,native_publication_sha256,account_id
	 FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space, profile).Scan(&r.HeadGeneration, &r.RetiredThrough, &operation, &state, &r.ExplicitLegacy, &receipt, &binding, &kind, &native, &account)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space, profile).Scan(&exists); err != nil || exists {
			return nil, ErrMemberAuthorityUnavailable
		}
		return r, nil
	}
	if err != nil || operation == uuid.Nil || r.HeadGeneration > math.MaxInt64 || r.RetiredThrough > r.HeadGeneration {
		return nil, ErrMemberAuthorityUnavailable
	}
	if account != nil {
		var banned bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bans WHERE space_id=$1 AND account_id=$2)`, space, *account).Scan(&banned); err != nil {
			return nil, err
		}
		if banned {
			r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
			return r, nil
		}
	}
	switch state {
	case "RETIRED":
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
		return r, nil
	case "PREPARING":
		if r.HeadGeneration == 0 || r.ExplicitLegacy || len(binding) != 32 || len(receipt) != 0 || len(native) != 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING
		r.PreparingBinding = ordinaryMembershipBinding(r.HeadGeneration, operation, false)
		r.PreparingAdmissionBindingSha256 = binding
		return r, nil
	case "ACTIVE":
		var published bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2 AND membership_generation=$3 AND admission_operation_id=$4)`, space, profile, r.HeadGeneration, operation).Scan(&published); err != nil || !published {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE
		r.PublicationKind = spacev1.MemberAdmissionPublicationKind(kind)
		r.PublishedBinding = ordinaryMembershipBinding(r.HeadGeneration, operation, r.ExplicitLegacy)
		if r.ExplicitLegacy {
			if r.HeadGeneration != 0 || kind != 2 || len(receipt) != 0 || len(native) != 0 {
				return nil, ErrMemberAuthorityUnavailable
			}
			return r, nil
		}
		if r.HeadGeneration == 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
		if kind == 1 {
			if len(receipt) != 32 || len(native) != 0 || len(binding) != 32 {
				return nil, ErrMemberAuthorityUnavailable
			}
			var role uuid.UUID
			var saved []byte
			var savedState string
			if err = tx.QueryRow(ctx, `SELECT role_id,role_receipt_sha256,state FROM space_member_admission_intents WHERE operation_id=$1 AND space_id=$2 AND profile_id=$3 AND generation=$4 AND binding_sha256=$5`, operation, space, profile, r.HeadGeneration, binding).Scan(&role, &saved, &savedState); err != nil || savedState != "PUBLISHED" || !bytes.Equal(saved, receipt) {
				return nil, ErrMemberAuthorityUnavailable
			}
			expected, err := memberadmission.EnsureReceiptFingerprint(memberadmission.EnsureBinding{ProtocolVersion: 1, SpaceID: space.String(), ProfileID: profile.String(), AdmissionOperationID: operation.String(), MembershipGeneration: r.HeadGeneration, AdmissionBindingSHA256: binding}, role.String())
			if err != nil || !bytes.Equal(expected[:], receipt) {
				return nil, ErrMemberAuthorityUnavailable
			}
			r.RoleReceiptSha256 = receipt
			return r, nil
		}
		if kind < 3 || kind > 6 || len(receipt) != 0 || len(native) != 32 {
			return nil, ErrMemberAuthorityUnavailable
		}
		var raw, digest []byte
		var nativeState string
		var nativePublishedAt time.Time
		if err = tx.QueryRow(ctx, `SELECT binding_bytes,binding_sha256,state,published_at FROM space_native_member_publications WHERE operation_id=$1 AND space_id=$2 AND profile_id=$3 AND generation=$4 AND kind=$5`, operation, space, profile, r.HeadGeneration, kind).Scan(&raw, &digest, &nativeState, &nativePublishedAt); err != nil || nativeState != "PUBLISHED" || !bytes.Equal(digest, native) {
			return nil, ErrMemberAuthorityUnavailable
		}
		message := &spacev1.NativeMembershipPublicationBinding{}
		if err = proto.Unmarshal(raw, message); err != nil || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
		canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
		if err != nil || !bytes.Equal(canonical, raw) {
			return nil, ErrMemberAuthorityUnavailable
		}
		expected := sha256.Sum256(append([]byte(nativeMemberPublicationDomain), raw...))
		if !bytes.Equal(expected[:], native) || message.GetProtocolVersion() != 1 || message.GetSpaceId() != space.String() || message.GetProfileId() != profile.String() || message.GetPublicationOperationId() != operation.String() || message.GetOrdinaryGeneration() != r.HeadGeneration || int32(message.GetKind()) != kind {
			return nil, ErrMemberAuthorityUnavailable
		}
		// Native producer paths are activated individually with their own durable
		// witness. Unimplemented creator/recovery/Bot evidence is not a proof.
		switch kind {
		case 3:
			if err := validateCreatorNativeProof(ctx, tx, message); err != nil {
				return nil, err
			}
		case 4:
			if err := validateCommunityBootstrapNativeProof(ctx, tx, message); err != nil {
				return nil, err
			}
		case 5:
			if err := validateCommunityRecoveryNativeProof(ctx, tx, message); err != nil {
				return nil, err
			}
		case 6:
			if err := validateBotNativeProof(ctx, tx, message); err != nil {
				return nil, err
			}
		default:
			return nil, ErrMemberAuthorityUnavailable
		}
		if !message.PublishedAt.AsTime().Equal(nativePublishedAt) {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.NativePublication = &spacev1.MemberAdmissionNativePublicationProof{Kind: r.PublicationKind, OperationId: operation.String(), BindingSha256: native}
		return r, nil
	default:
		return nil, ErrMemberAuthorityUnavailable
	}
}

func readCommunityMemberAuthority(ctx context.Context, tx pgx.Tx, space, profile uuid.UUID, sampled time.Time) (*spacev1.MemberAdmissionCommunityAuthority, error) {
	r := &spacev1.MemberAdmissionCommunityAuthority{State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE}
	var generation, owner uint64
	var episode uuid.UUID
	var state string
	err := tx.QueryRow(ctx, `SELECT generation,retired_through,owner_generation,episode_id,state FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2`, space, profile).Scan(&generation, &r.RetiredThrough, &owner, &episode, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_roster_members m JOIN community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation WHERE m.space_id=$1 AND m.profile_id=$2 AND a.status='active' AND m.revoked_at IS NULL AND m.lease_expires_at>$3 AND a.roster_lease_expires_at>$3)`, space, profile, sampled).Scan(&exists); err != nil || exists {
			return nil, ErrMemberAuthorityUnavailable
		}
		return r, nil
	}
	if err != nil || generation == 0 || generation > math.MaxInt64 || owner == 0 || owner > math.MaxInt64 || r.RetiredThrough > generation || episode == uuid.Nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if state == "RETIRED" {
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
		return r, nil
	}
	if state != "ACTIVE" {
		return nil, ErrMemberAuthorityUnavailable
	}
	var remaining time.Duration
	err = tx.QueryRow(ctx, `SELECT m.source_revision,(EXTRACT(EPOCH FROM (LEAST(m.lease_expires_at,a.roster_lease_expires_at)-$4::timestamptz))*1000000000)::bigint
	 FROM community_roster_members m JOIN community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation
	 WHERE m.space_id=$1 AND m.profile_id=$2 AND m.owner_generation=$3 AND a.status='active' AND m.revoked_at IS NULL
	 AND m.lease_expires_at>$4 AND a.roster_lease_expires_at>$4`, space, profile, owner, sampled).Scan(&r.RosterSourceRevision, &remaining)
	if errors.Is(err, pgx.ErrNoRows) {
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
		return r, nil
	}
	if err != nil || remaining <= 0 || r.RosterSourceRevision == 0 {
		return nil, ErrMemberAuthorityUnavailable
	}
	r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE
	r.Binding = &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Community{Community: &rolev1.CommunityMembershipBinding{OwnerGeneration: owner, MembershipGeneration: generation, MembershipEpisodeId: episode.String()}}}
	r.ValidityRemaining = durationpb.New(remaining)
	return r, nil
}
