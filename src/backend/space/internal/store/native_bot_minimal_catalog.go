package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
)

func checkNativeMinimal48Triggers(ctx context.Context, tx pgx.Tx) error {
	type trigger struct {
		table, name, function, enabled string
		kind                           int
	}
	triggers := []trigger{
		{"native_membership_guard.bot_terminal_registrations", "bot_terminal48_registration", "native_membership_guard.bot_terminal_registration_guard", "A", 7},
		{"public.spaces", "zzzzzzz_bot_minimal48_receipt_terminal", "native_membership_guard.bot_receipt48_terminal_cleanup", "O", 9},
	}
	for _, table := range []string{"bot_terminal_registrations", "bot_removal_receipts48", "catalog48_transition"} {
		triggers = append(triggers, trigger{"native_membership_guard." + table, "bot_minimal48_immutable", "native_membership_guard.bot_minimal48_immutable", "O", 27}, trigger{"native_membership_guard." + table, "bot_minimal48_no_truncate", "native_membership_guard.bot_minimal48_immutable", "O", 34})
	}
	batch := &pgx.Batch{}
	for _, t := range triggers {
		schema, table, _ := strings.Cut(t.table, ".")
		fs, fn, _ := strings.Cut(t.function, ".")
		batch.Queue(`SELECT EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
   WHERE n.nspname=$1 AND c.relname=$2 AND t.tgname=$3 AND pn.nspname=$4 AND p.proname=$5 AND p.pronargs=0 AND p.prorettype='trigger'::regtype AND t.tgenabled::text=$6 AND t.tgtype=$7
   AND NOT t.tgisinternal AND NOT t.tgdeferrable AND NOT t.tginitdeferred AND t.tgnargs=0 AND t.tgargs=''::bytea)`, schema, table, t.name, fs, fn, t.enabled, t.kind)
	}
	rows := tx.SendBatch(ctx, batch)
	for range triggers {
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
	if err := tx.QueryRow(ctx, `SELECT count(*)=3 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proname IN ('space_bot_global_terminal_register','space_bot_global_removal_saved48','space_bot_global_removal_record48')`).Scan(&valid); err != nil || !valid {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}
