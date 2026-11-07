package nativeinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Capture only original table/column ACLs and stable database/role properties;
// newly created native objects are intentionally outside this projection.
func captureLegacy24StableAccessTx(ctx context.Context, tx pgx.Tx, original map[string]legacy25TableDigest) (string, error) {
	if len(original) != len(legacy25DataTables) {
		return "", errors.New("HOLD: original access projection unavailable")
	}
	var common string
	err := tx.QueryRow(ctx, `SELECT jsonb_build_object(
 'database',(SELECT jsonb_build_array(d.datname,pg_get_userbyid(d.datdba),d.encoding,d.datlocprovider,d.datcollate,d.datctype,d.daticulocale,d.datcollversion,t.spcname,d.datconnlimit,d.datallowconn,d.datistemplate,d.datacl) FROM pg_database d JOIN pg_tablespace t ON t.oid=d.dattablespace WHERE d.datname='space_db'),
 'settings',(SELECT jsonb_agg(jsonb_build_array(CASE WHEN s.setrole=0 THEN '' ELSE pg_get_userbyid(s.setrole) END,s.setconfig) ORDER BY s.setrole) FROM pg_db_role_setting s JOIN pg_database d ON d.oid=s.setdatabase WHERE d.datname='space_db'),
 'schema',(SELECT jsonb_build_array(pg_get_userbyid(n.nspowner),n.nspacl) FROM pg_namespace n WHERE n.nspname='public'),
 'roles',(SELECT jsonb_agg(jsonb_build_array(r.rolname,r.rolsuper,r.rolinherit,r.rolcreaterole,r.rolcreatedb,r.rolcanlogin,r.rolreplication,r.rolbypassrls,r.rolconnlimit,r.rolvaliduntil,r.rolconfig) ORDER BY r.rolname) FROM pg_roles r),
 'memberships',(SELECT jsonb_agg(jsonb_build_array(pg_get_userbyid(m.roleid),pg_get_userbyid(m.member),pg_get_userbyid(m.grantor),m.admin_option,m.inherit_option,m.set_option) ORDER BY pg_get_userbyid(m.roleid),pg_get_userbyid(m.member),pg_get_userbyid(m.grantor)) FROM pg_auth_members m),
 'defaults',(SELECT jsonb_agg(jsonb_build_array(pg_get_userbyid(a.defaclrole),COALESCE(n.nspname,''),a.defaclobjtype,a.defaclacl) ORDER BY pg_get_userbyid(a.defaclrole),COALESCE(n.nspname,''),a.defaclobjtype) FROM pg_default_acl a LEFT JOIN pg_namespace n ON n.oid=a.defaclnamespace))::text`).Scan(&common)
	if err != nil || len(common) > 1<<20 {
		return "", errors.New("HOLD: original access projection unavailable")
	}
	values := make([]string, 0, len(legacy25DataTables)+2)
	values = append(values, common)
	tables := append(append([]string(nil), legacy25DataTables[:]...), "schema_migrations")
	for _, table := range tables {
		prior, ok := original[table]
		if table == "schema_migrations" {
			prior, ok = legacy25TableDigest{Columns: []string{"version", "dirty"}}, true
		}
		if !ok || len(prior.Columns) == 0 {
			return "", errors.New("HOLD: original access projection unavailable")
		}
		var value string
		var count int
		err = tx.QueryRow(ctx, `SELECT jsonb_build_array(pg_get_userbyid(c.relowner),c.relacl,(SELECT jsonb_agg(jsonb_build_array(a.attname,a.attacl) ORDER BY array_position($2::text[],a.attname::text)) FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped AND a.attname::text=ANY($2::text[])))::text,(SELECT count(*) FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped AND a.attname::text=ANY($2::text[])) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=$1`, table, prior.Columns).Scan(&value, &count)
		if err != nil || count != len(prior.Columns) {
			return "", errors.New("HOLD: original access projection unavailable")
		}
		values = append(values, value)
	}
	raw, err := json.Marshal(values)
	if err != nil || len(raw) > 1<<20 {
		return "", errors.New("HOLD: original access projection unavailable")
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
