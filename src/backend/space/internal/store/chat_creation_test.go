package store

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/spaceaudit"
)

func compositeStoreFixture(t *testing.T) (*SpaceStore, *spacev1.SpaceChatCreationBinding) {
	t.Helper()
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applyAuditLedgerMigrationForTest(t, ctx, st)
	applyAuditIngestionMigration(t, st)
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000031_composite_audit_reservation.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(body))
	require.NoError(t, err)
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Composite", "", "private")
	require.NoError(t, err)
	binding := &spacev1.SpaceChatCreationBinding{ProtocolVersion: 1, OperationId: uuid.NewString(), CreatorProfileId: owner.String(), SpaceId: space.ID.String(), ChatId: uuid.NewString(), ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, NodeId: uuid.NewString(), AuditEventId: uuid.NewString(), CreateRequestSha256: make([]byte, 32), Initiator: &spacev1.SpaceChatCreationBinding_User{User: &spacev1.SpaceChatCreationUser{AccountId: uuid.NewString(), SessionEpoch: 7, AccountType: "regular"}}}
	return st, binding
}

func TestCompositeStoreGlobalAuditReservation(t *testing.T) {
	for _, origin := range []string{"local", "role", "composite"} {
		t.Run(origin, func(t *testing.T) {
			st, b := compositeStoreFixture(t)
			ctx := context.Background()
			switch origin {
			case "local":
				insertAuditFixture(t, st, uuid.MustParse(b.AuditEventId), uuid.MustParse(b.SpaceId), uuid.MustParse(b.CreatorProfileId), "member_banned", time.Now().UTC())
			case "role":
				e := spaceaudit.ReplayEvent{AuditEventID: b.AuditEventId, SpaceID: b.SpaceId, ActorProfileID: b.CreatorProfileId, Action: "role_created", TargetType: "role", TargetID: uuid.NewString(), DetailsJSON: "{}", OccurredAt: timestamppb.Now()}
				require.NoError(t, st.ImportAuditEvent(ctx, "role", e))
			}
			_, err := st.CommitChatCreation(ctx, b, codes.OK)
			if origin != "composite" {
				require.ErrorIs(t, err, ErrCompositeConflict)
				var nodes int
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_tree_nodes WHERE id=$1`, b.NodeId).Scan(&nodes))
				require.Zero(t, nodes)
				return
			}
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details) VALUES($1,$2,$3,'member_banned','account',$4,'{}')`, b.AuditEventId, b.SpaceId, b.CreatorProfileId, uuid.New())
			require.Error(t, err, "local audit must not steal a reserved composite ID")
			e := spaceaudit.ReplayEvent{AuditEventID: b.AuditEventId, SpaceID: b.SpaceId, ActorProfileID: b.CreatorProfileId, Action: "chat_created", TargetType: "chat", TargetID: b.ChatId, DetailsJSON: "{}", OccurredAt: timestamppb.Now()}
			require.NoError(t, st.ImportAuditEvent(ctx, "chat", e), "original committed fact remains deliverable")
			require.NoError(t, st.ImportAuditEvent(ctx, "chat", e))
		})
	}
}

func TestCompositeStoreCrossSpaceAuditReservationRace(t *testing.T) {
	st, first := compositeStoreFixture(t)
	ctx := context.Background()
	second := proto.Clone(first).(*spacev1.SpaceChatCreationBinding)
	space, err := st.CreateSpace(ctx, uuid.MustParse(first.CreatorProfileId), "Other", "", "private")
	require.NoError(t, err)
	second.SpaceId, second.OperationId, second.ChatId, second.NodeId = space.ID.String(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, binding := range []*spacev1.SpaceChatCreationBinding{first, second} {
		go func(b *spacev1.SpaceChatCreationBinding) {
			<-start
			_, commitErr := st.CommitChatCreation(ctx, b, codes.OK)
			results <- commitErr
		}(binding)
	}
	close(start)
	one, two := <-results, <-results
	if one == nil {
		require.ErrorIs(t, two, ErrCompositeConflict)
	} else {
		require.ErrorIs(t, one, ErrCompositeConflict)
		require.NoError(t, two)
	}
	var fences, nodes int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_chat_creation_fences WHERE audit_event_id=$1`, first.AuditEventId).Scan(&fences))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_tree_nodes WHERE id IN ($1,$2)`, first.NodeId, second.NodeId).Scan(&nodes))
	require.Equal(t, 1, fences)
	require.Equal(t, 1, nodes)
}

func TestCompositeStoreWaitsForUncommittedLocalAuditReservation(t *testing.T) {
	st, b := compositeStoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details) VALUES($1,$2,$3,'member_banned','account',$4,'{}')`, b.AuditEventId, b.SpaceId, b.CreatorProfileId, uuid.New())
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		_, commitErr := st.CommitChatCreation(ctx, b, codes.OK)
		result <- commitErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		queryErr := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted)`).Scan(&waiting)
		return queryErr == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit(ctx))
	require.ErrorIs(t, <-result, ErrCompositeConflict)
	var nodes int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_tree_nodes WHERE id=$1`, b.NodeId).Scan(&nodes))
	require.Zero(t, nodes)
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
