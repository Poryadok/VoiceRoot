package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
)

func TestMemberAdmissionOutcomeRemainsHistoricalAfterLeave(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL retained admission outcome")
	}
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "goto", "34")
	st := &SpaceStore{Pool: database.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Saved admission outcome", "", "public")
	require.NoError(t, err)
	database.Run(true, "up")
	in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	intent, err := st.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	before, saved, err := st.ReadMemberAdmissionOutcome(ctx, in.OperationID)
	require.NoError(t, err)
	require.False(t, saved)
	require.Nil(t, before)
	// Controlled local receipt exercises stored outcome semantics only. Genuine
	// RolePG/protected transport publication is a separate cross-owner proof.
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), role.String())
	require.NoError(t, err)
	published, err := st.PublishMemberAdmission(ctx, in.OperationID, role, receipt[:], nil)
	require.NoError(t, err)
	require.NoError(t, st.RemoveMember(ctx, space.ID, in.ProfileID))
	active, err := st.IsSpaceMember(ctx, space.ID, in.ProfileID)
	require.NoError(t, err)
	require.False(t, active)
	var revision uint64
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT access_epoch FROM space_voice_access_epochs WHERE space_id=$1`, space.ID).Scan(&revision))
	outcome, saved, err := st.ReadMemberAdmissionOutcome(ctx, in.OperationID)
	require.NoError(t, err)
	require.True(t, saved)
	require.Equal(t, published, outcome)
	var after uint64
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT access_epoch FROM space_voice_access_epochs WHERE space_id=$1`, space.ID).Scan(&after))
	require.Equal(t, revision, after, "outcome replay creates no source/count/event mutation")
	active, err = st.IsSpaceMember(ctx, space.ID, in.ProfileID)
	require.NoError(t, err)
	require.False(t, active)
	unknown, saved, err := st.ReadMemberAdmissionOutcome(ctx, uuid.New())
	require.NoError(t, err)
	require.False(t, saved)
	require.Nil(t, unknown)
}
