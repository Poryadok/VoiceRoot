package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MemberCommunicationStateRow is a single serialized Space authority snapshot.
// A nil timeout means CLEAR at EvaluatedAt, never a consumer-local expiry grant.
type MemberCommunicationStateRow struct {
	SpaceID       uuid.UUID
	ProfileID     uuid.UUID
	AccessEpoch   uint64
	EvaluatedAt   time.Time
	TimedOutUntil *time.Time
}

// ResolveMemberCommunicationState evaluates only an eligible live exact member.
// The shared ownership lock also serializes timeout mutation and its epoch
// trigger. No application clock or membership pagination is involved.
func (s *SpaceStore) ResolveMemberCommunicationState(ctx context.Context, spaceID, profileID uuid.UUID) (*MemberCommunicationStateRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) (*MemberCommunicationStateRow, error) {
			return scoped.ResolveMemberCommunicationState(ctx, spaceID, profileID)
		})
	}
	var out MemberCommunicationStateRow
	var epoch int64
	err := s.db().QueryRow(ctx, `
WITH evaluation AS MATERIALIZED (SELECT clock_timestamp() AS evaluated_at)
SELECT sp.id, $2::uuid, vae.access_epoch, evaluation.evaluated_at,
       CASE WHEN timeout.timed_out_until > evaluation.evaluated_at THEN timeout.timed_out_until END
FROM spaces sp
JOIN space_voice_access_epochs vae ON vae.space_id=sp.id
CROSS JOIN evaluation
LEFT JOIN space_member_timeouts timeout ON timeout.space_id=sp.id AND timeout.profile_id=$2
WHERE sp.id=$1 AND EXISTS (
  SELECT 1 FROM space_members sm WHERE sm.space_id=sp.id AND sm.profile_id=$2
  UNION ALL
  SELECT 1 FROM community_roster_members crm JOIN community_owner_authority coa
    ON coa.space_id=crm.space_id AND coa.owner_generation=crm.owner_generation AND coa.status='active'
  WHERE crm.space_id=sp.id AND crm.profile_id=$2 AND crm.revoked_at IS NULL
    AND crm.lease_expires_at>evaluation.evaluated_at AND coa.roster_lease_expires_at>evaluation.evaluated_at
)
`, spaceID, profileID).Scan(&out.SpaceID, &out.ProfileID, &epoch, &out.EvaluatedAt, &out.TimedOutUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	if epoch <= 0 {
		return nil, errors.New("space store: invalid communication access epoch")
	}
	out.AccessEpoch = uint64(epoch)
	return &out, nil
}
