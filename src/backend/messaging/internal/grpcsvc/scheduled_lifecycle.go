package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/messageid"
	"voice/backend/messaging/internal/store"
)

type scheduleValidationKey struct{}

func scheduleValidationOnly(ctx context.Context) bool {
	value, _ := ctx.Value(scheduleValidationKey{}).(bool)
	return value
}

// Preserve all existing immediate fingerprints while binding both schedule arms
// to their exact instant. Server-time validation happens after retained replay.
func scheduledRequestFingerprint(req *messagingv1.SendMessageRequest, base string) (string, error) {
	if req.GetDeliverySchedule() == nil {
		return base, nil
	}
	var suffix string
	switch schedule := req.GetDeliverySchedule().(type) {
	case *messagingv1.SendMessageRequest_ScheduledAt:
		if schedule == nil || schedule.ScheduledAt == nil || schedule.ScheduledAt.CheckValid() != nil {
			return "", errors.New("invalid schedule")
		}
		suffix = "at/" + schedule.ScheduledAt.AsTime().UTC().Format(time.RFC3339Nano)
	case *messagingv1.SendMessageRequest_SendWhenOnline:
		if schedule == nil || !schedule.SendWhenOnline {
			return "", errors.New("invalid schedule")
		}
		suffix = "when_online"
	default:
		return "", errors.New("invalid schedule")
	}
	sum := sha256.Sum256([]byte("voice.scheduled.request.v1/" + base + "/" + suffix))
	return hex.EncodeToString(sum[:]), nil
}
func scheduledPayload(req *messagingv1.SendMessageRequest) *messagingv1.ScheduledMessagePayload {
	return &messagingv1.ScheduledMessagePayload{Content: req.Content, AttachmentsJson: req.AttachmentsJson, MentionsJson: req.MentionsJson, ThreadParentId: req.ThreadParentId, MessageKind: req.MessageKind, PostedAsChat: req.PostedAsChat, IsE2E: req.IsE2E, ContentType: req.ContentType, SendSilent: req.SendSilent}
}
func scheduledPayloadRequest(chat uuid.UUID, p *messagingv1.ScheduledMessagePayload) *messagingv1.SendMessageRequest {
	return &messagingv1.SendMessageRequest{Chat: &chatv1.ChatRef{Id: chat.String()}, Content: p.Content, AttachmentsJson: p.AttachmentsJson, MentionsJson: p.MentionsJson, ThreadParentId: p.ThreadParentId, MessageKind: p.MessageKind, PostedAsChat: p.PostedAsChat, IsE2E: p.IsE2E, ContentType: p.ContentType, SendSilent: p.SendSilent}
}
func scheduledRPCError(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return status.Error(codes.NotFound, "scheduled message not found")
	case errors.Is(err, store.ErrScheduledState):
		return status.Error(codes.FailedPrecondition, "scheduled message is no longer pending")
	case store.IsSendNamespaceConflict(err):
		return status.Error(codes.AlreadyExists, "client message key has a different request")
	default:
		return status.Error(codes.Unavailable, "scheduled message persistence unavailable")
	}
}
func scheduledToProto(r *store.ScheduledRecord) (*messagingv1.ScheduledMessage, error) {
	payload := &messagingv1.ScheduledMessagePayload{}
	if err := protojson.Unmarshal([]byte(r.PayloadJSON), payload); err != nil {
		return nil, status.Error(codes.Internal, "invalid stored schedule payload")
	}
	states := map[store.ScheduledMessageStatus]messagingv1.ScheduledMessageStatus{store.ScheduledMessagePending: messagingv1.ScheduledMessageStatus_SCHEDULED_MESSAGE_STATUS_PENDING, store.ScheduledMessageSent: messagingv1.ScheduledMessageStatus_SCHEDULED_MESSAGE_STATUS_SENT, store.ScheduledMessageCancelled: messagingv1.ScheduledMessageStatus_SCHEDULED_MESSAGE_STATUS_CANCELLED, store.ScheduledMessageFailed: messagingv1.ScheduledMessageStatus_SCHEDULED_MESSAGE_STATUS_FAILED}
	out := &messagingv1.ScheduledMessage{Id: r.ID.String(), Chat: &chatv1.ChatRef{Id: r.ChatID.String()}, SenderProfileId: r.SenderProfileID.String(), Payload: payload, Status: states[r.Status], CreatedAt: timestamppb.New(r.CreatedAt), UpdatedAt: timestamppb.New(r.UpdatedAt)}
	if r.ClientMessageID != nil {
		out.ClientMessageId = ptrString(r.ClientMessageID.String())
	}
	if r.SentMessageID != nil {
		out.SentMessageId = ptrString(r.SentMessageID.String())
	}
	if r.ScheduleKind == store.ScheduledMessageScheduleAt && r.ScheduledAt != nil {
		out.DeliverySchedule = &messagingv1.ScheduledMessage_ScheduledAt{ScheduledAt: timestamppb.New(*r.ScheduledAt)}
	} else {
		out.DeliverySchedule = &messagingv1.ScheduledMessage_SendWhenOnline{SendWhenOnline: true}
	}
	return out, nil
}
func scheduledSendResponse(r *store.ScheduledRecord) (*messagingv1.SendMessageResponse, error) {
	value, err := scheduledToProto(r)
	if err != nil {
		return nil, err
	}
	return &messagingv1.SendMessageResponse{ScheduledMessage: value}, nil
}
func (s *MessagingGRPC) createScheduledMessage(ctx context.Context, req *messagingv1.SendMessageRequest, chat, owner uuid.UUID, kind chatv1.ChatType, rpc, hash string, key *uuid.UUID) (*messagingv1.SendMessageResponse, error) {
	schedule, at, err := validateScheduledDelivery(req, kind, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	raw, err := protojson.Marshal(scheduledPayload(req))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid scheduled payload")
	}
	policy, err := s.loadMutationChatPolicy(ctx, chat)
	if err != nil {
		return nil, err
	}
	row, err := s.Messages.CreateScheduled(ctx, store.ScheduledRecord{ScheduledMessageRow: store.ScheduledMessageRow{ID: uuid.New(), ChatID: chat, SenderProfileID: owner, ClientMessageID: key, PayloadJSON: string(raw), ScheduleKind: schedule, ScheduledAt: at}, RequestRPC: rpc, RequestHash: hash}, policy.SpaceID)
	if err != nil {
		return nil, scheduledRPCError(err)
	}
	return scheduledSendResponse(row)
}

func (s *MessagingGRPC) ownedSchedule(ctx context.Context, raw string) (*store.ScheduledRecord, error) {
	owner, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	id, err := parseUUIDField("scheduled_message_id", raw)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Messages == nil || s.Messages.Pool == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	row, err := s.Messages.GetOwnedScheduled(ctx, id, owner)
	if err != nil {
		return nil, scheduledRPCError(err)
	}
	return row, nil
}

type scheduledCursor struct {
	Chat, Owner uuid.UUID
	Position    store.ScheduledPosition
}

func (s *MessagingGRPC) ListScheduledMessages(ctx context.Context, req *messagingv1.ListScheduledMessagesRequest) (*messagingv1.ListScheduledMessagesResponse, error) {
	owner, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chat, err := parseUUIDField("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	if err = validateChatRefMessaging(req.GetChat()); err != nil {
		return nil, err
	}
	if s == nil || s.Messages == nil || s.Messages.Pool == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	if isNilDependency(s.ChatGuard) {
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	}
	if _, err = s.resolveAuthoritativeChatType(ctx, chat, owner); err != nil {
		return nil, err
	}
	if err = s.ChatGuard.EnsureMember(ctx, chat, owner); err != nil {
		if errors.Is(err, store.ErrNotChatMember) {
			return nil, status.Error(codes.PermissionDenied, "not a chat member")
		}
		return nil, status.Error(codes.Unavailable, "chat membership unavailable")
	}
	limit := int(req.GetPage().GetPageSize())
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, status.Error(codes.InvalidArgument, "page_size must be between 1 and 100")
	}
	var position *store.ScheduledPosition
	if raw := req.GetPage().GetCursor(); raw != "" {
		bytes, decodeErr := base64.RawURLEncoding.DecodeString(raw)
		var cursor scheduledCursor
		if decodeErr != nil || len(bytes) > 1024 || json.Unmarshal(bytes, &cursor) != nil || cursor.Chat != chat || cursor.Owner != owner || cursor.Position.ID == uuid.Nil || cursor.Position.CreatedAt.IsZero() {
			return nil, status.Error(codes.InvalidArgument, "invalid scheduled cursor")
		}
		position = &cursor.Position
	}
	rows, err := s.Messages.ListPendingScheduled(ctx, chat, owner, position, limit+1)
	if err != nil {
		return nil, scheduledRPCError(err)
	}
	out := &messagingv1.ListScheduledMessagesResponse{Page: &commonv1.CursorPageResponse{}}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	for i := range rows {
		value, err := scheduledToProto(&rows[i])
		if err != nil {
			return nil, err
		}
		out.ScheduledMessages = append(out.ScheduledMessages, value)
	}
	out.Page.HasMore = hasMore
	if hasMore {
		last := rows[len(rows)-1]
		raw, _ := json.Marshal(scheduledCursor{chat, owner, store.ScheduledPosition{CreatedAt: last.CreatedAt, ID: last.ID}})
		out.Page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, nil
}
func (s *MessagingGRPC) UpdateScheduledMessage(ctx context.Context, req *messagingv1.UpdateScheduledMessageRequest) (*messagingv1.UpdateScheduledMessageResponse, error) {
	row, err := s.ownedSchedule(ctx, req.GetScheduledMessageId())
	if err != nil {
		return nil, err
	}
	if row.Status != store.ScheduledMessagePending {
		return nil, scheduledRPCError(store.ErrScheduledState)
	}
	if req.Payload == nil && req.DeliverySchedule == nil {
		return nil, status.Error(codes.InvalidArgument, "payload or schedule is required")
	}
	stored := &messagingv1.ScheduledMessagePayload{}
	if protojson.Unmarshal([]byte(row.PayloadJSON), stored) != nil {
		return nil, status.Error(codes.Internal, "invalid stored schedule payload")
	}
	if req.Payload != nil {
		stored = req.Payload
	}
	validate := scheduledPayloadRequest(row.ChatID, stored)
	if _, err = s.sendMessage(context.WithValue(ctx, scheduleValidationKey{}, true), validate, nil); err != nil {
		return nil, err
	}
	if req.DeliverySchedule != nil {
		switch schedule := req.DeliverySchedule.(type) {
		case *messagingv1.UpdateScheduledMessageRequest_ScheduledAt:
			validate.DeliverySchedule = &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: schedule.ScheduledAt}
		case *messagingv1.UpdateScheduledMessageRequest_SendWhenOnline:
			validate.DeliverySchedule = &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: schedule.SendWhenOnline}
		}
		kind, err := s.resolveAuthoritativeChatType(ctx, row.ChatID, row.SenderProfileID)
		if err != nil {
			return nil, err
		}
		row.ScheduleKind, row.ScheduledAt, err = validateScheduledDelivery(validate, kind, time.Now().UTC())
		if err != nil {
			return nil, err
		}
	}
	raw, err := protojson.Marshal(stored)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid scheduled payload")
	}
	row.PayloadJSON = string(raw)
	updated, err := s.Messages.UpdateScheduled(ctx, *row)
	if err != nil {
		return nil, scheduledRPCError(err)
	}
	out, err := scheduledToProto(updated)
	return &messagingv1.UpdateScheduledMessageResponse{ScheduledMessage: out}, err
}
func (s *MessagingGRPC) CancelScheduledMessage(ctx context.Context, req *messagingv1.CancelScheduledMessageRequest) (*messagingv1.CancelScheduledMessageResponse, error) {
	row, err := s.ownedSchedule(ctx, req.GetScheduledMessageId())
	if err != nil {
		return nil, err
	}
	if err = s.Messages.CancelScheduled(ctx, row.ID, row.SenderProfileID); err != nil {
		return nil, scheduledRPCError(err)
	}
	return &messagingv1.CancelScheduledMessageResponse{}, nil
}
func (s *MessagingGRPC) SendScheduledMessageNow(ctx context.Context, req *messagingv1.SendScheduledMessageNowRequest) (*messagingv1.SendScheduledMessageNowResponse, error) {
	row, err := s.ownedSchedule(ctx, req.GetScheduledMessageId())
	if err != nil {
		return nil, err
	}
	if row.Status == store.ScheduledMessageSent && row.SentMessageID != nil {
		message, err := s.GetMessage(ctx, &messagingv1.GetMessageRequest{MessageId: row.SentMessageID.String()})
		if err != nil {
			return nil, err
		}
		return &messagingv1.SendScheduledMessageNowResponse{Message: message.Message}, nil
	}
	if row.Status != store.ScheduledMessagePending {
		return nil, scheduledRPCError(store.ErrScheduledState)
	}
	payload := &messagingv1.ScheduledMessagePayload{}
	if protojson.Unmarshal([]byte(row.PayloadJSON), payload) != nil {
		return nil, status.Error(codes.Internal, "invalid stored schedule payload")
	}
	if row.DispatchMessageID == uuid.Nil {
		id, idErr := messageid.NewMessageID()
		if idErr != nil {
			return nil, status.Error(codes.Internal, "message identity unavailable")
		}
		row, err = s.Messages.ReserveScheduledDispatch(ctx, *row, id)
		if err != nil {
			if errors.Is(err, store.ErrScheduledState) {
				current, loadErr := s.Messages.GetOwnedScheduled(ctx, uuid.MustParse(req.GetScheduledMessageId()), rowOwner(ctx))
				if loadErr == nil && current.Status == store.ScheduledMessageSent {
					return s.SendScheduledMessageNow(ctx, req)
				}
			}
			return nil, scheduledRPCError(err)
		}
	}
	sent, err := s.sendMessage(store.WithScheduledDispatch(ctx, *row), scheduledPayloadRequest(row.ChatID, payload), nil)
	if err != nil {
		// A concurrent exact send-now may already have committed the one message.
		if status.Code(err) == codes.FailedPrecondition {
			current, loadErr := s.Messages.GetOwnedScheduled(ctx, row.ID, row.SenderProfileID)
			if loadErr == nil && current.Status == store.ScheduledMessageSent {
				return s.SendScheduledMessageNow(ctx, req)
			}
		}
		return nil, err
	}
	return &messagingv1.SendScheduledMessageNowResponse{Message: sent.Message}, nil
}

func rowOwner(ctx context.Context) uuid.UUID { owner, _ := authctx.ProfileID(ctx); return owner }
