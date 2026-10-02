package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

func TestPublicLifecycleReadRequiresVerifiedGatewayContext(t *testing.T) {
	service := &PublicLifecycleSpace{SpaceGRPC: &SpaceGRPC{Store: &store.SpaceStore{}}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, uuid.NewString()))
	_, err := service.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: uuid.NewString()})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestPublicLifecycleReadLiveACLAndFrozenMinimalOwnerProjection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test-owned PostgreSQL")
	}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	dir := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			require.NoError(t, err)
			_, err = pool.Exec(ctx, string(raw))
			require.NoError(t, err, entry.Name())
		}
	}
	st := &store.SpaceStore{Pool: pool}
	owner, member, outsider, account := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	row, err := st.CreateSpace(ctx, owner, "owner recovery", "private description", "private")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, row.ID, member)
	require.NoError(t, err)
	service := &PublicLifecycleSpace{SpaceGRPC: &SpaceGRPC{Store: st}}
	actorContext := func(actor uuid.UUID) context.Context {
		md := metadata.Pairs(authctx.HeaderUserID, account.String(), authctx.HeaderProfileID, actor.String(), authctx.HeaderSessionEpoch, "7")
		return principal.WithVerified(metadata.NewIncomingContext(ctx, md), principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "space", RPC: spacev1.SpaceService_GetSpace_FullMethodName, AccountID: account.String(), ProfileID: actor.String(), SessionEpoch: 7})
	}
	req := &spacev1.GetSpaceRequest{SpaceId: row.ID.String()}
	for _, actor := range []uuid.UUID{owner, member} {
		resp, err := service.GetSpace(actorContext(actor), req)
		require.NoError(t, err)
		require.Equal(t, "private description", resp.Space.Description)
	}
	_, err = service.GetSpace(actorContext(outsider), req)
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = st.ReserveLifecycleSchedule(ctx, account, owner, 7, &spacev1.DeleteSpaceRequest{SpaceId: row.ID.String(), OperationId: uuid.NewString(), ConfirmationName: row.Name, Proof: "test-owned opaque proof"})
	require.NoError(t, err)
	resp, err := service.GetSpace(actorContext(owner), req)
	require.NoError(t, err)
	require.True(t, proto.Equal(&spacev1.Space{Id: row.ID.String(), Name: row.Name}, resp.Space), "frozen owner receives only the minimal pending projection")
	for _, actor := range []uuid.UUID{member, outsider} {
		_, err = service.GetSpace(actorContext(actor), req)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	_, err = service.SpaceGRPC.GetSpace(actorContext(owner), req)
	require.Error(t, err, "ordinary listener cannot disclose recovery projection")
}
