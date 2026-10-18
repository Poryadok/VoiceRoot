package partycoverage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/retirementproof"
)

type resourceReadDB interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// ResourceProofReader resolves refs only from the installed protected issuer
// journal. Schema and owner OID come from trusted installation provisioning,
// independently of uploaded receipt bytes. This reader grants no mint authority.
type ResourceProofReader struct {
	db       resourceReadDB
	schema   string
	ownerOID uint32
}

func NewResourceProofReader(db resourceReadDB, schema string, ownerOID uint32) (*ResourceProofReader, error) {
	if db == nil || schema == "" || ownerOID == 0 {
		return nil, retirementproof.ErrUnavailable
	}
	return &ResourceProofReader{db, schema, ownerOID}, nil
}
func (r *ResourceProofReader) ReadResourceProof(ctx context.Context, ref retirementproof.ProofRef) (retirementproof.ResourceRecord, error) {
	var out retirementproof.ResourceRecord
	if r == nil || r.db == nil || ref.Protocol != retirementproof.ProtocolV1 || ref.ProofID == ([16]byte{}) || ref.ReceiptHash == (retirementproof.Digest{}) {
		return out, retirementproof.ErrUnavailable
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = r.checkReader(ctx, tx); err != nil {
		return out, err
	}
	var protocol int64
	var operation [16]byte
	var raw, hash []byte
	err = tx.QueryRow(ctx, `SELECT protocol,cutover_operation_id,canonical_receipt_bytes,receipt_sha256 FROM `+r.table("voice_party_resource_retirements")+` WHERE proof_id=$1`, ref.ProofID).Scan(&protocol, &operation, &raw, &hash)
	if err != nil {
		return out, fmt.Errorf("%w: protected receipt", retirementproof.ErrUnavailable)
	}
	if protocol != int64(ref.Protocol) || !bytes.Equal(hash, ref.ReceiptHash[:]) || retirementproof.Hash(retirementproof.ResourceDomain, raw) != ref.ReceiptHash || json.Unmarshal(raw, &out.Document) != nil {
		return out, retirementproof.ErrConflict
	}
	canonical, err := json.Marshal(out.Document)
	if err != nil || !bytes.Equal(canonical, raw) || out.Document.Protocol != ref.Protocol || out.Document.ProofID != ref.ProofID || out.Document.Cut.CutoverOperationID != operation {
		return out, retirementproof.ErrConflict
	}
	rows, err := tx.Query(ctx, `SELECT l.ordinal,c.journal_ref,c.kind,c.protocol,c.canonical_bytes,c.canonical_bytes_sha256 FROM `+r.table("voice_party_resource_component_links")+` l JOIN `+r.table("voice_party_resource_components")+` c ON c.journal_ref=l.journal_ref WHERE l.proof_id=$1 ORDER BY l.ordinal`, ref.ProofID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var ordinal, p int64
		var c retirementproof.ComponentRecord
		var h []byte
		if err = rows.Scan(&ordinal, &c.Ref.JournalRef, &c.Ref.Kind, &p, &c.CanonicalBytes, &h); err != nil {
			rows.Close()
			return out, err
		}
		if ordinal != int64(len(out.Components)) || p <= 0 || p > math.MaxUint32 || len(h) != 32 || len(c.CanonicalBytes) == 0 {
			rows.Close()
			return out, retirementproof.ErrConflict
		}
		c.Ref.Protocol = uint32(p)
		copy(c.Ref.CanonicalBytesHash[:], h)
		if sha256.Sum256(c.CanonicalBytes) != c.Ref.CanonicalBytesHash || ordinal >= int64(len(out.Document.Components)) || c.Ref != out.Document.Components[ordinal] {
			rows.Close()
			return out, retirementproof.ErrConflict
		}
		out.Components = append(out.Components, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(out.Components) != len(out.Document.Components) {
		return out, retirementproof.ErrConflict
	}
	out.Ref = ref
	out.CanonicalBytes = raw
	if err = tx.Commit(ctx); err != nil {
		return retirementproof.ResourceRecord{}, err
	}
	return out, nil
}

func (r *ResourceProofReader) table(name string) string {
	return pgx.Identifier{r.schema, name}.Sanitize()
}

func (r *ResourceProofReader) checkReader(ctx context.Context, tx pgx.Tx) error {
	var count int
	var unsafe bool
	err := tx.QueryRow(ctx, `SELECT count(*),coalesce(bool_or(c.relkind<>'r' OR c.relowner<>$2::oid
 OR NOT has_table_privilege(current_user,c.oid,'SELECT')
 OR EXISTS(SELECT 1 FROM pg_roles a WHERE pg_has_role(current_user,a.oid,'MEMBER') AND
 (a.rolsuper OR a.rolcreaterole OR a.rolcreatedb OR a.rolreplication OR a.rolbypassrls
 OR a.rolname IN ('pg_write_server_files','pg_read_server_files','pg_execute_server_program')
 OR pg_has_role(a.oid,c.relowner,'MEMBER') OR has_schema_privilege(a.oid,c.relnamespace,'CREATE')
 OR has_table_privilege(a.oid,c.oid,'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
 OR EXISTS(SELECT 1 FROM pg_proc p WHERE p.prosecdef AND has_function_privilege(a.oid,p.oid,'EXECUTE')
 AND (pg_has_role(p.proowner,c.relowner,'MEMBER') OR has_table_privilege(p.proowner,c.oid,'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')))))),true)
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname=$1 AND c.relname=ANY($3::text[])`, r.schema, r.ownerOID, []string{"voice_party_resource_components", "voice_party_resource_retirements", "voice_party_resource_component_links"}).Scan(&count, &unsafe)
	if err != nil || count != 3 || unsafe {
		return retirementproof.ErrUnavailable
	}
	return nil
}
