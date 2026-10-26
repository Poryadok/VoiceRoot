package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

var _ authoritysource.SchemaReader = (*Store)(nil)

func (s *Store) beginGISSource(ctx context.Context) (pgx.Tx, error) {
	if s == nil {
		return nil, errors.New("GIS owning pool unavailable")
	}
	return authoritysource.BeginRead(ctx, s.Pool, gisAuthorityCatalog)
}
func (s *Store) CheckAuthoritySourceSchema(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginGISSource(ctx)
	if err != nil {
		return err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	if _, err = gisSourceRevision(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func gisSourceRevision(ctx context.Context, tx pgx.Tx) (uint64, error) {
	var revision uint64
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.gis_authority_revision WHERE singleton AND (SELECT count(*) FROM public.gis_authority_revision)=1`).Scan(&revision); err != nil || revision == 0 {
		return 0, errors.New("GIS authority floor unavailable")
	}
	return revision, nil
}
func (s *Store) ReadAuthorityRevision(ctx context.Context, scope *authorityv1.SourceScope) (uint64, error) {
	if err := authoritysource.ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_GAME_INTEGRATION, scope); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginGISSource(ctx)
	if err != nil {
		return 0, err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	revision, err := gisSourceRevision(ctx, tx)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return revision, nil
}
func readGISRows[T any](ctx context.Context, tx pgx.Tx, query string, args ...any) ([]T, error) {
	values := []T{}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		if len(values) >= authoritysource.MaxSubjects {
			return nil, errors.New("GIS source exceeds bound")
		}
		var raw []byte
		var value T
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Store) ReadAuthoritySnapshot(ctx context.Context, scope *authorityv1.SourceScope) (authoritysource.State, error) {
	if err := authoritysource.ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_GAME_INTEGRATION, scope); err != nil {
		return authoritysource.State{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := s.beginGISSource(ctx)
	if err != nil {
		return authoritysource.State{}, err
	}
	defer authoritysource.RollbackRead(ctx, tx)
	revision, err := gisSourceRevision(ctx, tx)
	if err != nil {
		return authoritysource.State{}, err
	}
	state := authoritysource.GISState{SchemaVersion: 1, Revision: revision, SpaceID: scope.SpaceId, EnvironmentID: scope.EnvironmentId, Subjects: append([]string{}, scope.ProfileIds...), Bindings: []authoritysource.GISBinding{}, Installations: []authoritysource.GISInstallation{}, Mappings: []authoritysource.GISMapping{}, ManagedChats: []authoritysource.GISManagedChat{}, Sessions: []authoritysource.GISSession{}}
	var policy, redirects, origins []byte
	err = tx.QueryRow(ctx, `SELECT a.id::text,a.status,a.revision,e.kind,e.status,e.revision,e.provider_policy,e.redirect_uris,e.allowed_origins FROM public.environments e JOIN public.applications a ON a.id=e.application_id WHERE e.id=$1`, scope.EnvironmentId).Scan(&state.ApplicationID, &state.ApplicationStatus, &state.ApplicationRevision, &state.EnvironmentKind, &state.EnvironmentStatus, &state.EnvironmentRevision, &policy, &redirects, &origins)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return authoritysource.State{}, err
	}
	state.EnvironmentExists = err == nil
	if state.EnvironmentExists {
		// Several legacy tables have separate app/env foreign keys. Refuse any
		// inconsistent source row rather than silently losing a revocation fact.
		var invalid bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM public.player_bindings WHERE environment_id=$1 AND (application_id<>$2 OR permitted_character_context<>'[]'::jsonb)
 UNION ALL SELECT 1 FROM public.installations WHERE environment_id=$1 AND application_id<>$2
 UNION ALL SELECT 1 FROM public.gis_sessions WHERE environment_id=$1 AND application_id<>$2
 UNION ALL SELECT 1 FROM public.gis_app_notification_consents WHERE environment_id=$1 AND application_id<>$2
 )`, scope.EnvironmentId, state.ApplicationID).Scan(&invalid)
		if err != nil || invalid {
			return authoritysource.State{}, errors.New("GIS source authority tuple or character schema unavailable")
		}
		var p struct {
			Providers    []string `json:"providers"`
			PlayerScopes []string `json:"player_scopes"`
		}
		decoder := json.NewDecoder(bytes.NewReader(policy))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&p) != nil || json.Unmarshal(redirects, &state.RedirectURIs) != nil || json.Unmarshal(origins, &state.AllowedOrigins) != nil {
			return authoritysource.State{}, errors.New("GIS policy unavailable")
		}
		state.Providers = append([]string{}, p.Providers...)
		state.PlayerScopes = append([]string{}, p.PlayerScopes...)
		for _, items := range [][]string{state.Providers, state.PlayerScopes, state.RedirectURIs, state.AllowedOrigins} {
			slices.Sort(items)
		}
		state.Bindings, err = readGISRows[authoritysource.GISBinding](ctx, tx, `SELECT json_build_object('binding_id',binding_id,'account_id',account_id,'actor_id',actor_id,'profile_id',profile_id,'device_id',device_id,'status',status,'authority_revision',authority_revision) FROM public.player_bindings WHERE application_id=$1 AND environment_id=$2 AND (profile_id=ANY($3::uuid[]) OR actor_id=ANY($3::uuid[])) ORDER BY binding_id LIMIT 10001`, state.ApplicationID, scope.EnvironmentId, scope.ProfileIds)
		if err != nil {
			return authoritysource.State{}, err
		}
		state.Installations, err = readGISRows[authoritysource.GISInstallation](ctx, tx, `SELECT json_build_object('installation_id',id,'bot_id',COALESCE(bot_id::text,''),'status',status) FROM public.installations WHERE application_id=$1 AND environment_id=$2 ORDER BY id LIMIT 10001`, state.ApplicationID, scope.EnvironmentId)
		if err != nil {
			return authoritysource.State{}, err
		}
		state.Mappings, err = readGISRows[authoritysource.GISMapping](ctx, tx, `SELECT json_build_object('mapping_id',mapping_id,'kind',resource_kind,'resource_id',resource_id,'chat_id',COALESCE(chat_id::text,''),'status',status,'revision',mapping_revision) FROM public.game_resource_mappings WHERE application_id=$1 AND environment_id=$2 ORDER BY mapping_id LIMIT 10001`, state.ApplicationID, scope.EnvironmentId)
		if err != nil {
			return authoritysource.State{}, err
		}
		state.ManagedChats, err = readGISRows[authoritysource.GISManagedChat](ctx, tx, `SELECT json_build_object('binding_id',r.binding_id,'chat_id',r.chat_id,'status',r.status,'roster_revision',r.roster_revision,'lease_until_unix_millis',floor(extract(epoch FROM r.lease_expires_at)*1000)::bigint) FROM public.game_resource_binding_chats r JOIN public.player_bindings b ON (b.application_id,b.environment_id,b.binding_id)=(r.application_id,r.environment_id,r.binding_id) WHERE r.application_id=$1 AND r.environment_id=$2 AND (b.profile_id=ANY($3::uuid[]) OR b.actor_id=ANY($3::uuid[])) ORDER BY r.binding_id,r.chat_id LIMIT 10001`, state.ApplicationID, scope.EnvironmentId, scope.ProfileIds)
		if err != nil {
			return authoritysource.State{}, err
		}
		state.Sessions, err = readGISRows[authoritysource.GISSession](ctx, tx, `SELECT json_build_object('session_id',id,'kind',kind,'status',session_status,'roster_revision',roster_revision,'roster_complete',roster_complete,'members',ARRAY(SELECT DISTINCT p FROM unnest(members) p WHERE p=ANY($3::uuid[]) ORDER BY p),'chat_id',COALESCE(chat_id::text,''),'voice_room_id',COALESCE(voice_room_id::text,'')) FROM public.gis_sessions WHERE application_id=$1 AND environment_id=$2 AND members&&$3::uuid[] ORDER BY id LIMIT 10001`, state.ApplicationID, scope.EnvironmentId, scope.ProfileIds)
		if err != nil {
			return authoritysource.State{}, err
		}
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return authoritysource.State{}, err
	}
	if _, err = authoritysource.DecodeGISState(raw); err != nil {
		return authoritysource.State{}, err
	}
	var sampled time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled); err != nil {
		return authoritysource.State{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return authoritysource.State{}, err
	}
	return authoritysource.State{Complete: true, Revision: revision, CanonicalState: raw, ValidUntilUnixMillis: state.ValidUntilUnixMillis(sampled.UnixMilli())}, nil
}
