package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	chatv1 "voice.app/voice/chat/v1"
)

// GetSpaceChatScopeAuthority uses one statement snapshot of the Chat-owned
// resource, permanent head and independent lifecycle fence. No remote lookup or
// lock over network I/O can participate in this read.
func (s *DMStore) GetSpaceChatScopeAuthority(ctx context.Context, chatID, spaceID uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("chat scope authority unavailable")
	}
	var incarnation, headIncarnation uuid.UUID
	var revision, generation, attachmentGeneration, headAttachmentGeneration int64
	var kind, state string
	var present, gis, squad bool
	err := s.Pool.QueryRow(ctx, `SELECT c.type,c.scope_incarnation,h.scope_incarnation,h.revision,h.present,
        COALESCE(f.generation,0),COALESCE(f.state,'LIVE'),c.managed_by_application_id IS NOT NULL,
        EXISTS(SELECT 1 FROM chat_match_squad_operations m WHERE m.chat_id=c.id),c.report_attachment_generation,h.report_attachment_generation
        FROM chats c LEFT JOIN chat_scope_authority_heads h ON h.chat_id=c.id AND h.space_id=c.space_id
        LEFT JOIN chat_space_lifecycle_fences f ON f.space_id=c.space_id
        WHERE c.id=$1 AND c.space_id=$2`, chatID, spaceID).Scan(&kind, &incarnation, &headIncarnation, &revision, &present, &generation, &state, &gis, &squad, &attachmentGeneration, &headAttachmentGeneration)
	if err != nil {
		return nil, err
	}
	if incarnation == uuid.Nil || incarnation != headIncarnation || revision <= 0 || !present || generation < 0 || attachmentGeneration <= 0 || attachmentGeneration != headAttachmentGeneration {
		return nil, errors.New("chat scope authority inconsistent")
	}
	response := &chatv1.GetSpaceChatScopeAuthorityResponse{
		ProtocolVersion: 1, ChatId: chatID.String(), ExpectedSpaceId: spaceID.String(), ActualSpaceId: spaceID.String(),
		Exists: true, ChatScopeIncarnation: incarnation.String(), ChatAuthorityRevision: uint64(revision),
		LifecycleGeneration: uint64(generation), ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY,
		ReportAttachmentGeneration: uint64(attachmentGeneration),
	}
	switch kind {
	case "group":
		response.ChatType = chatv1.ChatType_CHAT_TYPE_GROUP
	case "channel":
		response.ChatType = chatv1.ChatType_CHAT_TYPE_CHANNEL
	default:
		return nil, errors.New("unsupported Chat scope type")
	}
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
		return nil, errors.New("unknown Chat lifecycle state")
	}
	if gis {
		response.ManagedKind = chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_GAME_INTEGRATION
	}
	if squad {
		response.ManagedKind = chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD
	}
	response.Live = state == "LIVE" && !gis && !squad
	return response, nil
}
