package sourcecustody

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSupervisionHold means source drain/recovery has not been verified. It never
// authorizes restore, promotion, replacement or reopening.
var ErrSupervisionHold = fmt.Errorf("%w: supervised recovery HOLD", ErrUnavailable)

// SupervisorClientConfig contains public monitor identity and controller-owned
// mTLS credentials. The monitor signing private key belongs to another process.
type SupervisorClientConfig struct {
	URL, CAFile, CertFile, KeyFile, PublicKeyFile string
	ServerCertSHA256                              [32]byte
}

func supervisorPin(value string) ([32]byte, error) {
	var out [32]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != value {
		return out, ErrUnavailable
	}
	copy(out[:], decoded)
	if out == [32]byte{} {
		return out, ErrUnavailable
	}
	return out, nil
}

func loadSupervisorClientConfig(lookup func(string) (string, bool)) (*SupervisorClientConfig, error) {
	names := []string{"CUSTODY_SUPERVISOR_URL", "CUSTODY_SUPERVISOR_TLS_CA_FILE", "CUSTODY_SUPERVISOR_CLIENT_CERT_FILE", "CUSTODY_SUPERVISOR_CLIENT_KEY_FILE", "CUSTODY_SUPERVISOR_SIGNER_PUBLIC_KEY_FILE", "CUSTODY_SUPERVISOR_SERVER_CERT_SHA256"}
	values := make([]string, len(names))
	for i, name := range names {
		value, ok := lookup(name)
		if !ok || value == "" || strings.TrimSpace(value) != value {
			return nil, ErrUnavailable
		}
		values[i] = value
	}
	pin, err := supervisorPin(values[5])
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(values[0])
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
		return nil, ErrUnavailable
	}
	return &SupervisorClientConfig{URL: strings.TrimSuffix(values[0], "/"), CAFile: values[1], CertFile: values[2], KeyFile: values[3], PublicKeyFile: values[4], ServerCertSHA256: pin}, nil
}

func readSupervisorPEM(path, kind string) (*pem.Block, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 4096 {
		return nil, ErrUnavailable
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != kind || len(rest) != 0 {
		return nil, ErrUnavailable
	}
	return block, nil
}

func readSupervisorPublicKey(path string) (ed25519.PublicKey, error) {
	block, err := readSupervisorPEM(path, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	key, ok := parsed.(ed25519.PublicKey)
	if err != nil || !ok || len(key) != ed25519.PublicKeySize {
		return nil, ErrUnavailable
	}
	return key, nil
}

type supervisorClient struct {
	client   *http.Client
	key      ed25519.PublicKey
	endpoint string
}

func newSupervisorClient(cfg *SupervisorClientConfig, custodian ed25519.PublicKey) (*supervisorClient, error) {
	if cfg == nil || cfg.ServerCertSHA256 == [32]byte{} {
		return nil, ErrUnavailable
	}
	key, err := readSupervisorPublicKey(cfg.PublicKeyFile)
	if err != nil || len(custodian) != ed25519.PublicKeySize || bytes.Equal(key, custodian) {
		return nil, ErrUnavailable
	}
	roots, err := certificatePool(cfg.CAFile)
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, ErrUnavailable
	}
	pin := cfg.ServerCertSHA256
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, TLSHandshakeTimeout: time.Second, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: u.Hostname(), Certificates: []tls.Certificate{cert}, VerifyConnection: func(state tls.ConnectionState) error {
		if len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 || sha256.Sum256(state.PeerCertificates[0].Raw) != pin {
			return ErrUnavailable
		}
		return nil
	}}}
	return &supervisorClient{client: &http.Client{Transport: transport, Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrUnavailable }}, key: key, endpoint: strings.TrimSuffix(cfg.URL, "/") + "/probe"}, nil
}

func (c *supervisorClient) probe(ctx context.Context, probe supervisionProbe) (MonitorResponse, error) {
	var response MonitorResponse
	if c == nil || c.client == nil || !validProbe(probe) {
		return response, ErrUnavailable
	}
	body, err := json.Marshal(probe)
	if err != nil {
		return response, ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return response, ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	wire, err := c.client.Do(request)
	if err != nil {
		return response, ErrUnavailable
	}
	defer func() { _ = wire.Body.Close() }()
	if wire.StatusCode != http.StatusOK {
		return response, ErrUnavailable
	}
	decoder := json.NewDecoder(io.LimitReader(wire.Body, 4097))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&response) != nil || decoder.Decode(new(any)) != io.EOF || VerifyMonitorResponse(response, c.key) != nil {
		return MonitorResponse{}, ErrUnavailable
	}
	return response, nil
}

// SupervisorConfig binds one process to one exact independently signed opening
// decision. SourceAdminURL connects to a maintenance database, never the closed
// source itself. Custody of the endpoint, storage and credentials is external.
type SupervisorConfig struct {
	Address, CertFile, KeyFile, ClientCAFile, DecisionFile             string
	PrivateKeyFile, CustodianPublicKeyFile, ControlURL, SourceAdminURL string
	ClientCertSHA256                                                   [32]byte
}

func LoadSupervisorConfig(lookup func(string) (string, bool)) (SupervisorConfig, error) {
	if lookup == nil {
		return SupervisorConfig{}, ErrUnavailable
	}
	names := []string{"CUSTODY_SUPERVISOR_LISTEN", "CUSTODY_SUPERVISOR_TLS_CERT_FILE", "CUSTODY_SUPERVISOR_TLS_KEY_FILE", "CUSTODY_SUPERVISOR_CLIENT_CA_FILE", "CUSTODY_SUPERVISOR_DECISION_FILE", "CUSTODY_SUPERVISOR_SIGNER_PRIVATE_KEY_FILE", "CUSTODY_SUPERVISOR_CUSTODIAN_PUBLIC_KEY_FILE", "CUSTODY_SUPERVISOR_CONTROL_DATABASE_URL", "CUSTODY_SUPERVISOR_SOURCE_ADMIN_DATABASE_URL", "CUSTODY_SUPERVISOR_CLIENT_CERT_SHA256"}
	values := make([]string, len(names))
	for i, name := range names {
		value, ok := lookup(name)
		if !ok || value == "" || strings.TrimSpace(value) != value {
			return SupervisorConfig{}, ErrUnavailable
		}
		values[i] = value
	}
	pin, err := supervisorPin(values[9])
	if err != nil || !validServerAddress(values[0], true) || !validControlURL(values[7]) || !validControlURL(values[8]) {
		return SupervisorConfig{}, ErrUnavailable
	}
	return SupervisorConfig{Address: values[0], CertFile: values[1], KeyFile: values[2], ClientCAFile: values[3], DecisionFile: values[4], PrivateKeyFile: values[5], CustodianPublicKeyFile: values[6], ControlURL: values[7], SourceAdminURL: values[8], ClientCertSHA256: pin}, nil
}

type supervisorHealth struct {
	control, maintenance *pgxpool.Pool
	claim                OpenAttestation
	observedOpening      bool
}

// check is serialized by the handler. It does not infer restore continuity from
// these observations; the independently retained decision evidence supplies it.
var errSupervisorOpeningChanged = errors.New("opening changed during supervisor observation")

func (h *supervisorHealth) check(ctx context.Context, initial bool) error {
	for attempt := 0; attempt < 2; attempt++ {
		err := h.checkSnapshot(ctx, initial)
		if !initial && errors.Is(err, errSupervisorOpeningChanged) && ctx.Err() == nil {
			continue
		}
		if err != nil {
			return ErrUnavailable
		}
		return nil
	}
	return ErrUnavailable
}

func (h *supervisorHealth) checkSnapshot(ctx context.Context, initial bool) error {
	// Every sample starts a new snapshot. Never combine CLOSED from one control
	// cut with ACTIVE from another, and never retain this snapshot across probes.
	control, err := h.control.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = control.Rollback(cleanup)
	}()
	a := h.claim
	var system, database, state, sourceID, phase, writer string
	var oid uint32
	var epoch int64
	var history, generation uuid.UUID
	var digest []byte
	var leaders int
	if control.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()),(SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND granted AND database=(SELECT oid FROM pg_database WHERE datname=current_database()) AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid) FROM pg_control_system()`, leaderKey).Scan(&system, &database, &oid, &leaders) != nil || system != a.ControlSystem || database != a.ControlDatabase || oid != a.ControlDatabaseOID || leaders != 1 {
		return ErrUnavailable
	}
	if control.QueryRow(ctx, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton`).Scan(&state, &epoch, &sourceID, &history, &generation, &digest) != nil || !same(a.Binding, epoch, sourceID, history, generation, digest) {
		return ErrUnavailable
	}
	var physical PhysicalSource
	if control.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, a.Binding.SourceID, a.Binding.Epoch).Scan(&physical.SystemIdentifier, &physical.Database, &physical.DatabaseOID) != nil || physical != a.Physical {
		return ErrUnavailable
	}
	if control.QueryRow(ctx, `SELECT phase FROM public.user_custody_creation WHERE singleton AND source_id=$1 AND epoch=$2 AND control_system_id=$3 AND source_system_id=$4 AND database_name=$5 AND database_oid=$6 AND history_generation=$7 AND user_source_generation=$8 AND keyset_digest=$9`, a.Binding.SourceID, a.Binding.Epoch, a.ControlSystem, a.Physical.SystemIdentifier, a.Physical.Database, a.Physical.DatabaseOID, a.Binding.HistoryGeneration, a.Binding.UserSourceGeneration, a.Binding.KeysetDigest[:]).Scan(&phase) != nil {
		return ErrUnavailable
	}
	dormant := phase == "INITIALIZED_CLOSED" || phase == "EXISTING_INITIALIZED_CLOSED" || phase == "RECOVERED_CLOSED"
	if initial && !dormant || dormant && (h.observedOpening || state != "CLOSED") || phase == "PREPARING_OPEN" && state != "CLOSED" || phase == "ACTIVE" && state != "OPEN" || !dormant && phase != "PREPARING_OPEN" && phase != "ACTIVE" {
		return ErrUnavailable
	}
	if !dormant {
		payload, err := OpenAttestationPayload(a)
		if err != nil {
			return ErrUnavailable
		}
		var recorded, signature []byte
		if control.QueryRow(ctx, `SELECT payload,signature,writer_role FROM public.user_custody_open_evidence WHERE challenge=$1 AND source_id=$2 AND epoch=$3`, a.Challenge[:], a.Binding.SourceID, a.Binding.Epoch).Scan(&recorded, &signature, &writer) != nil || !bytes.Equal(recorded, payload) || !bytes.Equal(signature, a.Signature) || writer != a.WriterRole {
			return ErrUnavailable
		}
		h.observedOpening = true
	}
	var administrator, allowed, login, unsafe, publicConnect bool
	var sessions, prepared int
	if h.maintenance.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&system, &database, &administrator) != nil || system != a.Physical.SystemIdentifier || database == a.Physical.Database || !administrator {
		return ErrUnavailable
	}
	if h.maintenance.QueryRow(ctx, `SELECT oid,datallowconn,EXISTS(SELECT 1 FROM aclexplode(COALESCE(datacl,acldefault('d',datdba))) a WHERE a.grantee=0 AND a.privilege_type='CONNECT') FROM pg_database WHERE datname=$1`, a.Physical.Database).Scan(&oid, &allowed, &publicConnect) != nil || oid != a.Physical.DatabaseOID || publicConnect {
		return ErrUnavailable
	}
	if h.maintenance.QueryRow(ctx, `SELECT rolcanlogin,EXISTS(SELECT 1 FROM pg_roles r WHERE pg_has_role(writer.oid,r.oid,'MEMBER') AND (r.rolsuper OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls)) FROM pg_roles writer WHERE rolname=$1`, a.WriterRole).Scan(&login, &unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	if h.maintenance.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1 OR usename=$3),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2 OR owner=$3)`, a.Physical.DatabaseOID, a.Physical.Database, a.WriterRole).Scan(&sessions, &prepared) != nil || prepared != 0 || phase == "ACTIVE" && !allowed {
		return ErrUnavailable
	}
	if dormant && (allowed || login || sessions != 0) {
		// Prepare commits the signed intent before enabling maintenance access.
		// A new sample must prove that transition; a stable mismatch still fails.
		return errSupervisorOpeningChanged
	}
	return nil
}

type supervisorHandler struct {
	mu           sync.Mutex
	responder    *monitorResponder
	health       *supervisorHealth
	lastProbe    time.Time
	lastSequence uint64
	failed       bool
	failure      chan struct{}
}

func (h *supervisorHandler) failLocked() {
	if !h.failed {
		h.failed = true
		_, _ = h.responder.stop()
		close(h.failure)
	}
}

func (h *supervisorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.TLS == nil || !r.TLS.HandshakeComplete || len(r.TLS.VerifiedChains) == 0 {
		http.Error(w, "unavailable", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/probe" || r.URL.RawQuery != "" || r.URL.RawPath != "" || r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "unavailable", http.StatusNotFound)
		return
	}
	var probe supervisionProbe
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&probe) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "unavailable", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	call, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()
	if h.failed || !h.lastProbe.IsZero() && !time.Now().Before(h.lastProbe.Add(supervisionDeadline)) || h.health.check(call, false) != nil || call.Err() != nil || h.responder.observe(probe) != nil {
		h.failLocked()
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	response, err := h.responder.respond(probe)
	if err != nil {
		h.failLocked()
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	// Retries of an identical sequence never renew either peer's deadline.
	if probe.Sequence > h.lastSequence {
		h.lastProbe = time.Now()
		h.lastSequence = probe.Sequence
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func recoverSupervisedSource(control, maintenance *pgxpool.Pool, claim OpenAttestation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request := CreationRequest{SourceID: claim.Binding.SourceID, Epoch: claim.Binding.Epoch, ControlSystemID: claim.ControlSystem, SourceSystemID: claim.Physical.SystemIdentifier, Database: claim.Physical.Database}
	for {
		err := RecoverInterruptedActivation(ctx, control, maintenance, request, claim.Physical, claim.WriterRole)
		if !errors.Is(err, errRecoveryLeaderBusy) {
			if err != nil {
				return ErrSupervisionHold
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ErrSupervisionHold
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// ServeSupervisor is the production executable's implementation. Every signed
// response performs actual SQL health checks. It never enables source access,
// activates guards, creates lineage or claims custody from configuration alone.
func ServeSupervisor(ctx context.Context, cfg SupervisorConfig, ready func(string)) error {
	if ctx == nil || ctx.Err() != nil || ready == nil || forbiddenPGEnvironment() || cfg.ClientCertSHA256 == [32]byte{} || !validServerAddress(cfg.Address, true) || !validControlURL(cfg.ControlURL) || !validControlURL(cfg.SourceAdminURL) {
		return ErrUnavailable
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return ErrUnavailable
	}
	roots, err := certificatePool(cfg.ClientCAFile)
	if err != nil {
		return err
	}
	custodian, err := readSupervisorPublicKey(cfg.CustodianPublicKeyFile)
	if err != nil {
		return err
	}
	block, err := readSupervisorPEM(cfg.PrivateKeyFile, "PRIVATE KEY")
	if err != nil {
		return err
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	private, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok || len(private) != ed25519.PrivateKeySize {
		return ErrUnavailable
	}
	defer clear(private)
	raw, err := os.ReadFile(cfg.DecisionFile)
	if err != nil || len(raw) == 0 || len(raw) > 16384 {
		return ErrUnavailable
	}
	var claim OpenAttestation
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&claim) != nil || decoder.Decode(new(any)) != io.EOF {
		return ErrUnavailable
	}
	incarnation, err := uuid.NewRandom()
	if err != nil {
		return ErrUnavailable
	}
	responder, err := newDecisionResponder(claim, custodian, private.Public().(ed25519.PublicKey), private, incarnation, time.Now())
	if err != nil {
		return err
	}
	defer func() { _, _ = responder.stop() }()
	control, err := openPinnedPostgres(ctx, cfg.ControlURL)
	if err != nil {
		return err
	}
	defer control.Close()
	maintenance, err := openPinnedPostgres(ctx, cfg.SourceAdminURL)
	if err != nil {
		return err
	}
	defer maintenance.Close()
	health := &supervisorHealth{control: control, maintenance: maintenance, claim: claim}
	initial, cancel := context.WithTimeout(ctx, 3*time.Second)
	err = health.check(initial, true)
	callErr := initial.Err()
	cancel()
	if err != nil || callErr != nil {
		return ErrSupervisionHold
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return ErrUnavailable
	}
	handler := &supervisorHandler{responder: responder, health: health, failure: make(chan struct{})}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: 2 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8192}
	served := make(chan error, 1)
	go func() {
		served <- server.Serve(tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, VerifyConnection: func(state tls.ConnectionState) error {
			if !validOperatorPeer(state, cfg.ClientCertSHA256) {
				return ErrUnavailable
			}
			return nil
		}}))
	}()
	ready(listener.Addr().String())
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	running := true
	for running {
		select {
		case <-ctx.Done():
			running = false
		case <-handler.failure:
			running = false
		case <-served:
			served = nil
			running = false
		case <-ticker.C:
			handler.mu.Lock()
			call, stop := context.WithTimeout(ctx, 500*time.Millisecond)
			if handler.failed || !handler.lastProbe.IsZero() && !time.Now().Before(handler.lastProbe.Add(supervisionDeadline)) || handler.lastProbe.IsZero() && time.Now().Unix() >= claim.ExpiresAtUnix || handler.health.check(call, false) != nil || call.Err() != nil {
				handler.failLocked()
			}
			stop()
			handler.mu.Unlock()
		}
	}
	handler.mu.Lock()
	handler.failLocked()
	handler.mu.Unlock()
	_ = server.Close()
	if served != nil {
		<-served
	}
	// Recovery may return HOLD: the source remains unavailable for transition or
	// reopen until a custodian obtains a verified recovery result. No host stop.
	if recoverSupervisedSource(control, maintenance, claim) != nil {
		return ErrSupervisionHold
	}
	return nil
}

// Keep entropy generation adjacent to the existing monitor protocol.
func randomSupervisionProbe(s *supervisionSession) (supervisionProbe, error) {
	var nonce [32]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return supervisionProbe{}, ErrUnavailable
	}
	return s.issue(nonce)
}
