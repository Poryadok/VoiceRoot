package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	chatv1 "voice.app/voice/chat/v1"
)

// LoadBotPinChatScope uses the existing authorized Chat SQL datasource. A
// retained head is required even when the Chat row was deleted; absence alone
// never proves that this cleanup target has completed.
func (s *SQLChatThreadPolicy) LoadBotPinChatScope(ctx context.Context, chatID, spaceID uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrStoreNotConfigured
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
	if incarnation == uuid.Nil || revision <= 0 || generation < 0 || present != (rowID != nil) || (rowID != nil && (rowSpace == nil || *rowSpace != spaceID || rowIncarnation == nil || *rowIncarnation != incarnation || kind == nil || (*kind != "group" && *kind != "channel"))) {
		return nil, errors.New("Bot pin Chat source is inconsistent")
	}
	response := &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: chatID.String(), ExpectedSpaceId: spaceID.String(), ActualSpaceId: spaceID.String(), ChatScopeIncarnation: incarnation.String(), ChatAuthorityRevision: uint64(revision), LifecycleGeneration: uint64(generation), Exists: rowID != nil}
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
		return nil, errors.New("Bot pin Chat lifecycle is inconsistent")
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
