package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	socialv1 "voice.app/voice/social/v1"
)

// TestSendFriendInvitation_BlockCheck_ProfileAccountsNil documents OPERATIONS.md /
// PLAN fail-closed when User S2S (ProfileAccounts) is unwired; Blocks may still be set.
func TestSendFriendInvitation_BlockCheck_ProfileAccountsNil(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	caller := uuid.New()
	target := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.ProfileAccounts = nil
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(
		withITProfileCtx(ctx, caller),
		&socialv1.SendFriendInvitationRequest{TargetProfileId: target.String()},
	)
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestSendFriendInvitation_BlockCheck_BlocksNil documents fail-closed when Blocks is
// nil while ProfileAccounts remains wired.
func TestSendFriendInvitation_BlockCheck_BlocksNil(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	caller := uuid.New()
	target := uuid.New()

	client, cleanup := startSocialGRPCTestServer(t, pool, func(s *SocialGRPC) {
		s.Blocks = nil
		// ProfileAccounts stays at harness default (deterministicProfileAccounts).
	})
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(
		withITProfileCtx(ctx, caller),
		&socialv1.SendFriendInvitationRequest{TargetProfileId: target.String()},
	)
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestSendFriendInvitation_BlockCheck_MissingCallerAccount documents that wired
// block deps must not skip the check when x-voice-user-id / AccountID is absent —
// Unauthenticated, not success.
func TestSendFriendInvitation_BlockCheck_MissingCallerAccount(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	caller := uuid.New()
	target := uuid.New()

	// Default harness wires ProfileAccounts + Blocks; only profile header (no account).
	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.SendFriendInvitation(
		withProfileCtx(ctx, caller),
		&socialv1.SendFriendInvitationRequest{TargetProfileId: target.String()},
	)
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
