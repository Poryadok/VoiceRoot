package principalgrpc

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

func admissionContext(t *testing.T, req proto.Message, method string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: method, RequestID: uuid.NewString(), RequestHash: hash})
}

func TestMemberAdmissionRequiresExactProtectedSpaceBinding(t *testing.T) {
	for _, kind := range []string{"ensure", "remove"} {
		t.Run(kind, func(t *testing.T) {
			var req proto.Message
			method := rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName
			if kind == "ensure" {
				req = &rolev1.EnsureDefaultMemberRoleRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), MembershipGeneration: 1, AdmissionOperationId: uuid.NewString(), AdmissionBindingSha256: make([]byte, 32)}
			} else {
				method = rolev1.RoleService_RemoveMemberRoles_FullMethodName
				req = &rolev1.RemoveMemberRolesRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), RemovalGenerationFloor: 1, RemovalOperationId: uuid.NewString(), RemovalBindingSha256: make([]byte, 32)}
			}
			ctx := admissionContext(t, req, method)
			require.NoError(t, RequireMemberAdmission(ctx, req, method))
			require.Equal(t, codes.Unauthenticated, status.Code(RequireMemberAdmission(context.Background(), req, method)))
			p, _ := principal.FromContext(ctx)
			for name, mutate := range map[string]func(*principal.Principal){
				"caller":   func(p *principal.Principal) { p.Issuer = "gateway"; p.Subject = "service:gateway" },
				"kind":     func(p *principal.Principal) { p.Kind = "delegated_user" },
				"audience": func(p *principal.Principal) { p.Audience = "space" },
				"method":   func(p *principal.Principal) { p.RPC = rolev1.RoleService_CheckPermission_FullMethodName },
				"hash":     func(p *principal.Principal) { p.RequestHash = "sha256:wrong" },
				"request":  func(p *principal.Principal) { p.RequestID = "" },
				"user":     func(p *principal.Principal) { p.ProfileID = uuid.NewString() },
				"bot":      func(p *principal.Principal) { p.BotID = uuid.NewString() },
			} {
				t.Run(name, func(t *testing.T) {
					changed := p
					mutate(&changed)
					require.Equal(t, codes.PermissionDenied, status.Code(RequireMemberAdmission(principal.WithVerified(context.Background(), changed), req, method)))
				})
			}
			unknown := proto.Clone(req)
			unknown.ProtoReflect().SetUnknown([]byte{0x78, 0x01})
			require.Equal(t, codes.InvalidArgument, status.Code(RequireMemberAdmission(admissionContext(t, unknown, method), unknown, method)))
		})
	}
}
