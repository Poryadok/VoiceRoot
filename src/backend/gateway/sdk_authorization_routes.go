package main

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type sdkAuthorizationPrincipal uint8

const (
	sdkAuthorizationSDKAccount sdkAuthorizationPrincipal = iota + 1
	sdkAuthorizationRegularAccount
	sdkAuthorizationCodeProof
	sdkAuthorizationLinkedBearer
)

type sdkAuthorizationRoutePolicy struct {
	principal sdkAuthorizationPrincipal
	rateGroup string
}

const sdkAuthorizationRouteBase = "/api/v1/auth/sdk/authorizations"
const sdkConversionRouteBase = "/api/v1/auth/sdk/conversions"

// sdkAuthorizationPolicy classifies the frozen Auth authorization and conversion
// routes. Auth remains authoritative for device possession, conversion proof,
// PKCE, linked credentials, ownership and profile selection.
func sdkAuthorizationPolicy(method, path string) (sdkAuthorizationRoutePolicy, bool) {
	if method == http.MethodPost && path == sdkConversionRouteBase+"/new" {
		return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationSDKAccount, rateGroup: "AuthOAuth"}, true
	}
	if method == http.MethodPost && path == sdkConversionRouteBase+"/existing" {
		return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationLinkedBearer, rateGroup: "AuthOAuth"}, true
	}
	conversionPath := strings.TrimPrefix(path, sdkConversionRouteBase+"/")
	if conversionPath != path {
		parts := strings.Split(conversionPath, "/")
		if len(parts) == 2 {
			if _, err := uuid.Parse(parts[0]); err == nil && method == http.MethodPost {
				switch parts[1] {
				case "status":
					return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationCodeProof, rateGroup: "AuthOAuth"}, true
				case "attach-new-target":
					return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationRegularAccount, rateGroup: "AuthOAuth"}, true
				}
			}
		}
		return sdkAuthorizationRoutePolicy{}, false
	}
	if method == http.MethodPost && path == sdkAuthorizationRouteBase {
		return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationSDKAccount, rateGroup: "AuthOAuth"}, true
	}
	if method == http.MethodPost && path == sdkAuthorizationRouteBase+"/linked-session" {
		return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationLinkedBearer, rateGroup: "AuthOAuth"}, true
	}
	requestPath := strings.TrimPrefix(path, sdkAuthorizationRouteBase+"/")
	if requestPath == path {
		return sdkAuthorizationRoutePolicy{}, false
	}
	parts := strings.Split(requestPath, "/")
	if len(parts) == 1 {
		if _, err := uuid.Parse(parts[0]); err == nil && method == http.MethodGet {
			return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationRegularAccount, rateGroup: "AuthOAuth"}, true
		}
		return sdkAuthorizationRoutePolicy{}, false
	}
	if len(parts) != 2 {
		return sdkAuthorizationRoutePolicy{}, false
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		return sdkAuthorizationRoutePolicy{}, false
	}
	switch {
	case method == http.MethodPost && parts[1] == "approve":
		return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationRegularAccount, rateGroup: "AuthOAuth"}, true
	case method == http.MethodPost && parts[1] == "exchange":
		return sdkAuthorizationRoutePolicy{principal: sdkAuthorizationCodeProof, rateGroup: "AuthOAuth"}, true
	default:
		return sdkAuthorizationRoutePolicy{}, false
	}
}

func effectiveAccountType(claims tokenClaims) string {
	accountType := strings.TrimSpace(claims.AccountType)
	if accountType == "" {
		return "regular"
	}
	return accountType
}

func hasSingleBearerAuthorization(r *http.Request) bool {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return false
	}
	value := values[0]
	return strings.HasPrefix(value, "Bearer ") && len(value) > len("Bearer ") && len(value) <= 16384
}
