package reporttargetprincipal

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"testing"
	"voice/backend/pkg/principal"
)

func TestReportTargetDomainBinding(t *testing.T) {
	req := wrapperspb.String("target")
	hash, _ := principal.RequestHash(req)
	for _, owner := range []string{"user", "messaging", "space", "story"} {
		p := principal.Principal{Kind: "service", Issuer: "moderation", Subject: "service:moderation", Audience: owner, RPC: Method(owner), RequestID: uuid.NewString(), RequestHash: hash}
		if err := Require(principal.WithVerified(context.Background(), p), owner, req); err != nil {
			t.Fatal(err)
		}
		for name, change := range map[string]func(*principal.Principal){
			"issuer":   func(p *principal.Principal) { p.Issuer = "gateway" },
			"subject":  func(p *principal.Principal) { p.Subject = "service:role" },
			"kind":     func(p *principal.Principal) { p.Kind = "delegated_user" },
			"audience": func(p *principal.Principal) { p.Audience = "role" },
			"rpc":      func(p *principal.Principal) { p.RPC = "/other" },
			"hash":     func(p *principal.Principal) { p.RequestHash = "sha256:bad" },
			"request":  func(p *principal.Principal) { p.RequestID = "bad" },
			"account":  func(p *principal.Principal) { p.AccountID = uuid.NewString() },
			"bot":      func(p *principal.Principal) { p.BotID = uuid.NewString() },
		} {
			t.Run(owner+"/"+name, func(t *testing.T) {
				bad := p
				change(&bad)
				if Require(principal.WithVerified(context.Background(), bad), owner, req) == nil {
					t.Fatal("accepted invalid binding")
				}
			})
		}
		called := false
		_, err := OrdinaryUnaryInterceptor(owner)(context.Background(), req, &grpc.UnaryServerInfo{FullMethod: Method(owner)}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		if called || status.Code(err) != codes.Unavailable {
			t.Fatal("ordinary listener exposed owner decision")
		}
	}
	if Require(context.Background(), "user", req) == nil {
		t.Fatal("accepted raw identity")
	}
	if Method("chat") != "" {
		t.Fatal("unapproved owner")
	}
}
