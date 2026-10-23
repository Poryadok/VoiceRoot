package identityassociation

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// validateIdentityShape complements the exact function/trigger catalog. A
// surviving function body is not proof that uniqueness or FK guards still exist.
func validateIdentityShape(ctx context.Context, tx pgx.Tx) error {
	type index struct {
		table, name string
		columns     []string
		primary     bool
		predicate   string
	}
	indexes := []index{
		{"profiles", "profiles_pkey", []string{"id"}, true, ""},
		{"identity_meta", "identity_meta_pkey", []string{"singleton"}, true, ""},
		{"identity_key_versions", "identity_key_versions_pkey", []string{"version"}, true, ""},
		{"identity_profile_reservations", "identity_profile_reservations_pkey", []string{"key_version", "profile_selector"}, true, ""},
		{"identity_associations", "identity_associations_pkey", []string{"association_id"}, true, ""},
		{"identity_associations", "identity_associations_receipt_id_key", []string{"receipt_id"}, false, ""},
		{"association_selector_aliases", "association_selector_aliases_pkey", []string{"association_id", "key_version", "kind"}, true, ""},
		{"association_selector_aliases", "association_unique_profile_alias", []string{"key_version", "selector"}, false, "(kind = 'PROFILE'::text)"},
		{"identity_parent_fences", "identity_parent_fences_pkey", []string{"owner_handle"}, true, ""},
		{"identity_account_parent_aliases", "identity_account_parent_aliases_pkey", []string{"key_version", "account_selector"}, true, ""},
		{"identity_journal", "identity_journal_pkey", []string{"revision"}, true, ""},
		{"profiles", "profiles_identity_association_id_key", []string{"identity_association_id"}, false, ""},
	}
	for _, expected := range indexes {
		var valid bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_index i JOIN pg_class idx ON idx.oid=i.indexrelid JOIN pg_am am ON am.oid=idx.relam
   WHERE i.indrelid=to_regclass($1) AND idx.relname=$2 AND i.indisunique AND i.indisvalid AND i.indisready AND NOT i.indisexclusion
   AND i.indisprimary=$3 AND am.amname='btree' AND i.indexprs IS NULL AND i.indnkeyatts=$4 AND i.indnatts=$4
   AND COALESCE(pg_get_expr(i.indpred,i.indrelid),'')=$5
   AND (SELECT array_agg(a.attname::text ORDER BY k.ordinality) FROM unnest(i.indkey) WITH ORDINALITY k(attnum,ordinality) JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum)=$6::text[])`, "public."+expected.table, expected.name, expected.primary, len(expected.columns), expected.predicate, expected.columns).Scan(&valid)
		if err != nil {
			return err
		}
		if !valid {
			return ErrIncomplete
		}
	}
	type foreign struct{ table, column, referenced, columnRef string }
	for _, expected := range []foreign{
		{"profiles", "identity_association_id", "identity_associations", "association_id"},
		{"identity_profile_reservations", "key_version", "identity_key_versions", "version"},
		{"association_selector_aliases", "association_id", "identity_associations", "association_id"},
		{"association_selector_aliases", "key_version", "identity_key_versions", "version"},
		{"identity_account_parent_aliases", "key_version", "identity_key_versions", "version"},
		{"identity_account_parent_aliases", "owner_handle", "identity_parent_fences", "owner_handle"},
	} {
		var valid bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint c JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attname=$2 JOIN pg_attribute target ON target.attrelid=c.confrelid AND target.attname=$4
   WHERE c.conrelid=to_regclass($1) AND c.confrelid=to_regclass($3) AND c.contype='f' AND c.convalidated AND NOT c.condeferrable
   AND c.conkey=ARRAY[a.attnum] AND c.confkey=ARRAY[target.attnum] AND c.confdeltype='a' AND c.confupdtype='a' AND c.confmatchtype='s')`, "public."+expected.table, expected.column, "public."+expected.referenced, expected.columnRef).Scan(&valid)
		if err != nil {
			return err
		}
		if !valid {
			return ErrIncomplete
		}
	}
	for _, name := range []string{"identity_birth_requires_profile", "identity_association_requires_journal"} {
		var valid bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.identity_associations'::regclass AND tgname=$1 AND tgdeferrable AND tginitdeferred AND tgconstraint<>0)`, name).Scan(&valid)
		if err != nil {
			return err
		}
		if !valid {
			return ErrIncomplete
		}
	}
	return nil
}
