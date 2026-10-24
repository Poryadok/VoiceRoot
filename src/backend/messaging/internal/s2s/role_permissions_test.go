package s2s

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type roleDecisionClient struct {
	rolev1.RoleServiceClient
	check func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)
}

func (c roleDecisionClient) CheckPermission(ctx context.Context, req *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return c.check(ctx, req)
}

func TestRolePermissionsSignsExactRequestAndReplacesMetadata(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "test", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	space, profile, chat := uuid.New(), uuid.New(), uuid.New()
	seen := map[string]bool{}
	client := &GRPCRolePermissions{Issuer: issuer}
	client.Client = roleDecisionClient{check: func(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("Role call lacks bounded deadline")
		}
		md, _ := metadata.FromOutgoingContext(ctx)
		if len(md) != 2 || len(md.Get("authorization")) != 1 || len(md.Get("x-request-id")) != 1 {
			t.Fatalf("unexpected outbound metadata keys: %v", md)
		}
		id := md.Get("x-request-id")[0]
		if _, err := uuid.Parse(id); err != nil || seen[id] {
			t.Fatal("request ID is not fresh UUID")
		}
		seen[id] = true
		if req.SpaceId != space.String() || req.ProfileId != profile.String() || req.GetChat().GetId() != chat.String() || req.PermissionName != "TEXT_CHAT_SEND_MESSAGES" || req.VoiceRoomId != nil {
			t.Fatal("decision subject or node changed")
		}
		hash, err := principal.RequestHash(req)
		if err != nil {
			t.Fatal(err)
		}
		cfg := principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }}
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		p, err := principal.VerifyService(ctx, token, cfg)
		if err != nil || p.Subject != "service:messaging" {
			t.Fatalf("invalid Messaging service credential: %v", err)
		}
		req.ProfileId = uuid.NewString()
		cfg.ExpectedRequestHash, _ = principal.RequestHash(req)
		if _, err = principal.VerifyService(ctx, token, cfg); err == nil {
			t.Fatal("credential accepted changed subject")
		}
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "external", "x-voice-profile-id", "spoof"))
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "external", "x-voice-service", "space"))
	for i := 0; i < 2; i++ {
		allowed, err := client.HasChatPermission(ctx, space, profile, chat, "TEXT_CHAT_SEND_MESSAGES")
		if err != nil || !allowed {
			t.Fatalf("decision: %v %v", allowed, err)
		}
	}
}

func TestRolePermissionsMissingSignerFailsBeforeRPC(t *testing.T) {
	called := false
	client := &GRPCRolePermissions{Client: roleDecisionClient{check: func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		called = true
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}}
	allowed, err := client.HasChatPermission(context.Background(), uuid.New(), uuid.New(), uuid.New(), "TEXT_CHAT_SEND_MESSAGES")
	if allowed || status.Code(err) != codes.Unavailable || called {
		t.Fatalf("missing signer admitted RPC: allowed=%v err=%v called=%v", allowed, err, called)
	}
}

func TestRolePermissionsRejectsGlobalDecisionBeforeRPC(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "test", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	client := &GRPCRolePermissions{Issuer: issuer, Client: roleDecisionClient{check: func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		called = true
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}}
	allowed, err := client.HasSpacePermission(context.Background(), uuid.New(), uuid.New(), "SPACE_MANAGE_BOTS")
	if allowed || status.Code(err) != codes.PermissionDenied || called {
		t.Fatalf("global decision admitted: allowed=%v err=%v called=%v", allowed, err, called)
	}
}
