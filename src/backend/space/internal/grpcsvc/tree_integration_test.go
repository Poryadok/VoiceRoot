package grpcsvc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestListSpaceTree_Member(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Tree"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	cat, err := client.CreateCategory(ctx, &spacev1.CreateCategoryRequest{
		SpaceId: spaceID, Name: "General", SortOrder: 0,
	})
	require.NoError(t, err)

	vr, err := client.CreateVoiceRoom(ctx, &spacev1.CreateVoiceRoomRequest{
		SpaceId: spaceID, Name: "Lobby",
	})
	require.NoError(t, err)

	chatID := uuid.New().String()
	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	_, err = client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId:    spaceID,
		Kind:       "text_chat",
		CategoryId: ptr(cat.GetCategory().GetId()),
		LinkedChat: &chatv1.ChatRef{Id: chatID, Type: &chatType},
	})
	require.NoError(t, err)

	tree, err := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Len(t, tree.GetCategories(), 1)
	require.Len(t, tree.GetNodes(), 2)
	require.Len(t, tree.GetVoiceRooms(), 1)
	require.Equal(t, vr.GetVoiceRoom().GetId(), tree.GetVoiceRooms()[0].GetId())
}

func TestListSpaceTree_NonMemberDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	_, _, otherCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Private"})
	require.NoError(t, err)

	_, err = client.ListSpaceTree(otherCtx, &spacev1.ListSpaceTreeRequest{
		SpaceId: created.GetSpace().GetId(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestReorderSpaceTree_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Reorder"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	vr, err := client.CreateVoiceRoom(ctx, &spacev1.CreateVoiceRoomRequest{SpaceId: spaceID, Name: "V"})
	require.NoError(t, err)

	chatA, chatB := uuid.New().String(), uuid.New().String()
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	nodeA, err := client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID, Kind: "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: chatA, Type: &chatType},
	})
	require.NoError(t, err)
	nodeB, err := client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID, Kind: "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: chatB, Type: &chatType},
	})
	require.NoError(t, err)

	voiceNodes, _ := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	var voiceNodeID string
	for _, n := range voiceNodes.GetNodes() {
		if n.GetKind() == "voice_room" {
			voiceNodeID = n.GetId()
		}
	}
	require.NotEmpty(t, voiceNodeID)

	_, err = client.ReorderSpaceTree(ctx, &spacev1.ReorderSpaceTreeRequest{
		SpaceId:        spaceID,
		OrderedNodeIds: []string{voiceNodeID, nodeB.GetSpaceTreeNode().GetId(), nodeA.GetSpaceTreeNode().GetId()},
	})
	require.NoError(t, err)

	tree, err := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, voiceNodeID, tree.GetNodes()[0].GetId())
	require.Equal(t, vr.GetVoiceRoom().GetId(), tree.GetNodes()[0].GetVoiceRoomId())
}

func TestPinTreeNode_UnpinTreeNode_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "PinSpace"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	chatA, chatB := uuid.New().String(), uuid.New().String()
	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	nodeA, err := client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID, Kind: "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: chatA, Type: &chatType},
	})
	require.NoError(t, err)
	_, err = client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID, Kind: "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: chatB, Type: &chatType},
	})
	require.NoError(t, err)

	pinned, err := client.PinTreeNode(ctx, &spacev1.PinTreeNodeRequest{
		SpaceId: spaceID,
		NodeId:  nodeA.GetSpaceTreeNode().GetId(),
	})
	require.NoError(t, err)
	require.True(t, pinned.GetSpaceTreeNode().GetIsPinned())

	tree, err := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.True(t, tree.GetNodes()[0].GetIsPinned())

	unpinned, err := client.UnpinTreeNode(ctx, &spacev1.UnpinTreeNodeRequest{
		SpaceId: spaceID,
		NodeId:  nodeA.GetSpaceTreeNode().GetId(),
	})
	require.NoError(t, err)
	require.False(t, unpinned.GetSpaceTreeNode().GetIsPinned())
}

func TestCreateCategory_NonOwnerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	_, _, otherCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Owned"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	_, err = client.CreateCategory(otherCtx, &spacev1.CreateCategoryRequest{
		SpaceId: spaceID, Name: "Nope",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestSpaceTree_OwnerCanUpdateAndDeleteCategory(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Category lifecycle"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	category, err := client.CreateCategory(ctx, &spacev1.CreateCategoryRequest{
		SpaceId:   spaceID,
		Name:      "Before",
		SortOrder: 2,
	})
	require.NoError(t, err)

	updated, err := client.UpdateCategory(ctx, &spacev1.UpdateCategoryRequest{
		CategoryId: category.GetCategory().GetId(),
		Name:       ptr("After"),
		SortOrder:  ptrInt32(7),
	})
	require.NoError(t, err)
	require.Equal(t, "After", updated.GetCategory().GetName())
	require.EqualValues(t, 7, updated.GetCategory().GetSortOrder())

	_, err = client.DeleteCategory(ctx, &spacev1.DeleteCategoryRequest{CategoryId: category.GetCategory().GetId()})
	require.NoError(t, err)

	tree, err := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Empty(t, tree.GetCategories())
}

func TestSpaceTree_OwnerCanUpdateAndDeleteVoiceRoomWithTreeNode(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Voice room lifecycle"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	room, err := client.CreateVoiceRoom(ctx, &spacev1.CreateVoiceRoomRequest{SpaceId: spaceID, Name: "Before"})
	require.NoError(t, err)
	roomID := room.GetVoiceRoom().GetId()

	updated, err := client.UpdateVoiceRoom(ctx, &spacev1.UpdateVoiceRoomRequest{
		VoiceRoomId: roomID,
		Name:        ptr("After"),
	})
	require.NoError(t, err)
	require.Equal(t, "After", updated.GetVoiceRoom().GetName())

	_, err = client.DeleteVoiceRoom(ctx, &spacev1.DeleteVoiceRoomRequest{VoiceRoomId: roomID})
	require.NoError(t, err)

	tree, err := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Empty(t, tree.GetVoiceRooms())
	require.Empty(t, tree.GetNodes(), "deleting a voice room must cascade to its tree node")
}

func TestSpaceTree_OwnerCanRemoveTextNode(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Text node lifecycle"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	node, err := client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId:    spaceID,
		Kind:       "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: uuid.New().String(), Type: &chatType},
	})
	require.NoError(t, err)

	_, err = client.RemoveTreeNode(ctx, &spacev1.RemoveTreeNodeRequest{
		SpaceId: spaceID,
		NodeId:  node.GetSpaceTreeNode().GetId(),
	})
	require.NoError(t, err)

	tree, err := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Empty(t, tree.GetNodes())
}

func TestSpaceTree_AuditWritesAreAtomicAndVisible(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ownerCtx := profileFixture(t)
	_, _, otherCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	applySpaceAuditLedgerMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Tree audit"})
	require.NoError(t, err)
	spaceID := uuid.MustParse(created.GetSpace().GetId())
	category, err := client.CreateCategory(ownerCtx, &spacev1.CreateCategoryRequest{
		SpaceId: spaceID.String(), Name: "General", SortOrder: 0,
	})
	require.NoError(t, err)
	categoryID := category.GetCategory().GetId()
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	chatA, chatB := uuid.New(), uuid.New()

	first, err := client.UpsertTreeNode(ownerCtx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID.String(), Kind: "text_chat", CategoryId: ptr(categoryID),
		LinkedChat: &chatv1.ChatRef{Id: chatA.String(), Type: &chatType}, SortOrder: ptrInt32(4),
	})
	require.NoError(t, err)
	firstID := uuid.MustParse(first.GetSpaceTreeNode().GetId())
	requireTreeAuditEntry(t, ctx, pool, spaceID, owner, "tree_node_upserted", "tree_node", firstID,
		map[string]any{"kind": "text_chat", "chat_id": chatA.String(), "category_id": categoryID,
			"sort_order": float64(4), "is_pinned": false})

	var upsertCountBefore, outboxCountBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='tree_node_upserted'`, spaceID).Scan(&upsertCountBefore))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox WHERE space_id=$1`, spaceID).Scan(&outboxCountBefore))
	_, err = client.UpsertTreeNode(ownerCtx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID.String(), NodeId: ptr(firstID.String()), Kind: "text_chat",
		CategoryId: ptr(categoryID), LinkedChat: &chatv1.ChatRef{Id: chatA.String(), Type: &chatType}, SortOrder: ptrInt32(4),
	})
	require.NoError(t, err)
	var upsertCountAfter, outboxCountAfter int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='tree_node_upserted'`, spaceID).Scan(&upsertCountAfter))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox WHERE space_id=$1`, spaceID).Scan(&outboxCountAfter))
	require.Equal(t, upsertCountBefore, upsertCountAfter, "same business values must not create another audit effect")
	require.Equal(t, outboxCountBefore, outboxCountAfter)

	second, err := client.UpsertTreeNode(ownerCtx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID.String(), Kind: "text_chat", CategoryId: ptr(categoryID),
		LinkedChat: &chatv1.ChatRef{Id: chatB.String(), Type: &chatType}, SortOrder: ptrInt32(5),
	})
	require.NoError(t, err)
	secondID := uuid.MustParse(second.GetSpaceTreeNode().GetId())
	_, err = client.ReorderSpaceTree(ownerCtx, &spacev1.ReorderSpaceTreeRequest{
		SpaceId: spaceID.String(), OrderedNodeIds: []string{secondID.String(), firstID.String()},
	})
	require.NoError(t, err)
	requireTreeAuditEntry(t, ctx, pool, spaceID, owner, "tree_reordered", "space", spaceID,
		map[string]any{"count": float64(2)})

	var reorderCountBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='tree_reordered'`, spaceID).Scan(&reorderCountBefore))
	_, err = client.ReorderSpaceTree(ownerCtx, &spacev1.ReorderSpaceTreeRequest{SpaceId: spaceID.String()})
	require.NoError(t, err, "empty reorder remains an accepted no-op")
	_, err = client.RemoveTreeNode(ownerCtx, &spacev1.RemoveTreeNodeRequest{SpaceId: spaceID.String(), NodeId: uuid.NewString()})
	require.Equal(t, codes.NotFound, status.Code(err), "missing-node removal stays rejected")
	_, err = client.UpsertTreeNode(otherCtx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID.String(), Kind: "text_chat", LinkedChat: &chatv1.ChatRef{Id: uuid.NewString(), Type: &chatType},
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "tree audit must not bypass existing authorization")
	var reorderCountAfter, allAuditCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='tree_reordered'`, spaceID).Scan(&reorderCountAfter))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action LIKE 'tree_%'`, spaceID).Scan(&allAuditCount))
	require.Equal(t, reorderCountBefore, reorderCountAfter)
	require.Equal(t, 3, allAuditCount, "two node creates and one reorder; no-op/rejected requests add no effects")

	_, err = client.RemoveTreeNode(ownerCtx, &spacev1.RemoveTreeNodeRequest{SpaceId: spaceID.String(), NodeId: firstID.String()})
	require.NoError(t, err)
	requireTreeAuditEntry(t, ctx, pool, spaceID, owner, "tree_node_removed", "tree_node", firstID, map[string]any{})

	page, err := client.GetAuditLog(ownerCtx, &spacev1.GetAuditLogRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, entry := range page.GetAuditLogList().GetEntries() {
		seen[entry.GetAction()] = true
	}
	require.True(t, seen["tree_node_upserted"])
	require.True(t, seen["tree_node_removed"])
	require.True(t, seen["tree_reordered"])
}

func TestSpaceTree_AuditInsertFailureRollsBackUpsert(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	applySpaceAuditLedgerMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Audit rollback"})
	require.NoError(t, err)
	spaceID := uuid.MustParse(created.GetSpace().GetId())

	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_tree_audit_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action LIKE 'tree_%' THEN RAISE EXCEPTION 'injected tree audit failure'; END IF; RETURN NEW; END $$;
CREATE TRIGGER fail_tree_audit_insert BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION fail_tree_audit_insert()`)
	require.NoError(t, err)
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.UpsertTreeNode(ownerCtx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID.String(), Kind: "text_chat", LinkedChat: &chatv1.ChatRef{Id: uuid.NewString(), Type: &chatType},
	})
	require.Error(t, err, "audit insertion failure must fail the tree mutation")

	tree, err := client.ListSpaceTree(ownerCtx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID.String()})
	require.NoError(t, err)
	require.Empty(t, tree.GetNodes(), "tree mutation must roll back with its audit failure")
	var audits, outbox int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action LIKE 'tree_%'`, spaceID).Scan(&audits))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox WHERE space_id=$1`, spaceID).Scan(&outbox))
	require.Zero(t, audits)
	require.Zero(t, outbox)
}

func applySpaceAuditLedgerMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	auditMigration, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000015_audit_ledger.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(auditMigration))
	require.NoError(t, err)
}

func requireTreeAuditEntry(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID, actorID uuid.UUID, action, targetType string, targetID uuid.UUID, wantDetails map[string]any) {
	t.Helper()
	var auditID, gotSpaceID, gotActorID, gotTargetID uuid.UUID
	var gotAction, gotTargetType, rawDetails string
	err := pool.QueryRow(ctx, `SELECT id,space_id,actor_profile_id,action,target_type,target_id,details::text
FROM audit_log WHERE space_id=$1 AND action=$2 AND target_id=$3 ORDER BY created_at DESC,id DESC LIMIT 1`,
		spaceID, action, targetID).Scan(&auditID, &gotSpaceID, &gotActorID, &gotAction, &gotTargetType, &gotTargetID, &rawDetails)
	require.NoError(t, err)
	require.Equal(t, spaceID, gotSpaceID)
	require.Equal(t, actorID, gotActorID)
	require.Equal(t, action, gotAction)
	require.Equal(t, targetType, gotTargetType)
	require.Equal(t, targetID, gotTargetID)
	var gotDetails map[string]any
	require.NoError(t, json.Unmarshal([]byte(rawDetails), &gotDetails))
	require.Equal(t, wantDetails, gotDetails)
	var outboxID, outboxSpaceID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT audit_event_id,space_id FROM audit_outbox WHERE audit_event_id=$1`, auditID).Scan(&outboxID, &outboxSpaceID))
	require.Equal(t, auditID, outboxID)
	require.Equal(t, spaceID, outboxSpaceID)
}

func ptr(s string) *string { return &s }

func ptrInt32(v int32) *int32 { return &v }
