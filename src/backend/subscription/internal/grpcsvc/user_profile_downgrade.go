package grpcsvc

import (
	"context"

	"github.com/google/uuid"

	userv1 "voice.app/voice/user/v1"
)

// UserGRPCProfileDowngrade delegates subscription downgrade profile selection to User service.
type UserGRPCProfileDowngrade struct {
	Client userv1.UserServiceClient
}

// ApplyDowngradeProfiles calls User.ApplyDowngradeProfiles over gRPC.
func (c *UserGRPCProfileDowngrade) ApplyDowngradeProfiles(ctx context.Context, accountID uuid.UUID, keptProfileIDs []uuid.UUID) error {
	if c == nil || c.Client == nil {
		return nil
	}
	kept := make([]string, 0, len(keptProfileIDs))
	for _, id := range keptProfileIDs {
		kept = append(kept, id.String())
	}
	_, err := c.Client.ApplyDowngradeProfiles(ctx, &userv1.ApplyDowngradeProfilesRequest{
		AccountId:      accountID.String(),
		KeptProfileIds: kept,
	})
	return err
}
