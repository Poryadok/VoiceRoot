package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// refreshEffectiveMemberCount runs after a source mutation under the caller's
// Space fence. A profile with two independent sources occupies one slot; losing
// one source never frees capacity while the other remains eligible.
func (s *SpaceStore) refreshEffectiveMemberCount(ctx context.Context, spaceID uuid.UUID) error {
	if s == nil || s.tx == nil {
		return errors.New("member count requires scoped Space transaction")
	}
	_, err := s.db().Exec(ctx, `UPDATE spaces SET member_count=(SELECT count(*)::int FROM (
	 SELECT profile_id FROM space_members WHERE space_id=$1
	 UNION SELECT m.profile_id FROM community_roster_members m JOIN community_owner_authority a
	 ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation AND a.status='active'
	 WHERE m.space_id=$1 AND m.revoked_at IS NULL AND m.lease_expires_at>clock_timestamp()
	 AND a.roster_lease_expires_at>clock_timestamp()
	) effective_members),updated_at=clock_timestamp() WHERE id=$1`, spaceID)
	return err
}
