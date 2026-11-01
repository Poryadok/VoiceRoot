package sourcecustody

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
)

func TestActivationOperatorRequiresMTLSAndSignedOneLeaderDecision(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	// Force the same pool pressure regardless of the host CPU count: the held
	// permit, shutdown drain and competing monitor consume all three slots.
	limitedConfig := control.Config().Copy()
	limitedConfig.MaxConns = 3
	limitedConfig.MinConns = 0
	limited, err := pgxpool.NewWithConfig(ctx, limitedConfig)
	require.NoError(t, err)
	t.Cleanup(limited.Close)
	control = limited
	controller.pool = limited
	handler := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public}
	pki := processTLS(t, t.TempDir(), "operator")
	cert, err := tls.LoadX509KeyPair(pki.serverCert, pki.serverKey)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots}
	server.StartTLS()
	defer server.Close()
	client := &http.Client{Timeout: 90 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots, Certificates: []tls.Certificate{pki.client}}}}
	defer client.CloseIdleConnections()
	unauthenticated := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots}}}
	defer unauthenticated.CloseIdleConnections()
	_, err = unauthenticated.Get(server.URL + "/challenge")
	require.Error(t, err)
	response, err := client.Get(server.URL + "/challenge")
	require.NoError(t, err)
	var challenge activationChallenge
	require.NoError(t, json.NewDecoder(response.Body).Decode(&challenge))
	require.NoError(t, response.Body.Close())
	require.Equal(t, claim.Challenge, challenge.Challenge)
	require.Equal(t, claim.Binding, challenge.Binding)
	post := func(payload []byte) int {
		t.Helper()
		response, err := client.Post(server.URL+"/open", "application/json", bytes.NewReader(payload))
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, response.Body)
		require.NoError(t, response.Body.Close())
		return response.StatusCode
	}
	bad := claim
	bad.Signature = append([]byte(nil), claim.Signature...)
	bad.Signature[0] ^= 1
	encoded, err := json.Marshal(bad)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, post(encoded))
	require.Equal(t, http.StatusBadRequest, post([]byte(`{"unexpected":true}`)))
	encoded, err = json.Marshal(claim)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, post(encoded))
	require.Equal(t, http.StatusServiceUnavailable, post(encoded), "the decision is one-use")
	permit, physical, err := controller.AcquireAttested(ctx, claim.Binding)
	require.NoError(t, err)
	require.Equal(t, claim.Physical, physical)
	require.NoError(t, permit.Release(ctx))
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "ACTIVE", phase)
	writerConfig := source.Config().Copy()
	writerConfig.ConnConfig.User = "custody_app"
	writerConfig.ConnConfig.Password = "fixture_only"
	writer, err := pgxpool.NewWithConfig(ctx, writerConfig)
	require.NoError(t, err)
	defer writer.Close()
	require.NoError(t, writer.Ping(ctx))
	invalidClose, err := client.Post(server.URL+"/close", "application/json", bytes.NewBufferString("x"))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, invalidClose.StatusCode)
	require.NoError(t, invalidClose.Body.Close())
	held, _, err := controller.AcquireAttested(ctx, claim.Binding)
	require.NoError(t, err)
	defer func() { _ = held.Release(context.Background()) }()
	type closeResult struct {
		response *http.Response
		err      error
	}
	result := make(chan closeResult, 1)
	go func() {
		response, err := client.Post(server.URL+"/close", "application/json", nil)
		result <- closeResult{response, err}
	}()
	require.Eventually(t, func() bool {
		var state string
		return control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state) == nil && state == "CLOSED"
	}, 5*time.Second, 25*time.Millisecond)
	select {
	case premature := <-result:
		if premature.response != nil {
			_ = premature.response.Body.Close()
		}
		t.Fatal("operator close returned while a capture permit was still held")
	case <-time.After(150 * time.Millisecond):
	}
	// A separately running monitor may acquire leadership immediately after
	// Shutdown releases it. Close must wait for that fence, not report a
	// spurious failure while the competing recovery holds the lock.
	monitorConn, err := control.Acquire(ctx)
	require.NoError(t, err)
	// The controller and held permit can exhaust the control pool while close
	// drains. Observe the waiter through an independent connection so the test
	// does not depend on the pool having an extra spare slot.
	observer, err := pgxpool.NewWithConfig(ctx, control.Config().Copy())
	require.NoError(t, err)
	defer observer.Close()
	monitorPID := monitorConn.Conn().PgConn().PID()
	waitCtx, stopWait := context.WithCancel(ctx)
	lockResult := make(chan error, 1)
	go func() {
		_, lockErr := monitorConn.Exec(waitCtx, `SELECT pg_advisory_lock($1::bigint)`, leaderKey)
		lockResult <- lockErr
	}()
	observed, acquired := false, false
	t.Cleanup(func() {
		stopWait()
		if !observed {
			acquired = <-lockResult == nil
		}
		if acquired {
			_, _ = monitorConn.Exec(context.Background(), `SELECT pg_advisory_unlock($1::bigint)`, leaderKey)
		}
		monitorConn.Release()
	})
	require.Eventually(t, func() bool {
		var waiting bool
		return observer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted)`, monitorPID).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 25*time.Millisecond)
	require.NoError(t, held.Release(ctx))
	select {
	case lockErr := <-lockResult:
		observed = true
		require.NoError(t, lockErr)
		acquired = true
	case <-time.After(5 * time.Second):
		t.Fatal("monitor did not acquire leadership")
	}
	select {
	case premature := <-result:
		if premature.response != nil {
			_ = premature.response.Body.Close()
		}
		t.Fatal("operator close failed while monitor held leadership")
	case <-time.After(150 * time.Millisecond):
	}
	_, err = monitorConn.Exec(ctx, `SELECT pg_advisory_unlock($1::bigint)`, leaderKey)
	require.NoError(t, err)
	acquired = false
	var closed *http.Response
	select {
	case outcome := <-result:
		require.NoError(t, outcome.err)
		closed = outcome.response
	case <-time.After(20 * time.Second):
		t.Fatal("operator close failed to drain and fence")
	}
	require.Equal(t, http.StatusNoContent, closed.StatusCode)
	require.NoError(t, closed.Body.Close())
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	var allowed, login bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, allowed)
	require.False(t, login)
	require.Eventually(t, func() bool {
		bounded, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return writer.Ping(bounded) != nil
	}, 5*time.Second, 50*time.Millisecond)
}

func TestActivationOperatorFencesWhenSourceOpeningFails(t *testing.T) {
	ctx, control, admin, _, keys, controller, claim, public, _, _ := openSourceFixture(t)
	// This pool points at the maintenance database, so the physical User
	// source transaction cannot begin after preparation has enabled access.
	wrongSource, err := pgxpool.NewWithConfig(ctx, admin.Config().Copy())
	require.NoError(t, err)
	defer wrongSource.Close()
	handler := &activationHandler{controller: controller, admin: admin, source: wrongSource, keys: keys, signer: public}
	encoded, err := json.Marshal(claim)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "https://operator/open", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.TLS = &tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	select {
	case <-controller.failure:
	case <-time.After(time.Second):
		t.Fatal("failed activation must terminate the custody process")
	}
	var phase, state string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "RECOVERY_REQUIRED", phase)
	require.Equal(t, "CLOSED", state)
	var allowed, login bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, allowed)
	require.False(t, login)
}

func TestActivationOperatorSignedCutoverActivatesGuardsBeforeOpening(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, profile, account := openSourceFixtureOptions(t, false)
	handler := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public}
	encoded, err := json.Marshal(claim)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "https://operator/open", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.TLS = &tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "OPEN", state)
	var active bool
	require.NoError(t, source.QueryRow(ctx, `SELECT guards_active FROM identity_meta WHERE singleton`).Scan(&active))
	require.True(t, active)
	config := source.Config().Copy()
	config.ConnConfig.User = "custody_app"
	config.ConnConfig.Password = "fixture_only"
	app, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer app.Close()
	identity, err := identityassociation.NewStore(app, keys, claim.Binding.HistoryGeneration)
	require.NoError(t, err)
	result, err := identity.CaptureProfilesForPhysicalSource(ctx, []uuid.UUID{profile}, claim.Binding.HistoryGeneration, claim.Binding.UserSourceGeneration, claim.Binding.KeysetDigest, identityassociation.PhysicalSourceIdentity{SystemIdentifier: claim.Physical.SystemIdentifier, Database: claim.Physical.Database, DatabaseOID: claim.Physical.DatabaseOID})
	require.NoError(t, err)
	require.Equal(t, account, result.Observations[profile].AccountID)
}

func TestActivationOperatorCloseFinishesAfterCallerDisconnects(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	handler := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public, opened: &claim}
	request := httptest.NewRequest(http.MethodPost, "https://operator/close", nil)
	request.TLS = &tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}
	disconnected, cancel := context.WithCancel(request.Context())
	cancel()
	request = request.WithContext(disconnected)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	var phase string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, allowed)
	require.False(t, login)
}

func TestActivationOperatorShutdownFencesOpenedSource(t *testing.T) {
	var stopParent context.CancelFunc
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixtureWithParent(t, true, func(parent context.Context) context.Context {
		var lifetime context.Context
		lifetime, stopParent = context.WithCancel(parent)
		return lifetime
	})
	defer stopParent()
	handler := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public}
	encoded, err := json.Marshal(claim)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "https://operator/open", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.TLS = &tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	observer, err := pgxpool.NewWithConfig(ctx, admin.Config().Copy())
	require.NoError(t, err)
	defer observer.Close()
	op := &activationOperator{server: &http.Server{Handler: handler}, admin: admin, source: source}
	held, _, err := controller.AcquireAttested(ctx, claim.Binding)
	require.NoError(t, err)
	stopParent()
	done := make(chan error, 1)
	go func() { done <- op.close() }()
	joined := false
	defer func() {
		_ = held.Release(context.Background())
		if !joined {
			<-done
		}
	}()
	require.Eventually(t, func() bool {
		var state string
		return control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state) == nil && state == "CLOSED"
	}, 5*time.Second, 25*time.Millisecond)
	select {
	case err := <-done:
		joined = true
		t.Fatalf("operator shutdown returned while a capture permit was held: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	require.NoError(t, held.Release(ctx))
	select {
	case err := <-done:
		joined = true
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("operator shutdown did not fence after the permit drained")
	}
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	var allowed, login bool
	require.NoError(t, observer.QueryRow(ctx, `SELECT datallowconn,(SELECT rolcanlogin FROM pg_roles WHERE rolname=$2) FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID, claim.WriterRole).Scan(&allowed, &login))
	require.False(t, allowed)
	require.False(t, login)
}

func TestActivationOperatorShutdownReportsFenceFailure(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	handler := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public, opened: &claim}
	observer, err := pgxpool.NewWithConfig(ctx, admin.Config().Copy())
	require.NoError(t, err)
	defer observer.Close()
	admin.Close()
	op := &activationOperator{server: &http.Server{Handler: handler}, admin: admin, source: source}
	require.Error(t, op.close(), "shutdown must report an incomplete physical fence")
	var state string
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	var allowed, login bool
	require.NoError(t, observer.QueryRow(ctx, `SELECT datallowconn,(SELECT rolcanlogin FROM pg_roles WHERE rolname=$2) FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID, claim.WriterRole).Scan(&allowed, &login))
	require.True(t, allowed)
	require.True(t, login)
	require.NotNil(t, handler.opened, "failed physical recovery must retain its claim")
	// A later recovery attempt can use restored maintenance access. It still
	// reports the earlier uncertain shutdown even when fencing now succeeds.
	handler.admin = observer
	handler.mu.Lock()
	retryErr := handler.fenceOpenedLocked()
	handler.mu.Unlock()
	require.Error(t, retryErr)
	require.Nil(t, handler.opened)
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, observer.QueryRow(ctx, `SELECT datallowconn,(SELECT rolcanlogin FROM pg_roles WHERE rolname=$2) FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID, claim.WriterRole).Scan(&allowed, &login))
	require.False(t, allowed)
	require.False(t, login)
}

func TestActivationOperatorFencesAfterPermitDrainTimeout(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	held, _, err := controller.AcquireAttested(ctx, claim.Binding)
	require.NoError(t, err)
	handler := &activationHandler{controller: controller, admin: admin, source: source, keys: keys, signer: public, opened: &claim}
	done := make(chan error, 1)
	joined := false
	defer func() {
		_ = held.Release(context.Background())
		if !joined {
			<-done
		}
	}()
	go func() {
		handler.mu.Lock()
		defer handler.mu.Unlock()
		done <- handler.fenceOpenedLocked()
	}()
	select {
	case err := <-done:
		joined = true
		require.Error(t, err, "a timed-out drain must not report a clean shutdown")
	case <-time.After(80 * time.Second):
		t.Fatal("shutdown did not finish its bounded drain and fence")
	}
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase, "the physical fence must still run after the drain timeout")
	var allowed, login bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn,(SELECT rolcanlogin FROM pg_roles WHERE rolname=$2) FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID, claim.WriterRole).Scan(&allowed, &login))
	require.False(t, allowed)
	require.False(t, login)
}

func TestActivationOperatorConfigIsAllOrNothing(t *testing.T) {
	values := map[string]string{}
	lookup := func(key string) (string, bool) { value, ok := values[key]; return value, ok }
	config, err := loadActivationOperatorConfig(lookup)
	require.NoError(t, err)
	require.Nil(t, config)
	values["CUSTODY_OPERATOR_LISTEN"] = "127.0.0.1:4444"
	_, err = loadActivationOperatorConfig(lookup)
	require.Error(t, err)
	for _, key := range []string{"CUSTODY_OPERATOR_TLS_CERT_FILE", "CUSTODY_OPERATOR_TLS_KEY_FILE", "CUSTODY_OPERATOR_CLIENT_CA_FILE", "CUSTODY_OPERATOR_SIGNER_PUBLIC_KEY_FILE", "CUSTODY_USER_KEY_MANIFEST", "CUSTODY_USER_KEYS_DIR"} {
		values[key] = "fixture"
	}
	values["CUSTODY_OPERATOR_CLIENT_CERT_SHA256"] = strings.Repeat("ab", 32)
	values["CUSTODY_SOURCE_ADMIN_DATABASE_URL"] = "postgres://operator:secret@localhost:5432/postgres?sslmode=verify-full&sslrootcert=fixture"
	values["CUSTODY_SOURCE_DATABASE_URL"] = "postgres://operator:secret@localhost:5432/user_db?sslmode=verify-full&sslrootcert=fixture"
	config, err = loadActivationOperatorConfig(lookup)
	require.NoError(t, err)
	require.NotNil(t, config)
	values["CUSTODY_OPERATOR_CLIENT_CERT_SHA256"] = strings.Repeat("00", 32)
	_, err = loadActivationOperatorConfig(lookup)
	require.Error(t, err)
	values["CUSTODY_OPERATOR_CLIENT_CERT_SHA256"] = strings.Repeat("ab", 32)
	values["CUSTODY_SOURCE_DATABASE_URL"] = "postgres://operator:secret@localhost:5432/user_db?sslmode=disable"
	_, err = loadActivationOperatorConfig(lookup)
	require.Error(t, err)
}

func TestOperatorClientCertificatePin(t *testing.T) {
	pki := processTLS(t, t.TempDir(), "operator-pin")
	cert, err := x509.ParseCertificate(pki.client.Certificate[0])
	require.NoError(t, err)
	pin := sha256.Sum256(cert.Raw)
	state := tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}, VerifiedChains: [][]*x509.Certificate{{cert}}}
	require.True(t, validOperatorPeer(state, pin))
	pin[0] ^= 1
	require.False(t, validOperatorPeer(state, pin))
	state.VerifiedChains = nil
	require.False(t, validOperatorPeer(state, sha256.Sum256(cert.Raw)))
}
