package main

import (
	"context"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
)

type chatPermissionRecorder struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.SetChatMemberPostAsChatRequest
	read *chatv1.GetChatPermissionsRequest
	err  error
}

func (s *chatPermissionRecorder) GetChatPermissions(_ context.Context, r *chatv1.GetChatPermissionsRequest) (*chatv1.GetChatPermissionsResponse, error) {
	s.read = r
	return &chatv1.GetChatPermissionsResponse{CanAddMembers: true}, s.err
}
func (s *chatPermissionRecorder) SetChatMemberPostAsChat(_ context.Context, r *chatv1.SetChatMemberPostAsChatRequest) (*chatv1.SetChatMemberPostAsChatResponse, error) {
	s.last = r
	return &chatv1.SetChatMemberPostAsChatResponse{}, s.err
}

func TestTranscodeChatPermissionsPathBindsResourceAndTarget(t *testing.T) {
	for _, deny := range []bool{false, true} {
		t.Run(map[bool]string{false: "allow", true: "deny"}[deny], func(t *testing.T) {
			rec := &chatPermissionRecorder{}
			if deny {
				rec.err = status.Error(codes.PermissionDenied, "denied")
			}
			conn, cleanup := startBufconnChatConn(t, rec)
			t.Cleanup(cleanup)
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims: map[string]tokenClaims{"token": {UserID: "account-1", ProfileID: "profile-1"}},
				transcoder:  &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
			})
			headers := map[string]string{"Authorization": "Bearer token"}
			resp := performRequest(h, http.MethodGet, "/api/v1/chats/chat-a/permissions", "", headers)
			want := http.StatusOK
			if deny {
				want = http.StatusForbidden
			}
			if resp.Code != want {
				t.Fatalf("read status=%d want=%d body=%s", resp.Code, want, resp.Body.String())
			}
			if rec.read == nil || rec.read.GetChatId() != "chat-a" {
				t.Fatalf("read=%+v", rec.read)
			}
			resp = performRequest(h, http.MethodPut, "/api/v1/chats/chat-a/members/member-a/post-as-chat",
				`{"chat_id":"forged-chat","target_profile_id":"forged-member","allowed":true}`, headers)
			want = http.StatusNoContent
			if deny {
				want = http.StatusForbidden
			}
			if resp.Code != want {
				t.Fatalf("write status=%d want=%d body=%s", resp.Code, want, resp.Body.String())
			}
			if rec.last == nil || rec.last.GetChatId() != "chat-a" || rec.last.GetTargetProfileId() != "member-a" || !rec.last.GetAllowed() {
				t.Fatalf("write=%+v", rec.last)
			}
		})
	}
}
