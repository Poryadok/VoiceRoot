package identityassociation

import (
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type terminalRecord struct {
	AssociationID     uuid.UUID
	HistoryGeneration uuid.UUID
	KeysetRevision    uint64
	Evidence          TerminalEvidence
	Parent            parentFact
}

func (r terminalRecord) digest() [32]byte {
	r.Evidence.ReceiptDigest = [32]byte{}
	encoded, _ := json.Marshal(r)
	return sha256.Sum256(append([]byte("voice.user.identity-association.terminal.v1\x00"), encoded...))
}
func newTerminalRecord(tx *IdentityTx, association uuid.UUID, revision uint64, now time.Time, parent ParentObservation) terminalRecord {
	r := terminalRecord{AssociationID: association, HistoryGeneration: tx.generation, KeysetRevision: tx.keysetRevision,
		Evidence: TerminalEvidence{OperationID: uuid.New(), Revision: revision, ReceiptID: uuid.New(), ObservedAt: now}, Parent: minimizedParent(parent)}
	r.Evidence.ReceiptDigest = r.digest()
	return r
}
func (r terminalRecord) valid(association uuid.UUID, generation uuid.UUID, revision uint64) bool {
	p := r.Parent
	e := r.Evidence
	return r.AssociationID == association && r.HistoryGeneration == generation && r.KeysetRevision > 0 && e.OperationID != uuid.Nil && e.ReceiptID != uuid.Nil && e.Revision > 0 && e.Revision <= revision && !e.ObservedAt.IsZero() && e.ReceiptDigest == r.digest() && p.State == ParentTerminal && p.Disposition == ParentIrreversible && p.Eligibility == ParentIneligible && p.Cycle != uuid.Nil && !p.EraseAt.IsZero() && !p.EraseAt.After(p.ObservedAt) && p.Cut.Owner == "auth" && p.Cut.Complete && p.Revision > 0 && p.Cut.ThroughRevision > 0
}
