package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

type recordingMediaClient struct {
	filev1.FileServiceClient
	ctx context.Context
	req *filev1.ValidateStoryMediaRequest
}

func (c *recordingMediaClient) ValidateStoryMedia(ctx context.Context, req *filev1.ValidateStoryMediaRequest, _ ...grpc.CallOption) (*filev1.ValidateStoryMediaResponse, error) {
	c.ctx, c.req = ctx, req
	return &filev1.ValidateStoryMediaResponse{}, nil
}

func TestMediaClientSignsExactStoryPrincipalAndDiscardsIncomingAuthority(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "story", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	recording := &recordingMediaClient{}
	client := &signedStoryMediaClient{FileServiceClient: recording, issuer: issuer}
	fileID, authorID := uuid.New(), uuid.New()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer attacker", "x-voice-profile-id", "attacker", "x-request-id", "client-id"))
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-user-id", "attacker", "authorization", "Bearer attacker"))
	var previousID, previousToken string
	for _, expected := range []storyv1.StoryMediaType{storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO} {
		require.NoError(t, client.ValidateStoryMedia(ctx, fileID, authorID, expected))
		require.Equal(t, fileID.String(), recording.req.GetFileId())
		require.Equal(t, authorID.String(), recording.req.GetAuthorProfileId())
		require.Equal(t, expected, recording.req.GetExpectedStoryType())
		md, ok := metadata.FromOutgoingContext(recording.ctx)
		require.True(t, ok)
		require.Len(t, md, 2)
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		id, token := md.Get("x-request-id")[0], md.Get("authorization")[0]
		require.NotEqual(t, "client-id", id)
		require.NotEqual(t, previousID, id)
		require.NotEqual(t, previousToken, token)
		hash, err := principal.RequestHash(recording.req)
		require.NoError(t, err)
		_, err = principal.VerifyService(context.Background(), token[len("Bearer "):], principal.VerifyConfig{ExpectedIssuer: "story", ExpectedAudience: "file", ExpectedRPC: filev1.FileService_ValidateStoryMedia_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		require.NoError(t, err)
		previousID, previousToken = id, token
	}
}

func TestStoryMediaPrincipalPartialConfigurationFails(t *testing.T) {
	for _, name := range []string{"FILE_PRINCIPAL_GRPC_ADDR", "FILE_PRINCIPAL_TLS_CA_FILE", "FILE_PRINCIPAL_TLS_SERVER_NAME", "STORY_PRINCIPAL_SIGNING_KEYS_DIR", "STORY_PRINCIPAL_ACTIVE_KID", "S2S_SIGNING_KEY_PEM", "S2S_SIGNING_KID"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "")
			_, _, _, err := loadSignedStoryMediaClientFromEnv()
			require.Error(t, err)
		})
	}
}

func TestStoryMediaPrincipalLoadsTwoKeysAndPublishesOnlySortedPublicKeys(t *testing.T) {
	keys := make(map[string]*rsa.PrivateKey)
	dir := t.TempDir()
	for _, kid := range []string{"z-current", "a-next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		keys[kid] = key
		writeStoryTestKey(t, filepath.Join(dir, kid+".pem"), key)
	}
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "file"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &keys["z-current"].PublicKey, keys["z-current"])
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	t.Setenv("STORY_PRINCIPAL_SIGNING_KEYS_DIR", dir)
	t.Setenv("STORY_PRINCIPAL_ACTIVE_KID", "z-current")
	t.Setenv("FILE_PRINCIPAL_GRPC_ADDR", "localhost:9091")
	t.Setenv("FILE_PRINCIPAL_TLS_CA_FILE", caPath)
	t.Setenv("FILE_PRINCIPAL_TLS_SERVER_NAME", "file")
	client, conn, public, err := loadSignedStoryMediaClientFromEnv()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NotNil(t, client)
	parsed, err := principal.ParseJWKS(public)
	require.NoError(t, err)
	require.Len(t, parsed, 2)
	require.Equal(t, keys["z-current"].N, parsed["z-current"].N)
	var document struct {
		Keys []map[string]any `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(public, &document))
	require.Equal(t, "a-next", document.Keys[0]["kid"])
	require.Equal(t, "z-current", document.Keys[1]["kid"])
	for _, key := range document.Keys {
		require.Len(t, key, 6)
		require.NotContains(t, key, "d")
		require.NotContains(t, key, "p")
		require.NotContains(t, key, "q")
	}
	for _, name := range []string{"FILE_PRINCIPAL_TLS_CA_FILE", "FILE_PRINCIPAL_TLS_SERVER_NAME", "STORY_PRINCIPAL_ACTIVE_KID"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "")
			_, _, _, err := loadSignedStoryMediaClientFromEnv()
			require.Error(t, err)
		})
	}
	t.Run("unknown active kid", func(t *testing.T) {
		t.Setenv("STORY_PRINCIPAL_ACTIVE_KID", "unknown")
		_, _, _, err := loadSignedStoryMediaClientFromEnv()
		require.Error(t, err)
	})
	for _, tc := range []struct {
		name     string
		contents []byte
	}{
		{"malformed", []byte("not a key")},
		{"wrong PEM type", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(keys["z-current"])})},
		{"duplicate", storyTestPEM(t, keys["z-current"])},
	} {
		t.Run(tc.name, func(t *testing.T) {
			badDir := t.TempDir()
			writeStoryTestKey(t, filepath.Join(badDir, "current.pem"), keys["z-current"])
			require.NoError(t, os.WriteFile(filepath.Join(badDir, "next.pem"), tc.contents, 0600))
			_, err := loadStorySigningKeys(badDir)
			require.Error(t, err)
		})
	}
	t.Run("escaping symlink", func(t *testing.T) {
		badDir := t.TempDir()
		writeStoryTestKey(t, filepath.Join(badDir, "current.pem"), keys["z-current"])
		if err := os.Symlink(filepath.Join(dir, "a-next.pem"), filepath.Join(badDir, "next.pem")); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		_, err := loadStorySigningKeys(badDir)
		require.Error(t, err)
	})
}

func storyTestPEM(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
func writeStoryTestKey(t *testing.T, path string, key *rsa.PrivateKey) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, storyTestPEM(t, key), 0600))
}
