package identityassociation

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdoptionSourcePin is recorded independently of an existing source during a
// fenced adoption, before provisioning any keys. It asserts no historical
// creation, migration, BIRTH, currentness or authority to activate guards.
// Receipt/evidence identifiers bind the custodian's reviewed input; they do not
// prove that backup/restore or exclusive custody actually happened.
type AdoptionSourcePin struct {
	ReceiptID                               uuid.UUID
	EvidenceDigest                          [32]byte
	HistoryGeneration, UserSourceGeneration uuid.UUID
	KeysetDigest                            [32]byte
	Physical                                PhysicalSourceIdentity
}

// NewAdoptedStore provisions only an exact, pristine inactive identity domain.
// Existing profiles are retained without assigning historical associations.
// The caller must first persist the adoption attempt outside the fenced source.
func NewAdoptedStore(ctx context.Context, pool *pgxpool.Pool, keys *Keyset, pin AdoptionSourcePin) (*Store, error) {
	if ctx == nil || ctx.Err() != nil || keys == nil || pin.ReceiptID == uuid.Nil || pin.EvidenceDigest == [32]byte{} || pin.Physical.SystemIdentifier == "" || pin.Physical.Database == "" || pin.Physical.DatabaseOID == 0 {
		return nil, ErrIncomplete
	}
	digest, err := keys.CustodyDigest()
	if err != nil || digest != pin.KeysetDigest {
		return nil, ErrIncomplete
	}
	return newPristineStoreForPhysicalSource(ctx, pool, keys, pin.HistoryGeneration, pin.UserSourceGeneration, &pin.Physical)
}
