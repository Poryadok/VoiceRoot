package store

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/space/internal/botmembershipprincipal"
)

func TestNativeBotPublicationOwnJournalAndLegacyIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native Bot publication")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	legacyProfile := uuid.New()
	space, err := st.CreateSpace(ctx, legacyProfile, "Native Bot publication", "", "public")
	require.NoError(t, err)
	db.Run(true, "up")
	op := uuid.New()
	in := botmembershipprincipal.InstallationBinding{OperationID: op, InstallOperationID: op, SpaceID: space.ID, ProfileID: uuid.New(), BotID: uuid.New(), InstallationID: uuid.New(), Generation: 1}
	request := sha256.Sum256([]byte("exact signed AddBotMember request"))
	tx, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer rollbackLifecycleTx(ctx, tx)
	require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space.ID}))
	require.NoError(t, checkMemberAuthorityCatalog(ctx, tx), "exact native42 catalog must qualify all Bot guards before publication")
	require.NoError(t, publishNativeBotInstallation(ctx, tx, in, request[:]))
	authority, err := readOrdinaryMemberAuthority(ctx, tx, space.ID, in.ProfileID)
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE, authority.State)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL, authority.PublicationKind)
	require.Empty(t, authority.RoleReceiptSha256, "Bot installation does not create a human default role")
	require.NoError(t, publishNativeBotInstallation(ctx, tx, in, request[:]), "exact current replay does not create another source")
	changed := request
	changed[0] ^= 1
	require.ErrorIs(t, publishNativeBotInstallation(ctx, tx, in, changed[:]), ErrMemberAdmissionConflict)
	legacy := in
	legacy.OperationID = uuid.New()
	legacy.InstallOperationID = legacy.OperationID
	legacy.BotID = uuid.New()
	legacy.InstallationID = uuid.New()
	legacy.ProfileID = legacyProfile
	require.ErrorIs(t, publishNativeBotInstallation(ctx, tx, legacy, request[:]), ErrMemberAuthorityUnavailable)
	var generation int64
	require.NoError(t, tx.QueryRow(ctx, `SELECT generation FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, legacyProfile).Scan(&generation))
	require.Zero(t, generation, "LEGACY0 never adopted by a fresh installation")
	cancel := in
	cancel.Remove = true
	cancel.OperationID = uuid.New()
	require.NoError(t, cancelNativeBotInstallation(ctx, tx, cancel, request[:]))
	require.ErrorIs(t, publishNativeBotInstallation(ctx, tx, in, request[:]), ErrMemberAdmissionStale, "retired Add cannot acknowledge a current installation")
	next := in
	next.Generation = 2
	next.OperationID = uuid.New()
	next.InstallOperationID = next.OperationID
	next.InstallationID = uuid.New()
	require.NoError(t, publishNativeBotInstallation(ctx, tx, next, request[:]))
	require.NoError(t, cancelNativeBotInstallation(ctx, tx, cancel, request[:]), "old cancellation replay does not remove reinstall")
	authority, err = readOrdinaryMemberAuthority(ctx, tx, space.ID, in.ProfileID)
	require.NoError(t, err)
	require.Equal(t, next.OperationID.String(), authority.GetPublishedBinding().GetOrdinary().GetPublicationIdentity())
	require.NoError(t, tx.Commit(ctx))
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_bot_installation_attempts WHERE space_id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
}

func TestNativeBotCatalog42RejectsCorruption(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native42 catalog")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	cases := []struct{ name, sql string }{
		{"future43", `UPDATE schema_migrations SET version=43`},
		{"dirty", `UPDATE schema_migrations SET dirty=true`},
		{"disabled_head", `ALTER TABLE space_bot_installation_heads DISABLE TRIGGER bot_installation_head_guard`},
		{"disabled_attempt", `ALTER TABLE space_bot_installation_attempts DISABLE TRIGGER bot_installation_attempt_guard`},
		{"disabled_cancel", `ALTER TABLE space_bot_installation_cancellations DISABLE TRIGGER bot_installation_cancellation_guard`},
		{"function_settings", `ALTER FUNCTION space_bot_installation_head_guard() SET search_path=public`},
		{"body", `CREATE OR REPLACE FUNCTION space_bot_installation_head_guard() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END'`},
		{"rls", `ALTER TABLE space_bot_installation_attempts ENABLE ROW LEVEL SECURITY`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tx, err := db.Pool.Begin(ctx)
			require.NoError(t, err)
			defer rollbackLifecycleTx(ctx, tx)
			require.NoError(t, checkMemberAuthorityCatalog(ctx, tx))
			_, err = tx.Exec(ctx, c.sql)
			require.NoError(t, err)
			require.ErrorIs(t, checkMemberAuthorityCatalog(ctx, tx), ErrMemberAuthorityUnavailable)
		})
	}
}
