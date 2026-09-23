package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	chatv1 "voice.app/voice/chat/v1"
)

func TestSubscribeRoleEvents_DoesNotRouteVoiceOrUnknownThroughChatID(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: jsStreamRoleEvents, Subjects: []string{"role.>"}, Retention: nats.LimitsPolicy})
	require.NoError(t, err)
	preprovisionRealtimeConsumer(t, js, jsStreamRoleEvents, roleConsumerDurableName("role-routing-boundary"), "role.>")
	hub := newWSHub()
	allowedChat, smuggledChat := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	allowed := hub.attachConn("inst", "allowed", "profile-a", 8)
	require.True(t, hub.addChat(allowed, allowedChat))
	smuggled := hub.attachConn("inst", "smuggled", "profile-b", 16)
	require.True(t, hub.addChat(smuggled, smuggledChat))
	sub, err := subscribeRoleEvents(js, hub, "role-routing-boundary", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	publish := func(subject string, p roleEventJSON) {
		b, marshalErr := json.Marshal(p)
		require.NoError(t, marshalErr)
		_, publishErr := js.Publish(subject, b)
		require.NoError(t, publishErr)
	}
	p := roleEventJSON{SpaceID: "33333333-3333-3333-3333-333333333333", VoiceRoomID: "44444444-4444-4444-4444-444444444444", ChatID: smuggledChat, RoleID: "55555555-5555-5555-5555-555555555555"}
	for _, subject := range []string{"role.voice_override_set", "role.voice_override_removed", "role.created", "role.updated", "role.deleted", "role.future_unknown", "role.future.role.chat_override_set"} {
		publish(subject, p)
	}
	// This final valid event is an ordered barrier on the same serial consumer.
	publish("role.chat_override_removed", roleEventJSON{SpaceID: p.SpaceID, ChatID: allowedChat, RoleID: p.RoleID})
	select {
	case got := <-allowed.fanout:
		require.Equal(t, "role_update", got.Op)
		require.Contains(t, string(got.D), "role.chat_override_removed")
	case <-time.After(5 * time.Second):
		t.Fatal("valid chat override not delivered")
	}
	select {
	case got := <-smuggled.fanout:
		t.Fatalf("non-chat role event leaked via chat_id: %+v", got)
	default:
	}
}

type boundaryMemberClient struct {
	chatv1.ChatServiceClient
	pages map[string]*chatv1.ListMembersResponse
	calls []string
	wait  bool
}

func (s *boundaryMemberClient) ListMembers(ctx context.Context, r *chatv1.ListMembersRequest, _ ...grpc.CallOption) (*chatv1.ListMembersResponse, error) {
	cursor := r.GetPage().GetCursor()
	s.calls = append(s.calls, cursor)
	if len(s.calls) > 8 {
		return nil, fmt.Errorf("test safety limit: pagination did not terminate")
	}
	if s.wait {
		if _, ok := ctx.Deadline(); !ok {
			return nil, fmt.Errorf("missing bounded deadline")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return s.pages[cursor], nil
}

func boundaryMemberPage(next string, ids ...string) *chatv1.ListMembersResponse {
	members := make([]*chatv1.ChatMember, 0, len(ids))
	for _, id := range ids {
		members = append(members, &chatv1.ChatMember{ProfileId: id})
	}
	return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{Members: members, NextCursor: next}}
}

func TestRecipientDeliveryStatesRejectsMalformedPartialSnapshot(t *testing.T) {
	for name, pages := range map[string]map[string]*chatv1.ListMembersResponse{
		"missing response":    {"": boundaryMemberPage("next", "member-a"), "next": nil},
		"missing member list": {"": boundaryMemberPage("next", "member-a"), "next": {}},
		"same cursor":         {"": boundaryMemberPage("next", "member-a"), "next": boundaryMemberPage("next", "member-b")},
		"cursor cycle":        {"": boundaryMemberPage("a", "member-a"), "a": boundaryMemberPage("b", "member-b"), "b": boundaryMemberPage("a", "member-c")},
		"duplicate member":    {"": boundaryMemberPage("next", "member-a"), "next": boundaryMemberPage("", "member-a")},
		"blank member":        {"": boundaryMemberPage("", "member-a", " ")},
		"nil member":          {"": {MemberList: &chatv1.MemberList{Members: []*chatv1.ChatMember{nil}}}},
	} {
		t.Run(name, func(t *testing.T) {
			client := &boundaryMemberClient{pages: pages}
			got, err := (&grpcChatMemberInboxLister{client: client}).RecipientDeliveryStates(context.Background(), "chat")
			require.Error(t, err)
			require.Nil(t, got)
			require.LessOrEqual(t, len(client.calls), 3, "must stop on malformed page, not test safety limit")
		})
	}
}

func TestRecipientDeliveryStatesAllowsEmptyTerminalPage(t *testing.T) {
	client := &boundaryMemberClient{pages: map[string]*chatv1.ListMembersResponse{"": boundaryMemberPage("")}}
	got, err := (&grpcChatMemberInboxLister{client: client}).RecipientDeliveryStates(context.Background(), "chat")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestRecipientDeliveryStatesAlwaysBoundsLookup(t *testing.T) {
	client := &boundaryMemberClient{wait: true}
	started := time.Now()
	got, err := (&grpcChatMemberInboxLister{client: client}).RecipientDeliveryStates(context.Background(), "chat")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, got)
	require.Less(t, time.Since(started), 5*time.Second)
}
