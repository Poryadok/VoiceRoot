package main

import (
	"context"
	"errors"
	"net/http"
	"reflect"
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

// unavailableTokenBlacklist keeps JWKS authentication fail-closed when Redis
// blacklist configuration is absent. The HTTP boundary maps this to the
// established auth_unavailable 503 response.
type unavailableTokenBlacklist struct{}

func (unavailableTokenBlacklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	return false, errors.New("token blacklist is unavailable")
}

type staticTokenValidator map[string]tokenClaims

func (v staticTokenValidator) Validate(r *http.Request) (tokenClaims, string) {
	token := voicejwt.BearerToken(r)
	if token == "" {
		return tokenClaims{}, "invalid_token"
	}
	claims, ok := v[token]
	if !ok {
		return tokenClaims{}, "invalid_token"
	}
	return claims, ""
}

// chainedTokenValidator checks static dev tokens first, then delegates (e.g. JWKS).
type chainedTokenValidator struct {
	static staticTokenValidator
	next   tokenValidator
}

func (v chainedTokenValidator) Validate(r *http.Request) (tokenClaims, string) {
	if len(v.static) > 0 {
		if claims, code := v.static.Validate(r); code == "" {
			return claims, ""
		}
	}
	if v.next != nil {
		return v.next.Validate(r)
	}
	return tokenClaims{}, "invalid_token"
}

func (g *gateway) authenticate(r *http.Request) (tokenClaims, string) {
	claims, code := g.tokenValidator.Validate(r)
	if code != "" {
		return tokenClaims{}, code
	}
	if claims.JTI != "" {
		revoked, err := g.tokenBlacklist.IsRevoked(r.Context(), claims.JTI)
		if err != nil {
			return tokenClaims{}, "auth_unavailable"
		}
		if revoked {
			return tokenClaims{}, "token_revoked"
		}
	}
	if g.config.sessionEpochStrict {
		if claims.SessionEpoch <= 0 {
			return tokenClaims{}, "invalid_token"
		}
		if isNilSessionEpochFloor(g.config.sessionEpochFloor) {
			return tokenClaims{}, "auth_unavailable"
		}
		minimum, err := g.config.sessionEpochFloor.Minimum(r.Context(), claims.UserID)
		if err != nil || minimum <= 0 {
			return tokenClaims{}, "auth_unavailable"
		}
		if claims.SessionEpoch < minimum {
			return tokenClaims{}, "token_revoked"
		}
	}
	return claims, ""
}

func isNilSessionEpochFloor(floor sessionEpochFloor) bool {
	if floor == nil {
		return true
	}
	value := reflect.ValueOf(floor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func applyClaims(r *http.Request, claims tokenClaims) {
	r.Header.Set("X-Voice-User-Id", claims.UserID)
	r.Header.Set("X-Voice-Profile-Id", claims.ProfileID)
	r.Header.Set("X-Voice-Roles", strings.Join(claims.Roles, ","))
	r.Header.Set("X-Voice-Subscription-Tier", claims.SubscriptionTier)
	accountType := strings.TrimSpace(claims.AccountType)
	if accountType == "" {
		accountType = "regular"
	}
	r.Header.Set("X-Voice-Account-Type", accountType)
}

func hasRole(claims tokenClaims, role string) bool {
	for _, candidate := range claims.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}
