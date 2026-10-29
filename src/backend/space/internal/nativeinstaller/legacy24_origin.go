package nativeinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/store"
)

// Detached private observation only. Neither these hashes nor the catalog
// comparison assert that a caller possesses backup or maintenance authority.
type legacy24OriginObservation struct {
	DatabaseOwner      string                            `json:"-"`
	Target             *store.NativeInstallerSourceFacts `json:"target"`
	CatalogSHA256      string                            `json:"catalog_sha256"`
	PreimageSHA256     string                            `json:"preimage_sha256"`
	AccessSHA256       string                            `json:"access_sha256"`
	StableAccessSHA256 string                            `json:"stable_access_sha256"`
	Rows               map[string]legacy25TableDigest    `json:"rows"`
	Derived            *legacy24Derived                  `json:"derived"`
	Spaces             []string                          `json:"spaces"`
	Extras             []legacy24Extra                   `json:"extras"`
	Functions          []legacy24FunctionAccess          `json:"functions"`
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
 'columns',(SELECT jsonb_agg(jsonb_build_array(c.relname,a.attname,a.attacl) ORDER BY c.relname,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND a.attnum>0 AND NOT a.attisdropped),
 'sequences',(SELECT jsonb_agg(jsonb_build_array(s.schemaname,s.sequencename,s.sequenceowner,s.data_type::text,s.start_value,s.min_value,s.max_value,s.increment_by,s.cycle,s.cache_size,s.last_value) ORDER BY s.schemaname,s.sequencename) FROM pg_sequences s WHERE s.schemaname='public'),
 'functions',(SELECT jsonb_agg(jsonb_build_array(p.proname,pg_get_function_identity_arguments(p.oid),pg_get_userbyid(p.proowner),p.proacl) ORDER BY p.proname,pg_get_function_identity_arguments(p.oid)) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public'),
 'extensions',(SELECT jsonb_agg(jsonb_build_array(e.extname,pg_get_userbyid(e.extowner)) ORDER BY e.extname) FROM pg_extension e),
 'roles',(SELECT jsonb_agg(jsonb_build_array(r.rolname,r.rolsuper,r.rolinherit,r.rolcreaterole,r.rolcreatedb,r.rolcanlogin,r.rolreplication,r.rolbypassrls,r.rolconnlimit,r.rolvaliduntil,r.rolconfig) ORDER BY r.rolname) FROM pg_roles r),
 'defaults',(SELECT jsonb_agg(jsonb_build_array(pg_get_userbyid(a.defaclrole),COALESCE(n.nspname,''),a.defaclobjtype,a.defaclacl) ORDER BY pg_get_userbyid(a.defaclrole),COALESCE(n.nspname,''),a.defaclobjtype) FROM pg_default_acl a LEFT JOIN pg_namespace n ON n.oid=a.defaclnamespace),
 'memberships',(SELECT jsonb_agg(jsonb_build_array(pg_get_userbyid(m.roleid),pg_get_userbyid(m.member),pg_get_userbyid(m.grantor),m.admin_option,m.inherit_option,m.set_option) ORDER BY pg_get_userbyid(m.roleid),pg_get_userbyid(m.member),pg_get_userbyid(m.grantor)) FROM pg_auth_members m)
)::text`

func observeLegacy24Origin(ctx context.Context, conn *pgx.Conn) (_ *legacy24OriginObservation, err error) {
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer finishLegacy24Read(tx, &err)
	if err = qualifyLegacy24InstallerTx(ctx, tx); err != nil {
		return nil, err
	}
	if err = qualifyLegacy24LifecycleTx(ctx, tx); err != nil {
		return nil, err
	}
	catalog, err := inspectLegacyCatalogTx(ctx, tx, 24, legacy24CatalogPG16SHA256)
	if err != nil {
		return nil, err
	}
	var noNativeSchema bool
	if err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname IN ('native_membership_guard','space_account_ban_guard'))`).Scan(&noNativeSchema); err != nil || !noNativeSchema {
		return nil, errors.New("HOLD: legacy24 native schema already exists")
	}
	var name, role, current, system, owner string
	var oid uint32
	if err = tx.QueryRow(ctx, `SELECT d.datname,d.oid,session_user,current_user,(SELECT system_identifier::text FROM pg_catalog.pg_control_system()),pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datname='space_db'`).Scan(&name, &oid, &role, &current, &system, &owner); err != nil ||
		name != "space_db" || oid == 0 || role == "" || owner == "" || role != current || !regexp.MustCompile(`^[1-9][0-9]{0,19}$`).MatchString(system) {
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
	stableAccess, err := captureLegacy24StableAccessTx(ctx, tx, rows)
	if err != nil {
		return nil, err
	}
	functions, err := captureLegacy24FunctionsTx(ctx, tx, nil)
	if err != nil {
		return nil, err
	}
	derived, err := captureLegacy24DerivedTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	extras, err := captureLegacy24ExtrasTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	ids, err := tx.Query(ctx, `SELECT id::text FROM public.spaces ORDER BY id LIMIT 1000001`)
	if err != nil {
		return nil, errors.New("legacy24 spaces unavailable")
	}
	spaces := make([]string, 0)
	for ids.Next() {
		var id string
		if ids.Scan(&id) != nil || len(spaces) >= 1000000 {
			ids.Close()
			return nil, errors.New("legacy24 spaces unavailable")
		}
		parsed, parseErr := uuid.Parse(id)
		if parseErr != nil || parsed == uuid.Nil || parsed.String() != id {
			ids.Close()
			return nil, errors.New("legacy24 spaces unavailable")
		}
		spaces = append(spaces, id)
	}
	if ids.Err() != nil {
		ids.Close()
		return nil, errors.New("legacy24 spaces unavailable")
	}
	ids.Close()
	if uint64(len(spaces)) != rows["spaces"].Count {
		return nil, errors.New("legacy24 spaces unavailable")
	}
	encoded, err := json.Marshal(struct {
		Rows               map[string]legacy25TableDigest `json:"rows"`
		Derived            *legacy24Derived               `json:"derived"`
		Spaces             []string                       `json:"spaces"`
		Extras             []legacy24Extra                `json:"extras"`
		StableAccessSHA256 string                         `json:"stable_access_sha256"`
		Functions          []legacy24FunctionAccess       `json:"functions"`
	}{rows, derived, spaces, extras, stableAccess, functions})
	if err != nil || len(encoded) > 1<<20 {
		return nil, errors.New("legacy24 preimage unavailable")
	}
	preimageHash := sha256.Sum256(encoded)
	if err = tx.Commit(ctx); err != nil {
		return nil, errors.New("legacy24 source unavailable")
	}
	return &legacy24OriginObservation{DatabaseOwner: owner, Target: &store.NativeInstallerSourceFacts{Version: 24, DatabaseSystemID: system,
		DatabaseName: name, DatabaseOID: oid, InstallerRole: role}, CatalogSHA256: catalog,
		PreimageSHA256: hex.EncodeToString(preimageHash[:]), AccessSHA256: accessHash, StableAccessSHA256: stableAccess, Rows: rows, Derived: derived, Spaces: spaces, Extras: extras, Functions: functions}, nil
}

func captureLegacyAccessTx(ctx context.Context, tx pgx.Tx) (string, error) {
	var sequenceAccess bool
	if err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='S' AND NOT has_sequence_privilege(session_user,c.oid,'SELECT'))`).Scan(&sequenceAccess); err != nil || !sequenceAccess {
		return "", errors.New("legacy24 sequence observation unavailable")
	}
	var digest string
	var size int
	query := `SELECT encode(sha256(convert_to(x.value,'UTF8')),'hex'),octet_length(x.value) FROM (` + legacy24AccessSQL + `) AS x(value)`
	if err := tx.QueryRow(ctx, query).Scan(&digest, &size); err != nil || size <= 0 || size > 1<<20 || len(digest) != 64 {
		return "", errors.New("legacy24 access observation unavailable")
	}
	rows, err := tx.Query(ctx, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='S' ORDER BY c.relname LIMIT 10001`)
	if err != nil {
		return "", errors.New("legacy24 sequence observation unavailable")
	}
	var names []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) != nil || len(names) >= 10000 {
			rows.Close()
			return "", errors.New("legacy24 sequence observation unavailable")
		}
		names = append(names, name)
	}
	if rows.Err() != nil {
		rows.Close()
		return "", errors.New("legacy24 sequence observation unavailable")
	}
	rows.Close()
	type sequenceState struct {
		Name   string `json:"name"`
		Last   int64  `json:"last"`
		Called bool   `json:"called"`
	}
	states := make([]sequenceState, 0, len(names))
	for _, name := range names {
		state := sequenceState{Name: name}
		if err = tx.QueryRow(ctx, `SELECT last_value,is_called FROM `+pgx.Identifier{"public", name}.Sanitize()).Scan(&state.Last, &state.Called); err != nil {
			return "", errors.New("legacy24 sequence observation unavailable")
		}
		states = append(states, state)
	}
	encoded, err := json.Marshal(struct {
		Access    string          `json:"access"`
		Sequences []sequenceState `json:"sequences"`
	}{digest, states})
	if err != nil || len(encoded) > 1<<20 {
		return "", errors.New("legacy24 access observation unavailable")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
