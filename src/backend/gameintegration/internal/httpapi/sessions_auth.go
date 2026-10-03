package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"voice/backend/gameintegration/internal/registry"
)

const sessionManageScope = "game.sessions.manage"

type SessionCredentialStore interface {
	VerifyCredential(context.Context, string, string, []byte) (registry.ServicePrincipal, error)
}

// RegistrySessionCredentialVerifier derives the session scope exclusively from
// GIS's live credential ledger; request headers never supply application scope.
type RegistrySessionCredentialVerifier struct {
	Store SessionCredentialStore
	Key   []byte
}

func (v RegistrySessionCredentialVerifier) VerifyGameServer(r *http.Request) (registry.SessionPrincipal, error) {
	if v.Store == nil || r == nil || len(v.Key) != 32 {
		return registry.SessionPrincipal{}, errors.New("session credential verification unavailable")
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || len(header) <= len("Bearer ") || strings.TrimSpace(header) != header {
		return registry.SessionPrincipal{}, registry.ErrInvalidServiceCredential
	}
	credential, err := v.Store.VerifyCredential(r.Context(), strings.TrimPrefix(header, "Bearer "), sessionManageScope, v.Key)
	if err != nil {
		return registry.SessionPrincipal{}, err
	}
	return registry.SessionPrincipal{ApplicationID: credential.ApplicationID, EnvironmentID: credential.EnvironmentID, Scopes: append([]string(nil), credential.Scopes...)}, nil
}
