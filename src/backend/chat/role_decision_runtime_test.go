package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type ordinaryRoleProbe struct {
	rolev1.RoleServiceClient
	calls atomic.Int64
}

func (c *ordinaryRoleProbe) CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	c.calls.Add(1)
	return &rolev1.CheckPermissionResponse{Allowed: true}, nil
}

type roleDecisionReceiver struct {
	rolev1.UnimplementedRoleServiceServer
	calls atomic.Int64
	deny  atomic.Bool
}

func (s *roleDecisionReceiver) CheckPermission(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	s.calls.Add(1)
	if s.deny.Load() {
		return nil, status.Error(codes.Unavailable, "owner unavailable")
	}
	return &rolev1.CheckPermissionResponse{Allowed: true}, nil
}
func roleViewRequest() *rolev1.CheckPermissionRequest {
	return &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
}

func TestChatRoleDecisionMissingTransportNeverFallsBack(t *testing.T) {
	for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		t.Setenv("CHAT_ROLE_DECISION_"+suffix, "")
	}
	ordinary := &ordinaryRoleProbe{}
	client, closeFn, err := newChatRoleDecisionClientFromEnv(ordinary, nil)
	require.NoError(t, err)
	defer closeFn()
	_, err = client.CheckPermission(t.Context(), roleViewRequest())
	require.Error(t, err)
	require.Zero(t, ordinary.calls.Load())
}

func TestChatRoleDecisionSignedMTLSAndNoFallback(t *testing.T) {
	caFile, serverCert, serverKey, clientCert, clientKey := authFloorTestPKI(t)
	roots := x509.NewCertPool()
	data, err := os.ReadFile(caFile)
	require.NoError(t, err)
	require.True(t, roots.AppendCertsFromPEM(data))
	cert, err := tls.LoadX509KeyPair(serverCert, serverKey)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	var mu sync.Mutex
	ids := map[string]bool{}
	jtis := map[string]bool{}
	receiver := &roleDecisionReceiver{}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})), grpc.UnaryInterceptor(func(ctx context.Context, body any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		md, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		hash, err := principal.RequestHash(body.(proto.Message))
		if err != nil {
			return nil, err
		}
		p, err := principal.VerifyService(ctx, md.BearerToken, principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName, ExpectedRequestID: md.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		if _, err := uuid.Parse(p.RequestID); err != nil {
			return nil, status.Error(codes.InvalidArgument, "request id")
		}
		mu.Lock()
		defer mu.Unlock()
		if ids[p.RequestID] || jtis[p.JWTID] {
			return nil, status.Error(codes.Unauthenticated, "reused credential")
		}
		ids[p.RequestID] = true
		jtis[p.JWTID] = true
		return next(ctx, body)
	}))
	rolev1.RegisterRoleServiceServer(server, receiver)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go server.Serve(listener)
	defer server.Stop()
	for suffix, value := range map[string]string{"GRPC_ADDR": listener.Addr().String(), "TLS_CA_FILE": caFile, "TLS_SERVER_NAME": "auth.fixture", "CLIENT_CERT_FILE": clientCert, "CLIENT_KEY_FILE": clientKey} {
		t.Setenv("CHAT_ROLE_DECISION_"+suffix, value)
	}
	ordinary := &ordinaryRoleProbe{}
	client, closeFn, err := newChatRoleDecisionClientFromEnv(ordinary, issuer)
	require.NoError(t, err)
	defer closeFn()
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(t.Context(), metadata.Pairs("authorization", "Bearer untrusted", "x-voice-profile-id", uuid.NewString(), "x-request-id", "untrusted")), 5*time.Second)
	defer cancel()
	req := roleViewRequest()
	for i := 0; i < 2; i++ {
		res, err := client.CheckPermission(ctx, req)
		require.NoError(t, err)
		require.True(t, res.Allowed)
	}
	require.EqualValues(t, 2, receiver.calls.Load())
	require.Zero(t, ordinary.calls.Load())
	receiver.deny.Store(true)
	_, err = client.CheckPermission(ctx, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, ordinary.calls.Load())
	t.Setenv("CHAT_ROLE_DECISION_TLS_SERVER_NAME", "wrong.fixture")
	wrong, closeWrong, err := newChatRoleDecisionClientFromEnv(ordinary, issuer)
	require.NoError(t, err)
	defer closeWrong()
	wrongCtx, cancelWrong := context.WithTimeout(t.Context(), time.Second)
	defer cancelWrong()
	_, err = wrong.CheckPermission(wrongCtx, req)
	require.Error(t, err)
	require.EqualValues(t, 3, receiver.calls.Load())
	require.Zero(t, ordinary.calls.Load())
	for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		t.Run("missing_"+suffix, func(t *testing.T) {
			t.Setenv("CHAT_ROLE_DECISION_"+suffix, "")
			_, _, err := newChatRoleDecisionClientFromEnv(ordinary, issuer)
			require.Error(t, err)
		})
	}
	_, _, err = newChatRoleDecisionClientFromEnv(ordinary, nil)
	require.Error(t, err)
}
