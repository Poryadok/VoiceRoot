package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDeepLinkURL(t *testing.T) {
	t.Parallel()

	const (
		spaceID      = "550e8400-e29b-41d4-a716-446655440000"
		chatID       = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
		voiceRoomID  = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"
		messageID    = "6ba7b812-9dad-11d1-80b4-00c04fd430c8"
		userID       = "6ba7b813-9dad-11d1-80b4-00c04fd430c8"
		inviteCode   = "abc123XYZ"
		username     = "vanya"
	)

	tests := []struct {
		name    string
		raw     string
		want    DeepLinkTarget
		wantErr bool
	}{
		// invite
		{
			name: "voice invite",
			raw:  "voice://invite/" + inviteCode,
			want: DeepLinkTarget{Kind: DeepLinkKindInvite, InviteCode: inviteCode, RawURL: "voice://invite/" + inviteCode},
		},
		{
			name: "https invite",
			raw:  "https://voice.gg/invite/" + inviteCode,
			want: DeepLinkTarget{Kind: DeepLinkKindInvite, InviteCode: inviteCode, RawURL: "https://voice.gg/invite/" + inviteCode},
		},
		{
			name: "https invite with trailing slash",
			raw:  "https://voice.gg/invite/" + inviteCode + "/",
			want: DeepLinkTarget{Kind: DeepLinkKindInvite, InviteCode: inviteCode, RawURL: "https://voice.gg/invite/" + inviteCode + "/"},
		},
		// space
		{
			name: "voice space",
			raw:  "voice://s/" + spaceID,
			want: DeepLinkTarget{Kind: DeepLinkKindSpace, SpaceID: spaceID, RawURL: "voice://s/" + spaceID},
		},
		{
			name: "https space",
			raw:  "https://voice.gg/s/" + spaceID,
			want: DeepLinkTarget{Kind: DeepLinkKindSpace, SpaceID: spaceID, RawURL: "https://voice.gg/s/" + spaceID},
		},
		// space chat
		{
			name: "voice space chat",
			raw:  "voice://s/" + spaceID + "/c/" + chatID,
			want: DeepLinkTarget{
				Kind:    DeepLinkKindSpaceChat,
				SpaceID: spaceID,
				ChatID:  chatID,
				RawURL:  "voice://s/" + spaceID + "/c/" + chatID,
			},
		},
		{
			name: "https space chat",
			raw:  "https://voice.gg/s/" + spaceID + "/c/" + chatID,
			want: DeepLinkTarget{
				Kind:    DeepLinkKindSpaceChat,
				SpaceID: spaceID,
				ChatID:  chatID,
				RawURL:  "https://voice.gg/s/" + spaceID + "/c/" + chatID,
			},
		},
		// voice room
		{
			name: "voice room",
			raw:  "voice://s/" + spaceID + "/v/" + voiceRoomID,
			want: DeepLinkTarget{
				Kind:        DeepLinkKindVoiceRoom,
				SpaceID:     spaceID,
				VoiceRoomID: voiceRoomID,
				RawURL:      "voice://s/" + spaceID + "/v/" + voiceRoomID,
			},
		},
		{
			name: "https voice room",
			raw:  "https://voice.gg/s/" + spaceID + "/v/" + voiceRoomID,
			want: DeepLinkTarget{
				Kind:        DeepLinkKindVoiceRoom,
				SpaceID:     spaceID,
				VoiceRoomID: voiceRoomID,
				RawURL:      "https://voice.gg/s/" + spaceID + "/v/" + voiceRoomID,
			},
		},
		// space message anchor
		{
			name: "voice space message",
			raw:  "voice://s/" + spaceID + "/c/" + chatID + "/m/" + messageID,
			want: DeepLinkTarget{
				Kind:      DeepLinkKindSpaceMessage,
				SpaceID:   spaceID,
				ChatID:    chatID,
				MessageID: messageID,
				RawURL:    "voice://s/" + spaceID + "/c/" + chatID + "/m/" + messageID,
			},
		},
		{
			name: "https space message",
			raw:  "https://voice.gg/s/" + spaceID + "/c/" + chatID + "/m/" + messageID,
			want: DeepLinkTarget{
				Kind:      DeepLinkKindSpaceMessage,
				SpaceID:   spaceID,
				ChatID:    chatID,
				MessageID: messageID,
				RawURL:    "https://voice.gg/s/" + spaceID + "/c/" + chatID + "/m/" + messageID,
			},
		},
		// chat outside space
		{
			name: "voice chat",
			raw:  "voice://ch/" + chatID,
			want: DeepLinkTarget{Kind: DeepLinkKindChat, ChatID: chatID, RawURL: "voice://ch/" + chatID},
		},
		{
			name: "https chat",
			raw:  "https://voice.gg/ch/" + chatID,
			want: DeepLinkTarget{Kind: DeepLinkKindChat, ChatID: chatID, RawURL: "https://voice.gg/ch/" + chatID},
		},
		// chat message outside space
		{
			name: "voice chat message",
			raw:  "voice://ch/" + chatID + "/m/" + messageID,
			want: DeepLinkTarget{
				Kind:      DeepLinkKindChatMessage,
				ChatID:    chatID,
				MessageID: messageID,
				RawURL:    "voice://ch/" + chatID + "/m/" + messageID,
			},
		},
		{
			name: "https chat message",
			raw:  "https://voice.gg/ch/" + chatID + "/m/" + messageID,
			want: DeepLinkTarget{
				Kind:      DeepLinkKindChatMessage,
				ChatID:    chatID,
				MessageID: messageID,
				RawURL:    "https://voice.gg/ch/" + chatID + "/m/" + messageID,
			},
		},
		// profile
		{
			name: "voice profile",
			raw:  "voice://u/" + username,
			want: DeepLinkTarget{Kind: DeepLinkKindProfile, Username: username, RawURL: "voice://u/" + username},
		},
		{
			name: "https profile",
			raw:  "https://voice.gg/u/" + username,
			want: DeepLinkTarget{Kind: DeepLinkKindProfile, Username: username, RawURL: "https://voice.gg/u/" + username},
		},
		// dm
		{
			name: "voice dm",
			raw:  "voice://dm/" + userID,
			want: DeepLinkTarget{Kind: DeepLinkKindDM, UserID: userID, RawURL: "voice://dm/" + userID},
		},
		{
			name: "https dm",
			raw:  "https://voice.gg/dm/" + userID,
			want: DeepLinkTarget{Kind: DeepLinkKindDM, UserID: userID, RawURL: "https://voice.gg/dm/" + userID},
		},
		// UTM query params preserved in RawURL but do not change kind
		{
			name: "https invite with utm",
			raw:  "https://voice.gg/invite/" + inviteCode + "?utm_source=share&utm_medium=copy",
			want: DeepLinkTarget{
				Kind:       DeepLinkKindInvite,
				InviteCode: inviteCode,
				RawURL:     "https://voice.gg/invite/" + inviteCode + "?utm_source=share&utm_medium=copy",
			},
		},
		// invalid
		{name: "empty", raw: "", wantErr: true},
		{name: "garbage", raw: "not-a-url", wantErr: true},
		{name: "foreign host", raw: "https://example.com/invite/foo", wantErr: true},
		{name: "unknown voice path", raw: "voice://unknown/foo", wantErr: true},
		{name: "bare https host", raw: "https://voice.gg/", wantErr: true},
		{name: "invite missing code", raw: "voice://invite/", wantErr: true},
		{name: "unsupported scheme", raw: "ftp://voice.gg/s/" + spaceID, wantErr: true},
		{name: "space missing id", raw: "https://voice.gg/s/", wantErr: true},
		{name: "chat message missing id", raw: "voice://ch/" + chatID + "/m/", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseDeepLinkURL(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
