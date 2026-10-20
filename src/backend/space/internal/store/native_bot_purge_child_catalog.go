package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

func checkNativePurgeChild49Triggers(ctx context.Context, tx pgx.Tx) error {
	checks := []struct {
		table, name, function string
		kind                  int
	}{
		{"native_membership_guard.bot_purge_children", "bot_purge49_immutable", "bot_purge_child_immutable", 31},
		{"native_membership_guard.bot_purge_children", "bot_purge49_no_truncate", "bot_purge_child_immutable", 34},
		{"native_membership_guard.catalog49_transition", "bot_purge49_transition_immutable", "bot_purge_child_immutable", 27},
		{"native_membership_guard.catalog49_transition", "bot_purge49_transition_no_truncate", "bot_purge_child_immutable", 34},
		{"public.spaces", "zzzzzzzz_bot_purge49_parent_terminal", "bot_purge_child_parent_completed", 9},
		{"public.space_lifecycle_aggregates", "zzzzzzzz_bot_purge49_parent_minimize", "bot_purge_child_parent_completed", 27},
	}
	batch := &pgx.Batch{}
	for _, check := range checks {
		schema, table, _ := strings.Cut(check.table, ".")
		batch.Queue(`SELECT EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
 WHERE n.nspname=$1 AND c.relname=$2 AND t.tgname=$3 AND t.tgtype=$4 AND t.tgenabled='O'
 AND NOT t.tgisinternal AND NOT t.tgdeferrable AND NOT t.tginitdeferred AND t.tgnargs=0 AND t.tgargs=''::bytea
 AND pn.nspname='native_membership_guard' AND p.proname=$5 AND p.pronargs=0 AND p.prorettype='trigger'::regtype)`, schema, table, check.name, check.kind, check.function)
	}
	rows := tx.SendBatch(ctx, batch)
	for range checks {
		var valid bool
		if err := rows.QueryRow().Scan(&valid); err != nil || !valid {
			_ = rows.Close()
			return ErrMemberAuthorityUnavailable
		}
	}
	if err := rows.Close(); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT count(*)=3 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proname IN ('space_bot_purge_child_saved','space_bot_purge_child_record','space_bot_purge_child_expire')`).Scan(&valid); err != nil || !valid {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}
