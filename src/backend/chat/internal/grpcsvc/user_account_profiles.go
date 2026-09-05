package grpcsvc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	userv1 "voice.app/voice/user/v1"
)

type userAccountProfilesClient interface {
	ListProfileIDsForAccount(context.Context, *userv1.ListProfileIDsForAccountRequest, ...grpc.CallOption) (*userv1.ListProfileIDsForAccountResponse, error)
}

// UserGRPCAccountProfiles resolves all User-owned profile IDs for an Auth account.
// The account-delete consumer must fail closed when User returns malformed data.
type UserGRPCAccountProfiles struct {
	Client userAccountProfilesClient
}

// NewUserGRPCAccountProfiles creates the Chat S2S adapter for User Service.
func NewUserGRPCAccountProfiles(client userv1.UserServiceClient) *UserGRPCAccountProfiles {
	return &UserGRPCAccountProfiles{Client: client}
}

// ListProfileIDsForAccount returns only valid, non-nil UUID profile IDs.
func (u *UserGRPCAccountProfiles) ListProfileIDsForAccount(ctx context.Context, accountID string) ([]uuid.UUID, error) {
	parsedAccountID, err := uuid.Parse(accountID)
	if err != nil || parsedAccountID == uuid.Nil {
		return nil, fmt.Errorf("invalid account_id")
	}
	if u == nil || u.Client == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	resp, err := u.Client.ListProfileIDsForAccount(privacyS2SContext(ctx), &userv1.ListProfileIDsForAccountRequest{
		AccountId: parsedAccountID.String(),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("user service returned empty profile response")
	}
	profileIDs := make([]uuid.UUID, 0, len(resp.GetProfileIds()))
	for _, rawProfileID := range resp.GetProfileIds() {
		profileID, err := uuid.Parse(rawProfileID)
		if err != nil || profileID == uuid.Nil {
			return nil, fmt.Errorf("user service returned invalid profile_id")
		}
		profileIDs = append(profileIDs, profileID)
	}
	return profileIDs, nil
}
