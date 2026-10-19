package partycoverage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RuntimeBinding is installed outside voice_db by the trusted deployment.
// Never derive AuthorityIncarnation from a restored database or request body.
type RuntimeBinding struct {
	EnvironmentID, AuthorityIncarnation                                         uuid.UUID
	CoverageEpoch                                                               uint64
	DatabaseIdentity, SourceCatalogHash, SchemaContractHash, WriterManifestHash [32]byte
}
type CoverageCut struct {
	CoverageID, AuthorityIncarnation uuid.UUID
	CoverageEpoch, HeadRevision      uint64
	ReceiptHash                      [32]byte
}

func (b RuntimeBinding) Validate() error {
	if b.EnvironmentID == uuid.Nil || b.AuthorityIncarnation == uuid.Nil || b.CoverageEpoch == 0 || b.CoverageEpoch > math.MaxInt64 ||
		b.DatabaseIdentity == [32]byte{} || b.SourceCatalogHash == [32]byte{} || b.SchemaContractHash == [32]byte{} || b.WriterManifestHash == [32]byte{} {
		return ErrUnavailable
	}
	return nil
}

// receiptDocument is canonical JSON produced only by the cutover issuer.
// Exact bytes are retained alongside their hash. Hashes alone are not proofs.
type receiptDocument struct {
	ProtocolVersion                                 int
	CoverageID, OperationID                         uuid.UUID
	Binding                                         RuntimeBinding
	FenceProofHash, DrainProofHash, NativeProofHash [32]byte
}

func canonicalReceipt(d receiptDocument) ([]byte, error) {
	if d.ProtocolVersion != 1 || d.CoverageID == uuid.Nil || d.OperationID == uuid.Nil || d.Binding.Validate() != nil ||
		d.FenceProofHash == [32]byte{} || d.DrainProofHash == [32]byte{} || d.NativeProofHash == [32]byte{} {
		return nil, ErrUnavailable
	}
	return json.Marshal(d)
}

// ReadCoverage reads the ACTIVE receipt in exactly the source-reader snapshot.
// It performs no network IO and takes no row/advisory locks. Callers must use a
// READ ONLY REPEATABLE READ transaction and compare the full cut across passes.
func ReadCoverage(ctx context.Context, tx pgx.Tx, binding RuntimeBinding) (CoverageCut, error) {
	var out CoverageCut
	if tx == nil || binding.Validate() != nil {
		return out, ErrUnavailable
	}
	var readonly, isolation string
	if err := tx.QueryRow(ctx, "SELECT current_setting('transaction_read_only'),current_setting('transaction_isolation')").Scan(&readonly, &isolation); err != nil {
		return out, fmt.Errorf("%w: transaction", ErrUnavailable)
	}
	if readonly != "on" || isolation != "repeatable read" {
		return out, ErrUnavailable
	}
	var raw, hash []byte
	var epoch, revision int64
	var env, inc, op uuid.UUID
	var db, catalog, schema, writer []byte
	err := tx.QueryRow(ctx, `SELECT h.environment_id,h.authority_incarnation,h.coverage_epoch,h.revision,r.coverage_id,r.cutover_operation_id,
 r.voice_database_identity,r.source_catalog_sha256,r.schema_contract_sha256,r.writer_manifest_sha256,r.canonical_receipt_bytes,r.receipt_sha256
 FROM voice_party_coverage_head h JOIN voice_party_coverage_receipts r ON r.coverage_id=h.coverage_id
 WHERE h.singleton AND h.phase='ACTIVE' AND r.environment_id=h.environment_id AND r.authority_incarnation=h.authority_incarnation
 AND r.coverage_epoch=h.coverage_epoch AND r.cutover_operation_id=h.current_operation_id`).Scan(&env, &inc, &epoch, &revision, &out.CoverageID, &op, &db, &catalog, &schema, &writer, &raw, &hash)
	if err != nil {
		return CoverageCut{}, fmt.Errorf("%w: active receipt", ErrUnavailable)
	}
	if env != binding.EnvironmentID || inc != binding.AuthorityIncarnation || epoch <= 0 || uint64(epoch) != binding.CoverageEpoch || revision <= 0 ||
		!bytes.Equal(db, binding.DatabaseIdentity[:]) || !bytes.Equal(catalog, binding.SourceCatalogHash[:]) ||
		!bytes.Equal(schema, binding.SchemaContractHash[:]) || !bytes.Equal(writer, binding.WriterManifestHash[:]) {
		return CoverageCut{}, ErrUnavailable
	}
	sum := sha256.Sum256(raw)
	if len(hash) != 32 || !bytes.Equal(hash, sum[:]) {
		return CoverageCut{}, ErrUnavailable
	}
	var document receiptDocument
	if json.Unmarshal(raw, &document) != nil {
		return CoverageCut{}, ErrUnavailable
	}
	canonical, err := canonicalReceipt(document)
	if err != nil || !bytes.Equal(canonical, raw) || document.Binding != binding || document.CoverageID != out.CoverageID || document.OperationID != op {
		return CoverageCut{}, ErrUnavailable
	}
	out.AuthorityIncarnation = inc
	out.CoverageEpoch = uint64(epoch)
	out.HeadRevision = uint64(revision)
	out.ReceiptHash = sum
	return out, nil
}

// CheckRuntimePrivileges rejects owner/superuser and effective write privileges,
// including inherited grants and the authenticated backend login's authority.
// SET ROLE/SESSION AUTHORIZATION cannot hide administrative credentials. The
// admitted roles must have no callable SECURITY DEFINER functions: even a
// restricted definer can bridge transitively into another definer's authority.
// actual runtime factory must call this before
// admitting any snapshot requests using the installed binding.
func CheckRuntimePrivileges(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return ErrUnavailable
	}
	var unsafe bool
	err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=pg_backend_pid() AND usesysid IS NOT NULL)
 OR EXISTS(SELECT 1 FROM pg_roles a WHERE
 (pg_has_role(current_user,a.oid,'MEMBER') OR pg_has_role(session_user,a.oid,'MEMBER')
 OR pg_has_role((SELECT usesysid FROM pg_stat_activity WHERE pid=pg_backend_pid()),a.oid,'MEMBER')) AND
 (a.rolsuper OR a.rolcreaterole OR a.rolcreatedb OR a.rolreplication OR a.rolbypassrls OR a.rolname IN ('pg_write_server_files','pg_read_server_files','pg_execute_server_program')))
 OR EXISTS(SELECT 1 FROM pg_class c WHERE c.oid IN ('voice_party_coverage_receipts'::regclass,'voice_party_coverage_head'::regclass,'voice_party_coverage_operations'::regclass,'voice_party_coverage_steps'::regclass,
 to_regclass('voice_party_resource_components'),to_regclass('voice_party_resource_retirements'),to_regclass('voice_party_resource_component_links'),
 to_regclass('voice_native_owner_works'),to_regclass('voice_native_owner_targets'),to_regclass('voice_native_owner_acceptances'))
 AND EXISTS(SELECT 1 FROM pg_roles a WHERE
 (pg_has_role(current_user,a.oid,'MEMBER') OR pg_has_role(session_user,a.oid,'MEMBER')
 OR pg_has_role((SELECT usesysid FROM pg_stat_activity WHERE pid=pg_backend_pid()),a.oid,'MEMBER')) AND
 (pg_has_role(a.oid,c.relowner,'MEMBER') OR has_schema_privilege(a.oid,c.relnamespace,'CREATE')
 OR has_table_privilege(a.oid,c.oid,'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
 OR EXISTS(SELECT 1 FROM pg_proc p WHERE p.prosecdef AND has_function_privilege(a.oid,p.oid,'EXECUTE')))))`).Scan(&unsafe)
	if err != nil || unsafe {
		return ErrUnavailable
	}
	return nil
}
