package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBotLifecycleHTTPRequiresNativeAuthentication(t *testing.T) {
	for _, mode := range []string{"signed", "static", "chained static", "shadowed signed", "non-strict", "duplicate bearer", "query token", "protected only"} {
		t.Run(mode, func(t *testing.T) {
			f := newCreatorFixture(t)
			claims := tokenClaims{UserID: f.payload["user_id"].(string), ProfileID: f.payload["profile_id"].(string), SessionEpoch: 7, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}
			client := &verifiedBotLifecycleClient{key: &f.key.PublicKey, claims: claims}
			cfg := f.config(nil)
			cfg.transcoder.clients.bot = &fakeBotClient{}
			cfg.transcoder.clients.botAuditAdmin = client
			token := signConfigJWT(t, "creator-key", f.key, f.payload)
			want := http.StatusNoContent
			switch mode {
			case "static", "chained static":
				token = "development-token"
				cfg.tokenValidator = staticTokenValidator{token: claims}
				if mode == "chained static" {
					cfg.tokenValidator = chainedTokenValidator{static: staticTokenValidator{token: claims}, next: f.validator}
				}
				want = http.StatusUnauthorized
			case "shadowed signed":
				cfg.tokenValidator = chainedTokenValidator{static: staticTokenValidator{token: {UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 99, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}}, next: f.validator}
			case "non-strict":
				cfg.sessionEpochStrict = false
				want = http.StatusServiceUnavailable
			case "duplicate bearer", "query token":
				want = http.StatusUnauthorized
			case "protected only":
				cfg.transcoder.clients.bot = nil
			}
			path := "/api/v1/bots/" + uuid.NewString()
			if mode == "query token" {
				path += "?access_token=" + token
			}
			r := httptest.NewRequest(http.MethodDelete, path, nil)
			if mode != "query token" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			if mode == "duplicate bearer" {
				r.Header.Add("Authorization", "Bearer "+token)
			}
			r.Header.Set("X-Voice-User-Id", uuid.NewString())
			w := httptest.NewRecorder()
			newGateway(cfg).ServeHTTP(w, r)
			require.Equal(t, want, w.Code, w.Body.String())
			if want == http.StatusNoContent {
				require.Equal(t, 1, client.calls)
			} else {
				require.Zero(t, client.calls)
			}
		})
	}
}

func TestBotLifecycleHTTPDispatchesAllProtectedRoutes(t *testing.T) {
	f := newCreatorFixture(t)
	claims := tokenClaims{UserID: f.payload["user_id"].(string), ProfileID: f.payload["profile_id"].(string), SessionEpoch: 7, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}
	client := &verifiedBotLifecycleClient{key: &f.key.PublicKey, claims: claims}
	cfg := f.config(nil)
	cfg.transcoder.clients.botAuditAdmin = client
	for _, tc := range []struct {
		method, suffix, body string
		status               int
	}{
		{http.MethodDelete, "", "", http.StatusNoContent},
		{http.MethodDelete, "/spaces/" + uuid.NewString(), "", http.StatusNoContent},
		{http.MethodPost, "/spaces/" + uuid.NewString() + "/install", `{ "acknowledgePrivilegedScopes": true }`, http.StatusOK},
	} {
		r := httptest.NewRequest(tc.method, "/api/v1/bots/"+uuid.NewString()+tc.suffix, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, f.payload))
		w := httptest.NewRecorder()
		newGateway(cfg).ServeHTTP(w, r)
		require.Equal(t, tc.status, w.Code, w.Body.String())
	}
	require.Equal(t, 3, client.calls)
}
