package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/messaging/internal/automodevidence"
	"voice/backend/pkg/automodwire"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func automodSourceDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	raw := os.Getenv("MESSAGING_AUTOMOD_TEST_DSN")
	var pool *pgxpool.Pool
	if raw == "" {
		pool = startPostgresForStoreTest(t, ctx)
		t.Cleanup(pool.Close)
	} else {
		admin, err := pgxpool.New(ctx, raw)
		require.NoError(t, err)
		name := "automod_source_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
		require.NoError(t, err)
		config, err := pgxpool.ParseConfig(raw)
		require.NoError(t, err)
		config.ConnConfig.Database = name
		pool, err = pgxpool.NewWithConfig(ctx, config)
		require.NoError(t, err)
		t.Cleanup(func() {
			pool.Close()
			_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
			admin.Close()
		})
	}
	seedMessagingSchema(t, ctx, pool)
	return pool
}

func TestAutoModProducerConcurrentSourceOrderAndCompoundMessage(t *testing.T) {
	ctx := context.Background()
	pool := automodSourceDatabase(t)
	store := &MessagesStore{Pool: pool}
	ring, err := automodevidence.NewRing(map[string][]byte{"current": bytes.Repeat([]byte{3}, 32), "next": bytes.Repeat([]byte{4}, 32)})
	require.NoError(t, err)
	_, err = store.ConfigureAutoMod(ctx, ring)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE automod_producer_state SET active=true`)
	require.NoError(t, err)
	account, subject, sender := uuid.New(), uuid.New(), uuid.New()
	admission := func() *AutoModAdmission {
		return &AutoModAdmission{Subject: &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}, SenderProfileID: sender, OperationID: uuid.New(), SessionEpoch: 1}
	}
	const sends = 12
	start := make(chan struct{})
	results := make(chan error, sends)
	for i := 0; i < sends; i++ {
		go func(i int) {
			<-start
			row := MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: sender, Content: "same source text", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", AutoModAdmission: admission()}
			if i%3 == 0 {
				row.IsE2E = true
			}
			if i%3 == 1 {
				row.Content = " \t\n"
			}
			_, _, err := store.InsertMessageWithOutbox(ctx, row, nil, nil)
			results <- err
		}(i)
	}
	close(start)
	for i := 0; i < sends; i++ {
		require.NoError(t, <-results)
	}
	rows, err := pool.Query(ctx, `SELECT sequence,source_at,payload FROM automod_evidence_outbox ORDER BY sequence`)
	require.NoError(t, err)
	var previous time.Time
	count := 0
	metadataOnly := 0
	for rows.Next() {
		var sequence int
		var sourceAt time.Time
		var payload []byte
		require.NoError(t, rows.Scan(&sequence, &sourceAt, &payload))
		count++
		require.Equal(t, count, sequence)
		require.True(t, sourceAt.After(previous))
		previous = sourceAt
		event, _, err := automodwire.DecodeCore(payload)
		require.NoError(t, err)
		if event.GetEligibility() != automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_ELIGIBLE {
			metadataOnly++
			require.Empty(t, event.GetDigests())
			require.Zero(t, event.GetLinkCount())
		}
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Equal(t, sends, count)
	require.Equal(t, 8, metadataOnly)
	main := MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: sender, Content: "forwarded main", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", AutoModAdmission: admission()}
	main.ForwardCommentary = &MessageRow{ID: uuid.New(), ChatID: main.ChatID, SenderProfileID: sender, Content: "https://x http://y https://z", Type: "regular", ContentType: "text", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	_, inserted, err := store.InsertMessageWithOutbox(ctx, main, nil, nil)
	require.NoError(t, err)
	require.True(t, inserted)
	var payload []byte
	var companion uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload,companion_id FROM automod_evidence_outbox JOIN automod_message_coverage USING(event_id) WHERE message_id=$1`, main.ID).Scan(&payload, &companion))
	require.Equal(t, main.ForwardCommentary.ID, companion)
	event, _, err := automodwire.DecodeCore(payload)
	require.NoError(t, err)
	require.Equal(t, uint64(sends+1), event.GetSequence())
	require.Equal(t, uint32(3), event.GetLinkCount())
	require.Len(t, event.GetDigests(), 2)
	var messageCount, eventCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&messageCount))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_evidence_outbox`).Scan(&eventCount))
	require.Equal(t, sends+2, messageCount)
	require.Equal(t, sends+1, eventCount, "a forward and commentary are one logical committed send")
	// Physical companion deletion minimizes the local link, never the account
	// detector's immutable core or committed source history.
	_, err = pool.Exec(ctx, `DELETE FROM messages WHERE id=$1`, main.ForwardCommentary.ID)
	require.NoError(t, err)
	var bridges int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_message_coverage WHERE message_id=$1`, main.ID).Scan(&bridges))
	require.Zero(t, bridges)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_evidence_outbox WHERE event_id=$1`, event.GetEventId()).Scan(&bridges))
	require.Equal(t, 1, bridges)
	var after []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM automod_evidence_outbox WHERE event_id=$1`, event.GetEventId()).Scan(&after))
	require.Equal(t, payload, after)
}

func TestAutoModProducerKeyRegistryCannotResetOrDropAliases(t *testing.T) {
	ctx := context.Background()
	pool := automodSourceDatabase(t)
	store := &MessagesStore{Pool: pool}
	keys := map[string][]byte{"current": bytes.Repeat([]byte{5}, 32), "next": bytes.Repeat([]byte{6}, 32)}
	ring, err := automodevidence.NewRing(keys)
	require.NoError(t, err)
	epoch, err := store.ConfigureAutoMod(ctx, ring)
	require.NoError(t, err)
	changed, err := automodevidence.NewRing(map[string][]byte{"current": bytes.Repeat([]byte{9}, 32), "next": keys["next"]})
	require.NoError(t, err)
	_, err = store.ConfigureAutoMod(ctx, changed)
	require.ErrorIs(t, err, automodevidence.ErrKeyRing)
	omitted, err := automodevidence.NewRing(map[string][]byte{"future": bytes.Repeat([]byte{7}, 32), "next": keys["next"]})
	require.NoError(t, err)
	_, err = store.ConfigureAutoMod(ctx, omitted)
	require.ErrorIs(t, err, automodevidence.ErrKeyRing)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_hmac_keys`).Scan(&count))
	require.Equal(t, 2, count, "rejected partial rotation must roll back newly registered keys")
	_, err = pool.Exec(ctx, `UPDATE automod_producer_state SET source_epoch=$1`, uuid.New())
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE automod_producer_state SET format_version=1`)
	require.Error(t, err, "an unsupported restored v1 cannot be made current by resetting the marker")
	repeated, err := store.ConfigureAutoMod(ctx, ring)
	require.NoError(t, err)
	require.Equal(t, epoch, repeated)
	_, err = pool.Exec(ctx, `UPDATE automod_hmac_keys SET retired_at=clock_timestamp() WHERE key_id='current'`)
	require.NoError(t, err)
	_, err = store.ConfigureAutoMod(ctx, ring)
	require.ErrorIs(t, err, automodevidence.ErrKeyRing)
	_, err = pool.Exec(ctx, `UPDATE automod_hmac_keys SET retired_at=NULL WHERE key_id='current'`)
	require.Error(t, err, "retired key identity cannot be reactivated by old configuration")
}

func TestAutoModProducerCommittedInsertAndExactReplay(t *testing.T) {
	ctx := context.Background()
	pool := automodSourceDatabase(t)
	ring, err := automodevidence.NewRing(map[string][]byte{"current": bytes.Repeat([]byte{1}, 32), "next": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	store := &MessagesStore{Pool: pool}
	epoch, err := store.ConfigureAutoMod(ctx, ring)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, epoch)
	repeated, err := store.ConfigureAutoMod(ctx, ring)
	require.NoError(t, err)
	require.Equal(t, epoch, repeated)
	_, err = pool.Exec(ctx, `UPDATE automod_producer_state SET active=true`)
	require.NoError(t, err)
	account, subject, sender := uuid.New(), uuid.New(), uuid.New()
	client := uuid.New()
	row := MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: sender, Content: "private source text https://a http://b HTTPS://c", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", ClientMessageID: &client, AdmissionRequestRPC: "/voice.messaging.v1.MessagingService/SendMessage", AdmissionRequestHash: "sha256:" + strings.Repeat("a", 64),
		AutoModAdmission: &AutoModAdmission{Subject: &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}, SenderProfileID: sender, OperationID: uuid.New(), SessionEpoch: 1}}
	grant := row.AutoModAdmission
	saved, inserted, err := store.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.NoError(t, err)
	require.True(t, inserted)
	require.Equal(t, row.ID, saved.ID)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_evidence_outbox`).Scan(&count))
	require.Equal(t, 1, count, "successful insertion must atomically carry one private evidence event")
	var payload []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM automod_evidence_outbox JOIN automod_message_coverage USING(event_id) WHERE message_id=$1`, row.ID).Scan(&payload))
	evidence, coreHash, err := automodwire.DecodeCore(payload)
	require.NoError(t, err)
	var storedHash []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT core_sha256 FROM automod_evidence_outbox WHERE event_id=$1`, evidence.GetEventId()).Scan(&storedHash))
	require.Equal(t, coreHash[:], storedHash)
	require.Equal(t, uint64(1), evidence.GetSequence())
	require.Equal(t, epoch.String(), evidence.GetSourceEpoch())
	require.Equal(t, uint32(3), evidence.GetLinkCount())
	require.Len(t, evidence.GetDigests(), 2)
	require.False(t, bytes.Contains(payload, []byte("private source text")))
	for _, origin := range []uuid.UUID{row.ID, row.ChatID, row.SenderProfileID, row.AutoModAdmission.OperationID} {
		require.False(t, bytes.Contains(payload, []byte(origin.String())), "retained detector core cannot carry origin identifiers")
	}
	row.ID = uuid.New()
	row.AutoModAdmission = nil
	replayed, inserted, err := store.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.NoError(t, err)
	require.False(t, inserted)
	require.Equal(t, saved.ID, replayed.ID)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_evidence_outbox`).Scan(&count))
	require.Equal(t, 1, count)
	var through int
	require.NoError(t, pool.QueryRow(ctx, `SELECT through_sequence FROM automod_source_subjects WHERE subject_id=$1`, subject).Scan(&through))
	require.Equal(t, 1, through)
	_, err = pool.Exec(ctx, `UPDATE automod_evidence_outbox SET payload=$1 WHERE event_id=$2`, []byte("changed"), evidence.GetEventId())
	require.Error(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM automod_evidence_outbox WHERE event_id=$1`, evidence.GetEventId())
	require.NoError(t, err)
	row.ID = saved.ID
	_, inserted, err = store.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.NoError(t, err)
	require.False(t, inserted, "same message UUID exact replay cannot infer insertion from ID equality")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_evidence_outbox`).Scan(&count))
	require.Zero(t, count, "delivery ACK removal cannot make exact replay a new send")
	newClient := uuid.New()
	row.ID = uuid.New()
	row.ClientMessageID = &newClient
	_, _, err = store.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.ErrorIs(t, err, ErrAutoModAdmission)
	_, _, err = (&MessagesStore{Pool: pool}).InsertMessageWithOutbox(ctx, row, nil, nil)
	require.Error(t, err, "active DB guard closes a forgotten legacy writer")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE id=$1`, row.ID).Scan(&count))
	require.Zero(t, count)
	row.AutoModAdmission = grant
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_private_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected private journal failure'; END $$; CREATE TRIGGER fail_private_evidence BEFORE INSERT ON automod_evidence_outbox FOR EACH ROW EXECUTE FUNCTION fail_private_evidence()`)
	require.NoError(t, err)
	_, _, err = store.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.Error(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE id=$1`, row.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT through_sequence FROM automod_source_subjects WHERE subject_id=$1`, subject).Scan(&through))
	require.Equal(t, 1, through, "journal failure rolls back source sequence and message")
	_, err = pool.Exec(ctx, `DROP TRIGGER fail_private_evidence ON automod_evidence_outbox; DROP FUNCTION fail_private_evidence()`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, subject)
	require.NoError(t, err)
	_, _, err = store.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.ErrorIs(t, err, ErrAutoModAdmission)
	_, err = pool.Exec(ctx, `UPDATE automod_hmac_keys SET fingerprint=$1 WHERE key_id='current'`, bytes.Repeat([]byte{9}, 32))
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE automod_producer_state SET active=false`)
	require.Error(t, err)
	t.Logf("committed subject cursor %d; exact replay retains one event (%s)", through, fmt.Sprint(evidence.GetEligibility()))
}
