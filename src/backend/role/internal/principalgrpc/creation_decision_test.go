package principalgrpc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestCreationDecision_OnlyExactSpaceSubjectPermission(t *testing.T) {
	for _, change := range []string{"valid", "issuer", "kind", "account", "hash", "request", "permission", "chat", "voice", "subject", "space", "actor"} {
		t.Run(change, func(t *testing.T) {
			req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_CREATE_IN_SPACE"}
			p := principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: "operation", RequestHash: "hash"}
			switch change {
			case "issuer":
				p.Issuer = "gateway"
			case "kind":
				p.Kind = "delegated_user"
			case "account":
				p.AccountID = uuid.NewString()
			case "hash":
				p.RequestHash = "other"
			case "request":
				p.RequestID = "other"
			case "permission":
				req.PermissionName = "SPACE_MANAGE_ROLES"
			case "chat":
				req.Chat = &chatv1.ChatRef{}
			case "voice":
				empty := ""
				req.VoiceRoomId = &empty
			case "subject":
				p.Subject = "service:bot"
			case "space":
				req.SpaceId = " " + req.SpaceId
			case "actor":
				req.ProfileId = uuid.Nil.String()
			}
			err := validateCreationDecision(p, req, "operation", "hash")
			if (err == nil) != (change == "valid") {
				t.Fatalf("%s accepted=%v", change, err == nil)
			}
		})
	}
}

func TestCreationDecision_StrictBoundaryStopsScopeExpansionBeforeHandler(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer space", "x-request-id", "operation"))
	interceptor := StrictUnaryInterceptor(verifierFunc(func(_ context.Context, _ string, method, requestID, hash string) (principal.Principal, error) {
		return principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: method, RequestID: requestID, RequestHash: hash}, nil
	}))
	req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_CREATE_IN_SPACE"}
	info := &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}
	calls := 0
	handler := func(ctx context.Context, _ any) (any, error) {
		calls++
		if _, ok := principal.FromContext(ctx); !ok {
			t.Fatal("verified context missing")
		}
		return nil, nil
	}
	if _, err := interceptor(ctx, req, info, handler); err != nil {
		t.Fatal(err)
	}
	req.PermissionName = "SPACE_MANAGE_ROLES"
	if _, err := interceptor(ctx, req, info, handler); err == nil {
		t.Fatal("expanded permission reached handler")
	}
	if calls != 1 {
		t.Fatalf("handler calls=%d", calls)
	}
}
