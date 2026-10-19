package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

func readQualifiedRoleAuthoritySnapshot(ctx context.Context, tx pgx.Tx, scope *authorityv1.SourceScope) (authoritysource.State, error) {
	state, err := readMemberRoleState(ctx, tx, scope.SpaceId)
	if err != nil {
		return authoritysource.State{}, err
	}
	if len(scope.VoiceRoomIds) > 0 {
		err = readRoleSourceRows(ctx, tx, `SELECT g.application_id::text,g.environment_id::text,g.session_id::text,g.voice_room_id::text,g.profile_id::text,g.roster_revision,s.voice_room_id::text,s.roster_revision,s.status,g.permission FROM game_session_grants g JOIN game_session_grant_sessions s USING(application_id,environment_id,session_id) WHERE g.voice_room_id=ANY($1::uuid[]) OR s.voice_room_id=ANY($1::uuid[]) ORDER BY g.application_id,g.environment_id,g.session_id,g.profile_id LIMIT 10001`, []any{scope.VoiceRoomIds}, func(row pgx.Rows) error {
			var g authoritysource.SDKSessionGrant
			var room *string
			var revision uint64
			var status, permission string
			if err := row.Scan(&g.ApplicationID, &g.EnvironmentID, &g.SessionID, &g.VoiceRoomID, &g.ProfileID, &g.RosterRevision, &room, &revision, &status, &permission); err != nil {
				return err
			}
			if room == nil || *room != g.VoiceRoomID || revision == 0 || revision != g.RosterRevision || status != "active" || permission != "VOICE_JOIN" {
				return errors.New("SDK grant conflicts with current owner session")
			}
			state.SDKSessionGrants = append(state.SDKSessionGrants, g)
			return nil
		})
		if err != nil {
			return authoritysource.State{}, err
		}
	}
	raw, err := json.Marshal(state)
	if err != nil || len(raw) > authoritysource.MaxStateBytes {
		return authoritysource.State{}, errors.New("qualified Role source exceeds bound")
	}
	if _, err = authoritysource.DecodeRoleStateV2(raw); err != nil {
		return authoritysource.State{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return authoritysource.State{}, err
	}
	return authoritysource.State{Complete: true, Revision: state.RoleRevision + state.SDKRevision, CanonicalState: raw}, nil
}
