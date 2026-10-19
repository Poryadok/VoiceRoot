package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
)

type legacySpaceFixtureClient struct {
	spacev1.SpaceServiceClient
	fixture *SpaceGRPC
}

// seedLegacySpaceForTest explicitly seeds pre-native fixture data for unrelated
// RPC tests. It never overrides CreateSpace or supplies a production bypass.
// Tests of creation/authentication must exercise the actual signed receiver.
// Store's native schema guards still reject this seed after migration33.
func seedLegacySpaceForTest(t *testing.T, client spacev1.SpaceServiceClient, ctx context.Context, request *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
	t.Helper()
	fixture, ok := client.(*legacySpaceFixtureClient)
	require.True(t, ok, "explicit legacy seed requires a storage fixture client")
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	ctx = metadata.NewIncomingContext(ctx, md.Copy())
	account, ok := authctx.AccountID(ctx)
	require.True(t, ok)
	profile, ok := authctx.ProfileID(ctx)
	require.True(t, ok)
	visibility := strings.TrimSpace(request.GetVisibility())
	if visibility == "" {
		visibility = "private"
	}
	svc := fixture.fixture
	// The old helper stops at21, but ordinary mutation guards now require the
	// permanent P3 fence introduced by28. Apply its canonical DDL, never create
	// a reduced substitute or disable the fail-closed production check.
	var fenceReady bool
	require.NoError(t, svc.Store.Pool.QueryRow(ctx, `SELECT to_regclass('public.space_permanent_purge_fences') IS NOT NULL`).Scan(&fenceReady))
	if !fenceReady {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000028_chat_creation_outcomes.up.sql"))
		require.NoError(t, err)
		_, err = svc.Store.Pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	row, err := svc.Store.CreateSpace(ctx, profile, strings.TrimSpace(request.GetName()), request.GetDescription(), visibility)
	if err != nil {
		return nil, err
	}
	if svc.SeedSpaceProActive {
		if err := svc.Store.UpsertSpaceSubscription(ctx, row.ID, account, "active"); err != nil {
			return nil, err
		}
	}
	if err := svc.bootstrapSpaceRoles(ctx, row.ID, row.OwnerProfileID); err != nil {
		return nil, err
	}
	if svc.SpaceEvents != nil {
		_ = svc.SpaceEvents.PublishSpaceCreated(ctx, row.ID.String(), row.OwnerProfileID.String())
	}
	return &spacev1.CreateSpaceResponse{Space: spaceRowToProto(row)}, nil
}
