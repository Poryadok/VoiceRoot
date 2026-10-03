package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/nodecache"
	"voice/backend/federation/protocol"
	"voice/backend/pkg/integrationtest"
)

func TestQ11FederationCleanStartAuthorityAPIs(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL testcontainer")
	}
	integrationtest.ConfigureDockerTesting()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "federation_db", "")
	require.NoError(t, migrate(ctx, pool))
	_, signingKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca, serverCert, operatorCert, nodeCert, otherNodeCert, wrongNodeCert := q11Certificates(t)
	api := &authorityAPI{Store: &authorityStore{Pool: pool, Key: signingKey, KeyID: "q11-test", Issuer: "test-master", Environment: "sandbox"}, Operators: map[string]bool{q11Fingerprint(operatorCert.Leaf): true}}
	server := httptest.NewUnstartedServer(api)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: ca, MinVersion: tls.VersionTLS13}
	server.StartTLS()
	t.Cleanup(server.Close)
	clientFor := func(cert tls.Certificate) *http.Client {
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: ca, Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}}}
	}
	operatorClient, nodeClient, wrongNodeClient := clientFor(operatorCert), clientFor(nodeCert), clientFor(wrongNodeCert)
	request := func(client *http.Client, method, path string, body any, requestID string) (*http.Response, []byte) {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, err = json.Marshal(body)
			require.NoError(t, err)
		}
		req, reqErr := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(raw))
		require.NoError(t, reqErr)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if requestID != "" {
			req.Header.Set("X-Request-ID", requestID)
		}
		resp, reqErr := client.Do(req)
		require.NoError(t, reqErr)
		defer resp.Body.Close()
		var response []byte
		response, reqErr = io.ReadAll(resp.Body)
		require.NoError(t, reqErr)
		return resp, response
	}
	post := func(client *http.Client, path string, body any, id string) (*http.Response, []byte) {
		return request(client, http.MethodPost, path, body, id)
	}
	newID := func() string { return uuid.NewString() }
	nodeA, nodeB, spaceA, spaceB, spaceC := newID(), newID(), newID(), newID(), newID()
	operatorID := newID()
	resp, responseBody := post(operatorClient, "/v1/nodes", map[string]string{"node_id": nodeA, "operator_id": operatorID, "endpoint": "https://node-a.test", "certificate_sha256": q11Fingerprint(nodeCert.Leaf)}, newID())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(responseBody))
	resp, responseBody = post(operatorClient, "/v1/nodes", map[string]string{"node_id": nodeB, "operator_id": operatorID, "endpoint": "https://node-b.test", "certificate_sha256": q11Fingerprint(otherNodeCert.Leaf)}, newID())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(responseBody))
	approvedA, raw := post(operatorClient, "/v1/nodes/"+nodeA+"/approve", map[string]bool{"ownership_verified": true}, newID())
	require.Equal(t, http.StatusOK, approvedA.StatusCode, string(raw))
	var credentialA credential
	require.NoError(t, json.Unmarshal(raw, &credentialA))
	approvedB, raw := post(operatorClient, "/v1/nodes/"+nodeB+"/approve", map[string]bool{"ownership_verified": true}, newID())
	require.Equal(t, http.StatusOK, approvedB.StatusCode, string(raw))
	var credentialB credential
	require.NoError(t, json.Unmarshal(raw, &credentialB))
	resp, responseBody = post(operatorClient, "/v1/nodes/"+nodeA+"/spaces/"+spaceA, map[string]any{}, newID())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(responseBody))
	resp, responseBody = post(operatorClient, "/v1/nodes/"+nodeA+"/spaces/"+spaceB, map[string]any{}, newID())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(responseBody))
	resp, responseBody = post(operatorClient, "/v1/nodes/"+nodeB+"/spaces/"+newID(), map[string]any{}, newID())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(responseBody))
	snapshots := map[string]Snapshot{}
	for _, pair := range []struct{ node, space string }{{nodeA, spaceA}, {nodeA, spaceB}} {
		permissions := make([]Permission, protocol.SnapshotPagePermissionLimit+1)
		for index := range permissions {
			permissions[index] = Permission{AccountID: newID(), ProfileID: newID(), ResourceID: newID(), SessionEpoch: 1, Actions: []string{"read"}}
		}
		snapshot := Snapshot{Version: 1, Complete: true, PageCount: 2, Revision: 1, ValidUntil: time.Now().Add(1500 * time.Millisecond).UnixMilli(), Permissions: permissions}
		snapshots[pair.space] = snapshot
		resp, response := post(operatorClient, "/v1/nodes/"+pair.node+"/spaces/"+pair.space+"/snapshot", snapshot, newID())
		require.Equal(t, http.StatusOK, resp.StatusCode, string(response))
	}
	// A normal node request carries the issued, scoped credential and obtains the signed snapshot.
	baseA := "/v1/nodes/" + nodeA + "/spaces/" + spaceA
	getWithBearer := func(client *http.Client, path, id, bearer string) (*http.Response, []byte) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
		require.NoError(t, reqErr)
		req.Header.Set("X-Request-ID", id)
		req.Header.Set("Authorization", "Bearer "+bearer)
		response, reqErr := client.Do(req)
		require.NoError(t, reqErr)
		defer response.Body.Close()
		data, reqErr := io.ReadAll(response.Body)
		require.NoError(t, reqErr)
		return response, data
	}
	resp, raw = getWithBearer(nodeClient, baseA+"/snapshot", newID(), credentialA.Secret)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	var signed Envelope
	require.NoError(t, json.Unmarshal(raw, &signed))
	payload, err := base64.RawURLEncoding.DecodeString(signed.Payload)
	require.NoError(t, err)
	var claims Claims
	require.NoError(t, json.Unmarshal(payload, &claims))
	signature, err := base64.RawURLEncoding.DecodeString(signed.Signature)
	require.NoError(t, err)
	require.True(t, ed25519.Verify(signingKey.Public().(ed25519.PublicKey), payload, signature))
	require.Equal(t, "snapshot_manifest", claims.Kind)
	require.NotNil(t, claims.Manifest)
	cache := nodecache.New(protocol.Scope{Issuer: claims.Issuer, Environment: claims.Environment, NodeID: claims.NodeID, SpaceID: claims.SpaceID, Generation: claims.Generation, Epoch: claims.Epoch}, map[string]ed25519.PublicKey{"q11-test": signingKey.Public().(ed25519.PublicKey)})
	require.NoError(t, cache.StageManifest(signed, time.Now()))
	var pageEnvelope Envelope
	for pageIndex := 0; pageIndex < claims.Manifest.PageCount; pageIndex++ {
		pageResp, pageRaw := getWithBearer(nodeClient, baseA+"/snapshot/pages/"+strconv.Itoa(pageIndex), newID(), credentialA.Secret)
		require.Equal(t, http.StatusOK, pageResp.StatusCode, string(pageRaw))
		require.NoError(t, json.Unmarshal(pageRaw, &pageEnvelope))
		require.NoError(t, cache.StagePage(pageEnvelope, time.Now()))
	}
	published := cache.Current()
	require.Equal(t, snapshots[spaceA], published)

	// The ordered event announces the committed revision; policy remains old
	// until the complete signed page set is staged and verified.
	secondSnapshot := snapshots[spaceA]
	secondSnapshot.Revision = 2
	secondSnapshot.ValidUntil = time.Now().Add(1500 * time.Millisecond).UnixMilli()
	resp, raw = post(operatorClient, baseA+"/snapshot", secondSnapshot, newID())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	revisionResp, revisionRaw := getWithBearer(nodeClient, baseA+"/revisions?after_revision=1", newID(), credentialA.Secret)
	require.Equal(t, http.StatusOK, revisionResp.StatusCode, string(revisionRaw))
	var revisionEnvelope Envelope
	require.NoError(t, json.Unmarshal(revisionRaw, &revisionEnvelope))
	require.NoError(t, cache.ObserveRevisionStream(revisionEnvelope, time.Now()))
	resp, raw = getWithBearer(nodeClient, baseA+"/snapshot", newID(), credentialA.Secret)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	require.NoError(t, json.Unmarshal(raw, &signed))
	require.NoError(t, cache.StageManifest(signed, time.Now()))
	require.NoError(t, json.Unmarshal(raw, &claims))
	for pageIndex := 0; pageIndex < claims.Manifest.PageCount; pageIndex++ {
		pageResp, pageRaw := getWithBearer(nodeClient, baseA+"/snapshot/pages/"+strconv.Itoa(pageIndex), newID(), credentialA.Secret)
		require.Equal(t, http.StatusOK, pageResp.StatusCode, string(pageRaw))
		require.NoError(t, json.Unmarshal(pageRaw, &pageEnvelope))
		require.NoError(t, cache.StagePage(pageEnvelope, time.Now()))
	}
	published = cache.Current()
	require.Equal(t, secondSnapshot, published)
	ack, err := cache.LeaseAck(newID())
	require.NoError(t, err)
	leaseBody := leaseRequest{Revision: ack.Revision, Hash: ack.Hash, Nonce: ack.Nonce}
	leaseResp, leaseRaw := q11Request(t, ctx, nodeClient, server.URL, http.MethodPost, baseA+"/lease", leaseBody, newID(), credentialA.Secret)
	require.Equal(t, http.StatusOK, leaseResp.StatusCode, string(leaseRaw))
	var leaseEnvelope Envelope
	require.NoError(t, json.Unmarshal(leaseRaw, &leaseEnvelope))
	leasePayload, err := base64.RawURLEncoding.DecodeString(leaseEnvelope.Payload)
	require.NoError(t, err)
	leaseSignature, err := base64.RawURLEncoding.DecodeString(leaseEnvelope.Signature)
	require.NoError(t, err)
	require.True(t, ed25519.Verify(signingKey.Public().(ed25519.PublicKey), leasePayload, leaseSignature))
	var leaseClaims Claims
	require.NoError(t, json.Unmarshal(leasePayload, &leaseClaims))
	require.Equal(t, "lease", leaseClaims.Kind)
	require.Equal(t, published.Revision, leaseClaims.Revision)

	// Each Q11-enumerated HTTP denial carries a request ID and must be persisted safely.
	type denied struct {
		name, method, path, requestID, bearer string
		actorClass, reasonCode                string
		client                                *http.Client
		body                                  any
		status                                int
		action                                string
		actorFingerprint                      string
		nodeID, spaceID                       *string
	}
	unknownApprovalNode := newID()
	denials := make([]denied, 0, 8)
	denials = append(denials,
		denied{name: "operator certificate on node route", method: http.MethodGet, path: baseA + "/snapshot", requestID: newID(), actorClass: "operator", reasonCode: "certificate_role_mismatch", client: operatorClient, status: http.StatusForbidden, action: "node.snapshot.read", actorFingerprint: q11Fingerprint(operatorCert.Leaf), nodeID: &nodeA, spaceID: &spaceA},
		denied{name: "node certificate on operator route", method: http.MethodPost, path: "/v1/nodes/" + unknownApprovalNode + "/approve", requestID: newID(), actorClass: "node", reasonCode: "certificate_role_mismatch", client: nodeClient, bearer: credentialA.Secret, body: map[string]bool{"ownership_verified": true}, status: http.StatusForbidden, action: "node.approve", actorFingerprint: q11Fingerprint(nodeCert.Leaf), nodeID: &unknownApprovalNode},
		denied{name: "trusted but unregistered certificate", method: http.MethodGet, path: baseA + "/snapshot", requestID: newID(), actorClass: "node", reasonCode: "node_certificate_mismatch", client: wrongNodeClient, bearer: credentialA.Secret, status: http.StatusForbidden, action: "node.snapshot.read", actorFingerprint: q11Fingerprint(wrongNodeCert.Leaf), nodeID: &nodeA, spaceID: &spaceA},
		denied{name: "cross-node", method: http.MethodGet, path: "/v1/nodes/" + nodeB + "/spaces/" + spaceA + "/snapshot", requestID: newID(), actorClass: "node", reasonCode: "node_scope_mismatch", client: nodeClient, bearer: credentialA.Secret, status: http.StatusForbidden, action: "node.snapshot.read", actorFingerprint: q11Fingerprint(nodeCert.Leaf), nodeID: &nodeB, spaceID: &spaceA},
		denied{name: "cross-space", method: http.MethodGet, path: "/v1/nodes/" + nodeA + "/spaces/" + spaceC + "/snapshot", requestID: newID(), actorClass: "node", reasonCode: "node_scope_mismatch", client: nodeClient, bearer: credentialA.Secret, status: http.StatusForbidden, action: "node.snapshot.read", actorFingerprint: q11Fingerprint(nodeCert.Leaf), nodeID: &nodeA, spaceID: &spaceC},
	)
	for _, tc := range denials {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := q11Request(t, ctx, tc.client, server.URL, tc.method, tc.path, tc.body, tc.requestID, tc.bearer)
			require.Equal(t, tc.status, resp.StatusCode)
			if got := resp.Header.Get("X-Request-ID"); got != tc.requestID {
				t.Errorf("request ID response header: got %q want %q", got, tc.requestID)
			}
			q11RequireAudit(t, ctx, pool, tc.requestID, tc.action, "denied", tc.status, tc.actorClass, tc.reasonCode, tc.actorFingerprint, tc.nodeID, tc.spaceID)
		})
	}
	// A well-formed GIS vgi1 credential is not a Federation node Bearer. It is
	// rejected before the snapshot handler can mint an envelope.
	crossAuthorityID := newID()
	crossAuthorityResponse, crossAuthorityBody := getWithBearer(nodeClient, baseA+"/snapshot", crossAuthorityID,
		"vgi1_"+newID()+"_"+strings.Repeat("A", 43))
	require.Equal(t, http.StatusForbidden, crossAuthorityResponse.StatusCode, string(crossAuthorityBody))
	require.NotContains(t, string(crossAuthorityBody), "payload")

	// Duplicate approval is a Q11-enumerated conflict and must not mint another credential.
	dupID := newID()
	resp, _ = post(operatorClient, "/v1/nodes/"+nodeA+"/approve", map[string]bool{"ownership_verified": true}, dupID)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	if got := resp.Header.Get("X-Request-ID"); got != dupID {
		t.Errorf("request ID response header: got %q want %q", got, dupID)
	}
	q11RequireAudit(t, ctx, pool, dupID, "node.approve", "conflict", http.StatusConflict, "operator", "approval_conflict", q11Fingerprint(operatorCert.Leaf), &nodeA, nil)

	// Inject a unique-constraint failure after an API approval UPDATE. The domain
	// transaction must roll back its row change while the Q11 approval-conflict
	// audit is appended in its separate transaction. This is rollback-path
	// evidence for the listed approval route, not a new denial route.
	rollbackNode := newID()
	resp, responseBody = post(operatorClient, "/v1/nodes", map[string]string{"node_id": rollbackNode, "operator_id": operatorID, "endpoint": "https://rollback-node.test", "certificate_sha256": digest([]byte("rollback-node-cert"))}, newID())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(responseBody))
	_, err = pool.Exec(ctx, `CREATE FUNCTION q11_fail_approval_after_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE unique_violation USING MESSAGE = 'q11 rollback injection'; END $$`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS q11_fail_approval_after_write ON federation_nodes`)
		_, _ = pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS q11_fail_approval_after_write()`)
	})
	_, err = pool.Exec(ctx, `CREATE TRIGGER q11_fail_approval_after_write AFTER UPDATE OF status ON federation_nodes FOR EACH ROW WHEN (OLD.status='pending' AND NEW.status='active') EXECUTE FUNCTION q11_fail_approval_after_write()`)
	require.NoError(t, err)
	rollbackID := newID()
	resp, responseBody = post(operatorClient, "/v1/nodes/"+rollbackNode+"/approve", map[string]bool{"ownership_verified": true}, rollbackID)
	require.Equal(t, http.StatusConflict, resp.StatusCode, string(responseBody))
	if got := resp.Header.Get("X-Request-ID"); got != rollbackID {
		t.Errorf("request ID response header: got %q want %q", got, rollbackID)
	}
	var rollbackStatus, rollbackCredential string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status,coalesce(credential_hash,'') FROM federation_nodes WHERE id=$1`, rollbackNode).Scan(&rollbackStatus, &rollbackCredential))
	require.Equal(t, "pending", rollbackStatus)
	require.Empty(t, rollbackCredential)
	q11RequireAudit(t, ctx, pool, rollbackID, "node.approve", "conflict", http.StatusConflict, "operator", "approval_conflict", q11Fingerprint(operatorCert.Leaf), &rollbackNode, nil)

	// Revocation invalidates the previously issued node bearer; audit identity remains the cert fingerprint.
	_, _ = post(operatorClient, "/v1/nodes/"+nodeA+"/suspend", map[string]any{}, newID())
	revokedID := newID()
	resp, _ = getWithBearer(nodeClient, baseA+"/snapshot", revokedID, credentialA.Secret)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	if got := resp.Header.Get("X-Request-ID"); got != revokedID {
		t.Errorf("request ID response header: got %q want %q", got, revokedID)
	}
	q11RequireAudit(t, ctx, pool, revokedID, "node.snapshot.read", "denied", http.StatusForbidden, "node", "credential_revoked", q11Fingerprint(nodeCert.Leaf), &nodeA, &spaceA)

	// Normalize missing/malformed/repeated request IDs, and echo the effective canonical UUID.
	noncanonicalID := "ABCDEFAB-CDEF-4ABC-8DEF-ABCDEFABCDEF"
	for _, value := range []string{"", "not-a-uuid", noncanonicalID, newID() + ", " + newID()} {
		requestID := value
		if strings.Contains(value, ",") {
			requestID = "multiple"
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/healthz", nil)
		require.NoError(t, reqErr)
		if value != "" && value != "multiple" {
			req.Header.Set("X-Request-ID", value)
		}
		if requestID == "multiple" {
			req.Header.Add("X-Request-ID", strings.Split(value, ", ")[0])
			req.Header.Add("X-Request-ID", strings.Split(value, ", ")[1])
		}
		response, reqErr := operatorClient.Do(req)
		require.NoError(t, reqErr)
		response.Body.Close()
		effective, parseErr := uuid.Parse(response.Header.Get("X-Request-ID"))
		require.NoError(t, parseErr)
		require.NotEqual(t, uuid.Nil, effective)
		if requestID == "" || requestID == "multiple" || value == "not-a-uuid" || value == noncanonicalID {
			require.NotEqual(t, value, response.Header.Get("X-Request-ID"))
		}
	}

	// Verify immutable storage by attempting mutations against only API-created audit rows.
	var auditID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM federation_http_audit WHERE request_id=$1`, dupID).Scan(&auditID))
	for _, query := range []string{
		`UPDATE federation_http_audit SET reason_code='tampered' WHERE id=$1`,
		`DELETE FROM federation_http_audit WHERE id=$1`,
	} {
		_, mutationErr := pool.Exec(ctx, query, auditID)
		require.Error(t, mutationErr, "audit mutation must be rejected: %s", query)
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_http_audit WHERE id=$1`, auditID).Scan(&count))
		require.Equal(t, 1, count)
	}
	_, mutationErr := pool.Exec(ctx, `TRUNCATE federation_http_audit`)
	require.Error(t, mutationErr, "audit truncate must be rejected")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_http_audit WHERE id=$1`, auditID).Scan(&count))
	require.Equal(t, 1, count)

	// Audit write failure must fail closed. DDL fault injection only; no registry rows are seeded.
	_, err = pool.Exec(ctx, `CREATE FUNCTION q11_fail_audit_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'q11 audit failure injection'; END $$`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS q11_fail_audit_insert ON federation_http_audit`)
		_, _ = pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS q11_fail_audit_insert()`)
	})
	_, err = pool.Exec(ctx, `CREATE TRIGGER q11_fail_audit_insert BEFORE INSERT ON federation_http_audit FOR EACH ROW EXECUTE FUNCTION q11_fail_audit_insert()`)
	require.NoError(t, err)
	failureID := newID()
	failureResponse, failureBody := q11Request(t, ctx, wrongNodeClient, server.URL, http.MethodGet, baseA+"/snapshot", nil, failureID, credentialA.Secret)
	require.Equal(t, http.StatusServiceUnavailable, failureResponse.StatusCode)
	require.Equal(t, failureID, failureResponse.Header.Get("X-Request-ID"))
	require.JSONEq(t, `{"error":"unavailable"}`, string(failureBody))
	require.NotContains(t, string(failureBody), credentialA.Secret)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_http_audit WHERE request_id=$1`, failureID).Scan(&count))
	require.Zero(t, count, "failed audit insert must not claim a completed denial")
	_ = credentialB
}

func q11Request(t *testing.T, ctx context.Context, client *http.Client, baseURL, method, path string, body any, requestID, bearer string) (*http.Response, []byte) {
	t.Helper()
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("X-Request-ID", requestID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, data
}

func q11RequireAudit(t *testing.T, ctx context.Context, pool *pgxpool.Pool, requestID, action, result string, status int, actorClass, reasonCode, fingerprint string, nodeID, spaceID *string) {
	t.Helper()
	var gotAction, gotResult, gotActor, gotFingerprint, gotNode, gotSpace, gotReason string
	var gotStatus int
	var gotCount int
	var createdAt, serverNow time.Time
	err := pool.QueryRow(ctx, `SELECT action,result,actor_class,actor_fingerprint_sha256,coalesce(target_node_id::text,''),coalesce(target_space_id::text,''),http_status,reason_code,created_at,clock_timestamp() FROM federation_http_audit WHERE request_id=$1`, requestID).Scan(&gotAction, &gotResult, &gotActor, &gotFingerprint, &gotNode, &gotSpace, &gotStatus, &gotReason, &createdAt, &serverNow)
	require.NoError(t, err)
	require.Equal(t, action, gotAction)
	require.Equal(t, result, gotResult)
	require.Equal(t, actorClass, gotActor)
	require.Equal(t, fingerprint, gotFingerprint)
	require.Equal(t, status, gotStatus)
	require.Equal(t, reasonCode, gotReason)
	require.False(t, createdAt.IsZero(), "audit timestamp must be persisted")
	require.False(t, createdAt.After(serverNow), "audit timestamp must come from the server")
	require.Less(t, serverNow.Sub(createdAt), time.Minute, "audit timestamp must be current")
	if nodeID != nil {
		require.Equal(t, *nodeID, gotNode)
	} else {
		require.Empty(t, gotNode, "unexpected target node")
	}
	if spaceID != nil {
		require.Equal(t, *spaceID, gotSpace)
	} else {
		require.Empty(t, gotSpace, "unexpected target Space")
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM federation_http_audit WHERE request_id=$1`, requestID).Scan(&gotCount))
	require.Equal(t, 1, gotCount)
}

func q11Certificates(t *testing.T) (*x509.CertPool, tls.Certificate, tls.Certificate, tls.Certificate, tls.Certificate, tls.Certificate) {
	t.Helper()
	now := time.Now()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	rootTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Q11 test CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	root, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)
	ca := x509.NewCertPool()
	ca.AddCert(root)
	makeCert := func(serial int64, name string, usages []x509.ExtKeyUsage, server bool) tls.Certificate {
		key, keyErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, keyErr)
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usages}
		if server {
			template.DNSNames = []string{"127.0.0.1", "localhost"}
			template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		}
		der, createErr := x509.CreateCertificate(rand.Reader, template, root, &key.PublicKey, rootKey)
		require.NoError(t, createErr)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyDER, marshalErr := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, marshalErr)
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
		pair, pairErr := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, pairErr)
		pair.Leaf, pairErr = x509.ParseCertificate(der)
		require.NoError(t, pairErr)
		return pair
	}
	return ca,
		makeCert(2, "q11-server", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, true),
		makeCert(3, "q11-operator", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, false),
		makeCert(4, "q11-node-a", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, false),
		makeCert(5, "q11-node-b", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, false),
		makeCert(6, "q11-unregistered", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, false)
}

func q11Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}
