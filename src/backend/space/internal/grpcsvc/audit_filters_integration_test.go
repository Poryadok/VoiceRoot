package grpcsvc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestGetAuditLog_FiltersCursorAndRevokedAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	owner, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	roles := &auditPermissionRoleStub{allowed: true}
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roles))
	t.Cleanup(cleanup)
	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Filtered audit"})
	require.NoError(t, err)
	spaceID := uuid.MustParse(created.GetSpace().GetId())
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	older, newer := uuid.New(), uuid.New()
	insertSpaceAuditEntry(t, ctx, pool, spaceID, older, owner, "member_banned", "account", `{}`, from)
	insertSpaceAuditEntry(t, ctx, pool, spaceID, newer, owner, "member_banned", "account", `{}`, from.Add(time.Minute))
	insertSpaceAuditEntry(t, ctx, pool, spaceID, uuid.New(), uuid.New(), "member_banned", "account", `{}`, from.Add(2*time.Minute))
	insertSpaceAuditEntry(t, ctx, pool, spaceID, uuid.New(), owner, "member_unbanned", "account", `{}`, from.Add(2*time.Minute))
	insertSpaceAuditEntry(t, ctx, pool, spaceID, uuid.New(), owner, "member_banned", "account", `{}`, from.Add(-time.Second))
	insertSpaceAuditEntry(t, ctx, pool, spaceID, uuid.New(), owner, "member_banned", "account", `{}`, to)
	actor, action := strings.ToUpper(owner.String()), "member_banned"
	request := &spacev1.GetAuditLogRequest{SpaceId: spaceID.String(), ActorProfileId: &actor, Action: &action,
		From: timestamppb.New(from), To: timestamppb.New(to), Page: &commonv1.CursorPageRequest{PageSize: 1}}
	first, err := client.GetAuditLog(ownerCtx, request)
	require.NoError(t, err)
	require.Len(t, first.GetAuditLogList().GetEntries(), 1)
	require.Equal(t, newer.String(), first.GetAuditLogList().GetEntries()[0].GetId())
	cursor := first.GetAuditLogList().GetNextCursor()
	require.NotEmpty(t, cursor)
	request.Page.Cursor = cursor
	lowercase := owner.String()
	request.ActorProfileId = &lowercase
	second, err := client.GetAuditLog(ownerCtx, request)
	require.NoError(t, err, "UUID spelling must normalize to the same filter")
	require.Len(t, second.GetAuditLogList().GetEntries(), 1)
	require.Equal(t, older.String(), second.GetAuditLogList().GetEntries()[0].GetId())
	require.Empty(t, second.GetAuditLogList().GetNextCursor())

	for _, change := range []func(*spacev1.GetAuditLogRequest){
		func(r *spacev1.GetAuditLogRequest) { r.ActorProfileId = nil },
		func(r *spacev1.GetAuditLogRequest) { r.Action = nil },
		func(r *spacev1.GetAuditLogRequest) { r.From = timestamppb.New(from.Add(-time.Second)) },
		func(r *spacev1.GetAuditLogRequest) { r.To = timestamppb.New(to.Add(time.Second)) },
		func(r *spacev1.GetAuditLogRequest) { r.Page.PageSize = 2 },
	} {
		changed := proto.Clone(request).(*spacev1.GetAuditLogRequest)
		change(changed)
		_, err := client.GetAuditLog(ownerCtx, changed)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	for _, invalid := range []*spacev1.GetAuditLogRequest{
		{ActorProfileId: proto.String("")}, {Action: proto.String("")},
		{From: &timestamppb.Timestamp{Nanos: -1}}, {To: &timestamppb.Timestamp{Seconds: 253402300800}},
		{From: timestamppb.New(to), To: timestamppb.New(from)},
	} {
		invalid.SpaceId = spaceID.String()
		_, err := client.GetAuditLog(ownerCtx, invalid)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	unknown := &spacev1.GetAuditLogRequest{SpaceId: spaceID.String(), Action: proto.String("unknown_action")}
	empty, err := client.GetAuditLog(ownerCtx, unknown)
	require.NoError(t, err)
	require.Empty(t, empty.GetAuditLogList().GetEntries())
	roles.allowed = false
	_, err = client.GetAuditLog(ownerCtx, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "continuation must recheck ACL before cursor access")
}
