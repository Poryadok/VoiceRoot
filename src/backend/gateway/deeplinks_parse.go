package main

import (
	"errors"
	"net/url"
	"strings"
)

var errInvalidDeepLink = errors.New("invalid deep link")

// DeepLinkKind identifies the navigation target (docs/features/deep-links.md).
type DeepLinkKind string

const (
	DeepLinkKindInvite        DeepLinkKind = "invite"
	DeepLinkKindSpace         DeepLinkKind = "space"
	DeepLinkKindSpaceChat     DeepLinkKind = "space_chat"
	DeepLinkKindVoiceRoom     DeepLinkKind = "voice_room"
	DeepLinkKindSpaceMessage  DeepLinkKind = "space_message"
	DeepLinkKindChat          DeepLinkKind = "chat"
	DeepLinkKindChatMessage   DeepLinkKind = "chat_message"
	DeepLinkKindProfile       DeepLinkKind = "profile"
	DeepLinkKindDM            DeepLinkKind = "dm"
)

// DeepLinkTarget is a parsed deep link before domain validation.
type DeepLinkTarget struct {
	Kind        DeepLinkKind
	SpaceID     string
	ChatID      string
	VoiceRoomID string
	MessageID   string
	InviteCode  string
	Username    string
	UserID      string
	RawURL      string
}

// ParseDeepLinkURL normalizes voice:// and https://voice.gg/ URLs.
func ParseDeepLinkURL(raw string) (DeepLinkTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DeepLinkTarget{}, errInvalidDeepLink
	}

	u, err := url.Parse(raw)
	if err != nil {
		return DeepLinkTarget{}, errInvalidDeepLink
	}

	path := strings.TrimPrefix(u.Path, "/")
	for strings.HasSuffix(path, "/") && path != "" {
		path = strings.TrimSuffix(path, "/")
	}

	switch strings.ToLower(u.Scheme) {
	case "voice":
		voicePath := strings.TrimPrefix(strings.TrimPrefix(raw, "voice://"), "voice:")
		voicePath = strings.TrimPrefix(voicePath, "//")
		voicePath = strings.Trim(voicePath, "/")
		return parseDeepLinkPath(voicePath, raw)
	case "https", "http":
		host := strings.ToLower(u.Hostname())
		if host != "voice.gg" && host != "www.voice.gg" {
			return DeepLinkTarget{}, errInvalidDeepLink
		}
		return parseDeepLinkPath(path, raw)
	default:
		return DeepLinkTarget{}, errInvalidDeepLink
	}
}

func parseDeepLinkPath(path, raw string) (DeepLinkTarget, error) {
	if path == "" {
		return DeepLinkTarget{}, errInvalidDeepLink
	}

	parts := strings.Split(path, "/")
	switch parts[0] {
	case "invite":
		if len(parts) != 2 || parts[1] == "" {
			return DeepLinkTarget{}, errInvalidDeepLink
		}
		return DeepLinkTarget{Kind: DeepLinkKindInvite, InviteCode: parts[1], RawURL: raw}, nil
	case "s":
		return parseSpacePath(parts, raw)
	case "ch":
		return parseChatPath(parts, raw)
	case "u":
		if len(parts) != 2 || parts[1] == "" {
			return DeepLinkTarget{}, errInvalidDeepLink
		}
		return DeepLinkTarget{Kind: DeepLinkKindProfile, Username: parts[1], RawURL: raw}, nil
	case "dm":
		if len(parts) != 2 || parts[1] == "" {
			return DeepLinkTarget{}, errInvalidDeepLink
		}
		return DeepLinkTarget{Kind: DeepLinkKindDM, UserID: parts[1], RawURL: raw}, nil
	default:
		return DeepLinkTarget{}, errInvalidDeepLink
	}
}

func parseSpacePath(parts []string, raw string) (DeepLinkTarget, error) {
	if len(parts) < 2 || parts[1] == "" {
		return DeepLinkTarget{}, errInvalidDeepLink
	}
	spaceID := parts[1]
	if len(parts) == 2 {
		return DeepLinkTarget{Kind: DeepLinkKindSpace, SpaceID: spaceID, RawURL: raw}, nil
	}
	switch parts[2] {
	case "c":
		if len(parts) == 4 && parts[3] != "" {
			return DeepLinkTarget{Kind: DeepLinkKindSpaceChat, SpaceID: spaceID, ChatID: parts[3], RawURL: raw}, nil
		}
		if len(parts) == 6 && parts[3] != "" && parts[4] == "m" && parts[5] != "" {
			return DeepLinkTarget{
				Kind:      DeepLinkKindSpaceMessage,
				SpaceID:   spaceID,
				ChatID:    parts[3],
				MessageID: parts[5],
				RawURL:    raw,
			}, nil
		}
	case "v":
		if len(parts) == 4 && parts[3] != "" {
			return DeepLinkTarget{Kind: DeepLinkKindVoiceRoom, SpaceID: spaceID, VoiceRoomID: parts[3], RawURL: raw}, nil
		}
	}
	return DeepLinkTarget{}, errInvalidDeepLink
}

func parseChatPath(parts []string, raw string) (DeepLinkTarget, error) {
	if len(parts) == 2 && parts[1] != "" {
		return DeepLinkTarget{Kind: DeepLinkKindChat, ChatID: parts[1], RawURL: raw}, nil
	}
	if len(parts) == 4 && parts[1] != "" && parts[2] == "m" && parts[3] != "" {
		return DeepLinkTarget{Kind: DeepLinkKindChatMessage, ChatID: parts[1], MessageID: parts[3], RawURL: raw}, nil
	}
	return DeepLinkTarget{}, errInvalidDeepLink
}
