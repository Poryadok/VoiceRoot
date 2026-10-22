package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type nativeReadinessFunctionPin struct {
	signature, body, language, result, volatility string
	definer, returnsSet                           bool
	config                                        string
}

var nativeMembershipGuardCoverage = []string{"spaces", "space_members", "space_ordinary_member_heads", "space_community_member_heads", "community_roster_members", "community_owner_authority", "space_membership_ownership_heads", "space_member_admission_intents", "space_native_member_publications", "space_native_creator_operations", "space_bot_installation_attempts", "space_bot_installation_heads", "space_bot_installation_cancellations", "space_effective_member_heads", "space_bans", "ownership_journal", "space_lifecycle_aggregates", "space_permanent_purge_fences", "community_owner_recovery_operations", "community_bootstrap_operations", "space_lifecycle_operations", "space_lifecycle_participants", "space_lifecycle_completions", "space_lifecycle_outbox", "space_deletion_tombstones"}

// Stable names/types/definitions, independent of installer OIDs and LOGIN names.
const nativeReadinessStructureSQL = `SELECT jsonb_build_object(
 'columns',(SELECT jsonb_agg(jsonb_build_array(n.nspname,c.relname,a.attnum,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attidentity,a.attgenerated,pg_get_expr(d.adbin,d.adrelid)) ORDER BY n.nspname,c.relname,a.attnum)
 FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
 WHERE a.attnum>0 AND NOT a.attisdropped AND c.relkind='r' AND (n.nspname='native_membership_guard' OR (n.nspname='public' AND c.relname='space_join_operations'))),
 'constraints',(SELECT jsonb_agg(jsonb_build_array(n.nspname,c.relname,k.conname,k.contype,k.condeferrable,k.condeferred,k.convalidated,pg_get_constraintdef(k.oid)) ORDER BY n.nspname,c.relname,k.conname)
 FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='native_membership_guard' OR (n.nspname='public' AND c.relname='space_join_operations')),
 'indexes',(SELECT jsonb_agg(jsonb_build_array(c.relname,i.indisunique,i.indisvalid,i.indisready,pg_get_indexdef(i.indexrelid)) ORDER BY c.relname,pg_get_indexdef(i.indexrelid))
 FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='native_membership_guard')
 )::text`

// PostgreSQL16 canonical definitions from the pinned1..44 migration cohort,
// including43 START columns/constraints and44's four protected tables. This
// expected value is compiled, never copied from the live epoch being checked.
const nativeReadinessStructureSHA = "6b43c9f367207020eee053cd3ab6eb4e057238df4c56f83e132bf0885742206d"

// This authenticates the installed guard before any public SECURITY DEFINER
// wrapper may trust its private epoch seal. The seal subsequently detects any
// table, column, constraint, trigger, function or private ACL change. It is not
// a caller-selected hash or permission to reseal an arbitrary current catalog.
func checkNativeReadinessCatalogVersion(ctx context.Context, tx pgx.Tx, version int64) error {
	observer50 := version == 50
	if observer50 {
		if err := checkAccountBanCatalog(ctx, tx); err != nil {
			return err
		}
		// Structure and every unchanged body retain the fixed45 qualification.
		// source_after and catalog have exact50 body/full-definition pins below.
		version = 45
	}
	var serverVersion int
	if err := tx.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer`).Scan(&serverVersion); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	structureSHA, err := nativeReadinessExpectedStructure(version, serverVersion)
	if err != nil {
		return err
	}
	pins := append([]nativeReadinessFunctionPin(nil), nativeReadinessFunctionPins...)
	helperCount := 1
	if version == 45 {
		pins = append(pins, nativeReadinessLegacySelectionPin)
		helperCount = 2
	} else if version != 44 {
		return ErrMemberAuthorityUnavailable
	}
	if observer50 {
		if _, _, err := requireNativeObserver50Definition(ctx, tx); err != nil {
			return err
		}
		if _, _, err := requireNativeCatalog50Definition(ctx, tx); err != nil {
			return err
		}
		for i := range pins {
			if pins[i].signature == "native_membership_guard.catalog()" {
				pins[i].body = nativeCatalog50BodySHA
			}
			if pins[i].signature == "native_membership_guard.source_after()" {
				pins[i].body = nativeObserver50BodySHA
			}
		}
	}
	var structure string
	if err := tx.QueryRow(ctx, nativeReadinessStructureSQL).Scan(&structure); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	structureDigest := sha256.Sum256([]byte(structure))
	if hex.EncodeToString(structureDigest[:]) != structureSHA {
		return ErrMemberAuthorityUnavailable
	}
	for _, pin := range pins {
		qualified, arguments, _ := strings.Cut(strings.TrimSuffix(pin.signature, ")"), "(")
		schema, name, _ := strings.Cut(qualified, ".")
		types := []string{}
		if arguments != "" {
			types = strings.Split(arguments, ",")
		}
		var body string
		var valid bool
		err := tx.QueryRow(ctx, `SELECT p.prosrc,
 l.lanname=$2 AND p.prorettype=to_regtype($3) AND p.provolatile::text=$4
 AND p.prosecdef=$5 AND p.proretset=$6 AND NOT p.proleakproof
 AND p.prokind='f' AND NOT p.proisstrict AND p.proparallel='u' AND p.provariadic=0 AND p.prosupport=0
 AND p.pronargdefaults=CASE WHEN p.proname='verify_completion' THEN 1 ELSE 0 END
 AND pg_get_expr(p.proargdefaults,0) IS NOT DISTINCT FROM CASE WHEN p.proname='verify_completion' THEN 'true' ELSE NULL END
 AND COALESCE(p.proconfig,ARRAY[]::text[])=CASE WHEN $7='' THEN ARRAY[]::text[] ELSE ARRAY[$7]::text[] END
 AND p.proowner=(SELECT nspowner FROM pg_namespace WHERE nspname='native_membership_guard')
 AND p.proowner=(SELECT relowner FROM pg_class WHERE oid='public.spaces'::regclass)
 AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a
  WHERE a.grantee<>p.proowner AND (a.grantee<>0 OR a.privilege_type<>'EXECUTE'
   OR n.nspname='native_membership_guard' OR p.proname='space_native_membership_initialize'))
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang
 WHERE n.nspname=$1 AND p.proname=$8 AND oidvectortypes(p.proargtypes)=COALESCE(
 (SELECT string_agg(format_type(to_regtype(arg),NULL),', ' ORDER BY ord) FROM unnest($9::text[]) WITH ORDINALITY a(arg,ord)), '')`, schema, pin.language, pin.result, pin.volatility, pin.definer, pin.returnsSet, pin.config, name, types).Scan(&body, &valid)
		if err != nil || !valid {
			return fmt.Errorf("%w: guard function %s metadata: %v", ErrMemberAuthorityUnavailable, pin.signature, err)
		}
		digest := sha256.Sum256([]byte(strings.ReplaceAll(body, "\r\n", "\n")))
		if hex.EncodeToString(digest[:]) != pin.body {
			return fmt.Errorf("%w: guard function %s body", ErrMemberAuthorityUnavailable, pin.signature)
		}
	}
	var valid bool
	// No shadow overload/helper or application privilege can extend the closed
	// private API or make a private receipt/epoch directly forgeable.
	if err := tx.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname='native_membership_guard' OR (n.nspname='public' AND p.proname LIKE 'space_native_membership_%'))=$1
 AND (SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='native_membership_guard' AND c.relkind='r')=4
 AND NOT EXISTS(SELECT 1 FROM pg_namespace n,LATERAL aclexplode(COALESCE(n.nspacl,acldefault('n',n.nspowner))) a
  WHERE n.nspname='native_membership_guard' AND a.grantee<>n.nspowner)
 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='native_membership_guard' AND c.relkind='r' AND
   (c.relname NOT IN ('epoch','spaces','frames','profiles') OR c.relpersistence<>'p' OR c.relrowsecurity OR c.relforcerowsecurity
    OR c.relowner<>n.nspowner OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=c.oid)
    OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=c.oid OR inhparent=c.oid)
    OR EXISTS(SELECT 1 FROM aclexplode(COALESCE(c.relacl,acldefault('r',c.relowner))) a WHERE a.grantee<>c.relowner)))`, len(pins)-helperCount).Scan(&valid); err != nil || !valid {
		return ErrMemberAuthorityUnavailable
	}
	for _, table := range nativeMembershipGuardCoverage {
		argument := "space_id"
		if table == "spaces" {
			argument = "id"
		}
		for _, trigger := range []struct {
			name, function string
			kind           int
			args           []byte
		}{
			{"a000_native_membership_capture", "source_before", 31, []byte(argument + "\x00")},
			{"a000_native_membership_no_truncate", "source_before", 34, []byte{}},
			{"zzzzzz_native_membership_observe", "source_after", 29, []byte(argument + "\x00")},
		} {
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE t.tgrelid=to_regclass($1) AND t.tgname=$2 AND t.tgtype=$3 AND t.tgargs=$4
 AND t.tgenabled='A' AND NOT t.tgisinternal AND NOT t.tgdeferrable AND NOT t.tginitdeferred
 AND n.nspname='native_membership_guard' AND p.proname=$5 AND p.pronargs=0 AND p.prorettype='trigger'::regtype)`, "public."+table, trigger.name, trigger.kind, trigger.args, trigger.function).Scan(&valid); err != nil || !valid {
				return ErrMemberAuthorityUnavailable
			}
		}
	}
	if err := tx.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
  JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
  WHERE n.nspname='native_membership_guard' AND c.relname='frames'
  AND t.tgname='membership_frame_completed' AND t.tgenabled='O' AND t.tgdeferrable AND t.tginitdeferred
  AND t.tgtype=21 AND pn.nspname='native_membership_guard' AND p.proname='commit_check' AND p.pronargs=0)
 AND EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace n ON n.oid=p.pronamespace WHERE t.tgrelid='public.spaces'::regclass
  AND t.tgname='zzzzz_native_membership_guard_terminal' AND t.tgenabled='O' AND NOT t.tgdeferrable
  AND t.tgtype=9 AND n.nspname='native_membership_guard' AND p.proname='terminal_cleanup' AND p.pronargs=0)
 AND EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid='public.space_join_operations'::regclass
  AND t.tgname='space_join_start_selection_immutable' AND t.tgenabled='O' AND NOT t.tgdeferrable
  AND t.tgtype=19 AND t.tgfoid='public.guard_join_start_selection_immutable()'::regprocedure)`).Scan(&valid); err != nil || !valid {
		return ErrMemberAuthorityUnavailable
	}
	if version == 45 {
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid='public.space_join_operations'::regclass
 AND t.tgname='space_join_legacy_selection_immutable' AND t.tgenabled='O' AND NOT t.tgdeferrable AND NOT t.tginitdeferred
 AND NOT t.tgisinternal AND t.tgtype=19 AND t.tgnargs=0 AND t.tgargs=''::bytea
 AND t.tgfoid='public.guard_join_legacy_selection_immutable()'::regprocedure)`).Scan(&valid); err != nil || !valid {
			return ErrMemberAuthorityUnavailable
		}
	}
	return nil
}
