package main

import (
	"net/http"
	"strings"
)

type tokenClaims struct {
	UserID           string
	ProfileID        string
	Roles            []string
	SubscriptionTier string
}

type tokenValidator interface {
	Validate(r *http.Request) (tokenClaims, bool)
}

type staticTokenValidator map[string]tokenClaims

func (v staticTokenValidator) Validate(r *http.Request) (tokenClaims, bool) {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return tokenClaims{}, false
	}
	claims, ok := v[strings.TrimPrefix(auth, prefix)]
	return claims, ok
}

func (g *gateway) authenticate(r *http.Request) (tokenClaims, bool) {
	return g.tokenValidator.Validate(r)
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
