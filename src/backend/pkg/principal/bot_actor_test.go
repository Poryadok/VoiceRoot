package principal

import (
	"context"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func botActorFixture(t *testing.T) (*Issuer, *rsa.PrivateKey, BotActorInput, VerifyConfig) {
	t.Helper()
	now := time.Unix(1_700_000_000, 0).UTC()
	_, key := testIssuer(t, now)
	issuer, err := NewIssuer(IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	input := BotActorInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CreateRole", RequestID: "operation-1", RequestHash: testRequestHash, ActorProfileID: "11111111-1111-4111-8111-111111111111", BotID: "22222222-2222-4222-8222-222222222222", SpaceID: "33333333-3333-4333-8333-333333333333", InstallationID: "44444444-4444-4444-8444-444444444444", BotScope: "SPACE_MANAGE_ROLES"}
	config := VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: input.Audience, ExpectedRPC: input.RPC, ExpectedRequestID: input.RequestID, ExpectedRequestHash: input.RequestHash, Clock: func() time.Time { return now }, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }}
	return issuer, key, input, config
}

func TestBotActorCredential_PreservesDistinctCapability(t *testing.T) {
	issuer, _, input, config := botActorFixture(t)
	token, err := issuer.IssueBotActor(input)
	require.NoError(t, err)
	got, err := VerifyBotActor(context.Background(), token, config)
	require.NoError(t, err)
	require.Equal(t, botActorType, got.Kind)
	require.Equal(t, "bot:"+input.BotID, got.Subject)
	require.Equal(t, input.ActorProfileID, got.ActorProfileID)
	require.Equal(t, input.BotID, got.BotID)
	require.Equal(t, input.SpaceID, got.SpaceID)
	require.Equal(t, input.InstallationID, got.InstallationID)
	require.Equal(t, input.BotScope, got.BotScope)
	require.Empty(t, got.AccountID)
	require.Empty(t, got.ProfileID)
	require.Zero(t, got.SessionEpoch)
	_, err = VerifyService(context.Background(), token, config)
	require.Error(t, err)
	_, err = VerifyDelegatedUser(context.Background(), token, config)
	require.Error(t, err)
}

func TestBotActorCredential_RequiresBotIssuerCompleteIdentityAndReplay(t *testing.T) {
	issuer, key, input, config := botActorFixture(t)
	foreign, err := NewIssuer(IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	_, err = foreign.IssueBotActor(input)
	require.Error(t, err)
	for _, change := range []func(*BotActorInput){
		func(i *BotActorInput) { i.ActorProfileID = "" },
		func(i *BotActorInput) { i.BotID = "" },
		func(i *BotActorInput) { i.SpaceID = "" },
		func(i *BotActorInput) { i.InstallationID = "" },
		func(i *BotActorInput) { i.BotScope = "" },
		func(i *BotActorInput) { i.BotScope = "MEMBER_ASSIGN_ROLES" },
		func(i *BotActorInput) { i.RPC = "/voice.role.v1.RoleService/DeleteRole" },
	} {
		invalid := input
		change(&invalid)
		_, err := issuer.IssueBotActor(invalid)
		require.Error(t, err)
	}
	token, err := issuer.IssueBotActor(input)
	require.NoError(t, err)
	config.ReplayGuard = nil
	_, err = VerifyBotActor(context.Background(), token, config)
	require.Error(t, err)
	config.ReplayGuard = func(context.Context, string, string, time.Time) error { return errors.New("replay store unavailable") }
	_, err = VerifyBotActor(context.Background(), token, config)
	require.Error(t, err)
}

func TestBotActorCredential_RejectsSignedMixedOrSubstitutedAuthority(t *testing.T) {
	_, key, input, config := botActorFixture(t)
	now := config.Clock().Unix()
	base := rawClaims{Type: botActorType, Issuer: "bot", Subject: "bot:" + input.BotID, Audience: input.Audience, RPC: input.RPC, RequestID: input.RequestID, RequestHash: input.RequestHash, ActorProfileID: input.ActorProfileID, BotID: input.BotID, SpaceID: input.SpaceID, InstallationID: input.InstallationID, BotScope: input.BotScope, IssuedAt: now, NotBefore: now, ExpiresAt: now + 30, JWTID: "signed-negative"}
	for name, change := range map[string]func(*rawClaims){
		"human account":        func(c *rawClaims) { c.AccountID = "human" },
		"human profile":        func(c *rawClaims) { c.ProfileID = input.ActorProfileID },
		"fake epoch":           func(c *rawClaims) { c.SessionEpoch = 1 },
		"human account type":   func(c *rawClaims) { c.AccountType = "regular" },
		"wrong subject":        func(c *rawClaims) { c.Subject = "service:bot" },
		"missing installation": func(c *rawClaims) { c.InstallationID = "" },
		"wrong action scope":   func(c *rawClaims) { c.BotScope = "MEMBER_ASSIGN_ROLES" },
	} {
		t.Run(name, func(t *testing.T) {
			claims := base
			change(&claims)
			token, err := signClaims(key, "current", claims)
			require.NoError(t, err)
			_, err = VerifyBotActor(context.Background(), token, config)
			require.Error(t, err)
		})
	}
	service := base
	service.Type, service.Subject = serviceType, "service:bot"
	token, err := signClaims(key, "current", service)
	require.NoError(t, err)
	_, err = VerifyService(context.Background(), token, config)
	require.Error(t, err, "service credential cannot hide interactive bot capability fields")
}

func TestDelegatedAuditActor_PreservesExplicitAccountType(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	issuer, key := testIssuer(t, now)
	for _, accountType := range []string{"guest", "regular"} {
		token, err := issuer.IssueDelegatedUser(DelegatedUserInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CreateRole", RequestID: "req-1", RequestHash: testRequestHash, AccountID: "account-1", ProfileID: "profile-1", SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute), AccountType: accountType})
		require.NoError(t, err)
		got, err := VerifyDelegatedUser(context.Background(), token, delegatedConfig(key, now))
		require.NoError(t, err)
		require.Equal(t, accountType, got.AccountType)
	}
}
