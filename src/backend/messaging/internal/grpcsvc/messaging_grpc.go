package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/mentions"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/messaging/internal/messageid"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/privacy"
	"voice/backend/role/permissions"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	gameintegrationv1 "voice.app/voice/gameintegration/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

const (
	defaultPageSize            = 50
	maxPageSize                = 100
	fallbackSize               = 50
	historyBlockScanMultiplier = 4
)

// MessagingGRPC implements MessagingService (app stack: DM send, history, read receipts).
type MessagingGRPC struct {
	messagingv1.UnimplementedMessagingServiceServer
	Messages              *store.MessagesStore
	SpacePurgeReceipts    SpacePurgeReceiptLookup
	SpaceManifestImporter SpaceManifestPageImporter
	SpaceLifecycleFences  SpaceLifecycleFenceApplier
	SpaceLifecyclePurger  SpaceLifecyclePurgeStore
	SpaceFileProducer     *SpaceFileProducerCoordinator
	// GameMessages owns strict JWS/Auth assertion verification, T16 binding and
	// chat authorization, File provenance checks, then the receipt transaction.
	// Missing authority dependencies leave this deliberately nil and fail closed.
	GameMessages GameMessageProcessor
	// GameTombstones is populated only when Messaging's service-owned signing
	// key is configured; the RPC also requires a verified moderation principal.
	GameTombstones GameTombstoneProcessor
	// ManagedChatPurger owns the durable content work set and calls File/Search
	// owner APIs before Messaging physically removes message payloads.
	ManagedChatPurger ManagedChatPurgeProcessor
	Reactions         *store.ReactionsStore
	Pins              *store.PinsStore
	SharedMedia       *store.SharedMediaStore
	ChatGuard         ChatGuard
	// Blocks and UserProfiles are optional S2S gates for SendMessage (Social + User); both must be set to enforce.
	Blocks            AccountPairBlockChecker
	AccountBlocks     AccountBlockChecker
	UserProfiles      ProfileAccountLookup
	ProfilePairBlocks ProfilePairBlockChecker
	// DeletedAccounts is the Auth S2S gate for DM writes. It is deliberately
	// separate from other optional S2S policy checks: a missing dependency must
	// fail closed for DM sends and forwards.
	DeletedAccounts AccountDeletedChecker
	// ChatTypeResolver is required before SendMessage, ForwardMessage, and
	// GetMessages use DM-specific account lifecycle logic. It prevents an
	// omitted or forged ChatRef.type from bypassing that policy.
	ChatTypeResolver  AuthoritativeChatTypeResolver
	Privacy           PrivacyChecker
	Friends           ProfileFriendChecker
	SpaceCoMembership SpaceCoMembershipChecker
	// Files is optional for text-only messages and required for non-empty attachments_json.
	Files                FileMetadataLookup
	AttachmentReferences *AttachmentReferenceCoordinator
	// MessageEvents is optional; when set, successful send/edit/delete publishes to NATS JetStream (stream message_events, subjects message.*).
	MessageEvents messageevents.MessageEventsPublisher
	// Moderation is optional; enforces space timeouts and chat slow mode before send.
	Moderation *store.SQLModerationGuard
	// ChatMentionsMeta loads chat members for mention validation.
	ChatMentionsMeta *store.SQLChatMentionsMeta
	// RolePermissions checks TEXT_CHAT_MENTION_* in space chats.
	RolePermissions mentions.RolePermissionChecker
	// UserPresence resolves online members for @here.
	UserPresence mentions.OnlinePresenceLookup
	// ChatThreadPolicy loads thread settings from chat_db (roles/threads (docs/features/roles.md)).
	ChatThreadPolicy *store.SQLChatThreadPolicy
	// ChatRolePermissions checks TEXT_CHAT_*_THREADS in space chats.
	ChatRolePermissions ChatRolePermissions
	// PlatformMod optional platform moderation (shadow ban, spam mute).
	PlatformMod PlatformModerationChecker
	// PreKeyBundles optional Signal pre-key directory (encryption (docs/features/encryption.md) E2E).
	PreKeyBundles *store.E2EPreKeyStore
	// Logger emits structured nats_publish errors when JetStream publish fails after a successful RPC.
	Logger *slog.Logger
}

// ChatRolePermissions checks permissions scoped to a text chat in a space.
type ChatRolePermissions interface {
	HasChatPermission(ctx context.Context, spaceID, profileID, chatID uuid.UUID, permission string) (bool, error)
}

func (s *MessagingGRPC) threadPolicyDeps() threadPolicyDeps {
	return threadPolicyDeps{
		Policy:    s.ChatThreadPolicy,
		Messages:  s.Messages,
		RolePerms: s.ChatRolePermissions,
	}
}

func (s *MessagingGRPC) SendMessage(ctx context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	// P-008 establishes the wire contract first. Until the schedule handler slice
	// owns durable creation, reject a populated arm so it cannot silently take
	// the existing immediate insert/event path.
	if req != nil && req.DeliverySchedule != nil {
		return nil, status.Error(codes.Unimplemented, "scheduled delivery is not implemented")
	}
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
	if isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	} else {
		if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	chatType, err := s.resolveAuthoritativeChatType(ctx, chatID, profileID)
	if err != nil {
		return nil, err
	}
	if err := s.checkDeletedDMWrite(ctx, chatType, chatID, profileID); err != nil {
		return nil, err
	}
	if err := s.checkDMBlocksForSend(ctx, chatType, chatID, profileID); err != nil {
		return nil, err
	}
	if err := s.checkDMPrivacyForSend(ctx, chatType, chatID, profileID); err != nil {
		return nil, err
	}
	if err := s.checkSpaceSendPermission(ctx, chatID, profileID); err != nil {
		return nil, err
	}

	isE2E := req.IsE2E != nil && *req.IsE2E
	if err := s.validateE2ESend(ctx, chatID, isE2E); err != nil {
		return nil, err
	}

	var threadParent *uuid.UUID
	if req.GetThreadParentId() != "" {
		tid, err := parseUUIDField("thread_parent_id", req.GetThreadParentId())
		if err != nil {
			return nil, err
		}
		threadParent = &tid
	}

	postedAsChat := req.GetPostedAsChat()
	if err := s.threadPolicyDeps().validateSend(ctx, chatID, profileID, threadParent, postedAsChat); err != nil {
		return nil, err
	}

	if s.Moderation != nil {
		if err := s.Moderation.EnsureCanSend(ctx, chatID, profileID); err != nil {
			if errors.Is(err, store.ErrMemberTimedOut) {
				return nil, status.Error(codes.PermissionDenied, "member is timed out in this space")
			}
			if errors.Is(err, store.ErrSlowModeActive) {
				return nil, status.Error(codes.ResourceExhausted, "slow mode is active")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	attachments := strings.TrimSpace(req.GetAttachmentsJson())
	if attachments == "" {
		attachments = "[]"
	}
	content := strings.TrimSpace(req.GetContent())
	contentType := resolveSendContentType(req, content, attachments)
	attachmentCount, err := s.validateAttachments(ctx, chatID, attachments, contentType)
	if err != nil {
		return nil, err
	}
	if err := s.checkAttachmentPrivacyForSend(ctx, chatID, profileID, attachments); err != nil {
		return nil, err
	}
	if content == "" && attachmentCount == 0 {
		return nil, status.Error(codes.InvalidArgument, "content or attachments is required")
	}
	if len(content) > 4000 {
		return nil, status.Error(codes.InvalidArgument, "content exceeds 4000 characters")
	}
	var ghostOnly bool
	if s.PlatformMod != nil {
		accountID, ok := authctx.AccountID(ctx)
		if !ok && s.UserProfiles != nil {
			var lookupErr error
			accountID, lookupErr = s.UserProfiles.AccountIDByProfileID(ctx, profileID)
			ok = lookupErr == nil
		}
		if ok {
			if err := s.PlatformMod.CheckMessageAllowed(ctx, profileID, chatID, content); err != nil {
				return nil, status.Error(codes.PermissionDenied, err.Error())
			}
			banned, err := s.PlatformMod.IsShadowBanned(ctx, accountID)
			if err != nil {
				return nil, status.Error(codes.Unavailable, "moderation_unavailable")
			}
			ghostOnly = banned
		}
	}
	mentionsRaw := strings.TrimSpace(req.GetMentionsJson())
	if mentionsRaw == "" || mentionsRaw == "[]" {
		// Align with text-chat.md: bare @everyone/@here/@uuid in content without mentions_json.
		mentionsRaw = mentions.EntriesJSONFromContent(content)
	}
	var mentionTargets []uuid.UUID
	mentionsJSON := mentionsRaw
	if mentionsRaw != "[]" {
		meta, merr := s.loadChatMeta(ctx, chatID)
		if merr != nil {
			if errors.Is(merr, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, merr.Error())
		}
		normalized, targets, verr := mentions.Process(ctx, meta, profileID, mentionsRaw, s.RolePermissions, s.UserPresence)
		if verr != nil {
			return nil, verr
		}
		mentionsJSON = normalized
		mentionTargets = targets
	}
	typeStr, kind := messageKindToWire(req.GetMessageKind())

	chatTypeName := chatTypeName(chatType)
	var displayChatID *uuid.UUID
	if s.ChatThreadPolicy != nil {
		pol, perr := s.ChatThreadPolicy.Load(ctx, chatID)
		if errors.Is(perr, store.ErrChatNotFound) {
			return nil, status.Error(codes.NotFound, "chat not found")
		}
		if perr != nil {
			return nil, status.Error(codes.Internal, perr.Error())
		}
		if pol != nil {
			chatTypeName = pol.ChatType
			if postedAsChat {
				cid := chatID
				displayChatID = &cid
			}
		}
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
		ChatType:        chatTypeName,
		SenderProfileID: profileID,
		PostedAsChat:    postedAsChat,
		DisplayChatID:   displayChatID,
		Content:         content,
		Type:            typeStr,
		ThreadParentID:  threadParent,
		AttachmentsJSON: attachments,
		MentionsJSON:    mentionsJSON,
		ClientMessageID: clientID,
		GhostOnly:       ghostOnly,
		IsE2E:           isE2E,
		ContentType:     contentType,
		SendSilent:      req.GetSendSilent(),
	}
	var outboxEvents []messageevents.OutboxEvent
	if !ghostOnly {
		hasMentions := len(mentionTargets) > 0
		threadParentID := ""
		if row.ThreadParentID != nil {
			threadParentID = row.ThreadParentID.String()
		}
		event, eventErr := messageevents.NewMessageSentOutbox(row.ID.String(), row.ChatID.String(), row.SenderProfileID.String(), hasMentions, threadParentID, row.IsE2E, store.EffectiveContentType(row.ContentType, row.Content, row.AttachmentsJSON), row.SendSilent)
		if eventErr != nil {
			return nil, status.Error(codes.Internal, "message event could not be encoded")
		}
		outboxEvents = append(outboxEvents, event)
		if hasMentions {
			ids := make([]string, 0, len(mentionTargets))
			for _, pid := range mentionTargets {
				ids = append(ids, pid.String())
			}
			appID, envID := gameMessageEventScope(row.GameAppID, row.GameEnvironmentID)
			mentioned, mentionErr := messageevents.NewMentionAddedOutbox(row.ID.String(), row.ChatID.String(), row.SenderProfileID.String(), ids, row.SendSilent, appID, envID)
			if mentionErr != nil {
				return nil, status.Error(codes.Internal, "mention event could not be encoded")
			}
			outboxEvents = append(outboxEvents, mentioned)
		}
	}
	saved, err := s.insertMessageWithAttachments(ctx, row, outboxEvents)
	if err != nil {
		if status.Code(err) != codes.Unknown {
			return nil, err
		}
		if strings.Contains(err.Error(), "attachments_json") || strings.Contains(err.Error(), "mentions_json") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &messagingv1.SendMessageResponse{Message: messageRowToProto(saved, kind, "", false)}, nil
}

func (s *MessagingGRPC) loadChatMeta(ctx context.Context, chatID uuid.UUID) (mentions.ChatMeta, error) {
	if s.ChatMentionsMeta != nil {
		return s.ChatMentionsMeta.LoadChatMeta(ctx, chatID)
	}
	return mentions.ChatMeta{}, errors.New("chat mentions meta not configured")
}

func (s *MessagingGRPC) loadMutationChatPolicy(ctx context.Context, chatID uuid.UUID) (*store.ChatThreadPolicy, error) {
	if s == nil || s.ChatThreadPolicy == nil {
		return nil, status.Error(codes.Unavailable, "authoritative chat Space scope unavailable")
	}
	policy, err := s.ChatThreadPolicy.Load(ctx, chatID)
	if errors.Is(err, store.ErrChatNotFound) {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "authoritative chat Space scope unavailable")
	}
	if policy == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	return policy, nil
}

type messageAttachment struct {
	FileID     string   `json:"file_id"`
	Type       string   `json:"type"`
	URL        string   `json:"url,omitempty"`
	PreviewURL string   `json:"preview_url,omitempty"`
	Lat        *float64 `json:"lat,omitempty"`
	Lon        *float64 `json:"lon,omitempty"`
	PackID     string   `json:"pack_id,omitempty"`
	StickerID  string   `json:"sticker_id,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	ProviderID string   `json:"provider_id,omitempty"`
}

// attachmentTypeMatchesFileMeta maps composer attachment types to File Service metadata types.
func attachmentTypeMatchesFileMeta(attType, fileType string) bool {
	attType = strings.ToLower(strings.TrimSpace(attType))
	fileType = strings.ToLower(strings.TrimSpace(fileType))
	if fileType == "" {
		return true
	}
	if attType == fileType {
		return true
	}
	switch attType {
	case "voice_message":
		return fileType == "audio"
	case "gif", "video_note":
		return fileType == "video"
	case "sticker":
		return fileType == "image" || fileType == "video"
	case "music":
		return fileType == "audio"
	default:
		return false
	}
}

func (s *MessagingGRPC) validateAttachments(ctx context.Context, chatID uuid.UUID, raw, contentType string) (int, error) {
	var attachments []messageAttachment
	if err := json.Unmarshal([]byte(raw), &attachments); err != nil {
		return 0, status.Error(codes.InvalidArgument, "attachments_json must be a JSON array")
	}
	return s.validateParsedAttachments(ctx, &chatID, attachments, contentType)
}

// validateForwardAttachments validates copied attachment payloads with the same
// rich-content rules as SendMessage. File Service access is evaluated for the
// forwarding profile, but the source file must not be required to belong to
// the destination chat: forwarded media retains its source chat linkage.
func (s *MessagingGRPC) validateForwardAttachments(ctx context.Context, raw, contentType string) (int, error) {
	var attachments []messageAttachment
	if err := json.Unmarshal([]byte(raw), &attachments); err != nil {
		return 0, status.Error(codes.InvalidArgument, "attachments_json must be a JSON array")
	}
	return s.validateParsedAttachments(ctx, nil, attachments, contentType)
}

func (s *MessagingGRPC) validateParsedAttachments(ctx context.Context, chatID *uuid.UUID, attachments []messageAttachment, contentType string) (int, error) {
	if len(attachments) == 0 {
		return 0, nil
	}
	switch strings.TrimSpace(contentType) {
	case "location", "article":
		if err := validateRichPayloadAttachments(contentType, attachments); err != nil {
			return 0, err
		}
		return len(attachments), nil
	case "sticker", "gif", "music", "video_note":
		if err := validateFileBackedRichAttachments(contentType, attachments); err != nil {
			return 0, err
		}
		return s.validateFileBackedAttachments(ctx, chatID, attachments)
	}
	return s.validateFileBackedAttachments(ctx, chatID, attachments)
}

func (s *MessagingGRPC) validateFileBackedAttachments(ctx context.Context, chatID *uuid.UUID, attachments []messageAttachment) (int, error) {
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
		if chatID != nil && meta.GetChat().GetId() != chatID.String() {
			return 0, status.Error(codes.FailedPrecondition, "attachment file is not linked to chat")
		}
		switch meta.GetScanResult() {
		case "clean", "skipped":
		default:
			return 0, status.Error(codes.FailedPrecondition, "attachment file is not clean")
		}
		if !attachmentTypeMatchesFileMeta(strings.TrimSpace(att.Type), meta.GetFileType()) {
			return 0, status.Error(codes.InvalidArgument, "attachments.type does not match file metadata")
		}
	}
	return len(attachments), nil
}

func (s *MessagingGRPC) checkDMBlocksForSend(ctx context.Context, chatType chatv1.ChatType, chatID, profileID uuid.UUID) error {
	if chatType != chatv1.ChatType_CHAT_TYPE_DM {
		return nil
	}
	if s == nil || isNilDependency(s.Blocks) || isNilDependency(s.UserProfiles) || isNilDependency(s.ChatGuard) {
		return status.Error(codes.Unavailable, "dm block status unavailable")
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
		return status.Error(codes.Unavailable, "dm block status unavailable")
	}
	peerAcct, err := s.UserProfiles.AccountIDByProfileID(ctx, peer)
	if err != nil {
		return status.Error(codes.Unavailable, "dm block status unavailable")
	}
	blocked, err := s.Blocks.AccountPairBlocked(ctx, accountID, peerAcct)
	if err != nil {
		return status.Error(codes.Unavailable, "dm block status unavailable")
	}
	if blocked {
		return status.Error(codes.PermissionDenied, "cannot send messages between blocked accounts")
	}
	return nil
}

// checkDeletedDMWrite prevents new writes to a DM when either participant's
// Auth account is soft-deleted. Account status is intentionally checked after
// target membership, but before idempotency/store work and event publication.
// Group and channel requests never consult Auth through this gate.
func (s *MessagingGRPC) checkDeletedDMWrite(ctx context.Context, chatType chatv1.ChatType, chatID, senderProfileID uuid.UUID) error {
	if chatType != chatv1.ChatType_CHAT_TYPE_DM {
		return nil
	}
	if s == nil || isNilDependency(s.DeletedAccounts) || isNilDependency(s.UserProfiles) || isNilDependency(s.ChatGuard) {
		return status.Error(codes.Unavailable, "dm account status unavailable")
	}

	senderAccountID, ok := authctx.AccountID(ctx)
	if !ok || senderAccountID == uuid.Nil {
		return status.Error(codes.Unavailable, "dm account status unavailable")
	}
	peerProfileID, err := s.ChatGuard.DMOtherProfileID(ctx, chatID, senderProfileID)
	if err != nil || peerProfileID == uuid.Nil {
		return status.Error(codes.Unavailable, "dm account status unavailable")
	}
	peerAccountID, err := s.UserProfiles.AccountIDByProfileID(ctx, peerProfileID)
	if err != nil || peerAccountID == uuid.Nil {
		return status.Error(codes.Unavailable, "dm account status unavailable")
	}

	deleted, err := s.DeletedAccounts.DeletedAmong(ctx, []uuid.UUID{senderAccountID, peerAccountID})
	if err != nil {
		return status.Error(codes.Unavailable, "dm account status unavailable")
	}
	if deleted == nil {
		return status.Error(codes.Unavailable, "dm account status unavailable")
	}
	for accountID := range deleted {
		if accountID != senderAccountID && accountID != peerAccountID {
			return status.Error(codes.Unavailable, "dm account status unavailable")
		}
	}
	if len(deleted) > 0 {
		// Do not reveal which participant was deleted or whether it was sender
		// or peer. The product contract requires a privacy-safe generic denial.
		return status.Error(codes.PermissionDenied, "permission denied")
	}
	return nil
}

func (s *MessagingGRPC) resolveAuthoritativeChatType(ctx context.Context, chatID, profileID uuid.UUID) (chatv1.ChatType, error) {
	if s == nil || isNilDependency(s.ChatTypeResolver) {
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, status.Error(codes.Unavailable, "chat type unavailable")
	}
	chatType, err := s.ChatTypeResolver.ResolveChatType(ctx, chatID, profileID)
	if err != nil {
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, status.Error(codes.Unavailable, "chat type unavailable")
	}
	switch chatType {
	case chatv1.ChatType_CHAT_TYPE_DM, chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL:
		return chatType, nil
	default:
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, status.Error(codes.Unavailable, "chat type unavailable")
	}
}

func chatTypeName(chatType chatv1.ChatType) string {
	switch chatType {
	case chatv1.ChatType_CHAT_TYPE_GROUP:
		return "group"
	case chatv1.ChatType_CHAT_TYPE_CHANNEL:
		return "channel"
	default:
		return "dm"
	}
}

// Game event scope is copied only from the immutable app/environment fields
// already persisted on the source Message row by the GIS ingress path. Preserve
// a partial pair so downstream consent enforcement can fail closed.
func gameMessageEventScope(applicationID, environmentID *uuid.UUID) (string, string) {
	var appID, envID string
	if applicationID != nil {
		appID = applicationID.String()
	}
	if environmentID != nil {
		envID = environmentID.String()
	}
	return appID, envID
}

func isNilDependency(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (s *MessagingGRPC) checkSpaceSendPermission(ctx context.Context, chatID, profileID uuid.UUID) error {
	if s.RolePermissions == nil || s.ChatThreadPolicy == nil {
		return nil
	}
	pol, err := s.ChatThreadPolicy.Load(ctx, chatID)
	if errors.Is(err, store.ErrChatNotFound) {
		return status.Error(codes.NotFound, "chat not found")
	}
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if pol == nil || pol.SpaceID == nil {
		return nil
	}
	chatType := strings.TrimSpace(pol.ChatType)
	if chatType != "channel" && chatType != "group" {
		return nil
	}
	allowed, err := s.RolePermissions.HasChatPermission(ctx, *pol.SpaceID, profileID, chatID, permissions.TextChatSendMessages)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			return status.Error(codes.Unavailable, "role service unavailable")
		}
		return status.Error(codes.Internal, err.Error())
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "send messages not permitted in this chat")
	}
	return nil
}

func (s *MessagingGRPC) checkDMPrivacyForSend(ctx context.Context, chatType chatv1.ChatType, chatID, senderProfileID uuid.UUID) error {
	if chatType != chatv1.ChatType_CHAT_TYPE_DM {
		return nil
	}
	if s == nil || isNilDependency(s.Privacy) || isNilDependency(s.ChatGuard) {
		return status.Error(codes.Unavailable, "dm privacy unavailable")
	}
	recipientProfileID, err := s.ChatGuard.DMOtherProfileID(ctx, chatID, senderProfileID)
	if err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return status.Error(codes.PermissionDenied, "not a chat member")
		}
		return status.Error(codes.Unavailable, "dm privacy unavailable")
	}
	allowDMAudience, err := s.Privacy.AllowDMAudience(ctx, recipientProfileID)
	if err != nil {
		return status.Error(codes.Unavailable, "dm privacy unavailable")
	}
	matcher := privacy.Matcher{Social: s.Friends, Space: s.SpaceCoMembership}
	if err := privacy.CheckAllowed(matcher, ctx, recipientProfileID, senderProfileID, allowDMAudience, guestguard.IsGuest(ctx)); err != nil {
		if errors.Is(err, privacy.ErrDenied) {
			return status.Error(codes.PermissionDenied, "dm blocked by recipient privacy settings")
		}
		return status.Error(codes.Internal, err.Error())
	}
	if guestguard.IsGuest(ctx) {
		allowed, err := s.Privacy.AllowGuestDM(ctx, recipientProfileID)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if !allowed {
			return status.Error(codes.PermissionDenied, "guest dm blocked by recipient privacy settings")
		}
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
	if err := s.requireMessageReadEntitlement(ctx, row.ChatID, profileID, row.CreatedAt); err != nil {
		return nil, err
	}
	if err := s.validateE2EEdit(ctx, row.ChatID, row.IsE2E); err != nil {
		return nil, err
	}
	mentionsRaw := mentions.EntriesJSONFromContent(content)
	var mentionTargets []uuid.UUID
	mentionsJSON := mentionsRaw
	if mentionsRaw != "[]" {
		meta, merr := s.loadChatMeta(ctx, row.ChatID)
		if merr != nil {
			if errors.Is(merr, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, merr.Error())
		}
		normalized, targets, verr := mentions.Process(ctx, meta, profileID, mentionsRaw, s.RolePermissions, s.UserPresence)
		if verr != nil {
			return nil, verr
		}
		mentionsJSON = normalized
		mentionTargets = targets
	}
	var mentionsPtr *string
	if mentionsJSON != row.MentionsJSON {
		mentionsPtr = &mentionsJSON
	}
	policy, err := s.loadMutationChatPolicy(ctx, row.ChatID)
	if err != nil {
		return nil, err
	}
	event, err := messageevents.NewMessageEditedOutbox(msgID.String(), row.ChatID.String(), row.IsE2E)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	outboxEvents := []messageevents.OutboxEvent{event}
	if len(mentionTargets) > 0 && mentionsJSON != row.MentionsJSON {
		ids := make([]string, 0, len(mentionTargets))
		for _, pid := range mentionTargets {
			ids = append(ids, pid.String())
		}
		appID, envID := gameMessageEventScope(row.GameAppID, row.GameEnvironmentID)
		mentionEvent, err := messageevents.NewMentionAddedOutbox(msgID.String(), row.ChatID.String(), row.SenderProfileID.String(), ids, row.SendSilent, appID, envID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		outboxEvents = append(outboxEvents, mentionEvent)
	}
	var spaceID uuid.UUID
	if policy.SpaceID != nil {
		spaceID = *policy.SpaceID
	}
	updated, err := s.Messages.UpdateMessageContentAndMentionsWithOutbox(ctx, row.ChatID, spaceID, msgID, profileID, content, mentionsPtr, outboxEvents)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "message not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &messagingv1.EditMessageResponse{Message: messageRowToProto(updated, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, "", false)}, nil
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
		policy, err := s.loadMutationChatPolicy(ctx, row.ChatID)
		if err != nil {
			return nil, err
		}
		var spaceID uuid.UUID
		if policy.SpaceID != nil {
			spaceID = *policy.SpaceID
		}
		if err := s.Messages.HideMessageForProfileWithMutationFence(ctx, row.ChatID, spaceID, msgID, profileID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return &messagingv1.DeleteMessageResponse{}, nil
	}
	if row.SenderProfileID != profileID {
		return nil, status.Error(codes.PermissionDenied, "only the message author can delete for everyone")
	}
	policy, err := s.loadMutationChatPolicy(ctx, row.ChatID)
	if err != nil {
		return nil, err
	}
	deletedEvent, err := messageevents.NewMessageDeletedOutbox(msgID.String(), row.ChatID.String())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var spaceID uuid.UUID
	if policy.SpaceID != nil {
		spaceID = *policy.SpaceID
	}
	if err := s.Messages.SoftDeleteMessageWithOutbox(ctx, row.ChatID, spaceID, msgID, profileID, deletedEvent); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "message not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
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
	if err := validateChatRefMessaging(req.GetChat()); err != nil {
		return nil, err
	}
	if isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	}
	if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return nil, status.Error(codes.PermissionDenied, "not a chat member")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	chatType, err := s.resolveAuthoritativeChatType(ctx, chatID, profileID)
	if err != nil {
		return nil, err
	}

	dmPeerState, err := s.dmPeerStateForHistory(ctx, chatType, chatID, profileID)
	if err != nil {
		return nil, err
	}

	pageSize := int(req.GetPage().GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	beforeFromCursor, afterFromCursor, err := store.DecodeHistoryCursor(req.GetPage().GetCursor(), chatID)
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

	filterBlockedSenders := chatType == chatv1.ChatType_CHAT_TYPE_GROUP || chatType == chatv1.ChatType_CHAT_TYPE_CHANNEL
	scanLimit := limit
	if filterBlockedSenders {
		scanLimit = historyBlockScanLimit(limit)
	}
	rows, err := s.Messages.ListMessages(ctx, chatID, profileID, mode, refID, scanLimit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	hasMore := len(rows) > scanLimit
	if hasMore {
		rows = rows[:scanLimit]
	}
	cursorRows := rows
	if filterBlockedSenders {
		visibleRows, err := s.filterBlockedHistoryRows(ctx, profileID, rows)
		if err != nil {
			return nil, err
		}
		if len(visibleRows) > limit {
			hasMore = true
			visibleRows = visibleRows[:limit]
			cursorRows = visibleRows
		} else if !hasMore {
			cursorRows = visibleRows
		}
		rows = visibleRows
	}
	paginationRows := append([]store.MessageRow(nil), cursorRows...)
	visibleRows := rows[:0]
	for i := range rows {
		if err := s.requireMessageReadEntitlement(ctx, chatID, profileID, rows[i].CreatedAt); err != nil {
			if status.Code(err) == codes.NotFound {
				continue
			}
			return nil, err
		}
		visibleRows = append(visibleRows, rows[i])
	}
	rows = visibleRows

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
	pinnedSet := map[uuid.UUID]bool{}
	if s.Pins != nil && len(msgIDs) > 0 {
		pinnedSet, err = s.Pins.PinnedSetForMessageIDs(ctx, chatID, msgIDs)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	msgs := make([]*messagingv1.Message, 0, len(rows))
	for i := range rows {
		msgs = append(msgs, messageRowToProto(&rows[i], messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, reactionsByMsg[rows[i].ID], pinnedSet[rows[i].ID]))
	}

	next := ""
	if hasMore {
		next = nextCursorForPage(chatID, mode, paginationRows)
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
	return &messagingv1.GetMessagesResponse{MessageList: ml, DmPeerState: dmPeerState}, nil
}

func historyBlockScanLimit(pageLimit int) int {
	if pageLimit < 1 {
		pageLimit = defaultPageSize
	}
	limit := pageLimit * historyBlockScanMultiplier
	if limit > maxPageSize*historyBlockScanMultiplier {
		return maxPageSize * historyBlockScanMultiplier
	}
	return limit
}

func (s *MessagingGRPC) filterBlockedHistoryRows(ctx context.Context, viewerProfileID uuid.UUID, rows []store.MessageRow) ([]store.MessageRow, error) {
	if isNilDependency(s.ProfilePairBlocks) {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	blockedByProfile := make(map[uuid.UUID]bool)
	visible := make([]store.MessageRow, 0, len(rows))
	for i := range rows {
		senderProfileID := rows[i].SenderProfileID
		if senderProfileID == viewerProfileID {
			visible = append(visible, rows[i])
			continue
		}
		if senderProfileID == uuid.Nil {
			return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
		}
		blocked, ok := blockedByProfile[senderProfileID]
		if !ok {
			var err error
			blocked, err = s.ProfilePairBlocks.ProfilePairBlocked(ctx, viewerProfileID, senderProfileID)
			if err != nil {
				return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
			}
			blockedByProfile[senderProfileID] = blocked
		}
		if !blocked {
			visible = append(visible, rows[i])
		}
	}
	return visible, nil
}

// dmPeerStateForHistory resolves only the other participant's Auth state for a
// selected DM. It runs after membership and before history reads, so a failed
// dependency cannot leak a partial page. Group and channel history intentionally
// bypasses User and Auth and leaves the optional response field absent.
func (s *MessagingGRPC) dmPeerStateForHistory(ctx context.Context, chatType chatv1.ChatType, chatID, senderProfileID uuid.UUID) (*messagingv1.DmPeerState, error) {
	if chatType != chatv1.ChatType_CHAT_TYPE_DM {
		return nil, nil
	}
	if s == nil || isNilDependency(s.ChatGuard) || isNilDependency(s.UserProfiles) || isNilDependency(s.DeletedAccounts) {
		return nil, status.Error(codes.Unavailable, "dm account status unavailable")
	}

	peerProfileID, err := s.ChatGuard.DMOtherProfileID(ctx, chatID, senderProfileID)
	if err != nil || peerProfileID == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "dm account status unavailable")
	}
	senderAccountID, err := s.UserProfiles.AccountIDByProfileID(ctx, senderProfileID)
	if err != nil || senderAccountID == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "dm account status unavailable")
	}
	peerAccountID, err := s.UserProfiles.AccountIDByProfileID(ctx, peerProfileID)
	if err != nil || peerAccountID == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "dm account status unavailable")
	}

	deleted, err := s.DeletedAccounts.DeletedAmong(ctx, []uuid.UUID{senderAccountID, peerAccountID})
	if err != nil || deleted == nil {
		return nil, status.Error(codes.Unavailable, "dm account status unavailable")
	}
	for accountID := range deleted {
		if accountID != senderAccountID && accountID != peerAccountID {
			return nil, status.Error(codes.Unavailable, "dm account status unavailable")
		}
	}
	state := messagingv1.DmPeerState_DM_PEER_STATE_ACTIVE
	if _, deleted := deleted[peerAccountID]; deleted {
		state = messagingv1.DmPeerState_DM_PEER_STATE_DELETED
	}
	return &state, nil
}

func (s *MessagingGRPC) GetMessage(ctx context.Context, req *messagingv1.GetMessageRequest) (*messagingv1.GetMessageResponse, error) {
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
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
	if profileID, ok := authctx.ProfileID(ctx); ok {
		if s.ChatGuard != nil {
			if err := s.ChatGuard.EnsureMember(ctx, row.ChatID, profileID); err != nil {
				if errors.Is(err, store.ErrNotChatMember) {
					return nil, status.Error(codes.PermissionDenied, "not a chat member")
				}
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		if err := s.requireMessageReadEntitlement(ctx, row.ChatID, profileID, row.CreatedAt); err != nil {
			return nil, err
		}
		if row.GhostOnly && row.SenderProfileID != profileID {
			return nil, status.Error(codes.NotFound, "message not found")
		}
	}
	kind := messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED
	if row.Type == "forward" {
		kind = messagingv1.MessageKind_MESSAGE_KIND_FORWARD
	}
	return &messagingv1.GetMessageResponse{Message: messageRowToProto(row, kind, "", false)}, nil
}

// ResolveGameAction is a narrow GIS-to-Messaging authority read. GIS has
// already validated the player's live session; Messaging independently checks
// the profile's current chat membership before returning the immutable card.
func (s *MessagingGRPC) ResolveGameAction(ctx context.Context, req *messagingv1.ResolveGameActionRequest) (*messagingv1.ResolveGameActionResponse, error) {
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "gameintegration" || verified.Subject != "service:gameintegration" {
		return nil, status.Error(codes.PermissionDenied, "GIS service principal required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || verified.Audience != "messaging" || verified.RPC != messagingv1.MessagingService_ResolveGameAction_FullMethodName ||
		verified.RequestID == "" || verified.RequestHash != hash {
		return nil, status.Error(codes.Unauthenticated, "invalid principal binding")
	}
	if s == nil || s.Messages == nil || isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.Unavailable, "game action read unavailable")
	}
	messageID, err := parseUUIDField("message_id", req.GetMessageId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "game action unavailable")
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "game action unavailable")
	}
	row, err := s.Messages.GetMessageByID(ctx, messageID)
	if err != nil || row.DeletedAt != nil || row.GameCardJSON == nil || row.GameAppID == nil || row.GameEnvironmentID == nil ||
		row.GameInstallationID == nil || row.GameBotID == nil {
		return nil, status.Error(codes.NotFound, "game action unavailable")
	}
	if err := s.ChatGuard.EnsureMember(ctx, row.ChatID, profileID); err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return nil, status.Error(codes.NotFound, "game action unavailable")
		}
		return nil, status.Error(codes.Unavailable, "game action read unavailable")
	}
	return &messagingv1.ResolveGameActionResponse{Message: messageRowToProto(row, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, "", false)}, nil
}

func (s *MessagingGRPC) GetThreadMessages(ctx context.Context, req *messagingv1.GetThreadMessagesRequest) (*messagingv1.GetThreadMessagesResponse, error) {
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
	parentID, err := parseUUIDField("thread_parent_id", req.GetThreadParentId())
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
	parent, err := s.Messages.GetMessageByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "thread parent not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if parent.DeletedAt != nil || parent.ChatID != chatID || parent.ThreadParentID != nil {
		return nil, status.Error(codes.NotFound, "thread parent not found")
	}
	if err := s.requireMessageReadEntitlement(ctx, chatID, profileID, parent.CreatedAt); err != nil {
		return nil, err
	}

	pageSize := int(req.GetPage().GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	beforeFromCursor, afterFromCursor, err := store.DecodeHistoryCursor(req.GetPage().GetCursor(), chatID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page cursor")
	}

	var mode store.ListMode
	var refID *uuid.UUID
	switch {
	case beforeFromCursor != nil:
		mode = store.ListBeforeID
		refID = beforeFromCursor
	case afterFromCursor != nil:
		mode = store.ListAfterID
		refID = afterFromCursor
	default:
		mode = store.ListLatest
	}

	rows, err := s.Messages.ListThreadMessages(ctx, chatID, profileID, parentID, mode, refID, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	paginationRows := append([]store.MessageRow(nil), rows...)
	visibleRows := rows[:0]
	for i := range rows {
		if err := s.requireMessageReadEntitlement(ctx, chatID, profileID, rows[i].CreatedAt); err != nil {
			if status.Code(err) == codes.NotFound {
				continue
			}
			return nil, err
		}
		visibleRows = append(visibleRows, rows[i])
	}
	rows = visibleRows
	msgs := make([]*messagingv1.Message, 0, len(rows))
	for i := range rows {
		msgs = append(msgs, messageRowToProto(&rows[i], messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, "[]", false))
	}
	next := ""
	if hasMore {
		next = nextCursorForPage(chatID, mode, paginationRows)
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
	return &messagingv1.GetThreadMessagesResponse{MessageList: ml}, nil
}

func (s *MessagingGRPC) ListThreads(ctx context.Context, req *messagingv1.ListThreadsRequest) (*messagingv1.ListThreadsResponse, error) {
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
	if isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.FailedPrecondition, "chat membership not configured")
	}
	// Membership must precede cursor handling, including while the projection
	// is unavailable. Never turn an authorization denial into a fallback page.
	if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
		if errors.Is(err, store.ErrNotChatMember) || status.Code(err) == codes.PermissionDenied {
			return nil, status.Error(codes.PermissionDenied, "not a chat member")
		}
		if s.Logger != nil {
			s.Logger.ErrorContext(ctx, "thread list membership check failed", slog.String("error", err.Error()))
		}
		return nil, threadListUnavailable(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, threadListUnavailable(err)
	}
	// The accepted successor requires a READY per-viewer projection. Until
	// its durable membership/build contract is implemented, every projection
	// is missing. The legacy aggregate ignores hides/ghost visibility and is
	// unsafe even for page one; do not invoke it or restart a cursor chain.
	return nil, threadListUnavailable(nil)
}

func threadListUnavailable(cause error) error {
	code := codes.Unavailable
	switch {
	case errors.Is(cause, context.Canceled) || status.Code(cause) == codes.Canceled:
		code = codes.Canceled
	case errors.Is(cause, context.DeadlineExceeded) || status.Code(cause) == codes.DeadlineExceeded:
		code = codes.DeadlineExceeded
	}
	return status.Error(code, "thread list unavailable")
}

func nextCursorForPage(chatID uuid.UUID, mode store.ListMode, rows []store.MessageRow) string {
	if len(rows) == 0 {
		return ""
	}
	switch mode {
	case store.ListAfterID:
		return store.EncodeAfterCursor(chatID, rows[len(rows)-1].ID)
	default:
		return store.EncodeBeforeCursor(chatID, rows[len(rows)-1].ID)
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
	if isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	} else {
		if err := s.ChatGuard.EnsureMember(ctx, targetChatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	targetChatType, err := s.resolveAuthoritativeChatType(ctx, targetChatID, profileID)
	if err != nil {
		return nil, err
	}
	if err := s.checkDeletedDMWrite(ctx, targetChatType, targetChatID, profileID); err != nil {
		return nil, err
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
	if err := s.requireMessageReadEntitlement(ctx, source.ChatID, profileID, source.CreatedAt); err != nil {
		return nil, err
	}
	if source.GhostOnly {
		return nil, status.Error(codes.PermissionDenied, "cannot forward shadow-banned messages")
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, source.ChatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	attachments := strings.TrimSpace(source.AttachmentsJSON)
	if attachments == "" {
		attachments = "[]"
	}
	contentType := store.EffectiveContentType(source.ContentType, source.Content, attachments)
	if _, err := s.validateForwardAttachments(ctx, attachments, contentType); err != nil {
		return nil, err
	}

	withoutAttribution := req.GetWithoutAttribution()

	if err := s.checkDMBlocksForSend(ctx, targetChatType, targetChatID, profileID); err != nil {
		return nil, err
	}
	if err := s.checkDMPrivacyForSend(ctx, targetChatType, targetChatID, profileID); err != nil {
		return nil, err
	}
	// FW-03 copy-as-new stays available when allow_forward=false (screen-controls: Always).
	if !withoutAttribution {
		if err := s.checkForwardAuthorPrivacy(ctx, source, profileID); err != nil {
			return nil, err
		}
	}
	if err := s.checkSpaceSendPermission(ctx, targetChatID, profileID); err != nil {
		return nil, err
	}
	if err := s.validateE2ESend(ctx, targetChatID, source.IsE2E); err != nil {
		return nil, err
	}

	chatType := chatTypeName(targetChatType)
	postedAsChat := false
	if s.ChatThreadPolicy != nil {
		pol, perr := s.ChatThreadPolicy.Load(ctx, targetChatID)
		if errors.Is(perr, store.ErrChatNotFound) {
			return nil, status.Error(codes.NotFound, "chat not found")
		}
		if perr != nil {
			return nil, status.Error(codes.Internal, perr.Error())
		}
		if pol != nil {
			chatType = pol.ChatType
			if pol.ChatType == "channel" && !pol.AllowUserMainFeed {
				postedAsChat = true
			}
		}
	}
	if err := s.threadPolicyDeps().validateSend(ctx, targetChatID, profileID, nil, postedAsChat); err != nil {
		return nil, err
	}
	if s.Moderation != nil {
		if err := s.Moderation.EnsureCanSend(ctx, targetChatID, profileID); err != nil {
			if errors.Is(err, store.ErrMemberTimedOut) {
				return nil, status.Error(codes.PermissionDenied, "member is timed out in this space")
			}
			if errors.Is(err, store.ErrSlowModeActive) {
				return nil, status.Error(codes.ResourceExhausted, "slow mode is active")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if attachments != "[]" {
		if err := s.checkAttachmentPrivacyForSend(ctx, targetChatID, profileID, attachments); err != nil {
			return nil, err
		}
	}
	var ghostOnly bool
	if s.PlatformMod != nil {
		accountID, acctOK := authctx.AccountID(ctx)
		if !acctOK && s.UserProfiles != nil {
			var lookupErr error
			accountID, lookupErr = s.UserProfiles.AccountIDByProfileID(ctx, profileID)
			acctOK = lookupErr == nil
		}
		if acctOK {
			if err := s.PlatformMod.CheckMessageAllowed(ctx, profileID, targetChatID, source.Content); err != nil {
				return nil, status.Error(codes.PermissionDenied, err.Error())
			}
			banned, err := s.PlatformMod.IsShadowBanned(ctx, accountID)
			if err != nil {
				return nil, status.Error(codes.Unavailable, "moderation_unavailable")
			}
			ghostOnly = banned
		}
	}

	commentary := strings.TrimSpace(req.GetCommentary())
	if commentary != "" {
		if len(commentary) > 4000 {
			return nil, status.Error(codes.InvalidArgument, "commentary exceeds 4000 characters")
		}
		if err := s.insertForwardCommentary(ctx, targetChatID, profileID, chatType, commentary, ghostOnly); err != nil {
			return nil, err
		}
	}

	msgID, err := messageid.NewMessageID()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var displayChatID *uuid.UUID
	if postedAsChat {
		cid := targetChatID
		displayChatID = &cid
	}

	row := store.MessageRow{
		ID:              msgID,
		ChatID:          targetChatID,
		ChatType:        chatType,
		SenderProfileID: profileID,
		PostedAsChat:    postedAsChat,
		DisplayChatID:   displayChatID,
		Content:         source.Content,
		AttachmentsJSON: attachments,
		MentionsJSON:    "[]",
		GhostOnly:       ghostOnly,
		IsE2E:           source.IsE2E,
		ContentType:     contentType,
	}
	kind := messagingv1.MessageKind_MESSAGE_KIND_FORWARD
	if withoutAttribution {
		// FW-03: copy as new regular message — no Forwarded-from attribution.
		row.Type = "regular"
		kind = messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	} else {
		originID, originSender := forwardAttribution(source)
		row.Type = "forward"
		row.ForwardFromID = &originID
		row.ForwardFromSender = originSender
	}

	var outboxEvents []messageevents.OutboxEvent
	if !ghostOnly {
		sent, eventErr := messageevents.NewMessageSentOutbox(row.ID.String(), row.ChatID.String(), row.SenderProfileID.String(), false, "", row.IsE2E, store.EffectiveContentType(row.ContentType, row.Content, row.AttachmentsJSON), row.SendSilent)
		if eventErr != nil {
			return nil, status.Error(codes.Internal, "message event could not be encoded")
		}
		outboxEvents = append(outboxEvents, sent)
		if !withoutAttribution {
			forwarded, forwardErr := messageevents.NewMessageForwardedOutbox(row.ID.String(), source.ChatID.String(), row.ChatID.String(), profileID.String())
			if forwardErr != nil {
				return nil, status.Error(codes.Internal, "forward event could not be encoded")
			}
			outboxEvents = append(outboxEvents, forwarded)
		}
	}
	saved, err := s.insertMessageWithAttachments(ctx, row, outboxEvents)
	if err != nil {
		if status.Code(err) != codes.Unknown {
			return nil, err
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &messagingv1.ForwardMessageResponse{Message: messageRowToProto(saved, kind, "", false)}, nil
}

func (s *MessagingGRPC) insertForwardCommentary(ctx context.Context, chatID, profileID uuid.UUID, chatType, content string, ghostOnly bool) error {
	msgID, err := messageid.NewMessageID()
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	row := store.MessageRow{
		ID:              msgID,
		ChatID:          chatID,
		ChatType:        chatType,
		SenderProfileID: profileID,
		Content:         content,
		Type:            "regular",
		AttachmentsJSON: "[]",
		MentionsJSON:    "[]",
		GhostOnly:       ghostOnly,
		ContentType:     "text",
	}
	var events []messageevents.OutboxEvent
	if !ghostOnly {
		event, eventErr := messageevents.NewMessageSentOutbox(row.ID.String(), row.ChatID.String(), row.SenderProfileID.String(), false, "", false, "text", row.SendSilent)
		if eventErr != nil {
			return status.Error(codes.Internal, "message event could not be encoded")
		}
		events = append(events, event)
	}
	_, err = s.insertMessageWithAttachments(ctx, row, events)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
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

	policy, err := s.loadMutationChatPolicy(ctx, msg.ChatID)
	if err != nil {
		return uuid.Nil, err
	}
	var spaceID uuid.UUID
	if policy.SpaceID != nil {
		spaceID = *policy.SpaceID
	}
	var event messageevents.OutboxEvent
	if add {
		appID, envID := gameMessageEventScope(msg.GameAppID, msg.GameEnvironmentID)
		event, err = messageevents.NewReactionAddedOutbox(messageID.String(), msg.ChatID.String(), profileID.String(), msg.SenderProfileID.String(), emoji, appID, envID)
	} else {
		event, err = messageevents.NewReactionRemovedOutbox(messageID.String(), msg.ChatID.String(), profileID.String(), emoji)
	}
	if err != nil {
		return uuid.Nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.Reactions.MutateReactionWithOutbox(ctx, msg.ChatID, spaceID, messageID, profileID, emoji, add, event); err != nil {
		return uuid.Nil, status.Error(codes.Internal, err.Error())
	}
	return msg.ChatID, nil
}

func (s *MessagingGRPC) PinMessage(ctx context.Context, req *messagingv1.PinMessageRequest) (*messagingv1.PinMessageResponse, error) {
	if err := s.mutatePin(ctx, req.GetChat(), req.GetMessageId(), true); err != nil {
		return nil, err
	}
	return &messagingv1.PinMessageResponse{}, nil
}

func (s *MessagingGRPC) UnpinMessage(ctx context.Context, req *messagingv1.UnpinMessageRequest) (*messagingv1.UnpinMessageResponse, error) {
	if err := s.mutatePin(ctx, req.GetChat(), req.GetMessageId(), false); err != nil {
		return nil, err
	}
	return &messagingv1.UnpinMessageResponse{}, nil
}

func (s *MessagingGRPC) GetPinnedMessages(ctx context.Context, req *messagingv1.GetPinnedMessagesRequest) (*messagingv1.GetPinnedMessagesResponse, error) {
	if s == nil || s.Messages == nil || s.Pins == nil {
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
	pins, err := s.Pins.ListPins(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	msgs := make([]*messagingv1.Message, 0, len(pins))
	for _, pin := range pins {
		row, err := s.Messages.GetMessageByID(ctx, pin.MessageID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		if row.DeletedAt != nil || row.ChatID != chatID {
			continue
		}
		if err := s.requireMessageReadEntitlement(ctx, chatID, profileID, row.CreatedAt); err != nil {
			if status.Code(err) == codes.NotFound {
				continue
			}
			return nil, err
		}
		reactionsJSON := ""
		if s.Reactions != nil {
			byMsg, err := s.Reactions.ReactionsJSONByMessageIDs(ctx, []uuid.UUID{row.ID}, profileID)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			reactionsJSON = byMsg[row.ID]
		}
		msgs = append(msgs, messageRowToProto(row, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, reactionsJSON, true))
	}
	return &messagingv1.GetPinnedMessagesResponse{
		MessageList: &messagingv1.MessageList{Messages: msgs},
	}, nil
}

func (s *MessagingGRPC) mutatePin(ctx context.Context, chatRef *chatv1.ChatRef, messageIDStr string, pin bool) error {
	if s == nil || s.Messages == nil || s.Pins == nil {
		return status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat.id", chatRef.GetId())
	if err != nil {
		return err
	}
	if err := validateChatRefMessaging(chatRef); err != nil {
		return err
	}
	messageID, err := parseUUIDField("message_id", messageIDStr)
	if err != nil {
		return err
	}
	if s.ChatGuard != nil {
		if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
			if errors.Is(err, store.ErrNotChatMember) {
				return status.Error(codes.PermissionDenied, "not a chat member")
			}
			return status.Error(codes.Internal, err.Error())
		}
	}
	if err := s.checkCanPinMessage(ctx, chatID, profileID); err != nil {
		return err
	}
	msg, err := s.Messages.GetMessageByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "message not found")
		}
		return status.Error(codes.Internal, err.Error())
	}
	if msg.DeletedAt != nil || msg.ChatID != chatID {
		return status.Error(codes.NotFound, "message not found")
	}
	if pin {
		if err := s.requireMessageReadEntitlement(ctx, chatID, profileID, msg.CreatedAt); err != nil {
			return err
		}
	}
	policy, err := s.loadMutationChatPolicy(ctx, chatID)
	if err != nil {
		return err
	}
	var spaceID uuid.UUID
	if policy.SpaceID != nil {
		spaceID = *policy.SpaceID
	}
	var event messageevents.OutboxEvent
	if pin {
		event, err = messageevents.NewMessagePinnedOutbox(messageID.String(), chatID.String(), profileID.String())
	} else {
		event, err = messageevents.NewMessageUnpinnedOutbox(messageID.String(), chatID.String(), profileID.String())
	}
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if err := s.Pins.MutatePinWithOutbox(ctx, chatID, spaceID, messageID, profileID, pin, event); err != nil {
		if errors.Is(err, store.ErrPinLimitReached) {
			return status.Error(codes.ResourceExhausted, "pin limit reached")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func (s *MessagingGRPC) checkCanPinMessage(ctx context.Context, chatID, profileID uuid.UUID) error {
	meta, err := s.loadChatMeta(ctx, chatID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if meta.SpaceID != nil {
		if s.RolePermissions == nil {
			return status.Error(codes.FailedPrecondition, "role permissions not configured")
		}
		allowed, err := s.RolePermissions.HasChatPermission(ctx, *meta.SpaceID, profileID, chatID, permissions.TextChatPinMessages)
		if err != nil {
			if st, ok := status.FromError(err); ok {
				return st.Err()
			}
			return status.Error(codes.Internal, err.Error())
		}
		if !allowed {
			return status.Error(codes.PermissionDenied, "missing TEXT_CHAT_PIN_MESSAGES")
		}
		return nil
	}
	chatType := strings.TrimSpace(meta.ChatType)
	if chatType != "group" && chatType != "channel" {
		return nil
	}
	if s.ChatGuard == nil {
		return status.Error(codes.FailedPrecondition, "chat guard not configured")
	}
	role, err := s.ChatGuard.MemberRole(ctx, chatID, profileID)
	if err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return status.Error(codes.PermissionDenied, "not a chat member")
		}
		return status.Error(codes.Internal, err.Error())
	}
	switch role {
	case "owner", "admin":
		return nil
	default:
		return status.Error(codes.PermissionDenied, "missing TEXT_CHAT_PIN_MESSAGES")
	}
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

// checkForwardAuthorPrivacy enforces privacy.md allow_forward for FW-04.
// Evaluates the original author (re-forward attribution), not intermediate forwarders.
// Authors may always forward their own messages; unknown/deleted attribution fails open.
func (s *MessagingGRPC) checkForwardAuthorPrivacy(ctx context.Context, source *store.MessageRow, forwarderProfileID uuid.UUID) error {
	if s == nil || s.Privacy == nil || source == nil {
		return nil
	}
	authorID, ok := originalForwardAuthorID(source)
	if !ok {
		return nil
	}
	if authorID == forwarderProfileID {
		return nil
	}
	allowed, err := s.Privacy.AllowForward(ctx, authorID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "forwarding blocked by author privacy settings")
	}
	return nil
}

func originalForwardAuthorID(source *store.MessageRow) (uuid.UUID, bool) {
	if source.Type == "forward" {
		if id, err := uuid.Parse(strings.TrimSpace(source.ForwardFromSender)); err == nil {
			return id, true
		}
		return uuid.Nil, false
	}
	return source.SenderProfileID, true
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
	publishReceipt := s.shouldPublishReadReceipt(ctx, chatID, profileID)
	policy, err := s.loadMutationChatPolicy(ctx, chatID)
	if err != nil {
		return nil, err
	}
	var spaceID uuid.UUID
	if policy.SpaceID != nil {
		spaceID = *policy.SpaceID
	}
	var event *messageevents.OutboxEvent
	if publishReceipt {
		created, err := messageevents.NewMessageReadOutbox(lastRead.String(), chatID.String(), profileID.String())
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		event = &created
	}
	if err := s.Messages.UpsertReadStateAndOutbox(ctx, chatID, spaceID, profileID, lastRead, publishReceipt, event); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
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
	lid, upd, err := s.Messages.GetReadPosition(ctx, chatID, profileID)
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
	chatIDs, refByID, err := s.authorizedChatRefs(ctx, profileID, req.GetChats())
	if err != nil {
		return nil, err
	}
	out := make(map[string]*messagingv1.ReadState, len(chatIDs))
	for _, chatID := range chatIDs {
		lid, upd, err := s.Messages.GetReadPosition(ctx, chatID, profileID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if lid == nil {
			continue
		}
		ref := refByID[chatID]
		if ref == nil {
			ref = chatRefFromID(chatID, "dm")
		}
		out[chatID.String()] = &messagingv1.ReadState{
			Chat:              ref,
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
	chatIDs, refByID, err := s.authorizedChatRefs(ctx, profileID, req.GetChats())
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
		if row.LastMessageAt != nil {
			if err := s.requireMessageReadEntitlement(ctx, chatID, profileID, *row.LastMessageAt); err != nil {
				if status.Code(err) != codes.NotFound {
					return nil, err
				}
				row.LastMessagePreview = ""
				row.LastMessageAt = nil
				row.LastMessageContentType = ""
				row.LastMessageDeliveryState = ""
			}
		}
		// A previously published receipt must also disappear from the durable
		// list response after either DM participant opts out. Delivery remains.
		if row.LastMessageDeliveryState == "read" && !s.shouldPublishReadReceipt(ctx, chatID, profileID) {
			row.LastMessageDeliveryState = "delivered"
		}
		ref := refByID[chatID]
		if ref == nil {
			ref = chatRefFromID(chatID, "dm")
		}
		item := &messagingv1.ChatListMetadata{
			Chat:        ref,
			UnreadCount: row.UnreadCount,
		}
		if row.LastMessagePreview != "" {
			preview := row.LastMessagePreview
			item.LastMessagePreview = &preview
		}
		if row.LastMessageAt != nil {
			item.LastMessageAt = timestamppb.New(*row.LastMessageAt)
		}
		if row.LastMessageDeliveryState != "" {
			outgoing := row.LastMessageIsOutgoing
			item.LastMessageIsOutgoing = &outgoing
			state := mapLastMessageDeliveryState(row.LastMessageDeliveryState)
			item.LastMessageDeliveryState = &state
		}
		if ct := mapLastMessageContentType(row.LastMessageContentType); ct != messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_UNSPECIFIED {
			item.LastMessageContentType = &ct
		}
		out[chatID.String()] = item
	}
	return &messagingv1.GetChatListMetadataResponse{ByChatId: out}, nil
}

func (s *MessagingGRPC) authorizedChatRefs(ctx context.Context, profileID uuid.UUID, refs []*chatv1.ChatRef) ([]uuid.UUID, map[uuid.UUID]*chatv1.ChatRef, error) {
	seen := make(map[uuid.UUID]struct{}, len(refs))
	refByID := make(map[uuid.UUID]*chatv1.ChatRef, len(refs))
	out := make([]uuid.UUID, 0, len(refs))
	for _, ref := range refs {
		if err := validateChatRefMessaging(ref); err != nil {
			return nil, nil, err
		}
		chatID, err := parseUUIDField("chat.id", ref.GetId())
		if err != nil {
			return nil, nil, err
		}
		if _, ok := seen[chatID]; ok {
			continue
		}
		if s.ChatGuard != nil {
			if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
				if errors.Is(err, store.ErrNotChatMember) {
					return nil, nil, status.Error(codes.PermissionDenied, "not a chat member")
				}
				return nil, nil, status.Error(codes.Internal, err.Error())
			}
		}
		seen[chatID] = struct{}{}
		refByID[chatID] = ref
		out = append(out, chatID)
	}
	return out, refByID, nil
}

func chatRefFromID(chatID uuid.UUID, chatType string) *chatv1.ChatRef {
	ref := &chatv1.ChatRef{Id: chatID.String()}
	switch chatType {
	case "group":
		t := chatv1.ChatType_CHAT_TYPE_GROUP
		ref.Type = &t
	case "channel":
		t := chatv1.ChatType_CHAT_TYPE_CHANNEL
		ref.Type = &t
	default:
		t := chatv1.ChatType_CHAT_TYPE_DM
		ref.Type = &t
	}
	return ref
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
		chatv1.ChatType_CHAT_TYPE_GROUP,
		chatv1.ChatType_CHAT_TYPE_CHANNEL:
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

func messageRowToProto(m *store.MessageRow, kind messagingv1.MessageKind, reactionsJSON string, isPinned bool) *messagingv1.Message {
	if m == nil {
		return nil
	}
	chatType := chatv1.ChatType_CHAT_TYPE_DM
	switch m.ChatType {
	case "group":
		chatType = chatv1.ChatType_CHAT_TYPE_GROUP
	case "channel":
		chatType = chatv1.ChatType_CHAT_TYPE_CHANNEL
	}
	out := &messagingv1.Message{
		Id:              m.ID.String(),
		Chat:            &chatv1.ChatRef{Id: m.ChatID.String(), Type: &chatType},
		SenderProfileId: m.SenderProfileID.String(),
		PostedAsChat:    m.PostedAsChat,
		Content:         m.Content,
		Type:            m.Type,
		AttachmentsJson: m.AttachmentsJSON,
		MentionsJson:    m.MentionsJSON,
		ReactionsJson:   reactionsJSON,
		CreatedAt:       timestamppb.New(m.CreatedAt.UTC()),
		IsE2E:           m.IsE2E,
	}
	if isPinned {
		out.IsPinned = ptrBool(true)
	}
	if m.DisplayChatID != nil {
		out.DisplayChatId = ptrString(m.DisplayChatID.String())
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
	if m.GameCardJSON != nil && m.GameAppID != nil && m.GameEnvironmentID != nil && m.GameInstallationID != nil && m.GameBotID != nil {
		card := new(gameintegrationv1.GameCard)
		if err := protojson.Unmarshal([]byte(*m.GameCardJSON), card); err == nil {
			out.GameCard = card
			out.GameAppId = m.GameAppID.String()
			out.GameEnvironmentId = m.GameEnvironmentID.String()
			out.GameInstallationId = m.GameInstallationID.String()
			out.GameBotId = m.GameBotID.String()
			if m.GameCharacterBindingID != nil {
				out.GameCharacterBindingId = ptrString(m.GameCharacterBindingID.String())
			}
			actionsEnabled := m.GameCardActionsEnabled
			out.GameCardActionsEnabled = actionsEnabled
		}
	}
	for _, result := range m.GameActionResults {
		out.GameActionResults = append(out.GameActionResults, &messagingv1.GameActionResult{
			OperationId: result.OperationID.String(), ActionId: result.ActionID, ResultId: result.ResultID.String(),
			StateVersion: result.StateVersion, Status: result.Status, SafeSummary: result.SafeSummary,
			RecordedAt: timestamppb.New(result.RecordedAt.UTC()),
		})
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
	if out.GameCard != nil {
		cardKind := messagingv1.MessageKind_MESSAGE_KIND_GAME_CARD
		out.MessageKind = &cardKind
	}
	if ct := mapLastMessageContentType(store.EffectiveContentType(m.ContentType, m.Content, m.AttachmentsJSON)); ct != messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_UNSPECIFIED {
		out.ContentType = &ct
	}
	return out
}

func (s *MessagingGRPC) ListSharedMedia(ctx context.Context, req *messagingv1.ListSharedMediaRequest) (*messagingv1.ListSharedMediaResponse, error) {
	if s == nil || s.SharedMedia == nil {
		return nil, status.Error(codes.FailedPrecondition, "shared media not configured")
	}
	if isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.Unavailable, "chat history entitlement unavailable")
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
	if err := s.ChatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return nil, status.Error(codes.PermissionDenied, "not a chat member")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	kind, err := protoSharedMediaKind(req.GetKind())
	if err != nil {
		return nil, err
	}
	pageSize := defaultSharedMediaPageSize
	cursor := ""
	if page := req.GetPage(); page != nil {
		if page.GetPageSize() > 0 {
			pageSize = page.GetPageSize()
		}
		cursor = strings.TrimSpace(page.GetCursor())
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	rows, nextCursor, hasMore, err := s.SharedMedia.List(ctx, chatID, kind, cursor, pageSize)
	if err != nil {
		if strings.Contains(err.Error(), "invalid shared media cursor") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	visibleRows := rows[:0]
	for i := range rows {
		allowed, err := s.messageReadEntitled(ctx, chatID, profileID, rows[i].CreatedAt)
		if err != nil {
			return nil, err
		}
		if allowed {
			visibleRows = append(visibleRows, rows[i])
		}
	}
	rows = visibleRows

	fileIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.FileID != nil {
			fileIDs = append(fileIDs, row.FileID.String())
		}
	}
	metaByID := map[string]*filev1.FileMetadata{}
	if len(fileIDs) > 0 && s.Files != nil {
		resp, err := s.Files.GetBulkMetadata(ctx, &filev1.GetBulkMetadataRequest{FileIds: fileIDs})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		metaByID = resp.GetBulkFileMetadata().GetByFileId()
	}

	items := make([]*messagingv1.SharedMediaItem, 0, len(rows))
	for _, row := range rows {
		item := &messagingv1.SharedMediaItem{
			MessageId:       row.MessageID.String(),
			SenderProfileId: row.SenderProfileID.String(),
			CreatedAt:       timestamppb.New(row.CreatedAt),
			SortOrder:       row.SortOrder,
		}
		if row.FileID != nil {
			fid := row.FileID.String()
			item.FileId = &fid
			meta := metaByID[fid]
			if meta == nil {
				continue
			}
			if meta.GetStatus() != "ready" {
				continue
			}
			switch meta.GetScanResult() {
			case "clean", "skipped":
			default:
				continue
			}
			if meta.GetChat().GetId() != "" && meta.GetChat().GetId() != chatID.String() {
				continue
			}
			attType := row.AttachmentType
			item.AttachmentType = &attType
			if name := strings.TrimSpace(meta.GetOriginalName()); name != "" {
				item.OriginalName = &name
			}
			size := meta.GetSizeBytes()
			item.SizeBytes = &size
			if wire := strings.TrimSpace(row.E2EKeyWire); wire != "" {
				item.E2EKeyWire = &wire
			}
		} else if row.ExternalURL != "" {
			url := row.ExternalURL
			item.ExternalUrl = &url
			if title := strings.TrimSpace(row.Title); title != "" {
				item.Title = &title
			}
		} else {
			continue
		}
		items = append(items, item)
	}

	return &messagingv1.ListSharedMediaResponse{
		SharedMediaList: &messagingv1.SharedMediaList{
			Items:      items,
			NextCursor: nextCursor,
			HasMore:    hasMore,
			Page: &commonv1.CursorPageResponse{
				NextCursor: nextCursor,
				HasMore:    hasMore,
			},
		},
	}, nil
}

const defaultSharedMediaPageSize int32 = 20

func protoSharedMediaKind(k messagingv1.SharedMediaKind) (store.SharedMediaKind, error) {
	switch k {
	case messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_MEDIA:
		return store.SharedMediaKindMedia, nil
	case messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_FILES:
		return store.SharedMediaKindFiles, nil
	case messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_LINKS:
		return store.SharedMediaKindLinks, nil
	case messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_VOICE:
		return store.SharedMediaKindVoice, nil
	case messagingv1.SharedMediaKind_SHARED_MEDIA_KIND_STICKERS:
		return store.SharedMediaKindStickers, nil
	default:
		return 0, status.Error(codes.InvalidArgument, "kind is required")
	}
}

func mapLastMessageDeliveryState(state string) messagingv1.LastMessageDeliveryState {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "none":
		return messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_NONE
	case "sent":
		return messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_SENT
	case "delivered":
		return messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_DELIVERED
	case "read":
		return messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_READ
	default:
		return messagingv1.LastMessageDeliveryState_LAST_MESSAGE_DELIVERY_STATE_UNSPECIFIED
	}
}

func resolveSendContentType(req *messagingv1.SendMessageRequest, content, attachmentsJSON string) string {
	if req != nil && req.ContentType != nil {
		if stored := contentTypeProtoToStore(req.GetContentType()); stored != "" {
			return stored
		}
	}
	return store.EffectiveContentType("", content, attachmentsJSON)
}

func contentTypeProtoToStore(ct messagingv1.MessageContentType) string {
	switch ct {
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_TEXT:
		return "text"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_PHOTO:
		return "photo"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_VIDEO:
		return "video"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_DOCUMENT:
		return "document"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_VOICE:
		return "voice"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_STICKER:
		return "sticker"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_GIF:
		return "gif"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_ARTICLE:
		return "article"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_LOCATION:
		return "location"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_VIDEO_NOTE:
		return "video_note"
	case messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_MUSIC:
		return "music"
	default:
		return ""
	}
}

func validateRichPayloadAttachments(contentType string, attachments []messageAttachment) error {
	for _, att := range attachments {
		typ := strings.ToLower(strings.TrimSpace(att.Type))
		switch strings.TrimSpace(contentType) {
		case "location":
			if typ != "location" {
				return status.Error(codes.InvalidArgument, "attachments.type must be location")
			}
			if att.Lat == nil || att.Lon == nil {
				return status.Error(codes.InvalidArgument, "location attachments require lat and lon")
			}
			if *att.Lat < -90 || *att.Lat > 90 || *att.Lon < -180 || *att.Lon > 180 {
				return status.Error(codes.InvalidArgument, "location coordinates out of range")
			}
		case "article":
			if typ != "article" && typ != "link" {
				return status.Error(codes.InvalidArgument, "attachments.type must be article")
			}
			u := strings.TrimSpace(att.URL)
			if u == "" {
				return status.Error(codes.InvalidArgument, "article attachments require url")
			}
			if !strings.HasPrefix(strings.ToLower(u), "https://") {
				return status.Error(codes.InvalidArgument, "article url must be https")
			}
		default:
			return status.Error(codes.InvalidArgument, "unsupported rich content_type")
		}
	}
	return nil
}

func validateFileBackedRichAttachments(contentType string, attachments []messageAttachment) error {
	if len(attachments) != 1 {
		return status.Error(codes.InvalidArgument, "file-backed rich messages require exactly one attachment")
	}
	att := attachments[0]
	typ := strings.ToLower(strings.TrimSpace(att.Type))
	switch strings.TrimSpace(contentType) {
	case "sticker":
		if typ != "sticker" {
			return status.Error(codes.InvalidArgument, "attachments.type must be sticker")
		}
		if _, err := parseUUIDField("attachments.file_id", att.FileID); err != nil {
			return err
		}
		if _, err := parseUUIDField("attachments.pack_id", att.PackID); err != nil {
			return err
		}
		if _, err := parseUUIDField("attachments.sticker_id", att.StickerID); err != nil {
			return err
		}
	case "gif":
		if typ != "gif" {
			return status.Error(codes.InvalidArgument, "attachments.type must be gif")
		}
		if _, err := parseUUIDField("attachments.file_id", att.FileID); err != nil {
			return err
		}
	case "music":
		if typ != "music" {
			return status.Error(codes.InvalidArgument, "attachments.type must be music")
		}
		if _, err := parseUUIDField("attachments.file_id", att.FileID); err != nil {
			return err
		}
	case "video_note":
		if typ != "video_note" {
			return status.Error(codes.InvalidArgument, "attachments.type must be video_note")
		}
		if _, err := parseUUIDField("attachments.file_id", att.FileID); err != nil {
			return err
		}
	default:
		return status.Error(codes.InvalidArgument, "unsupported file-backed rich content_type")
	}
	return nil
}

func mapLastMessageContentType(state string) messagingv1.MessageContentType {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "text":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_TEXT
	case "photo":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_PHOTO
	case "video":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_VIDEO
	case "document":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_DOCUMENT
	case "voice":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_VOICE
	case "sticker":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_STICKER
	case "gif":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_GIF
	case "article":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_ARTICLE
	case "location":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_LOCATION
	case "video_note":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_VIDEO_NOTE
	case "music":
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_MUSIC
	default:
		return messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_UNSPECIFIED
	}
}

func ptrString(s string) *string { return &s }

func ptrBool(v bool) *bool { return &v }
