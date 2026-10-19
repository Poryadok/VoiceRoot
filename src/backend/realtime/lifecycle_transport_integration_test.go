//go:build realtimep3integration

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	realtimev1 "voice.app/voice/realtime/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/realtimeprincipal"
)

type manifestTestCA struct {
	certificate *x509.Certificate
	key         *rsa.PrivateKey
	pem         []byte
}

func newManifestTestCA(t *testing.T, serial int64) manifestTestCA {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "manifest transport CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return manifestTestCA{cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}
func (ca manifestTestCA) leaf(t *testing.T, serial int64, usage x509.ExtKeyUsage) (tls.Certificate, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "manifest leaf"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	require.NoError(t, err)
	certPath, keyPath := filepath.Join(t.TempDir(), "leaf.crt"), filepath.Join(t.TempDir(), "leaf.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	return certificate, certPath, keyPath
}

func TestProtectedRealtimeActualPGReceiptsRequireMTLSExactPrincipalAndReplay(t *testing.T) {
	store, _ := fanoutPostgres(t)
	serverCA, clientCA, foreignCA := newManifestTestCA(t, 1), newManifestTestCA(t, 2), newManifestTestCA(t, 3)
	serverCertificate, certFile, keyFile := serverCA.leaf(t, 10, x509.ExtKeyUsageServerAuth)
	clientCertificate, _, _ := clientCA.leaf(t, 11, x509.ExtKeyUsageClientAuth)
	foreignCertificate, _, _ := foreignCA.leaf(t, 12, x509.ExtKeyUsageClientAuth)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(serverCA.pem))
	serverCAFile, clientCAFile := filepath.Join(t.TempDir(), "server-ca.crt"), filepath.Join(t.TempDir(), "client-ca.crt")
	require.NoError(t, os.WriteFile(serverCAFile, serverCA.pem, 0600))
	require.NoError(t, os.WriteFile(clientCAFile, clientCA.pem, 0600))
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := make([]map[string]string, 0, 2)
	for kid, key := range map[string]*rsa.PrivateKey{"current": current, "next": next} {
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	var outage atomic.Bool
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if outage.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": keys}))
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCertificate}}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	runtime, err := realtimeprincipal.New(context.Background(), realtimeprincipal.Config{Issuer: "space", Audience: "realtime", TLSCertFile: certFile, TLSKeyFile: keyFile, ClientCAFile: clientCAFile, JWKSURL: jwks.URL, JWKSCAFile: serverCAFile, ReplayAddr: replay.Addr(), RefreshAfter: 50 * time.Millisecond, HardExpiry: time.Second})
	require.NoError(t, err)
	defer runtime.Close()
	require.NoError(t, runtime.Ready(context.Background()))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(runtime.ServerOptions()...)
	realtimev1.RegisterRealtimeLifecycleServiceServer(server, &realtimeLifecycleGRPC{store: store})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	connect := func(certs []tls.Certificate) *grpc.ClientConn {
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	connection := connect([]tls.Certificate{clientCertificate})
	request := hubManifest(uuid.New(), uuid.New(), uuid.New())
	method := realtimev1.RealtimeLifecycleService_ImportSpacePurgeManifestPage_FullMethodName
	sign := func(issuerName, audience, rpc, id string, message proto.Message) string {
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuerName, KeyID: "current", PrivateKey: current})
		require.NoError(t, err)
		hash, err := principal.RequestHash(message)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: id, RequestHash: hash})
		require.NoError(t, err)
		return token
	}
	call := func(conn *grpc.ClientConn, rpc, token, id string, req, resp proto.Message) error {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		return conn.Invoke(ctx, rpc, req, resp)
	}
	id := request.DeletionOperationId
	token := sign("space", "realtime", method, id, request)
	first := new(realtimev1.ImportSpacePurgeManifestPageResponse)
	require.NoError(t, call(connection, method, token, id, request, first))
	require.NotNil(t, first.Receipt)
	require.Equal(t, codes.Unauthenticated, status.Code(call(connection, method, token, id, request, new(realtimev1.ImportSpacePurgeManifestPageResponse))))
	again := new(realtimev1.ImportSpacePurgeManifestPageResponse)
	require.NoError(t, call(connection, method, sign("space", "realtime", method, id, request), id, request, again))
	require.True(t, proto.Equal(first, again))
	for _, issuer := range []string{"realtime", "chat", "gateway", "role"} {
		require.Equal(t, codes.Unauthenticated, status.Code(call(connection, method, sign(issuer, "realtime", method, id, request), id, request, new(realtimev1.ImportSpacePurgeManifestPageResponse))))
	}
	require.Equal(t, codes.Unauthenticated, status.Code(call(connection, method, sign("space", "chat", method, id, request), id, request, new(realtimev1.ImportSpacePurgeManifestPageResponse))))
	token = sign("space", "realtime", method, id, request)
	changed := proto.Clone(request).(*realtimev1.ImportSpacePurgeManifestPageRequest)
	changed.SealsManifest = false
	require.Equal(t, codes.Unauthenticated, status.Code(call(connection, method, token, id, changed, new(realtimev1.ImportSpacePurgeManifestPageResponse))))
	for _, certs := range [][]tls.Certificate{nil, {foreignCertificate}} {
		require.Error(t, call(connect(certs), method, sign("space", "realtime", method, id, request), id, request, new(realtimev1.ImportSpacePurgeManifestPageResponse)))
	}
	require.Equal(t, codes.Unimplemented, status.Code(call(connection, "/voice.realtime.v1.RealtimeLifecycleService/RecoverIncarnation", sign("space", "realtime", method, id, request), id, request, new(realtimev1.ImportSpacePurgeManifestPageResponse))))
	fence := &realtimev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: request.SpaceId, DeletionOperationId: id, Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: request.Page.Manifest}}
	fenceMethod := realtimev1.RealtimeLifecycleService_ApplySpaceLifecycleFence_FullMethodName
	require.NoError(t, call(connection, fenceMethod, sign("space", "realtime", fenceMethod, id, fence), id, fence, new(realtimev1.ApplySpaceLifecycleFenceResponse)))
	fence.Fence.Generation = 2
	fence.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	require.NoError(t, call(connection, fenceMethod, sign("space", "realtime", fenceMethod, id, fence), id, fence, new(realtimev1.ApplySpaceLifecycleFenceResponse)))
	purge := &realtimev1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: request.SpaceId, DeletionOperationId: id, Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_REALTIME, Manifest: request.Page.Manifest, PurgeDecidedAt: timestamppb.Now()}}
	purgeMethod := realtimev1.RealtimeLifecycleService_PurgeSpace_FullMethodName
	require.NoError(t, call(connection, purgeMethod, sign("space", "realtime", purgeMethod, id, purge), id, purge, new(realtimev1.PurgeSpaceResponse)))
	outage.Store(true)
	require.NoError(t, runtime.Ready(context.Background()), "last-good set remains usable before its original hard expiry")
	require.Eventually(t, func() bool { return runtime.Ready(context.Background()) != nil }, 2*time.Second, 10*time.Millisecond, "publisher failure must expire lifecycle readiness")
	outage.Store(false)
	require.Eventually(t, func() bool { return runtime.Ready(context.Background()) == nil }, 2*time.Second, 10*time.Millisecond, "only valid HTTPS refresh restores cache readiness")
	replay.Close()
	require.Error(t, runtime.Ready(context.Background()))
	require.Equal(t, codes.Unavailable, status.Code(call(connection, method, sign("space", "realtime", method, id, request), id, request, new(realtimev1.ImportSpacePurgeManifestPageResponse))))
}
