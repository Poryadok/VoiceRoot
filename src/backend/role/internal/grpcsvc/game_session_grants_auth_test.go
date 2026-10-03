package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type gameSessionGrantCall struct {
	name      string
	issuer    string
	subject   string
	rpc       string
	request   any
	requestID string
	invoke    func(context.Context, any) error
}

func gameSessionGrantCalls(svc *RoleGRPC) []gameSessionGrantCall {
	apply := &rolev1.ApplyGameSessionGrantsRequest{
		ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(), SessionId: uuid.NewString(),
		VoiceRoomId: uuid.NewString(), OperationId: uuid.NewString(), RosterRevision: 1,
		ProfileIds: []string{uuid.NewString()},
	}
	revoke := &rolev1.RevokeGameSessionGrantsRequest{
		ApplicationId: apply.ApplicationId, EnvironmentId: apply.EnvironmentId, SessionId: apply.SessionId,
		OperationId: uuid.NewString(),
	}
	check := &rolev1.CheckGameSessionGrantRequest{
		ApplicationId: apply.ApplicationId, EnvironmentId: apply.EnvironmentId, SessionId: apply.SessionId,
		VoiceRoomId: apply.VoiceRoomId, ProfileId: apply.ProfileIds[0],
	}
	return []gameSessionGrantCall{
		{
			name: "apply", issuer: "gameintegration", subject: "service:gameintegration",
			rpc: rolev1.RoleService_ApplyGameSessionGrants_FullMethodName, request: apply,
			requestID: apply.OperationId,
			invoke: func(ctx context.Context, request any) error {
				_, err := svc.ApplyGameSessionGrants(ctx, request.(*rolev1.ApplyGameSessionGrantsRequest))
				return err
			},
		},
		{
			name: "revoke", issuer: "gameintegration", subject: "service:gameintegration",
			rpc: rolev1.RoleService_RevokeGameSessionGrants_FullMethodName, request: revoke,
			requestID: revoke.OperationId,
			invoke: func(ctx context.Context, request any) error {
				_, err := svc.RevokeGameSessionGrants(ctx, request.(*rolev1.RevokeGameSessionGrantsRequest))
				return err
			},
		},
		{
			name: "check", issuer: "voice", subject: "service:voice",
			rpc: rolev1.RoleService_CheckGameSessionGrant_FullMethodName, request: check,
			requestID: uuid.NewString(),
			invoke: func(ctx context.Context, request any) error {
				_, err := svc.CheckGameSessionGrant(ctx, request.(*rolev1.CheckGameSessionGrantRequest))
				return err
			},
		},
	}
}

func gameSessionGrantPrincipal(t *testing.T, call gameSessionGrantCall, request any) principal.Principal {
	t.Helper()
	hash, err := principal.RequestHash(request.(proto.Message))
	require.NoError(t, err)
	return principal.Principal{
		Kind: "service", Issuer: call.issuer, Subject: call.subject, Audience: "role",
		RPC: call.rpc, RequestID: call.requestID, RequestHash: hash,
	}
}

func TestGameSessionGrantHandlersRequireExactVerifiedCallerAndRequestBinding(t *testing.T) {
	svc := &RoleGRPC{}
	for _, call := range gameSessionGrantCalls(svc) {
		t.Run(call.name, func(t *testing.T) {
			require.Equal(t, codes.PermissionDenied, status.Code(call.invoke(context.Background(), call.request)))
			verified := gameSessionGrantPrincipal(t, call, call.request)
			require.Equal(t, codes.Unavailable, status.Code(call.invoke(principal.WithVerified(context.Background(), verified), call.request)), "valid identity reaches the dependency boundary")

			for _, mutate := range []struct {
				name   string
				change func(*principal.Principal)
			}{
				{"principal kind", func(p *principal.Principal) { p.Kind = "delegated_user" }},
				{"issuer", func(p *principal.Principal) { p.Issuer = "gateway" }},
				{"subject", func(p *principal.Principal) { p.Subject = "service:space" }},
				{"audience", func(p *principal.Principal) { p.Audience = "voice" }},
				{"exact rpc", func(p *principal.Principal) { p.RPC = "/voice.role.v1.RoleService/ListRoles" }},
				{"request hash", func(p *principal.Principal) { p.RequestHash = "sha256:invalid" }},
			} {
				t.Run(mutate.name, func(t *testing.T) {
					invalid := verified
					mutate.change(&invalid)
					require.Equal(t, codes.PermissionDenied, status.Code(call.invoke(principal.WithVerified(context.Background(), invalid), call.request)))
				})
			}
			if call.name != "check" {
				invalid := verified
				invalid.RequestID = uuid.NewString()
				require.Equal(t, codes.PermissionDenied, status.Code(call.invoke(principal.WithVerified(context.Background(), invalid), call.request)), "Apply/Revoke request ID must equal operation_id")
			}
		})
	}
}

func TestGameSessionGrantHandlersRejectMalformedScopeBeforeStore(t *testing.T) {
	svc := &RoleGRPC{}
	for _, call := range gameSessionGrantCalls(svc) {
		t.Run(call.name, func(t *testing.T) {
			for _, mutate := range []struct {
				name   string
				change func(any)
			}{
				{"application id", func(req any) {
					switch r := req.(type) {
					case *rolev1.ApplyGameSessionGrantsRequest:
						r.ApplicationId = "bad"
					case *rolev1.RevokeGameSessionGrantsRequest:
						r.ApplicationId = "bad"
					case *rolev1.CheckGameSessionGrantRequest:
						r.ApplicationId = "bad"
					}
				}},
				{"session id", func(req any) {
					switch r := req.(type) {
					case *rolev1.ApplyGameSessionGrantsRequest:
						r.SessionId = "bad"
					case *rolev1.RevokeGameSessionGrantsRequest:
						r.SessionId = "bad"
					case *rolev1.CheckGameSessionGrantRequest:
						r.SessionId = "bad"
					}
				}},
			} {
				t.Run(mutate.name, func(t *testing.T) {
					request := proto.Clone(call.request.(proto.Message))
					mutate.change(request)
					verified := gameSessionGrantPrincipal(t, call, request)
					ctx := principal.WithVerified(context.Background(), verified)
					require.Equal(t, codes.InvalidArgument, status.Code(call.invoke(ctx, request)))
				})
			}
		})
	}
}
