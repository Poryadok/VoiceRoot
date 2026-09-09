package grpcsvc

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"
	"voice/backend/space/internal/store"

	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

type auditPermissionRoleStub struct {
	rolev1.RoleServiceClient
	allowed        bool
	permissionErr  error
	lastSpaceID    string
	lastProfileID  string
	lastPermission string
}

func (s *auditPermissionRoleStub) BootstrapSpaceRoles(context.Context, *rolev1.BootstrapSpaceRolesRequest, ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func (s *auditPermissionRoleStub) CheckPermission(_ context.Context, req *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	s.lastSpaceID = req.GetSpaceId()
	s.lastProfileID = req.GetProfileId()
	s.lastPermission = req.GetPermissionName()
	if s.permissionErr != nil {
		return nil, s.permissionErr
	}
	return &rolev1.CheckPermissionResponse{Allowed: s.allowed}, nil
}

func insertSpaceAuditEntry(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID, id, actor uuid.UUID, action, targetType, details string, createdAt time.Time) uuid.UUID {
	t.Helper()
	targetID := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO audit_log (id, space_id, actor_profile_id, action, target_type, target_id, details, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
`, id, spaceID, actor, action, targetType, targetID, details, createdAt)
	require.NoError(t, err)
	return targetID
}

func TestRevokeInvite_RecordsAuditEntry(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Invite audit"})
	require.NoError(t, err)
	spaceID := uuid.MustParse(created.GetSpace().GetId())
	invite, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	inviteID := uuid.MustParse(invite.GetInvite().GetId())

	_, err = client.RevokeInvite(ownerCtx, &spacev1.RevokeInviteRequest{InviteId: inviteID.String()})
	require.NoError(t, err)

	var actorID uuid.UUID
	var action, targetType, details string
	var targetID uuid.UUID
	err = pool.QueryRow(ctx, `
SELECT actor_profile_id, action, target_type, target_id, details::text
FROM audit_log
WHERE space_id = $1 AND action = 'invite_revoked'
`, spaceID).Scan(&actorID, &action, &targetType, &targetID, &details)
	require.NoError(t, err)
	require.Equal(t, owner, actorID)
	require.Equal(t, "invite_revoked", action)
	require.Equal(t, "invite", targetType)
	require.Equal(t, inviteID, targetID)
	require.JSONEq(t, `{}`, details)
}

func TestGetAuditLog_MapsFieldsOrdersAndScopesToRequestedSpace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	requested, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Audit requested"})
	require.NoError(t, err)
	other, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Audit other"})
	require.NoError(t, err)
	requestedID := uuid.MustParse(requested.GetSpace().GetId())
	otherID := uuid.MustParse(other.GetSpace().GetId())

	createdAt := time.Date(2026, 8, 31, 10, 11, 12, 345000000, time.UTC)
	entryID := uuid.MustParse("ffffffff-ffff-4fff-8fff-ffffffffffff")
	targetID := insertSpaceAuditEntry(t, ctx, pool, requestedID, entryID, owner, "member_banned", "account", `{"reason":"spam","days":7}`, createdAt)
	insertSpaceAuditEntry(t, ctx, pool, requestedID, uuid.MustParse("11111111-1111-4111-8111-111111111111"), owner, "older", "profile", `{}`, createdAt.Add(-time.Minute))
	insertSpaceAuditEntry(t, ctx, pool, otherID, uuid.New(), owner, "must_not_leak", "profile", `{}`, createdAt.Add(time.Hour))

	resp, err := client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{
		SpaceId: requestedID.String(),
		Page:    &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	entries := resp.GetAuditLogList().GetEntries()
	require.Len(t, entries, 2)
	got := entries[0]
	require.Equal(t, entryID.String(), got.GetId())
	require.Equal(t, requestedID.String(), got.GetSpaceId())
	require.Equal(t, owner.String(), got.GetActorProfileId())
	require.Equal(t, "member_banned", got.GetAction())
	require.Equal(t, "account", got.GetTargetType())
	require.Equal(t, targetID.String(), got.GetTargetId())
	require.JSONEq(t, `{"reason":"spam","days":7}`, got.GetDetailsJson())
	require.Equal(t, createdAt, got.GetCreatedAt().AsTime())
	require.Equal(t, "older", entries[1].GetAction())
	require.Empty(t, resp.GetAuditLogList().GetNextCursor())
}

func TestGetAuditLog_PageLimitsAndContinuation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Paged audit"})
	require.NoError(t, err)
	spaceID := uuid.MustParse(created.GetSpace().GetId())
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 101; i++ {
		insertSpaceAuditEntry(t, ctx, pool, spaceID, uuid.New(), owner, fmt.Sprintf("event_%03d", i), "space", `{}`, base.Add(time.Duration(i)*time.Second))
	}

	defaultPage, err := client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	require.Len(t, defaultPage.GetAuditLogList().GetEntries(), 50)
	require.NotEmpty(t, defaultPage.GetAuditLogList().GetNextCursor())

	maxPage, err := client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{
		SpaceId: spaceID.String(),
		Page:    &commonv1.CursorPageRequest{PageSize: 1000},
	})
	require.NoError(t, err)
	require.Len(t, maxPage.GetAuditLogList().GetEntries(), 100)
	require.NotEmpty(t, maxPage.GetAuditLogList().GetNextCursor())

	finalPage, err := client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{
		SpaceId: spaceID.String(),
		Page: &commonv1.CursorPageRequest{
			PageSize: 100,
			Cursor:   maxPage.GetAuditLogList().GetNextCursor(),
		},
	})
	require.NoError(t, err)
	require.Len(t, finalPage.GetAuditLogList().GetEntries(), 1)
	require.Empty(t, finalPage.GetAuditLogList().GetNextCursor())
}

func TestGetAuditLog_MalformedCursor_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Bad cursor"})
	require.NoError(t, err)

	validBase64 := func(json string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(json))
	}
	for _, tc := range []struct {
		name   string
		cursor string
	}{
		{name: "not base64", cursor: "not-an-audit-cursor"},
		{name: "missing timestamp", cursor: validBase64(`{"i":"11111111-1111-4111-8111-111111111111"}`)},
		{name: "missing id", cursor: validBase64(`{"t":"2026-09-03T10:00:00Z"}`)},
		{name: "zero timestamp", cursor: validBase64(`{"t":"0001-01-01T00:00:00Z","i":"11111111-1111-4111-8111-111111111111"}`)},
		{name: "zero uuid", cursor: validBase64(`{"t":"2026-09-03T10:00:00Z","i":"00000000-0000-0000-0000-000000000000"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{
				SpaceId: created.GetSpace().GetId(),
				Page:    &commonv1.CursorPageRequest{Cursor: tc.cursor, PageSize: 10},
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestGetAuditLog_OwnerFallbackOnlyWhenRolesUnwired(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Owner fallback"})
	require.NoError(t, err)

	_, err = client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{SpaceId: created.GetSpace().GetId()})
	require.NoError(t, err)

	outsiderCtx := withAccountProfileCtx(context.Background(), uuid.New(), uuid.New())
	_, err = client.GetAuditLog(outsiderCtx, &spacev1.GetAuditLogRequest{SpaceId: created.GetSpace().GetId()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetAuditLog_UsesViewAuditPermissionAndHonorsExplicitDeny(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	roles := &auditPermissionRoleStub{allowed: false}
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roles))
	t.Cleanup(cleanup)
	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Explicit deny"})
	require.NoError(t, err)

	_, err = client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{SpaceId: created.GetSpace().GetId()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, created.GetSpace().GetId(), roles.lastSpaceID)
	require.Equal(t, owner.String(), roles.lastProfileID)
	require.Equal(t, permissions.SpaceViewAuditLog, roles.lastPermission)
}

func TestGetAuditLog_ViewAuditPermissionAllowsNonOwner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	roles := &auditPermissionRoleStub{allowed: true}
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roles))
	t.Cleanup(cleanup)
	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Audit role allow"})
	require.NoError(t, err)

	nonOwnerProfile := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2)`, uuid.MustParse(created.GetSpace().GetId()), nonOwnerProfile)
	require.NoError(t, err)
	nonOwnerCtx := withAccountProfileCtx(context.Background(), uuid.New(), nonOwnerProfile)
	resp, err := client.GetAuditLog(nonOwnerCtx, &spacev1.GetAuditLogRequest{SpaceId: created.GetSpace().GetId()})
	require.NoError(t, err)
	require.NotNil(t, resp.GetAuditLogList())
	require.Equal(t, created.GetSpace().GetId(), roles.lastSpaceID)
	require.Equal(t, nonOwnerProfile.String(), roles.lastProfileID)
	require.Equal(t, permissions.SpaceViewAuditLog, roles.lastPermission)
}

func TestGetAuditLog_ViewAuditPermissionDeniesNonMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	roles := &auditPermissionRoleStub{allowed: true}
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roles))
	t.Cleanup(cleanup)
	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Audit non-member deny"})
	require.NoError(t, err)

	nonMemberCtx := withAccountProfileCtx(context.Background(), uuid.New(), uuid.New())
	_, err = client.GetAuditLog(nonMemberCtx, &spacev1.GetAuditLogRequest{SpaceId: created.GetSpace().GetId()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetAuditLog_RoleDependencyFailure_Unavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	roles := &auditPermissionRoleStub{allowed: true}
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roles))
	t.Cleanup(cleanup)
	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Role down"})
	require.NoError(t, err)

	for _, dependencyCode := range []codes.Code{
		codes.Unavailable,
		codes.DeadlineExceeded,
		codes.Unknown,
		codes.Internal,
	} {
		t.Run(dependencyCode.String(), func(t *testing.T) {
			roles.permissionErr = status.Error(dependencyCode, "role dependency failed")
			_, err := client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{SpaceId: created.GetSpace().GetId()})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, created.GetSpace().GetId(), roles.lastSpaceID)
			require.Equal(t, owner.String(), roles.lastProfileID)
			require.Equal(t, permissions.SpaceViewAuditLog, roles.lastPermission)
		})
	}
}

func TestRequireSpacePermission_RoleDependencyFailure_Unavailable(t *testing.T) {
	owner, _, ownerCtx := profileFixture(t)
	roles := &auditPermissionRoleStub{allowed: true}
	svc := &SpaceGRPC{Roles: roles}
	spaceID := uuid.New()

	for _, dependencyCode := range []codes.Code{
		codes.Unavailable,
		codes.DeadlineExceeded,
		codes.Unknown,
		codes.Internal,
	} {
		t.Run(dependencyCode.String(), func(t *testing.T) {
			roles.permissionErr = status.Error(dependencyCode, "role dependency failed")
			err := svc.requireSpacePermission(directServerContext(ownerCtx), spaceID, permissions.SpaceViewAuditLog)
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Equal(t, spaceID.String(), roles.lastSpaceID)
			require.Equal(t, owner.String(), roles.lastProfileID)
			require.Equal(t, permissions.SpaceViewAuditLog, roles.lastPermission)
		})
	}
}

func TestGetAuditLog_CrossInstance_WaitsForSpaceMutationSaga(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	owner := uuid.New()
	member := uuid.New()
	spaceID := createSpaceWithMembers(t, pool, owner, member)
	transferCtx, cancelTransfer := context.WithCancel(directServerContext(withAccountProfileCtx(ctx, uuid.New(), owner)))
	defer cancelTransfer()
	tracer := &ownershipUpdateContextTracer{
		auditInsertStarted:  make(chan struct{}),
		blockAuditInsert:    make(chan struct{}),
		cancelOnAuditInsert: cancelTransfer,
	}
	tracedPool := tracedSpacePool(t, pool, tracer)
	lockPoolA := independentSpacePool(t, pool, 1)
	lockPoolB := independentSpacePool(t, pool, 1)
	svcA := &SpaceGRPC{
		Store:          &store.SpaceStore{Pool: tracedPool},
		MutationLocker: store.NewSpaceMutationLocker(lockPoolA),
	}
	roles := &auditPermissionRoleStub{allowed: true}
	svcB := &SpaceGRPC{
		Store:          &store.SpaceStore{Pool: pool},
		Roles:          roles,
		MutationLocker: store.NewSpaceMutationLocker(lockPoolB),
	}

	transferDone := make(chan error, 1)
	go func() {
		_, err := svcA.TransferOwnership(transferCtx, &spacev1.TransferOwnershipRequest{
			SpaceId:           spaceID.String(),
			NewOwnerProfileId: member.String(),
		})
		transferDone <- err
	}()
	select {
	case <-tracer.auditInsertStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("first instance did not reach the blocked ownership audit write")
	}
	defer func() {
		select {
		case <-tracer.blockAuditInsert:
		default:
			close(tracer.blockAuditInsert)
		}
	}()

	memberCtx := directServerContext(withAccountProfileCtx(ctx, uuid.New(), member))
	readCtx, cancelRead := context.WithTimeout(memberCtx, 200*time.Millisecond)
	defer cancelRead()
	_, readErr := svcB.GetAuditLog(readCtx, &spacev1.GetAuditLogRequest{SpaceId: spaceID.String()})
	permissionBeforeRelease := roles.lastPermission

	close(tracer.blockAuditInsert)
	transferErr := requireTransferResult(t, transferDone)
	require.Equal(t, codes.Internal, status.Code(transferErr))
	resp, err := svcB.GetAuditLog(memberCtx, &spacev1.GetAuditLogRequest{SpaceId: spaceID.String()})
	require.NoError(t, err, "authorized member must read audit after the ownership saga releases the lease")
	require.NotNil(t, resp.GetAuditLogList())
	require.Equal(t, codes.DeadlineExceeded, status.Code(readErr), "audit reads must not observe the transfer's temporary owner window")
	require.Empty(t, permissionBeforeRelease, "permission evaluation must happen only after the mutation lease is acquired")
	require.Equal(t, permissions.SpaceViewAuditLog, roles.lastPermission)

	row, err := (&store.SpaceStore{Pool: pool}).GetSpace(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, owner, row.OwnerProfileID)
	requireOwnershipTransferAuditCount(t, pool, spaceID.String(), 0)
}
