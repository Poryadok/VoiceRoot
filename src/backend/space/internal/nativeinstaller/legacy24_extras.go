package nativeinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Only independent, plain tables in non-reserved schemas are supported.
// All other external object families remain a named HOLD before migration.
type legacy24Extra struct {
	Schema     string `json:"schema"`
	Table      string `json:"table"`
	Definition string `json:"definition"`
	Count      uint64 `json:"count"`
	SHA256     string `json:"sha256"`
}

func captureLegacy24ExtrasTx(ctx context.Context, tx pgx.Tx) ([]legacy24Extra, error) {
	const namespaces = `n.nspname NOT IN ('public','information_schema','native_membership_guard','space_account_ban_guard') AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'`
	// Restrict the supported class to independent regular tables with built-in
	// column types and no expressions, inheritance, triggers or constraints.
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_event_trigger) AND NOT EXISTS(SELECT 1 FROM pg_publication) AND NOT EXISTS(SELECT 1 FROM pg_subscription)`).Scan(&valid); err != nil || !valid {
		return nil, errors.New("HOLD: unsupported Space database object")
	}
	query := `SELECT NOT EXISTS(
	 SELECT 1 FROM pg_namespace n WHERE ` + namespaces + ` AND (
	 EXISTS(SELECT 1 FROM pg_class c WHERE c.relnamespace=n.oid AND (c.relkind<>'r' OR c.relpersistence<>'p' OR c.relrowsecurity OR c.relforcerowsecurity)) OR
	 EXISTS(SELECT 1 FROM pg_proc p WHERE p.pronamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_operator o WHERE o.oprnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_collation o WHERE o.collnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_conversion o WHERE o.connamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_opclass o WHERE o.opcnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_opfamily o WHERE o.opfnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_extension o WHERE o.extnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_statistic_ext o WHERE o.stxnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_ts_config o WHERE o.cfgnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_ts_dict o WHERE o.dictnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_ts_parser o WHERE o.prsnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_ts_template o WHERE o.tmplnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_type t WHERE t.typnamespace=n.oid AND NOT EXISTS(SELECT 1 FROM pg_class c WHERE c.oid=t.typrelid AND c.relnamespace=n.oid AND c.relkind='r') AND NOT EXISTS(SELECT 1 FROM pg_type element JOIN pg_class c ON c.oid=element.typrelid WHERE element.oid=t.typelem AND c.relnamespace=n.oid AND c.relkind='r')) OR
	 EXISTS(SELECT 1 FROM pg_class c JOIN pg_attribute a ON a.attrelid=c.oid JOIN pg_type t ON t.oid=a.atttypid JOIN pg_namespace tn ON tn.oid=t.typnamespace WHERE c.relnamespace=n.oid AND a.attnum>0 AND NOT a.attisdropped AND (tn.nspname<>'pg_catalog' OR a.atthasdef OR a.attgenerated<>'' OR a.attidentity<>'')) OR
	 EXISTS(SELECT 1 FROM pg_class c JOIN pg_attribute a ON a.attrelid=c.oid WHERE c.relnamespace=n.oid AND a.attnum>0 AND a.attisdropped) OR
	 EXISTS(SELECT 1 FROM pg_class c JOIN pg_constraint k ON k.conrelid=c.oid WHERE c.relnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_class c JOIN pg_rewrite r ON r.ev_class=c.oid WHERE c.relnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_class c JOIN pg_trigger t ON t.tgrelid=c.oid WHERE c.relnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_class c JOIN pg_inherits i ON i.inhrelid=c.oid OR i.inhparent=c.oid WHERE c.relnamespace=n.oid) OR
	 EXISTS(SELECT 1 FROM pg_class c JOIN pg_policy p ON p.polrelid=c.oid WHERE c.relnamespace=n.oid)
	))`
	if err := tx.QueryRow(ctx, query).Scan(&valid); err != nil || !valid {
		return nil, errors.New("HOLD: unsupported independent Space object")
	}
	rows, err := tx.Query(ctx, `SELECT n.nspname,c.relname,jsonb_build_array(pg_get_userbyid(n.nspowner),n.nspacl,pg_get_userbyid(c.relowner),c.relacl,(SELECT jsonb_agg(jsonb_build_array(a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attcollation,a.attacl) ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped))::text FROM pg_namespace n JOIN pg_class c ON c.relnamespace=n.oid WHERE `+namespaces+` ORDER BY n.nspname,c.relname LIMIT 1001`)
	if err != nil {
		return nil, errors.New("HOLD: independent Space inventory unavailable")
	}
	objects := make([]legacy24Extra, 0)
	for rows.Next() {
		var o legacy24Extra
		if rows.Scan(&o.Schema, &o.Table, &o.Definition) != nil || len(objects) >= 1000 {
			rows.Close()
			return nil, errors.New("HOLD: independent Space inventory unavailable")
		}
		objects = append(objects, o)
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, errors.New("HOLD: independent Space inventory unavailable")
	}
	rows.Close()
	var total uint64
	for i := range objects {
		o := &objects[i]
		name := pgx.Identifier{o.Schema, o.Table}.Sanitize()
		if _, err = tx.Exec(ctx, `LOCK TABLE `+name+` IN ACCESS SHARE MODE`); err != nil {
			return nil, errors.New("HOLD: independent Space table unavailable")
		}
		q := fmt.Sprintf(`SELECT CASE WHEN octet_length(to_jsonb(t)::text)<=%d THEN to_jsonb(t)::text ELSE NULL END FROM %s t ORDER BY to_jsonb(t)::text COLLATE "C" LIMIT %d`, 8<<20, name, 1000001)
		values, e := tx.Query(ctx, q)
		if e != nil {
			return nil, errors.New("HOLD: independent Space table unavailable")
		}
		h := sha256.New()
		for values.Next() {
			var value *string
			if values.Scan(&value) != nil || value == nil {
				values.Close()
				return nil, errors.New("HOLD: independent Space table unavailable")
			}
			o.Count++
			total += uint64(len(*value))
			if o.Count > 1000000 || total > 512<<20 {
				values.Close()
				return nil, errors.New("HOLD: independent Space table exceeds bound")
			}
			var length [8]byte
			binary.BigEndian.PutUint64(length[:], uint64(len(*value)))
			_, _ = h.Write(length[:])
			_, _ = h.Write([]byte(*value))
		}
		if values.Err() != nil {
			values.Close()
			return nil, errors.New("HOLD: independent Space table unavailable")
		}
		values.Close()
		o.SHA256 = hex.EncodeToString(h.Sum(nil))
	}
	// An empty non-reserved schema cannot be observed via the table inventory.
	var schemas int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM pg_namespace n WHERE `+namespaces).Scan(&schemas); err != nil {
		return nil, errors.New("HOLD: independent Space inventory unavailable")
	}
	seen := map[string]bool{}
	for _, o := range objects {
		seen[o.Schema] = true
	}
	if len(seen) != schemas {
		return nil, errors.New("HOLD: unsupported independent Space schema")
	}
	return objects, nil
}
