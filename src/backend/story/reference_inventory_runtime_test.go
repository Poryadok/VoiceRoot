package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/referenceinventory"
	"voice/backend/story/internal/inventorygrpc"
)

func TestReferenceInventoryRuntimeAbsentAndMissingStore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runtime, err := startReferenceInventory(context.Background(), inventorygrpc.Config{}, false, nil, logger)
	if err != nil || runtime != nil {
		t.Fatalf("absent runtime: %v %v", runtime, err)
	}
	// Frozen interface baselines compile, but cannot install an enabled exporter
	// before their actual Store implements the immutable inventory contract.
	runtime, err = startReferenceInventory(context.Background(), inventorygrpc.Config{}, true, struct{}{}, logger)
	if err == nil || runtime != nil {
		t.Fatalf("missing Store accepted: %v %v", runtime, err)
	}
}

type runtimeInventoryStore struct {
	descriptor *commonv1.ReferenceInventoryDescriptor
	calls      int
}

func (s *runtimeInventoryStore) BeginReferenceInventory(context.Context, *commonv1.BeginReferenceInventoryRequest) (*commonv1.ReferenceInventoryDescriptor, error) {
	s.calls++
	return s.descriptor, nil
}
func (s *runtimeInventoryStore) ReadReferenceInventoryPage(context.Context, *commonv1.ReferenceInventoryIdentity, uint32) (*commonv1.ReferenceInventoryPage, error) {
	return nil, referenceinventory.ErrUnavailable
}
func (s *runtimeInventoryStore) ReadReferenceInventoryChanges(context.Context, *commonv1.ReferenceInventoryIdentity, uint64, uint32) (*commonv1.ReadReferenceInventoryChangesResponse, error) {
	return nil, referenceinventory.ErrUnavailable
}

func TestReferenceInventoryRuntimeServesPrivateFileRPCAndCloses(t *testing.T) {
	ca, caKey, caPEM := storyMediaTestCA(t)
	_, serverPEM, serverKey := storyMediaTestLeaf(t, ca, caKey, x509.ExtKeyUsageServerAuth, []string{"story"})
	clientCert, _, _ := storyMediaTestLeaf(t, ca, caKey, x509.ExtKeyUsageClientAuth, nil)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(storyJWKS(map[string]*rsa.PrivateKey{"current": key, "next": next}))
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(raw) }))
	defer jwks.Close()
	dir := t.TempDir()
	write := func(name string, raw []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cfg := inventorygrpc.Config{ListenAddr: "127.0.0.1:0", TLSCertFile: write("server.pem", serverPEM), TLSKeyFile: write("server-key.pem", serverKey), ClientCAFile: write("ca.pem", caPEM), FileJWKSURL: jwks.URL, JWKSCAFile: write("jwks-ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.Certificate().Raw})), ReplayAddr: miniredis.RunT(t).Addr()}
	id := &commonv1.ReferenceInventoryIdentity{ProtocolVersion: 1, WriterProtocolVersion: 1, SourceService: "story", SourceComponent: "story.media.v1", InstallationId: uuid.NewString(), RestoreCohortId: uuid.NewString(), MigrationId: uuid.NewString(), FileIncarnationId: uuid.NewString(), OwnerIncarnationId: uuid.NewString(), SnapshotId: uuid.NewString()}
	descriptor, err := referenceinventory.Describe(nil, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	store := &runtimeInventoryStore{descriptor: descriptor}
	runtime, err := startReferenceInventory(context.Background(), cfg, true, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)
	conn, err := grpc.NewClient(runtime.listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "story", Certificates: []tls.Certificate{clientCert}})))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	req := &commonv1.BeginReferenceInventoryRequest{Identity: id, OperationId: uuid.NewString()}
	hash, _ := principal.RequestHash(req)
	rid := uuid.NewString()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "current", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "story", RPC: inventorygrpc.BeginMethod, RequestID: rid, RequestHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", rid)), 3*time.Second)
	defer cancel()
	if _, err := storyv1.NewStoryReferenceInventoryServiceClient(conn).BeginReferenceInventory(ctx, req); err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 {
		t.Fatalf("calls %d", store.calls)
	}
	if len(runtime.server.GetServiceInfo()) != 1 {
		t.Fatal("private listener exposes additional services")
	}
	if err := runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	probe, err := net.Listen("tcp", runtime.listener.Addr().String())
	if err != nil {
		t.Fatalf("listener was not closed: %v", err)
	}
	_ = probe.Close()
}

func TestReferenceInventoryUnavailableOnOrdinaryStoryListener(t *testing.T) {
	server := grpc.NewServer()
	registerOrdinaryStoryServices(server, &storyv1.UnimplementedStoryServiceServer{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = storyv1.NewStoryReferenceInventoryServiceClient(conn).BeginReferenceInventory(ctx, &commonv1.BeginReferenceInventoryRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("ordinary listener exposes inventory: %v", err)
	}
}
