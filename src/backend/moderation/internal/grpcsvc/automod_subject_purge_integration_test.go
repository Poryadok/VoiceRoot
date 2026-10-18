package grpcsvc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
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
