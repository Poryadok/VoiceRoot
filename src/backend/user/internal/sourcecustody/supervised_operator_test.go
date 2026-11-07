package sourcecustody

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperatorConfigRequiresIndependentSupervisor(t *testing.T) {
	values := map[string]string{
		"CUSTODY_OPERATOR_LISTEN":             "127.0.0.1:4444",
		"CUSTODY_OPERATOR_CLIENT_CERT_SHA256": strings.Repeat("ab", 32),
		"CUSTODY_SOURCE_ADMIN_DATABASE_URL":   "postgres://operator:secret@localhost:5432/postgres?sslmode=verify-full&sslrootcert=fixture",
		"CUSTODY_SOURCE_DATABASE_URL":         "postgres://operator:secret@localhost:5432/user_db?sslmode=verify-full&sslrootcert=fixture",
	}
	for _, name := range []string{"CUSTODY_OPERATOR_TLS_CERT_FILE", "CUSTODY_OPERATOR_TLS_KEY_FILE", "CUSTODY_OPERATOR_CLIENT_CA_FILE", "CUSTODY_OPERATOR_SIGNER_PUBLIC_KEY_FILE", "CUSTODY_USER_KEY_MANIFEST", "CUSTODY_USER_KEYS_DIR"} {
		values[name] = "fixture"
	}
	_, err := loadActivationOperatorConfig(func(name string) (string, bool) { value, ok := values[name]; return value, ok })
	require.Error(t, err, "production operator configuration must include a separate authenticated supervisor")
}

func TestActivationOperatorRequiresIndependentSupervisor(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	handler := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public}
	body, err := json.Marshal(claim)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "https://custody/open", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.TLS = &tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusServiceUnavailable, response.Code, "a custodian signature alone must not open production admission")
	var state string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, allowed)
	require.False(t, login)
}
