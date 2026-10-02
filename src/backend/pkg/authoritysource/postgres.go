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
	if _, err = tx.Exec(ctx, `SET LOCAL row_security=off; SET LOCAL search_path=public,pg_catalog`); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(catalog.Tables)+1)
	names = append(names, pgx.Identifier{"public", "schema_migrations"}.Sanitize())
	for _, name := range catalog.Tables {
		names = append(names, pgx.Identifier{"public", name}.Sanitize())
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS SHARE MODE"); err != nil {
		return nil, err
	}
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(version=$1 AND NOT dirty) FROM public.schema_migrations`, catalog.Version).Scan(&valid); err != nil || !valid {
		return nil, errors.New("owner schema version unavailable")
	}
	tables := append([]string{"schema_migrations"}, catalog.Tables...)
	if err = tx.QueryRow(ctx, `SELECT count(*)=$2 AND bool_and(c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
  AND NOT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=c.oid)
  AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
  AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=c.oid OR inhparent=c.oid))
  FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($1)`, tables, len(tables)).Scan(&valid); err != nil || !valid {
		return nil, errors.New("owner source relation unavailable")
	}
	for _, trigger := range catalog.Triggers {
		arguments := []byte{}
		if trigger.Argument != "" {
			arguments = []byte(trigger.Argument + "\x00")
		}
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid
   JOIN pg_namespace n ON n.oid=p.pronamespace WHERE t.tgrelid=to_regclass($1) AND t.tgname=$2 AND NOT t.tgisinternal
   AND t.tgenabled='O' AND t.tgtype=$3 AND n.nspname='public' AND p.proname=$4 AND t.tgargs=$5)`,
			"public."+trigger.Table, trigger.Name, trigger.Type, trigger.Function, arguments).Scan(&valid); err != nil || !valid {
			return nil, fmt.Errorf("owner source trigger unavailable: %s", trigger.Name)
		}
	}
	for _, function := range catalog.Functions {
		var body string
		if err = tx.QueryRow(ctx, `SELECT p.prosrc FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
   JOIN pg_language l ON l.oid=p.prolang WHERE n.nspname='public' AND p.proname=$1 AND p.pronargs=$2
   AND NOT p.prosecdef AND p.provolatile='v' AND l.lanname='plpgsql' AND p.prorettype=to_regtype($3)`, function.Name, function.Arguments, function.ReturnType).Scan(&body); err != nil {
			return nil, errors.New("owner source function unavailable")
		}
		hash := sha256.Sum256([]byte(body))
		if hex.EncodeToString(hash[:]) != function.BodySHA256 {
			return nil, errors.New("owner source function changed")
		}
	}
	failed = false
	return tx, nil
}

// RollbackRead releases failed/cancelled reads without reusing an already dead
// deadline or allowing a failed connection to hold cleanup forever.
func RollbackRead(ctx context.Context, tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	_ = tx.Rollback(cleanup)
}
