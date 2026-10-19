package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

// Controlled source facts isolate persistence. Actual protected Chat source
// interoperability is a separate runtime oracle, not inferred from these rows.
func chatScopeObservationFixture(t *testing.T, space, chat uuid.UUID) (store.ChatScopeObservation, *chatv1.GetBotChatScopeAuthorityRequest, *chatv1.GetBotChatScopeAuthorityResponse) {
	t.Helper()
	request := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), OperationId: uuid.NewString()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	digest := sha256.Sum256(body)
	response := &chatv1.GetBotChatScopeAuthorityResponse{RequestSha256: digest[:], Scope: &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), ActualSpaceId: space.String(), OperationId: request.OperationId, Exists: true, Live: true, ChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY}}
	observation, err := store.NewChatScopeObservation(request, response, true)
	require.NoError(t, err)
	return observation, request, response
}

func chatScopeDownSQL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../migrations/bot_db/000010_bot_chat_scope_observations.down.sql"))
	require.NoError(t, err)
	return string(body)
}

func TestChatScopeDurableBytesAndExactCompletionPreserveLaterDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL immutable source")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Source bytes", "", `[]`, uuid.New())
	require.NoError(t, err)
	chat, space, installer := uuid.New(), uuid.New(), uuid.New()
	observation, request, response := chatScopeObservationFixture(t, space, chat)
	operation := uuid.MustParse(request.OperationId)
	wantRequest, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	wantResponse, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	require.NoError(t, err)
	qualified := store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation})
	request.ExpectedSpaceId, response.Scope.ChatId = uuid.NewString(), uuid.NewString()
	intent, err := st.PrepareInstallation(qualified, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	var actualRequest, actualResponse []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT request_bytes,response_bytes FROM bot_chat_scope_observations WHERE observation_id=$1`, operation).Scan(&actualRequest, &actualResponse))
	require.Equal(t, wantRequest, actualRequest)
	require.Equal(t, wantResponse, actualResponse)
	require.NoError(t, st.CompleteInstallation(qualified, intent, []uuid.UUID{chat}))
	var attached uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT scope_observation_id FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2`, bot.ID, chat).Scan(&attached))
	require.Equal(t, operation, attached)
	restarted := &store.BotStore{Pool: st.Pool}
	again, err := restarted.PrepareInstallation(context.Background(), bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.Equal(t, intent.OperationID, again.OperationID)
	require.NoError(t, st.SetChatEnabled(context.Background(), bot.ID, chat, space, installer, false))
	require.NoError(t, restarted.CompleteInstallation(context.Background(), again, []uuid.UUID{chat}))
	enabled, err := st.IsChatWhitelisted(ctx, bot.ID, chat)
	require.NoError(t, err)
	require.False(t, enabled, "old exact completion cannot undo a later disable")
}

func TestChatScopeAdmissionSetAndLateInsertFailureAreAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL atomic provenance")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Atomic source", "", `[]`, uuid.New())
	require.NoError(t, err)
	chat, space := uuid.New(), uuid.New()
	observation, _, _ := chatScopeObservationFixture(t, space, chat)
	qualified := store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation})
	_, err = st.PrepareInstallation(qualified, bot.ID, space, uuid.New(), []uuid.UUID{chat, uuid.New()})
	require.ErrorIs(t, err, store.ErrChatScopeObservation)
	_, err = st.Pool.Exec(ctx, `ALTER TABLE bot_chat_scope_observations ADD CONSTRAINT injected_source_failure CHECK(false) NOT VALID`)
	require.NoError(t, err)
	_, err = st.PrepareInstallation(qualified, bot.ID, space, uuid.New(), []uuid.UUID{chat})
	require.Error(t, err)
	for _, table := range []string{"bot_installation_intents", "bot_chat_scope_observations", "bot_chat_whitelist"} {
		var count int
		require.NoError(t, st.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE bot_id=$1", bot.ID).Scan(&count))
		require.Zero(t, count, table)
	}
}

func TestChatScopeDownWaitsForActualFirstAdmissionAndPreservesIt(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL first writer DOWN")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Concurrent source", "", `[]`, uuid.New())
	require.NoError(t, err)
	chat, space := uuid.New(), uuid.New()
	observation, _, _ := chatScopeObservationFixture(t, space, chat)
	qualified := store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation})
	holder, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer holder.Release()
	_, err = holder.Exec(ctx, `SELECT pg_advisory_lock(2471001)`)
	require.NoError(t, err)
	defer func() { _, _ = holder.Exec(context.Background(), `SELECT pg_advisory_unlock(2471001)`) }()
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION be170_pause_scope_admission() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(2471001); RETURN NEW; END $$; CREATE TRIGGER be170_pause_scope_admission BEFORE INSERT ON bot_chat_scope_observations FOR EACH ROW EXECUTE FUNCTION be170_pause_scope_admission()`)
	require.NoError(t, err)
	writerDone := make(chan error, 1)
	go func() {
		_, err := st.PrepareInstallation(qualified, bot.ID, space, uuid.New(), []uuid.UUID{chat})
		writerDone <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=2471001 AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	dropper, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer dropper.Release()
	pid := dropper.Conn().PgConn().PID()
	downDone := make(chan error, 1)
	down := chatScopeDownSQL(t)
	go func() { _, err := dropper.Exec(ctx, down); downDone <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := st.Pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond, "full DOWN waits for actual first admission")
	_, err = holder.Exec(ctx, `SELECT pg_advisory_unlock(2471001)`)
	require.NoError(t, err)
	require.NoError(t, <-writerDone)
	require.ErrorContains(t, <-downDone, "authority is retained")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_scope_observations`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestChatScopeDownRejectsOldRRAndAllowsEmptyDefaultRC(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL rollback isolation")
	}
	for _, old := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty RC", true: "old RR"}[old], func(t *testing.T) {
			ctx := t.Context()
			st := startBotStore(t)
			if !old {
				_, err := st.Pool.Exec(ctx, chatScopeDownSQL(t))
				require.NoError(t, err)
				return
			}
			tx, err := st.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
			require.NoError(t, err)
			defer tx.Rollback(context.Background())
			var count int
			require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM bot_chat_scope_observations`).Scan(&count))
			require.Zero(t, count)
			bot, _, err := st.CreateBot(ctx, uuid.New(), "RR source", "", `[]`, uuid.New())
			require.NoError(t, err)
			chat, space := uuid.New(), uuid.New()
			observation, _, _ := chatScopeObservationFixture(t, space, chat)
			_, err = st.PrepareInstallation(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation}), bot.ID, space, uuid.New(), []uuid.UUID{chat})
			require.NoError(t, err)
			_, err = tx.Exec(ctx, chatScopeDownSQL(t))
			require.ErrorContains(t, err, "READ COMMITTED")
			require.NoError(t, tx.Rollback(ctx))
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_scope_observations`).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestChatScopeOfficialMigration10RetainsAcceptedProvenance(t *testing.T) {
	if testing.Short() {
		t.Skip("official complete Bot migrations")
	}
	ctx := t.Context()
	_, file, _, _ := runtime.Caller(0)
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(filepath.Dir(file), "../../../migrations/bot_db"))
	database.Run(true, "up")
	st := &store.BotStore{Pool: database.Pool}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Official scope", "", `[]`, uuid.New())
	require.NoError(t, err)
	chat, space := uuid.New(), uuid.New()
	observation, _, _ := chatScopeObservationFixture(t, space, chat)
	_, err = st.PrepareInstallation(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation}), bot.ID, space, uuid.New(), []uuid.UUID{chat})
	require.NoError(t, err)
	database.Run(false, "goto", "9")
	var count int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_scope_observations`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestChatScopeRequalificationPreservesOriginalInstallationLineage(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL original/current Chat lineage")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Original Chat source", "", `[]`, uuid.New())
	require.NoError(t, err)
	chat, space, installer := uuid.New(), uuid.New(), uuid.New()
	original, request, _ := chatScopeObservationFixture(t, space, chat)
	qualified := store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{original})
	intent, err := st.PrepareInstallation(qualified, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(qualified, intent, []uuid.UUID{chat}))
	assertLineage := func(current string) {
		t.Helper()
		var origin, actual, installation *string
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_jsonb(w)->>'origin_scope_observation_id',to_jsonb(w)->>'scope_observation_id',to_jsonb(w)->>'source_installation_id' FROM bot_chat_whitelist w WHERE bot_id=$1 AND chat_id=$2`, bot.ID, chat).Scan(&origin, &actual, &installation))
		require.Equal(t, request.OperationId, valueOrEmpty(origin), "original accepted target survives requalification")
		require.Equal(t, current, valueOrEmpty(actual))
		require.Equal(t, intent.InstallationID.String(), valueOrEmpty(installation))
	}
	assertLineage(request.OperationId)
	require.NoError(t, st.SetChatEnabled(ctx, bot.ID, chat, space, installer, false))
	fresh, enabledRequest, _ := chatScopeObservationFixture(t, space, chat)
	require.NoError(t, st.SetChatEnabled(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{fresh}), bot.ID, chat, space, installer, true))
	assertLineage(enabledRequest.OperationId)
	setter, setterRequest, _ := chatScopeObservationFixture(t, space, chat)
	actual, err := st.InstallInSpace(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{setter}), bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.Equal(t, intent.InstallationID, actual)
	assertLineage(setterRequest.OperationId)
	operation, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	var scope []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT scope_bytes FROM bot_deletion_installations WHERE operation_id=$1`, operation.OperationID).Scan(&scope))
	var frozen struct {
		ChatSources []struct {
			ChatID   uuid.UUID `json:"chat_id"`
			Original struct {
				OperationID uuid.UUID `json:"operation_id"`
			} `json:"original"`
			Current struct {
				OperationID uuid.UUID `json:"operation_id"`
			} `json:"current"`
		} `json:"chat_sources"`
	}
	require.NoError(t, json.Unmarshal(scope, &frozen))
	require.Len(t, frozen.ChatSources, 1, "root survives local whitelist deletion with both source observations")
	require.Equal(t, chat, frozen.ChatSources[0].ChatID)
	require.Equal(t, uuid.MustParse(request.OperationId), frozen.ChatSources[0].Original.OperationID)
	require.Equal(t, uuid.MustParse(setterRequest.OperationId), frozen.ChatSources[0].Current.OperationID)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestPendingInstallationDeletionFreezesAcceptedOriginalChatSource(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL pending installation snapshot")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Pending original source", "", `[]`, uuid.New())
	require.NoError(t, err)
	type expected struct {
		intent   *store.InstallationIntent
		request  *chatv1.GetBotChatScopeAuthorityRequest
		response *chatv1.GetBotChatScopeAuthorityResponse
		chat     uuid.UUID
	}
	var cases []expected
	for _, removed := range []bool{false, true} {
		space, chat, installer := uuid.New(), uuid.New(), uuid.New()
		observation, request, response := chatScopeObservationFixture(t, space, chat)
		qualified := store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation})
		intent, err := st.PrepareInstallation(qualified, bot.ID, space, installer, []uuid.UUID{chat})
		require.NoError(t, err)
		if removed {
			require.NoError(t, st.CompleteInstallation(qualified, intent, []uuid.UUID{chat}))
			_, err = st.InstallInSpace(ctx, bot.ID, space, installer, nil)
			require.NoError(t, err)
		}
		cases = append(cases, expected{intent: intent, request: request, response: response, chat: chat})
	}
	operation, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	for _, want := range cases {
		var raw []byte
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT scope_bytes FROM bot_deletion_installations WHERE operation_id=$1 AND space_id=$2`, operation.OperationID, want.intent.SpaceID).Scan(&raw))
		var frozen store.BotDeletionTarget
		require.NoError(t, json.Unmarshal(raw, &frozen))
		require.Len(t, frozen.ChatSources, 1, "accepted PREPARING source must be recoverable from immutable root without joining its live intent")
		require.Equal(t, want.intent.InstallationID, frozen.ChatSources[0].InstallationID)
		require.Equal(t, want.chat, frozen.ChatSources[0].ChatID)
		require.Equal(t, uuid.MustParse(want.request.OperationId), frozen.ChatSources[0].Original.OperationID)
		require.Nil(t, frozen.ChatSources[0].Current, "no invented current whitelist observation")
		requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(want.request)
		require.NoError(t, err)
		responseBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(want.response)
		require.NoError(t, err)
		require.Equal(t, requestBytes, frozen.ChatSources[0].Original.Request)
		require.Equal(t, responseBytes, frozen.ChatSources[0].Original.Response)
		requestSHA, responseSHA := sha256.Sum256(requestBytes), sha256.Sum256(responseBytes)
		require.Equal(t, requestSHA[:], frozen.ChatSources[0].Original.RequestSHA)
		require.Equal(t, responseSHA[:], frozen.ChatSources[0].Original.ResponseSHA)
		_, err = st.Pool.Exec(ctx, `UPDATE bot_chat_scope_observations SET response_bytes='corrupt later live source'::bytea WHERE observation_id=$1`, uuid.MustParse(want.request.OperationId))
		require.NoError(t, err)
		var again []byte
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT scope_bytes FROM bot_deletion_installations WHERE operation_id=$1 AND space_id=$2`, operation.OperationID, want.intent.SpaceID).Scan(&again))
		require.Equal(t, raw, again, "frozen obligation never joins later observation state")
	}
}

func TestChatObservationCannotBeRelabeledForReinstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL source lineage")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Source lineage", "", `[]`, uuid.New())
	require.NoError(t, err)
	chat, space, installer := uuid.New(), uuid.New(), uuid.New()
	old, _, _ := chatScopeObservationFixture(t, space, chat)
	qualified := store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{old})
	first, err := st.PrepareInstallation(qualified, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(qualified, first, []uuid.UUID{chat}))
	cancel, err := st.BeginInstallationCancellation(ctx, bot.ID, space, installer)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallationCancellation(ctx, cancel))
	_, err = st.PrepareInstallation(qualified, bot.ID, space, installer, []uuid.UUID{chat})
	require.ErrorIs(t, err, store.ErrChatScopeObservation, "one completed read cannot be relabeled as a new installation admission")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents WHERE bot_id=$1 AND state<>'CANCELED'`, bot.ID).Scan(&count))
	require.Zero(t, count)
	fresh, _, _ := chatScopeObservationFixture(t, space, chat)
	next, err := st.PrepareInstallation(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{fresh}), bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.EqualValues(t, 2, next.Generation)
	require.NotEqual(t, first.OperationID, next.OperationID)
}
