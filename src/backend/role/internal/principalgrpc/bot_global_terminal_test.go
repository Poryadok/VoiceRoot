package principalgrpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestRequireBotGlobalTerminalExactClaimSet(t *testing.T) {
	req := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot", Audience: "role", RPC: rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName, RequestID: req.DecisionId, RequestHash: hash}
	require.NoError(t, RequireBotGlobalTerminal(principal.WithVerified(context.Background(), p), req))
	require.Error(t, RequireBotGlobalTerminal(context.Background(), req))
	for name, change := range map[string]func(*principal.Principal){
		"kind": func(p *principal.Principal) { p.Kind = "bot_actor" }, "issuer": func(p *principal.Principal) { p.Issuer = "space" }, "subject": func(p *principal.Principal) { p.Subject = "other" }, "audience": func(p *principal.Principal) { p.Audience = "space" }, "rpc": func(p *principal.Principal) { p.RPC = rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName }, "root": func(p *principal.Principal) { p.RequestID = uuid.NewString() }, "hash": func(p *principal.Principal) { p.RequestHash = "bad" },
		"account": func(p *principal.Principal) { p.AccountID = uuid.NewString() }, "profile": func(p *principal.Principal) { p.ProfileID = uuid.NewString() }, "epoch": func(p *principal.Principal) { p.SessionEpoch = 1 }, "account_type": func(p *principal.Principal) { p.AccountType = "human" }, "bot_claim": func(p *principal.Principal) { p.BotID = req.BotId }, "actor": func(p *principal.Principal) { p.ActorProfileID = uuid.NewString() }, "installation": func(p *principal.Principal) { p.InstallationID = uuid.NewString() }, "scope": func(p *principal.Principal) { p.BotScope = "global" }, "space": func(p *principal.Principal) { p.SpaceID = uuid.NewString() },
	} {
		t.Run(name, func(t *testing.T) {
			bad := p
			change(&bad)
			require.Error(t, RequireBotGlobalTerminal(principal.WithVerified(context.Background(), bad), req))
		})
	}
	for _, name := range []string{"schema", "bot", "root", "unknown"} {
		t.Run(name, func(t *testing.T) {
			r := proto.Clone(req).(*rolev1.RegisterBotGlobalTerminalRequest)
			switch name {
			case "schema":
				r.ProtocolVersion = 2
			case "bot":
				r.BotId = uuid.Nil.String()
			case "root":
				r.DecisionId = "not-uuid"
			case "unknown":
				r.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			}
			h, err := principal.RequestHash(r)
			require.NoError(t, err)
			bound := p
			bound.RequestHash = h
			bound.RequestID = r.DecisionId
			require.Error(t, RequireBotGlobalTerminal(principal.WithVerified(context.Background(), bound), r))
		})
	}
}
