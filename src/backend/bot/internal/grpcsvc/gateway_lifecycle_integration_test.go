package grpcsvc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/principalgrpc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func TestGatewayLifecycleMTLSVerifiedOwnerWithoutRawHeadersBeforeDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL and mTLS initiating boundary")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_gateway_lifecycle", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	owner, foreign, profile := uuid.New(), uuid.New(), uuid.New()
	actualOwner, actualFloor := actualJavaInitiatingFloor(t)
	if actualFloor != nil {
		owner = actualOwner
	}
	bot, _, err := st.CreateBot(ctx, owner, "Protected", "", `[]`, uuid.New())
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "gateway-current", PrivateKey: key})
	require.NoError(t, err)
	var mu sync.Mutex
	seen := map[string]bool{}
	floor := int64(9)
	verifier := principalgrpc.GatewayUserVerifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(_ context.Context, _ string, jti string, _ time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		if seen[jti] {
			return errors.New("replay")
		}
		seen[jti] = true
		return nil
	}, Epoch: func(ctx context.Context, account string, epoch int64) error {
		if actualFloor != nil {
			return actualFloor.RequireCurrent(ctx, account, epoch)
		}
		if epoch < floor {
			return status.Error(codes.Unauthenticated, "stale")
		}
		return nil
	}}
	tlsKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &tlsKey.PublicKey, tlsKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(tlsKey)})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})), grpc.UnaryInterceptor(principalgrpc.GatewayLifecycleUnaryInterceptor(verifier)), grpc.StreamInterceptor(principalgrpc.GatewayLifecycleStreamInterceptor()))
	botv1.RegisterBotServiceServer(server, grpcsvc.NewBotGRPC(st, nil))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///bot-protected", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{certificate}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := botv1.NewBotServiceClient(conn)
	streamCtx, stopStream := context.WithTimeout(ctx, 2*time.Second)
	defer stopStream()
	stream, streamErr := client.PollEvents(streamCtx, &botv1.PollEventsRequest{})
	if streamErr == nil {
		_, streamErr = stream.Recv()
	}
	require.Equal(t, codes.Unimplemented, status.Code(streamErr), "private Gateway listener exposes only three unary lifecycle methods")
	request := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	signed := func(account uuid.UUID, epoch int64) context.Context {
		requestID := uuid.NewString()
		token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: botv1.BotService_DeleteBot_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: epoch, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	}
	_, err = client.DeleteBot(metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-user-id", owner.String(), "x-voice-profile-id", profile.String())), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	mixed := signed(owner, 9)
	md, _ := metadata.FromOutgoingContext(mixed)
	md = md.Copy()
	md.Set("x-voice-user-id", owner.String())
	_, err = client.DeleteBot(metadata.NewOutgoingContext(ctx, md), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	if actualFloor != nil && os.Getenv("BOT_AUTH_FLOOR_INTEROP_EXPECT") == "0" {
		_, err = client.DeleteBot(signed(owner, 9), request)
		require.Equal(t, codes.Unavailable, status.Code(err))
		live, err := st.GetBotByID(ctx, bot.ID)
		require.NoError(t, err)
		require.Equal(t, "live", live.Status)
		var decisions int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&decisions))
		require.Zero(t, decisions, "Auth outage must precede every owner decision and live revoke")
		return
	}
	_, err = client.DeleteBot(signed(foreign, 9), request)
	if actualFloor != nil {
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	} else {
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	_, err = client.DeleteBot(signed(owner, 8), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	unchanged, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "live", unchanged.Status)
	accepted := signed(owner, 9)
	_, err = client.DeleteBot(accepted, request)
	require.NoError(t, err)
	_, err = client.DeleteBot(accepted, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	disabled, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "disabled", disabled.Status)
}
