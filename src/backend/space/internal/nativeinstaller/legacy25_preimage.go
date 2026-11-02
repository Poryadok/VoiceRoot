package nativeinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// This is the fixed source25 table set, not a caller-selected table list.
// schema_migrations is intentionally excluded: the real migration advances it.
var legacy25DataTables = [...]string{
	"audit_compensation_authorizations", "audit_cursor_signing_key", "audit_log", "audit_outbox",
	"categories", "community_bootstrap_operations", "community_owner_authority", "community_owner_recovery_operations",
	"community_roster_members", "community_roster_operations", "invites", "ownership_journal", "ownership_outbox",
	"space_bans", "space_deletion_tombstones", "space_lifecycle_aggregates", "space_lifecycle_completions",
	"space_lifecycle_operations", "space_lifecycle_outbox", "space_lifecycle_participants", "space_member_timeouts",
	"space_members", "space_subscriptions", "space_tree_nodes", "space_voice_access_epochs", "space_voice_access_outbox",
	"spaces", "voice_rooms",
}

type legacy25TableDigest struct {
	Columns []string `json:"columns"`
	Count   uint64   `json:"count"`
	SHA256  string   `json:"sha256"`
}

func captureLegacy25ProjectionTx(ctx context.Context, tx pgx.Tx, original map[string]legacy25TableDigest) (map[string]legacy25TableDigest, error) {
	const maxRows = 1_000_000
	const maxRowBytes = 8 << 20
	const maxTotalBytes = 512 << 20
	result := make(map[string]legacy25TableDigest, len(legacy25DataTables))
	if original != nil && len(original) != len(legacy25DataTables) {
		return nil, errors.New("legacy25 projection unavailable")
	}
	var total uint64
	for _, table := range legacy25DataTables {
		var columns []string
		if original == nil {
			columnRows, err := tx.Query(ctx, `SELECT a.attname FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=$1 AND c.relkind='r' AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, table)
			if err != nil {
				return nil, errors.New("legacy25 projection unavailable")
			}
			for columnRows.Next() {
				var col string
				if err := columnRows.Scan(&col); err != nil {
					columnRows.Close()
					return nil, errors.New("legacy25 projection unavailable")
				}
				columns = append(columns, col)
			}
			if err := columnRows.Err(); err != nil {
				columnRows.Close()
				return nil, errors.New("legacy25 projection unavailable")
			}
			columnRows.Close()
		} else {
			prior, ok := original[table]
			if !ok {
				return nil, errors.New("legacy25 projection unavailable")
			}
			columns = append([]string(nil), prior.Columns...)
		}
		if len(columns) == 0 || len(columns) > 1600 {
			return nil, errors.New("legacy25 projection unavailable")
		}
		fields := make([]string, 0, len(columns))
		seen := make(map[string]bool, len(columns))
		for _, col := range columns {
			if col == "" || seen[col] {
				return nil, errors.New("legacy25 projection unavailable")
			}
			seen[col] = true
			fields = append(fields, pgx.Identifier{col}.Sanitize())
		}
		name := pgx.Identifier{"public", table}.Sanitize()
		query := fmt.Sprintf(`SELECT CASE WHEN octet_length(to_jsonb(t)::text)<=%d THEN to_jsonb(t)::text ELSE NULL END FROM (SELECT %s FROM %s) t ORDER BY to_jsonb(t)::text COLLATE "C" LIMIT %d`, maxRowBytes, strings.Join(fields, ","), name, maxRows+1)
		rows, err := tx.Query(ctx, query)
		if err != nil {
			return nil, errors.New("legacy25 preimage unavailable")
		}
		h := sha256.New()
		var count uint64
		for rows.Next() {
			var row *string
			if err := rows.Scan(&row); err != nil || row == nil {
				rows.Close()
				return nil, errors.New("legacy25 preimage unavailable")
			}
			count++
			total += uint64(len(*row))
			if count > maxRows || total > maxTotalBytes {
				rows.Close()
				return nil, errors.New("legacy25 preimage exceeds bound")
			}
			var length [8]byte
			binary.BigEndian.PutUint64(length[:], uint64(len(*row)))
			_, _ = h.Write(length[:])
			_, _ = h.Write([]byte(*row))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, errors.New("legacy25 preimage unavailable")
		}
		rows.Close()
		result[table] = legacy25TableDigest{Columns: columns, Count: count, SHA256: hex.EncodeToString(h.Sum(nil))}
	}
	return result, nil
}

func beginLegacySourceRead(ctx context.Context, conn *pgx.Conn) (pgx.Tx, error) {
	if ctx == nil || conn == nil {
		return nil, errors.New("legacy source unavailable")
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, errors.New("legacy source unavailable")
	}
	if _, err = tx.Exec(ctx, `SET LOCAL search_path=pg_catalog; SET LOCAL TIME ZONE 'UTC'; SET LOCAL DateStyle='ISO, YMD'`); err != nil {
		failure := errors.New("legacy source unavailable")
		finishLegacy24Read(tx, &failure)
		return nil, failure
	}
	locked := make([]string, 0, len(legacy25DataTables)+1)
	for _, table := range legacy25DataTables {
		locked = append(locked, pgx.Identifier{"public", table}.Sanitize())
	}
	locked = append(locked, pgx.Identifier{"public", "schema_migrations"}.Sanitize())
	if _, err = tx.Exec(ctx, `LOCK TABLE `+strings.Join(locked, ",")+` IN ACCESS SHARE MODE`); err != nil {
		failure := errors.New("legacy source unavailable")
		finishLegacy24Read(tx, &failure)
		return nil, failure
	}
	return tx, nil
}
