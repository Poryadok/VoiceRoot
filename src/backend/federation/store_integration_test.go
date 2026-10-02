package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestPostgresAuthorityLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL testcontainer")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	require.NoError(t, migrate(ctx, pool))
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	s := &authorityStore{Pool: pool, Key: key, KeyID: "test", Issuer: "master", Environment: "sandbox"}
	node, other, space := uuid.NewString(), uuid.NewString(), uuid.NewString()
	pin, otherPin := digest([]byte("node-cert")), digest([]byte("other-cert"))
	require.NoError(t, s.enroll(ctx, node, uuid.NewString(), "https://node.test", pin))
	require.ErrorIs(t, s.place(ctx, node, space), errForbidden)
	cred, err := s.changeNode(ctx, node, "approve", "", "operator")
	require.NoError(t, err)
	require.NoError(t, s.enroll(ctx, other, uuid.NewString(), "https://other.test", otherPin))
	_, err = s.changeNode(ctx, other, "approve", "", "operator")
	require.NoError(t, err)
	require.NoError(t, s.place(ctx, node, space))
	require.NoError(t, s.place(ctx, node, space))
	require.ErrorIs(t, s.place(ctx, other, space), errConflict)
	snap := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 1, ValidUntil: time.Now().Add(1500 * time.Millisecond).UnixMilli(), Permissions: []Permission{}}
	require.NoError(t, s.publish(ctx, node, space, snap))
	require.NoError(t, s.publish(ctx, node, space, snap))
	// Exercise the actual API authorization path with the credential issued by
	// changeNode, including the node's verified client certificate.
	nodeCert := &x509.Certificate{Raw: []byte("node-cert"), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	api := &authorityAPI{Store: s, Operators: map[string]bool{}}
	apiRequest := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://master"+path, strings.NewReader(body))
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{nodeCert}, VerifiedChains: [][]*x509.Certificate{{nodeCert}}}
		r.Header.Set("Authorization", "Bearer "+cred.Secret)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	basePath := "/v1/nodes/" + node + "/spaces/" + space
	apiSnapshot := apiRequest("GET", basePath+"/snapshot", "")
	require.Equal(t, 200, apiSnapshot.Code, apiSnapshot.Body.String())
	apiAckRaw, err := json.Marshal(leaseRequest{Revision: 1, Hash: digest(mustJSON(t, snap)), Nonce: uuid.NewString()})
	require.NoError(t, err)
	apiLease := apiRequest("POST", basePath+"/lease", string(apiAckRaw))
	require.Equal(t, 200, apiLease.Code, apiLease.Body.String())
	changed := snap
	changed.ValidUntil++
	require.ErrorIs(t, s.publish(ctx, node, space, changed), errConflict)
	changed = snap
	changed.Revision = 3
	require.ErrorIs(t, s.publish(ctx, node, space, changed), errConflict)
	for _, attempt := range []struct{ node, space, pin, secret string }{{node, space, otherPin, cred.Secret}, {node, space, pin, "wrong"}, {other, space, pin, cred.Secret}, {node, uuid.NewString(), pin, cred.Secret}} {
		_, err = s.issue(ctx, attempt.node, attempt.space, attempt.pin, attempt.secret, nil)
		require.ErrorIs(t, err, errForbidden)
	}
	envelope, err := s.issue(ctx, node, space, pin, cred.Secret, nil)
	require.NoError(t, err)
	require.NotEmpty(t, envelope.Signature)
	raw, err := json.Marshal(snap)
	require.NoError(t, err)
	ack := leaseRequest{Revision: 1, Hash: digest(raw), Nonce: uuid.NewString()}
	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, issueErr := s.issue(ctx, node, space, pin, cred.Secret, &ack)
			results <- issueErr
		}()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for issueErr := range results {
		if issueErr == nil {
			successes++
		} else if errors.Is(issueErr, errConflict) {
			conflicts++
		} else {
			t.Fatalf("concurrent lease issue: %v", issueErr)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	_, err = s.issue(ctx, node, space, pin, cred.Secret, &ack)
	require.ErrorIs(t, err, errConflict)
	changed = snap
	changed.Revision = 2
	changed.ValidUntil = time.Now().Add(1500 * time.Millisecond).UnixMilli()
	require.NoError(t, s.publish(ctx, node, space, changed))
	ack.Nonce = uuid.NewString()
	_, err = s.issue(ctx, node, space, pin, cred.Secret, &ack)
	require.ErrorIs(t, err, errConflict)
	// A new store instance uses persisted registry and snapshot, with no warm cache.
	restarted := *s
	_, err = restarted.issue(ctx, node, space, pin, cred.Secret, nil)
	require.NoError(t, err)
	rotated, err := s.changeNode(ctx, node, "rotate", otherPin+"invalid", "operator")
	require.Error(t, err)
	require.Empty(t, rotated.Secret)
	rotated, err = s.changeNode(ctx, node, "rotate", otherPin, "operator")
	require.ErrorIs(t, err, errConflict)
	require.Empty(t, rotated.Secret)
	_, err = s.issue(ctx, node, space, pin, cred.Secret, nil)
	require.NoError(t, err, "duplicate-pin rotation must preserve old credential")
	rotated, err = s.changeNode(ctx, node, "rotate", digest([]byte("new-cert")), "operator")
	require.NoError(t, err)
	_, err = s.issue(ctx, node, space, pin, cred.Secret, nil)
	require.ErrorIs(t, err, errForbidden)
	_, err = s.issue(ctx, node, space, digest([]byte("new-cert")), rotated.Secret, nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE federation_placements SET valid_until=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = s.issue(ctx, node, space, digest([]byte("new-cert")), rotated.Secret, nil)
	require.ErrorIs(t, err, errForbidden)
	// Refresh a valid source snapshot, then race lease issuance with revocation.
	changed.Revision = 3
	changed.ValidUntil = time.Now().Add(1500 * time.Millisecond).UnixMilli()
	require.NoError(t, s.publish(ctx, node, space, changed))
	raw, err = json.Marshal(changed)
	require.NoError(t, err)
	ack = leaseRequest{Revision: 3, Hash: digest(raw), Nonce: uuid.NewString()}
	start := make(chan struct{})
	issueResult := make(chan error, 1)
	revokeResult := make(chan error, 1)
	go func() {
		<-start
		_, raceErr := s.issue(ctx, node, space, digest([]byte("new-cert")), rotated.Secret, &ack)
		issueResult <- raceErr
	}()
	go func() {
		<-start
		_, raceErr := s.changeNode(ctx, node, "suspend", "", "operator")
		revokeResult <- raceErr
	}()
	close(start)
	issueErr, revokeErr := <-issueResult, <-revokeResult
	require.NoError(t, revokeErr, "revocation must complete regardless of which transaction gets the row lock first")
	require.True(t, issueErr == nil || errors.Is(issueErr, errForbidden) || errors.Is(issueErr, errConflict), "racing issue failed unexpectedly: %v", issueErr)
	_, err = s.changeNode(ctx, node, "suspend", "", "operator")
	require.NoError(t, err)
	_, err = s.issue(ctx, node, space, digest([]byte("new-cert")), rotated.Secret, nil)
	require.ErrorIs(t, err, errForbidden)
	_, err = s.changeNode(ctx, node, "approve", "", "operator")
	require.ErrorIs(t, err, errConflict)
	_, err = s.changeNode(ctx, node, "defederate", "", "operator")
	require.NoError(t, err)
	_, err = s.changeNode(ctx, node, "rotate", pin, "operator")
	require.ErrorIs(t, err, errConflict)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestAPIBoundariesBeforeDatabase(t *testing.T) {
	now := time.Now()
	operator := &x509.Certificate{Raw: []byte("operator"), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	node := &x509.Certificate{Raw: []byte("node"), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	api := &authorityAPI{Operators: map[string]bool{digest(operator.Raw): true}}
	for _, tt := range []struct {
		name, method, path, body string
		cert                     *x509.Certificate
		code                     int
	}{
		{"no certificate", "POST", "/v1/nodes", "{}", nil, 403},
		{"node cannot enroll", "POST", "/v1/nodes", "{}", node, 403},
		{"operator cannot read node snapshot", "GET", "/v1/nodes/" + uuid.NewString() + "/spaces/" + uuid.NewString() + "/snapshot", "", operator, 503},
		{"missing attestation", "POST", "/v1/nodes/" + uuid.NewString() + "/approve", "{}", operator, 400},
		{"unknown field", "POST", "/v1/nodes/" + uuid.NewString() + "/approve", `{"ownership_verified":true,"admin":true}`, operator, 400},
		{"trailing body", "POST", "/v1/nodes/" + uuid.NewString() + "/approve", `{"ownership_verified":true}{}`, operator, 400},
		{"node cannot suspend", "POST", "/v1/nodes/" + uuid.NewString() + "/suspend", "{}", node, 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "https://master"+tt.path, strings.NewReader(tt.body))
			if tt.cert != nil {
				r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{tt.cert}, VerifiedChains: [][]*x509.Certificate{{tt.cert}}}
			}
			w := httptest.NewRecorder()
			api.ServeHTTP(w, r)
			require.Equal(t, tt.code, w.Code)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			if tt.name == "operator cannot read node snapshot" {
				requestID, parseErr := uuid.Parse(w.Header().Get("X-Request-ID"))
				require.NoError(t, parseErr)
				require.NotEqual(t, uuid.Nil, requestID)
				require.JSONEq(t, `{"error":"unavailable"}`, w.Body.String())
			}
		})
	}
}
