package sessionfloor

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/principal"
)

// This explicitly selected proof calls the actual Java read-only Auth handler.
// All files/keys belong to its bounded test fixture; none is deployment config.
func TestActualJavaBotSessionFloor(t *testing.T) {
	dir := os.Getenv("BOT_AUTH_FLOOR_INTEROP_DIR")
	if dir == "" {
		t.Skip("requires owned real Java mTLS floor fixture")
	}
	readyBytes, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	require.NoError(t, err)
	var ready struct {
		Port      int    `json:"port"`
		AccountID string `json:"account_id"`
		Clock     string `json:"clock"`
		Kid       string `json:"kid"`
		RPC       string `json:"rpc"`
	}
	require.NoError(t, json.Unmarshal(readyBytes, &ready))
	require.Equal(t, RPC, ready.RPC)
	clock, err := time.Parse(time.RFC3339, ready.Clock)
	require.NoError(t, err)
	key, err := principal.LoadRSAPrivateKeyFile(filepath.Join(dir, "bot-signing-key.pem"))
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: ready.Kid, PrivateKey: key, Clock: func() time.Time { return clock }})
	require.NoError(t, err)
	expected, err := strconv.ParseInt(os.Getenv("BOT_AUTH_FLOOR_INTEROP_EXPECT"), 10, 64)
	require.NoError(t, err)
	host := os.Getenv("BOT_AUTH_FLOOR_INTEROP_HOST")
	require.NotEmpty(t, host)
	client, conn, err := (Config{Address: net.JoinHostPort(host, strconv.Itoa(ready.Port)), CAFile: filepath.Join(dir, "server-cert.pem"), ServerName: "localhost", ClientCertFile: filepath.Join(dir, "bot-cert.pem"), ClientKeyFile: filepath.Join(dir, "bot-key.pem")}).Dial(issuer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	floor, err := client.CurrentFloor(t.Context(), ready.AccountID)
	if expected == 0 {
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Zero(t, floor)
		require.Equal(t, codes.Unavailable, status.Code(client.RequireCurrent(t.Context(), ready.AccountID, 100)))
		return
	}
	require.NoError(t, err)
	require.Equal(t, expected, floor)
	require.Equal(t, codes.Unauthenticated, status.Code(client.RequireCurrent(t.Context(), ready.AccountID, expected-1)))
	require.NoError(t, client.RequireCurrent(t.Context(), ready.AccountID, expected))
	require.Equal(t, codes.Unauthenticated, status.Code(client.RequireCurrent(t.Context(), "00000000-0000-0000-0000-000000000272", expected)), "missing account must not admit an initiating owner")
}
