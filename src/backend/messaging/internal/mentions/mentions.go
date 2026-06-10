package mentions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"
)

// Entry is one element of mentions_json: [{type, target_id?}].
type Entry struct {
	Type     string `json:"type"`
	TargetID string `json:"target_id,omitempty"`
}

// ChatMeta describes a chat for mention validation.
type ChatMeta struct {
	ChatID   uuid.UUID
	ChatType string // dm | group | channel
	SpaceID  *uuid.UUID
	Members  []uuid.UUID
}

// RolePermissionChecker validates space-scoped text chat permissions.
type RolePermissionChecker interface {
	HasSpacePermission(ctx context.Context, spaceID, profileID uuid.UUID, permission string) (bool, error)
}

// OnlinePresenceLookup returns profile IDs that are online or idle among the given set.
type OnlinePresenceLookup interface {
	FilterOnlineProfileIDs(ctx context.Context, profileIDs []uuid.UUID) ([]uuid.UUID, error)
}

var (
	ErrInvalidMentionsJSON = errors.New("invalid mentions_json")
	ErrInvalidMentionType  = errors.New("invalid mention type")
	ErrNotChatMember       = errors.New("mentioned user is not a chat member")
	ErrBroadcastInDM       = errors.New("broadcast mentions are not allowed in dm")
	ErrBroadcastNoSpace    = errors.New("broadcast mentions require a space chat")
	ErrPermissionDenied    = errors.New("mention permission denied")
)

// Process validates mentions_json, normalizes storage JSON, and returns profile IDs to notify (excludes sender).
func Process(
	ctx context.Context,
	meta ChatMeta,
	senderProfileID uuid.UUID,
	mentionsJSON string,
	roles RolePermissionChecker,
	presence OnlinePresenceLookup,
) (normalized string, notifyTargets []uuid.UUID, err error) {
	raw := strings.TrimSpace(mentionsJSON)
	if raw == "" {
		return "[]", nil, nil
	}
	if !json.Valid([]byte(raw)) {
		return "", nil, status.Error(codes.InvalidArgument, ErrInvalidMentionsJSON.Error())
	}
	var entries []Entry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return "", nil, status.Error(codes.InvalidArgument, ErrInvalidMentionsJSON.Error())
	}
	if len(entries) == 0 {
		return "[]", nil, nil
	}

	memberSet := make(map[uuid.UUID]struct{}, len(meta.Members))
	for _, m := range meta.Members {
		memberSet[m] = struct{}{}
	}

	targetSet := make(map[uuid.UUID]struct{})
	normalizedEntries := make([]Entry, 0, len(entries))

	for _, e := range entries {
		typ := strings.ToLower(strings.TrimSpace(e.Type))
		switch typ {
		case "user":
			tid := strings.TrimSpace(e.TargetID)
			if tid == "" {
				return "", nil, status.Error(codes.InvalidArgument, "user mention requires target_id")
			}
			pid, perr := uuid.Parse(tid)
			if perr != nil {
				return "", nil, status.Error(codes.InvalidArgument, "invalid user target_id")
			}
			if _, ok := memberSet[pid]; !ok {
				return "", nil, status.Error(codes.InvalidArgument, ErrNotChatMember.Error())
			}
			normalizedEntries = append(normalizedEntries, Entry{Type: "user", TargetID: pid.String()})
			if pid != senderProfileID {
				targetSet[pid] = struct{}{}
			}
		case "everyone", "here":
			if meta.ChatType == "dm" {
				return "", nil, status.Error(codes.InvalidArgument, ErrBroadcastInDM.Error())
			}
			if meta.SpaceID == nil {
				return "", nil, status.Error(codes.InvalidArgument, ErrBroadcastNoSpace.Error())
			}
			perm := permissions.TextChatMentionAllInChat
			if typ == "here" {
				perm = permissions.TextChatMentionAllOnline
			}
			if roles == nil {
				return "", nil, status.Error(codes.Unavailable, "role service not configured")
			}
			allowed, rerr := roles.HasSpacePermission(ctx, *meta.SpaceID, senderProfileID, perm)
			if rerr != nil {
				if status.Code(rerr) == codes.Unavailable {
					return "", nil, status.Error(codes.Unavailable, rerr.Error())
				}
				return "", nil, status.Error(codes.Internal, rerr.Error())
			}
			if !allowed {
				return "", nil, status.Error(codes.PermissionDenied, ErrPermissionDenied.Error())
			}
			var expand []uuid.UUID
			if typ == "everyone" {
				expand = append([]uuid.UUID(nil), meta.Members...)
			} else {
				if presence == nil {
					return "", nil, status.Error(codes.Unavailable, "presence service not configured")
				}
				online, perr := presence.FilterOnlineProfileIDs(ctx, meta.Members)
				if perr != nil {
					if status.Code(perr) == codes.Unavailable {
						return "", nil, status.Error(codes.Unavailable, perr.Error())
					}
					return "", nil, status.Error(codes.Internal, perr.Error())
				}
				expand = online
			}
			normalizedEntries = append(normalizedEntries, Entry{Type: typ})
			for _, pid := range expand {
				if pid != senderProfileID {
					targetSet[pid] = struct{}{}
				}
			}
		default:
			return "", nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%s: %s", ErrInvalidMentionType.Error(), typ))
		}
	}

	out, merr := json.Marshal(normalizedEntries)
	if merr != nil {
		return "", nil, status.Error(codes.Internal, merr.Error())
	}
	notifyTargets = make([]uuid.UUID, 0, len(targetSet))
	for pid := range targetSet {
		notifyTargets = append(notifyTargets, pid)
	}
	return string(out), notifyTargets, nil
}
