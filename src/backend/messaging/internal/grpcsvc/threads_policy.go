package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"
	"voice/backend/role/permissions"
)

const (
	permCreateThreads = permissions.TextChatCreateThreads
	permSendInThreads = permissions.TextChatSendInThreads
)

type threadPolicyDeps struct {
	Policy    threadPolicyLoader
	Messages  threadMessageLookup
	RolePerms mentionsRoleChecker
	Members   ChatGuard
}

type threadPolicyLoader interface {
	Load(context.Context, uuid.UUID) (*store.ChatThreadPolicy, error)
}

type threadMessageLookup interface {
	GetMessageByID(context.Context, uuid.UUID) (*store.MessageRow, error)
	ThreadHasReplies(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type mentionsRoleChecker interface {
	HasChatPermission(ctx context.Context, spaceID, profileID, chatID uuid.UUID, permission string) (bool, error)
}

type officialChatIdentityAuthority interface {
	CanPostAsChat(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

func (d threadPolicyDeps) validateOfficialChatIdentity(ctx context.Context, chatID, profileID uuid.UUID) error {
	authority, ok := d.Members.(officialChatIdentityAuthority)
	if !ok || isNilDependency(authority) {
		return status.Error(codes.Unavailable, "chat identity authority unavailable")
	}
	allowed, err := authority.CanPostAsChat(ctx, chatID, profileID)
	if errors.Is(err, store.ErrNotChatMember) || status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.NotFound {
		return status.Error(codes.PermissionDenied, "chat identity permission denied")
	}
	if err != nil {
		return status.Error(codes.Unavailable, "chat identity authority unavailable")
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "chat identity permission denied")
	}
	return nil
}

func (d threadPolicyDeps) validateSend(ctx context.Context, chatID, profileID uuid.UUID, threadParentID *uuid.UUID, postedAsChat bool) error {
	if isNilDependency(d.Policy) {
		return status.Error(codes.FailedPrecondition, "chat policy not configured")
	}
	pol, err := d.Policy.Load(ctx, chatID)
	if errors.Is(err, store.ErrChatNotFound) {
		return status.Error(codes.NotFound, "chat not found")
	}
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if pol == nil {
		return status.Error(codes.Unavailable, "chat policy unavailable")
	}

	if threadParentID != nil {
		// A thread changes reply permissions, not authority to impersonate a
		// standalone channel. Self-identity replies retain their thread policy.
		if (pol.ChatType == "channel" || pol.ChatType == "group") && pol.SpaceID == nil && postedAsChat {
			if err := d.validateOfficialChatIdentity(ctx, chatID, profileID); err != nil {
				return err
			}
		}
		return d.validateThreadReply(ctx, chatID, profileID, pol, *threadParentID)
	}
	return d.validateMainFeedSend(ctx, chatID, profileID, pol, postedAsChat)
}

func (d threadPolicyDeps) validateMainFeedSend(ctx context.Context, chatID, profileID uuid.UUID, pol *store.ChatThreadPolicy, postedAsChat bool) error {
	if pol == nil {
		return status.Error(codes.Unavailable, "chat policy unavailable")
	}
	switch pol.ChatType {
	case "dm":
		return nil
	case "group":
		if pol.SpaceID == nil && postedAsChat {
			if err := d.validateOfficialChatIdentity(ctx, chatID, profileID); err != nil {
				return err
			}
		}
		if !pol.AllowUserMainFeed {
			return status.Error(codes.PermissionDenied, "user main-feed posts are disabled for this chat")
		}
		return nil
	case "channel":
		if pol.SpaceID == nil {
			return d.validateStandaloneChannelSend(ctx, chatID, profileID, pol.AllowUserMainFeed, postedAsChat)
		}
		if isNilDependency(d.RolePerms) {
			return status.Error(codes.Unavailable, "role service unavailable")
		}
		allowed, err := d.RolePerms.HasChatPermission(ctx, *pol.SpaceID, profileID, chatID, permissions.TextChatSendMessages)
		if err != nil {
			return status.Error(codes.Unavailable, "role service unavailable")
		}
		if !allowed {
			return status.Error(codes.PermissionDenied, "send messages not permitted in this chat")
		}
		return nil // The Space Role decision authorizes the existing main-feed mode.
	default:
		return status.Error(codes.InvalidArgument, "unsupported chat type")
	}
}

func (d threadPolicyDeps) validateStandaloneChannelSend(ctx context.Context, chatID, profileID uuid.UUID, allowUser, postedAsChat bool) error {
	if postedAsChat {
		return d.validateOfficialChatIdentity(ctx, chatID, profileID)
	}
	if isNilDependency(d.Members) {
		return status.Error(codes.Unavailable, "chat role authority unavailable")
	}
	role, err := d.Members.MemberRole(ctx, chatID, profileID)
	if errors.Is(err, store.ErrNotChatMember) {
		return status.Error(codes.PermissionDenied, "not a chat member")
	}
	if err != nil {
		return status.Error(codes.Unavailable, "chat role authority unavailable")
	}
	role = strings.TrimSpace(role)
	if role != "owner" && role != "admin" && role != "member" {
		return status.Error(codes.PermissionDenied, "unknown chat member role")
	}
	if !allowUser {
		return status.Error(codes.PermissionDenied, "user main-feed posts are disabled for this chat")
	}
	return nil
}

func (d threadPolicyDeps) validateThreadReply(ctx context.Context, chatID, profileID uuid.UUID, pol *store.ChatThreadPolicy, parentID uuid.UUID) error {
	if isNilDependency(d.Messages) {
		return status.Error(codes.FailedPrecondition, "messages store not configured")
	}
	parent, err := d.Messages.GetMessageByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "thread parent not found")
		}
		return status.Error(codes.Internal, err.Error())
	}
	if parent.DeletedAt != nil {
		return status.Error(codes.NotFound, "thread parent not found")
	}
	if parent.ChatID != chatID {
		return status.Error(codes.NotFound, "thread parent not found")
	}
	if parent.ThreadParentID != nil {
		return status.Error(codes.InvalidArgument, "nested thread replies are not supported")
	}

	switch pol.ChatType {
	case "dm":
		return nil
	case "group":
		if !pol.ThreadsEnabled {
			return status.Error(codes.FailedPrecondition, "threads are disabled for this chat")
		}
	case "channel":
		// channel threads always allowed when parent exists
	default:
		return status.Error(codes.InvalidArgument, "unsupported chat type")
	}

	if pol.SpaceID == nil || d.RolePerms == nil {
		return nil
	}
	hasReplies, err := d.Messages.ThreadHasReplies(ctx, chatID, parentID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	perm := permCreateThreads
	if hasReplies {
		perm = permSendInThreads
	}
	allowed, err := d.RolePerms.HasChatPermission(ctx, *pol.SpaceID, profileID, chatID, perm)
	if err != nil {
		return status.Error(codes.Unavailable, "role service unavailable")
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "missing thread permission")
	}
	return nil
}
