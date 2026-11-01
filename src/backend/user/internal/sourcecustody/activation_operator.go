package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/user/internal/identityassociation"
)

// ActivationOperatorConfig identifies an independently administered operator
// endpoint and its pinned signer. The signer is responsible for the truth of
// the writer, Auth, administrator, endpoint and restore-custody evidence.
type ActivationOperatorConfig struct {
	Address, CertFile, KeyFile, ClientCAFile, CustodianKeyFile string
	AdminURL, SourceURL, KeyManifest, KeysDir                  string
	ClientCertSHA256                                           [32]byte
}

func loadActivationOperatorConfig(lookup func(string) (string, bool)) (*ActivationOperatorConfig, error) {
	if lookup == nil {
		return nil, ErrUnavailable
	}
	names := []string{"CUSTODY_OPERATOR_LISTEN", "CUSTODY_OPERATOR_TLS_CERT_FILE", "CUSTODY_OPERATOR_TLS_KEY_FILE", "CUSTODY_OPERATOR_CLIENT_CA_FILE", "CUSTODY_OPERATOR_CLIENT_CERT_SHA256", "CUSTODY_OPERATOR_SIGNER_PUBLIC_KEY_FILE", "CUSTODY_SOURCE_ADMIN_DATABASE_URL", "CUSTODY_SOURCE_DATABASE_URL", "CUSTODY_USER_KEY_MANIFEST", "CUSTODY_USER_KEYS_DIR"}
	values := make([]string, len(names))
	present := 0
	for i, name := range names {
		var ok bool
		values[i], ok = lookup(name)
		if ok {
			present++
		}
	}
	if present == 0 {
		return nil, nil
	}
	if present != len(names) {
		return nil, ErrUnavailable
	}
	for _, value := range values {
		if value == "" || strings.TrimSpace(value) != value {
			return nil, ErrUnavailable
		}
	}
	decoded, err := hex.DecodeString(values[4])
	if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != values[4] {
		return nil, ErrUnavailable
	}
	var clientPin [32]byte
	copy(clientPin[:], decoded)
	if clientPin == [32]byte{} || !validServerAddress(values[0], false) || !validControlURL(values[6]) || !validControlURL(values[7]) {
		return nil, ErrUnavailable
	}
	return &ActivationOperatorConfig{Address: values[0], CertFile: values[1], KeyFile: values[2], ClientCAFile: values[3], ClientCertSHA256: clientPin, CustodianKeyFile: values[5], AdminURL: values[6], SourceURL: values[7], KeyManifest: values[8], KeysDir: values[9]}, nil
}

func validOperatorPeer(state tls.ConnectionState, expected [32]byte) bool {
	if len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 || expected == [32]byte{} {
		return false
	}
	actual := sha256.Sum256(state.PeerCertificates[0].Raw)
	return subtle.ConstantTimeCompare(actual[:], expected[:]) == 1
}

type activationChallenge struct {
	Challenge                      [32]byte
	ControlSystem, ControlDatabase string
	ControlDatabaseOID             uint32
	Binding                        Binding
}

type activationHandler struct {
	controller    *Controller
	admin, source *pgxpool.Pool
	keys          *identityassociation.Keyset
	signer        ed25519.PublicKey
	mu            sync.Mutex
	opened        *OpenAttestation
}

func (h *activationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.TLS == nil || !r.TLS.HandshakeComplete || len(r.TLS.VerifiedChains) == 0 || r.URL.RawQuery != "" {
		http.Error(w, "unavailable", http.StatusForbidden)
		return
	}
	if r.URL.Path == "/challenge" && r.Method == http.MethodGet {
		challenge, err := h.controller.ActivationChallenge()
		if err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(activationChallenge{Challenge: challenge, ControlSystem: h.controller.controlSystem, ControlDatabase: h.controller.controlDatabase, ControlDatabaseOID: h.controller.controlDatabaseOID, Binding: h.controller.binding})
		return
	}
	if r.URL.Path == "/close" && r.Method == http.MethodPost {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
		if err != nil || len(body) != 0 {
			http.Error(w, "unavailable", http.StatusBadRequest)
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.fenceOpenedLocked() != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		h.controller.mu.Lock()
		h.controller.poisonLocked()
		h.controller.mu.Unlock()
		return
	}
	if r.URL.Path != "/open" || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "unavailable", http.StatusNotFound)
		return
	}
	var claim OpenAttestation
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&claim) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "unavailable", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if h.controller.PrepareOpenSource(ctx, h.admin, claim, h.signer) != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if claim.GuardCutover && h.controller.ActivateQualifiedGuards(ctx, h.admin, h.source, h.keys, claim, h.signer) != nil {
		h.cancelPrepared(claim)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.controller.OpenQualified(ctx, h.admin, h.source, h.keys, claim, h.signer) != nil {
		h.cancelPrepared(claim)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	h.opened = &claim
	w.WriteHeader(http.StatusNoContent)
}

// fenceOpenedLocked is shared by the authenticated close endpoint and orderly
// server cleanup. The caller holds h.mu until the transition is complete.
func (h *activationHandler) fenceOpenedLocked() error {
	if h.opened == nil {
		return ErrUnavailable
	}
	claim := *h.opened
	// Once a transition starts, finish fencing even if the caller disconnects.
	drainCtx, stopDrain := context.WithTimeout(context.Background(), 30*time.Second)
	closeErr := h.controller.Shutdown(drainCtx)
	stopDrain()
	// A stalled permit must not consume the physical recovery budget. Shutdown
	// poisons/releases this leader on an uncertain drain; recovery then obtains
	// its own leadership lock and validates the physical binding independently.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request := CreationRequest{SourceID: claim.Binding.SourceID, Epoch: claim.Binding.Epoch, ControlSystemID: claim.ControlSystem, SourceSystemID: claim.Physical.SystemIdentifier, Database: claim.Physical.Database}
	recoveryErr := RecoverInterruptedActivation(ctx, h.controller.pool, h.admin, request, claim.Physical, claim.WriterRole)
	for errors.Is(recoveryErr, errRecoveryLeaderBusy) {
		select {
		case <-ctx.Done():
			recoveryErr = ErrUnavailable
		case <-time.After(25 * time.Millisecond):
			recoveryErr = RecoverInterruptedActivation(ctx, h.controller.pool, h.admin, request, claim.Physical, claim.WriterRole)
		}
	}
	if recoveryErr == nil {
		h.opened = nil
	}
	if closeErr != nil || recoveryErr != nil {
		h.controller.mu.Lock()
		h.controller.poisonLocked()
		h.controller.mu.Unlock()
		return ErrUnavailable
	}
	return nil
}

// A cancellation between the two operations still requires a durable close
// and physical fencing. If the admin connection cannot be obtained, poison the
// leader and leave the durable intent for independent recovery.
func (h *activationHandler) cancelPrepared(claim OpenAttestation) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := h.admin.Acquire(ctx)
	if err == nil {
		defer admin.Release()
	}
	h.controller.mu.Lock()
	defer h.controller.mu.Unlock()
	if !h.controller.activationPrepared {
		return
	}
	h.controller.activationPrepared = false
	if err != nil {
		h.controller.poisonLocked()
		return
	}
	h.controller.abortOpenLocked(admin, claim)
}

type activationOperator struct {
	server        *http.Server
	admin, source *pgxpool.Pool
	done          chan error
}

type activationOperatorMaterials struct {
	certificate tls.Certificate
	clientRoots *x509.CertPool
	signer      ed25519.PublicKey
	keys        *identityassociation.Keyset
}

// Load all local operator material before performing any recovery mutation.
func loadActivationOperatorMaterials(cfg *ActivationOperatorConfig) (*activationOperatorMaterials, error) {
	if cfg == nil || forbiddenPGEnvironment() || !validControlURL(cfg.AdminURL) || !validControlURL(cfg.SourceURL) || !validServerAddress(cfg.Address, false) || cfg.ClientCertSHA256 == [32]byte{} {
		return nil, ErrUnavailable
	}
	if _, err := pinnedPostgresConfig(cfg.AdminURL); err != nil {
		return nil, ErrUnavailable
	}
	if _, err := pinnedPostgresConfig(cfg.SourceURL); err != nil {
		return nil, ErrUnavailable
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	roots, err := certificatePool(cfg.ClientCAFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	file, err := os.Open(cfg.CustodianKeyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	closeErr := file.Close()
	if err != nil || closeErr != nil || len(raw) == 0 || len(raw) > 4096 {
		return nil, ErrUnavailable
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "PUBLIC KEY" || len(rest) != 0 {
		return nil, ErrUnavailable
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	key, ok := parsed.(ed25519.PublicKey)
	if err != nil || !ok || len(key) != ed25519.PublicKeySize {
		return nil, ErrUnavailable
	}
	keys, err := identityassociation.LoadProductionKeyset(cfg.KeyManifest, cfg.KeysDir)
	if err != nil {
		return nil, ErrUnavailable
	}
	return &activationOperatorMaterials{certificate: cert, clientRoots: roots, signer: append(ed25519.PublicKey(nil), key...), keys: keys}, nil
}

func startActivationOperator(ctx context.Context, cfg *ActivationOperatorConfig, materials *activationOperatorMaterials, controller *Controller) (*activationOperator, error) {
	if ctx == nil || ctx.Err() != nil || cfg == nil || materials == nil || controller == nil {
		return nil, ErrUnavailable
	}
	admin, err := openPinnedPostgres(ctx, cfg.AdminURL)
	if err != nil {
		return nil, ErrUnavailable
	}
	source, err := openPinnedPostgres(ctx, cfg.SourceURL)
	if err != nil {
		admin.Close()
		return nil, ErrUnavailable
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		admin.Close()
		source.Close()
		return nil, ErrUnavailable
	}
	handler := &activationHandler{controller: controller, admin: admin, source: source, keys: materials.keys, signer: materials.signer}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192}
	op := &activationOperator{server: server, admin: admin, source: source, done: make(chan error, 1)}
	clientPin := cfg.ClientCertSHA256
	go func() {
		err := server.Serve(tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{materials.certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: materials.clientRoots, VerifyConnection: func(state tls.ConnectionState) error {
			if !validOperatorPeer(state, clientPin) {
				return ErrUnavailable
			}
			return nil
		}}))
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		op.done <- err
	}()
	return op, nil
}

func (op *activationOperator) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := op.server.Shutdown(ctx)
	if err != nil {
		_ = op.server.Close()
	}
	var fenceErr error
	handler, ok := op.server.Handler.(*activationHandler)
	if !ok {
		fenceErr = ErrUnavailable
	} else {
		handler.mu.Lock()
		if handler.opened != nil {
			fenceErr = handler.fenceOpenedLocked()
		}
		handler.mu.Unlock()
	}
	op.admin.Close()
	op.source.Close()
	if err != nil || fenceErr != nil {
		return ErrUnavailable
	}
	return nil
}
