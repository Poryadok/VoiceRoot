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

var _ authoritysource.Reader = (*SpaceStore)(nil)

func (s *SpaceStore) CheckAuthoritySourceSchema(ctx context.Context) error {
	if s == nil || s.tx != nil {
		return errors.New("source preflight requires owning pool")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := authoritysource.BeginRead(ctx, s.Pool, spaceAuthorityCatalog)
	if err != nil {
		return err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	return tx.Commit(ctx)
}

func (s *SpaceStore) beginAuthorityRead(ctx context.Context, scope *authorityv1.SourceScope) (pgx.Tx, error) {
	if err := authoritysource.ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, scope); err != nil {
		return nil, err
	}
	if s == nil || s.tx != nil {
		return nil, errors.New("source read requires owning pool")
	}
	return authoritysource.BeginRead(ctx, s.Pool, spaceAuthorityCatalog)
}

func spaceSourceRevision(ctx context.Context, tx pgx.Tx, space string) (uint64, error) {
	var revision uint64
	if err := tx.QueryRow(ctx, `SELECT access_epoch FROM public.space_voice_access_epochs WHERE space_id=$1`, space).Scan(&revision); err != nil || revision == 0 {
		return 0, errors.New("complete Space revision unavailable")
	}
	var conflict bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_voice_access_outbox WHERE space_id=$1 AND access_epoch>$2)`, space, revision).Scan(&conflict); err != nil || conflict {
		return 0, errors.New("Space revision conflicts with durable history")
	}
	return revision, nil
}

func (s *SpaceStore) ReadAuthorityRevision(ctx context.Context, scope *authorityv1.SourceScope) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginAuthorityRead(ctx, scope)
	if err != nil {
		return 0, err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	revision, err := spaceSourceRevision(ctx, tx, scope.SpaceId)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return revision, nil
}

func readSpaceSourceRows(ctx context.Context, tx pgx.Tx, query string, space string, scan func(pgx.Rows) error) error {
	rows, err := tx.Query(ctx, query, space)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > authoritysource.MaxSubjects {
			return errors.New("complete Space source exceeds bound")
		}
		if err = scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *SpaceStore) ReadAuthoritySnapshot(ctx context.Context, scope *authorityv1.SourceScope) (authoritysource.State, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginAuthorityRead(ctx, scope)
	if err != nil {
		return authoritysource.State{}, err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	revision, err := spaceSourceRevision(ctx, tx, scope.SpaceId)
	if err != nil {
		return authoritysource.State{}, err
	}
	state := authoritysource.SpaceState{SchemaVersion: 1, SpaceID: scope.SpaceId, Revision: revision,
		Members: []string{}, BannedAccounts: []string{}, Timeouts: []authoritysource.SpaceTimeout{}, VoiceRooms: []string{}, Categories: []string{}, Tree: []authoritysource.SpaceTreeResource{}, CommunityMembers: []authoritysource.CommunityMember{}}
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1),
 COALESCE((SELECT owner_profile_id::text FROM public.spaces WHERE id=$1),''),
 COALESCE((SELECT visibility FROM public.spaces WHERE id=$1),''),
 COALESCE((SELECT allow_guests FROM public.spaces WHERE id=$1),false),
 EXISTS(SELECT 1 FROM public.ownership_journal WHERE space_id=$1 AND state NOT IN ('completed','aborted')),
 COALESCE((SELECT phase FROM public.space_lifecycle_aggregates WHERE space_id=$1),''),
 COALESCE((SELECT generation FROM public.space_lifecycle_aggregates WHERE space_id=$1),0),
 EXISTS(SELECT 1 FROM public.space_deletion_tombstones WHERE space_id=$1)`, scope.SpaceId).Scan(&state.Exists, &state.OwnerProfileID, &state.Visibility, &state.AllowGuests, &state.OwnershipFrozen, &state.DeletionPhase, &state.DeletionGeneration, &state.Purged)
	if err != nil {
		return authoritysource.State{}, err
	}
	for _, set := range []struct {
		query  string
		target *[]string
	}{
		{`SELECT profile_id::text FROM public.space_members WHERE space_id=$1 ORDER BY profile_id LIMIT 10001`, &state.Members},
		{`SELECT account_id::text FROM public.space_bans WHERE space_id=$1 ORDER BY account_id LIMIT 10001`, &state.BannedAccounts},
		{`SELECT id::text FROM public.voice_rooms WHERE space_id=$1 ORDER BY id LIMIT 10001`, &state.VoiceRooms},
		{`SELECT id::text FROM public.categories WHERE space_id=$1 ORDER BY id LIMIT 10001`, &state.Categories},
	} {
		err = readSpaceSourceRows(ctx, tx, set.query, scope.SpaceId, func(row pgx.Rows) error {
			var id string
			if err := row.Scan(&id); err != nil {
				return err
			}
			*set.target = append(*set.target, id)
			return nil
		})
		if err != nil {
			return authoritysource.State{}, err
		}
	}
	err = readSpaceSourceRows(ctx, tx, `SELECT profile_id::text,timed_out_until FROM public.space_member_timeouts WHERE space_id=$1 ORDER BY profile_id LIMIT 10001`, scope.SpaceId, func(row pgx.Rows) error {
		var item authoritysource.SpaceTimeout
		var until time.Time
		if err := row.Scan(&item.ProfileID, &until); err != nil {
			return err
		}
		item.UntilUnixMillis = until.UnixMilli()
		state.Timeouts = append(state.Timeouts, item)
		return nil
	})
	if err != nil {
		return authoritysource.State{}, err
	}
	err = readSpaceSourceRows(ctx, tx, `SELECT id::text,COALESCE(category_id::text,''),kind,COALESCE(chat_id::text,voice_room_id::text) FROM public.space_tree_nodes WHERE space_id=$1 ORDER BY id LIMIT 10001`, scope.SpaceId, func(row pgx.Rows) error {
		var item authoritysource.SpaceTreeResource
		if err := row.Scan(&item.NodeID, &item.CategoryID, &item.Kind, &item.ResourceID); err != nil {
			return err
		}
		state.Tree = append(state.Tree, item)
		return nil
	})
	if err != nil {
		return authoritysource.State{}, err
	}
	community := authoritysource.CommunityAuthority{}
	var until *time.Time
	err = tx.QueryRow(ctx, `SELECT application_id::text,environment_id::text,owner_account_id::text,owner_profile_id::text,owner_generation,status,roster_source_revision,roster_lease_expires_at FROM public.community_owner_authority WHERE space_id=$1`, scope.SpaceId).Scan(&community.ApplicationID, &community.EnvironmentID, &community.OwnerAccountID, &community.OwnerProfileID, &community.OwnerGeneration, &community.Status, &community.RosterRevision, &until)
	if err == nil {
		if until != nil {
			community.LeaseUntilUnixMillis = until.UnixMilli()
		}
		state.Community = &community
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return authoritysource.State{}, err
	}
	err = readSpaceSourceRows(ctx, tx, `SELECT profile_id::text,owner_generation,source_revision,lease_expires_at,revoked_at IS NOT NULL FROM public.community_roster_members WHERE space_id=$1 ORDER BY profile_id LIMIT 10001`, scope.SpaceId, func(row pgx.Rows) error {
		var item authoritysource.CommunityMember
		var lease time.Time
		if err := row.Scan(&item.ProfileID, &item.OwnerGeneration, &item.SourceRevision, &lease, &item.Revoked); err != nil {
			return err
		}
		item.LeaseUntilUnixMillis = lease.UnixMilli()
		state.CommunityMembers = append(state.CommunityMembers, item)
		return nil
	})
	if err != nil {
		return authoritysource.State{}, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return authoritysource.State{}, err
	}
	if _, err = authoritysource.DecodeSpaceState(raw); err != nil {
		return authoritysource.State{}, err
	}
	var sampled time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled); err != nil {
		return authoritysource.State{}, err
	}
	validUntil := state.ValidUntil(sampled.UnixMilli())
	if err = tx.Commit(ctx); err != nil {
		return authoritysource.State{}, err
	}
	return authoritysource.State{Complete: true, Revision: revision, CanonicalState: raw, ValidUntilUnixMillis: validUntil}, nil
}
