package store

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

func compositeStoreFixture(t *testing.T) (*SpaceStore, *spacev1.SpaceChatCreationBinding) {
	t.Helper()
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Composite", "", "private")
	require.NoError(t, err)
	binding := &spacev1.SpaceChatCreationBinding{ProtocolVersion: 1, OperationId: uuid.NewString(), CreatorProfileId: owner.String(), SpaceId: space.ID.String(), ChatId: uuid.NewString(), ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, NodeId: uuid.NewString(), AuditEventId: uuid.NewString(), CreateRequestSha256: make([]byte, 32), Initiator: &spacev1.SpaceChatCreationBinding_User{User: &spacev1.SpaceChatCreationUser{AccountId: uuid.NewString(), SessionEpoch: 7, AccountType: "regular"}}}
	return st, binding
}

func TestCompositeStoreReplayOriginalWithoutReinsertingDeletedNode(t *testing.T) {
	st, b := compositeStoreFixture(t)
	ctx := context.Background()
	first, err := st.CommitChatCreation(ctx, b, codes.OK)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED, first.State)
	_, err = st.Pool.Exec(ctx, `DELETE FROM space_tree_nodes WHERE id=$1`, uuid.MustParse(b.NodeId))
	require.NoError(t, err)
	// Later authority denial cannot rewrite the original committed decision.
	again, err := st.CommitChatCreation(ctx, b, codes.PermissionDenied)
	require.NoError(t, err)
	require.True(t, proto.Equal(first, again))
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_tree_nodes WHERE id=$1`, uuid.MustParse(b.NodeId)).Scan(&count))
	require.Zero(t, count)
	changed := proto.Clone(b).(*spacev1.SpaceChatCreationBinding)
	changed.GetUser().SessionEpoch++
	_, err = st.CommitChatCreation(ctx, changed, codes.OK)
	require.ErrorIs(t, err, ErrCompositeConflict)
	changed = proto.Clone(b).(*spacev1.SpaceChatCreationBinding)
	changed.CreateRequestSha256[0] = 1
	_, err = st.CommitChatCreation(ctx, changed, codes.OK)
	require.ErrorIs(t, err, ErrCompositeConflict)
}

func TestCompositeStoreCommitAndAbortHaveOneImmutableWinner(t *testing.T) {
	st, b := compositeStoreFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan *spacev1.SpaceChatCreationOutcome, 2)
	errs := make(chan error, 2)
	for _, code := range []codes.Code{codes.OK, codes.PermissionDenied} {
		wg.Add(1)
		go func(code codes.Code) {
			defer wg.Done()
			out, err := st.CommitChatCreation(ctx, b, code)
			results <- out
			errs <- err
		}(code)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var first *spacev1.SpaceChatCreationOutcome
	for out := range results {
		if first == nil {
			first = out
		} else {
			require.True(t, proto.Equal(first, out))
		}
	}
	var nodes, fences int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_tree_nodes WHERE id=$1`, uuid.MustParse(b.NodeId)).Scan(&nodes))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_chat_creation_fences WHERE operation_id=$1`, uuid.MustParse(b.OperationId)).Scan(&fences))
	require.Equal(t, 1, fences)
	if first.State == spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED {
		require.Equal(t, 1, nodes)
	} else {
		require.Zero(t, nodes)
	}
}

func TestCompositeStoreFailedEvidenceInsertRollsBackNodeAndFence(t *testing.T) {
	st, b := compositeStoreFixture(t)
	ctx := context.Background()
	// A real database failure after the node/fence writes must roll back both.
	_, err := st.Pool.Exec(ctx, `ALTER TABLE space_chat_creation_evidence ADD CONSTRAINT injected_failure CHECK(false)`)
	require.NoError(t, err)
	_, err = st.CommitChatCreation(ctx, b, codes.OK)
	require.Error(t, err)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_tree_nodes WHERE id=$1)+(SELECT count(*) FROM space_chat_creation_fences WHERE operation_id=$2)`, uuid.MustParse(b.NodeId), uuid.MustParse(b.OperationId)).Scan(&count))
	require.Zero(t, count)
	_, err = st.GetChatCreationOutcome(ctx, uuid.MustParse(b.SpaceId), uuid.MustParse(b.OperationId))
	require.ErrorIs(t, err, ErrCompositeUnknown)
}

func TestCompositeStoreExpiryIsEnforcedBeforePhysicalCleanup(t *testing.T) {
	st, b := compositeStoreFixture(t)
	ctx := context.Background()
	_, err := st.CommitChatCreation(ctx, b, codes.OK)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE space_chat_creation_fences SET deletion_operation_id=$2,purge_generation=1,purge_completed_at=transaction_timestamp()-interval '31 days',full_evidence_expires_at=transaction_timestamp()-interval '1 day' WHERE operation_id=$1`, uuid.MustParse(b.OperationId), uuid.New())
	require.NoError(t, err)
	_, err = st.GetChatCreationOutcome(ctx, uuid.MustParse(b.SpaceId), uuid.MustParse(b.OperationId))
	require.ErrorIs(t, err, ErrCompositeResultExpired)
	_, err = st.CommitChatCreation(ctx, b, codes.OK)
	require.ErrorIs(t, err, ErrCompositeResultExpired)
	changed := proto.Clone(b).(*spacev1.SpaceChatCreationBinding)
	changed.NodeId = uuid.NewString()
	_, err = st.CommitChatCreation(ctx, changed, codes.OK)
	require.ErrorIs(t, err, ErrCompositeConflict)
}
