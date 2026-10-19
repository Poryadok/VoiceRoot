// Package admissionlock serializes trusted identity admission across native
// Voice journals and SDK conversion. Callers own the transaction and lock order.
package admissionlock

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Namespace uses PostgreSQL's two-int key space, distinct from the bigint
// operation lock used by SDK conversion.
const Namespace int32 = 0x564F4943

type Identity struct{ AccountID, ProfileID uuid.UUID }

type Transaction interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// Lock acquires the complete identity union before any mapping/fence writes.
// Native writers must first lock their room/head; conversion must never lock
// those rooms after this barrier. Use READ COMMITTED and read current ownership
// in a separate statement after Lock returns.
func Lock(ctx context.Context, tx Transaction, identities []Identity) error {
	labels := make(map[string]struct{}, len(identities)*2)
	for _, identity := range identities {
		if identity.AccountID == uuid.Nil || identity.ProfileID == uuid.Nil {
			return errors.New("identity admission lock requires trusted account and profile")
		}
		labels["account:"+identity.AccountID.String()] = struct{}{}
		labels["profile:"+identity.ProfileID.String()] = struct{}{}
	}
	keys := make(map[int32]struct{}, len(labels))
	for label := range labels {
		var key int32
		if err := tx.QueryRow(ctx, `SELECT hashtext($1::text)`, label).Scan(&key); err != nil {
			return err
		}
		keys[key] = struct{}{}
	}
	ordered := make([]int32, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, key := range ordered {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, Namespace, key); err != nil {
			return err
		}
	}
	return nil
}
