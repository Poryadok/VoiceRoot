package nativeinstaller

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// This qualifies the source-side administrative owner and public CREATE
// boundary. The controller separately owns backup, writer cut and Role rights.
func qualifyLegacy24InstallerTx(ctx context.Context, tx pgx.Tx) error {
	var ok bool
	query := `SELECT current_user=session_user
	 AND EXISTS(SELECT 1 FROM pg_database d WHERE d.datname=current_database() AND pg_has_role(session_user,d.datdba,'USAGE'))
	 AND EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname='public' AND pg_has_role(session_user,n.nspowner,'USAGE') AND NOT EXISTS(SELECT 1 FROM aclexplode(coalesce(n.nspacl,acldefault('n',n.nspowner))) a WHERE a.grantee=0 AND a.privilege_type='CREATE'))
	 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND NOT pg_has_role(session_user,c.relowner,'USAGE'))
	 AND NOT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND NOT pg_has_role(session_user,p.proowner,'USAGE'))
	 AND NOT EXISTS(SELECT 1 FROM pg_extension e WHERE NOT pg_has_role(session_user,e.extowner,'USAGE'))`
	if err := tx.QueryRow(ctx, query).Scan(&ok); err != nil || !ok {
		return errors.New("HOLD: legacy24 installer ownership or public CREATE boundary")
	}
	return nil
}

// Reject source states known to make the existing native initializer refuse a
// present Space. This is a bounded preflight, not a substitute for the actual
// initializer or the controller's separate Role authority qualification.
func qualifyLegacy24LifecycleTx(ctx context.Context, tx pgx.Tx) error {
	var ok bool
	query := `SELECT NOT EXISTS(SELECT 1 FROM public.spaces s LEFT JOIN public.space_lifecycle_aggregates a ON a.space_id=s.id WHERE
	 (a.space_id IS NOT NULL AND (a.generation<=0 OR a.phase NOT IN ('LIVE','SCHEDULE_PENDING','FREEZE_PENDING','SCHEDULED','RESTORE_DECIDED','PURGE_DECIDED','PURGING'))) OR
	 (a.space_id IS NULL AND (EXISTS(SELECT 1 FROM public.space_lifecycle_operations x WHERE x.space_id=s.id) OR EXISTS(SELECT 1 FROM public.space_lifecycle_participants x WHERE x.space_id=s.id) OR EXISTS(SELECT 1 FROM public.space_lifecycle_completions x WHERE x.space_id=s.id) OR EXISTS(SELECT 1 FROM public.space_lifecycle_outbox x WHERE x.space_id=s.id) OR EXISTS(SELECT 1 FROM public.space_deletion_tombstones x WHERE x.space_id=s.id))) OR
	 EXISTS(SELECT 1 FROM public.community_owner_authority x WHERE x.space_id=s.id AND x.status='active' AND x.owner_profile_id IS DISTINCT FROM s.owner_profile_id))`
	if err := tx.QueryRow(ctx, query).Scan(&ok); err != nil || !ok {
		return errors.New("HOLD: legacy24 existing Space lifecycle or ownership unavailable")
	}
	return nil
}

// Source qualification is not maintenance, backup, or product-rights authority.
func admitLegacy24Source(ctx context.Context, conn *pgx.Conn) (*legacy24OriginObservation, error) {
	return observeLegacy24Origin(ctx, conn)
}
