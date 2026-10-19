package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeMemberCatalogCannotReadTemporaryRetiredSourceShadow(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native source namespaces")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Native source namespace", "", "public")
	require.NoError(t, err)
	profile := uuid.New()
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, profile)
	require.NoError(t, err)
	db.Run(true, "up")
	var original uuid.UUID
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT publication_id FROM public.space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&original))
	_, err = db.Pool.Exec(ctx, `DELETE FROM public.space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, profile)
	require.NoError(t, err)
	var floor uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT retired_through FROM public.space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='RETIRED'`, space.ID, profile).Scan(&floor))
	require.Positive(t, floor)
	tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer rollbackLifecycleTx(ctx, tx)
	require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space.ID}))
	_, err = tx.Exec(ctx, `CREATE TEMP TABLE space_ordinary_member_heads ON COMMIT DROP AS SELECT * FROM public.space_ordinary_member_heads;
 CREATE TEMP TABLE space_members (LIKE public.space_members INCLUDING DEFAULTS) ON COMMIT DROP`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE pg_temp.space_ordinary_member_heads SET generation=0,retired_through=0,publication_id=$3,state='ACTIVE',legacy=true,publication_kind=2,admission_binding_sha256=NULL,role_receipt_sha256=NULL,native_publication_sha256=NULL WHERE space_id=$1 AND profile_id=$2`, space.ID, profile, original)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO pg_temp.space_members(space_id,profile_id,membership_generation,admission_operation_id) VALUES($1,$2,0,$3)`, space.ID, profile, original)
	require.NoError(t, err)
	require.NoError(t, checkMemberAuthorityCatalog(ctx, tx), "public catalog checks alone must not leave temporary relations ahead of public source reads")
	authority, err := readOrdinaryMemberAuthority(ctx, tx, space.ID, profile)
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED, authority.State, "temporary old ACTIVE LEGACY0 must not override the real retirement floor")
	require.Equal(t, floor, authority.RetiredThrough)
	scoped := &SpaceStore{Pool: db.Pool, tx: tx}
	_, err = scoped.ReadScopedMemberSourceBindings(ctx, space.ID, profile)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}
