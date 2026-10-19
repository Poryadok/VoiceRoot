//go:build linux

package nativeapp

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/queue"
	w "voice/backend/pkg/mmadmissionwire"
	"voice/backend/pkg/principal"
)

type rootSnapshotFixture struct {
	callsv1.UnimplementedVoiceServiceServer
	keys               map[string]*rsa.PublicKey
	accounts, profiles []uuid.UUID
	room               uuid.UUID
	mu                 sync.Mutex
	seen               map[string]bool
	hold, entered      chan struct{}
}

func (f *rootSnapshotFixture) GetMatchmakingPartySnapshot(ctx context.Context, req *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid fixture principal")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid fixture request")
	}
	_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "matchmaking", ExpectedAudience: "voice", ExpectedRPC: partysource.SnapshotMethod, ExpectedRequestID: req.SearchOperationId, ExpectedRequestHash: hash, KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
		key := f.keys[kid]
		if issuer != "matchmaking" || key == nil {
			return nil, w.ErrInvalid
		}
		return key, nil
	}, ReplayGuard: func(_ context.Context, issuer, id string, _ time.Time) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.seen[id] {
			return w.ErrInvalid
		}
		f.seen[id] = true
		return nil
	}})
	if err != nil || transport.RequestID != req.SearchOperationId || req.InitiatorAccountId != f.accounts[0].String() || req.InitiatorProfileId != f.profiles[0].String() || req.InitiatorSessionEpoch != 7 {
		return nil, status.Error(codes.Unauthenticated, "invalid fixture authority")
	}
	f.mu.Lock()
	hold, entered := f.hold, f.entered
	f.mu.Unlock()
	if hold != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	response := &callsv1.GetMatchmakingPartySnapshotResponse{ProtocolVersion: 1, Kind: callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_VOICE_ROSTER, RoomId: f.room.String(), RosterVersion: 1, MemberAuthorityVersion: 1, SourceAuthoritySha256: make([]byte, 32)}
	response.SourceAuthoritySha256[0] = 1
	order := []int{0, 1}
	slices.SortFunc(order, func(a, b int) int { return strings.Compare(f.profiles[a].String(), f.profiles[b].String()) })
	for _, index := range order {
		response.MemberProfileIds = append(response.MemberProfileIds, f.profiles[index].String())
		response.MemberAuthorities = append(response.MemberAuthorities, &callsv1.MatchmakingPartyMemberAuthority{ProfileId: f.profiles[index].String(), AccountId: f.accounts[index].String(), AdmittedSessionEpoch: 7})
	}
	return response, nil
}

func rootLaunchCertificate(t *testing.T, server bool) tls.Certificate {
	t.Helper()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "owned native root IPC"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	if server {
		cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		cert.DNSNames = []string{"mm-controller.test"}
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func rootApplicationLogin(t *testing.T, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	role := "mm_root_" + uuid.NewString()[:8]
	_, err := admin.Exec(ctx, `CREATE ROLE `+role+` LOGIN PASSWORD 'owned-root-fixture'`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `GRANT USAGE ON SCHEMA public TO `+role+`; GRANT SELECT ON ALL TABLES IN SCHEMA public TO `+role+`; GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO `+role+`; GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO `+role)
	require.NoError(t, err)
	rows, err := admin.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public'`)
	require.NoError(t, err)
	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	for _, name := range tables {
		if strings.HasPrefix(name, "mm_native_") || name == "matchmaking_match_response_receipts" || name == "matchmaking_native_timeout_outbox" {
			continue
		}
		_, err = admin.Exec(ctx, `GRANT INSERT,UPDATE,DELETE ON `+pgx.Identifier{"public", name}.Sanitize()+` TO `+role)
		require.NoError(t, err)
	}
	_, err = admin.Exec(ctx, `GRANT INSERT ON mm_native_party_origins,matchmaking_match_response_receipts,matchmaking_native_timeout_outbox TO `+role+`; GRANT UPDATE(delivered_at,lease_token,lease_until,next_attempt_at) ON matchmaking_native_timeout_outbox TO `+role)
	require.NoError(t, err)
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = "owned-root-fixture"
	app, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		app.Close()
		_, _ = admin.Exec(context.Background(), `DROP OWNED BY `+role)
		_, _ = admin.Exec(context.Background(), `DROP ROLE `+role)
	})
	return app
}

// This fixture owns only disposable Linux objects. The controller observation,
// upstream semantic catalogue and launch image are explicitly LOCAL_OWNED test
// authority; they never qualify an installed deployment or native Voice cut.
func TestNativeFixedExecutablePrivateBootstrapServingAndClose(t *testing.T) {
	if os.Getenv("BE011_TEST_NATIVE_ROOT") != "1" {
		t.Skip("requires own Linux root driver with dedicated RO installation mount")
	}
	require.Zero(t, os.Geteuid())
	ctx, cancel := context.WithTimeout(context.Background(), 85*time.Second)
	defer cancel()
	source := &rootSnapshotFixture{accounts: []uuid.UUID{uuid.New(), uuid.New()}, profiles: []uuid.UUID{uuid.New(), uuid.New()}, room: uuid.New(), seen: make(map[string]bool)}
	cfg, admin, identity := nativeFactoryConfiguration(t, func(server *grpc.Server, keys []principal.JWKSKey) {
		source.keys = make(map[string]*rsa.PublicKey)
		for _, key := range keys {
			source.keys[key.KeyID] = key.PublicKey
		}
		callsv1.RegisterVoiceServiceServer(server, source)
	})
	app := rootApplicationLogin(t, admin)
	databaseURL, err := url.Parse(cfg.values["DATABASE_URL"])
	require.NoError(t, err)
	databaseURL.User = url.UserPassword(app.Config().ConnConfig.User, app.Config().ConnConfig.Password)
	databaseURL.Path = "/" + app.Config().ConnConfig.Database
	cfg.values["DATABASE_URL"] = databaseURL.String()
	cfg.values["LISTEN_ADDR"] = fixtureAddress(t)
	cfg.values["MATCHMAKING_GRPC_LISTEN"] = fixtureAddress(t)
	cfg.values["MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN"] = fixtureAddress(t)
	// All these fixture files are readable only by the actual serving cohort.
	for key, path := range cfg.values {
		if !strings.HasSuffix(key, "_FILE") && !strings.HasSuffix(key, "_DIR") {
			continue
		}
		if path == "" {
			continue
		}
		for parent := filepath.Dir(path); parent != "/tmp" && strings.HasPrefix(parent, "/tmp/"); parent = filepath.Dir(parent) {
			require.NoError(t, os.Chmod(parent, 0755))
		}
		require.NoError(t, filepath.WalkDir(path, func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			return os.Chown(p, 65532, 65532)
		}))
	}
	executable, err := os.ReadFile("/service-native")
	require.NoError(t, err)
	d := cfg.descriptor
	d.AdmittedExecutableHash = sha256.Sum256(executable)
	facts := observeDBFixture(t, app)
	d.Binding.DatabaseBindingHash = w.Hash(facts.DatabaseBindingHash)
	d.Binding.SchemaContractHash = w.Hash(facts.SchemaContractHash)
	d.Binding.ExpectedLoginRoleOID = facts.SessionRoleOID
	d.Binding.ExpectedLoginRoleName = facts.SessionRoleName
	d.Binding.SourceFactoryManifestHash, d.Binding.FreshRouteManifestHash = nativeCodeContracts()
	cfg.descriptor = d
	initialRaw, err := json.Marshal(d)
	require.NoError(t, err)
	resolved, err := resolveConfig(func(key string) string { return cfg.values[key] }, func() ([]byte, error) { return initialRaw, nil })
	require.NoError(t, err)
	cfg.values = resolved.values
	contracts, err := observeConfigurationContracts(cfg)
	require.NoError(t, err)
	d.ConfigContractHash = contracts.Config
	d.CredentialRefsManifestHash = contracts.Credentials
	d.UpstreamContractManifestHash = contracts.Upstream
	d.BrokerContractManifestHash = contracts.Broker
	d, stamp := installNativeHeadFixture(t, admin, d)
	require.NoError(t, checkNativeDatabase(ctx, app, d, stamp))
	dRaw, err := json.Marshal(d)
	require.NoError(t, err)
	writeProtected := func(name string, raw []byte) {
		require.NoError(t, os.WriteFile(filepath.Join("/control", name), raw, 0444))
	}
	writeProtected(filepath.Base(w.DescriptorPath), dRaw)
	require.NoError(t, os.MkdirAll(w.WorkloadSocketDir, 0770))
	require.NoError(t, os.Chown(w.WorkloadSocketDir, 0, 65532))
	require.NoError(t, os.Chmod(w.WorkloadSocketDir, 0770))
	serverTLS, clientTLS := rootLaunchCertificate(t, true), rootLaunchCertificate(t, false)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join("/control", filepath.Base(w.ReadbackSocket)), Net: "unix"})
	require.NoError(t, err)
	defer listener.Close()
	require.NoError(t, os.Chmod(listener.Addr().String(), 0666))
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	require.NoError(t, err)
	parent := os.NewFile(uintptr(pair[0]), "owned-driver-bootstrap")
	child := os.NewFile(uintptr(pair[1]), "owned-native-fd3")
	raw, err := net.FileConn(parent)
	require.NoError(t, err)
	require.NoError(t, parent.Close())
	defer raw.Close()
	command := exec.Command("/service-native")
	command.Env = []string{"PATH=/usr/bin:/bin"}
	for key, value := range cfg.values {
		command.Env = append(command.Env, key+"="+value)
	}
	command.ExtraFiles = []*os.File{child}
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65532, Gid: 65532}}
	output, err := os.Create(filepath.Join(t.TempDir(), "native-child.log"))
	require.NoError(t, err)
	defer output.Close()
	command.Stdout = output
	command.Stderr = output
	require.NoError(t, command.Start())
	require.NoError(t, child.Close())
	childDone := make(chan error, 1)
	go func() { childDone <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGTERM)
		select {
		case <-childDone:
		case <-time.After(4 * time.Second):
			_ = command.Process.Kill()
			<-childDone
		}
		log, _ := os.ReadFile(output.Name())
		t.Logf("actual native child: %s", log)
	})
	bh, err := w.BindingHash(d.Binding)
	require.NoError(t, err)
	sc, err := x509.ParseCertificate(serverTLS.Certificate[0])
	require.NoError(t, err)
	cc, err := x509.ParseCertificate(clientTLS.Certificate[0])
	require.NoError(t, err)
	ns := w.ID(uuid.New())
	peer := w.ReadbackPeerBindingV1{Version: 1, Transport: w.LaunchTransport, AuthorityScope: d.AuthorityScope, ScopeID: d.ScopeID, EnvironmentID: d.Binding.EnvironmentID, InstallationID: d.Binding.InstallationID, ControllerID: d.ControllerID, BindingHash: bh, ConsumerReplicaID: d.ReplicaID, ConsumerProcessLaunchID: d.ProcessLaunchID, ControllerProcessLaunchID: w.ID(uuid.New()), ControllerLaunchObservationHash: w.Hash{21}, ConsumerLaunchObservationHash: w.Hash{22}, SocketMountObservationHash: w.Hash{23}, ControllerExpectedKernelPeer: w.NamespacedKernelPeer{PID: int32(os.Getpid()), UID: 0, GID: 0, ObserverNamespaceID: ns}, ConsumerExpectedKernelPeer: w.NamespacedKernelPeer{PID: int32(command.Process.Pid), UID: 65532, GID: 65532, ObserverNamespaceID: ns}, ControllerImageDigest: d.AdmittedImageDigest, ControllerExecutableHash: w.Hash{24}, ConsumerImageDigest: d.AdmittedImageDigest, ConsumerExecutableHash: d.AdmittedExecutableHash, ControllerCertificateDER: sc.Raw, ControllerCertificateDERHash: sha256.Sum256(sc.Raw), ControllerSPKIHash: sha256.Sum256(sc.RawSubjectPublicKeyInfo), ConsumerCertificateDER: cc.Raw, ConsumerCertificateDERHash: sha256.Sum256(cc.Raw), ConsumerSPKIHash: sha256.Sum256(cc.RawSubjectPublicKeyInfo), ControllerServerName: "mm-controller.test", LaunchKeyCustodyContractHash: w.Hash{25}}
	peer.PeerBindingHash, err = w.PeerBindingHash(peer)
	require.NoError(t, err)
	peerRaw, err := json.Marshal(peer)
	require.NoError(t, err)
	writeProtected(filepath.Base(w.PeerBindingPath), peerRaw)
	var open atomic.Bool
	var exchanges sync.WaitGroup
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		for {
			connection, e := listener.AcceptUnix()
			if e != nil {
				return
			}
			exchanges.Add(1)
			go func() {
				defer exchanges.Done()
				defer connection.Close()
				_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
				secure, e := w.AuthenticateReadbackServer(ctx, connection, peer, serverTLS)
				if e != nil {
					return
				}
				var request w.ObserveRequestV1
				if w.ReadFrame(secure, &request) != nil {
					return
				}
				var tail [1]byte
				if n, e := secure.Read(tail[:]); n != 0 || e != io.EOF {
					return
				}
				state := w.Armed
				var openHash *w.Hash
				if open.Load() {
					state = w.Open
					h := w.Hash{26}
					openHash = &h
				}
				response := w.ObserveResponseV1{ProtocolVersion: 1, Nonce: request.Nonce, ControllerID: d.ControllerID, AuthorityScope: d.AuthorityScope, ScopeID: d.ScopeID, State: state, StateRevision: 1, TransitionID: d.Binding.TransitionID, DatabaseLineageID: d.DatabaseLineageID, ExactBinding: d.Binding, BindingHash: bh, ArmedReceiptHash: d.Binding.AdmissionReceiptHash, OpenReceiptHash: openHash, LaunchObservationHash: peer.ConsumerLaunchObservationHash, ContinuityReceiptsRootHash: stamp.ContinuityHistoryRootHash, ActionID: request.ActionID, ActionRequestHash: request.ActionRequestHash}
				if w.WriteFrame(secure, response) == nil {
					_ = secure.CloseWrite()
				}
			}()
		}
	}()
	defer func() { _ = listener.Close(); <-serverDone; exchanges.Wait() }()
	expected := w.LaunchExpectations{ControllerProcessLaunchID: peer.ControllerProcessLaunchID, ControllerObservationHash: peer.ControllerLaunchObservationHash, ConsumerObservationHash: peer.ConsumerLaunchObservationHash, SocketMountObservationHash: peer.SocketMountObservationHash, CustodyContractHash: peer.LaunchKeyCustodyContractHash, ControllerNamespaceID: ns, ConsumerNamespaceID: ns}
	require.NoError(t, w.WriteNativeLaunchBootstrap(ctx, raw.(*net.UnixConn), dRaw, peer.ConsumerExpectedKernelPeer.Kernel(), expected, clientTLS))
	controlPath := filepath.Join(w.WorkloadSocketDir, d.ReplicaID.String()+".sock")
	requestControl := func(method string) (w.WorkloadResponseV1, error) {
		var response w.WorkloadResponseV1
		connection, e := net.DialTimeout("unix", controlPath, time.Second)
		if e != nil {
			return response, e
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
		request := w.WorkloadRequestV1{ProtocolVersion: 1, Method: method, ReplicaID: d.ReplicaID, ProcessLaunchID: d.ProcessLaunchID, TransitionID: d.Binding.TransitionID, ExpectedBinding: d.Binding, Nonce: w.Hash{27}}
		if e = w.WriteFrame(connection, request); e != nil {
			return response, e
		}
		_ = connection.(*net.UnixConn).CloseWrite()
		e = w.ReadFrame(connection, &response)
		return response, e
	}
	var prepared w.WorkloadResponseV1
	require.Eventually(t, func() bool { prepared, err = requestControl("READ_REPLICA_STATUS"); return err == nil }, 15*time.Second, 25*time.Millisecond)
	require.Equal(t, w.Armed, prepared.State)
	for _, address := range []string{cfg.values["LISTEN_ADDR"], cfg.values["MATCHMAKING_GRPC_LISTEN"], cfg.values["MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN"]} {
		connection, e := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if connection != nil {
			_ = connection.Close()
		}
		require.Error(t, e, "ARMED cannot serve ordinary/protected/health")
	}
	open.Store(true)
	require.Eventually(t, func() bool {
		response, e := http.Get("http://" + cfg.values["LISTEN_ADDR"] + "/ready")
		if e != nil {
			return false
		}
		defer response.Body.Close()
		return response.StatusCode == http.StatusOK
	}, 10*time.Second, 25*time.Millisecond)
	connection, err := grpc.NewClient(cfg.values["MATCHMAKING_GRPC_LISTEN"], grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()
	client := matchmakingv1.NewMatchmakingServiceClient(connection)
	_, err = client.ListGames(ctx, &matchmakingv1.ListGamesRequest{})
	require.NoError(t, err)
	_, err = client.StartSearch(ctx, &matchmakingv1.StartSearchRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	// Real delegated JWT + mTLS enters the same fixed child and obtains its
	// source over a real service-principal-verified upstream mTLS call.
	keyBytes, err := os.ReadFile(filepath.Join(cfg.values["MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR"], "current.pem"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyBytes)
	require.NotNil(t, block)
	private, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: private.(*rsa.PrivateKey)})
	require.NoError(t, err)
	protectedConnection, err := grpc.NewClient(cfg.values["MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN"], grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{identity.client}, RootCAs: identity.roots, ServerName: "localhost"})))
	require.NoError(t, err)
	defer protectedConnection.Close()
	protectedClient := matchmakingv1.NewMatchmakingServiceClient(protectedConnection)
	rdb := redis.NewClient(&redis.Options{Addr: cfg.values["MATCHMAKING_REDIS_ADDR"]})
	defer rdb.Close()
	for _, account := range source.accounts {
		require.NoError(t, rdb.Set(ctx, "auth:session:min_epoch:"+account.String(), "7", 0).Err())
		defer rdb.Del(context.Background(), "auth:session:min_epoch:"+account.String())
	}
	var gameID uuid.UUID
	require.NoError(t, admin.QueryRow(ctx, `SELECT id FROM games WHERE name='Dota 2'`).Scan(&gameID))
	search := &matchmakingv1.StartSearchRequest{GameId: gameID.String(), Mode: "5v5 Ranked", CriteriaJson: `{"region":"eu","self":{"role":"Carry","rank":"Herald"},"sought":{"rank_min":"Herald","rank_max":"Guardian"}}`, OperationId: uuid.NewString()}
	searchContext := func(req *matchmakingv1.StartSearchRequest) context.Context {
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		token, e := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_StartSearch_FullMethodName, RequestID: req.OperationId, RequestHash: hash, AccountID: source.accounts[0].String(), ProfileID: source.profiles[0].String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.OperationId))
	}
	started, err := protectedClient.StartSearch(searchContext(search), search)
	require.NoError(t, err)
	var partyID uuid.UUID
	require.NoError(t, admin.QueryRow(ctx, `SELECT party_id FROM search_sessions WHERE id=$1`, started.GetSearchSession().GetId()).Scan(&partyID))
	var sessionIDs []uuid.UUID
	rows, err := admin.Query(ctx, `SELECT id FROM search_sessions WHERE party_id=$1 ORDER BY id`, partyID)
	require.NoError(t, err)
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		sessionIDs = append(sessionIDs, id)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Len(t, sessionIDs, 2)
	q := &queue.RedisQueue{Client: rdb, Prefix: "mm"}
	require.Eventually(t, func() bool {
		ids, e := q.ListSessionIDs(ctx, gameID, search.Mode, "eu", 100)
		return e == nil && slices.Contains(ids, sessionIDs[0]) && slices.Contains(ids, sessionIDs[1])
	}, 5*time.Second, 25*time.Millisecond, "actual root projector enqueues complete captured party")
	for _, profile := range source.profiles {
		require.NoError(t, rdb.Exists(ctx, "mm:lock:profile:"+profile.String()).Err())
		defer rdb.Del(context.Background(), "mm:lock:profile:"+profile.String())
	}
	_, err = admin.Exec(ctx, `UPDATE search_sessions SET timeout_at=statement_timestamp()-interval '1 second' WHERE party_id=$1`, partyID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		var terminal, delivered int
		if admin.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='timeout' AND recovery_generation=1`, partyID).Scan(&terminal) != nil {
			return false
		}
		if admin.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox WHERE party_id=$1 AND delivered_at IS NOT NULL`, partyID).Scan(&delivered) != nil {
			return false
		}
		ids, e := q.ListSessionIDs(ctx, gameID, search.Mode, "eu", 100)
		if e != nil || terminal != 2 || delivered != 2 || slices.Contains(ids, sessionIDs[0]) || slices.Contains(ids, sessionIDs[1]) {
			return false
		}
		for _, profile := range source.profiles {
			n, e := rdb.Exists(ctx, "mm:lock:profile:"+profile.String()).Result()
			if e != nil || n != 0 {
				return false
			}
		}
		return true
	}, 40*time.Second, 50*time.Millisecond, "actual root timeout -> both PG facts/PubAck -> physical Redis RELEASE and profile unlock")
	for _, id := range sessionIDs {
		defer rdb.HDel(context.Background(), "mm:queue:session-generations", id.String())
	}
	// A held, authenticated upstream RPC keeps the real root registration live.
	// Private CLOSE cannot acknowledge drain before that request commits/returns.
	source.mu.Lock()
	source.hold = make(chan struct{})
	source.entered = make(chan struct{}, 1)
	hold, entered := source.hold, source.entered
	source.mu.Unlock()
	search.OperationId = uuid.NewString()
	pending := make(chan error, 1)
	signed := searchContext(search)
	go func() { _, e := protectedClient.StartSearch(signed, search); pending <- e }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("actual authenticated snapshot request did not enter")
	}
	type closeResult struct {
		response w.WorkloadResponseV1
		err      error
	}
	closing := make(chan closeResult, 1)
	go func() { response, e := requestControl("CLOSE_REPLICA"); closing <- closeResult{response, e} }()
	require.Eventually(t, func() bool {
		response, e := http.Get("http://" + cfg.values["LISTEN_ADDR"] + "/ready")
		if e != nil {
			return false
		}
		defer response.Body.Close()
		return response.StatusCode == http.StatusServiceUnavailable
	}, time.Second, 10*time.Millisecond, "CLOSE has actually closed admission while the RPC is still held")
	select {
	case <-closing:
		t.Fatal("private CLOSE bypassed active StartSearch RPC")
	case <-time.After(75 * time.Millisecond):
	}
	close(hold)
	require.NoError(t, <-pending)
	closeOutcome := <-closing
	closed, err := closeOutcome.response, closeOutcome.err
	require.NoError(t, err)
	require.Equal(t, w.Closed, closed.State)
	require.Empty(t, closed.InFlightWorkIDs)
	require.Empty(t, closed.UnresolvedProviderOperationIDs)
	response, err := http.Get("http://" + cfg.values["LISTEN_ADDR"] + "/ready")
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	_ = response.Body.Close()
	_, err = client.ListGames(ctx, &matchmakingv1.ListGamesRequest{})
	require.NoError(t, err, "terminal/history/catalog remain available after forward closure")
	t.Logf("actual /service-native PID%d UID65532 privateFD3 -> ARMED no serving -> OPEN readiness/catalog -> irreversible private CLOSE", command.Process.Pid)
}
