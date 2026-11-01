package sourcecustody

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This dormant contract is not an upgrade, legacy-controller fence, source
// stop, or admission authority. Each challenge is capped, but lifetime
// retention and safe cleanup remain unimplemented production prerequisites.
// Opener is the separately authenticated opening controller; Controller is
// the restricted probe-publisher identity and cannot perform opening.
type monitorSchemaRoles struct{ Admin, Owner, Opener, Controller, Responder string }

const monitorProbeFunctionBody = `
BEGIN
 IF NOT EXISTS(SELECT 1 FROM public.user_custody_open_evidence e WHERE e.challenge=NEW.challenge AND pg_catalog.sha256(e.payload)=NEW.decision_digest) THEN
  RAISE EXCEPTION 'invalid monitor probe binding';
 END IF;
 RETURN NEW;
END
`

const monitorSchemaDDL = `
CREATE TABLE public.user_custody_monitor_probe(
 challenge bytea NOT NULL,sequence bigint NOT NULL,decision_digest bytea NOT NULL,nonce bytea NOT NULL,
 CONSTRAINT monitor_probe_pk PRIMARY KEY(challenge,sequence),
 CONSTRAINT monitor_probe_nonce_unique UNIQUE(challenge,sequence,nonce),
 CONSTRAINT monitor_probe_evidence_fk FOREIGN KEY(challenge) REFERENCES public.user_custody_open_evidence(challenge),
 CONSTRAINT monitor_probe_sequence_check CHECK(sequence BETWEEN 1 AND 86400),
 CONSTRAINT monitor_probe_challenge_check CHECK(octet_length(challenge)=32 AND challenge<>decode(repeat('00',32),'hex')),
 CONSTRAINT monitor_probe_digest_check CHECK(octet_length(decision_digest)=32 AND decision_digest<>decode(repeat('00',32),'hex')),
 CONSTRAINT monitor_probe_nonce_check CHECK(octet_length(nonce)=32 AND nonce<>decode(repeat('00',32),'hex')));
CREATE TABLE public.user_custody_monitor_response(
 challenge bytea NOT NULL,sequence bigint NOT NULL,kind smallint NOT NULL,nonce bytea NOT NULL,incarnation uuid NOT NULL,signature bytea NOT NULL,
 CONSTRAINT monitor_response_pk PRIMARY KEY(challenge,sequence,kind),
 CONSTRAINT monitor_response_probe_fk FOREIGN KEY(challenge,sequence,nonce) REFERENCES public.user_custody_monitor_probe(challenge,sequence,nonce),
 CONSTRAINT monitor_response_kind_check CHECK((kind=1 AND sequence=1) OR (kind=2 AND sequence>1) OR kind=3),
 CONSTRAINT monitor_response_incarnation_check CHECK(incarnation<>'00000000-0000-0000-0000-000000000000'::uuid),
 CONSTRAINT monitor_response_signature_check CHECK(octet_length(signature)=64));
CREATE UNIQUE INDEX monitor_response_one_stop ON public.user_custody_monitor_response(challenge) WHERE kind=3;
CREATE FUNCTION public.user_custody_monitor_probe_validate() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $monitor$` + monitorProbeFunctionBody + `$monitor$;
CREATE TRIGGER user_custody_monitor_probe_validate BEFORE INSERT ON public.user_custody_monitor_probe FOR EACH ROW EXECUTE FUNCTION public.user_custody_monitor_probe_validate();`

// qualifyMonitorSchema checks only these supplied sessions during this call. It
// assumes trusted PostgreSQL host/admin/network and excludes clone/restore
// custody. It does not fence old credentials or authorize future connections;
// READ COMMITTED uses multiple snapshots and these table locks do not fence
// role/catalog DDL or prepared changes. A production caller must establish
// quiescence, hold appropriate locks, and qualify each fresh connection at use.
func qualifyMonitorSchema(ctx context.Context, admin, opener, controller, responder *pgxpool.Conn, roles monitorSchemaRoles) error {
	if ctx == nil || ctx.Err() != nil || admin == nil || opener == nil || controller == nil || responder == nil {
		return ErrUnavailable
	}
	names := []string{roles.Admin, roles.Owner, roles.Opener, roles.Controller, roles.Responder}
	seen := map[string]bool{}
	for _, name := range names {
		if !databaseName.MatchString(name) || seen[name] {
			return ErrUnavailable
		}
		seen[name] = true
	}
	tx, err := admin.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := tx.Rollback(c); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			_ = admin.Conn().Close(c)
		}
	}()
	if _, err = tx.Exec(ctx, `SET LOCAL search_path=pg_catalog,public,pg_temp`); err != nil {
		return ErrUnavailable
	}
	if _, err = tx.Exec(ctx, `LOCK TABLE public.user_custody_open_evidence,public.user_custody_monitor_probe,public.user_custody_monitor_response IN ACCESS SHARE MODE`); err != nil {
		return ErrUnavailable
	}
	// Independent sessions must actually reach the same database, not merely
	// report the same database name or restored identity. A random advisory lock
	// is scoped to this physical database; inability to establish it fails shut.
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		return ErrUnavailable
	}
	key := int64(binary.BigEndian.Uint64(random[:]))
	var ok bool
	if tx.QueryRow(ctx, `SELECT current_user=$1 AND session_user=$1 AND current_setting('session_replication_role')='origin' AND (SELECT rolsuper AND rolcanlogin FROM pg_roles WHERE rolname=$1) AND pg_try_advisory_xact_lock($2::bigint)`, roles.Admin, key).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	for _, pair := range []struct {
		conn *pgxpool.Conn
		role string
	}{{opener, roles.Opener}, {controller, roles.Controller}, {responder, roles.Responder}} {
		t, err := pair.conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			return ErrUnavailable
		}
		if _, err = t.Exec(ctx, `SET LOCAL search_path=pg_catalog,pg_temp`); err != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if t.Rollback(cleanup) != nil {
				_ = pair.conn.Conn().Close(cleanup)
			}
			cancel()
			return ErrUnavailable
		}
		var valid bool
		var pid int32
		var started time.Time
		err = t.QueryRow(ctx, `SELECT session_user=$1 AND current_user=$1 AND current_setting('session_replication_role')='origin' AND NOT pg_try_advisory_xact_lock_shared($2::bigint),pg_backend_pid(),(SELECT backend_start FROM pg_stat_activity WHERE pid=pg_backend_pid())`, pair.role, key).Scan(&valid, &pid, &started)
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		rollbackErr := t.Rollback(cleanup)
		if rollbackErr != nil {
			_ = pair.conn.Conn().Close(cleanup)
		}
		cancel()
		if err != nil || rollbackErr != nil || !valid {
			return ErrUnavailable
		}
		// Match the connected backend's identity against the admin's live
		// catalog as well; a lock collision alone is not session identity.
		if _, err = tx.Exec(ctx, `SELECT pg_stat_clear_snapshot()`); err != nil {
			return ErrUnavailable
		}
		if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND backend_start=$2 AND usename=$3 AND datid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND backend_type='client backend')`, pid, started, pair.role).Scan(&valid) != nil || !valid {
			return ErrUnavailable
		}
	}
	if tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_roles WHERE rolname=ANY($1::text[]))=5
 AND EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$2 AND NOT (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls))
 AND (SELECT count(*) FROM pg_roles WHERE rolname=ANY($3::text[]) AND rolcanlogin AND NOT (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls))=3
 AND NOT EXISTS(SELECT 1 FROM pg_auth_members m JOIN pg_roles a ON a.oid=m.member JOIN pg_roles b ON b.oid=m.roleid WHERE a.rolname=ANY($1::text[]) OR b.rolname=ANY($1::text[]))
 AND NOT EXISTS(SELECT 1 FROM pg_db_role_setting s CROSS JOIN LATERAL unnest(s.setconfig) setting WHERE (s.setrole=0 OR s.setrole IN (SELECT oid FROM pg_roles WHERE rolname=ANY($1::text[]))) AND (s.setdatabase=0 OR s.setdatabase=(SELECT oid FROM pg_database WHERE datname=current_database())) AND split_part(setting,'=',1)='session_replication_role')
 AND NOT EXISTS(SELECT 1 FROM pg_namespace n CROSS JOIN LATERAL aclexplode(COALESCE(n.nspacl,acldefault('n',n.nspowner))) a WHERE n.nspname='public' AND a.grantee=0 AND a.privilege_type='CREATE')
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_namespace n ON true WHERE r.rolname=ANY($3::text[]) AND n.oid<>pg_my_temp_schema() AND NOT pg_is_other_temp_schema(n.oid) AND has_schema_privilege(r.oid,n.oid,'CREATE'))`, names, roles.Owner, []string{roles.Opener, roles.Controller, roles.Responder}).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	// Exact object kinds, owners and columns; the protected evidence table is
	// qualified separately below for the challenge/payload reference contract.
	columns := map[string][]string{
		"user_custody_monitor_probe":    {"challenge:bytea", "sequence:bigint", "decision_digest:bytea", "nonce:bytea"},
		"user_custody_monitor_response": {"challenge:bytea", "sequence:bigint", "kind:smallint", "nonce:bytea", "incarnation:uuid", "signature:bytea"},
	}
	for table, want := range columns {
		var got []string
		err = tx.QueryRow(ctx, `SELECT array_agg(a.attname||':'||format_type(a.atttypid,a.atttypmod) ORDER BY a.attnum) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 WHERE n.nspname='public' AND c.relname=$1 AND c.relkind='r' AND c.relpersistence='p' AND NOT c.relispartition AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND NOT c.relhassubclass AND NOT EXISTS(SELECT 1 FROM pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid) AND pg_get_userbyid(c.relowner)=$2 AND NOT a.attisdropped AND a.attnotnull AND a.attidentity='' AND a.attgenerated='' AND NOT EXISTS(SELECT 1 FROM pg_attrdef d WHERE d.adrelid=c.oid AND d.adnum=a.attnum)`, table, roles.Owner).Scan(&got)
		if err != nil || !slices.Equal(got, want) {
			return ErrUnavailable
		}
		var count int
		if tx.QueryRow(ctx, `SELECT count(*) FROM pg_attribute WHERE attrelid=('public.'||$1)::regclass AND attnum>0`, table).Scan(&count) != nil || count != len(want) {
			return ErrUnavailable
		}
	}
	wantConstraints := map[string]string{

		"monitor_probe_pk": "PRIMARY KEY (challenge, sequence)", "monitor_probe_nonce_unique": "UNIQUE (challenge, sequence, nonce)", "monitor_probe_evidence_fk": "FOREIGN KEY (challenge) REFERENCES user_custody_open_evidence(challenge)",
		"monitor_probe_sequence_check":  "CHECK (((sequence >= 1) AND (sequence <= 86400)))",
		"monitor_probe_challenge_check": "CHECK (((octet_length(challenge) = 32) AND (challenge <> decode(repeat('00'::text, 32), 'hex'::text))))",
		"monitor_probe_digest_check":    "CHECK (((octet_length(decision_digest) = 32) AND (decision_digest <> decode(repeat('00'::text, 32), 'hex'::text))))",
		"monitor_probe_nonce_check":     "CHECK (((octet_length(nonce) = 32) AND (nonce <> decode(repeat('00'::text, 32), 'hex'::text))))",
		"monitor_response_pk":           "PRIMARY KEY (challenge, sequence, kind)", "monitor_response_probe_fk": "FOREIGN KEY (challenge, sequence, nonce) REFERENCES user_custody_monitor_probe(challenge, sequence, nonce)",
		"monitor_response_kind_check":        "CHECK ((((kind = 1) AND (sequence = 1)) OR ((kind = 2) AND (sequence > 1)) OR (kind = 3)))",
		"monitor_response_incarnation_check": "CHECK ((incarnation <> '00000000-0000-0000-0000-000000000000'::uuid))", "monitor_response_signature_check": "CHECK ((octet_length(signature) = 64))",
	}
	rows, err := tx.Query(ctx, `SELECT c.relname,k.conname,pg_get_constraintdef(k.oid),k.convalidated,k.condeferrable,k.condeferred FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid WHERE k.contype<>'n' AND k.conrelid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass)`)
	if err != nil {
		return ErrUnavailable
	}
	count := 0
	for rows.Next() {
		var table, name, definition string
		var valid, deferrable, deferred bool
		if rows.Scan(&table, &name, &definition, &valid, &deferrable, &deferred) != nil || !valid || deferrable || deferred || wantConstraints[name] != definition || (strings.HasPrefix(name, "monitor_probe_") && table != "user_custody_monitor_probe") || (strings.HasPrefix(name, "monitor_response_") && table != "user_custody_monitor_response") {
			rows.Close()
			return ErrUnavailable
		}
		delete(wantConstraints, name)
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil || count != 12 || len(wantConstraints) != 0 {
		return ErrUnavailable
	}
	wantIndexes := map[string]string{
		"monitor_probe_nonce_unique": "CREATE UNIQUE INDEX monitor_probe_nonce_unique ON public.user_custody_monitor_probe USING btree (challenge, sequence, nonce)",
		"monitor_probe_pk":           "CREATE UNIQUE INDEX monitor_probe_pk ON public.user_custody_monitor_probe USING btree (challenge, sequence)",
		"monitor_response_one_stop":  "CREATE UNIQUE INDEX monitor_response_one_stop ON public.user_custody_monitor_response USING btree (challenge) WHERE (kind = 3)",
		"monitor_response_pk":        "CREATE UNIQUE INDEX monitor_response_pk ON public.user_custody_monitor_response USING btree (challenge, sequence, kind)",
	}
	rows, err = tx.Query(ctx, `SELECT c.relname,pg_get_indexdef(i.indexrelid) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE i.indrelid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass)`)
	if err != nil {
		return ErrUnavailable
	}
	count = 0
	for rows.Next() {
		var name, definition string
		if rows.Scan(&name, &definition) != nil || wantIndexes[name] != definition {
			rows.Close()
			return ErrUnavailable
		}
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil || count != len(wantIndexes) {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM pg_index WHERE indrelid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass))=4
 AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE contype='n' AND conrelid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass) AND NOT convalidated)
 AND NOT EXISTS(SELECT 1 FROM pg_index WHERE indrelid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass) AND (NOT indisvalid OR NOT indisready OR NOT indislive OR indexprs IS NOT NULL))
 AND EXISTS(SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relname='monitor_response_one_stop' AND i.indrelid='public.user_custody_monitor_response'::regclass AND i.indisunique AND i.indkey::text='1' AND pg_get_expr(i.indpred,i.indrelid)='(kind = 3)')
 AND (SELECT count(*) FROM pg_trigger WHERE tgrelid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass,'public.user_custody_open_evidence'::regclass) AND NOT tgisinternal)=1
 AND EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.user_custody_monitor_probe'::regclass AND tgname='user_custody_monitor_probe_validate' AND tgenabled='O' AND tgtype=7 AND tgfoid='public.user_custody_monitor_probe_validate()'::regprocedure AND tgqual IS NULL AND tgnargs=0 AND tgattr::text='' AND tgconstraint=0 AND tgoldtable IS NULL AND tgnewtable IS NULL)
 AND (SELECT count(*) FROM pg_trigger WHERE tgrelid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass,'public.user_custody_open_evidence'::regclass) AND tgisinternal)=8
 AND NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass,'public.user_custody_open_evidence'::regclass) AND tgisinternal AND tgenabled<>'O')
 AND EXISTS(SELECT 1 FROM pg_proc p JOIN pg_language l ON l.oid=p.prolang WHERE p.oid='public.user_custody_monitor_probe_validate()'::regprocedure AND pg_get_userbyid(p.proowner)=$1 AND l.lanname='plpgsql' AND NOT p.prosecdef AND NOT p.proisstrict AND NOT p.proleakproof AND NOT p.proretset AND p.prokind='f' AND p.provolatile='v' AND p.proparallel='u' AND p.prosupport=0 AND p.proargdefaults IS NULL AND p.prorettype='trigger'::regtype AND p.pronargs=0 AND p.proconfig=ARRAY['search_path=pg_catalog'] AND p.prosrc=$2)
 AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE((SELECT proacl FROM pg_proc WHERE oid='public.user_custody_monitor_probe_validate()'::regprocedure),acldefault('f',(SELECT proowner FROM pg_proc WHERE oid='public.user_custody_monitor_probe_validate()'::regprocedure)))) a WHERE a.grantee=0 OR pg_get_userbyid(a.grantee)<>$1)
 AND EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='user_custody_open_evidence' AND c.relkind='r' AND c.relpersistence='p' AND NOT c.relispartition AND pg_get_userbyid(c.relowner)=$1 AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND NOT c.relhassubclass AND NOT EXISTS(SELECT 1 FROM pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid))
 AND EXISTS(SELECT 1 FROM pg_constraint k JOIN pg_index i ON i.indexrelid=k.conindid WHERE k.conrelid='public.user_custody_open_evidence'::regclass AND k.contype='p' AND k.convalidated AND NOT k.condeferrable AND pg_get_constraintdef(k.oid)='PRIMARY KEY (challenge)' AND i.indisvalid AND i.indisready AND i.indislive AND pg_get_indexdef(i.indexrelid)='CREATE UNIQUE INDEX user_custody_open_evidence_pkey ON public.user_custody_open_evidence USING btree (challenge)')
 AND (SELECT count(*) FROM pg_attribute a WHERE a.attrelid='public.user_custody_open_evidence'::regclass AND a.attname IN ('challenge','payload') AND NOT a.attisdropped AND a.attnotnull AND a.atttypid='bytea'::regtype AND a.attidentity='' AND a.attgenerated='' AND NOT EXISTS(SELECT 1 FROM pg_attrdef d WHERE d.adrelid=a.attrelid AND d.adnum=a.attnum))=2
 AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass,'public.user_custody_open_evidence'::regclass))`, roles.Owner, monitorProbeFunctionBody).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	// Reject any column grant, public ACL, unknown grantee or grant option. The
	// two runtime roles have exactly the transport privileges, not DDL/DELETE.
	if tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid WHERE c.oid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass,'public.user_custody_open_evidence'::regclass) AND a.attacl IS NOT NULL)
 AND NOT EXISTS(SELECT 1 FROM pg_class c CROSS JOIN LATERAL aclexplode(COALESCE(c.relacl,acldefault('r',c.relowner))) a WHERE c.oid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass,'public.user_custody_open_evidence'::regclass) AND (a.grantee=0 OR pg_get_userbyid(a.grantee)<>ALL($1::text[]) OR (a.is_grantable AND pg_get_userbyid(a.grantee)<>$2)))
 AND has_table_privilege($3,'public.user_custody_monitor_probe','SELECT') AND has_table_privilege($3,'public.user_custody_monitor_probe','INSERT') AND has_table_privilege($3,'public.user_custody_monitor_response','SELECT') AND has_table_privilege($3,'public.user_custody_open_evidence','SELECT')
 AND has_table_privilege($4,'public.user_custody_monitor_probe','SELECT') AND has_table_privilege($4,'public.user_custody_monitor_response','SELECT') AND has_table_privilege($4,'public.user_custody_monitor_response','INSERT')
 AND has_table_privilege($6,'public.user_custody_open_evidence','SELECT') AND has_table_privilege($6,'public.user_custody_open_evidence','INSERT') AND NOT has_table_privilege($6,'public.user_custody_monitor_probe','SELECT') AND NOT has_table_privilege($6,'public.user_custody_monitor_probe','INSERT') AND NOT has_table_privilege($6,'public.user_custody_monitor_response','SELECT') AND NOT has_table_privilege($6,'public.user_custody_monitor_response','INSERT')
 AND NOT EXISTS(SELECT 1 FROM pg_roles r CROSS JOIN pg_class c CROSS JOIN unnest(ARRAY['UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER']) p WHERE r.rolname=ANY($5::text[]) AND c.oid IN ('public.user_custody_monitor_probe'::regclass,'public.user_custody_monitor_response'::regclass,'public.user_custody_open_evidence'::regclass) AND has_table_privilege(r.oid,c.oid,p))
 AND NOT has_table_privilege($3,'public.user_custody_monitor_response','INSERT') AND NOT has_table_privilege($4,'public.user_custody_monitor_probe','INSERT') AND NOT has_table_privilege($4,'public.user_custody_open_evidence','INSERT') AND NOT has_table_privilege($4,'public.user_custody_open_evidence','SELECT') AND NOT has_table_privilege($3,'public.user_custody_open_evidence','INSERT')`, []string{roles.Owner, roles.Opener, roles.Controller, roles.Responder}, roles.Owner, roles.Controller, roles.Responder, []string{roles.Opener, roles.Controller, roles.Responder}, roles.Opener).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT
 NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_settings s ON true WHERE r.rolname=ANY($1::text[]) AND ((s.context IN ('superuser','superuser-backend') AND has_parameter_privilege(r.oid,s.name,'SET')) OR has_parameter_privilege(r.oid,s.name,'ALTER SYSTEM')))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_database d ON true WHERE r.rolname=ANY($1::text[]) AND has_database_privilege(r.oid,d.oid,'CREATE'))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_class c ON c.relowner=r.oid WHERE r.rolname=ANY($1::text[]))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_proc p ON p.proowner=r.oid WHERE r.rolname=ANY($1::text[]))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_proc p ON true WHERE r.rolname=ANY($1::text[]) AND p.prosecdef AND has_function_privilege(r.oid,p.oid,'EXECUTE'))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_proc p ON true JOIN pg_namespace n ON n.oid=p.pronamespace WHERE r.rolname=ANY($1::text[]) AND n.nspname='pg_catalog' AND has_function_privilege(r.oid,p.oid,'EXECUTE') AND (p.proname=ANY($2::text[]) OR NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')))`, []string{roles.Opener, roles.Controller, roles.Responder}, []string{"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file", "pg_ls_logdir", "pg_ls_waldir", "pg_ls_archive_statusdir", "pg_ls_tmpdir", "pg_ls_replslotdir", "pg_ls_logicalmapdir", "pg_ls_logicalsnapdir", "lo_import", "lo_export", "pg_reload_conf", "pg_rotate_logfile", "pg_promote", "pg_create_restore_point", "pg_switch_wal", "pg_backup_start", "pg_backup_stop"}).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	if tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}
