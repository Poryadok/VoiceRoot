package principalruntime

import (
	"context"
	"crypto/tls"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	filev1 "voice.app/voice/file/v1"
)

func TestProtectedFileRequiresClientCertificateBeforeJWTOrHandler(t *testing.T) {
	fixture := newTransportFixture(t, false)
	foreign := newTransportFixture(t, false)
	token := signedCredential(t, fixture.active, "current", nil)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request-1"))
	for _, certificates := range [][]tls.Certificate{nil, foreign.clientTLS.Certificates} {
		config := fixture.clientTLS.Clone()
		config.Certificates = certificates
		connection, err := grpc.NewClient(fixture.address, grpc.WithTransportCredentials(credentials.NewTLS(config)))
		require.NoError(t, err)
		bounded, cancel := context.WithTimeout(ctx, time.Second)
		_, err = filev1.NewFileServiceClient(connection).ValidateStoryMedia(bounded, storyMediaRequest())
		cancel()
		require.NoError(t, connection.Close())
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Zero(t, fixture.service.calls.Load())
	}
	require.NoError(t, callMedia(t, fixture, token, storyMediaRequest()), "failed TLS handshake must not consume the valid JWT replay entry")
}
