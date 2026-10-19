package principal

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestRequireDelegatedUserBoundIdentity(t *testing.T) {
	request := wrapperspb.String("exact request")
	hash, err := RequestHash(request)
	require.NoError(t, err)
	account, profile := uuid.NewString(), uuid.NewString()
	base := Principal{Kind: "delegated_user", Issuer: "gateway", Subject: account,
		AccountID: account, ProfileID: profile, SessionEpoch: 7, Audience: "space",
		RPC: "/space/read", RequestID: uuid.NewString(), RequestHash: hash}
	for _, accountType := range []string{"", "regular", "guest", "sdk-account"} {
		p := base
		p.AccountType = accountType
		// Neither raw incoming nor outgoing identity may override a verified actor.
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString()))
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-profile-id", uuid.NewString()))
		got, err := RequireDelegatedUser(WithVerified(ctx, p), "space", "/space/read", request)
		require.NoError(t, err)
		require.Equal(t, UserIdentity{uuid.MustParse(account), uuid.MustParse(profile), 7, accountType}, got)
	}
	for name, mutate := range map[string]func(*Principal){
		"kind":              func(p *Principal) { p.Kind = "service" },
		"issuer":            func(p *Principal) { p.Issuer = "space" },
		"subject":           func(p *Principal) { p.Subject = uuid.NewString() },
		"audience":          func(p *Principal) { p.Audience = "role" },
		"rpc":               func(p *Principal) { p.RPC = "/space/write" },
		"request id":        func(p *Principal) { p.RequestID = "" },
		"request hash":      func(p *Principal) { p.RequestHash = "sha256:wrong" },
		"nil account":       func(p *Principal) { p.AccountID = uuid.Nil.String(); p.Subject = p.AccountID },
		"nil profile":       func(p *Principal) { p.ProfileID = uuid.Nil.String() },
		"account alias":     func(p *Principal) { p.AccountID = "urn:uuid:" + account; p.Subject = p.AccountID },
		"profile alias":     func(p *Principal) { p.ProfileID = strings.ReplaceAll(profile, "-", "") },
		"padded profile":    func(p *Principal) { p.ProfileID += " " },
		"uppercase profile": func(p *Principal) { p.ProfileID = strings.ToUpper("abcdefab-cdef-4abc-8def-abcdefabcdef") },
		"zero epoch":        func(p *Principal) { p.SessionEpoch = 0 },
		"negative epoch":    func(p *Principal) { p.SessionEpoch = -1 },
		"account type":      func(p *Principal) { p.AccountType = "ADMIN" },
		"actor":             func(p *Principal) { p.ActorProfileID = profile },
		"bot":               func(p *Principal) { p.BotID = uuid.NewString() },
		"space":             func(p *Principal) { p.SpaceID = uuid.NewString() },
		"installation":      func(p *Principal) { p.InstallationID = uuid.NewString() },
		"bot scope":         func(p *Principal) { p.BotScope = "read" },
	} {
		t.Run(name, func(t *testing.T) {
			p := base
			mutate(&p)
			got, err := RequireDelegatedUser(WithVerified(context.Background(), p), "space", "/space/read", request)
			require.ErrorIs(t, err, ErrInvalidDelegatedUser)
			require.Equal(t, UserIdentity{}, got)
		})
	}
	for name, ctx := range map[string]context.Context{
		"missing":  context.Background(),
		"raw only": metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-user-id", account, "x-voice-profile-id", profile, "x-voice-session-epoch", "7")),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := RequireDelegatedUser(ctx, "space", "/space/read", request)
			require.ErrorIs(t, err, ErrInvalidDelegatedUser)
			require.Equal(t, UserIdentity{}, got)
		})
	}
	ctx := WithVerified(context.Background(), base)
	_, err = RequireDelegatedUser(ctx, "", "/space/read", request)
	require.ErrorIs(t, err, ErrInvalidDelegatedUser)
	_, err = RequireDelegatedUser(ctx, "space", "", request)
	require.ErrorIs(t, err, ErrInvalidDelegatedUser)
	_, err = RequireDelegatedUser(ctx, "space", "/space/read", wrapperspb.String("changed"))
	require.ErrorIs(t, err, ErrInvalidDelegatedUser)
	_, err = RequireDelegatedUser(ctx, "space", "/space/read", nil)
	require.ErrorIs(t, err, ErrInvalidDelegatedUser)
	var typedNil *wrapperspb.StringValue
	_, err = RequireDelegatedUser(ctx, "space", "/space/read", typedNil)
	require.ErrorIs(t, err, ErrInvalidDelegatedUser)
}
