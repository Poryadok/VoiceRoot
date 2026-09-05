package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/social/internal/authctx"

	socialv1 "voice.app/voice/social/v1"
)

type stubProfileAccounts map[uuid.UUID]uuid.UUID

func (m stubProfileAccounts) AccountIDByProfileID(_ context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if acct, ok := m[profileID]; ok {
		return acct, nil
	}
	return uuid.Nil, status.Error(codes.NotFound, "profile not found")
}

func withProfileAndAccountCtx(ctx context.Context, profileID, accountID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(ctx,
		authctx.HeaderProfileID, profileID.String(),
		authctx.HeaderUserID, accountID.String(),
	)
}

// TestSendFriendInvitation_BlockedAccounts_PermissionDenied documents DM parity: blocked accounts cannot send friend invites.
func TestSendFriendInvitation_BlockedAccounts_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	profA := uuid.New()
	profB := uuid.New()
	accA := uuid.New()
	accB := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.AccountProfiles = stubAccountProfiles{
			accA: {profA},
			accB: {profB},
		}
		s.ProfileAccounts = stubProfileAccounts{
			profA: accA,
			profB: accB,
		}
	})
	t.Cleanup(cleanup)

	_, err := client.BlockAccount(withAccountCtx(ctx, accA), &socialv1.BlockAccountRequest{BlockedAccountId: accB.String()})
	require.NoError(t, err)

	_, err = client.SendFriendInvitation(
		withProfileAndAccountCtx(ctx, profA, accA),
		&socialv1.SendFriendInvitationRequest{TargetProfileId: profB.String()},
	)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = client.SendFriendInvitation(
		withProfileAndAccountCtx(ctx, profB, accB),
		&socialv1.SendFriendInvitationRequest{TargetProfileId: profA.String()},
	)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
