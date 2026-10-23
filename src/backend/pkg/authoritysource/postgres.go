package authoritysource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Catalog is a fixed owner-authored schema contract, never request data.
type Catalog struct {
	Version   int64
	Tables    []string
	Triggers  []CatalogTrigger
	Functions []CatalogFunction
}
type CatalogFunction struct {
	Name       string
	Arguments  int16
	ReturnType string
	BodySHA256 string
}
type CatalogTrigger struct {
	Table, Name, Function string
	Type                  int16
	Argument              string
}

// BeginRead locks against schema changes before checking the source catalog.
// Domain state and its durable counters share this read-only repeatable-read cut.
func BeginRead(ctx context.Context, pool *pgxpool.Pool, catalog Catalog) (pgx.Tx, error) {
	if pool == nil || catalog.Version <= 0 || len(catalog.Tables) == 0 {
		return nil, errors.New("owner catalog unavailable")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			RollbackRead(ctx, tx)
		}
	}()
	if err = CheckCatalog(ctx, tx, catalog); err != nil {
		return nil, err
	}
	failed = false
	return tx, nil
}

// ValidateTx validates the same fixed owner contract inside an existing
// native transaction. Callers must already hold their complete domain fence.
func ValidateTx(ctx context.Context, tx pgx.Tx, catalog Catalog) error {
	return CheckCatalog(ctx, tx, catalog)
}

func CheckCatalog(ctx context.Context, tx pgx.Tx, catalog Catalog) error {
	if tx == nil || catalog.Version <= 0 || len(catalog.Tables) == 0 {
		return errors.New("owner catalog unavailable")
	}
	var err error
	if _, err = tx.Exec(ctx, `SET LOCAL row_security=off; SET LOCAL search_path=public,pg_catalog`); err != nil {
		return err
	}
	names := make([]string, 0, len(catalog.Tables)+1)
	names = append(names, pgx.Identifier{"public", "schema_migrations"}.Sanitize())
	for _, name := range catalog.Tables {
		names = append(names, pgx.Identifier{"public", name}.Sanitize())
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS SHARE MODE"); err != nil {
		return err
	}
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(version=$1 AND NOT dirty) FROM public.schema_migrations`, catalog.Version).Scan(&valid); err != nil || !valid {
		return errors.New("owner schema version unavailable")
	}
	tables := append([]string{"schema_migrations"}, catalog.Tables...)
	if err = tx.QueryRow(ctx, `SELECT count(*)=$2 AND bool_and(c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
  AND NOT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=c.oid)
  AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
  AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=c.oid OR inhparent=c.oid))
  FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($1)`, tables, len(tables)).Scan(&valid); err != nil || !valid {
		return errors.New("owner source relation unavailable")
	}
	// Validate complete fixed sets in two roundtrips. Per-item roundtrips made
	// the native two-stage protocol exhaust its existing5s budget under load;
	// all table/type/namespace/argument/body predicates remain mandatory.
	// Cast declared identifiers to PostgreSQL name, preserving the server's
	// identifier truncation used by the former individually typed parameters.
	if len(catalog.Triggers) > 0 {
		n := len(catalog.Triggers)
		tables, names, functions := make([]string, n), make([]string, n), make([]string, n)
		types, args := make([]int16, n), make([][]byte, n)
		for i, t := range catalog.Triggers {
			tables[i], names[i], functions[i], types[i], args[i] = t.Table, t.Name, t.Function, t.Type, []byte{}
			if t.Argument != "" {
				args[i] = []byte(t.Argument + "\x00")
			}
		}
		err = tx.QueryRow(ctx, `SELECT count(*)=$6 FROM unnest($1::text[],$2::text[],$3::text[],$4::smallint[],$5::bytea[]) e(table_name,trigger_name,function_name,trigger_type,arguments)
 JOIN pg_trigger t ON t.tgrelid=to_regclass(format('public.%I',e.table_name)) AND t.tgname=e.trigger_name::name
 JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE NOT t.tgisinternal AND t.tgenabled='O' AND t.tgtype=e.trigger_type AND n.nspname='public' AND p.proname=e.function_name::name AND t.tgargs=e.arguments`, tables, names, functions, types, args, n).Scan(&valid)
		if err != nil {
			return fmt.Errorf("owner source trigger unavailable: %w", err)
		}
		if !valid {
			return errors.New("owner source trigger unavailable")
		}
	}
	if len(catalog.Functions) > 0 {
		n := len(catalog.Functions)
		names, returns, hashes := make([]string, n), make([]string, n), make([]string, n)
		arguments := make([]int16, n)
		for i, f := range catalog.Functions {
			names[i], returns[i], hashes[i], arguments[i] = f.Name, f.ReturnType, f.BodySHA256, f.Arguments
		}
		rows, err := tx.Query(ctx, `SELECT e.ordinal,e.body_hash,p.prosrc FROM unnest($1::text[],$2::smallint[],$3::text[],$4::text[]) WITH ORDINALITY e(name,arguments,return_type,body_hash,ordinal)
 JOIN pg_proc p ON p.proname=e.name::name AND p.pronargs=e.arguments AND p.prorettype=to_regtype(e.return_type)
 JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang
 WHERE n.nspname='public' AND NOT p.prosecdef AND p.provolatile='v' AND l.lanname='plpgsql'`, names, arguments, returns, hashes)
		if err != nil {
			return errors.New("owner source function unavailable")
		}
		defer rows.Close()
		count := 0
		seen := make([]bool, n)
		for rows.Next() {
			var ordinal int64
			var expected, body string
			if err := rows.Scan(&ordinal, &expected, &body); err != nil {
				return err
			}
			if ordinal < 1 || ordinal > int64(n) || seen[ordinal-1] {
				return errors.New("owner source function ambiguous")
			}
			seen[ordinal-1] = true
			hash := sha256.Sum256([]byte(body))
			if hex.EncodeToString(hash[:]) != expected {
				return errors.New("owner source function changed")
			}
			count++
			if count > n {
				return errors.New("owner source function ambiguous")
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if count != n {
			return fmt.Errorf("owner source function unavailable: expected %d, got %d", n, count)
		}
	}
	return nil
}

// RollbackRead releases failed/cancelled reads without reusing an already dead
// deadline or allowing a failed connection to hold cleanup forever.
func RollbackRead(ctx context.Context, tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	_ = tx.Rollback(cleanup)
}
