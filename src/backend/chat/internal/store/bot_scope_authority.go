package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	chatv1 "voice.app/voice/chat/v1"
)

// GetBotChatScopeAuthority reads the current resource or its retained last
// attachment in one statement snapshot. It never changes the permanent head.
func (s *DMStore) GetBotChatScopeAuthority(ctx context.Context, chatID, spaceID uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("chat Bot scope authority unavailable")
	}
	var incarnation uuid.UUID
	var rowID, rowSpace, rowIncarnation *uuid.UUID
	var revision, generation int64
	var present, gis, squad bool
	var kind *string
	var state string
	err := s.Pool.QueryRow(ctx, `SELECT h.scope_incarnation,h.revision,h.present,c.id,c.space_id,c.scope_incarnation,c.type,
        COALESCE(f.generation,0),COALESCE(f.state,'LIVE'),COALESCE(c.managed_by_application_id IS NOT NULL,false),
        EXISTS(SELECT 1 FROM chat_match_squad_operations m WHERE m.chat_id=c.id)
        FROM chat_scope_authority_heads h LEFT JOIN chats c ON c.id=h.chat_id
        LEFT JOIN chat_space_lifecycle_fences f ON f.space_id=h.space_id
        WHERE h.chat_id=$1 AND h.space_id=$2`, chatID, spaceID).Scan(&incarnation, &revision, &present, &rowID, &rowSpace, &rowIncarnation, &kind, &generation, &state, &gis, &squad)
	if err != nil {
		return nil, err
	}
	if rowID != nil && (rowSpace == nil || *rowSpace != spaceID || kind == nil || (*kind != "group" && *kind != "channel")) {
		return nil, pgx.ErrNoRows
	}
	if incarnation == uuid.Nil || revision <= 0 || generation < 0 || present != (rowID != nil) || (rowID != nil && (rowIncarnation == nil || *rowIncarnation != incarnation)) {
		return nil, errors.New("chat Bot scope authority inconsistent")
	}
	response := &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: chatID.String(), ExpectedSpaceId: spaceID.String(), ActualSpaceId: spaceID.String(), Exists: rowID != nil, ChatScopeIncarnation: incarnation.String(), ChatAuthorityRevision: uint64(revision), LifecycleGeneration: uint64(generation)}
	switch state {
	case "LIVE":
		response.LifecycleState = chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE
	case "FROZEN":
		response.LifecycleState = chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_FROZEN
	case "PURGE_DECIDED":
		response.LifecycleState = chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_PURGE_DECIDED
	case "PURGED":
		response.LifecycleState = chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_PURGED
	default:
		return nil, errors.New("chat Bot lifecycle authority inconsistent")
	}
	if rowID != nil {
		response.Live = state == "LIVE"
		response.ChatType = chatv1.ChatType_CHAT_TYPE_GROUP
		if *kind == "channel" {
			response.ChatType = chatv1.ChatType_CHAT_TYPE_CHANNEL
		}
		response.ManagedKind = chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY
		if gis {
			response.ManagedKind = chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_GAME_INTEGRATION
		}
		if squad {
			response.ManagedKind = chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD
		}
	}
	return response, nil
}
