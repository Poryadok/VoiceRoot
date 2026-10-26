package catalogassets

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/chat/internal/store"
)

type DestinationChats interface {
	FindChatByID(context.Context, uuid.UUID) (*store.ChatRow, error)
	IsChatMember(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	IsMemberDeletedForSelf(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}
type DestinationSpaces interface {
	IsSpaceMember(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

// NativeDestinationAuthority reuses the native Chat lifecycle-filtered reader
// and membership stores. Roles must be the protected Chat current-ACL client;
// no user metadata is fabricated for a pure File caller. This is a fresh read,
// not a cross-service mutation lease or a replacement for Messaging send policy.
type NativeDestinationAuthority struct {
	Chats  DestinationChats
	Spaces DestinationSpaces
	Roles  rolev1.RoleServiceClient
}

func (a *NativeDestinationAuthority) RequirePlaintextMember(ctx context.Context, actor Actor, ref *chatv1.ChatRef) error {
	_, err := a.ObservePlaintextMember(ctx, actor, ref)
	return err
}

func (a *NativeDestinationAuthority) ObservePlaintextMember(ctx context.Context, actor Actor, ref *chatv1.ChatRef) (chatv1.ChatType, error) {
	if actor.AccountID == uuid.Nil || actor.ProfileID == uuid.Nil || !validDestination(ref) {
		return 0, status.Error(codes.InvalidArgument, "invalid catalog destination")
	}
	if a == nil || a.Chats == nil {
		return 0, status.Error(codes.Unavailable, "catalog destination store unavailable")
	}
	id := uuid.MustParse(ref.Id)
	row, kind, err := a.native(ctx, actor, id, ref.GetType())
	if err != nil {
		return 0, err
	}
	if row.SpaceID != nil {
		if a.Roles == nil {
			return 0, status.Error(codes.Unavailable, "catalog destination ACL unavailable")
		}
		response, err := a.Roles.CheckPermission(ctx, &rolev1.CheckPermissionRequest{SpaceId: row.SpaceID.String(), ProfileId: actor.ProfileID.String(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: id.String(), Type: &kind}})
		if err != nil {
			return 0, status.Error(codes.Unavailable, "catalog destination ACL unavailable")
		}
		if !response.GetAllowed() {
			return 0, status.Error(codes.PermissionDenied, "catalog destination denied")
		}
	}
	// Remote ACL work cannot freeze native membership, lifecycle, or E2E state.
	// Re-read them and reject a changed scope before returning the observation.
	current, currentKind, err := a.native(ctx, actor, id, ref.GetType())
	if err != nil {
		return 0, err
	}
	if currentKind != kind || (row.SpaceID == nil) != (current.SpaceID == nil) || row.SpaceID != nil && *row.SpaceID != *current.SpaceID {
		return 0, status.Error(codes.Unavailable, "catalog destination changed")
	}
	return kind, nil
}

func (a *NativeDestinationAuthority) native(ctx context.Context, actor Actor, id uuid.UUID, expected chatv1.ChatType) (*store.ChatRow, chatv1.ChatType, error) {
	row, err := a.Chats.FindChatByID(ctx, id)
	if err != nil {
		return nil, 0, status.Error(codes.Unavailable, "catalog destination store unavailable")
	}
	if row == nil {
		return nil, 0, status.Error(codes.NotFound, "catalog destination unavailable")
	}
	var kind chatv1.ChatType
	switch row.Type {
	case "dm":
		kind = chatv1.ChatType_CHAT_TYPE_DM
	case "group":
		kind = chatv1.ChatType_CHAT_TYPE_GROUP
	case "channel":
		kind = chatv1.ChatType_CHAT_TYPE_CHANNEL
	default:
		return nil, 0, status.Error(codes.Unavailable, "invalid native catalog destination")
	}
	if row.ID != id || row.Type == "dm" && row.SpaceID != nil {
		return nil, 0, status.Error(codes.Unavailable, "invalid native catalog scope")
	}
	if expected != 0 && expected != kind {
		return nil, 0, status.Error(codes.InvalidArgument, "catalog destination type mismatch")
	}
	if row.E2EEnabled {
		return nil, 0, status.Error(codes.FailedPrecondition, "catalog requires plaintext destination")
	}
	var member bool
	if row.SpaceID != nil {
		if a.Spaces == nil {
			return nil, 0, status.Error(codes.Unavailable, "catalog space membership unavailable")
		}
		member, err = a.Spaces.IsSpaceMember(ctx, *row.SpaceID, actor.ProfileID)
	} else {
		member, err = a.Chats.IsChatMember(ctx, id, actor.ProfileID)
	}
	if err != nil {
		return nil, 0, status.Error(codes.Unavailable, "catalog membership unavailable")
	}
	if !member {
		return nil, 0, status.Error(codes.PermissionDenied, "catalog destination denied")
	}
	if row.Type == "dm" {
		deleted, err := a.Chats.IsMemberDeletedForSelf(ctx, id, actor.ProfileID)
		if err != nil {
			return nil, 0, status.Error(codes.Unavailable, "catalog membership unavailable")
		}
		if deleted {
			return nil, 0, status.Error(codes.PermissionDenied, "catalog destination denied")
		}
	}
	return row, kind, nil
}
