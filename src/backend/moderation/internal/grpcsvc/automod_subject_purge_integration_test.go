package grpcsvc

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"time"
	"voice/backend/moderation/internal/automod"
)

func TestAutoModStore_SubjectParticipantRollsBackWithAndWithoutAccountMapping(t *testing.T) {
	for _, mapped := range []bool{false, true} {
		t.Run(fmt.Sprint(mapped), func(t *testing.T) {
			ctx := context.Background()
			pool := automodAdmissionDatabase(t)
			integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
			s := automod.NewStore(pool)
			subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
			if mapped {
				require.NoError(t, s.Provision(ctx, subject))
			}
			_, err := pool.Exec(ctx, `CREATE TABLE subject_participant(subject_id uuid PRIMARY KEY); INSERT INTO subject_participant VALUES('`+subject.ID.String()+`')`)
			require.NoError(t, err)
			injected := errors.New("subject participant unavailable")
			fail := true
			accountCalls := 0
			s.BeforeTerminalPurge = func(context.Context, pgx.Tx, uuid.UUID) error { accountCalls++; return nil }
			s.BeforeTerminalSubjectPurge = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
				require.Equal(t, subject.ID, id)
				var terminal bool
				require.NoError(t, tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, id).Scan(&terminal))
				require.True(t, terminal)
				_, err := tx.Exec(ctx, `DELETE FROM subject_participant WHERE subject_id=$1`, id)
				if err != nil {
					return err
				}
				if fail {
					return injected
				}
				return nil
			}
			require.ErrorIs(t, s.Purge(ctx, subject.ID), injected)
			var retained bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM subject_participant WHERE subject_id=$1) AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject.ID).Scan(&retained))
			require.True(t, retained)
			fail = false
			require.NoError(t, s.Purge(ctx, subject.ID))
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subject_participant`).Scan(&count))
			require.Zero(t, count)
			if mapped {
				require.Equal(t, 2, accountCalls)
			} else {
				require.Zero(t, accountCalls, "unknown subject never fabricates account participant authority")
			}
		})
	}
}

func TestAutoModStore_ClosedBootBeforeCommitRollsBackEvidenceEffects(t *testing.T) {
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	s := automod.NewStore(pool)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, s.Provision(ctx, subject))
	ready := func() *automod.BootGate {
		g := automod.NewBootGate()
		id, generation := g.Identity()
		for phase := automod.BootLocalIssuer; phase <= automod.BootEffectsReady; phase++ {
			require.NoError(t, g.Advance(automod.BootReceipt{BootID: id, Generation: generation, Phase: phase, ProofID: uuid.New()}))
		}
		return g
	}
	g := ready()
	check, err := g.AcquireEffects(ctx)
	require.NoError(t, err)
	calls := 0
	hash := sha256.Sum256([]byte("immutable account detector core"))
	e := automod.Envelope{Subject: subject, EventID: uuid.New(), Sequence: 1, RecordedAt: time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond), Eligibility: automod.Eligible, Digests: []automod.Digest{{KeyID: "current", Hash: hash}}, LinkCount: 3, Hash: hash}
	e.BeforeCommit = func(ctx context.Context) error {
		calls++
		if calls == 3 {
			g.Close()
		}
		return check(ctx)
	}
	_, err = s.Ingest(ctx, e)
	require.ErrorIs(t, err, automod.ErrBootClosed)
	require.Equal(t, 3, calls)
	var sequence, actions, inbox int
	require.NoError(t, pool.QueryRow(ctx, `SELECT through_sequence FROM automod_subjects WHERE subject_id=$1`, subject.ID).Scan(&sequence))
	require.Zero(t, sequence)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_actions`).Scan(&actions))
	require.Zero(t, actions)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_inbox`).Scan(&inbox))
	require.Zero(t, inbox)
	newBoot := ready()
	_, err = s.Ingest(ctx, e)
	require.ErrorIs(t, err, automod.ErrBootClosed, "old captured check remains invalid after another boot is ready")
	e.BeforeCommit, err = newBoot.AcquireEffects(ctx)
	require.NoError(t, err)
	receipt, err := s.Ingest(ctx, e)
	require.NoError(t, err)
	require.Equal(t, int64(1), receipt.Through)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_actions`).Scan(&actions))
	require.Equal(t, 1, actions)
}
