package inventory

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type composedIdentity struct {
	Certificate       tls.Certificate
	CertPath, KeyPath string
}

func composedIdentityFiles(t *testing.T, dir, name string, ca *x509.Certificate, caKey *ecdsa.PrivateKey) composedIdentity {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	require.NoError(t, err)
	uri, err := url.Parse("spiffe://voice/service/" + name)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{name}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, URIs: []*url.URL{uri}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certPEM, keyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	out := composedIdentity{Certificate: cert, CertPath: filepath.Join(dir, name+".crt"), KeyPath: filepath.Join(dir, name+".key")}
	require.NoError(t, os.WriteFile(out.CertPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(out.KeyPath, keyPEM, 0600))
	return out
}

type composedGC struct{ calls atomic.Int64 }

func (g *composedGC) DeleteObject(context.Context, string) error { g.calls.Add(1); return nil }

// This is an actual two-service source proof: an isolated Story process owns
// its own PostgreSQL inventory, signed mTLS export and replay runtime. File
// imports that export into another database and runs its actual object-GC path.
// It proves retention only; no global release authority is injected or claimed.
func TestStoryProcessInventoryProtectsHistoricalSharedBlob(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)
	filePool := integrationtest.StartPostgres(t, ctx, "file_inventory_composed", "")
	storyPool := integrationtest.StartPostgres(t, ctx, "story_inventory_composed", "")
	_, sourceFile, _, _ := runtime.Caller(0)
	backend := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", ".."))
	migrations, err := filepath.Glob(filepath.Join(backend, "migrations", "file_db", "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, migrations)
	for _, name := range migrations {
		sql, e := os.ReadFile(name)
		require.NoError(t, e)
		_, e = filePool.Exec(ctx, string(sql))
		require.NoError(t, e, filepath.Base(name))
	}
	author, blob, fileA, fileB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	digest := sha256.Sum256([]byte(blob.String()))
	_, err = filePool.Exec(ctx, `INSERT INTO file_blobs(blob_id,sha256,original_r2_key,state,gc_operation_id,next_attempt_at) VALUES($1,$2,'owned-inventory/original','GC_PENDING',$3,now())`, blob, digest[:], uuid.New())
	require.NoError(t, err)
	for _, file := range []uuid.UUID{fileA, fileB} {
		_, err = filePool.Exec(ctx, `INSERT INTO files(id,blob_id,uploader_profile_id,original_name,mime_type,size_bytes,sha256_hash,r2_key,status,file_type,is_e2e,scan_result) VALUES($1,$2,$3,'historical.png','image/png',1,$4,'owned-inventory/original','ready','image',false,'clean')`, file, blob, author, fmt.Sprintf("%x", digest))
		require.NoError(t, err)
	}
	dir := t.TempDir()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "owned inventory test CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(caPath, caPEM, 0600))
	serverCert := composedIdentityFiles(t, dir, "story-inventory", ca, caKey)
	fileCert := composedIdentityFiles(t, dir, "file", ca, caKey)
	jwksCert := composedIdentityFiles(t, dir, "file-jwks", ca, caKey)
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var keys []map[string]string
	for kid, key := range map[string]*rsa.PrivateKey{"current": current, "next": next} {
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{jwksCert.Certificate}}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	replay := miniredis.RunT(t)
	readyPath, stopPath := filepath.Join(dir, "ready.json"), filepath.Join(dir, "stop")
	input := map[string]any{"DatabaseURL": storyPool.Config().ConnString(), "ReadyPath": readyPath, "StopPath": stopPath, "Pins": []map[string]string{{"FileID": fileA.String(), "AuthorID": author.String()}, {"FileID": fileB.String(), "AuthorID": author.String()}}, "Config": map[string]any{"ListenAddr": "127.0.0.1:0", "TLSCertFile": serverCert.CertPath, "TLSKeyFile": serverCert.KeyPath, "ClientCAFile": caPath, "ReplayAddr": replay.Addr(), "FileJWKSURL": jwks.URL + "/.well-known/jwks.json", "JWKSCAFile": caPath}}
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	inputPath := filepath.Join(dir, "owner-input.json")
	require.NoError(t, os.WriteFile(inputPath, raw, 0600))
	logFile, err := os.Create(filepath.Join(dir, "owner-process.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = logFile.Close() })
	fixtureLog := func() string {
		b, _ := os.ReadFile(logFile.Name())
		if len(b) > 4096 {
			b = b[len(b)-4096:]
		}
		return string(b)
	}
	command := exec.CommandContext(ctx, "go", "test", "-p", "1", "-count=1", "-timeout", "5m", "-run", "^TestReferenceInventoryOwnerFixtureProcess$", ".")
	command.Dir = filepath.Join(backend, "story")
	command.Env = append(os.Environ(), "VOICE_REFERENCE_INVENTORY_OWNER_FIXTURE="+inputPath, "GOMAXPROCS=2")
	command.Stdout = logFile
	command.Stderr = logFile
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, command.Start())
	done := make(chan struct{})
	var processErr error
	go func() { processErr = command.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = os.WriteFile(stopPath, []byte("stop"), 0600)
		select {
		case <-done:
			if processErr != nil {
				t.Errorf("owned Story fixture failed: %v (private log %s)", processErr, logFile.Name())
			}
		case <-time.After(10 * time.Second):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
			t.Error("owned Story fixture did not stop")
		}
	})
	var ready struct {
		Address, OwnerIncarnationID string
		Stories                     map[string]string
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for ready.Address == "" {
		select {
		case <-ctx.Done():
			t.Fatal("Story fixture readiness deadline")
		case <-done:
			t.Fatalf("Story fixture exited before readiness: %v\n%s", processErr, fixtureLog())
		case <-ticker.C:
			if b, e := os.ReadFile(readyPath); e == nil {
				require.NoError(t, json.Unmarshal(b, &ready))
			}
		}
	}
	require.Len(t, ready.Stories, 2)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "current", PrivateKey: current})
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	source, err := DialStorySource(ready.Address, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "story-inventory", Certificates: []tls.Certificate{fileCert.Certificate}}, issuer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = source.Close() })
	identity := &commonv1.ReferenceInventoryIdentity{ProtocolVersion: 1, InstallationId: uuid.NewString(), RestoreCohortId: uuid.NewString(), MigrationId: uuid.NewString(), FileIncarnationId: uuid.NewString(), SourceService: "story", SourceComponent: "story.media.v1", OwnerIncarnationId: ready.OwnerIncarnationID, WriterProtocolVersion: 1, SnapshotId: uuid.NewString()}
	files := store.NewFilesStore(filePool)
	operation := uuid.New()
	require.NoError(t, files.CreateInventoryOperation(ctx, operation, identity))
	gc := &composedGC{}
	count, err := files.RunReferenceGCOnce(ctx, gc, 1)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Zero(t, gc.calls.Load(), "the not-yet-arrived snapshot cannot leave a deletion window")
	worker := &Importer{Files: files, Source: source}
	require.NoError(t, worker.Run(ctx, operation))
	o, err := files.LoadInventoryOperation(ctx, operation)
	require.NoError(t, err)
	require.Equal(t, "CATCHING_UP", o.State)
	require.EqualValues(t, 2, o.Descriptor.Manifest.TotalCount)
	require.Zero(t, o.Descriptor.Manifest.UnresolvedCount)
	var holds, refs, issues, missing int
	require.NoError(t, filePool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_holds WHERE blob_id=$1`, blob).Scan(&holds))
	require.Equal(t, 2, holds, "two actual highlighted Story pins share retained bytes")
	require.NoError(t, filePool.QueryRow(ctx, `SELECT count(*) FROM file_references`).Scan(&refs))
	require.Zero(t, refs, "historical import must not grant a readable reference")
	require.NoError(t, filePool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_items WHERE issue_reason IS NOT NULL`).Scan(&issues))
	require.Zero(t, issues)
	require.NoError(t, filePool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_components WHERE state='MISSING'`).Scan(&missing))
	require.GreaterOrEqual(t, missing, 2, "non-Story owners remain explicitly missing")
	page, err := source.ReadReferenceInventoryPage(ctx, &commonv1.ReadReferenceInventoryPageRequest{Identity: identity, PageIndex: 0})
	require.NoError(t, err)
	require.NoError(t, files.ApplyInventoryPage(ctx, operation, page.Page), "exact authenticated page replay is receipt-only")
	require.NoError(t, worker.Run(ctx, operation), "fresh signed retry uses the saved snapshot/cursor")
	count, err = files.RunReferenceGCOnce(ctx, gc, 1)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Zero(t, gc.calls.Load())
	var state string
	var claimed *time.Time
	require.NoError(t, filePool.QueryRow(ctx, `SELECT state,deleted_at FROM file_blobs WHERE blob_id=$1`, blob).Scan(&state, &claimed))
	require.Equal(t, "GC_PENDING", state)
	require.Nil(t, claimed, "barrier prevents the irreversible claim, not only DELETE")
	require.NoError(t, files.InvalidateInventoryCohort(ctx, uuid.MustParse(identity.MigrationId)))
	require.Error(t, worker.Run(ctx, operation), "invalidated cohort cannot advance an old cursor")
	count, err = files.RunReferenceGCOnce(ctx, gc, 1)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Zero(t, gc.calls.Load())
}
