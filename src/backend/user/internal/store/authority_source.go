package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

var _ authoritysource.SchemaReader = (*ProfileStore)(nil)

func (s *ProfileStore) CheckAuthoritySourceSchema(ctx context.Context) error {
	if s == nil {
		return errors.New("user source pool unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := authoritysource.BeginRead(ctx, s.pool, userAuthorityCatalog)
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
	return authoritysource.BeginRead(ctx, s.pool, userAuthorityCatalog)
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
