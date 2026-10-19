package spacebarrier

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// LockIdentities uses BE106 c10's namespace and actual sorted signed hash IDs.
// Caller already owns Space and every required native room resource.
func LockIdentities(ctx context.Context, tx pgx.Tx, account, profile string) error {
	rows, err := tx.Query(ctx, `SELECT DISTINCT hashtext(value) key FROM unnest(ARRAY['account:'||$1::text,'profile:'||$2::text]) value ORDER BY key`, account, profile)
	if err != nil {
		return err
	}
	var keys []int32
	for rows.Next() {
		var key int32
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(0x564F4943), key); err != nil {
			return err
		}
	}
	return nil
}
