package grpcsvc

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

func TestGameCardReadProjectionKeepsActionsDisabledAndForwardProjectionStripsCard(t *testing.T) {
	card := `{"schema_version":1,"revision":"1","title":"Relic found","safe_summary":"A relic was found","facts":[{"label":"Location","value":"North gate"}],"actions":[{"action_id":"00000000-0000-4000-8000-000000000020","action_type":"inspect","label":"Inspect","arguments_json":"{\"target\":\"relic\"}"}],"media_reference_ids":["00000000-0000-4000-8000-000000000030"]}`
	appID, envID, installID, botID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	row := &store.MessageRow{ID: uuid.New(), ChatID: uuid.New(), ChatType: "group", SenderProfileID: uuid.New(),
		Content: "A creature was discovered.", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
		GameCardJSON: &card, GameAppID: &appID, GameEnvironmentID: &envID, GameInstallationID: &installID, GameBotID: &botID}
	message := messageRowToProto(row, messagingv1.MessageKind_MESSAGE_KIND_UNSPECIFIED, "[]", false)
	require.Equal(t, messagingv1.MessageKind_MESSAGE_KIND_GAME_CARD, message.GetMessageKind())
	require.Equal(t, "A creature was discovered.", message.GetContent(), "fallback text remains the ordinary message content")
	require.Equal(t, "Relic found", message.GetGameCard().GetTitle())
	require.False(t, message.GetGameCardActionsEnabled())
	require.Equal(t, []string{"00000000-0000-4000-8000-000000000030"}, message.GetGameCard().GetMediaReferenceIds())

	// ForwardMessage constructs a new MessageRow from source.Content and safe
	// attachments only; it deliberately does not copy the game-card sidecar.
	forward := &store.MessageRow{ID: uuid.New(), ChatID: uuid.New(), ChatType: "group", SenderProfileID: uuid.New(),
		Content: row.Content, Type: "forward", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	forwarded := messageRowToProto(forward, messagingv1.MessageKind_MESSAGE_KIND_FORWARD, "[]", false)
	require.Nil(t, forwarded.GetGameCard())
	require.Empty(t, forwarded.GetGameAppId())
	require.False(t, forwarded.GetGameCardActionsEnabled())
}
