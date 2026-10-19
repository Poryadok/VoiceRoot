package store

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/messaging/internal/automodevidence"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func TestAutoModSpaceFreezeErasesOnlyLocalCoverageAndRestoreCannotRebuild(t *testing.T) {
	ctx := context.Background()
	pool := automodSourceDatabase(t)
	s := &MessagesStore{Pool: pool}
	ring, err := automodevidence.NewRing(map[string][]byte{"current": bytes.Repeat([]byte{1}, 32), "next": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	_, err = s.ConfigureAutoMod(ctx, ring)
	require.NoError(t, err)
	account, subject, sender, chat, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	insert := func(id uuid.UUID) {
		row := MessageRow{ID: uuid.New(), ChatID: id, SenderProfileID: sender, Content: "same account evidence", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", AutoModAdmission: &AutoModAdmission{Subject: &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}, SenderProfileID: sender, OperationID: uuid.New(), SessionEpoch: 1}}
		_, _, err = s.InsertMessageWithOutbox(ctx, row, nil, nil)
		require.NoError(t, err)
	}
	insert(chat)
	insert(chat)
	insert(foreign)
	var oldEvent, oldMessage uuid.UUID
	var oldCreated time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id,message_id,created_at FROM automod_message_coverage WHERE chat_id=$1 ORDER BY event_id LIMIT 1`, chat).Scan(&oldEvent, &oldMessage, &oldCreated))
	// An orphaned restored bridge has no remaining message row to join.
	_, err = pool.Exec(ctx, `DELETE FROM messages WHERE id=$1`, oldMessage)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO automod_message_coverage(event_id,message_id,chat_id,created_at) VALUES($1,$2,$3,$4)`, oldEvent, oldMessage, chat, oldCreated)
	require.NoError(t, err)
	space, operation := uuid.New(), uuid.New()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ItemCount: 1, ManifestSha256: messagingChatManifestSHA(space, operation, 7, []uuid.UUID{chat})}
	_, err = s.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{SpaceID: space, DeletionOperationID: operation, ScheduleGeneration: 7, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, ItemIds: []string{chat.String()}, PageSha256: make([]byte, 32)}, SealsManifest: true, RequestBytes: []byte("page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("page-receipt")})
	require.NoError(t, err)
	freeze := SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: operation, Generation: 7, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest, RequestBytes: []byte("freeze"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("frozen")}
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_coverage_erasure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected coverage failure'; END $$; CREATE TRIGGER fail_coverage_erasure BEFORE DELETE ON automod_message_coverage FOR EACH ROW EXECUTE FUNCTION fail_coverage_erasure()`)
	require.NoError(t, err)
	_, err = s.ApplySpaceLifecycleFence(ctx, freeze)
	require.Error(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_space_lifecycle_fences WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count, "privacy failure must roll back the FROZEN acknowledgement")
	_, err = pool.Exec(ctx, `DROP TRIGGER fail_coverage_erasure ON automod_message_coverage;DROP FUNCTION fail_coverage_erasure()`)
	require.NoError(t, err)
	_, err = s.ApplySpaceLifecycleFence(ctx, freeze)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_message_coverage WHERE chat_id=$1`, chat).Scan(&count))
	require.Zero(t, count, "FROZEN must erase exact-manifest origin bridges without a Mod acknowledgement")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_message_coverage WHERE chat_id=$1`, foreign).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_evidence_outbox`).Scan(&count))
	require.Equal(t, 3, count, "Space deletion is not account evidence expiry")
	_, err = s.ApplySpaceLifecycleFence(ctx, freeze)
	require.NoError(t, err)
	freeze.Generation = 8
	freeze.State = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	freeze.RequestBytes = []byte("restore")
	freeze.ReceiptBytes = []byte("restored")
	_, err = s.ApplySpaceLifecycleFence(ctx, freeze)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_message_coverage WHERE chat_id=$1`, chat).Scan(&count))
	require.Zero(t, count, "product restore must not rebuild old origin links")
	insert(chat)
	// Simulate a backup restoring only an obsolete bridge after the lifecycle
	// authority was recovered. Its missing primary must not defeat cleanup.
	_, err = pool.Exec(ctx, `INSERT INTO automod_message_coverage(event_id,message_id,chat_id,created_at) VALUES($1,$2,$3,$4)`, oldEvent, oldMessage, chat, oldCreated)
	require.NoError(t, err)
	require.NoError(t, s.ReconcileAutoModOriginCoverage(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_message_coverage WHERE event_id=$1`, oldEvent).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_message_coverage WHERE chat_id=$1`, chat).Scan(&count))
	require.Equal(t, 1, count, "new post-restore send keeps its independent coverage")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_evidence_outbox`).Scan(&count))
	require.Equal(t, 4, count)
}

func TestAutoModManagedPurgeMinimizesBeforeExternalOwnerWait(t *testing.T) {
	ctx := context.Background()
	pool := automodSourceDatabase(t)
	s := &MessagesStore{Pool: pool}
	ring, err := automodevidence.NewRing(map[string][]byte{"current": bytes.Repeat([]byte{3}, 32), "next": bytes.Repeat([]byte{4}, 32)})
	require.NoError(t, err)
	_, err = s.ConfigureAutoMod(ctx, ring)
	require.NoError(t, err)
	account, subject, sender, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	row := MessageRow{ID: uuid.New(), ChatID: chat, SenderProfileID: sender, Content: "account evidence", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", AutoModAdmission: &AutoModAdmission{Subject: &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}, SenderProfileID: sender, OperationID: uuid.New(), SessionEpoch: 1}}
	_, _, err = s.InsertMessageWithOutbox(ctx, row, nil, nil)
	require.NoError(t, err)
	var event uuid.UUID
	var created, cutoff time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id,created_at FROM automod_message_coverage WHERE message_id=$1`, row.ID).Scan(&event, &created))
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&cutoff))
	operation := uuid.New()
	work, err := s.StartManagedChatPurge(ctx, operation, chat, cutoff, make([]byte, 32))
	require.NoError(t, err)
	require.Equal(t, "PENDING", work.State)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_message_coverage WHERE chat_id=$1`, chat).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE id=$1`, row.ID).Scan(&count))
	require.Equal(t, 1, count, "external owner wait still has the normal product payload")
	_, err = pool.Exec(ctx, `INSERT INTO automod_message_coverage(event_id,message_id,chat_id,created_at) VALUES($1,$2,$3,$4)`, event, row.ID, chat, created)
	require.NoError(t, err)
	require.NoError(t, s.ReconcileAutoModOriginCoverage(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_message_coverage WHERE chat_id=$1`, chat).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_evidence_outbox WHERE event_id=$1`, event).Scan(&count))
	require.Equal(t, 1, count)
}
