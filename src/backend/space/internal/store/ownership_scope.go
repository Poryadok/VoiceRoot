package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// lockOwnershipTransaction uses disjoint PostgreSQL advisory key spaces:
// operation two-int32 first, then space bigint. A future ordinary transaction
// may take the space lock alone but must never call this while holding it.
// The transaction and all reads/writes remain on one connection.
func lockOwnershipTransaction(ctx context.Context, tx pgx.Tx, operationID, spaceID uuid.UUID) error {
	material := append([]byte("voice.space.ownership.operation.v1\x00"), operationID[:]...)
	digest := sha256.Sum256(material)
	first := int32(binary.BigEndian.Uint32(digest[:4]))
	second := int32(binary.BigEndian.Uint32(digest[4:8]))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, first, second); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID))
	return err
}
