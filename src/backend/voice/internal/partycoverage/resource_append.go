package partycoverage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/retirementproof"
)

// Private storage path, not an operator proof-import/mint API. Only the actual
// resource-observing issuer may call it; persistence does not qualify native
// owner completion or advance the coverage head.
type resourceJournal struct {
	conn   *pgx.Conn
	schema string
}

func resourceProofID(operation uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(operation, []byte(retirementproof.ResourceDomain))
}
func (j resourceJournal) append(ctx context.Context, r retirementproof.ResourceRecord) error {
	if j.conn == nil || j.schema == "" {
		return retirementproof.ErrUnavailable
	}
	if err := retirementproof.ValidateResourceRecord(r); err != nil {
		return err
	}
	cut := r.Document.Cut
	if r.Ref.ProofID != resourceProofID(cut.CutoverOperationID) {
		return retirementproof.ErrConflict
	}
	for _, c := range r.Components {
		if c.Ref.JournalRef != uuid.NewSHA1(r.Ref.ProofID, []byte(c.Ref.Kind)) {
			return retirementproof.ErrConflict
		}
	}
	tx, err := j.conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// A lost acknowledgement reads immutable history, even if this cut has since
	// advanced. That readback cannot change a head or qualify new owner work.
	var saved, hash []byte
	err = tx.QueryRow(ctx, `SELECT canonical_receipt_bytes,receipt_sha256 FROM `+j.table("voice_party_resource_retirements")+` WHERE proof_id=$1`, r.Ref.ProofID).Scan(&saved, &hash)
	if err == nil {
		if !bytes.Equal(saved, r.CanonicalBytes) || !bytes.Equal(hash, r.Ref.ReceiptHash[:]) {
			return retirementproof.ErrConflict
		}
		if err = j.checkStored(ctx, tx, r); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var operation, environment, incarnation uuid.UUID
	var epoch int64
	var phase string
	err = tx.QueryRow(ctx, `SELECT current_operation_id,environment_id,authority_incarnation,coverage_epoch,phase FROM `+j.table("voice_party_coverage_head")+` WHERE singleton FOR UPDATE`).Scan(&operation, &environment, &incarnation, &epoch, &phase)
	if err != nil {
		return retirementproof.ErrConflict
	}
	// Another identical caller may have committed while we waited for this
	// lock. READ COMMITTED sees that winner; never allocate/insert a second row.
	err = tx.QueryRow(ctx, `SELECT canonical_receipt_bytes,receipt_sha256 FROM `+j.table("voice_party_resource_retirements")+` WHERE proof_id=$1`, r.Ref.ProofID).Scan(&saved, &hash)
	if err == nil {
		if !bytes.Equal(saved, r.CanonicalBytes) || !bytes.Equal(hash, r.Ref.ReceiptHash[:]) {
			return retirementproof.ErrConflict
		}
		if err = j.checkStored(ctx, tx, r); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if operation != cut.CutoverOperationID || environment != cut.EnvironmentID || incarnation != cut.AuthorityIncarnation || epoch <= 0 || uint64(epoch) != cut.CoverageEpoch || phase != "FROZEN" {
		return retirementproof.ErrConflict
	}
	var planBytes, planHash []byte
	err = tx.QueryRow(ctx, `SELECT plan_bytes,plan_sha256 FROM `+j.table("voice_party_coverage_operations")+` WHERE operation_id=$1`, operation).Scan(&planBytes, &planHash)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(planBytes)
	var plan cutPlan
	if !bytes.Equal(sum[:], planHash) || sum != cut.PlanHash || json.Unmarshal(planBytes, &plan) != nil || plan.OperationID != operation || plan.Binding.Validate() != nil {
		return retirementproof.ErrConflict
	}
	b := plan.Binding
	if b.EnvironmentID != cut.EnvironmentID || b.AuthorityIncarnation != cut.AuthorityIncarnation || b.CoverageEpoch != cut.CoverageEpoch || b.DatabaseIdentity != cut.DatabaseIdentity || b.SourceCatalogHash != cut.SourceCatalogHash || b.SchemaContractHash != cut.SchemaContractHash || b.WriterManifestHash != cut.WriterManifestHash {
		return retirementproof.ErrConflict
	}
	identity, err := ObserveDatabaseIdentity(ctx, tx)
	if err != nil || identity != cut.DatabaseIdentity {
		return retirementproof.ErrUnavailable
	}
	for _, c := range r.Components {
		_, err = tx.Exec(ctx, `INSERT INTO `+j.table("voice_party_resource_components")+` VALUES($1,$2,$3,$4,$5) ON CONFLICT(journal_ref) DO NOTHING`, c.Ref.JournalRef, c.Ref.Kind, int64(c.Ref.Protocol), c.CanonicalBytes, c.Ref.CanonicalBytesHash[:])
		if err != nil {
			return err
		}
		var kind string
		var protocol int64
		var raw, h []byte
		err = tx.QueryRow(ctx, `SELECT kind,protocol,canonical_bytes,canonical_bytes_sha256 FROM `+j.table("voice_party_resource_components")+` WHERE journal_ref=$1`, c.Ref.JournalRef).Scan(&kind, &protocol, &raw, &h)
		if err != nil {
			return err
		}
		if kind != c.Ref.Kind || protocol != int64(c.Ref.Protocol) || !bytes.Equal(raw, c.CanonicalBytes) || !bytes.Equal(h, c.Ref.CanonicalBytesHash[:]) {
			return retirementproof.ErrConflict
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO `+j.table("voice_party_resource_retirements")+` VALUES($1,$2,$3,$4,$5)`, r.Ref.ProofID, int64(r.Ref.Protocol), operation, r.CanonicalBytes, r.Ref.ReceiptHash[:])
	if err != nil {
		return err
	}
	for i, c := range r.Components {
		_, err = tx.Exec(ctx, `INSERT INTO `+j.table("voice_party_resource_component_links")+` VALUES($1,$2,$3)`, r.Ref.ProofID, int64(i), c.Ref.JournalRef)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (j resourceJournal) table(name string) string { return pgx.Identifier{j.schema, name}.Sanitize() }
func (j resourceJournal) checkStored(ctx context.Context, tx pgx.Tx, r retirementproof.ResourceRecord) error {
	rows, err := tx.Query(ctx, `SELECT l.ordinal,c.journal_ref,c.kind,c.protocol,c.canonical_bytes,c.canonical_bytes_sha256 FROM `+j.table("voice_party_resource_component_links")+` l JOIN `+j.table("voice_party_resource_components")+` c ON c.journal_ref=l.journal_ref WHERE l.proof_id=$1 ORDER BY l.ordinal`, r.Ref.ProofID)
	if err != nil {
		return err
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var ordinal, protocol int64
		var id uuid.UUID
		var kind string
		var raw, hash []byte
		if err = rows.Scan(&ordinal, &id, &kind, &protocol, &raw, &hash); err != nil {
			return err
		}
		if i >= len(r.Components) {
			return retirementproof.ErrConflict
		}
		c := r.Components[i]
		if ordinal != int64(i) || id != c.Ref.JournalRef || kind != c.Ref.Kind || protocol != int64(c.Ref.Protocol) || !bytes.Equal(raw, c.CanonicalBytes) || !bytes.Equal(hash, c.Ref.CanonicalBytesHash[:]) {
			return retirementproof.ErrConflict
		}
		i++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if i != len(r.Components) {
		return retirementproof.ErrConflict
	}
	return nil
}
