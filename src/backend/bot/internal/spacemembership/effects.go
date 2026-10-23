package spacemembership

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"sort"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/bot/internal/store"
)

func (c *Client) ValidateChats(ctx context.Context, r store.MembershipRecord, request *botv1.InstallBotInSpaceRequest) error {
	if c == nil || c.Store == nil || c.chat == nil {
		return ErrUnavailable
	}
	for _, ref := range request.AllowedChats {
		id, err := uuid.Parse(ref.GetId())
		if err != nil {
			return ErrUnavailable
		}
		source, err := c.Store.PrepareMembershipChat(ctx, c.Source, r, id)
		if err != nil {
			return err
		}
		if source.Response == nil {
			call, cancel, err := c.callContext(ctx, "chat", chatv1.ChatScopeAuthorityService_GetBotChatScopeAuthority_FullMethodName, source.Request.OperationId, source.Request)
			if err != nil {
				return err
			}
			source.Response, err = c.chat.GetBotChatScopeAuthority(call, source.Request)
			cancel()
			if err != nil {
				return err
			}
			if _, err = store.NewChatScopeObservation(source.Request, source.Response, true); err != nil {
				return err
			}
			if err = c.Store.SaveMembershipChat(ctx, c.Source, r, source); err != nil {
				return err
			}
		}
		if ref.Type != nil && *ref.Type != source.Response.GetScope().GetChatType() {
			return ErrUnavailable
		}
	}
	return nil
}

// Receipt validators below are unchanged ports of the reviewed native cleanup
// protocol. Only space_uninstall/native is constructed here; legacy/global
// evidence never acquires authority by passing through this producer.
func (c *Client) Cleanup(ctx context.Context, r store.MembershipRecord, _ []uuid.UUID) error {
	if c == nil || c.Store == nil || c.role == nil || c.messaging == nil {
		return ErrUnavailable
	}
	d, err := c.Store.MembershipCleanupDecision(ctx, r)
	if err != nil {
		return err
	}
	decisionHash, scopeHash := d.Digests()
	roleRequest := &rolev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewSHA1(d.ID, []byte("role")).String(), DecisionId: d.ID.String(), DecisionSha256: decisionHash, Purpose: "space_uninstall", BotId: r.BotID.String(), ActorProfileId: r.ActorProfileID.String(), SpaceId: r.SpaceID.String(), FrozenScopeSha256: scopeHash, Evidence: &rolev1.ApplyBotInstallationRemovalRequest_Native{Native: &rolev1.BotRemovalNativeEvidence{InstallationId: r.InstallationID.String(), InstallOperationId: r.AddOperationID.String(), Generation: uint64(r.Generation)}}}
	roleEffect, err := c.Store.PrepareMembershipEffect(ctx, c.Source, r, "role", rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName, roleRequest)
	if err != nil {
		return err
	}
	roleResponse := new(rolev1.ApplyBotInstallationRemovalResponse)
	if roleEffect.Acknowledged {
		if proto.Unmarshal(roleEffect.ResponseBytes, roleResponse) != nil {
			return ErrUnavailable
		}
	} else {
		call, cancel, e := c.callContext(ctx, "role", roleEffect.Method, roleRequest.OperationId, roleRequest)
		if e != nil {
			return e
		}
		roleResponse, err = c.role.ApplyBotInstallationRemoval(call, roleRequest)
		cancel()
		if err != nil {
			return err
		}
	}
	if err = validateRoleDeletionReceipt(roleRequest, roleResponse); err != nil {
		return err
	}
	if !roleEffect.Acknowledged {
		roleEffect.ResponseBytes, err = proto.MarshalOptions{Deterministic: true}.Marshal(roleResponse)
		if err != nil {
			return err
		}
		if err = c.Store.RecordMembershipEffectAcknowledged(ctx, c.Source, r, roleEffect); err != nil {
			return err
		}
	}
	byChat := map[uuid.UUID][]store.MembershipChatSource{}
	for _, s := range d.Sources {
		byChat[s.ChatID] = append(byChat[s.ChatID], s)
	}
	chats := make([]uuid.UUID, 0, len(byChat))
	for id := range byChat {
		chats = append(chats, id)
	}
	sort.Slice(chats, func(i, j int) bool { return chats[i].String() < chats[j].String() })
	for _, chat := range chats {
		key := "messaging:" + chat.String()
		req := &messagingv1.ApplyBotChatPinRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewSHA1(d.ID, []byte(key)).String(), DecisionId: d.ID.String(), DecisionSha256: decisionHash, Purpose: "space_uninstall", BotId: r.BotID.String(), ActorProfileId: r.ActorProfileID.String(), SpaceId: r.SpaceID.String(), ChatId: chat.String(), FrozenScopeSha256: scopeHash, Evidence: &messagingv1.ApplyBotChatPinRemovalRequest_Native{Native: &messagingv1.BotRemovalNativeEvidence{InstallationId: r.InstallationID.String(), InstallOperationId: r.AddOperationID.String(), Generation: uint64(r.Generation)}}}
		for _, s := range byChat[chat] {
			req.Attachments = append(req.Attachments, &messagingv1.BotChatPinRemovalAttachment{AttachmentId: s.AttachmentID.String(), OriginalScopeEvidence: &messagingv1.BotChatPinRemovalAttachment_OriginalScope{OriginalScope: &messagingv1.BotRemovalChatScopeObservation{Request: s.Request, Response: s.Response}}})
		}
		sort.Slice(req.Attachments, func(i, j int) bool { return req.Attachments[i].AttachmentId < req.Attachments[j].AttachmentId })
		effect, e := c.Store.PrepareMembershipEffect(ctx, c.Source, r, key, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName, req)
		if e != nil {
			return e
		}
		response := new(messagingv1.ApplyBotChatPinRemovalResponse)
		if effect.Acknowledged {
			if proto.Unmarshal(effect.ResponseBytes, response) != nil {
				return ErrUnavailable
			}
		} else {
			call, cancel, e := c.callContext(ctx, "messaging", effect.Method, req.OperationId, req)
			if e != nil {
				return e
			}
			response, err = c.messaging.ApplyBotChatPinRemoval(call, req)
			cancel()
			if err != nil {
				return err
			}
		}
		if err = validateMessagingDeletionReceipt(req, response); err != nil {
			return err
		}
		if !effect.Acknowledged {
			effect.ResponseBytes, err = proto.MarshalOptions{Deterministic: true}.Marshal(response)
			if err != nil {
				return err
			}
			if err = c.Store.RecordMembershipEffectAcknowledged(ctx, c.Source, r, effect); err != nil {
				return err
			}
		}
	}
	return nil
}
