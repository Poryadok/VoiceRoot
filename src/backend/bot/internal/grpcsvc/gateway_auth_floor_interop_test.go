package grpcsvc_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/sessionfloor"
	"voice/backend/pkg/principal"
)

func actualJavaInitiatingFloor(t *testing.T) (uuid.UUID, *sessionfloor.Client) {
	t.Helper()
	dir := os.Getenv("BOT_AUTH_FLOOR_INTEROP_DIR")
	if dir == "" {
		return uuid.Nil, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	require.NoError(t, err)
	var ready struct {
		Port      int    `json:"port"`
		AccountID string `json:"account_id"`
		Clock     string `json:"clock"`
		Kid       string `json:"kid"`
	}
	require.NoError(t, json.Unmarshal(data, &ready))
	clock, err := time.Parse(time.RFC3339, ready.Clock)
	require.NoError(t, err)
	key, err := principal.LoadRSAPrivateKeyFile(filepath.Join(dir, "bot-signing-key.pem"))
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: ready.Kid, PrivateKey: key, Clock: func() time.Time { return clock }})
	require.NoError(t, err)
	host := os.Getenv("BOT_AUTH_FLOOR_INTEROP_HOST")
	require.NotEmpty(t, host)
	client, conn, err := (sessionfloor.Config{Address: net.JoinHostPort(host, strconv.Itoa(ready.Port)), CAFile: filepath.Join(dir, "server-cert.pem"), ServerName: "localhost", ClientCertFile: filepath.Join(dir, "bot-cert.pem"), ClientKeyFile: filepath.Join(dir, "bot-key.pem")}).Dial(issuer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return uuid.MustParse(ready.AccountID), client
}
