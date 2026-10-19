package gisprincipal

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func configureBotChatFixtureJWKS(t *testing.T, cfg *Config) {
	t.Helper()
	values := []string{os.Getenv("VOICE_BOT_CHAT_JWKS_URL"), os.Getenv("VOICE_BOT_CHAT_JWKS_CA_FILE"), os.Getenv("VOICE_BOT_CHAT_JWKS_CLIENT_CERT_FILE"), os.Getenv("VOICE_BOT_CHAT_JWKS_CLIENT_KEY_FILE")}
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	if count == 0 {
		return
	}
	require.Equal(t, 4, count, "external protected JWKS configuration is all-or-none")
	require.NotEmpty(t, os.Getenv("VOICE_BOT_CHAT_SIGNER_KEY_FILE"))
	cfg.JWKSURL, cfg.JWKSCAFile, cfg.JWKSClientCertFile, cfg.JWKSClientKeyFile = values[0], values[1], values[2], values[3]
}

type botChatSourceHandoff struct {
	Address                   string
	Space, Chat               uuid.UUID
	CA, ClientCert, ClientKey string
	Signer                    *rsa.PrivateKey
	KeyID, Incarnation        string
	Revision                  uint64
}

// Optional source-only cross-process fixture. All addresses/keys belong to the
// caller's disposable Linux process namespace, and disappear during teardown.
func runBotChatSourceHandoff(t *testing.T, ctx context.Context, pool *pgxpool.Pool, f botChatSourceHandoff) bool {
	t.Helper()
	directory := os.Getenv("VOICE_BOT_CHAT_HANDOFF_DIR")
	if directory == "" {
		return false
	}
	require.NoError(t, os.MkdirAll(directory, 0700))
	ready := filepath.Join(directory, "ready.json")
	complete := filepath.Join(directory, "complete.json")
	retire := filepath.Join(directory, "retire-chat")
	retired := filepath.Join(directory, "retired-chat")
	for _, name := range []string{ready, complete, retire, retired} {
		_, err := os.Stat(name)
		require.True(t, os.IsNotExist(err), "fresh handoff directory required")
	}
	signer := filepath.Join(directory, "fixture-bot-key.pem")
	key, err := x509.MarshalPKCS8PrivateKey(f.Signer)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(signer, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600))
	t.Cleanup(func() {
		for _, path := range []string{ready, signer, retired} {
			_ = os.Remove(path)
		}
	})
	manifest := map[string]any{"fixture_only": true, "address": f.Address, "server_name": "localhost", "space_id": f.Space, "chat_id": f.Chat, "ca_file": f.CA, "client_cert_file": f.ClientCert, "client_key_file": f.ClientKey, "signer_key_file": signer, "signer_key_id": f.KeyID}
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(ready, body, 0600))
	deadline := time.NewTimer(150 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	didRetire := false
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
			return true
		case <-deadline.C:
			t.Fatal("Bot Chat consumer did not complete within bounded fixture lifetime")
			return true
		case <-ticker.C:
			if _, err := os.Stat(retire); err == nil && !didRetire {
				_, err = pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, f.Chat)
				require.NoError(t, err)
				var incarnation string
				var revision uint64
				require.NoError(t, pool.QueryRow(ctx, `SELECT scope_incarnation::text,revision FROM chat_scope_authority_heads WHERE chat_id=$1`, f.Chat).Scan(&incarnation, &revision))
				require.Equal(t, f.Incarnation, incarnation)
				require.Greater(t, revision, f.Revision)
				require.NoError(t, os.WriteFile(retired, []byte("retained-head"), 0600))
				didRetire = true
			}
			if _, err := os.Stat(complete); err == nil {
				require.True(t, didRetire, "actual retired Chat control required")
				return true
			}
		}
	}
}
