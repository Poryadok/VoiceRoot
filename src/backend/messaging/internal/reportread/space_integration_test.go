package reportread

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// Invoked by Space's actual native44 TLS fixture, whose initial Space has no
// lifecycle aggregate/history. It shares neither a synthetic response nor a
// fabricated principal with the consumer.
func TestReportOwnerActualInitialNative44Space(t *testing.T) {
	endpoint := os.Getenv("REPORT_READ_NATIVE_ENDPOINT")
	if endpoint == "" {
		t.Skip("invoked by actual Space native44 integration fixture")
	}
	ca, err := os.ReadFile(os.Getenv("REPORT_READ_NATIVE_CA"))
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	cert, err := tls.LoadX509KeyPair(os.Getenv("REPORT_READ_NATIVE_CERT"), os.Getenv("REPORT_READ_NATIVE_KEY"))
	require.NoError(t, err)
	keyBytes, err := os.ReadFile(os.Getenv("REPORT_READ_NATIVE_ISSUER"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyBytes)
	require.NotNil(t, block)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	private, ok := key.(*rsa.PrivateKey)
	require.True(t, ok)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: private})
	require.NoError(t, err)
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	owners := &Owners{Issuer: issuer, Space: spacev1.NewSpaceServiceClient(conn)}
	observation, err := owners.ReadSpace(context.Background(), uuid.MustParse(os.Getenv("REPORT_READ_NATIVE_SPACE")), uuid.MustParse(os.Getenv("REPORT_READ_NATIVE_VIEWER")), &chatv1.ChatRef{Id: os.Getenv("REPORT_READ_NATIVE_CHAT"), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()})
	require.NoError(t, err, "genuine initial lifecycle generation zero is eligible")
	require.NotNil(t, observation)
	require.True(t, observation.Facts.Eligible)
	require.Zero(t, observation.Facts.LifecycleGeneration)
	require.False(t, observation.Facts.HistoryKnown)
	require.Positive(t, observation.Facts.LedgerRevision)
}
