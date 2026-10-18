package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

type installationScopeStub struct {
	allowed bool
	err     error
	calls   int
	ids     [4]uuid.UUID
	scope   string
}

func (s *installationScopeStub) CheckCurrentInstallationScope(_ context.Context, bot, space, actor, installation uuid.UUID, scope string) (bool, error) {
	s.calls++
	s.ids = [4]uuid.UUID{bot, space, actor, installation}
	s.scope = scope
	return s.allowed, s.err
}
func installationScopeRequest() *botv1.CheckBotInstallationScopeRequest {
	return &botv1.CheckBotInstallationScopeRequest{BotId: uuid.NewString(), SpaceId: uuid.NewString(), ActorProfileId: uuid.NewString(), InstallationId: uuid.NewString(), BotScope: "TEXT_CHAT_CREATE_IN_SPACE"}
}
func installationScopeContext(t *testing.T, req *botv1.CheckBotInstallationScopeRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	if err != nil {
		t.Fatal(err)
	}
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "bot", RPC: botv1.BotService_CheckBotInstallationScope_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
}
func TestCheckBotInstallationScope_ExactCurrentDecisionAndEcho(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "revoked", true: "current"}[allowed], func(t *testing.T) {
			req := installationScopeRequest()
			reader := &installationScopeStub{allowed: allowed}
			svc := &BotGRPC{InstallationScopeReader: reader}
			resp, err := svc.CheckBotInstallationScope(installationScopeContext(t, req), req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.GetAllowed() != allowed || resp.GetBotId() != req.BotId || resp.GetSpaceId() != req.SpaceId || resp.GetActorProfileId() != req.ActorProfileId || resp.GetInstallationId() != req.InstallationId || resp.GetBotScope() != req.BotScope {
				t.Fatal("decision omitted or changed binding")
			}
			if reader.calls != 1 || reader.ids != [4]uuid.UUID{uuid.MustParse(req.BotId), uuid.MustParse(req.SpaceId), uuid.MustParse(req.ActorProfileId), uuid.MustParse(req.InstallationId)} || reader.scope != req.BotScope {
				t.Fatal("lookup did not bind exact installation")
			}
		})
	}
}
func TestCheckBotInstallationScope_RejectsUntrustedAndChangedBindingBeforeLookup(t *testing.T) {
	req := installationScopeRequest()
	reader := &installationScopeStub{}
	svc := &BotGRPC{InstallationScopeReader: reader}
	if _, err := svc.CheckBotInstallationScope(context.Background(), req); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("raw context: %v", err)
	}
	ctx := installationScopeContext(t, req)
	req.InstallationId = uuid.NewString()
	if _, err := svc.CheckBotInstallationScope(ctx, req); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("changed signed tuple: %v", err)
	}
	if reader.calls != 0 {
		t.Fatal("unauthenticated lookup")
	}
}
func TestCheckBotInstallationScope_DependencyFailureIsUnknown(t *testing.T) {
	req := installationScopeRequest()
	reader := &installationScopeStub{err: errors.New("database unavailable")}
	svc := &BotGRPC{InstallationScopeReader: reader}
	if resp, err := svc.CheckBotInstallationScope(installationScopeContext(t, req), req); status.Code(err) != codes.Unavailable || resp != nil {
		t.Fatalf("dependency failure became decision: %v %v", resp, err)
	}
}

func TestCheckBotInstallationScope_RejectsMalformedOrUnsupportedBinding(t *testing.T) {
	for _, field := range []string{"bot", "space", "actor", "installation", "scope", "unknown"} {
		t.Run(field, func(t *testing.T) {
			req := installationScopeRequest()
			switch field {
			case "bot":
				req.BotId = uuid.Nil.String()
			case "space":
				req.SpaceId = " " + req.SpaceId
			case "actor":
				req.ActorProfileId = "invalid"
			case "installation":
				req.InstallationId = ""
			case "scope":
				req.BotScope = "SPACE_MANAGE_ROLES"
			case "unknown":
				req.ProtoReflect().SetUnknown([]byte{0x78, 0x01})
			}
			reader := &installationScopeStub{allowed: true}
			svc := &BotGRPC{InstallationScopeReader: reader}
			if field == "unknown" {
				if _, err := svc.CheckBotInstallationScope(context.Background(), req); err == nil {
					t.Fatal("unknown request accepted")
				}
			} else if _, err := svc.CheckBotInstallationScope(installationScopeContext(t, req), req); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("malformed request: %v", err)
			}
			if reader.calls != 0 {
				t.Fatal("invalid binding reached storage")
			}
		})
	}
}
