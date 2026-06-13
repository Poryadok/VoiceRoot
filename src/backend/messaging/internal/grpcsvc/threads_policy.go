package grpcsvc

import (
	"context"
	"errors"

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
	Policy    *store.SQLChatThreadPolicy
	Messages  *store.MessagesStore
	RolePerms mentionsRoleChecker
}

type mentionsRoleChecker interface {
	HasChatPermission(ctx context.Context, spaceID, profileID, chatID uuid.UUID, permission string) (bool, error)
}

func (d threadPolicyDeps) validateSend(ctx context.Context, chatID, profileID uuid.UUID, threadParentID *uuid.UUID, postedAsChat bool) error {
	if d.Policy == nil {
		return status.Error(codes.FailedPrecondition, "chat policy not configured")
	}
	pol, err := d.Policy.Load(ctx, chatID)
	if errors.Is(err, store.ErrChatNotFound) {
		return status.Error(codes.NotFound, "chat not found")
	}
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	if threadParentID != nil {
		return d.validateThreadReply(ctx, chatID, profileID, pol, *threadParentID)
	}
	return d.validateMainFeedSend(ctx, chatID, profileID, pol, postedAsChat)
}

func (d threadPolicyDeps) validateMainFeedSend(ctx context.Context, chatID, profileID uuid.UUID, pol *store.ChatThreadPolicy, postedAsChat bool) error {
	switch pol.ChatType {
	case "dm":
		return nil
	case "group":
		if !pol.AllowUserMainFeed {
			return status.Error(codes.PermissionDenied, "user main-feed posts are disabled for this chat")
		}
		return nil
	case "channel":
		if pol.AllowUserMainFeed && !postedAsChat {
			return nil
		}
		if postedAsChat {
			return nil
		}
		return status.Error(codes.PermissionDenied, "channel main-feed posts require posted_as_chat")
	default:
		return status.Error(codes.InvalidArgument, "unsupported chat type")
	}
}

func (d threadPolicyDeps) validateThreadReply(ctx context.Context, chatID, profileID uuid.UUID, pol *store.ChatThreadPolicy, parentID uuid.UUID) error {
	if d.Messages == nil {
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
