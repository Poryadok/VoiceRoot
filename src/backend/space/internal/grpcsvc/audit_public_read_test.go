package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

func TestPublicAuditReadRejectsRawActor(t *testing.T) {
	service := &PublicLifecycleSpace{SpaceGRPC: &SpaceGRPC{Store: &store.SpaceStore{}}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, uuid.NewString()))
	_, err := service.GetAuditLog(ctx, &spacev1.GetAuditLogRequest{SpaceId: uuid.NewString()})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestPublicAuditReadFiltersACLAndFrozenDisclosure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
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
	row, err := st.CreateSpace(ctx, owner, "audit public", "private", "private")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, row.ID, member)
	require.NoError(t, err)
	roles := &auditPermissionRoleStub{allowed: true}
	service := &PublicLifecycleSpace{SpaceGRPC: &SpaceGRPC{Store: st, Roles: roles, AuditCursorKey: []byte(strings.Repeat("k", 32))}}
	auditID := uuid.New()
	insertSpaceAuditEntry(t, ctx, pool, row.ID, auditID, owner, "member_banned", "account", `{"reason":"fixture ban"}`, time.Now().UTC())
	req := &spacev1.GetAuditLogRequest{SpaceId: row.ID.String(), Action: proto.String("member_banned")}
	actorContext := func(actor uuid.UUID) context.Context { return boundAuditFixture(ctx, account, actor, req) }
	resp, err := service.GetAuditLog(actorContext(member), req)
	require.NoError(t, err)
	require.Len(t, resp.GetAuditLogList().GetEntries(), 1)
	require.Equal(t, auditID.String(), resp.GetAuditLogList().Entries[0].Id)
	_, err = service.GetAuditLog(actorContext(outsider), req)
	require.Equal(t, codes.NotFound, status.Code(err))
	roles.allowed = false
	_, err = service.GetAuditLog(actorContext(member), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	roles.allowed = true
	_, err = st.ReserveLifecycleSchedule(ctx, account, owner, 7, &spacev1.DeleteSpaceRequest{SpaceId: row.ID.String(), OperationId: uuid.NewString(), ConfirmationName: row.Name, Proof: "test-owned-proof"})
	require.NoError(t, err)
	_, err = service.GetAuditLog(actorContext(owner), req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	for _, actor := range []uuid.UUID{member, outsider} {
		resp, err = service.GetAuditLog(actorContext(actor), req)
		require.Nil(t, resp)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
}
