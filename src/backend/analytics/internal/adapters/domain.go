package adapters

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	eventsv1 "voice.app/voice/events/v1"
	idhash "voice/backend/analytics/internal/hash"
)

// Mapper converts domain IDs to hashed analytics fields.
type Mapper struct {
	HashKey string
}

func (m Mapper) event(eventType, source string, occurredAt *timestamppb.Timestamp, accountID, profileID string, props map[string]any) *analyticsv1.AnalyticsEvent {
	if occurredAt == nil {
		occurredAt = timestamppb.Now()
	}
	propsJSON := "{}"
	if len(props) > 0 {
		if b, err := json.Marshal(props); err == nil {
			propsJSON = string(b)
		}
	}
	ev := &analyticsv1.AnalyticsEvent{
		EventId:        uuid.NewString(),
		EventType:      eventType,
		SourceService:  source,
		Timestamp:      occurredAt,
		PropertiesJson: propsJSON,
	}
	if h := idhash.ID(m.HashKey, accountID); h != "" {
		ev.UserIdHashed = &h
	}
	if h := idhash.ID(m.HashKey, profileID); h != "" {
		ev.ProfileIdHashed = &h
	}
	return ev
}

func (m Mapper) FromUser(ev *eventsv1.UserStreamEvent) *analyticsv1.AnalyticsEvent {
	if ev == nil {
		return nil
	}
	switch p := ev.GetPayload().(type) {
	case *eventsv1.UserStreamEvent_UserRegistered:
		return m.event("user_registered", "auth", ev.GetOccurredAt(), p.UserRegistered.GetAccountId(), "", map[string]any{
			"type": p.UserRegistered.GetType(), "method": p.UserRegistered.GetMethod(),
		})
	case *eventsv1.UserStreamEvent_UserLoggedIn:
		return m.event("user_login", "auth", ev.GetOccurredAt(), p.UserLoggedIn.GetAccountId(), "", nil)
	case *eventsv1.UserStreamEvent_UserGuestConverted:
		return m.event("guest_converted", "auth", ev.GetOccurredAt(), p.UserGuestConverted.GetAccountId(), "", nil)
	case *eventsv1.UserStreamEvent_ProfileCreated:
		return m.event("profile_created", "user", ev.GetOccurredAt(), p.ProfileCreated.GetAccountId(), p.ProfileCreated.GetProfileId(), nil)
	case *eventsv1.UserStreamEvent_ProfileSwitched:
		return m.event("profile_switched", "user", ev.GetOccurredAt(), p.ProfileSwitched.GetAccountId(), p.ProfileSwitched.GetProfileId(), nil)
	default:
		return nil
	}
}

func (m Mapper) FromMessage(ev *eventsv1.MessageStreamEvent) *analyticsv1.AnalyticsEvent {
	if ev == nil {
		return nil
	}
	switch p := ev.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		return m.event("message_sent", "messaging", ev.GetOccurredAt(), "", p.MessageSent.GetSenderProfileId(), map[string]any{
			"chat_id": p.MessageSent.GetChatId(), "message_id": p.MessageSent.GetMessageId(),
		})
	case *eventsv1.MessageStreamEvent_ReactionAdded:
		return m.event("reaction_added", "messaging", ev.GetOccurredAt(), "", p.ReactionAdded.GetProfileId(), map[string]any{
			"chat_id": p.ReactionAdded.GetChatId(),
		})
	default:
		return nil
	}
}

func (m Mapper) FromChat(ev *eventsv1.ChatStreamEvent) *analyticsv1.AnalyticsEvent {
	if ev == nil {
		return nil
	}
	switch p := ev.GetPayload().(type) {
	case *eventsv1.ChatStreamEvent_ChatCreated:
		return m.event("chat_created", "chat", ev.GetOccurredAt(), "", "", map[string]any{
			"chat_id": p.ChatCreated.GetChatId(), "type": p.ChatCreated.GetType(),
		})
	case *eventsv1.ChatStreamEvent_ChatMemberChanged:
		if p.ChatMemberChanged.GetChange() != "joined" {
			return nil
		}
		return m.event("space_joined", "space", ev.GetOccurredAt(), "", p.ChatMemberChanged.GetProfileId(), map[string]any{
			"chat_id": p.ChatMemberChanged.GetChatId(),
		})
	case *eventsv1.ChatStreamEvent_SpaceCreated:
		return m.event("space_created", "space", ev.GetOccurredAt(), "", p.SpaceCreated.GetOwnerProfileId(), map[string]any{
			"space_id": p.SpaceCreated.GetSpaceId(),
		})
	default:
		return nil
	}
}

func (m Mapper) FromMatchmaking(ev *eventsv1.MatchmakingStreamEvent) *analyticsv1.AnalyticsEvent {
	if ev == nil {
		return nil
	}
	switch p := ev.GetPayload().(type) {
	case *eventsv1.MatchmakingStreamEvent_SearchStarted:
		return m.event("mm_search_started", "matchmaking", ev.GetOccurredAt(), "", p.SearchStarted.GetProfileId(), nil)
	case *eventsv1.MatchmakingStreamEvent_MatchFound:
		return m.event("mm_match_found", "matchmaking", ev.GetOccurredAt(), "", "", map[string]any{
			"match_id": p.MatchFound.GetMatchId(),
		})
	default:
		return nil
	}
}

func (m Mapper) FromVoice(ev *eventsv1.VoiceStreamEvent) *analyticsv1.AnalyticsEvent {
	if ev == nil {
		return nil
	}
	switch p := ev.GetPayload().(type) {
	case *eventsv1.VoiceStreamEvent_CallStarted:
		return m.event("call_started", "voice", ev.GetOccurredAt(), "", p.CallStarted.GetInitiatorProfileId(), map[string]any{
			"chat_id": p.CallStarted.GetChatId(),
		})
	case *eventsv1.VoiceStreamEvent_CallEnded:
		return m.event("call_ended", "voice", ev.GetOccurredAt(), "", "", map[string]any{
			"room_id": p.CallEnded.GetRoomId(),
		})
	case *eventsv1.VoiceStreamEvent_ScreenShareStarted:
		return m.event("screen_share_started", "voice", ev.GetOccurredAt(), "", p.ScreenShareStarted.GetProfileId(), nil)
	default:
		return nil
	}
}

func (m Mapper) FromStory(ev *eventsv1.StoryStreamEvent) *analyticsv1.AnalyticsEvent {
	if ev == nil {
		return nil
	}
	switch p := ev.GetPayload().(type) {
	case *eventsv1.StoryStreamEvent_StoryCreated:
		return m.event("story_created", "story", ev.GetOccurredAt(), "", p.StoryCreated.GetAuthorProfileId(), nil)
	case *eventsv1.StoryStreamEvent_StoryViewed:
		return m.event("story_viewed", "story", ev.GetOccurredAt(), "", p.StoryViewed.GetViewerProfileId(), nil)
	default:
		return nil
	}
}

func (m Mapper) FromBot(ev *eventsv1.BotStreamEvent) *analyticsv1.AnalyticsEvent {
	if ev == nil {
		return nil
	}
	switch p := ev.GetPayload().(type) {
	case *eventsv1.BotStreamEvent_CommandExecuted:
		return m.event("bot_command_executed", "bot", ev.GetOccurredAt(), "", "", map[string]any{
			"command": p.CommandExecuted.GetCommand(),
		})
	default:
		return nil
	}
}

func (m Mapper) FromAnalyticsSubject(subject string, data []byte) *analyticsv1.AnalyticsEvent {
	subject = strings.TrimSpace(subject)
	if !strings.HasPrefix(subject, "analytics.") {
		return nil
	}
	var ev analyticsv1.AnalyticsEvent
	if err := protojson.Unmarshal(data, &ev); err == nil && ev.GetEventId() != "" {
		return &ev
	}
	parts := strings.Split(subject, ".")
	eventType := parts[len(parts)-1]
	if len(parts) >= 3 {
		eventType = parts[1] + "_" + parts[2]
	}
	return m.event(eventType, parts[1], timestamppb.New(time.Now().UTC()), "", "", map[string]any{"raw": string(data)})
}
