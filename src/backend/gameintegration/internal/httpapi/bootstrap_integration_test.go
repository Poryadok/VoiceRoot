package httpapi

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/integrationtest"
	voicejwt "voice/backend/pkg/jwt"
)

const (
	bootstrapIssuer   = "https://auth.bootstrap.test"
	bootstrapAudience = "voice-api"
	bootstrapKeyID    = "bootstrap-fake-jwks"
)

type bootstrapAuthorityState struct{}

func (bootstrapAuthorityState) Minimum(context.Context, string) (int64, error)  { return 1, nil }
func (bootstrapAuthorityState) IsRevoked(context.Context, string) (bool, error) { return false, nil }

type bootstrapJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// TestGameIntegrationCleanBootstrapUsesOwnerAndSeparateOperatorAPIs uses a
// fake Voice JWKS only; it does not perform or claim real Google OIDC proof.
func TestGameIntegrationCleanBootstrapUsesOwnerAndSeparateOperatorAPIs(t *testing.T) {
	runGameIntegrationCleanBootstrap(t, false)
}

// TestGameIntegrationProductionEnvironmentCredentialDeniedWithSeededFixture
// keeps the denied production-environment case in the ordinary GIS suite. The
// API cannot create production state yet, so this test is deliberately excluded
// from the SQL-seed-free Q11 clean-start command.
func TestGameIntegrationProductionEnvironmentCredentialDeniedWithSeededFixture(t *testing.T) {
	runGameIntegrationCleanBootstrap(t, true)
}

func runGameIntegrationCleanBootstrap(t *testing.T, includeProductionFixture bool) {
	ctx := context.Background()
	migration := filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_bootstrap", migration)
	t12Migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(t12Migration))
	require.NoError(t, err)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	public := key.PublicKey
	exponent := big.NewInt(int64(public.E)).Bytes()
	jwk := bootstrapJWK{
		Kty: "RSA", Kid: bootstrapKeyID, Use: "sig", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(public.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(exponent),
	}
	keySet, err := json.Marshal(map[string]any{"keys": []bootstrapJWK{jwk}})
	require.NoError(t, err)
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(keySet)
	}))
	t.Cleanup(jwks.Close)
	authorizer := Authorizer{
		Tokens: voicejwt.NewJWKSValidator(jwks.URL, bootstrapIssuer, bootstrapAudience),
		State:  bootstrapAuthorityState{},
	}
	storeNow := time.Now().UTC()
	store := &registry.Store{Pool: pool, Now: func() time.Time { return storeNow }}
	handler := NewHandler(authorizer, store)
	applicant, secondOwner, operator, guest := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	applicantToken := bootstrapAccessToken(t, key, applicant, "regular")
	secondOwnerToken := bootstrapAccessToken(t, key, secondOwner, "regular")
	operatorToken := bootstrapAccessToken(t, key, operator, "regular")
	guestToken := bootstrapAccessToken(t, key, guest, "guest")
	handler.OperatorAccounts = map[uuid.UUID]struct{}{
		applicant: {}, // Even an allowlisted applicant cannot approve their own app.
		operator:  {},
	}
	handler.CredentialKey = []byte("0123456789abcdef0123456789abcdef")

	call := func(method, path, token, idempotencyKey, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if idempotencyKey != "" {
			r.Header.Set("Idempotency-Key", idempotencyKey)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	decode := func(w *httptest.ResponseRecorder) map[string]any {
		var result map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		return result
	}

	unauthorized := call(http.MethodPost, "/api/v1/game-integrations/applications", "", "unauthenticated", `{"name":"Rejected"}`)
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	guestResponse := call(http.MethodPost, "/api/v1/game-integrations/applications", guestToken, "guest-create", `{"name":"Rejected"}`)
	require.Equal(t, http.StatusForbidden, guestResponse.Code)
	foreignOwner := call(http.MethodPost, "/api/v1/game-integrations/applications", applicantToken, "foreign-owner", `{"name":"Rejected","owner_account_id":"`+secondOwner.String()+`"}`)
	require.Equal(t, http.StatusBadRequest, foreignOwner.Code)

	created := call(http.MethodPost, "/api/v1/game-integrations/applications", applicantToken, "t11-app-1", `{"name":"HerdTrip"}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	appID, err := uuid.Parse(decode(created)["application_id"].(string))
	require.NoError(t, err)

	selfApproval := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/admissions/sandbox", applicantToken, "self-approval", "")
	require.Equal(t, http.StatusForbidden, selfApproval.Code)
	require.Equal(t, "SELF_APPROVAL_DENIED", decode(selfApproval)["error_code"])
	unlistedOperator := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/admissions/sandbox", secondOwnerToken, "unlisted-approval", "")
	require.Equal(t, http.StatusForbidden, unlistedOperator.Code)
	require.Equal(t, "OPERATOR_REQUIRED", decode(unlistedOperator)["error_code"])

	approved := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/admissions/sandbox", operatorToken, "t11-approve-1", "")
	require.Equal(t, http.StatusCreated, approved.Code, approved.Body.String())
	envID, err := uuid.Parse(decode(approved)["environment_id"].(string))
	require.NoError(t, err)
	require.Equal(t, "sandbox", decode(approved)["kind"])

	secondApp := call(http.MethodPost, "/api/v1/game-integrations/applications", secondOwnerToken, "t11-app-2", `{"name":"Dejavu"}`)
	require.Equal(t, http.StatusCreated, secondApp.Code)
	secondAppID, err := uuid.Parse(decode(secondApp)["application_id"].(string))
	require.NoError(t, err)
	secondApproved := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+secondAppID.String()+"/admissions/sandbox", operatorToken, "t11-approve-2", "")
	require.Equal(t, http.StatusCreated, secondApproved.Code, secondApproved.Body.String())
	secondEnvID, err := uuid.Parse(decode(secondApproved)["environment_id"].(string))
	require.NoError(t, err)
	wrongOwner := call(http.MethodPut,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/policy",
		secondOwnerToken, "wrong-owner-policy", `{"expected_revision":1,"redirect_uris":["https://game.example/callback"],"allowed_origins":["https://game.example"],"providers":["google"],"player_scopes":["game.chat.read"]}`)
	require.Equal(t, http.StatusForbidden, wrongOwner.Code)
	wrongApplication := call(http.MethodPut,
		"/api/v1/game-integrations/applications/"+secondAppID.String()+"/environments/"+envID.String()+"/policy",
		applicantToken, "wrong-app-policy", `{"expected_revision":1,"redirect_uris":["https://game.example/callback"],"allowed_origins":["https://game.example"],"providers":["google"],"player_scopes":["game.chat.read"]}`)
	require.Equal(t, http.StatusForbidden, wrongApplication.Code)
	wrongEnvironment := call(http.MethodPut,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+uuid.NewString()+"/policy",
		applicantToken, "wrong-env-policy", `{"expected_revision":1,"redirect_uris":["https://game.example/callback"],"allowed_origins":["https://game.example"],"providers":["google"],"player_scopes":["game.chat.read"]}`)
	require.Equal(t, http.StatusForbidden, wrongEnvironment.Code)

	_, err = store.LoadAuthorizationPolicy(ctx, envID)
	require.ErrorIs(t, err, registry.ErrPolicyUnavailable)
	configured := call(http.MethodPut,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/policy",
		applicantToken, "t11-policy-1", `{"expected_revision":1,"redirect_uris":["https://game.example/callback"],"allowed_origins":["https://game.example"],"providers":["google"],"player_scopes":["game.chat.read"]}`)
	require.Equal(t, http.StatusOK, configured.Code, configured.Body.String())
	policy, err := store.LoadAuthorizationPolicy(ctx, envID)
	require.NoError(t, err)
	require.Equal(t, appID, policy.ApplicationID)
	require.Equal(t, envID, policy.EnvironmentID)
	require.Equal(t, []string{"google"}, policy.Providers)
	require.Equal(t, []string{"https://game.example/callback"}, policy.RedirectURIs)
	require.Equal(t, []string{"https://game.example"}, policy.AllowedOrigins)
	credential := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials",
		applicantToken, "t11-credential-1", `{"scopes":["game.events.write"]}`)
	require.Equal(t, http.StatusCreated, credential.Code, credential.Body.String())
	require.Equal(t, "no-store", credential.Header().Get("Cache-Control"))
	credentialBody := decode(credential)
	credentialID, err := uuid.Parse(credentialBody["credential_id"].(string))
	require.NoError(t, err)
	secret := credentialBody["secret"].(string)
	require.True(t, strings.HasPrefix(secret, "vgi1_"))
	firstPrincipal, err := store.VerifyCredential(ctx, secret, "game.events.write", handler.CredentialKey)
	require.NoError(t, err)
	require.Equal(t, appID, firstPrincipal.ApplicationID)
	require.Equal(t, envID, firstPrincipal.EnvironmentID)

	// A lost issue response can be recovered with the same API request, while
	// changing the idempotency key creates a distinct rotation generation.
	credentialRetry := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials",
		applicantToken, "t11-credential-1", `{"scopes":["game.events.write"]}`)
	require.Equal(t, http.StatusCreated, credentialRetry.Code, credentialRetry.Body.String())
	require.Equal(t, credentialBody, decode(credentialRetry))
	otherOwnerRevoke := call(http.MethodDelete,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials/"+credentialID.String(),
		secondOwnerToken, "", "")
	require.Equal(t, http.StatusForbidden, otherOwnerRevoke.Code)

	rotated := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials",
		applicantToken, "t11-credential-2", `{"scopes":["game.events.write"]}`)
	require.Equal(t, http.StatusCreated, rotated.Code, rotated.Body.String())
	rotatedBody := decode(rotated)
	rotatedRetry := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials",
		applicantToken, "t11-credential-2", `{"scopes":["game.events.write"]}`)
	require.Equal(t, http.StatusCreated, rotatedRetry.Code, rotatedRetry.Body.String())
	require.Equal(t, rotatedBody, decode(rotatedRetry))
	rotatedID, err := uuid.Parse(rotatedBody["credential_id"].(string))
	require.NoError(t, err)
	rotatedSecret := rotatedBody["secret"].(string)
	require.NotEqual(t, credentialID, rotatedID)
	require.Equal(t, float64(2), rotatedBody["generation"])
	_, err = store.VerifyCredential(ctx, secret, "game.events.write", handler.CredentialKey)
	require.NoError(t, err, "rotation keeps the prior credential valid during its bounded overlap")
	_, err = store.VerifyCredential(ctx, rotatedSecret, "game.events.write", handler.CredentialKey)
	require.NoError(t, err)
	var priorCredentialWithinOverlap bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT expires_at <= now() + interval '10 minutes' FROM service_credentials WHERE id=$1`, credentialID).Scan(&priorCredentialWithinOverlap))
	require.True(t, priorCredentialWithinOverlap, "rotation must bound prior credential overlap to ten minutes")

	revoked := call(http.MethodDelete,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials/"+rotatedID.String(),
		applicantToken, "", "")
	require.Equal(t, http.StatusNoContent, revoked.Code)
	_, err = store.VerifyCredential(ctx, rotatedSecret, "game.events.write", handler.CredentialKey)
	require.ErrorIs(t, err, registry.ErrInvalidServiceCredential, "revocation ends admission immediately")
	var firstRevokeAuditCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1 AND environment_id=$2 AND action='revoke_credential' AND operation_key=$3`, appID, envID, rotatedID.String()).Scan(&firstRevokeAuditCount))
	require.Equal(t, 1, firstRevokeAuditCount, "first revoke must append one audit event")
	revokedRetry := call(http.MethodDelete,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials/"+rotatedID.String(),
		applicantToken, "", "")
	require.Equal(t, http.StatusNoContent, revokedRetry.Code)
	var revokeAuditCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1 AND environment_id=$2 AND action='revoke_credential' AND operation_key=$3`, appID, envID, rotatedID.String()).Scan(&revokeAuditCount))
	require.Equal(t, 1, revokeAuditCount, "idempotent revoke must append one audit event")

	var storedDigest []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT secret_digest FROM service_credentials WHERE id=$1`, credentialID).Scan(&storedDigest))
	require.NotContains(t, string(storedDigest), secret)
	var rawCredentialColumns int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema='public' AND table_name='service_credentials'
		AND column_name IN ('secret','raw_secret','secret_value')`).Scan(&rawCredentialColumns))
	require.Zero(t, rawCredentialColumns)

	wrongApplicationCredential := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+secondAppID.String()+"/environments/"+envID.String()+"/credentials",
		applicantToken, "t11-wrong-app-credential", `{"scopes":["game.events.write"]}`)
	require.Equal(t, http.StatusForbidden, wrongApplicationCredential.Code)
	wrongOwnerCredential := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials",
		secondOwnerToken, "t11-wrong-owner-credential", `{"scopes":["game.events.write"]}`)
	require.Equal(t, http.StatusForbidden, wrongOwnerCredential.Code)
	crossEnvironmentCredential := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+secondEnvID.String()+"/credentials",
		applicantToken, "t11-cross-env-credential", `{"scopes":["game.events.write"]}`)
	require.Equal(t, http.StatusForbidden, crossEnvironmentCredential.Code)
	var prodEnvironments int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM environments WHERE application_id=$1 AND kind='production'`, appID).Scan(&prodEnvironments))
	require.Zero(t, prodEnvironments)

	if includeProductionFixture {
		// Seed the persisted production state that this API slice cannot create.
		productionEnvID := uuid.New()
		_, err = pool.Exec(ctx, `UPDATE applications SET status='active' WHERE id=$1`, appID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO environments (id, application_id, kind, status) VALUES ($1,$2,'production','active')`, productionEnvID, appID)
		require.NoError(t, err)
		productionCredential := call(http.MethodPost,
			"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+productionEnvID.String()+"/credentials",
			applicantToken, "t11-production-environment-credential", `{"scopes":["game.events.write"]}`)
		require.Equal(t, http.StatusForbidden, productionCredential.Code)

		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM environments WHERE application_id=$1 AND kind='production'`, appID).Scan(&prodEnvironments))
		require.Equal(t, 1, prodEnvironments)
	}
	var draftApplications int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM applications WHERE owner_account_id=$1 AND status='draft'`, applicant).Scan(&draftApplications))
	require.Zero(t, draftApplications)
	var createdAudit, approvedAudit, policyAudit, credentialAudit int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1 AND action='create_application'`, appID).Scan(&createdAudit))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1 AND actor_kind='operator' AND actor_id=$2 AND action='approve_sandbox'`, appID, operator).Scan(&approvedAudit))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1 AND environment_id=$2 AND action='update_sandbox_policy'`, appID, envID).Scan(&policyAudit))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM registry_audit WHERE application_id=$1 AND environment_id=$2 AND actor_kind='account' AND actor_id=$3 AND action='issue_credential'`, appID, envID, applicant).Scan(&credentialAudit))
	require.Equal(t, 1, createdAudit)
	require.Equal(t, 1, approvedAudit)
	require.Equal(t, 1, policyAudit)
	require.Equal(t, 2, credentialAudit, "initial issuance and rotation must each be audited once")

	storeNow = storeNow.Add(10*time.Minute + time.Nanosecond)
	expiredReveal := call(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/environments/"+envID.String()+"/credentials",
		applicantToken, "t11-credential-1", `{"scopes":["game.events.write"]}`)
	require.Equal(t, http.StatusConflict, expiredReveal.Code)
	require.Equal(t, "CREDENTIAL_REVEAL_EXPIRED", decode(expiredReveal)["error_code"])
}

func bootstrapAccessToken(t *testing.T, key *rsa.PrivateKey, userID uuid.UUID, accountType string) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": bootstrapKeyID, "typ": "JWT"})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]any{
		"iss": bootstrapIssuer, "aud": bootstrapAudience, "user_id": userID.String(), "profile_id": uuid.NewString(),
		"account_type": accountType, "jti": uuid.NewString(), "session_epoch": 1,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
