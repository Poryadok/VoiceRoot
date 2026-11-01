package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/user/internal/userauthoritycatalog"
)

var _ authoritysource.SchemaReader = (*ProfileStore)(nil)

func (s *ProfileStore) CheckAuthoritySourceSchema(ctx context.Context) error {
	if s == nil {
		return errors.New("user source pool unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginUserAuthorityRead(ctx)
	if err != nil {
		return err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	return tx.Commit(ctx)
}
func (s *ProfileStore) beginAuthorityRead(ctx context.Context, scope *authorityv1.SourceScope) (pgx.Tx, error) {
	if err := authoritysource.ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER, scope); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errors.New("user source pool unavailable")
	}
	return s.beginUserAuthorityRead(ctx)
}

// The version query is only a selector. BeginRead checks the selected clean
// version again while holding the source catalog locks for the entire cut.
func (s *ProfileStore) beginUserAuthorityRead(ctx context.Context) (pgx.Tx, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("user source pool unavailable")
	}
	var version int64
	if err := s.pool.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil {
		return nil, errors.New("user source schema unavailable")
	}
	catalog := userAuthorityCatalog
	switch version {
	case 21:
	case 22:
		catalog = userauthoritycatalog.Catalog()
	default:
		return nil, errors.New("user source schema version unavailable")
	}
	tx, err := authoritysource.BeginRead(ctx, s.pool, catalog)
	if err != nil {
		return nil, err
	}
	if version == 21 {
		// A relabelled or partially installed identity migration must not gain
		// authority through the older catalog. This is not an anti-rollback proof.
		var present bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND (c.relname LIKE 'identity\_%' ESCAPE '\' OR c.relname LIKE 'association\_%' ESCAPE '\')
 UNION ALL SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE n.nspname='public' AND (p.proname LIKE 'identity\_%' ESCAPE '\' OR p.proname='reject_user_profile_identity_rebind')
 UNION ALL SELECT 1 FROM pg_attribute a WHERE a.attrelid='public.profiles'::regclass AND NOT a.attisdropped
 AND a.attname IN ('identity_association_id','identity_baseline_cut')
 UNION ALL SELECT 1 FROM pg_trigger t WHERE NOT t.tgisinternal AND t.tgrelid IN ('public.profiles'::regclass,'public.user_account_lifecycle'::regclass,'public.sdk_author_tombstones'::regclass,'public.user_authority_revision'::regclass)
 AND (t.tgname LIKE 'identity\_%' ESCAPE '\' OR t.tgname='user_profile_identity_immutable')
)`).Scan(&present)
		if err != nil || present {
			authoritysource.RollbackRead(ctx, tx)
			return nil, errors.New("user source identity schema unavailable")
		}
	}
	return tx, nil
}
func userSourceRevision(ctx context.Context, tx pgx.Tx) (uint64, error) {
	var revision uint64
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.user_authority_revision WHERE singleton`).Scan(&revision); err != nil || revision == 0 {
		return 0, errors.New("complete User revision unavailable")
	}
	return revision, nil
}
func (s *ProfileStore) ReadAuthorityRevision(ctx context.Context, scope *authorityv1.SourceScope) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginAuthorityRead(ctx, scope)
	if err != nil {
		return 0, err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	revision, err := userSourceRevision(ctx, tx)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return revision, nil
}
func (s *ProfileStore) ReadAuthoritySnapshot(ctx context.Context, scope *authorityv1.SourceScope) (authoritysource.State, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginAuthorityRead(ctx, scope)
	if err != nil {
		return authoritysource.State{}, err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	revision, err := userSourceRevision(ctx, tx)
	if err != nil {
		return authoritysource.State{}, err
	}
	state := authoritysource.UserState{SchemaVersion: 1, Revision: revision, Profiles: []authoritysource.UserProfile{}, Tombstones: []authoritysource.UserAuthorTombstone{}}
	// LEFT JOIN keeps an explicit missing fact for each ID. No display, privacy,
	// receipt, request hash or proof material leaves the owning User database.
	rows, err := tx.Query(ctx, `SELECT requested.id::text,p.id IS NOT NULL,COALESCE(p.account_id::text,''),COALESCE(p.sdk_eligibility_revision,0),COALESCE(p.deleted_at IS NOT NULL,false),COALESCE(p.frozen_at IS NOT NULL,false),EXISTS(SELECT 1 FROM public.user_account_lifecycle a WHERE a.account_id=p.account_id AND a.state='ACCOUNT_INACTIVE')
 FROM unnest($1::uuid[]) requested(id) LEFT JOIN public.profiles p ON p.id=requested.id ORDER BY requested.id LIMIT 10001`, scope.ProfileIds)
	if err != nil {
		return authoritysource.State{}, err
	}
	for rows.Next() {
		var item authoritysource.UserProfile
		if err = rows.Scan(&item.ProfileID, &item.Exists, &item.AccountID, &item.ProfileRevision, &item.Deleted, &item.Frozen, &item.AccountInactive); err != nil {
			rows.Close()
			return authoritysource.State{}, err
		}
		state.Profiles = append(state.Profiles, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(state.Profiles) != len(scope.ProfileIds) {
		return authoritysource.State{}, errors.New("complete User profile set unavailable")
	}
	rows, err = tx.Query(ctx, `SELECT source_account_id::text,source_actor_id::text,target_account_id::text,target_profile_id::text,profile_revision,tombstone_revision FROM public.sdk_author_tombstones WHERE source_actor_id=ANY($1::uuid[]) ORDER BY source_actor_id,source_account_id LIMIT 10001`, scope.ProfileIds)
	if err != nil {
		return authoritysource.State{}, err
	}
	for rows.Next() {
		if len(state.Tombstones) >= authoritysource.MaxSubjects {
			rows.Close()
			return authoritysource.State{}, errors.New("complete User tombstones exceed bound")
		}
		var item authoritysource.UserAuthorTombstone
		if err = rows.Scan(&item.SourceAccountID, &item.SourceActorID, &item.TargetAccountID, &item.TargetProfileID, &item.ProfileRevision, &item.TombstoneRevision); err != nil {
			rows.Close()
			return authoritysource.State{}, err
		}
		state.Tombstones = append(state.Tombstones, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return authoritysource.State{}, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return authoritysource.State{}, err
	}
	if _, err = authoritysource.DecodeUserState(raw); err != nil {
		return authoritysource.State{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return authoritysource.State{}, err
	}
	return authoritysource.State{Complete: true, Revision: revision, CanonicalState: raw}, nil
}
