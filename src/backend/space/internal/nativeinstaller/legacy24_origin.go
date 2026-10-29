package nativeinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/store"
)

// Detached private observation only. Neither these hashes nor the catalog
// comparison assert that a caller possesses backup or maintenance authority.
type legacy24OriginObservation struct {
	Target         *store.NativeInstallerSourceFacts `json:"target"`
	CatalogSHA256  string                            `json:"catalog_sha256"`
	PreimageSHA256 string                            `json:"preimage_sha256"`
	AccessSHA256   string                            `json:"access_sha256"`
	Rows           map[string]legacy25TableDigest    `json:"rows"`
}

// The complete source-owned public object set was fixed by the catalog
// observation. This snapshot records its actual owners/ACLs, database and
// schema properties, role memberships and default privileges for comparison.
// It does not independently declare any observed role or grant safe.
const legacy24AccessSQL = `SELECT jsonb_build_object(
 'database',(SELECT jsonb_build_array(d.datname,pg_get_userbyid(d.datdba),d.encoding,d.datlocprovider,d.datcollate,d.datctype,d.daticulocale,d.datcollversion,t.spcname,d.datconnlimit,d.datallowconn,d.datistemplate,d.datacl) FROM pg_database d JOIN pg_tablespace t ON t.oid=d.dattablespace WHERE d.datname='space_db'),
 'settings',(SELECT jsonb_agg(jsonb_build_array(CASE WHEN s.setrole=0 THEN '' ELSE pg_get_userbyid(s.setrole) END,s.setconfig) ORDER BY s.setrole) FROM pg_db_role_setting s JOIN pg_database d ON d.oid=s.setdatabase WHERE d.datname='space_db'),
 'schema',(SELECT jsonb_build_array(n.nspname,pg_get_userbyid(n.nspowner),n.nspacl) FROM pg_namespace n WHERE n.nspname='public'),
 'relations',(SELECT jsonb_agg(jsonb_build_array(c.relname,pg_get_userbyid(c.relowner),c.relacl) ORDER BY c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'),
 'functions',(SELECT jsonb_agg(jsonb_build_array(p.proname,pg_get_function_identity_arguments(p.oid),pg_get_userbyid(p.proowner),p.proacl) ORDER BY p.proname,pg_get_function_identity_arguments(p.oid)) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public'),
 'defaults',(SELECT jsonb_agg(jsonb_build_array(pg_get_userbyid(a.defaclrole),COALESCE(n.nspname,''),a.defaclobjtype,a.defaclacl) ORDER BY pg_get_userbyid(a.defaclrole),COALESCE(n.nspname,''),a.defaclobjtype) FROM pg_default_acl a LEFT JOIN pg_namespace n ON n.oid=a.defaclnamespace),
 'memberships',(SELECT jsonb_agg(jsonb_build_array(pg_get_userbyid(m.roleid),pg_get_userbyid(m.member),pg_get_userbyid(m.grantor),m.admin_option,m.inherit_option,m.set_option) ORDER BY pg_get_userbyid(m.roleid),pg_get_userbyid(m.member),pg_get_userbyid(m.grantor)) FROM pg_auth_members m)
)::text`

func observeLegacy24Origin(ctx context.Context, conn *pgx.Conn) (*legacy24OriginObservation, error) {
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	catalog, err := inspectLegacyCatalogTx(ctx, tx, 24, legacy24CatalogPG16SHA256)
	if err != nil {
		return nil, err
	}
	var name, role, current, system string
	var oid uint32
	if err = tx.QueryRow(ctx, `SELECT d.datname,d.oid,session_user,current_user,(SELECT system_identifier::text FROM pg_catalog.pg_control_system()) FROM pg_catalog.pg_database d WHERE d.datname='space_db'`).Scan(&name, &oid, &role, &current, &system); err != nil ||
		name != "space_db" || oid == 0 || role == "" || role != current || !regexp.MustCompile(`^[1-9][0-9]{0,19}$`).MatchString(system) {
		return nil, errors.New("legacy24 target identity unavailable")
	}
	accessHash, err := captureLegacyAccessTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	rows, err := captureLegacy25ProjectionTx(ctx, tx, nil)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(rows)
	if err != nil || len(encoded) > 1<<20 {
		return nil, errors.New("legacy24 preimage unavailable")
	}
	preimageHash := sha256.Sum256(encoded)
	if err = tx.Commit(ctx); err != nil {
		return nil, errors.New("legacy24 source unavailable")
	}
	return &legacy24OriginObservation{Target: &store.NativeInstallerSourceFacts{Version: 24, DatabaseSystemID: system,
		DatabaseName: name, DatabaseOID: oid, InstallerRole: role}, CatalogSHA256: catalog,
		PreimageSHA256: hex.EncodeToString(preimageHash[:]), AccessSHA256: accessHash, Rows: rows}, nil
}

func captureLegacyAccessTx(ctx context.Context, tx pgx.Tx) (string, error) {
	var access string
	if err := tx.QueryRow(ctx, legacy24AccessSQL).Scan(&access); err != nil || len(access) == 0 || len(access) > 1<<20 {
		return "", errors.New("legacy24 access observation unavailable")
	}
	hash := sha256.Sum256([]byte(access))
	return hex.EncodeToString(hash[:]), nil
}
