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
// Compatible additive migrations (including participant-set34) remain usable.
func checkMemberAuthorityCatalog(ctx context.Context, tx pgx.Tx) error {
	tables := append([]string{}, spaceAuthorityCatalog.Tables...)
	tables = append(tables, "schema_migrations", "space_entry_policies", "space_ordinary_member_heads", "space_community_member_heads", "space_member_admission_intents", "space_native_member_publications", "space_membership_ownership_heads", "space_lifecycle_operations", "space_lifecycle_participants", "space_lifecycle_completions", "space_lifecycle_outbox", "space_permanent_purge_fences", "space_member_admission_outbox", "space_member_role_cleanup")
	names := make([]string, 0, len(tables))
	for _, table := range tables {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	if _, err := tx.Exec(ctx, `SET LOCAL row_security=off; SET LOCAL search_path=public,pg_catalog`); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	if _, err := tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS SHARE MODE"); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(version>=33 AND NOT dirty) FROM public.schema_migrations`).Scan(&valid); err != nil || !valid {
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
	for _, table := range []string{"space_ordinary_member_heads", "space_community_member_heads", "space_entry_policies", "space_member_admission_intents", "space_membership_ownership_heads", "space_native_member_publications", "space_lifecycle_operations", "space_lifecycle_participants", "space_lifecycle_completions", "space_lifecycle_outbox", "space_permanent_purge_fences"} {
		triggers = append(triggers, authoritysource.CatalogTrigger{Table: table, Name: "space_authority_revision_" + table, Function: "space_authority_revision_changed", Type: 29, Argument: "space_id"}, authoritysource.CatalogTrigger{Table: table, Name: "space_authority_revision_" + table + "_truncate", Function: "space_authority_revision_floor_guard", Type: 34})
	}
	triggers = append(triggers,
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
	)
	for _, trigger := range triggers {
		arguments := []byte{}
		if trigger.Argument != "" {
			arguments = []byte(trigger.Argument + "\x00")
		}
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace n ON n.oid=p.pronamespace
	 WHERE t.tgrelid=to_regclass($1) AND t.tgname=$2 AND NOT t.tgisinternal AND t.tgenabled='O' AND t.tgtype=$3
	 AND n.nspname='public' AND p.proname=$4 AND p.pronargs=0 AND p.prorettype='trigger'::regtype AND t.tgargs=$5)`, "public."+trigger.Table, trigger.Name, trigger.Type, trigger.Function, arguments).Scan(&valid); err != nil || !valid {
			return ErrMemberAuthorityUnavailable
		}
	}
	functions := append([]authoritysource.CatalogFunction{}, spaceAuthorityCatalog.Functions...)
	functions = append(functions, memberAuthorityFunctions...)
	for _, function := range functions {
		var body, volatility string
		signature := "public." + function.Name + "()"
		if function.Name == "space_voice_access_bump" {
			signature = "public.space_voice_access_bump(uuid,uuid,uuid)"
		} else if function.Name == "space_membership_terminal_cleanup_allowed" {
			signature = "public.space_membership_terminal_cleanup_allowed(uuid)"
		}
		if err := tx.QueryRow(ctx, `SELECT p.prosrc,p.provolatile::text FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang
	 WHERE n.nspname='public' AND p.proname=$1 AND p.pronargs=$2 AND NOT p.prosecdef AND l.lanname='plpgsql' AND p.prorettype=to_regtype($3) AND p.oid=to_regprocedure($4)`, function.Name, function.Arguments, function.ReturnType, signature).Scan(&body, &volatility); err != nil {
			return ErrMemberAuthorityUnavailable
		}
		expectedVolatility := "v"
		if function.Name == "space_membership_terminal_cleanup_allowed" {
			expectedVolatility = "s"
		}
		digest := sha256.Sum256([]byte(body))
		if volatility != expectedVolatility || hex.EncodeToString(digest[:]) != function.BodySHA256 {
			return ErrMemberAuthorityUnavailable
		}
	}
	return nil
}
