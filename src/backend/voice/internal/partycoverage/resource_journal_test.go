package partycoverage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

func TestProtectedResourceReaderRejectsWriterAndRecoversExactBytes(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000016_party_resource_retirement.up.sql"))
	require.NoError(t, err)
	_, err = c.Exec(ctx, string(up))
	require.NoError(t, err)
	operation := uuid.New()
	b := fixtureBinding()
	plan := cutPlan{operation, b, []byte(`{"fixture":"storage provenance only"}`)}
	_, err = (journal{c}).prepare(ctx, plan)
	require.NoError(t, err)
	// Deliberately a storage fixture, not a complete retirement document. The
	// trusted verifier separately rejects incomplete catalogue/resource evidence.
	d := retirementproof.ResourceDocument{Protocol: 1, ProofID: uuid.New(), Kind: retirementproof.WholeRealmRetired, Cut: retirementproof.CutBinding{CutoverOperationID: operation}}
	componentBytes := []byte(`{"fixture":"not retirement evidence"}`)
	component := retirementproof.ComponentRef{Kind: "DOCKER_FENCE", Protocol: 1, CanonicalBytesHash: sha256.Sum256(componentBytes), JournalRef: uuid.New()}
	d.Components = []retirementproof.ComponentRef{component}
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	hash := retirementproof.Hash(retirementproof.ResourceDomain, raw)
	ref := retirementproof.ProofRef{Protocol: 1, ProofID: d.ProofID, ReceiptHash: hash}
	_, err = c.Exec(ctx, `INSERT INTO voice_party_resource_components VALUES($1,$2,$3,$4,$5)`, component.JournalRef, component.Kind, component.Protocol, componentBytes, component.CanonicalBytesHash[:])
	require.NoError(t, err)
	_, err = c.Exec(ctx, `INSERT INTO voice_party_resource_retirements VALUES($1,1,$2,$3,$4)`, d.ProofID, operation, raw, hash[:])
	require.NoError(t, err)
	_, err = c.Exec(ctx, `INSERT INTO voice_party_resource_component_links VALUES($1,0,$2)`, d.ProofID, component.JournalRef)
	require.NoError(t, err)
	var ownerOID uint32
	require.NoError(t, c.QueryRow(ctx, `SELECT relowner FROM pg_class WHERE oid='voice_party_resource_retirements'::regclass`).Scan(&ownerOID))
	reader, err := NewResourceProofReader(c, schema, ownerOID)
	require.NoError(t, err)
	_, err = reader.ReadResourceProof(ctx, ref)
	require.ErrorIs(t, err, retirementproof.ErrUnavailable, "owner/superuser cannot be maintenance verifier")
	for _, table := range []string{"voice_party_resource_components", "voice_party_resource_retirements", "voice_party_resource_component_links"} {
		_, err = c.Exec(ctx, "DELETE FROM "+table)
		require.Error(t, err)
	}
	_, err = c.Exec(ctx, `UPDATE voice_party_resource_retirements SET protocol=protocol`)
	require.Error(t, err)
	_, err = c.Exec(ctx, `TRUNCATE voice_party_resource_components,voice_party_resource_retirements,voice_party_resource_component_links`)
	require.Error(t, err)
	down, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000016_party_resource_retirement.down.sql"))
	require.NoError(t, err)
	_, err = c.Exec(ctx, string(down))
	require.Error(t, err, "retained journal cannot be rolled back")
	_, err = c.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	role := "proof_reader_" + uuid.NewString()[:8]
	_, err = c.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		c.Exec(context.Background(), "RESET ROLE")
		c.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		c.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	_, err = c.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	got, err := reader.ReadResourceProof(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, raw, got.CanonicalBytes)
	require.Equal(t, componentBytes, got.Components[0].CanonicalBytes)
	wrongOwner, err := NewResourceProofReader(c, schema, ownerOID+1)
	require.NoError(t, err)
	_, err = wrongOwner.ReadResourceProof(ctx, ref)
	require.ErrorIs(t, err, retirementproof.ErrUnavailable)
	wrong := ref
	wrong.ReceiptHash[0]++
	_, err = reader.ReadResourceProof(ctx, wrong)
	require.Error(t, err)
	wrong = ref
	wrong.ProofID = uuid.New()
	_, err = reader.ReadResourceProof(ctx, wrong)
	require.Error(t, err)
	_, err = c.Exec(ctx, "RESET ROLE")
	require.NoError(t, err)
	// A callable issuer-owned SECURITY DEFINER function would be an alternate
	// authority path even when direct INSERT is absent.
	_, err = c.Exec(ctx, `CREATE FUNCTION proof_privilege_bridge() RETURNS int LANGUAGE SQL SECURITY DEFINER AS 'SELECT 1'`)
	require.NoError(t, err)
	_, err = c.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = reader.ReadResourceProof(ctx, ref)
	require.ErrorIs(t, err, retirementproof.ErrUnavailable, "definer privilege bridge must not authenticate proof")
	_, err = c.Exec(ctx, "RESET ROLE")
	require.NoError(t, err)
	_, err = c.Exec(ctx, `DROP FUNCTION proof_privilege_bridge()`)
	require.NoError(t, err)
	_, err = c.Exec(ctx, "GRANT INSERT ON voice_party_resource_components TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = reader.ReadResourceProof(ctx, ref)
	require.ErrorIs(t, err, retirementproof.ErrUnavailable, "read/write credential cannot authenticate protected evidence")
	tx, err := c.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.ErrorIs(t, CheckRuntimePrivileges(ctx, tx), ErrUnavailable, "serving startup must reject issuer-table write authority")
}

func TestRuntimeRejectsMaintenanceOnlyNativeOwnerJournalWrites(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	// Privilege-contract fixtures only: actual017 schema/owner transitions belong
	// to160. Serving must reject writes even before that source is integrated.
	tables := []string{"voice_native_owner_works", "voice_native_owner_targets", "voice_native_owner_acceptances"}
	for _, table := range tables {
		_, err := c.Exec(ctx, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+"(fixture int)")
		require.NoError(t, err)
	}
	role := "native_guard_" + uuid.NewString()[:8]
	_, err := c.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		c.Exec(context.Background(), "ROLLBACK")
		c.Exec(context.Background(), "RESET ROLE")
		c.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		c.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	_, err = c.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	check := func() error {
		_, err := c.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
		require.NoError(t, err)
		tx, err := c.Begin(ctx)
		require.NoError(t, err)
		result := CheckRuntimePrivileges(ctx, tx)
		require.NoError(t, tx.Rollback(ctx))
		_, err = c.Exec(ctx, "RESET ROLE")
		require.NoError(t, err)
		return result
	}
	require.NoError(t, check())
	for _, table := range tables {
		_, err = c.Exec(ctx, "GRANT INSERT ON "+pgx.Identifier{table}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
		require.NoError(t, err)
		require.ErrorIs(t, check(), ErrUnavailable, "serving may not mint native owner work/acceptance")
		_, err = c.Exec(ctx, "REVOKE INSERT ON "+pgx.Identifier{table}.Sanitize()+" FROM "+pgx.Identifier{role}.Sanitize())
		require.NoError(t, err)
	}
	_, err = c.Exec(ctx, `CREATE FUNCTION native_owner_privilege_bridge() RETURNS int LANGUAGE SQL SECURITY DEFINER AS 'SELECT 1'`)
	require.NoError(t, err)
	require.ErrorIs(t, check(), ErrUnavailable, "callable private-journal owner bridge is unsafe")
	_, err = c.Exec(ctx, `DROP FUNCTION native_owner_privilege_bridge()`)
	require.NoError(t, err)
	group := "native_journal_writer_" + uuid.NewString()[:8]
	_, err = c.Exec(ctx, "CREATE ROLE "+pgx.Identifier{group}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		c.Exec(context.Background(), "RESET ROLE")
		c.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{group}.Sanitize())
		c.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{group}.Sanitize())
	})
	_, err = c.Exec(ctx, "GRANT INSERT ON "+pgx.Identifier{tables[0]}.Sanitize()+" TO "+pgx.Identifier{group}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "ALTER ROLE "+pgx.Identifier{role}.Sanitize()+" NOINHERIT")
	require.NoError(t, err)
	_, err = c.Exec(ctx, "GRANT "+pgx.Identifier{group}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	require.ErrorIs(t, check(), ErrUnavailable, "SET ROLE route to non-inherited journal writer is unsafe")
}
