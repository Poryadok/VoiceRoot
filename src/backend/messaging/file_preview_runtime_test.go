package main

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/integrationtest"
)

type previewLivePolicy struct{}

func (previewLivePolicy) Load(context.Context, uuid.UUID) (*store.ChatThreadPolicy, error) {
	return &store.ChatThreadPolicy{}, nil
}

func TestFilePreviewRuntimeDurableRollbackReplayAndRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated PostgreSQL and NATS")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "preview_runtime", "")
	defer pool.Close()
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	// Historical patches 000003..000010 are already folded into init.
	for _, name := range []string{"000001_init", "000002_client_message_id", "000011_last_delivered_message_id", "000012_messages_content_type", "000016_game_message_revisions", "000017_game_message_execution_permits", "000018_t52_game_cards", "000019_t57_game_action_results", "000020_managed_chat_purge", "000021_space_lifecycle", "000022_space_manifest_import", "000023_space_lifecycle_fences", "000024_space_lifecycle_related_mutation_gates", "000025_space_file_producer", "000026_attachment_send_intents", "000027_message_event_outbox", "000028_music_metadata", "000029_music_file_snapshot", "000030_send_request_receipts", "000034_file_preview"} {
		integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src/backend/migrations/messaging_db", name+".up.sql"))
	}
	messages := &store.MessagesStore{Pool: pool}
	file, id, chat := uuid.New(), uuid.New(), uuid.New()
	snapshot := &eventsv1.FilePreviewSnapshot{ProtocolVersion: 1, FileId: file.String(), MetadataRevision: 1, Status: "ready", FileType: "image", SizeBytes: 42, ScanResult: "clean", Width: proto.Int32(20)}
	ref := &filev1.FileReferenceKey{FileId: file.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: id.String()}
	row := store.MessageRow{ID: id, ChatID: chat, SenderProfileID: uuid.New(), Type: "regular", ContentType: "photo", AttachmentsJSON: `[{"file_id":"` + file.String() + `","type":"photo"}]`, MentionsJSON: "[]", FilePreviewBindings: []store.FilePreviewBinding{{Slot: 0, FileID: file.String(), Kind: "photo", Reference: ref, Snapshot: snapshot}}}
	_, _, err := messages.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.NoError(t, err)
	broker, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir(),
		Users: []*server.User{
			{Username: "admin", Password: "fixture-admin"},
			{Username: "messaging", Password: "fixture-messaging", Permissions: &server.Permissions{
				Publish:   &server.SubjectPermission{Allow: []string{"$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata", "$JS.ACK.file_events.messaging_music_metadata.>"}},
				Subscribe: &server.SubjectPermission{Allow: []string{"_INBOX.voice.messaging.>"}},
			}},
		},
	})
	require.NoError(t, err)
	go broker.Start()
	require.True(t, broker.ReadyForConnections(5*time.Second))
	t.Cleanup(broker.Shutdown)
	nc, err := nats.Connect(broker.ClientURL(), nats.UserInfo("admin", "fixture-admin"))
	require.NoError(t, err)
	defer nc.Close()
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: musicMetadataStream, Subjects: []string{musicMetadataSubject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	// Preserve populated stream data. DeliverNew is not silently changed to replay.
	_, err = js.Publish(musicMetadataSubject, []byte("historical unrelated data"))
	require.NoError(t, err)
	_, err = js.AddConsumer(musicMetadataStream, musicMetadataConsumerConfig())
	require.NoError(t, err)
	consumerURL := "nats://messaging:fixture-messaging@" + strings.TrimPrefix(broker.ClientURL(), "nats://")
	// The consumer uses its existing INFO/ACK/inbox authority, without CREATE
	// or arbitrary file publication. Observe actual broker permission denials.
	denied := make(chan error, 2)
	limited, err := nats.Connect(consumerURL, nats.CustomInboxPrefix("_INBOX.voice.messaging"), nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) { denied <- err }))
	require.NoError(t, err)
	defer limited.Close()
	limitedJS, err := limited.JetStream(nats.MaxWait(200 * time.Millisecond))
	require.NoError(t, err)
	_, err = limitedJS.AddConsumer(musicMetadataStream, &nats.ConsumerConfig{Durable: "forbidden", AckPolicy: nats.AckExplicitPolicy})
	require.Error(t, err)
	require.Eventually(t, func() bool { return len(denied) > 0 }, time.Second, time.Millisecond)
	require.ErrorContains(t, <-denied, "Permissions Violation")
	require.NoError(t, limited.Publish(musicMetadataSubject, []byte("forbidden")))
	require.NoError(t, limited.Flush())
	require.Eventually(t, func() bool { return len(denied) > 0 }, time.Second, time.Millisecond)
	require.ErrorContains(t, <-denied, "Permissions Violation")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	consumerCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- runMusicMetadataConsumerOnce(consumerCtx, consumerURL, messages, previewLivePolicy{}, logger)
	}()
	defer func() { cancel(); <-done }()
	require.Eventually(t, func() bool {
		info, e := js.ConsumerInfo(musicMetadataStream, musicMetadataDurable)
		return e == nil && info.PushBound
	}, 5*time.Second, 20*time.Millisecond)
	_, err = pool.Exec(ctx, `CREATE FUNCTION reject_preview_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected preview outbox failure'; END $$; CREATE TRIGGER reject_preview_outbox BEFORE INSERT ON message_event_outbox FOR EACH ROW EXECUTE FUNCTION reject_preview_outbox()`)
	require.NoError(t, err)
	event := &eventsv1.FileStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.FileStreamEvent_FileProcessed{FileProcessed: &eventsv1.FileProcessed{FileId: file.String(), Status: "ready", Preview: proto.Clone(snapshot).(*eventsv1.FilePreviewSnapshot)}}}
	event.GetFileProcessed().Preview.MetadataRevision = 2
	event.GetFileProcessed().Preview.Width = proto.Int32(99)
	raw, err := proto.Marshal(event)
	require.NoError(t, err)
	_, err = js.Publish(musicMetadataSubject, raw)
	require.NoError(t, err)
	require.Eventually(t, func() bool { work, e := messages.FilePreviewPending(ctx); return e == nil && len(work) == 1 }, 5*time.Second, 20*time.Millisecond)
	work, err := messages.FilePreviewPending(ctx)
	require.NoError(t, err)
	_, err = messages.RefreshFilePreview(ctx, work[0], nil)
	require.Error(t, err)
	before, err := messages.GetMessageByID(ctx, id)
	require.NoError(t, err)
	require.Contains(t, before.AttachmentsJSON, `"width": 20`)
	// The authoritative fact survives an application rollback and consumer restart.
	cancel()
	<-done
	consumerCtx, cancel = context.WithCancel(ctx)
	defer cancel()
	done = make(chan error, 1)
	go func() {
		done <- runMusicMetadataConsumerOnce(consumerCtx, consumerURL, messages, previewLivePolicy{}, logger)
	}()
	_, err = pool.Exec(ctx, `DROP TRIGGER reject_preview_outbox ON message_event_outbox`)
	require.NoError(t, err)
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		runFilePreviewPending(workerCtx, messages, previewLivePolicy{}, logger)
	}()
	defer func() { stopWorker(); <-workerDone }()
	require.Eventually(t, func() bool {
		saved, e := messages.GetMessageByID(ctx, id)
		return e == nil && strings.Contains(saved.AttachmentsJSON, `"width": 99`)
	}, 5*time.Second, 20*time.Millisecond)
	_, err = js.Publish(musicMetadataSubject, raw)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		info, e := js.ConsumerInfo(musicMetadataStream, musicMetadataDurable)
		return e == nil && info.NumAckPending == 0 && info.NumPending == 0
	}, 5*time.Second, 20*time.Millisecond)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE subject='message.edited'`).Scan(&count))
	require.Equal(t, 1, count)
	info, err := js.StreamInfo(musicMetadataStream)
	require.NoError(t, err)
	require.EqualValues(t, 3, info.State.Msgs)
	consumer, err := js.ConsumerInfo(musicMetadataStream, musicMetadataDurable)
	require.NoError(t, err)
	require.Equal(t, nats.DeliverNewPolicy, consumer.Config.DeliverPolicy)
	require.Equal(t, musicMetadataDurable, consumer.Config.DeliverGroup)
}
