package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	voicejwt "voice/backend/pkg/jwt"
)

type fakeEpochAndBlacklist struct {
	minimum int64
	revoked bool
	err     error
}

func (f fakeEpochAndBlacklist) Minimum(context.Context, string) (int64, error) {
	return f.minimum, f.err
}

func (f fakeEpochAndBlacklist) IsRevoked(context.Context, string) (bool, error) {
	return f.revoked, f.err
}

func TestAuthorizerRejectsRevokedStaleAndUnavailableTokens(t *testing.T) {
	base := voicejwt.Claims{UserID: "owner", AccountType: "regular", SessionEpoch: 4, JTI: "jti-1"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications", nil)
	request.Header.Set("Authorization", "Bearer player-token")
	for _, tc := range []struct {
		name   string
		claims voicejwt.Claims
		state  fakeEpochAndBlacklist
		want   string
	}{
		{name: "current", claims: base, state: fakeEpochAndBlacklist{minimum: 4}},
		{name: "stale", claims: base, state: fakeEpochAndBlacklist{minimum: 5}, want: "token_revoked"},
		{name: "blacklisted", claims: base, state: fakeEpochAndBlacklist{minimum: 4, revoked: true}, want: "token_revoked"},
		{name: "store unavailable", claims: base, state: fakeEpochAndBlacklist{err: errors.New("redis down")}, want: "auth_unavailable"},
		{name: "missing jti", claims: voicejwt.Claims{UserID: "owner", SessionEpoch: 4}, state: fakeEpochAndBlacklist{minimum: 4}, want: "invalid_token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authorizer := Authorizer{Tokens: testValidator{claims: tc.claims}, State: tc.state}
			_, code := authorizer.Validate(request)
			require.Equal(t, tc.want, code)
		})
	}
}

func TestAuthorizerRejectsQueryTokenAndForgedPrincipalHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications?access_token=player-token", nil)
	r.Header.Set("X-Voice-User-Id", "forged-owner")
	authorizer := Authorizer{
		Tokens: testValidator{claims: voicejwt.Claims{UserID: "owner", SessionEpoch: 1, JTI: "jti-1"}},
		State:  fakeEpochAndBlacklist{minimum: 1},
	}
	_, code := authorizer.Validate(r)
	require.Equal(t, "invalid_token", code)
}
