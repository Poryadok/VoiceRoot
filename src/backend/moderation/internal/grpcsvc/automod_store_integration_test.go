package grpcsvc

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
)

func TestAutoModStore_OrderedInboxAndAtomicSanction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	s := automod.NewStore(pool)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, s.BindLiveSubjectTx(ctx, tx, subject.AccountID, subject.ID, subject.Revision))
	require.NoError(t, s.RequireLiveSubjectTx(ctx, tx, subject.AccountID, subject.ID, subject.Revision))
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, s.Provision(ctx, subject))
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	event := func(seq int64) automod.Envelope {
		return automod.Envelope{Subject: subject, EventID: uuid.New(), Sequence: seq, RecordedAt: now.Add(time.Duration(seq) * time.Second), Eligibility: automod.Eligible,
			Hash: sha256.Sum256([]byte(fmt.Sprintf("event-%d", seq))), Digests: []automod.Digest{{KeyID: "first", Hash: sha256.Sum256([]byte("same-pattern"))}}}
	}
	// Fifth may arrive first, but cannot silently establish an arbitrary baseline.
	fifth := event(5)
	r, err := s.Ingest(ctx, fifth)
	require.NoError(t, err)
	require.EqualValues(t, 0, r.Through)
	for seq := int64(1); seq <= 4; seq++ {
		_, err = s.Ingest(ctx, event(seq))
		require.NoError(t, err)
	}
	var through, actions int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT through_sequence FROM automod_subjects WHERE subject_id=$1`, subject.ID).Scan(&through))
	require.EqualValues(t, 5, through)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_actions`).Scan(&actions))
	require.EqualValues(t, 1, actions)
	_, err = s.Ingest(ctx, fifth)
	require.NoError(t, err)
	for seq := int64(6); seq <= 9; seq++ {
		_, err = s.Ingest(ctx, event(seq))
		require.NoError(t, err)
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM sanctions`).Scan(&actions))
	require.Zero(t, actions, "sixth event cannot reuse the first five")
	// Crash-equivalent downstream write failure rolls back inbox cursor, samples,
	// strike, sanction and Auth reconciliation intent together.
	_, err = pool.Exec(ctx, `CREATE FUNCTION reject_automod_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected outbox failure'; END $$;
	CREATE TRIGGER reject_automod_event BEFORE INSERT ON automod_event_outbox FOR EACH ROW EXECUTE FUNCTION reject_automod_event()`)
	require.NoError(t, err)
	tenth := event(10)
	_, err = s.Ingest(ctx, tenth)
	require.ErrorContains(t, err, "injected outbox failure")
	require.NoError(t, pool.QueryRow(ctx, `SELECT through_sequence FROM automod_subjects WHERE subject_id=$1`, subject.ID).Scan(&through))
	require.EqualValues(t, 9, through)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM sanctions`).Scan(&actions))
	require.Zero(t, actions)
	_, err = pool.Exec(ctx, `DROP TRIGGER reject_automod_event ON automod_event_outbox`)
	require.NoError(t, err)
	r, err = s.Ingest(ctx, tenth)
	require.NoError(t, err)
	require.EqualValues(t, 10, r.Through)
	_, err = s.Ingest(ctx, tenth)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM sanctions WHERE type='perm_ban' AND issued_by IS NULL AND issuer_service='service:moderation/automod'`).Scan(&actions))
	require.EqualValues(t, 1, actions)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_event_outbox`).Scan(&actions))
	require.EqualValues(t, 1, actions)
	changed := tenth
	changed.Hash = sha256.Sum256([]byte("different immutable bytes"))
	_, err = s.Ingest(ctx, changed)
	require.ErrorIs(t, err, automod.ErrConflict)
	changed = event(11)
	changed.Subject.Epoch = uuid.New()
	_, err = s.Ingest(ctx, changed)
	require.ErrorIs(t, err, automod.ErrConflict)
}

func TestAutoModStore_KeyAliasesAndTerminalFence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	s := automod.NewStore(pool)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, s.Provision(ctx, subject))
	old := automod.Digest{KeyID: "old", Hash: sha256.Sum256([]byte("old digest"))}
	fresh := automod.Digest{KeyID: "new", Hash: sha256.Sum256([]byte("new digest"))}
	for seq := int64(1); seq <= 6; seq++ {
		digests := []automod.Digest{old}
		if seq >= 3 {
			digests = []automod.Digest{old, fresh}
		}
		if seq >= 5 {
			digests = []automod.Digest{fresh}
		}
		_, err := s.Ingest(ctx, automod.Envelope{Subject: subject, EventID: uuid.New(), Sequence: seq, RecordedAt: time.Now().UTC(), Eligibility: automod.Eligible, Digests: digests, Hash: sha256.Sum256([]byte(fmt.Sprint(seq)))})
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_actions`).Scan(&count))
	require.Equal(t, 1, count, "dual HMAC aliases preserve count and strike without double counting")
	require.NoError(t, s.Purge(ctx, subject.ID))
	_, err := s.Ingest(ctx, automod.Envelope{Subject: subject, EventID: uuid.New(), Sequence: 7, RecordedAt: time.Now().UTC(), Eligibility: automod.NoText, Hash: sha256.Sum256([]byte("late"))})
	require.ErrorIs(t, err, automod.ErrPurged)
	require.ErrorIs(t, s.Provision(ctx, subject), automod.ErrPurged)
	for _, table := range []string{"automod_subjects", "automod_inbox", "automod_patterns", "automod_pattern_aliases", "automod_actions", "automod_restriction_sync"} {
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		require.Zero(t, count, table+" must not retain account or digest links after terminal purge")
	}
}

func TestAutoModStore_TerminalFenceRacesFirstProvisionAndRollsBackParticipants(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	s := automod.NewStore(pool)
	for attempt := 0; attempt < 12; attempt++ {
		subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
		start := make(chan struct{})
		provisioned := make(chan error, 1)
		purged := make(chan error, 1)
		go func() { <-start; provisioned <- s.Provision(ctx, subject) }()
		go func() {
			<-start
			err := s.Purge(ctx, subject.ID)
			if errors.Is(err, automod.ErrConflict) {
				err = s.Purge(ctx, subject.ID)
			}
			purged <- err
		}()
		close(start)
		err := <-provisioned
		require.True(t, err == nil || errors.Is(err, automod.ErrPurged), "unexpected provision error: %v", err)
		require.NoError(t, <-purged)
		require.ErrorIs(t, s.Provision(ctx, subject), automod.ErrPurged)
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects WHERE subject_id=$1`, subject.ID).Scan(&count))
		require.Zero(t, count, "terminal import cannot race with first bind to leave raw account links")
	}
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, s.Provision(ctx, subject))
	injected := errors.New("report participant rollback")
	s.BeforeTerminalPurge = func(context.Context, pgx.Tx, uuid.UUID) error { return injected }
	require.ErrorIs(t, s.Purge(ctx, subject.ID), injected)
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_subjects WHERE subject_id=$1) AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject.ID).Scan(&exists))
	require.True(t, exists, "failed participant cannot commit half of the terminal transaction")
	s.BeforeTerminalPurge = nil
	require.NoError(t, s.Purge(ctx, subject.ID))
}
