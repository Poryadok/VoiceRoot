package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

// This characterizes the repaired client through File's actual mTLS/JWKS/replay
// runtime, strict interceptor and PostgreSQL-backed ValidateStoryMedia handler.
// No verified principal is injected by the fixture.
func TestStoryMediaClientRealProtectedFile(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL and File subprocess")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	pool := integrationtest.StartPostgres(t, ctx, "story_file_mtls", "")
	for _, name := range []string{"000001_init.up.sql", "000002_premium_upload_limit.up.sql", "000003_story_context.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "migrations", "file_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err)
	}
	fileID, authorID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO files (id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result) VALUES ($1,$2,'media','image/png',1,$3,'ready','image','clean')`, fileID, authorID, "attachments/"+fileID.String())
	require.NoError(t, err)
	snapshot := func() string {
		var row string
		require.NoError(t, pool.QueryRow(ctx, `SELECT row_to_json(files)::text FROM files WHERE id=$1`, fileID).Scan(&row))
		return row
	}
	before := snapshot()
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0600))
		return path
	}
	ca, caKey, caPEM := storyMediaTestCA(t)
	_, serverPEM, serverKey := storyMediaTestLeaf(t, ca, caKey, x509.ExtKeyUsageServerAuth, []string{"file"})
	_, clientPEM, clientKey := storyMediaTestLeaf(t, ca, caKey, x509.ExtKeyUsageClientAuth, nil)
	caPath := write("ca.pem", caPEM)
	certPath, keyPath := write("client.pem", clientPEM), write("client.key", clientKey)
	serverCertPath, serverKeyPath := write("server.pem", serverPEM), write("server.key", serverKey)
	signingDir := filepath.Join(dir, "signing")
	require.NoError(t, os.Mkdir(signingDir, 0700))
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		writeStoryTestKey(t, filepath.Join(signingDir, kid+".pem"), key)
	}
	for name, value := range map[string]string{
		"FILE_PRINCIPAL_GRPC_ADDR": "127.0.0.1:1", "FILE_PRINCIPAL_TLS_CA_FILE": caPath,
		"FILE_PRINCIPAL_TLS_SERVER_NAME": "file", "FILE_PRINCIPAL_CLIENT_CERT_FILE": certPath,
		"FILE_PRINCIPAL_CLIENT_KEY_FILE": keyPath, "STORY_PRINCIPAL_SIGNING_KEYS_DIR": signingDir,
		"STORY_PRINCIPAL_ACTIVE_KID": "current",
	} {
		t.Setenv(name, value)
	}
	_, initialConn, public, err := loadSignedStoryMediaClientFromEnv()
	require.NoError(t, err)
	require.NoError(t, initialConn.Close())
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(public)
	}))
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	jwksCA := write("jwks-ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.Certificate().Raw}))
	replay := miniredis.RunT(t)
	fixture := filepath.Join(dir, "file-listener")
	build := exec.CommandContext(ctx, "go", "build", "-o", fixture, "./testfixture/story_media_listener")
	build.Dir = filepath.Join("..", "file")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	command := exec.CommandContext(ctx, fixture)
	input, err := command.StdinPipe()
	require.NoError(t, err)
	outputPipe, err := command.StdoutPipe()
	require.NoError(t, err)
	stderr, err := os.Create(filepath.Join(dir, "file-stderr.log"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stderr.Close()) })
	command.Stderr = stderr
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		_ = input.Close()
		waitErr := command.Wait()
		log, _ := os.ReadFile(stderr.Name())
		require.NoError(t, waitErr, "%s", log)
	})
	configuration := map[string]any{"DSN": pool.Config().ConnString(), "Config": map[string]any{
		"ClientCAFile": caPath, "JWKSURLs": map[string]string{"story": jwks.URL, "messaging": jwks.URL},
		"RefreshAfter": 30 * time.Second, "HardExpiry": 2 * time.Minute, "UnknownKIDCooldown": 5 * time.Second,
		"ReplayAddr": replay.Addr(), "JWKSCAFile": jwksCA, "JWKSClientCertFile": certPath, "JWKSClientKeyFile": keyPath,
		"TLSCertFile": serverCertPath, "TLSKeyFile": serverKeyPath, "ListenAddr": "127.0.0.1:0",
	}}
	require.NoError(t, json.NewEncoder(input).Encode(configuration))
	var address string
	require.NoError(t, json.NewDecoder(bufio.NewReader(outputPipe)).Decode(&address))
	t.Setenv("FILE_PRINCIPAL_GRPC_ADDR", address)
	client, conn, _, err := loadSignedStoryMediaClientFromEnv()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	call := func(c *signedStoryMediaClient, author uuid.UUID) error {
		callCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		return c.ValidateStoryMedia(callCtx, fileID, author, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO)
	}
	t.Run("trusted configured client reaches real persistence", func(t *testing.T) {
		require.NoError(t, call(client, authorID))
		require.Equal(t, before, snapshot(), "validation is read-only")
		require.Equal(t, codes.PermissionDenied, status.Code(call(client, uuid.New())), "real File ownership predicate")
		_, err := pool.Exec(ctx, `UPDATE files SET status='processing' WHERE id=$1`, fileID)
		require.NoError(t, err)
		require.Equal(t, codes.FailedPrecondition, status.Code(call(client, authorID)), "real File lifecycle predicate")
		_, err = pool.Exec(ctx, `UPDATE files SET status='ready' WHERE id=$1`, fileID)
		require.NoError(t, err)
	})
	for _, missing := range []bool{true, false} {
		name := "untrusted certificate"
		if missing {
			name = "missing certificate"
		}
		t.Run(name, func(t *testing.T) {
			config := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "file"}
			if !missing {
				foreignCA, foreignKey, _ := storyMediaTestCA(t)
				pair, _, _ := storyMediaTestLeaf(t, foreignCA, foreignKey, x509.ExtKeyUsageClientAuth, nil)
				config.Certificates = []tls.Certificate{pair}
			}
			badConn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(config)), grpc.WithDisableRetry())
			require.NoError(t, err)
			defer badConn.Close()
			require.Equal(t, codes.Unavailable, status.Code(call(&signedStoryMediaClient{FileServiceClient: filev1.NewFileServiceClient(badConn), issuer: client.issuer}, authorID)))
		})
	}
	t.Run("trusted TLS does not replace signed exact binding", func(t *testing.T) {
		bound := func(recording *recordingMediaClient) context.Context {
			md, _ := metadata.FromOutgoingContext(recording.ctx)
			return metadata.NewOutgoingContext(ctx, md.Copy())
		}
		recording := &recordingMediaClient{}
		signer := &signedStoryMediaClient{FileServiceClient: recording, issuer: client.issuer}
		require.NoError(t, call(signer, authorID))
		changed := proto.Clone(recording.req).(*filev1.ValidateStoryMediaRequest)
		changed.AuthorProfileId = uuid.NewString()
		_, err := client.FileServiceClient.ValidateStoryMedia(bound(recording), changed)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.NoError(t, call(signer, authorID))
		md, _ := metadata.FromOutgoingContext(recording.ctx)
		md = md.Copy()
		md.Set("x-request-id", uuid.NewString())
		_, err = client.FileServiceClient.ValidateStoryMedia(metadata.NewOutgoingContext(ctx, md), recording.req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		_, err = client.FileServiceClient.ValidateStoryMedia(ctx, recording.req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		keys, err := loadStorySigningKeys(signingDir)
		require.NoError(t, err)
		otherIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: keys["current"]})
		require.NoError(t, err)
		require.Equal(t, codes.PermissionDenied, status.Code(call(&signedStoryMediaClient{FileServiceClient: client.FileServiceClient, issuer: otherIssuer}, authorID)))
	})
	require.Equal(t, before, snapshot())
}
