package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	chatv1 "voice.app/voice/chat/v1"
	searchv1 "voice.app/voice/search/v1"
)

type entitlementChatClient struct {
	chatv1.ChatServiceClient
	allowed bool
	seen    int
	caller  string
	cutoff  time.Time
	times   []time.Time
}

func (c *entitlementChatClient) CheckMessageReadEntitlement(ctx context.Context, req *chatv1.CheckMessageReadEntitlementRequest, _ ...grpc.CallOption) (*chatv1.CheckMessageReadEntitlementResponse, error) {
	c.seen++
	createdAt := req.GetMessageCreatedAt().AsTime()
	c.times = append(c.times, createdAt)
	md, _ := metadata.FromOutgoingContext(ctx)
	callers := md.Get("x-voice-internal-caller")
	if len(callers) == 1 {
		c.caller = callers[0]
	}
	return &chatv1.CheckMessageReadEntitlementResponse{Entitled: c.allowed && !createdAt.Before(c.cutoff)}, nil
}

func TestFilterManagedHistoryHitsChecksChatAtMessageTimestamp(t *testing.T) {
	client := &entitlementChatClient{}
	service := &SearchGRPC{ChatEntitlement: client}
	viewer, chatID := uuid.New(), uuid.New()
	hits := []MessageHit{
		{MessageID: uuid.New(), ChatID: chatID, CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		{MessageID: uuid.New(), ChatID: chatID, CreatedAt: time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)},
	}
	client.allowed = false
	got, err := service.filterManagedHistoryHits(context.Background(), viewer, hits)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Equal(t, 2, client.seen)
	require.Equal(t, "search", client.caller)
	client.allowed = true
	got, err = service.filterManagedHistoryHits(context.Background(), viewer, hits[:1])
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestFilterManagedHistoryHitsFailsClosedForMissingTimestamp(t *testing.T) {
	service := &SearchGRPC{ChatEntitlement: &entitlementChatClient{allowed: true}}
	viewer, chatID := uuid.New(), uuid.New()
	_, err := service.filterManagedHistoryHits(context.Background(), viewer, []MessageHit{{
		MessageID: uuid.New(), ChatID: chatID, Snippet: "must not bypass Chat authorization",
	}})
	require.Error(t, err, "a missing immutable message timestamp cannot bypass Chat's interval decision")
}

func TestSearchInChatFiltersQuotedSnippetByMessageTimeEntitlement(t *testing.T) {
	cutoff := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	chatID, viewer := uuid.New(), uuid.New()
	beforeJoin, afterJoin := cutoff.Add(-time.Nanosecond), cutoff
	messages := &stubMessageSearch{inChatHits: []MessageHit{
		{MessageID: uuid.New(), ChatID: chatID, CreatedAt: beforeJoin, Snippet: "quoted private text"},
		{MessageID: uuid.New(), ChatID: chatID, CreatedAt: afterJoin, Snippet: "quoted available text"},
	}}
	chat := &entitlementChatClient{allowed: true, cutoff: cutoff}
	client := startSearchGRPCTestServer(t, &SearchGRPC{Messages: messages, ChatEntitlement: chat})
	response, err := client.SearchInChat(ctxWithProfile(viewer), &searchv1.SearchInChatRequest{
		Chat:  &chatv1.ChatRef{Id: chatID.String()},
		Query: "quoted",
	})
	require.NoError(t, err)
	hits := response.GetSearchResults().GetHits()
	require.Len(t, hits, 1)
	require.Equal(t, "quoted available text", hits[0].GetSnippet())
	require.Equal(t, []time.Time{beforeJoin, afterJoin}, chat.times)
	require.Equal(t, 2, chat.seen)
}
