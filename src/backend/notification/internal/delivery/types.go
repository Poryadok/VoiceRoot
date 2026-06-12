package delivery

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType mirrors product notification buckets from docs/features/notifications.md.
type NotificationType string

const (
	TypeNewMessage   NotificationType = "new_message"
	TypeMention      NotificationType = "mention"
	TypeReply        NotificationType = "reply"
	TypeReaction     NotificationType = "reaction"
	TypeFriendReq    NotificationType = "friend_request"
	TypeMatchFound      NotificationType = "match_found"
	TypeSearchNudge     NotificationType = "search_nudge"
	TypeSearchTimeout   NotificationType = "search_timeout"
	TypeIncomingCall    NotificationType = "incoming_call"
	TypeSystem       NotificationType = "system"
)

// DeliveryInput captures routing context for a single recipient.
type DeliveryInput struct {
	RecipientProfileID uuid.UUID
	SenderProfileID    uuid.UUID
	ChatID             string
	Type               NotificationType
	IsOnline           bool
	At                 time.Time
}

// DeliveryDecision selects in-app vs push channels.
type DeliveryDecision struct {
	InApp bool
	Push  bool
}

// SettingsSnapshot is the effective notification settings for a recipient+chat.
type SettingsSnapshot struct {
	ChatMuted            bool
	SuppressTypes        []NotificationType
	MentionOverridesMute bool
}

// QuietHoursSnapshot is the effective DND schedule for a recipient.
type QuietHoursSnapshot struct {
	Enabled           bool
	StartTime         string // HH:MM
	EndTime           string // HH:MM
	Timezone          string
	OverrideMentions  bool
	At                time.Time
}

// GroupingState tracks collapsed push metadata per chat.
type GroupingState struct {
	CollapseTag string
	Counter     int
	LastBody    string
}
