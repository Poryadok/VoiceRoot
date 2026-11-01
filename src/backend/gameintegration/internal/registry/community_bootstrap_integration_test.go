package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCommunityBootstrapOwnerApprovalIsDurableExactAndExpiring(t *testing.T) {
	store, ctx := startT32SessionStore(t)
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000021_t37_community_bootstrap.up.sql"))
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	recoveryMigration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000022_t37_owner_recovery.up.sql"))
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, string(recoveryMigration))
	require.NoError(t, err)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	var ownerAccount uuid.UUID
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT owner_account_id FROM applications WHERE id=$1`, principal.ApplicationID).Scan(&ownerAccount))
	ownerProfile, operator, secondOperator := uuid.New(), uuid.New(), uuid.New()
	operationID := uuid.New()
	in := OpenCommunityBootstrapInput{
		OperationID: operationID, ApplicationID: principal.ApplicationID, EnvironmentID: principal.EnvironmentID,
		OwnerAccountID: ownerAccount, OwnerProfileID: ownerProfile, CorporationKey: "corp-7", TemplateID: "guild-default",
	}
	op, err := store.OpenCommunityBootstrap(ctx, in)
	require.NoError(t, err)
	require.Equal(t, "pending", op.Status)
	require.Nil(t, op.OperatorID, "opening a request has no product side effect or implicit approval")
	replay, err := (&Store{Pool: store.Pool}).OpenCommunityBootstrap(ctx, in)
	require.NoError(t, err)
	require.Equal(t, op, replay, "restart/retry returns the same pending operation")
	changed := in
	changed.TemplateID = "other-template"
	_, err = store.OpenCommunityBootstrap(ctx, changed)
	require.ErrorIs(t, err, ErrCommunityBootstrapConflict)
	wrongOwner := in
	wrongOwner.OperationID = uuid.New()
	wrongOwner.OwnerAccountID = uuid.New()
	_, err = store.OpenCommunityBootstrap(ctx, wrongOwner)
	require.ErrorIs(t, err, ErrInvalidApplication)
	_, err = store.ApproveCommunityBootstrap(ctx, principal.ApplicationID, operationID, ownerAccount)
	require.ErrorIs(t, err, ErrInvalidApplication, "the human Owner cannot self-approve")

	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }
	approved, err := store.ApproveCommunityBootstrap(ctx, principal.ApplicationID, operationID, operator)
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Status)
	require.Equal(t, operator, *approved.OperatorID)
	require.True(t, now.Add(15*time.Minute).Equal(*approved.ExpiresAt))
	approvedReplay, err := (&Store{Pool: store.Pool, Now: func() time.Time { return now }}).ApproveCommunityBootstrap(ctx, principal.ApplicationID, operationID, operator)
	require.NoError(t, err)
	require.Equal(t, approved, approvedReplay, "approval replay does not extend its expiry")
	_, err = store.ApproveCommunityBootstrap(ctx, principal.ApplicationID, operationID, secondOperator)
	require.ErrorIs(t, err, ErrCommunityBootstrapConflict, "a different operator cannot overwrite a live approval")

	now = now.Add(16 * time.Minute)
	reapproved, err := store.ApproveCommunityBootstrap(ctx, principal.ApplicationID, operationID, secondOperator)
	require.NoError(t, err, "expired approval may be renewed by a separate allowlisted operator")
	require.Equal(t, secondOperator, *reapproved.OperatorID)
	require.True(t, now.Add(15*time.Minute).Equal(*reapproved.ExpiresAt))
	spaceID := uuid.New()
	completed, err := store.CompleteCommunityBootstrap(ctx, operationID, ownerAccount, spaceID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", completed.Status)
	require.Equal(t, spaceID, *completed.SpaceID)
	receiptReplay, err := (&Store{Pool: store.Pool}).CompleteCommunityBootstrap(ctx, operationID, ownerAccount, spaceID)
	require.NoError(t, err)
	require.Equal(t, completed, receiptReplay, "exact receipt retry returns the saved Space identity")
	_, err = store.CompleteCommunityBootstrap(ctx, operationID, ownerAccount, uuid.New())
	require.ErrorIs(t, err, ErrCommunityBootstrapConflict, "one operation cannot reconcile to two Spaces")
}

func TestCommunityBootstrapRequiresActiveSandboxAndUniqueCorporation(t *testing.T) {
	store, ctx := startT32SessionStore(t)
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000021_t37_community_bootstrap.up.sql"))
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	recoveryMigration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000022_t37_owner_recovery.up.sql"))
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, string(recoveryMigration))
	require.NoError(t, err)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	var ownerAccount uuid.UUID
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT owner_account_id FROM applications WHERE id=$1`, principal.ApplicationID).Scan(&ownerAccount))
	input := OpenCommunityBootstrapInput{OperationID: uuid.New(), ApplicationID: principal.ApplicationID, EnvironmentID: principal.EnvironmentID, OwnerAccountID: ownerAccount, OwnerProfileID: uuid.New(), CorporationKey: "corp-7", TemplateID: "guild-default"}
	first, err := store.OpenCommunityBootstrap(ctx, input)
	require.NoError(t, err)
	now := time.Now().UTC()
	store.Now = func() time.Time { return now }
	_, err = store.ApproveCommunityBootstrap(ctx, principal.ApplicationID, input.OperationID, uuid.New())
	require.NoError(t, err)
	now = now.Add(16 * time.Minute)
	_, err = store.CompleteCommunityBootstrap(ctx, input.OperationID, ownerAccount, uuid.New())
	require.ErrorIs(t, err, ErrCommunityBootstrapExpired, "expired approval cannot persist a Space receipt")
	changedID := input
	changedID.OperationID = uuid.New()
	_, err = store.OpenCommunityBootstrap(ctx, changedID)
	require.ErrorIs(t, err, ErrCommunityBootstrapConflict, "one application/environment can bind a corporation only once")
	_, err = store.Pool.Exec(ctx, `UPDATE environments SET status='suspended' WHERE id=$1`, principal.EnvironmentID)
	require.NoError(t, err)
	changedID.CorporationKey = "corp-8"
	_, err = store.OpenCommunityBootstrap(ctx, changedID)
	require.ErrorIs(t, err, ErrInvalidApplication, "suspended sandbox cannot open a community bootstrap")
	var count int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_community_bootstrap_operations WHERE operation_id=$1`, first.OperationID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestCommunityOwnerRecoveryFreezesReopensAndFencesStaleGenerations(t *testing.T) {
	store, ctx := startT32SessionStore(t)
	installT37CommunitySchema(t, store, ctx)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	var ownerAccount uuid.UUID
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT owner_account_id FROM applications WHERE id=$1`, principal.ApplicationID).Scan(&ownerAccount))
	ownerProfile, replacement, replacementProfile := uuid.New(), uuid.New(), uuid.New()
	bootstrapID, spaceID := uuid.New(), uuid.New()
	_, err := store.OpenCommunityBootstrap(ctx, OpenCommunityBootstrapInput{OperationID: bootstrapID, ApplicationID: principal.ApplicationID, EnvironmentID: principal.EnvironmentID, OwnerAccountID: ownerAccount, OwnerProfileID: ownerProfile, CorporationKey: "corp-recovery", TemplateID: "guild-default"})
	require.NoError(t, err)
	store.Now = func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) }
	_, err = store.ApproveCommunityBootstrap(ctx, principal.ApplicationID, bootstrapID, uuid.New())
	require.NoError(t, err)
	_, err = store.CompleteCommunityBootstrap(ctx, bootstrapID, ownerAccount, spaceID)
	require.NoError(t, err)
	grantBinding := uuid.New()
	insertCommunityRosterBinding(t, ctx, store, principal, grantBinding, "character-owner")
	_, err = store.ApplyCommunityRosterSnapshot(ctx, principal, testCommunityRosterInput(t, "corp-recovery", 1, 1,
		CommunityRosterMember{BindingID: grantBinding, CharacterKey: "character-owner", SourceMembershipRevision: 1,
			SourceRank: "officer", GrantReasons: []string{"membership", "officer_rank"}},
	))
	require.NoError(t, err)

	evidence := make([]byte, 32)
	for i := range evidence {
		evidence[i] = byte(i + 1)
	}
	firstID := uuid.New()
	first := OpenOwnerRecoveryInput{OperationID: firstID, ApplicationID: principal.ApplicationID, EnvironmentID: principal.EnvironmentID, CorporationKey: "corp-recovery", ReplacementAccountID: replacement, ReplacementProfileID: replacementProfile, ExpectedGeneration: 1, ReasonCode: "owner_lost", EvidenceSHA256: evidence}
	opened, err := store.OpenCommunityOwnerRecovery(ctx, first)
	require.NoError(t, err)
	require.Equal(t, "pending", opened.Status)
	replay, err := (&Store{Pool: store.Pool}).OpenCommunityOwnerRecovery(ctx, first)
	require.NoError(t, err)
	require.Equal(t, opened, replay, "a restart retry returns the exact frozen recovery operation")
	var authority OwnerAuthority
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT owner_account_id,owner_profile_id,owner_generation,status,space_id FROM gis_community_owner_authority WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3`, principal.ApplicationID, principal.EnvironmentID, "corp-recovery").Scan(&authority.OwnerAccountID, &authority.OwnerProfileID, &authority.Generation, &authority.Status, &authority.SpaceID))
	require.Equal(t, "recovery_pending", authority.Status, "opening recovery freezes the current authority generation")
	_, err = store.OpenCommunityBootstrap(ctx, OpenCommunityBootstrapInput{OperationID: uuid.New(), ApplicationID: principal.ApplicationID, EnvironmentID: principal.EnvironmentID, OwnerAccountID: ownerAccount, OwnerProfileID: ownerProfile, CorporationKey: "corp-recovery", TemplateID: "guild-default"})
	require.ErrorIs(t, err, ErrInvalidApplication, "old Owner cannot reopen GIS writes during recovery")

	// Parent suspension must not disable protected security cancellation or
	// recovery of this nonempty community; ordinary bootstrap has its own gate.
	_, err = store.Pool.Exec(ctx, `UPDATE applications SET status='suspended',revision=revision+1 WHERE id=$1`, principal.ApplicationID)
	require.NoError(t, err)
	operator, secondOperator := uuid.New(), uuid.New()
	cancelled, err := store.CancelCommunityOwnerRecovery(ctx, principal.ApplicationID, firstID, operator)
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelled.Status)
	require.Equal(t, operator, *cancelled.CancelledBy)
	_, err = store.OpenCommunityOwnerRecovery(ctx, first)
	require.NoError(t, err, "exact replay of a cancelled op returns its durable cancelled receipt")

	secondID := uuid.New()
	second := first
	second.OperationID = secondID
	second.ExpectedGeneration = 1
	opened, err = store.OpenCommunityOwnerRecovery(ctx, second)
	require.NoError(t, err, "cancellation restores the old generation so a new request can open")
	store.Now = func() time.Time { return time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC) }
	_, err = store.ApproveCommunityOwnerRecovery(ctx, principal.ApplicationID, secondID, replacement)
	require.ErrorIs(t, err, ErrOwnerRecoveryConflict, "replacement cannot approve itself")
	approved, err := store.ApproveCommunityOwnerRecovery(ctx, principal.ApplicationID, secondID, operator)
	require.NoError(t, err)
	require.Equal(t, operator, *approved.OperatorID)
	_, err = store.ApproveCommunityOwnerRecovery(ctx, principal.ApplicationID, secondID, secondOperator)
	require.ErrorIs(t, err, ErrOwnerRecoveryConflict, "a live approval cannot be replaced by another operator")
	completed, err := store.CompleteCommunityOwnerRecovery(ctx, secondID, replacement, replacementProfile, spaceID, 2)
	require.NoError(t, err)
	require.Equal(t, "succeeded", completed.Status)
	require.Equal(t, int64(2), *completed.ResultGeneration)
	var oldGenerationRevocations int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_community_grants
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key='corp-recovery' AND owner_generation=1
		AND revoked_at IS NOT NULL AND revocation_reason='owner_generation_changed'`,
		principal.ApplicationID, principal.EnvironmentID).Scan(&oldGenerationRevocations))
	require.Equal(t, 2, oldGenerationRevocations, "owner takeover durably revokes every old-generation reason")
	require.ErrorIs(t, func() error { _, err := store.GetCommunityRosterSnapshot(ctx, principal, "corp-recovery"); return err }(),
		ErrStaleCommunityRoster, "new Owner generation must publish a fresh complete roster")
	_, err = store.OpenCommunityOwnerRecovery(ctx, second)
	require.ErrorIs(t, err, ErrOwnerRecoveryConflict, "stale generation cannot begin a second takeover after completion")
	_, err = store.ApproveCommunityOwnerRecovery(ctx, principal.ApplicationID, secondID, operator)
	require.ErrorIs(t, err, ErrOwnerRecoveryConflict, "old approval cannot act after authority generation advanced")
	_, err = store.CompleteCommunityOwnerRecovery(ctx, secondID, ownerAccount, ownerProfile, spaceID, 2)
	require.ErrorIs(t, err, ErrOwnerRecoveryConflict, "stale old Owner identity cannot complete takeover")
	var current OwnerAuthority
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT owner_account_id,owner_profile_id,owner_generation,status,space_id FROM gis_community_owner_authority WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3`, principal.ApplicationID, principal.EnvironmentID, "corp-recovery").Scan(&current.OwnerAccountID, &current.OwnerProfileID, &current.Generation, &current.Status, &current.SpaceID))
	require.Equal(t, replacement, current.OwnerAccountID)
	require.Equal(t, replacementProfile, current.OwnerProfileID)
	require.Equal(t, int64(2), current.Generation)
	require.Equal(t, "active", current.Status)
}

func installT37CommunitySchema(t *testing.T, store *Store, ctx context.Context) {
	t.Helper()
	for _, name := range []string{"000021_t37_community_bootstrap.up.sql", "000022_t37_owner_recovery.up.sql", "000023_t38_community_roster.up.sql", "000024_t38_community_roster_space_receipt.up.sql"} {
		migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", name))
		require.NoError(t, err)
		_, err = store.Pool.Exec(ctx, string(migration))
		require.NoError(t, err)
	}
}
