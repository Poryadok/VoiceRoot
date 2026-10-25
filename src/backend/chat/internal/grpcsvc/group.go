package grpcsvc

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

func (s *ChatGRPC) CreateChat(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	chatType := req.GetType()
	if chatType != chatv1.ChatType_CHAT_TYPE_GROUP && chatType != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return nil, status.Error(codes.InvalidArgument, "only group and channel chats are supported")
	}
	initiator, err := chatcreation.Creator(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified Chat create initiator required")
	}
	caller := initiator.ProfileID
	spaceIDRaw := strings.TrimSpace(req.GetSpaceId())
	// Guests may participate only after a regular member explicitly invites them
	// to a standalone chat that opted in. Space creation remains owned by the
	// existing Space and Role policy path.
	if spaceIDRaw == "" {
		if initiator.User == nil || initiator.AccountType != "regular" {
			return nil, status.Error(codes.PermissionDenied, "regular account required")
		}
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	var topic *string
	if req.Topic != nil {
		t := strings.TrimSpace(req.GetTopic())
		topic = &t
	}

	var row *store.ChatRow
	replayed := false
	requestID, err := parseCreateRequestID(req.GetRequestId())
	if err != nil {
		return nil, err
	}
	if spaceIDRaw != "" {
		if s.Composite == nil {
			return nil, status.Error(codes.Unavailable, "protected Space Chat creation unavailable")
		}
		return s.Composite.Create(ctx, req)
	}
	if requestID != nil {
		kind := "group"
		if chatType == chatv1.ChatType_CHAT_TYPE_CHANNEL {
			kind = "channel"
		}
		row, replayed, err = s.DM.CreateChatWithRequestID(ctx, store.ChatCreateRequest{
			CreatorProfileID: caller,
			RequestID:        *requestID,
			Type:             kind,
			Name:             name,
			Topic:            topic,
		})
		if errors.Is(err, store.ErrChatCreateRequestConflict) {
			return nil, status.Error(codes.AlreadyExists, "request_id already used for a different chat create")
		}
	} else if chatType == chatv1.ChatType_CHAT_TYPE_CHANNEL {
		row, err = s.DM.CreateChannelChat(ctx, caller, name, topic)
	} else {
		row, err = s.DM.CreateGroupChat(ctx, caller, name, topic)
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil && !replayed {
		eventType := "group"
		if row.Type == "channel" {
			eventType = "channel"
		}
		if err := s.ChatEvents.PublishChatCreated(ctx, row.ID.String(), eventType); err != nil {
			s.logPublishError(ctx, "chat.created", err, slog.String("chat_id", row.ID.String()))
		}
		if row.Type == "group" && row.SpaceID == nil {
			if err := s.ChatEvents.PublishChatMemberChanged(ctx, row.ID.String(), caller.String(), "joined"); err != nil {
				s.logPublishError(ctx, "chat.member_changed", err, slog.String("chat_id", row.ID.String()), slog.String("profile_id", caller.String()))
			}
		}
	}
	return &chatv1.CreateChatResponse{Chat: chatRowToProto(row)}, nil
}

func (s *ChatGRPC) UpdateChat(ctx context.Context, req *chatv1.UpdateChatRequest) (*chatv1.UpdateChatResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err := s.ensureMatchSquadMutable(ctx, chatID); err != nil {
		return nil, err
	}
	if row.Type != "group" && row.Type != "channel" {
		return nil, status.Error(codes.InvalidArgument, "only group and channel chats can be updated")
	}
	if req.MemberAdditionsEnabled != nil && (row.SpaceID != nil || row.ManagedByApplicationID != nil) {
		return nil, status.Error(codes.FailedPrecondition, "ordinary chat permissions unavailable for this scope")
	}
	role, err := s.DM.GetMemberRole(ctx, chatID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var name, avatar, topic *string
	var slowMode *int32
	var threadsEnabled, allowUserMainFeed, allowGuests *bool
	if req.Name != nil {
		n := strings.TrimSpace(req.GetName())
		name = &n
	}
	if req.AvatarUrl != nil {
		a := strings.TrimSpace(req.GetAvatarUrl())
		avatar = &a
	}
	if req.Topic != nil {
		t := strings.TrimSpace(req.GetTopic())
		topic = &t
	}
	if req.SlowModeSeconds != nil {
		slow := req.GetSlowModeSeconds()
		slowMode = &slow
	}
	if req.ThreadsEnabled != nil {
		v := req.GetThreadsEnabled()
		threadsEnabled = &v
	}
	if req.AllowUserMainFeed != nil {
		v := req.GetAllowUserMainFeed()
		allowUserMainFeed = &v
	}
	if req.AllowGuests != nil {
		v := req.GetAllowGuests()
		allowGuests = &v
	}
	changedFields := updateChatChangedFields(req)

	slowModeOnly := slowMode != nil && name == nil && avatar == nil && topic == nil && threadsEnabled == nil && allowUserMainFeed == nil && allowGuests == nil && req.MemberAdditionsEnabled == nil
	if row.SpaceID != nil && slowModeOnly && s.Roles != nil {
		if err := canSetSpaceChatSlowMode(ctx, s.Roles, *row.SpaceID, caller); err != nil {
			return nil, err
		}
	} else if row.SpaceID == nil && role != "owner" && role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "only a standalone group owner or admin can update the chat")
	} else if row.SpaceID != nil && role != "owner" {
		return nil, status.Error(codes.PermissionDenied, "only the chat owner can update the chat")
	}

	if row.SpaceID != nil && allowGuests != nil {
		return nil, status.Error(codes.FailedPrecondition, "space chat guest admission is owned by Space and Role services")
	}
	if row.SpaceID != nil && req.MemberAdditionsEnabled != nil {
		return nil, status.Error(codes.FailedPrecondition, "space invitations are owned by Space and Role services")
	}
	var updated *store.ChatRow
	if req.MemberAdditionsEnabled == nil {
		updated, err = s.DM.UpdateGroupChat(ctx, chatID, name, avatar, topic, slowMode, threadsEnabled, allowUserMainFeed, allowGuests)
	} else {
		permissionStore, permissionErr := s.permissionStore()
		if permissionErr != nil {
			return nil, permissionErr
		}
		updated, err = permissionStore.UpdateGroupChatWithPermissions(ctx, chatID, caller, name, avatar, topic, slowMode, threadsEnabled, allowUserMainFeed, allowGuests, req.MemberAdditionsEnabled)
	}
	if err != nil {
		if req.MemberAdditionsEnabled != nil {
			return nil, chatPermissionError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if updated == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if len(changedFields) > 0 && s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatUpdated(ctx, chatID.String(), changedFields); err != nil {
			s.logPublishError(ctx, "chat.updated", err, slog.String("chat_id", chatID.String()))
		}
	}
	return &chatv1.UpdateChatResponse{Chat: chatRowToProto(updated)}, nil
}

func updateChatChangedFields(req *chatv1.UpdateChatRequest) []string {
	if req == nil {
		return nil
	}
	fields := make([]string, 0, 7)
	if req.Name != nil {
		fields = append(fields, "name")
	}
	if req.Topic != nil {
		fields = append(fields, "topic")
	}
	if req.SlowModeSeconds != nil {
		fields = append(fields, "slow_mode_seconds")
	}
	if req.AvatarUrl != nil {
		fields = append(fields, "avatar_url")
	}
	if req.ThreadsEnabled != nil {
		fields = append(fields, "threads_enabled")
	}
	if req.AllowUserMainFeed != nil {
		fields = append(fields, "allow_user_main_feed")
	}
	if req.AllowGuests != nil {
		fields = append(fields, "allow_guests")
	}
	if req.MemberAdditionsEnabled != nil {
		fields = append(fields, "member_additions_enabled")
	}
	return fields
}

func (s *ChatGRPC) AddMembers(ctx context.Context, req *chatv1.AddMembersRequest) (*chatv1.AddMembersResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err := s.ensureMatchSquadMutable(ctx, chatID); err != nil {
		return nil, err
	}
	if row.Type != "group" {
		if row.Type != "channel" || row.SpaceID != nil {
			return nil, status.Error(codes.InvalidArgument, "add members only supported for standalone groups and channels")
		}
	}
	if row.SpaceID != nil {
		return nil, status.Error(codes.FailedPrecondition, "space chat membership is owned by Space and Role services")
	}
	if err := authctx.RequireRegular(ctx); err != nil {
		return nil, err
	}
	member, err := s.DM.IsChatMember(ctx, chatID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a chat member")
	}

	ids := make([]uuid.UUID, 0, len(req.GetProfileIds()))
	guestIDs := make([]uuid.UUID, 0, len(req.GetProfileIds()))
	for _, raw := range req.GetProfileIds() {
		pid, perr := parseUUIDField("profile_id", raw)
		if perr != nil {
			return nil, perr
		}
		if s.Profiles != nil {
			if _, perr := s.Profiles.AccountIDByProfileID(ctx, pid); perr != nil {
				return nil, perr
			}
			guestProfiles, ok := s.Profiles.(GuestProfileLookup)
			if !ok {
				return nil, status.Error(codes.FailedPrecondition, "guest profile lookup not configured")
			}
			guest, perr := guestProfiles.IsGuestProfile(ctx, pid)
			if perr != nil {
				return nil, status.Error(codes.Unavailable, "guest profile lookup unavailable")
			}
			if guest {
				guestIDs = append(guestIDs, pid)
			}
		} else {
			return nil, status.Error(codes.FailedPrecondition, "profile service not configured")
		}
		if err := s.ensureInvitePrivacy(ctx, caller, pid); err != nil {
			if !authctx.IsInternalService(ctx) {
				return nil, err
			}
		}
		ids = append(ids, pid)
	}

	if authctx.IsInternalService(ctx) {
		ctx = store.WithGroupMinMembers(ctx, 2)
	}
	permissionStore, permissionErr := s.permissionStore()
	if permissionErr != nil {
		return nil, permissionErr
	}
	added, err := permissionStore.AddGroupMembersAs(ctx, chatID, caller, ids, guestIDs)
	if errors.Is(err, store.ErrRoleChangeForbidden) || errors.Is(err, store.ErrNotGroupMember) || errors.Is(err, store.ErrChatPermissionScope) {
		return nil, chatPermissionError(err)
	}
	if errors.Is(err, store.ErrGroupTooFewMembers) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, store.ErrGroupMemberLimit) {
		return nil, status.Error(codes.ResourceExhausted, err.Error())
	}
	if errors.Is(err, store.ErrGuestAdmissionDisabled) {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		for _, pid := range added {
			if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), pid.String(), "joined"); err != nil {
				s.logPublishError(ctx, "chat.member_changed", err, slog.String("chat_id", chatID.String()), slog.String("profile_id", pid.String()))
			}
		}
	}
	return &chatv1.AddMembersResponse{}, nil
}

func (s *ChatGRPC) RemoveMember(ctx context.Context, req *chatv1.RemoveMemberRequest) (*chatv1.RemoveMemberResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	targetID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err := s.ensureMatchSquadMutable(ctx, chatID); err != nil {
		return nil, err
	}
	if (row.Type != "group" && row.Type != "channel") || row.SpaceID != nil {
		return nil, status.Error(codes.InvalidArgument, "remove member only supported for standalone groups and channels")
	}
	// Space membership policy is owned by the Space role model.  Preserve the
	// existing chat-level owner rule here; standalone groups use the atomic
	// owner/admin hierarchy below.
	removeErr := s.DM.RemoveStandaloneGroupMember(ctx, chatID, caller, targetID)
	if err := removeErr; err != nil {
		if errors.Is(err, store.ErrCannotRemoveOwner) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, store.ErrRoleChangeForbidden) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if errors.Is(err, store.ErrGroupTooFewMembers) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "member not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), targetID.String(), "removed"); err != nil {
			s.logPublishError(ctx, "chat.member_changed", err, slog.String("chat_id", chatID.String()), slog.String("profile_id", targetID.String()))
		}
	}
	return &chatv1.RemoveMemberResponse{}, nil
}

func (s *ChatGRPC) LeaveChat(ctx context.Context, req *chatv1.LeaveChatRequest) (*chatv1.LeaveChatResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err := s.ensureMatchSquadMutable(ctx, chatID); err != nil {
		return nil, err
	}
	if (row.Type != "group" && row.Type != "channel") || row.SpaceID != nil {
		return nil, status.Error(codes.InvalidArgument, "leave only supported for standalone groups and channels")
	}
	if err := s.DM.LeaveGroupChat(ctx, chatID, caller); err != nil {
		if errors.Is(err, store.ErrOwnerMustTransfer) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, store.ErrGroupTooFewMembers) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "not a group member")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), caller.String(), "left"); err != nil {
			s.logPublishError(ctx, "chat.member_changed", err, slog.String("chat_id", chatID.String()), slog.String("profile_id", caller.String()))
		}
	}
	return &chatv1.LeaveChatResponse{}, nil
}

// SetGroupMemberRole changes a standalone group member between member and admin.
// Admins may promote members; only the owner may demote an admin. Ownership moves
// exclusively through TransferGroupOwnership.
func (s *ChatGRPC) SetGroupMemberRole(ctx context.Context, req *chatv1.SetGroupMemberRoleRequest) (*chatv1.SetGroupMemberRoleResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	targetID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	role := strings.TrimSpace(req.GetRole())
	if role != "admin" && role != "member" {
		return nil, status.Error(codes.InvalidArgument, "role must be admin or member")
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err := s.ensureMatchSquadMutable(ctx, chatID); err != nil {
		return nil, err
	}
	if row.Type != "group" || row.SpaceID != nil {
		return nil, status.Error(codes.InvalidArgument, "role changes are supported only for standalone groups")
	}
	if err := s.DM.SetStandaloneGroupMemberRole(ctx, chatID, caller, targetID, role); err != nil {
		if errors.Is(err, store.ErrNotGroupMember) || errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "group member not found")
		}
		if errors.Is(err, store.ErrRoleChangeForbidden) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if errors.Is(err, store.ErrRoleChangeInvalid) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), targetID.String(), "role_changed"); err != nil {
			s.logPublishError(ctx, "chat.member_changed", err, slog.String("chat_id", chatID.String()), slog.String("profile_id", targetID.String()))
		}
	}
	return &chatv1.SetGroupMemberRoleResponse{}, nil
}

func (s *ChatGRPC) TransferGroupOwnership(ctx context.Context, req *chatv1.TransferGroupOwnershipRequest) (*chatv1.TransferGroupOwnershipResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	newOwner, err := parseUUIDField("new_owner_profile_id", req.GetNewOwnerProfileId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err := s.ensureMatchSquadMutable(ctx, chatID); err != nil {
		return nil, err
	}
	if row.Type != "group" || row.SpaceID != nil {
		return nil, status.Error(codes.InvalidArgument, "transfer only supported for standalone groups")
	}
	if err := s.DM.TransferGroupOwnership(ctx, chatID, caller, newOwner); err != nil {
		if errors.Is(err, store.ErrNotGroupOwner) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "new owner is not a group member")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), newOwner.String(), "owner_transferred"); err != nil {
			s.logPublishError(ctx, "chat.member_changed", err, slog.String("chat_id", chatID.String()), slog.String("profile_id", newOwner.String()))
		}
	}
	return &chatv1.TransferGroupOwnershipResponse{}, nil
}

type matchSquadOwnershipReader interface {
	IsMatchSquadChat(context.Context, uuid.UUID) (bool, error)
}

func (s *ChatGRPC) ensureMatchSquadMutable(ctx context.Context, chatID uuid.UUID) error {
	reader, ok := s.DM.(matchSquadOwnershipReader)
	if !ok {
		return nil
	}
	owned, err := reader.IsMatchSquadChat(ctx, chatID)
	if err != nil {
		return status.Error(codes.Unavailable, "Chat ownership lookup unavailable")
	}
	if owned {
		return status.Error(codes.FailedPrecondition, "MatchSquad membership is controlled by Matchmaking")
	}
	return nil
}
