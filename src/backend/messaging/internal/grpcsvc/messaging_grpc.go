package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/messaging/internal/messageid"
	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
	fallbackSize    = 50
)

// MessagingGRPC implements MessagingService (Phase 1: DM send, history, read receipts).
type MessagingGRPC struct {
	messagingv1.UnimplementedMessagingServiceServer
	Messages  *store.MessagesStore
	Reactions *store.ReactionsStore
	ChatGuard ChatGuard
	// Blocks and UserProfiles are optional S2S gates for SendMessage (Social + User); both must be set to enforce.
	Blocks       AccountPairBlockChecker
	UserProfiles ProfileAccountLookup
	// Files is optional for text-only messages and required for non-empty attachments_json.
	Files FileMetadataLookup
	// MessageEvents is optional; when set, successful send/edit/delete publishes to NATS JetStream (stream message_events, subjects message.*).
	MessageEvents messageevents.MessageEventsPublisher
}

func (s *MessagingGRPC) SendMessage(ctx context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	if err := validateChatRefMessaging(req.GetChat()); err != nil {
		return nil, err
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if err := s.checkDMBlocksForSend(ctx, chatID, profileID); err != nil {
		return nil, err
	}
	attachments := strings.TrimSpace(req.GetAttachmentsJson())
	if attachments == "" {
		attachments = "[]"
	}
	attachmentCount, err := s.validateAttachments(ctx, chatID, attachments)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.GetContent())
	if content == "" && attachmentCount == 0 {
		return nil, status.Error(codes.InvalidArgument, "content or attachments is required")
	}
	if len(content) > 4000 {
		return nil, status.Error(codes.InvalidArgument, "content exceeds 4000 characters")
	}
	mentions := strings.TrimSpace(req.GetMentionsJson())
	if mentions == "" {
		mentions = "[]"
	}
	typeStr, kind := messageKindToWire(req.GetMessageKind())

	var threadParent *uuid.UUID
	if req.GetThreadParentId() != "" {
		tid, err := parseUUIDField("thread_parent_id", req.GetThreadParentId())
		if err != nil {
			return nil, err
		}
		threadParent = &tid
	}

	var clientID *uuid.UUID
	if cid := strings.TrimSpace(req.GetClientMessageId()); cid != "" {
		parsed, err := parseUUIDField("client_message_id", cid)
		if err != nil {
			return nil, err
		}
		clientID = &parsed
	}

	msgID, err := messageid.NewMessageID()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	row := store.MessageRow{
		ID:              msgID,
		ChatID:          chatID,
		SenderProfileID: profileID,
		Content:         content,
		Type:            typeStr,
		ThreadParentID:  threadParent,
		AttachmentsJSON: attachments,
		MentionsJSON:    mentions,
		ClientMessageID: clientID,
	}
	saved, err := s.Messages.InsertMessage(ctx, row)
	if err != nil {
		if strings.Contains(err.Error(), "attachments_json") || strings.Contains(err.Error(), "mentions_json") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.MessageEvents != nil {
		if err := s.MessageEvents.PublishMessageSent(ctx, saved.ID.String(), saved.ChatID.String(), saved.SenderProfileID.String()); err != nil {
			log.Printf("messaging: publish message.sent: %v", err)
		}
	}
	return &messagingv1.SendMessageResponse{Message: messageRowToProto(saved, kind, "")}, nil
}

type messageAttachment struct {
	FileID     string `json:"file_id"`
	Type       string `json:"type"`
	URL        string `json:"url,omitempty"`
	PreviewURL string `json:"preview_url,omitempty"`
}

func (s *MessagingGRPC) validateAttachments(ctx context.Context, chatID uuid.UUID, raw string) (int, error) {
	var attachments []messageAttachment
	if err := json.Unmarshal([]byte(raw), &attachments); err != nil {
		return 0, status.Error(codes.InvalidArgument, "attachments_json must be a JSON array")
	}
	if len(attachments) == 0 {
		return 0, nil
	}
	if s.Files == nil {
		return 0, status.Error(codes.FailedPrecondition, "file metadata lookup is not configured")
	}
	fileIDs := make([]string, 0, len(attachments))
	for _, att := range attachments {
		fileID := strings.TrimSpace(att.FileID)
		if _, err := parseUUIDField("attachments.file_id", fileID); err != nil {
			return 0, err
		}
		if strings.TrimSpace(att.Type) == "" {
			return 0, status.Error(codes.InvalidArgument, "attachments.type is required")
		}
		fileIDs = append(fileIDs, fileID)
	}
	resp, err := s.Files.GetBulkMetadata(ctx, &filev1.GetBulkMetadataRequest{FileIds: fileIDs})
	if err != nil {
		return 0, status.Error(codes.Internal, err.Error())
	}
	byID := resp.GetBulkFileMetadata().GetByFileId()
	for _, att := range attachments {
		meta := byID[strings.TrimSpace(att.FileID)]
		if meta == nil {
			return 0, status.Error(codes.FailedPrecondition, "attachment file is not available")
		}
		if meta.GetStatus() != "ready" {
			return 0, status.Error(codes.FailedPrecondition, "attachment file is not ready")
		}
		if meta.GetChat().GetId() != chatID.String() {
			return 0, status.Error(codes.FailedPrecondition, "attachment file is not linked to chat")
		}
		switch meta.GetScanResult() {
		case "clean", "skipped":
		default:
			return 0, status.Error(codes.FailedPrecondition, "attachment file is not clean")
		}
		if ft := strings.TrimSpace(meta.GetFileType()); ft != "" && ft != strings.TrimSpace(att.Type) {
			return 0, status.Error(codes.InvalidArgument, "attachments.type does not match file metadata")
		}
	}
	return len(attachments), nil
}

func (s *MessagingGRPC) checkDMBlocksForSend(ctx context.Context, chatID, profileID uuid.UUID) error {
	if s.Blocks == nil || s.UserProfiles == nil || s.ChatGuard == nil {
		return nil
	}
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing account")
	}
	peer, err := s.ChatGuard.DMOtherProfileID(ctx, chatID, profileID)
	if err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return status.Error(codes.PermissionDenied, "not a chat member")
		}
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return nil
		}
		if strings.Contains(err.Error(), "dm must have exactly two members") {
			return nil
		}
		return status.Error(codes.Internal, err.Error())
	}
	peerAcct, err := s.UserProfiles.AccountIDByProfileID(ctx, peer)
	if err != nil {
		return err
	}
	blocked, err := s.Blocks.AccountPairBlocked(ctx, accountID, peerAcct)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if blocked {
		return status.Error(codes.PermissionDenied, "cannot send messages between blocked accounts")
	}
	return nil
}

func (s *MessagingGRPC) EditMessage(ctx context.Context, req *messagingv1.EditMessageRequest) (*messagingv1.EditMessageResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	msgID, err := parseUUIDField("message_id", req.GetMessageId())
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.GetContent())
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	if len(content) > 4000 {
		return nil, status.Error(codes.InvalidArgument, "content exceeds 4000 characters")
	}
	row, err := s.Messages.GetMessageByID(ctx, msgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "message not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row.DeletedAt != nil {
		return nil, status.Error(codes.NotFound, "message not found")
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, row.ChatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if row.SenderProfileID != profileID {
		return nil, status.Error(codes.PermissionDenied, "only the message author can edit")
	}
	updated, err := s.Messages.UpdateMessageContent(ctx, msgID, profileID, content)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "message not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.MessageEvents != nil {
		if err := s.MessageEvents.PublishMessageEdited(ctx, updated.ID.String(), updated.ChatID.String()); err != nil {
			log.Printf("messaging: publish message.edited: %v", err)
		}
	}
	return &messagingv1.EditMessageResponse{Message: messageRowToProto(updated, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, "")}, nil
}

func (s *MessagingGRPC) DeleteMessage(ctx context.Context, req *messagingv1.DeleteMessageRequest) (*messagingv1.DeleteMessageResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	msgID, err := parseUUIDField("message_id", req.GetMessageId())
	if err != nil {
		return nil, err
	}
	row, err := s.Messages.GetMessageByID(ctx, msgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "message not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row.DeletedAt != nil {
		return nil, status.Error(codes.NotFound, "message not found")
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, row.ChatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	scope := req.GetScope()
	if scope == messagingv1.DeleteScope_DELETE_SCOPE_UNSPECIFIED {
		scope = messagingv1.DeleteScope_DELETE_SCOPE_FOR_EVERYONE
	}
	if scope == messagingv1.DeleteScope_DELETE_SCOPE_FOR_ME {
		if err := s.Messages.HideMessageForProfile(ctx, msgID, profileID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return &messagingv1.DeleteMessageResponse{}, nil
	}
	if row.SenderProfileID != profileID {
		return nil, status.Error(codes.PermissionDenied, "only the message author can delete for everyone")
	}
	if err := s.Messages.SoftDeleteMessage(ctx, msgID, profileID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "message not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.MessageEvents != nil {
		if err := s.MessageEvents.PublishMessageDeleted(ctx, msgID.String(), row.ChatID.String()); err != nil {
			log.Printf("messaging: publish message.deleted: %v", err)
		}
	}
	return &messagingv1.DeleteMessageResponse{}, nil
}

func (s *MessagingGRPC) GetMessages(ctx context.Context, req *messagingv1.GetMessagesRequest) (*messagingv1.GetMessagesResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	if err := validateChatRefDM(req.GetChat()); err != nil {
		return nil, err
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	pageSize := int(req.GetPage().GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	beforeFromCursor, afterFromCursor, err := store.DecodeHistoryCursor(req.GetPage().GetCursor())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page cursor")
	}

	beforeExplicit := strings.TrimSpace(req.GetBeforeMessageId())
	afterExplicit := strings.TrimSpace(req.GetAfterMessageId())
	lastExplicit := strings.TrimSpace(req.GetLastMessageId())

	if beforeExplicit != "" && afterExplicit != "" {
		return nil, status.Error(codes.InvalidArgument, "before_message_id and after_message_id are mutually exclusive")
	}
	if beforeExplicit != "" && lastExplicit != "" {
		return nil, status.Error(codes.InvalidArgument, "before_message_id and last_message_id are mutually exclusive")
	}
	if afterExplicit != "" && lastExplicit != "" && afterExplicit != lastExplicit {
		return nil, status.Error(codes.InvalidArgument, "after_message_id and last_message_id disagree")
	}

	var useFallback bool
	var mode store.ListMode
	var refID *uuid.UUID

	switch {
	case beforeExplicit != "":
		id, err := parseUUIDField("before_message_id", beforeExplicit)
		if err != nil {
			return nil, err
		}
		exists, err := s.Messages.MessageExists(ctx, chatID, id)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !exists {
			useFallback = true
		} else {
			mode = store.ListBeforeID
			refID = &id
		}
	case afterExplicit != "" || lastExplicit != "":
		raw := afterExplicit
		if raw == "" {
			raw = lastExplicit
		}
		id, err := parseUUIDField("after_message_id", raw)
		if err != nil {
			return nil, err
		}
		exists, err := s.Messages.MessageExists(ctx, chatID, id)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !exists {
			useFallback = true
		} else {
			mode = store.ListAfterID
			refID = &id
		}
	case beforeFromCursor != nil:
		exists, err := s.Messages.MessageExists(ctx, chatID, *beforeFromCursor)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !exists {
			useFallback = true
		} else {
			mode = store.ListBeforeID
			refID = beforeFromCursor
		}
	case afterFromCursor != nil:
		exists, err := s.Messages.MessageExists(ctx, chatID, *afterFromCursor)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !exists {
			useFallback = true
		} else {
			mode = store.ListAfterID
			refID = afterFromCursor
		}
	default:
		mode = store.ListLatest
	}

	limit := pageSize
	if useFallback {
		limit = fallbackSize
		mode = store.ListLatest
		refID = nil
	}

	rows, err := s.Messages.ListMessages(ctx, chatID, profileID, mode, refID, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	msgIDs := make([]uuid.UUID, len(rows))
	for i := range rows {
		msgIDs[i] = rows[i].ID
	}
	reactionsByMsg := map[uuid.UUID]string{}
	if s.Reactions != nil && len(msgIDs) > 0 {
		reactionsByMsg, err = s.Reactions.ReactionsJSONByMessageIDs(ctx, msgIDs, profileID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	msgs := make([]*messagingv1.Message, 0, len(rows))
	for i := range rows {
		msgs = append(msgs, messageRowToProto(&rows[i], messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, reactionsByMsg[rows[i].ID]))
	}

	next := ""
	if hasMore {
		next = nextCursorForPage(mode, rows)
	}

	ml := &messagingv1.MessageList{
		Messages:   msgs,
		NextCursor: next,
		HasMore:    hasMore,
		Page: &commonv1.CursorPageResponse{
			NextCursor: next,
			HasMore:    hasMore,
		},
	}
	return &messagingv1.GetMessagesResponse{MessageList: ml}, nil
}

func nextCursorForPage(mode store.ListMode, rows []store.MessageRow) string {
	if len(rows) == 0 {
		return ""
	}
	switch mode {
	case store.ListAfterID:
		return store.EncodeAfterCursor(rows[len(rows)-1].ID)
	default:
		return store.EncodeBeforeCursor(rows[len(rows)-1].ID)
	}
}

func (s *MessagingGRPC) ForwardMessage(ctx context.Context, req *messagingv1.ForwardMessageRequest) (*messagingv1.ForwardMessageResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	sourceID, err := parseUUIDField("source_message_id", req.GetSourceMessageId())
	if err != nil {
		return nil, err
	}
	if err := validateChatRefMessaging(req.GetTargetChat()); err != nil {
		return nil, err
	}
	targetChatID, err := parseUUIDField("target_chat.id", req.GetTargetChat().GetId())
	if err != nil {
		return nil, err
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, targetChatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	source, err := s.Messages.GetMessageByID(ctx, sourceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "message not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if source.DeletedAt != nil {
		return nil, status.Error(codes.NotFound, "message not found")
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, source.ChatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	originID, originSender := forwardAttribution(source)
	msgID, err := messageid.NewMessageID()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	saved, err := s.Messages.InsertMessage(ctx, store.MessageRow{
		ID:                msgID,
		ChatID:            targetChatID,
		SenderProfileID:   profileID,
		Content:           source.Content,
		Type:              "forward",
		ForwardFromID:     &originID,
		ForwardFromSender: originSender,
		AttachmentsJSON:   source.AttachmentsJSON,
		MentionsJSON:      "[]",
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.MessageEvents != nil {
		if err := s.MessageEvents.PublishMessageSent(ctx, saved.ID.String(), saved.ChatID.String(), saved.SenderProfileID.String()); err != nil {
			log.Printf("messaging: publish message.sent: %v", err)
		}
	}
	kind := messagingv1.MessageKind_MESSAGE_KIND_FORWARD
	return &messagingv1.ForwardMessageResponse{Message: messageRowToProto(saved, kind, "")}, nil
}

func (s *MessagingGRPC) AddReaction(ctx context.Context, req *messagingv1.AddReactionRequest) (*messagingv1.AddReactionResponse, error) {
	chatID, err := s.mutateReaction(ctx, req.GetMessageId(), req.GetEmoji(), true)
	if err != nil {
		return nil, err
	}
	_ = chatID
	return &messagingv1.AddReactionResponse{}, nil
}

func (s *MessagingGRPC) RemoveReaction(ctx context.Context, req *messagingv1.RemoveReactionRequest) (*messagingv1.RemoveReactionResponse, error) {
	chatID, err := s.mutateReaction(ctx, req.GetMessageId(), req.GetEmoji(), false)
	if err != nil {
		return nil, err
	}
	_ = chatID
	return &messagingv1.RemoveReactionResponse{}, nil
}

func (s *MessagingGRPC) mutateReaction(ctx context.Context, messageIDStr, emoji string, add bool) (uuid.UUID, error) {
	if s == nil || s.Messages == nil || s.Reactions == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	messageID, err := parseUUIDField("message_id", messageIDStr)
	if err != nil {
		return uuid.Nil, err
	}
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return uuid.Nil, status.Error(codes.InvalidArgument, "emoji is required")
	}

	msg, err := s.Messages.GetMessageByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, status.Error(codes.NotFound, "message not found")
		}
		return uuid.Nil, status.Error(codes.Internal, err.Error())
	}
	if msg.DeletedAt != nil {
		return uuid.Nil, status.Error(codes.NotFound, "message not found")
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, msg.ChatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return uuid.Nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return uuid.Nil, status.Error(codes.Internal, err.Error())
		}
	}

	if add {
		if err := s.Reactions.UpsertReaction(ctx, messageID, profileID, emoji); err != nil {
			return uuid.Nil, status.Error(codes.Internal, err.Error())
		}
		if s.MessageEvents != nil {
			if err := s.MessageEvents.PublishReactionAdded(ctx, messageID.String(), msg.ChatID.String(), profileID.String(), msg.SenderProfileID.String(), emoji); err != nil {
				log.Printf("messaging: publish reaction.added: %v", err)
			}
		}
	} else {
		if err := s.Reactions.DeleteReaction(ctx, messageID, profileID, emoji); err != nil {
			return uuid.Nil, status.Error(codes.Internal, err.Error())
		}
		if s.MessageEvents != nil {
			if err := s.MessageEvents.PublishReactionRemoved(ctx, messageID.String(), msg.ChatID.String(), profileID.String(), emoji); err != nil {
				log.Printf("messaging: publish reaction.removed: %v", err)
			}
		}
	}
	return msg.ChatID, nil
}

func forwardAttribution(source *store.MessageRow) (uuid.UUID, string) {
	if source.Type == "forward" && source.ForwardFromID != nil {
		sender := source.ForwardFromSender
		if sender == "" {
			sender = source.SenderProfileID.String()
		}
		return *source.ForwardFromID, sender
	}
	return source.ID, source.SenderProfileID.String()
}

func (s *MessagingGRPC) MarkRead(ctx context.Context, req *messagingv1.MarkReadRequest) (*messagingv1.MarkReadResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	if err := validateChatRefDM(req.GetChat()); err != nil {
		return nil, err
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	lastRead, err := parseUUIDField("last_read_message_id", req.GetLastReadMessageId())
	if err != nil {
		return nil, err
	}
	okMsg, err := s.Messages.MessageExists(ctx, chatID, lastRead)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !okMsg {
		return nil, status.Error(codes.NotFound, "message not found in chat")
	}
	if err := s.Messages.UpsertReadReceipt(ctx, chatID, profileID, lastRead); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.MessageEvents != nil {
		if err := s.MessageEvents.PublishMessageRead(ctx, lastRead.String(), chatID.String(), profileID.String()); err != nil {
			log.Printf("messaging: publish message.read chat=%s profile=%s: %v", chatID, profileID, err)
		}
	}
	return &messagingv1.MarkReadResponse{}, nil
}

func (s *MessagingGRPC) GetReadState(ctx context.Context, req *messagingv1.GetReadStateRequest) (*messagingv1.GetReadStateResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	if err := validateChatRefDM(req.GetChat()); err != nil {
		return nil, err
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	lid, upd, err := s.Messages.GetReadReceipt(ctx, chatID, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if lid == nil {
		return &messagingv1.GetReadStateResponse{}, nil
	}
	return &messagingv1.GetReadStateResponse{
		ReadState: &messagingv1.ReadState{
			Chat:              req.GetChat(),
			ProfileId:         profileID.String(),
			LastReadMessageId: lid.String(),
			UpdatedAt:         timestamppb.New(*upd),
		},
	}, nil
}

func (s *MessagingGRPC) GetBulkReadState(ctx context.Context, req *messagingv1.GetBulkReadStateRequest) (*messagingv1.GetBulkReadStateResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatIDs, err := s.authorizedChatIDs(ctx, profileID, req.GetChats())
	if err != nil {
		return nil, err
	}
	out := make(map[string]*messagingv1.ReadState, len(chatIDs))
	for _, chatID := range chatIDs {
		lid, upd, err := s.Messages.GetReadReceipt(ctx, chatID, profileID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if lid == nil {
			continue
		}
		dm := chatv1.ChatType_CHAT_TYPE_DM
		out[chatID.String()] = &messagingv1.ReadState{
			Chat:              &chatv1.ChatRef{Id: chatID.String(), Type: &dm},
			ProfileId:         profileID.String(),
			LastReadMessageId: lid.String(),
			UpdatedAt:         timestamppb.New(*upd),
		}
	}
	return &messagingv1.GetBulkReadStateResponse{ByChatId: out}, nil
}

func (s *MessagingGRPC) GetChatListMetadata(ctx context.Context, req *messagingv1.GetChatListMetadataRequest) (*messagingv1.GetChatListMetadataResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatIDs, err := s.authorizedChatIDs(ctx, profileID, req.GetChats())
	if err != nil {
		return nil, err
	}
	rows, err := s.Messages.GetChatListMetadata(ctx, profileID, chatIDs)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make(map[string]*messagingv1.ChatListMetadata, len(rows))
	for _, chatID := range chatIDs {
		row := rows[chatID]
		dm := chatv1.ChatType_CHAT_TYPE_DM
		item := &messagingv1.ChatListMetadata{
			Chat:        &chatv1.ChatRef{Id: chatID.String(), Type: &dm},
			UnreadCount: row.UnreadCount,
		}
		if row.LastMessagePreview != "" {
			preview := row.LastMessagePreview
			item.LastMessagePreview = &preview
		}
		if row.LastMessageAt != nil {
			item.LastMessageAt = timestamppb.New(*row.LastMessageAt)
		}
		out[chatID.String()] = item
	}
	return &messagingv1.GetChatListMetadataResponse{ByChatId: out}, nil
}

func (s *MessagingGRPC) authorizedChatIDs(ctx context.Context, profileID uuid.UUID, refs []*chatv1.ChatRef) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{}, len(refs))
	out := make([]uuid.UUID, 0, len(refs))
	for _, ref := range refs {
		if err := validateChatRefDM(ref); err != nil {
			return nil, err
		}
		chatID, err := parseUUIDField("chat.id", ref.GetId())
		if err != nil {
			return nil, err
		}
		if _, ok := seen[chatID]; ok {
			continue
		}
		if s.ChatGuard != nil {
			if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
				if errors.Is(err, store.ErrNotChatMember) {
					return nil, status.Error(codes.PermissionDenied, "not a chat member")
				}
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		seen[chatID] = struct{}{}
		out = append(out, chatID)
	}
	return out, nil
}

func validateChatRefDM(ref *chatv1.ChatRef) error {
	if ref == nil {
		return status.Error(codes.InvalidArgument, "chat is required")
	}
	if ref.GetType() != chatv1.ChatType_CHAT_TYPE_UNSPECIFIED && ref.GetType() != chatv1.ChatType_CHAT_TYPE_DM {
		return status.Error(codes.InvalidArgument, "only dm chats are supported")
	}
	return nil
}

func validateChatRefMessaging(ref *chatv1.ChatRef) error {
	if ref == nil {
		return status.Error(codes.InvalidArgument, "chat is required")
	}
	switch ref.GetType() {
	case chatv1.ChatType_CHAT_TYPE_UNSPECIFIED,
		chatv1.ChatType_CHAT_TYPE_DM,
		chatv1.ChatType_CHAT_TYPE_GROUP:
		return nil
	default:
		return status.Error(codes.InvalidArgument, "unsupported chat type")
	}
}

func messageKindToWire(k messagingv1.MessageKind) (typeStr string, out messagingv1.MessageKind) {
	if k == messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED {
		return "regular", messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	}
	switch k {
	case messagingv1.MessageKind_MESSAGE_KIND_REGULAR:
		return "regular", messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	case messagingv1.MessageKind_MESSAGE_KIND_SYSTEM:
		return "system", messagingv1.MessageKind_MESSAGE_KIND_SYSTEM
	case messagingv1.MessageKind_MESSAGE_KIND_FORWARD:
		return "forward", messagingv1.MessageKind_MESSAGE_KIND_FORWARD
	default:
		return "regular", messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	}
}

func messageRowToProto(m *store.MessageRow, kind messagingv1.MessageKind, reactionsJSON string) *messagingv1.Message {
	if m == nil {
		return nil
	}
	dm := chatv1.ChatType_CHAT_TYPE_DM
	out := &messagingv1.Message{
		Id:              m.ID.String(),
		Chat:            &chatv1.ChatRef{Id: m.ChatID.String(), Type: &dm},
		SenderProfileId: m.SenderProfileID.String(),
		PostedAsChat:    false,
		Content:         m.Content,
		Type:            m.Type,
		AttachmentsJson: m.AttachmentsJSON,
		MentionsJson:    m.MentionsJSON,
		ReactionsJson:   reactionsJSON,
		CreatedAt:       timestamppb.New(m.CreatedAt.UTC()),
	}
	if m.ThreadParentID != nil {
		out.ThreadParentId = ptrString(m.ThreadParentID.String())
	}
	if m.EditedAt != nil {
		out.EditedAt = timestamppb.New(m.EditedAt.UTC())
	}
	if m.DeletedAt != nil {
		out.DeletedAt = timestamppb.New(m.DeletedAt.UTC())
	}
	if m.ForwardFromID != nil {
		out.ForwardFromId = ptrString(m.ForwardFromID.String())
	}
	if m.ForwardFromSender != "" {
		out.ForwardFromSender = ptrString(m.ForwardFromSender)
	}
	if kind != messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED {
		k := kind
		out.MessageKind = &k
	} else {
		switch m.Type {
		case "system":
			k := messagingv1.MessageKind_MESSAGE_KIND_SYSTEM
			out.MessageKind = &k
		case "forward":
			k := messagingv1.MessageKind_MESSAGE_KIND_FORWARD
			out.MessageKind = &k
		default:
			k := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
			out.MessageKind = &k
		}
	}
	return out
}

func ptrString(s string) *string { return &s }
