package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func checkNativeGlobalRemoval46Triggers(ctx context.Context, tx pgx.Tx) error {
	type trigger struct {
		table, name, function, enabled string
		kind                           int
	}
	triggers := []trigger{
		{"native_membership_guard.bot_terminals", "bot_terminal_registration", "native_membership_guard.bot_terminal_registration_guard()", "A", 7},
		{"public.space_bot_installation_attempts", "a001_bot_global_terminal", "native_membership_guard.bot_add_terminal_guard()", "A", 23},
		{"public.spaces", "zzzzzz_bot_global_receipt_terminal", "native_membership_guard.bot_receipt_terminal_cleanup()", "O", 9},
	}
	for _, table := range []string{"bot_terminals", "bot_removal_receipts", "catalog46_transition"} {
		triggers = append(triggers,
			trigger{"native_membership_guard." + table, "bot_global_immutable", "native_membership_guard.bot_global_immutable()", "O", 27},
			trigger{"native_membership_guard." + table, "bot_global_no_truncate", "native_membership_guard.bot_global_immutable()", "O", 34})
	}
	batch := &pgx.Batch{}
	for _, t := range triggers {
		tableSchema, tableName, _ := strings.Cut(t.table, ".")
		functionSchema, functionName, _ := strings.Cut(strings.TrimSuffix(t.function, "()"), ".")
		// A serving LOGIN intentionally has no USAGE on the private schema.
		// Authenticate metadata through pg_catalog, never grant schema access
		// merely to make a regclass/regprocedure text lookup succeed.
		batch.Queue(`SELECT EXISTS(SELECT 1 FROM pg_trigger t
 JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace cn ON cn.oid=c.relnamespace
 JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
 WHERE cn.nspname=$1 AND c.relname=$2 AND t.tgname=$3 AND pn.nspname=$4 AND p.proname=$5
 AND p.pronargs=0 AND p.prorettype='trigger'::regtype AND t.tgenabled::text=$6 AND t.tgtype=$7 AND NOT t.tgisinternal
 AND NOT t.tgdeferrable AND NOT t.tginitdeferred AND t.tgnargs=0 AND t.tgargs=''::bytea)`, tableSchema, tableName, t.name, functionSchema, functionName, t.enabled, t.kind)
	}
	rows := tx.SendBatch(ctx, batch)
	for range triggers {
		var valid bool
		if err := rows.QueryRow().Scan(&valid); err != nil || !valid {
			_ = rows.Close()
			return fmt.Errorf("%w: global46 trigger metadata: %v", ErrMemberAuthorityUnavailable, err)
		}
	}
	if err := rows.Close(); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT count(*)=3 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE n.nspname='public' AND p.proname IN ('space_bot_native_terminal','space_bot_global_removal_saved','space_bot_global_removal_record')`).Scan(&valid); err != nil || !valid {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}
