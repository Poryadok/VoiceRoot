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

const socialHeaderAccountType = "x-voice-account-type"

func withGuestProfileCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
	return metadata.AppendToOutgoingContext(ctx, socialHeaderAccountType, "guest")
}

// TestSendFriendInvitation_GuestCaller_PermissionDenied documents auth-and-contacts.md:
// guests cannot send friend invitations.
func TestSendFriendInvitation_GuestCaller_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	guestAccount := uuid.New()
	guestProfile := uuid.New()
	target := uuid.New()

	_, err := client.SendFriendInvitation(
		withGuestProfileCtx(ctx, guestAccount, guestProfile),
		&socialv1.SendFriendInvitationRequest{TargetProfileId: target.String()},
	)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
