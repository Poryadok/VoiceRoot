package main

import (
	"context"
	"net/http"
	"strings"

	voicejwt "voice/backend/pkg/jwt"
)

// tokenClaims mirrors JWT access token claims (voice/backend/pkg/jwt).
type tokenClaims = voicejwt.Claims

type tokenValidator interface {
	Validate(r *http.Request) (tokenClaims, string)
}

type tokenBlacklist interface {
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

type noTokenBlacklist struct{}

func (noTokenBlacklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type staticTokenValidator map[string]tokenClaims

func (v staticTokenValidator) Validate(r *http.Request) (tokenClaims, string) {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return tokenClaims{}, "invalid_token"
	}
	claims, ok := v[strings.TrimPrefix(auth, prefix)]
	if !ok {
		return tokenClaims{}, "invalid_token"
	}
	return claims, ""
}

func (g *gateway) authenticate(r *http.Request) (tokenClaims, string) {
	claims, code := g.tokenValidator.Validate(r)
	if code != "" {
		return tokenClaims{}, code
	}
	if claims.JTI == "" {
		return claims, ""
	}
	revoked, err := g.tokenBlacklist.IsRevoked(r.Context(), claims.JTI)
	if err != nil {
		return tokenClaims{}, "auth_unavailable"
	}
	if revoked {
		return tokenClaims{}, "token_revoked"
	}
	return claims, ""
}

func applyClaims(r *http.Request, claims tokenClaims) {
	r.Header.Set("X-Voice-User-Id", claims.UserID)
	r.Header.Set("X-Voice-Profile-Id", claims.ProfileID)
	r.Header.Set("X-Voice-Roles", strings.Join(claims.Roles, ","))
	r.Header.Set("X-Voice-Subscription-Tier", claims.SubscriptionTier)
}

func hasRole(claims tokenClaims, role string) bool {
	for _, candidate := range claims.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}
