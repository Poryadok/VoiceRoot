package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

// ReadScopedMemberSourceBindings is a local finite-source capture for an
// already-member journal. It requires the caller's fenced READ COMMITTED
// transaction. An older repeatable snapshot cannot be refreshed by a fence.
// Call again after the caller's final SQL writes and compare the exact set
// before commit. This captures source identity, never a portable eligibility
// allow, and does not replace verified actor/account/invite/target guards.
func (s *SpaceStore) ReadScopedMemberSourceBindings(ctx context.Context, space, profile uuid.UUID) ([]*rolev1.MembershipBinding, error) {
	if s == nil || s.Pool == nil || s.tx == nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil || profile == uuid.Nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var held bool
	if err := s.tx.QueryRow(ctx, `SELECT current_setting('transaction_isolation')='read committed'
	 AND EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND pid=pg_backend_pid()
	 AND granted AND mode='ExclusiveLock' AND objsubid=1
	 AND classid::bigint=(($1::bigint>>32)&4294967295) AND objid::bigint=($1::bigint&4294967295))`, spaceMutationAdvisoryKey(space)).Scan(&held); err != nil || !held {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	lifecycle, err := readMemberLifecycle(ctx, s.tx, space)
	if err != nil || lifecycle.GetState() != spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE {
		return nil, ErrMemberAuthorityUnavailable
	}
	ownership, err := readMemberOwnership(ctx, s.tx, space)
	if err != nil || ownership.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE {
		return nil, ErrMemberAuthorityUnavailable
	}
	revision, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var sampled, finished time.Time
	if err = s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	ordinary, err := readOrdinaryMemberAuthority(ctx, s.tx, space, profile)
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	community, err := readCommunityMemberAuthority(ctx, s.tx, space, profile, sampled)
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	last, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil || last != revision {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil || finished.Before(sampled) || !finished.Before(sampled.Add(5*time.Second)) {
		return nil, ErrMemberAuthorityUnavailable
	}
	var bindings []*rolev1.MembershipBinding
	if ordinary.GetState() == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
		if ordinary.PublishedBinding == nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		bindings = append(bindings, proto.Clone(ordinary.PublishedBinding).(*rolev1.MembershipBinding))
	}
	if community.GetState() == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
		if community.Binding == nil || community.GetValidityRemaining().AsDuration() <= finished.Sub(sampled) {
			return nil, ErrMemberAuthorityUnavailable
		}
		bindings = append(bindings, proto.Clone(community.Binding).(*rolev1.MembershipBinding))
	}
	if len(bindings) == 0 {
		return nil, pgx.ErrNoRows
	}
	return bindings, nil
}
