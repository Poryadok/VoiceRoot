package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/messaging/internal/gameprotocol"
)

func TestGameMessageStoreReceiptFirstAndRevisionChain(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	s := &MessagesStore{Pool: pool}
	app, env, op, chat, msg := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	profile := uuid.New()
	create := gameprotocol.Message{
		Compact: "create-jws", Operation: "create", KeyID: uuid.New(), DeviceID: uuid.New(), AuthorityRevision: 1, ApplicationID: app, EnvironmentID: env,
		OperationID: op, ChatID: chat, MessageID: msg, Revision: 1, Content: []byte("hello"), ContentSHA256: hashContent([]byte("hello")),
	}
	attachment := gameprotocol.Attachment{
		FileID: uuid.New(), ObjectRevision: 1, ByteLength: 23,
		ContentSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		MediaType:     "image/png",
	}
	create.Attachments = []gameprotocol.Attachment{attachment}
	first, err := s.ApplyGameMessage(ctx, create, profile)
	require.NoError(t, err)
	require.Equal(t, msg, first.ID)
	require.JSONEq(t, `[{"file_id":"`+attachment.FileID.String()+`","object_revision":1,"byte_length":23,"content_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","media_type":"image/png"}]`, first.AttachmentsJSON)
	createKey := gameprotocol.ReceiptKey{ApplicationID: app, EnvironmentID: env, OperationID: op, ChatID: chat, MessageID: msg, Revision: 1, Compact: create.Compact}
	lookedUp, found, err := s.LookupGameMessageReceipt(ctx, createKey)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "hello", lookedUp.Content)

	// An exact retry is receipt-first and returns the original result.
	replayed, err := s.ApplyGameMessage(ctx, create, profile)
	require.NoError(t, err)
	require.Equal(t, first.ID, replayed.ID)
	require.Equal(t, "hello", replayed.Content)

	// A reused operation ID with different signed bytes is a conflict.
	changed := create
	changed.Compact = "different-jws"
	changed.Content = []byte("changed")
	changed.ContentSHA256 = hashContent(changed.Content)
	_, err = s.ApplyGameMessage(ctx, changed, profile)
	require.ErrorIs(t, err, ErrGameOperationConflict)
	createKey.Compact = changed.Compact
	_, found, err = s.LookupGameMessageReceipt(ctx, createKey)
	require.ErrorIs(t, err, ErrGameOperationConflict)
	require.False(t, found)

	// A revision cannot skip, fork, or continue after a terminal delete.
	edit := create
	edit.Compact, edit.Operation, edit.OperationID, edit.Revision = "edit-jws", "edit", uuid.New(), 2
	edit.Content = []byte("edited")
	edit.ContentSHA256 = hashContent(edit.Content)
	edit.Attachments = nil
	edit.PreviousRevisionHash = hashCompact(create.Compact)
	second, err := s.ApplyGameMessage(ctx, edit, profile)
	require.NoError(t, err)
	require.Equal(t, "edited", second.Content)
	require.Equal(t, "[]", second.AttachmentsJSON)
	// A receipt retains its original immutable response after later revisions.
	replayedCreate, err := s.ApplyGameMessage(ctx, create, profile)
	require.NoError(t, err)
	require.Equal(t, "hello", replayedCreate.Content)

	delete := edit
	delete.Compact, delete.Operation, delete.OperationID, delete.Revision = "delete-jws", "delete", uuid.New(), 3
	delete.Content = nil
	delete.ContentSHA256 = hashContent(nil)
	delete.PreviousRevisionHash = hashCompact(edit.Compact)
	_, err = s.ApplyGameMessage(ctx, delete, profile)
	require.NoError(t, err)
	replayedDelete, err := s.ApplyGameMessage(ctx, delete, profile)
	require.NoError(t, err)
	require.NotNil(t, replayedDelete.DeletedAt)

	postDelete := delete
	postDelete.Compact, postDelete.Operation, postDelete.OperationID, postDelete.Revision = "edit-after-delete", "edit", uuid.New(), 4
	_, err = s.ApplyGameMessage(ctx, postDelete, profile)
	require.ErrorIs(t, err, ErrGameMessageTerminal)
}

func TestGameMessageStoreConcurrentExactRetryCommitsOneRevision(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	s := &MessagesStore{Pool: pool}
	message := gameprotocol.Message{
		Compact: "concurrent-create-jws", Operation: "create", KeyID: uuid.New(), DeviceID: uuid.New(), AuthorityRevision: 1, ApplicationID: uuid.New(), EnvironmentID: uuid.New(),
		OperationID: uuid.New(), ChatID: uuid.New(), MessageID: uuid.New(), Revision: 1,
		Content: []byte("hello"), ContentSHA256: hashContent([]byte("hello")),
	}
	profile := uuid.New()
	const retries = 8
	results := make(chan *MessageRow, retries)
	errorsCh := make(chan error, retries)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < retries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			row, err := s.ApplyGameMessage(ctx, message, profile)
			if err != nil {
				errorsCh <- err
				return
			}
			results <- row
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsCh)
	require.Empty(t, errorsCh)
	for row := range results {
		require.Equal(t, message.MessageID, row.ID)
	}
	var revisions, receipts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, message.MessageID).Scan(&revisions))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_operation_receipts WHERE operation_id=$1`, message.OperationID).Scan(&receipts))
	require.Equal(t, 1, revisions)
	require.Equal(t, 1, receipts)
}

func TestGameMessageStoreEnforcesMonotonicDeviceAuthorityRevision(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	s := &MessagesStore{Pool: pool}
	app, env, device, key := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	apply := func(name string, revision int64, keyID uuid.UUID) error {
		message := gameprotocol.Message{
			Compact: name, Operation: "create", KeyID: keyID, DeviceID: device, AuthorityRevision: revision,
			ApplicationID: app, EnvironmentID: env, OperationID: uuid.New(), ChatID: uuid.New(), MessageID: uuid.New(), Revision: 1,
			Content: []byte(name), ContentSHA256: hashContent([]byte(name)),
		}
		_, err := s.ApplyGameMessage(ctx, message, uuid.New())
		return err
	}
	require.NoError(t, apply("first", 5, key))
	require.NoError(t, apply("same-revision-same-key", 5, key))
	require.ErrorIs(t, apply("lower", 4, key), ErrGameAuthorityRevisionConflict)
	require.ErrorIs(t, apply("same-revision-different-key", 5, uuid.New()), ErrGameAuthorityRevisionConflict)
	newKey := uuid.New()
	require.NoError(t, apply("forward-jump", 8, newKey))
	var revision int64
	var storedKey uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT authority_revision,key_id FROM game_message_device_authorities WHERE application_id=$1 AND environment_id=$2 AND device_id=$3`, app, env, device).Scan(&revision, &storedKey))
	require.EqualValues(t, 8, revision)
	require.Equal(t, newKey, storedKey)
}

func TestGameMessageStoreAppendsOneImmutableModeratorTombstone(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_game_message_revisions.up.sql"))
	s := &MessagesStore{Pool: pool}
	app, env, chat, msg, action := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	create := gameprotocol.Message{Compact: "create-signed", Operation: "create", KeyID: uuid.New(), DeviceID: uuid.New(), AuthorityRevision: 1, ApplicationID: app, EnvironmentID: env, OperationID: uuid.New(), ChatID: chat, MessageID: msg, Revision: 1, Content: []byte("message"), ContentSHA256: hashContent([]byte("message"))}
	_, err := s.ApplyGameMessage(ctx, create, uuid.New())
	require.NoError(t, err)
	input := gameprotocol.GameMessageTombstone{ApplicationID: app, EnvironmentID: env, ChatID: chat, MessageID: msg, ActionID: action, ReasonClass: "moderation"}
	signerCalls := 0
	signer := func(tombstone gameprotocol.GameMessageTombstone) (string, error) {
		signerCalls++
		require.EqualValues(t, 2, tombstone.Revision)
		require.Equal(t, hashCompact(create.Compact), tombstone.PreviousRevisionHash)
		return "signed-tombstone", nil
	}
	row, compact, err := s.AppendGameMessageTombstone(ctx, input, signer)
	require.NoError(t, err)
	require.Equal(t, "signed-tombstone", compact)
	require.NotNil(t, row.DeletedAt)
	require.Equal(t, 1, signerCalls)
	_, replay, err := s.AppendGameMessageTombstone(ctx, input, signer)
	require.NoError(t, err)
	require.Equal(t, compact, replay)
	require.Equal(t, 1, signerCalls, "an exact retry returns its original signature")
	changed := input
	changed.ReasonClass = "system_retention"
	_, _, err = s.AppendGameMessageTombstone(ctx, changed, signer)
	require.ErrorIs(t, err, ErrGameOperationConflict)
	_, err = s.ApplyGameMessage(ctx, gameprotocol.Message{Compact: "resurrection", Operation: "edit", KeyID: create.KeyID, DeviceID: create.DeviceID, AuthorityRevision: 1, ApplicationID: app, EnvironmentID: env, OperationID: uuid.New(), ChatID: chat, MessageID: msg, Revision: 3, Content: []byte("resurrected"), ContentSHA256: hashContent([]byte("resurrected")), PreviousRevisionHash: hashCompact(compact)}, uuid.New())
	require.ErrorIs(t, err, ErrGameMessageTerminal)
	var revisions, actions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE chat_id=$1 AND message_id=$2`, chat, msg).Scan(&revisions))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM game_message_tombstone_actions WHERE action_id=$1`, action).Scan(&actions))
	require.Equal(t, 2, revisions)
	require.Equal(t, 1, actions)
}
