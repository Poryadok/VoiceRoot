package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeAuthorityCurrent39ReadAndPreparing(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL current native migration")
	}
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "goto", "34")
	st := &SpaceStore{Pool: database.Pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Current native authority", "", "public")
	require.NoError(t, err)
	// Retain the historical exact41 qualification oracle independently of the
	// new native42 Bot cohort, whose own positives/negatives run separately.
	database.Run(true, "goto", "41")
	request := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: owner.String()}
	first, err := st.ReadMemberAdmissionAuthority(ctx, request)
	require.NoError(t, err, "actual current39 source must qualify; no historical version relabeling")
	require.Zero(t, first.GetLifecycle().GetGeneration())
	require.Equal(t, spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE, first.GetLifecycle().GetState())
	require.True(t, first.GetOrdinary().GetExplicitLegacy())
	require.Positive(t, first.GetSpaceAuthorityRevision())
	input := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	intent, err := st.PrepareMemberAdmission(ctx, input)
	require.NoError(t, err)
	request.ProfileId = input.ProfileID.String()
	request.ExpectedGeneration = &intent.Generation
	operation := input.OperationID.String()
	request.ExpectedAdmissionOperationId = &operation
	preparing, err := st.ReadMemberAdmissionAuthority(ctx, request)
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING, preparing.GetOrdinary().GetState())
	require.Equal(t, intent.BindingSHA256, preparing.GetOrdinary().GetPreparingAdmissionBindingSha256())
	require.Nil(t, preparing.GetOrdinary().GetPublishedBinding())
	require.Empty(t, preparing.GetOrdinary().GetRoleReceiptSha256())
	var members int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1`, space.ID).Scan(&members))
	require.Equal(t, 1, members, "PREPARING has no membership authority")
	var terminalDDL, terminalBody string
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT pg_get_functiondef(oid),prosrc FROM pg_proc WHERE oid='public.space_terminal_purge_ready(uuid)'::regprocedure`).Scan(&terminalDDL, &terminalBody))
	t.Logf("canonical current11 terminal prosrc SHA256=%x", sha256.Sum256([]byte(terminalBody)))
	cases := []struct{ name, mutate, restore string }{
		{"dirty", `UPDATE schema_migrations SET dirty=true`, `UPDATE schema_migrations SET dirty=false`},
		{"future", `UPDATE schema_migrations SET version=43`, `UPDATE schema_migrations SET version=41`},
		{"historical", `UPDATE schema_migrations SET version=34`, `UPDATE schema_migrations SET version=41`},
		{"disabled_guard", `ALTER TABLE space_members DISABLE TRIGGER space_ordinary_publication_guard`, `ALTER TABLE space_members ENABLE TRIGGER space_ordinary_publication_guard`},
		{"row_security", `ALTER TABLE space_ordinary_member_heads ENABLE ROW LEVEL SECURITY`, `ALTER TABLE space_ordinary_member_heads DISABLE ROW LEVEL SECURITY`},
		{"missing_private_journal", `ALTER TABLE space_join_operations RENAME TO missing_join_operations`, `ALTER TABLE missing_join_operations RENAME TO space_join_operations`},
		{"missing_effective_ledger", `ALTER TABLE space_effective_member_heads RENAME TO missing_effective_heads`, `ALTER TABLE missing_effective_heads RENAME TO space_effective_member_heads`},
		{"disabled_effective_liveness", `ALTER TABLE space_effective_member_heads DISABLE TRIGGER effective_member_liveness_guard`, `ALTER TABLE space_effective_member_heads ENABLE TRIGGER effective_member_liveness_guard`},
		{"effective_rls", `ALTER TABLE space_effective_member_heads ENABLE ROW LEVEL SECURITY`, `ALTER TABLE space_effective_member_heads DISABLE ROW LEVEL SECURITY`},
		{"effective_function_settings", `ALTER FUNCTION space_effective_member_head_guard() SET search_path=pg_catalog`, `ALTER FUNCTION space_effective_member_head_guard() RESET ALL`},
		{"corrupt_terminal_predicate", `CREATE OR REPLACE FUNCTION space_terminal_purge_ready(target uuid) RETURNS boolean LANGUAGE sql STABLE AS 'SELECT true'`, terminalDDL},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := database.Pool.Exec(ctx, test.mutate)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := database.Pool.Exec(ctx, test.restore)
				require.NoError(t, err)
			})
			_, err = st.ReadMemberAdmissionAuthority(ctx, request)
			require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
			_, err = st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()})
			require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, fmt.Sprintf("writer must reject %s before allocating", test.name))
		})
	}
}
