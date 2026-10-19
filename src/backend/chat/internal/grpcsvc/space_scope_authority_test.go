package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type scopeAuthorityStub struct {
	calls int
	err   error
}

func (s *scopeAuthorityStub) GetSpaceChatScopeAuthority(_ context.Context, chat, space uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	s.calls++
	return &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), ActualSpaceId: space.String(), Exists: true, Live: true, ChatAuthorityRevision: 3, ChatScopeIncarnation: uuid.NewString(), LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY}, s.err
}

func TestSpaceScopeAuthorityBoundaryBindsExactCallerAndBody(t *testing.T) {
	request := &chatv1.GetSpaceChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: uuid.NewString(), ExpectedSpaceId: uuid.NewString(), OperationId: uuid.NewString()}
	st := &scopeAuthorityStub{}
	service := &SpaceChatScopeAuthorityGRPC{Store: st}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "role", Subject: "service:role", Audience: "chat", RPC: chatv1.ChatScopeAuthorityService_GetSpaceChatScopeAuthority_FullMethodName, RequestID: request.OperationId, RequestHash: hash}
	for _, issuer := range []string{"role", "space"} {
		current := p
		current.Issuer = issuer
		current.Subject = "service:" + issuer
		response, err := service.GetSpaceChatScopeAuthority(principal.WithVerified(context.Background(), current), request)
		require.NoError(t, err)
		require.Equal(t, request.OperationId, response.OperationId)
	}
	before := st.calls
	for name, mutate := range map[string]func(*principal.Principal){
		"caller":   func(p *principal.Principal) { p.Issuer = "search"; p.Subject = "service:search" },
		"subject":  func(p *principal.Principal) { p.Subject = "service:space" },
		"audience": func(p *principal.Principal) { p.Audience = "role" },
		"method":   func(p *principal.Principal) { p.RPC = "/voice.chat.v1.ChatService/GetChat" },
		"hash":     func(p *principal.Principal) { p.RequestHash = "sha256:wrong" },
		"request":  func(p *principal.Principal) { p.RequestID = uuid.NewString() },
		"actor":    func(p *principal.Principal) { p.ProfileID = uuid.NewString() },
	} {
		t.Run(name, func(t *testing.T) {
			current := p
			mutate(&current)
			_, err := service.GetSpaceChatScopeAuthority(principal.WithVerified(context.Background(), current), request)
			require.Error(t, err)
		})
	}
	_, err = service.GetSpaceChatScopeAuthority(context.Background(), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, mutate := range []func(*chatv1.GetSpaceChatScopeAuthorityRequest){func(r *chatv1.GetSpaceChatScopeAuthorityRequest) { r.ProtocolVersion = 2 }, func(r *chatv1.GetSpaceChatScopeAuthorityRequest) { r.ChatId = "bad" }, func(r *chatv1.GetSpaceChatScopeAuthorityRequest) { r.ExpectedSpaceId = uuid.Nil.String() }, func(r *chatv1.GetSpaceChatScopeAuthorityRequest) { r.OperationId = "bad" }, func(r *chatv1.GetSpaceChatScopeAuthorityRequest) { r.ProtoReflect().SetUnknown([]byte{0x78, 1}) }} {
		changed := proto.Clone(request).(*chatv1.GetSpaceChatScopeAuthorityRequest)
		mutate(changed)
		_, err = service.GetSpaceChatScopeAuthority(principal.WithVerified(context.Background(), p), changed)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	require.Equal(t, before, st.calls, "invalid callers/body must not reach the store")
	for _, test := range []struct {
		err  error
		code codes.Code
	}{{pgx.ErrNoRows, codes.NotFound}, {errors.New("private database detail"), codes.Unavailable}} {
		st.err = test.err
		_, err = service.GetSpaceChatScopeAuthority(principal.WithVerified(context.Background(), p), request)
		require.Equal(t, test.code, status.Code(err))
		require.NotContains(t, err.Error(), "private database detail")
	}
}
