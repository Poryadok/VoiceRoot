package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
		spaceID, purchaserAccountID, status, "sync_"+spaceID.String(), now, now.AddDate(0, 1, 0))
	return err
}

// SyncSpaceProSubscription upserts entitlement cache from Subscription service webhook path.
func (s *SpaceStore) SyncSpaceProSubscription(ctx context.Context, spaceID, purchaserAccountID uuid.UUID, status string) error {
	status = strings.TrimSpace(status)
	switch status {
	case "active", "grace_period":
		return s.UpsertSpaceSubscription(ctx, spaceID, purchaserAccountID, status)
	case "cancelled", "expired":
		_, err := s.Pool.Exec(ctx, `DELETE FROM space_subscriptions WHERE space_id = $1`, spaceID)
		return err
	default:
		return fmt.Errorf("unsupported space subscription status %q", status)
	}
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

// FinalizeSpacePro marks cached Space Pro entitlement cancelled for a space.
func (s *SpaceStore) FinalizeSpacePro(ctx context.Context, spaceID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
UPDATE space_subscriptions
SET status = 'cancelled', updated_at = now()
WHERE space_id = $1`, spaceID)
	return err
}
