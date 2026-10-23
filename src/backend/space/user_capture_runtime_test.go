package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
	"voice/backend/space/internal/store"
	"voice/backend/space/internal/usercapture"
)

var captureEnvNames = []string{"SPACE_USER_CAPTURE_GRPC_ADDR", "SPACE_USER_CAPTURE_TLS_CA_FILE", "SPACE_USER_CAPTURE_TLS_SERVER_NAME", "SPACE_USER_CAPTURE_CLIENT_CERT_FILE", "SPACE_USER_CAPTURE_CLIENT_KEY_FILE"}

func TestUserCaptureRuntimeConfigFailsClosed(t *testing.T) {
	valid := map[string]string{"DATABASE_URL": "postgres://fixture"}
	for _, name := range captureEnvNames {
		valid[name] = "explicit"
	}
	_, enabled, err := loadUserCaptureRuntimeConfig(func(string) string { return "" }, false)
	require.NoError(t, err)
	require.False(t, enabled)
	for _, missing := range append(append([]string{}, captureEnvNames...), "DATABASE_URL", "protected listener") {
		t.Run(missing, func(t *testing.T) {
			values := map[string]string{}
			for name, value := range valid {
				values[name] = value
			}
			delete(values, missing)
			_, _, err := loadUserCaptureRuntimeConfig(func(name string) string { return values[name] }, missing != "protected listener")
			require.Error(t, err, "partial transport must never become disabled or ordinary fallback")
		})
	}
	for _, name := range captureEnvNames {
		t.Run("whitespace "+name, func(t *testing.T) {
			_, _, err := loadUserCaptureRuntimeConfig(func(key string) string {
				if key == name {
					return " explicit "
				}
				return valid[key]
			}, true)
			require.Error(t, err)
		})
	}
	_, enabled, err = loadUserCaptureRuntimeConfig(func(name string) string { return valid[name] }, true)
	require.NoError(t, err)
	require.True(t, enabled)
}

// This peer proves only workload TLS and the actual generated client/signature
// dispatch. It returns no User facts and cannot qualify a successful Ban or B15.
type captureUnavailablePeer struct {
	userv1.UnimplementedUserServiceServer
}

func (*captureUnavailablePeer) CaptureSpaceMemberAccountBindings(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	return nil, status.Error(codes.Unavailable, "owner facts intentionally absent")
}

type captureRuntimePKI struct {
	config userCaptureRuntimeConfig
	roots  *x509.CertPool
	server tls.Certificate
}

func captureRuntimeCredentials(t *testing.T) captureRuntimePKI {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "capture runtime test CA"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	require.NoError(t, err)
	ca, err = x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	issue := func(serial int64, usage x509.ExtKeyUsage) (tls.Certificate, string, string) {
		leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{"capture.user.test"}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
		cert, err := x509.CreateCertificate(rand.Reader, leaf, ca, &leafKey.PublicKey, key)
		require.NoError(t, err)
		certFile, keyFile := filepath.Join(dir, fmt.Sprintf("%d.pem", serial)), filepath.Join(dir, fmt.Sprintf("%d.key", serial))
		require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}), 0600))
		require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)}), 0600))
		return tls.Certificate{Certificate: [][]byte{cert}, PrivateKey: leafKey}, certFile, keyFile
	}
	server, _, _ := issue(2, x509.ExtKeyUsageServerAuth)
	_, certFile, keyFile := issue(3, x509.ExtKeyUsageClientAuth)
	return captureRuntimePKI{config: userCaptureRuntimeConfig{address: "127.0.0.1:1", caFile: caFile, serverName: "capture.user.test", certFile: certFile, keyFile: keyFile}, roots: roots, server: server}
}

func TestUserCaptureRuntimeRejectsMissingIssuerAndBadMaterial(t *testing.T) {
	pki := captureRuntimeCredentials(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	for _, name := range []string{"issuer", "CA", "client certificate", "client key"} {
		t.Run(name, func(t *testing.T) {
			config, signer := pki.config, issuer
			switch name {
			case "issuer":
				signer = nil
			case "CA":
				config.caFile = filepath.Join(t.TempDir(), "missing")
			case "client certificate":
				config.certFile = pki.config.caFile
			case "client key":
				config.keyFile = pki.config.caFile
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			runtime, err := newUserCaptureRuntime(ctx, config, signer)
			require.Error(t, err)
			require.Nil(t, runtime)
		})
	}
}

func TestUserCaptureRuntimeReadySigningAndClose(t *testing.T) {
	pki := captureRuntimeCredentials(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	observed := make(chan principal.Principal, 2)
	guard := func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		auth, ids := md.Get("authorization"), md.Get("x-request-id")
		if len(auth) != 1 || len(ids) != 1 {
			return nil, status.Error(codes.Unauthenticated, "exact metadata required")
		}
		hash, err := principal.RequestHash(request.(*userv1.CaptureSpaceMemberAccountBindingsRequest))
		if err != nil {
			return nil, err
		}
		actor, err := principal.VerifyService(ctx, strings.TrimPrefix(auth[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "user", ExpectedRPC: info.FullMethod, ExpectedRequestID: ids[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "signed request required")
		}
		observed <- actor
		return handler(ctx, request)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.server}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})), grpc.UnaryInterceptor(guard))
	userv1.RegisterUserServiceServer(server, &captureUnavailablePeer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	pki.config.address = listener.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	runtime, err := newUserCaptureRuntime(ctx, pki.config, issuer)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	t.Cleanup(func() { _ = runtime.Close() })
	require.Equal(t, connectivity.Ready, runtime.conn.GetState())
	st := &store.SpaceStore{AccountCaptures: runtime.client}
	id := uuid.NewString()
	request := &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: id, SpaceId: id, Purpose: userv1.SpaceAccountCapturePurpose_SPACE_ACCOUNT_CAPTURE_PURPOSE_BAN_PREFLIGHT, Cohort: &userv1.SpaceAccountCaptureCohort{Entries: []*userv1.SpaceAccountCaptureEntry{{ProfileId: id, Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{PublicationId: id}}}}}}
	request.CohortDigest = spaceaccountcapture.CohortDigest(request.Cohort)
	var first principal.Principal
	for i := 0; i < 2; i++ {
		response, err := st.AccountCaptures.CaptureSpaceMemberAccountBindings(ctx, request)
		require.ErrorIs(t, err, usercapture.ErrUnavailable)
		require.Nil(t, response)
		select {
		case actor := <-observed:
			require.Equal(t, "service:space", actor.Subject)
			require.Empty(t, actor.AccountID)
			require.Empty(t, actor.ProfileID)
			require.Equal(t, userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName, actor.RPC)
			require.NotEqual(t, id, actor.RequestID)
			if i == 1 {
				require.NotEqual(t, first.RequestID, actor.RequestID)
				require.NotEqual(t, first.JWTID, actor.JWTID)
			}
			first = actor
		case <-ctx.Done():
			t.Fatal("actual protected dispatch was not observed")
		}
	}
	require.NoError(t, runtime.Close())
	require.Equal(t, connectivity.Shutdown, runtime.conn.GetState())
	for _, name := range []string{"server identity", "trust", "unavailable peer"} {
		t.Run(name, func(t *testing.T) {
			config := pki.config
			switch name {
			case "server identity":
				config.serverName = "wrong.user.test"
			case "trust":
				config.caFile = captureRuntimeCredentials(t).config.caFile
			case "unavailable peer":
				config.address = "127.0.0.1:1"
			}
			bounded, stop := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer stop()
			runtime, err := newUserCaptureRuntime(bounded, config, issuer)
			require.Error(t, err)
			require.Nil(t, runtime)
		})
	}
}
