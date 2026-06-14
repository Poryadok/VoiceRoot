package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const freeSpaceMemberCap = 50

var ErrMemberCapReached = errors.New("space member cap reached")

// HasActiveSpacePro reports whether the space has active or grace_period Space Pro.
func (s *SpaceStore) HasActiveSpacePro(ctx context.Context, spaceID uuid.UUID) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1 FROM space_subscriptions
	WHERE space_id = $1 AND status IN ('active', 'grace_period')
)`, spaceID).Scan(&exists)
	return exists, err
}

// UpsertSpaceSubscription seeds entitlement cache (tests / sync from Subscription service).
func (s *SpaceStore) UpsertSpaceSubscription(ctx context.Context, spaceID, purchaserAccountID uuid.UUID, status string) error {
	now := time.Now().UTC()
	_, err := s.Pool.Exec(ctx, `DELETE FROM space_subscriptions WHERE space_id = $1`, spaceID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO space_subscriptions (
	space_id, purchaser_account_id, plan, status, provider, provider_subscription_id,
	current_period_start, current_period_end
) VALUES ($1, $2, 'space_pro', $3, 'paddle', $4, $5, $6)`,
		spaceID, purchaserAccountID, status, "test_"+spaceID.String(), now, now.AddDate(0, 1, 0))
	return err
}

// MemberCap returns max members for a space based on entitlement.
func (s *SpaceStore) MemberCap(ctx context.Context, spaceID uuid.UUID) (int32, error) {
	hasPro, err := s.HasActiveSpacePro(ctx, spaceID)
	if err != nil {
		return 0, err
	}
	if hasPro {
		return 5000, nil
	}
	return freeSpaceMemberCap, nil
}
