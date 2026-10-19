package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/messageevents"
)

func TestForwardCompoundRejectsNestedOrDifferentActorCommentary(t *testing.T) {
	row := MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: uuid.New(), ChatType: "group"}
	c := MessageRow{ID: uuid.New(), ChatID: row.ChatID, SenderProfileID: row.SenderProfileID, ChatType: row.ChatType, Content: "commentary", Type: "regular", ContentType: "text", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	row.ForwardCommentary = &c
	require.NoError(t, validateForwardCommentary(row))
	c.SenderProfileID = uuid.New()
	require.Error(t, validateForwardCommentary(row))
	c.SenderProfileID = row.SenderProfileID
	c.ForwardCommentary = &MessageRow{}
	require.Error(t, validateForwardCommentary(row))
}

func TestForwardCompoundCommentaryRequiresParentAttribution(t *testing.T) {
	for _, postedAsChat := range []bool{false, true} {
		name := "profile"
		if postedAsChat {
			name = "channel"
		}
		t.Run(name, func(t *testing.T) {
			row := MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: uuid.New(), ChatType: "channel", PostedAsChat: postedAsChat}
			if postedAsChat {
				row.DisplayChatID = &row.ChatID
			}
			commentary := MessageRow{ID: uuid.New(), ChatID: row.ChatID, SenderProfileID: row.SenderProfileID, ChatType: row.ChatType, Content: "commentary", Type: "regular", ContentType: "text", AttachmentsJSON: "[]", MentionsJSON: "[]", PostedAsChat: postedAsChat, DisplayChatID: row.DisplayChatID}
			row.ForwardCommentary = &commentary
			require.NoError(t, validateForwardCommentary(row))
			commentary.PostedAsChat = !postedAsChat
			require.Error(t, validateForwardCommentary(row), "companion cannot change publication identity")
			commentary.PostedAsChat = postedAsChat
			otherChat := uuid.New()
			commentary.DisplayChatID = &otherChat
			require.Error(t, validateForwardCommentary(row), "companion cannot display another chat")
			if postedAsChat {
				commentary.DisplayChatID = nil
				require.Error(t, validateForwardCommentary(row), "official commentary must retain destination display")
			}
		})
	}
}

func TestForwardCompoundExpiredAfterAcquireStaysInvisibleAndReleasesExactRefs(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL attachment recovery")
	}
	st, row, request := attachmentIntentFixture(t)
	row.ClientMessageID = nil
	c := MessageRow{ID: uuid.New(), ChatID: row.ChatID, SenderProfileID: row.SenderProfileID, ChatType: row.ChatType, Content: "commentary", Type: "regular", ContentType: "text", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	row.ForwardCommentary = &c
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var owner string
	_, err := st.InsertMessageWithReferencesAndOutbox(ctx, row, nil, request, func(_ context.Context, r *filev1.AcquireFileReferencesRequest) error {
		owner = r.References[0].OwnerId
		cancel()
		return nil
	}, nil)
	require.Error(t, err)
	var messages, events int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM message_event_outbox)`).Scan(&messages, &events))
	require.Equal(t, []int{0, 0}, []int{messages, events})
	require.Equal(t, row.ID.String(), owner)
	_, err = st.Pool.Exec(context.Background(), `UPDATE messaging_attachment_send_intents SET expires_at=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	released := 0
	require.NoError(t, st.ReconcileAttachmentIntents(context.Background(), func(_ context.Context, r *filev1.ReleaseFileReferencesRequest) error {
		released++
		require.Len(t, r.References, 1)
		require.Equal(t, owner, r.References[0].OwnerId)
		return nil
	}))
	require.Equal(t, 1, released)
}

func TestForwardCompoundRollsBackCommentaryOnOutboxFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL compound commit")
	}
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	st := &MessagesStore{Pool: pool}
	row := MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: uuid.New(), ChatType: "group", Type: "forward", Content: "forward", ContentType: "text", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	commentary := row
	commentary.ID, commentary.Type, commentary.Content = uuid.New(), "regular", "commentary"
	row.ForwardCommentary = &commentary
	first, err := messageevents.NewMessageSentOutbox(commentary.ID.String(), row.ChatID.String(), row.SenderProfileID.String(), false, "", false, "text", false)
	require.NoError(t, err)
	invalid := first
	invalid.MessageID = uuid.New()
	_, _, err = st.InsertMessageWithOutbox(ctx, row, nil, []messageevents.OutboxEvent{first, invalid})
	require.Error(t, err)
	var messages, events int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM message_event_outbox)`).Scan(&messages, &events))
	require.Equal(t, []int{0, 0}, []int{messages, events})
	second, err := messageevents.NewMessageSentOutbox(row.ID.String(), row.ChatID.String(), row.SenderProfileID.String(), false, "", false, "text", false)
	require.NoError(t, err)
	_, _, err = st.InsertMessageWithOutbox(ctx, row, nil, []messageevents.OutboxEvent{first, second})
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM message_event_outbox)`).Scan(&messages, &events))
	require.Equal(t, []int{2, 2}, []int{messages, events})
}
