package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/role/permissions"
)

var _ authoritysource.Reader = (*RoleStore)(nil)

// CheckAuthoritySourceSchema gates registration before the dedicated listener
// serves. Every subsequent read repeats the guard inside its consistent cut.
func (s *RoleStore) CheckAuthoritySourceSchema(ctx context.Context) error {
	if s == nil || s.tx != nil {
		return errors.New("source preflight requires owning pool")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := authoritysource.BeginRead(ctx, s.Pool, roleAuthorityCatalog)
	if err != nil {
		return err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	var revision uint64
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.role_sdk_authority_revision WHERE singleton`).Scan(&revision); err != nil || revision == 0 {
		return errors.New("sDK source revision unavailable")
	}
	return tx.Commit(ctx)
}

func roleSourceRevisions(ctx context.Context, tx pgx.Tx, space string) (uint64, uint64, error) {
	var role, sdk uint64
	err := tx.QueryRow(ctx, `SELECT r.policy_epoch,s.revision FROM public.role_voice_policy_epochs r
 CROSS JOIN public.role_sdk_authority_revision s WHERE r.space_id=$1 AND s.singleton`, space).Scan(&role, &sdk)
	if err != nil || role == 0 || sdk == 0 {
		return 0, 0, errors.New("complete Role revision unavailable")
	}
	var conflict bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.role_voice_policy_outbox WHERE space_id=$1 AND policy_epoch>$2)`, space, role).Scan(&conflict)
	if err != nil || conflict {
		return 0, 0, errors.New("role revision conflicts with durable history")
	}
	return role, sdk, nil
}

func (s *RoleStore) beginAuthorityRead(ctx context.Context, scope *authorityv1.SourceScope) (pgx.Tx, error) {
	if err := authoritysource.ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_ROLE, scope); err != nil {
		return nil, err
	}
	if s == nil || s.tx != nil {
		return nil, errors.New("source read requires owning pool")
	}
	return authoritysource.BeginRead(ctx, s.Pool, roleAuthorityCatalog)
}

func (s *RoleStore) ReadAuthorityRevision(ctx context.Context, scope *authorityv1.SourceScope) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginAuthorityRead(ctx, scope)
	if err != nil {
		return 0, err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	role, sdk, err := roleSourceRevisions(ctx, tx, scope.SpaceId)
	if err != nil {
		return 0, err
	}
	// Both positive bigint clocks only increase. The sum fits uint64 and changes
	// for every Role or SDK mutation, unlike a potentially reversible state hash.
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return role + sdk, nil
}

func readRoleSourceRows(ctx context.Context, tx pgx.Tx, query string, args []any, scan func(pgx.Rows) error) error {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > authoritysource.MaxSubjects {
			return errors.New("complete Role source exceeds bound")
		}
		if err = scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *RoleStore) ReadAuthoritySnapshot(ctx context.Context, scope *authorityv1.SourceScope) (authoritysource.State, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginAuthorityRead(ctx, scope)
	if err != nil {
		return authoritysource.State{}, err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	role, sdk, err := roleSourceRevisions(ctx, tx, scope.SpaceId)
	if err != nil {
		return authoritysource.State{}, err
	}
	all, err := permissions.AllMask()
	if err != nil {
		return authoritysource.State{}, err
	}
	state := authoritysource.RoleState{SchemaVersion: authoritysource.SchemaVersion, SpaceID: scope.SpaceId,
		RoleRevision: role, SDKRevision: sdk, AllPermissions: all, Roles: []authoritysource.RoleDefinition{},
		Assignments: []authoritysource.RoleAssignment{}, ChatOverrides: []authoritysource.RoleOverride{},
		VoiceOverrides: []authoritysource.RoleOverride{}, SDKSessionGrants: []authoritysource.SDKSessionGrant{}}
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL),
 EXISTS(SELECT 1 FROM public.ownership_transfer_v2 WHERE space_id=$1 AND state='prepared'),
 COALESCE((SELECT state FROM public.role_space_deletion_fences WHERE space_id=$1),''),
 COALESCE((SELECT generation FROM public.role_space_deletion_fences WHERE space_id=$1),0)`, scope.SpaceId).
		Scan(&state.Retired, &state.OwnershipFrozen, &state.DeletionState, &state.DeletionGeneration)
	if err != nil {
		return authoritysource.State{}, err
	}
	knownRoles := map[string]bool{}
	defaults := 0
	err = readRoleSourceRows(ctx, tx, `SELECT id::text,name,permissions,is_default_join FROM public.roles WHERE space_id=$1 ORDER BY id LIMIT 10001`, []any{scope.SpaceId}, func(row pgx.Rows) error {
		var item authoritysource.RoleDefinition
		var name string
		if err := row.Scan(&item.ID, &name, &item.Permissions, &item.DefaultJoin); err != nil {
			return err
		}
		if item.Permissions&^all != 0 {
			return errors.New("unknown Role authority bits")
		}
		item.Owner = name == permissions.RoleOwner
		knownRoles[item.ID] = true
		if item.DefaultJoin {
			defaults++
		}
		state.Roles = append(state.Roles, item)
		return nil
	})
	if err != nil {
		return authoritysource.State{}, err
	}
	if defaults > 1 {
		return authoritysource.State{}, errors.New("multiple default Role definitions")
	}
	err = readRoleSourceRows(ctx, tx, `SELECT space_id::text,profile_id::text,role_id::text FROM public.member_roles
 WHERE space_id=$1 OR role_id IN (SELECT id FROM public.roles WHERE space_id=$1) ORDER BY space_id,profile_id,role_id LIMIT 10001`, []any{scope.SpaceId}, func(row pgx.Rows) error {
		var item authoritysource.RoleAssignment
		var space string
		if err := row.Scan(&space, &item.ProfileID, &item.RoleID); err != nil {
			return err
		}
		if space != scope.SpaceId || !knownRoles[item.RoleID] {
			return errors.New("role assignment crosses owner scope")
		}
		state.Assignments = append(state.Assignments, item)
		return nil
	})
	if err != nil {
		return authoritysource.State{}, err
	}
	for _, kind := range []struct {
		table, id   string
		destination *[]authoritysource.RoleOverride
	}{
		{"chat_overrides", "chat_id", &state.ChatOverrides}, {"voice_room_overrides", "voice_room_id", &state.VoiceOverrides},
	} {
		query := `SELECT o.` + kind.id + `::text,o.role_id::text,o.allow,o.deny FROM public.` + kind.table + ` o
  JOIN public.roles r ON r.id=o.role_id WHERE r.space_id=$1 ORDER BY o.` + kind.id + `,o.role_id LIMIT 10001`
		err = readRoleSourceRows(ctx, tx, query, []any{scope.SpaceId}, func(row pgx.Rows) error {
			var item authoritysource.RoleOverride
			if err := row.Scan(&item.ResourceID, &item.RoleID, &item.Allow, &item.Deny); err != nil {
				return err
			}
			if (item.Allow|item.Deny)&^all != 0 {
				return errors.New("unknown override authority bits")
			}
			*kind.destination = append(*kind.destination, item)
			return nil
		})
		if err != nil {
			return authoritysource.State{}, err
		}
	}
	if len(scope.VoiceRoomIds) > 0 {
		err = readRoleSourceRows(ctx, tx, `SELECT g.application_id::text,g.environment_id::text,g.session_id::text,
  g.voice_room_id::text,g.profile_id::text,g.roster_revision,s.voice_room_id::text,s.roster_revision,s.status,g.permission
  FROM public.game_session_grants g JOIN public.game_session_grant_sessions s USING(application_id,environment_id,session_id)
  WHERE g.voice_room_id=ANY($1::uuid[]) OR s.voice_room_id=ANY($1::uuid[])
  ORDER BY g.application_id,g.environment_id,g.session_id,g.profile_id LIMIT 10001`, []any{scope.VoiceRoomIds}, func(row pgx.Rows) error {
			var item authoritysource.SDKSessionGrant
			var room *string
			var revision uint64
			var status, permission string
			if err := row.Scan(&item.ApplicationID, &item.EnvironmentID, &item.SessionID, &item.VoiceRoomID, &item.ProfileID, &item.RosterRevision, &room, &revision, &status, &permission); err != nil {
				return err
			}
			if room == nil || *room != item.VoiceRoomID || revision == 0 || revision != item.RosterRevision || status != "active" || permission != "VOICE_JOIN" {
				return errors.New("sDK grant conflicts with owning session")
			}
			state.SDKSessionGrants = append(state.SDKSessionGrants, item)
			return nil
		})
		if err != nil {
			return authoritysource.State{}, err
		}
	}
	raw, err := json.Marshal(state)
	if err != nil || len(raw) > authoritysource.MaxStateBytes {
		return authoritysource.State{}, errors.New("complete Role source exceeds bound")
	}
	if _, err = authoritysource.DecodeRoleState(raw); err != nil {
		return authoritysource.State{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return authoritysource.State{}, err
	}
	return authoritysource.State{Complete: true, Revision: role + sdk, CanonicalState: raw}, nil
}
