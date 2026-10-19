package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/authoritysource"
)

// This native contract validates its consumed facts and owner functions rather
// than borrowing the Federation source's exact migration25 activation gate.
// The current cohort is official34, private journals35/36, queue37 and forward
// native39 and explicit versioned decision40. Native39 preserves the unpublished33 guard/function bodies while
// current34 retains the mandatory eleven-participant terminal predicate.
// Earlier, dirty and future schemas are unavailable; journals grant no authority.
func checkMemberAuthorityCatalog(ctx context.Context, tx pgx.Tx) error {
	// Lock the migration marker before selecting the finite qualified cohort.
	// A table from a newer migration is never evidence that its schema is ready.
	if _, err := tx.Exec(ctx, `LOCK TABLE public.schema_migrations IN ACCESS SHARE MODE`); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	var version int64
	var dirty bool
	if err := tx.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty); err != nil || dirty || (version != 39 && version != 40 && version != 41 && version != 42 && version != 44 && version != 45) {
		return ErrMemberAuthorityUnavailable
	}
	tables := append([]string{}, spaceAuthorityCatalog.Tables...)
	tables = append(tables, "schema_migrations", "space_entry_policies", "space_ordinary_member_heads", "space_community_member_heads", "space_member_admission_intents", "space_native_member_publications", "space_membership_ownership_heads", "space_lifecycle_operations", "space_lifecycle_participants", "space_lifecycle_completions", "space_lifecycle_outbox", "space_permanent_purge_fences", "space_member_admission_outbox", "space_member_role_cleanup")
	tables = append(tables, "community_bootstrap_operations")
	tables = append(tables, "space_creator_principal_replays")
	tables = append(tables, "space_native_creator_operations")
	tables = append(tables, "space_join_attempts", "space_join_approval_operations", "space_join_operations",
		"space_join_request_groups", "space_join_request_attempts", "space_join_request_selections")
	if version == 41 || version == 42 || version == 44 || version == 45 {
		tables = append(tables, "space_effective_member_heads")
	}
	if version == 42 || version == 44 || version == 45 {
		tables = append(tables, "space_bot_installation_heads", "space_bot_installation_attempts", "space_bot_installation_cancellations")
	}
	names := make([]string, 0, len(tables))
	for _, table := range tables {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	// Explicit pg_temp LAST prevents PostgreSQL's implicit temporary-schema
	// precedence from replacing qualified native source/floor reads afterward.
	if _, err := tx.Exec(ctx, `SET LOCAL row_security=off; SET LOCAL search_path=pg_catalog,public,pg_temp`); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	if _, err := tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS SHARE MODE"); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(version=$1 AND NOT dirty) FROM public.schema_migrations`, version).Scan(&valid); err != nil || !valid {
		return ErrMemberAuthorityUnavailable
	}
	if err := tx.QueryRow(ctx, `SELECT count(*)=$2 AND bool_and(c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
	 AND NOT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=c.oid)
	 AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
	 AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=c.oid OR inhparent=c.oid))
	 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($1)`, tables, len(tables)).Scan(&valid); err != nil || !valid {
		return ErrMemberAuthorityUnavailable
	}
	triggers := append([]authoritysource.CatalogTrigger{}, spaceAuthorityCatalog.Triggers...)
	for _, table := range []string{"space_ordinary_member_heads", "space_community_member_heads", "space_entry_policies", "space_member_admission_intents", "space_membership_ownership_heads", "space_native_member_publications", "space_lifecycle_operations", "space_lifecycle_participants", "space_lifecycle_completions", "space_lifecycle_outbox", "space_permanent_purge_fences", "community_bootstrap_operations", "space_native_creator_operations"} {
		triggers = append(triggers, authoritysource.CatalogTrigger{Table: table, Name: "space_authority_revision_" + table, Function: "space_authority_revision_changed", Type: 29, Argument: "space_id"}, authoritysource.CatalogTrigger{Table: table, Name: "space_authority_revision_" + table + "_truncate", Function: "space_authority_revision_floor_guard", Type: 34})
	}
	triggers = append(triggers,
		authoritysource.CatalogTrigger{Table: "space_member_admission_intents", Name: "member_admission_intent_guard", Function: "space_member_admission_intent_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "space_member_admission_intents", Name: "member_admission_intent_truncate", Function: "space_member_admission_intent_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "space_native_creator_operations", Name: "native_creator_operation_guard", Function: "space_native_creator_operation_guard", Type: 31},
		authoritysource.CatalogTrigger{Table: "space_native_creator_operations", Name: "native_creator_operation_truncate", Function: "space_native_creator_operation_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "space_creator_principal_replays", Name: "creator_principal_replay_guard", Function: "space_creator_principal_replay_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "space_creator_principal_replays", Name: "creator_principal_replay_truncate", Function: "space_creator_principal_replay_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "space_member_role_cleanup", Name: "member_role_cleanup_guard", Function: "space_member_role_cleanup_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "space_member_role_cleanup", Name: "member_role_cleanup_truncate", Function: "space_member_role_cleanup_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "space_ordinary_member_heads", Name: "ordinary_member_head_guard", Function: "space_member_head_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "space_ordinary_member_heads", Name: "ordinary_member_head_truncate", Function: "space_member_head_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "space_community_member_heads", Name: "community_member_head_guard", Function: "space_member_head_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "space_community_member_heads", Name: "community_member_head_truncate", Function: "space_member_head_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "space_members", Name: "space_ordinary_publication_guard", Function: "space_ordinary_publication_guard", Type: 31},
		authoritysource.CatalogTrigger{Table: "community_roster_members", Name: "space_community_episode_changed", Function: "space_community_episode_changed", Type: 29},
		authoritysource.CatalogTrigger{Table: "community_owner_authority", Name: "space_community_owner_episode_changed", Function: "space_community_owner_episode_changed", Type: 25},
		authoritysource.CatalogTrigger{Table: "spaces", Name: "space_membership_ownership_changed", Function: "space_membership_ownership_changed", Type: 29},
		authoritysource.CatalogTrigger{Table: "spaces", Name: "space_membership_terminal_cleanup", Function: "space_membership_terminal_cleanup", Type: 9},
		authoritysource.CatalogTrigger{Table: "space_membership_ownership_heads", Name: "membership_ownership_head_guard", Function: "space_membership_ownership_head_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "space_membership_ownership_heads", Name: "membership_ownership_head_truncate", Function: "space_membership_ownership_head_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "space_member_admission_outbox", Name: "member_admission_event_guard", Function: "space_member_admission_event_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "space_member_admission_outbox", Name: "member_admission_event_truncate", Function: "space_member_admission_event_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "space_native_member_publications", Name: "native_member_publication_guard", Function: "space_native_member_publication_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "space_native_member_publications", Name: "native_member_publication_truncate", Function: "space_native_member_publication_guard", Type: 34},
	)
	if version == 41 || version == 42 || version == 44 || version == 45 {
		triggers = append(triggers,
			authoritysource.CatalogTrigger{Table: "space_effective_member_heads", Name: "effective_member_head_guard", Function: "space_effective_member_head_guard", Type: 27},
			authoritysource.CatalogTrigger{Table: "space_effective_member_heads", Name: "effective_member_head_truncate", Function: "space_effective_member_head_guard", Type: 34},
			authoritysource.CatalogTrigger{Table: "space_effective_member_heads", Name: "effective_member_liveness_guard", Function: "space_effective_member_liveness_guard", Type: 23},
			authoritysource.CatalogTrigger{Table: "spaces", Name: "zz_space_effective_membership_terminal_cleanup", Function: "space_effective_membership_terminal_cleanup", Type: 9})
	}

	if version == 42 || version == 44 || version == 45 {
		for _, table := range []string{"space_bot_installation_heads", "space_bot_installation_attempts", "space_bot_installation_cancellations"} {
			triggers = append(triggers, authoritysource.CatalogTrigger{Table: table, Name: table + "_liveness", Function: "space_bot_installation_liveness_guard", Type: 23}, authoritysource.CatalogTrigger{Table: table, Name: "space_authority_revision_" + table, Function: "space_authority_revision_changed", Type: 29, Argument: "space_id"}, authoritysource.CatalogTrigger{Table: table, Name: "space_authority_revision_" + table + "_truncate", Function: "space_authority_revision_floor_guard", Type: 34})
		}
		triggers = append(triggers,
			authoritysource.CatalogTrigger{Table: "space_bot_installation_heads", Name: "bot_installation_head_guard", Function: "space_bot_installation_head_guard", Type: 31},
			authoritysource.CatalogTrigger{Table: "space_bot_installation_heads", Name: "bot_installation_head_truncate", Function: "space_bot_installation_head_guard", Type: 34},
			authoritysource.CatalogTrigger{Table: "space_bot_installation_attempts", Name: "bot_installation_attempt_guard", Function: "space_bot_installation_attempt_guard", Type: 27},
			authoritysource.CatalogTrigger{Table: "space_bot_installation_attempts", Name: "bot_installation_attempt_truncate", Function: "space_bot_installation_attempt_guard", Type: 34},
			authoritysource.CatalogTrigger{Table: "space_bot_installation_cancellations", Name: "bot_installation_cancellation_guard", Function: "space_bot_installation_cancellation_guard", Type: 27},
			authoritysource.CatalogTrigger{Table: "space_bot_installation_cancellations", Name: "bot_installation_cancellation_truncate", Function: "space_bot_installation_cancellation_guard", Type: 34},
			authoritysource.CatalogTrigger{Table: "spaces", Name: "zzzz_space_bot_installation_terminal_cleanup", Function: "space_bot_installation_terminal_cleanup", Type: 9})
	}
	for _, trigger := range triggers {
		arguments := []byte{}
		if trigger.Argument != "" {
			arguments = []byte(trigger.Argument + "\x00")
		}
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace n ON n.oid=p.pronamespace
	 WHERE t.tgrelid=to_regclass($1) AND t.tgname=$2 AND NOT t.tgisinternal AND t.tgenabled='O' AND NOT t.tgdeferrable AND NOT t.tginitdeferred AND t.tgtype=$3
	 AND n.nspname='public' AND p.proname=$4 AND p.pronargs=0 AND p.prorettype='trigger'::regtype AND t.tgargs=$5)`, "public."+trigger.Table, trigger.Name, trigger.Type, trigger.Function, arguments).Scan(&valid); err != nil || !valid {
			return ErrMemberAuthorityUnavailable
		}
	}
	functions := append([]authoritysource.CatalogFunction{}, spaceAuthorityCatalog.Functions...)
	functions = append(functions, memberAuthorityFunctions...)
	if version == 41 || version == 42 || version == 44 || version == 45 {
		functions = append(functions, effectiveMemberAuthorityFunctions...)
	}
	if version == 42 || version == 44 || version == 45 {
		functions = append(functions, botInstallationAuthorityFunctions...)
	}
	for _, function := range functions {
		var body, volatility string
		language := "plpgsql"
		signature := "public." + function.Name + "()"
		if function.Name == "space_voice_access_bump" {
			signature = "public.space_voice_access_bump(uuid,uuid,uuid)"
		} else if function.Name == "space_membership_terminal_cleanup_allowed" {
			signature = "public.space_membership_terminal_cleanup_allowed(uuid)"
		} else if function.Name == "space_terminal_purge_ready" {
			signature = "public.space_terminal_purge_ready(uuid)"
			language = "sql"
		}
		if err := tx.QueryRow(ctx, `SELECT p.prosrc,p.provolatile::text FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang
	 WHERE n.nspname='public' AND p.proname=$1 AND p.pronargs=$2 AND NOT p.prosecdef AND p.proconfig IS NULL AND l.lanname=$5 AND p.prorettype=to_regtype($3) AND p.oid=to_regprocedure($4)`, function.Name, function.Arguments, function.ReturnType, signature, language).Scan(&body, &volatility); err != nil {
			return ErrMemberAuthorityUnavailable
		}
		expectedVolatility := "v"
		if function.Name == "space_membership_terminal_cleanup_allowed" || function.Name == "space_terminal_purge_ready" {
			expectedVolatility = "s"
		}
		digest := sha256.Sum256([]byte(body))
		if volatility != expectedVolatility || hex.EncodeToString(digest[:]) != function.BodySHA256 {
			return ErrMemberAuthorityUnavailable
		}
	}
	if version == 44 || version == 45 {
		return checkNativeReadinessCatalogVersion(ctx, tx, version)
	}
	return nil
}
