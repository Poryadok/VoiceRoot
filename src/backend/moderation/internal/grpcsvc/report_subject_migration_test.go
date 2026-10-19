package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"voice/backend/moderation/internal/automod"
)

func TestReportSubjectMigrationRollback(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	root := filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db")
	down, err := os.ReadFile(filepath.Join(root, "000008_report_subject_provenance.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err, "empty expand can roll back")
	integrationtestApplySQL(t, ctx, pool, filepath.Join(root, "000008_report_subject_provenance.up.sql"))
	reports := newReportFixtureStore(pool)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	check, _ := reportOpenPrivacy(ctx)
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), subject, check, "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "rollback cannot erase admitted subject provenance")
	var stored uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT reporter_subject_id FROM reports WHERE id=$1`, row.ID).Scan(&stored))
	require.Equal(t, subject.ID, stored)
}
