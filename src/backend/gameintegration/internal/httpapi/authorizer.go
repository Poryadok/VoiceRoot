package httpapi

import (
	"context"
	"net/http"
	"strings"

	voicejwt "voice/backend/pkg/jwt"
)

type AuthorityState interface {
	Minimum(context.Context, string) (int64, error)
	IsRevoked(context.Context, string) (bool, error)
}

// Authorizer revalidates the player's bearer at the owning service. Forwarded
// principal headers from Gateway are never used as identity evidence.
type Authorizer struct {
	Tokens TokenValidator
	State  AuthorityState
}

func (a Authorizer) Validate(r *http.Request) (voicejwt.Claims, string) {
	if a.Tokens == nil || a.State == nil {
		return voicejwt.Claims{}, "auth_unavailable"
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") ||
		strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) == "" {
		return voicejwt.Claims{}, "invalid_token"
	}
	claims, code := a.Tokens.Validate(r)
	if code != "" {
		return voicejwt.Claims{}, code
	}
	if claims.UserID == "" || claims.JTI == "" || claims.SessionEpoch <= 0 {
		return voicejwt.Claims{}, "invalid_token"
	}
	minimum, err := a.State.Minimum(r.Context(), claims.UserID)
	if err != nil || minimum <= 0 {
		return voicejwt.Claims{}, "auth_unavailable"
	}
	if claims.SessionEpoch < minimum {
		return voicejwt.Claims{}, "token_revoked"
	}
	revoked, err := a.State.IsRevoked(r.Context(), claims.JTI)
	if err != nil {
		return voicejwt.Claims{}, "auth_unavailable"
	}
	if revoked {
		return voicejwt.Claims{}, "token_revoked"
	}
	return claims, ""
}
