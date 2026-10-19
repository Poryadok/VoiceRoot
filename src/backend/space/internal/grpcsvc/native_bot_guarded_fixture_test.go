package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/store"
)

func nativeBotTransportStore(t *testing.T, ctx context.Context, guarded bool) (*store.SpaceStore, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	dsn, id, profile := os.Getenv("VOICE_BOT_NATIVE_SPACE_DSN"), os.Getenv("VOICE_BOT_NATIVE_SPACE_ID"), os.Getenv("VOICE_BOT_NATIVE_PROFILE_ID")
	if dsn != "" || id != "" || profile != "" {
		require.True(t, guarded, "external fixture is explicitly native44")
		require.NotEmpty(t, dsn)
		require.NotEmpty(t, id)
		require.NotEmpty(t, profile)
		space, err := uuid.Parse(id)
		require.NoError(t, err)
		_, err = uuid.Parse(profile)
		require.NoError(t, err)
		pool, err := pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		t.Cleanup(pool.Close)
		var version int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version))
		require.EqualValues(t, 44, version)
		st := &store.SpaceStore{Pool: pool}
		// The actual protected getter proves serving LOGIN + COMPLETE, typed
		// source/catalog and fences. The fixture never initializes external DBs.
		_, err = st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: id, ProfileId: profile})
		require.NoError(t, err)
		return st, pool, space
	}
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "goto", "34")
	st := &store.SpaceStore{Pool: database.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Protected native Bot", "", "public")
	require.NoError(t, err)
	if guarded {
		database.Run(true, "goto", "44")
		require.NoError(t, st.InitializeNativeMembership(ctx, space.ID))
		st = &store.SpaceStore{Pool: nativeBotServingLogin(t, ctx, database.Pool)}
	} else {
		database.Run(true, "goto", "42")
	}
	return st, database.Pool, space.ID
}

func nativeBotExternalJWKS(t *testing.T, cfg *botmembershipprincipal.Config) {
	t.Helper()
	names := []string{"VOICE_BOT_NATIVE_JWKS_URL", "VOICE_BOT_NATIVE_JWKS_CA_FILE", "VOICE_BOT_NATIVE_JWKS_CLIENT_CERT_FILE", "VOICE_BOT_NATIVE_JWKS_CLIENT_KEY_FILE"}
	values := make([]string, len(names))
	count := 0
	for i, name := range names {
		values[i] = os.Getenv(name)
		if values[i] != "" {
			count++
		}
	}
	if count == 0 {
		return
	}
	require.Equal(t, len(names), count, "external protected JWKS tuple must be complete")
	require.NotEmpty(t, os.Getenv("VOICE_BOT_NATIVE_SIGNER_KEY_FILE"))
	cfg.JWKSURLs = map[string]string{"bot": values[0]}
	cfg.JWKSCAFile, cfg.JWKSClientCertFile, cfg.JWKSClientKeyFile = values[1], values[2], values[3]
}

// A fixture may supply the same private Bot identity used by its Role/Chat
// listeners. Transport certificates remain specific to this receiver.
func nativeBotFixtureSigner(t *testing.T, variable string) *rsa.PrivateKey {
	t.Helper()
	path := os.Getenv(variable)
	if path == "" {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		return key
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	block, rest := pem.Decode(raw)
	require.NotNil(t, block)
	require.Empty(t, rest)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	key, ok := parsed.(*rsa.PrivateKey)
	require.True(t, ok)
	require.GreaterOrEqual(t, key.N.BitLen(), 2048)
	require.NoError(t, key.Validate())
	return key
}

// Same actual independent LOGIN as the native Store proof; never SET ROLE on
// an owner session and never write a readiness registry as the serving user.
func nativeBotServingLogin(t *testing.T, ctx context.Context, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	role := "bot_receiver_" + uuid.New().String()[:8]
	quoted := pgx.Identifier{role}.Sanitize()
	_, err := admin.Exec(ctx, "CREATE ROLE "+quoted+" LOGIN PASSWORD 'isolated-fixture-only'")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+quoted+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+quoted)
	require.NoError(t, err)
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = "isolated-fixture-only"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP OWNED BY "+quoted)
		_, _ = admin.Exec(context.Background(), "DROP ROLE "+quoted)
	})
	var actual string
	require.NoError(t, pool.QueryRow(ctx, "SELECT session_user").Scan(&actual))
	require.Equal(t, role, actual)
	return pool
}
