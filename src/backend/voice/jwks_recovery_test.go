package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/pkg/spacecommunication"
	"voice/backend/voice/internal/principaljwks"
	"voice/backend/voice/internal/s2s"
	"voice/backend/voice/internal/spacemedia"
	"voice/backend/voice/internal/store"
)

type recoverySpaceAuthority struct {
	spacev1.UnimplementedSpaceServiceServer
	calls atomic.Int32
}

func (s *recoverySpaceAuthority) ResolveVoiceRoomAccess(_ context.Context, req *spacev1.ResolveVoiceRoomAccessRequest) (*spacev1.ResolveVoiceRoomAccessResponse, error) {
	s.calls.Add(1)
	return &spacev1.ResolveVoiceRoomAccessResponse{ProtocolVersion: 1, SpaceId: req.GetSpace().GetId(), ProfileId: req.GetProfileId(),
		Active: true, Member: true, Discoverable: true, AccessEpoch: 4, EvaluatedAt: timestamppb.Now(), CommunicationState: spacev1.MemberCommunicationState_MEMBER_COMMUNICATION_STATE_CLEAR}, nil
}

// This regression uses a real persisted COMMITTED/ACTIVE admission, production
// reconciliation, real signed mTLS Space client/verifier and a cold HTTPS JWKS
// cache. Space's domain decision, Role/Auth/media and Redis are explicit test
// fixtures; this is not full Space/Voice/provider acceptance.
func TestNonemptyAdmissionRecoveryNeedsPublishedVoiceJWKS(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL admission recovery integration")
	}
	ctx := t.Context()
	pool := compositionPostgres(t, ctx)
	admissions := spacemedia.NewPostgresAdmissionStore(pool)
	admission := compositionAdmission(t, ctx, admissions)
	redisClient := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	calls := store.NewRedisCallStore(redisClient, uuid.NewString()+":")
	coordinator := &spacemedia.Coordinator{ProjectionGates: calls, Store: calls, Calls: calls, Admissions: admissions,
		SessionEpochChecker: compositionAuthority{}, Grants: compositionAuthority{}, Media: &compositionMedia{}}
	var ready atomic.Bool
	coordinator.OnReadiness = ready.Store
	cfg, caPath, clientCert, clientKey, spaceCert, spaceKey, roots := recoveryTLS(t)
	// Reserve a known currently absent endpoint; the first reconciliation models
	// the legacy root's former publisher-after-recovery ordering.
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	cfg.ListenAddr = reserved.Addr().String()
	require.NoError(t, reserved.Close())
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: cfg.KeyID, PrivateKey: cfg.SigningKey})
	require.NoError(t, err)
	authority := &recoverySpaceAuthority{}
	withColdSpaceVerifier := func(check func()) {
		t.Helper()
		replay := miniredis.RunT(t)
		runtime, err := socialprincipal.New(ctx, socialprincipal.Config{Target: "space", Capability: "space-voice-communication",
			JWKSURLs:     map[string]string{"voice": "https://" + cfg.ListenAddr + "/internal/v1/principal/jwks.json"},
			RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second,
			ReplayAddr: replay.Addr(), JWKSCAFile: caPath, TLSCertFile: spaceCert, TLSKeyFile: spaceKey, ListenAddr: "127.0.0.1:0",
			ClientCAFile: caPath, ClientCAs: roots, JWKSClientCertFile: clientCert, JWKSClientKeyFile: clientKey})
		require.NoError(t, err)
		defer func() { _ = runtime.Close() }()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		server := grpc.NewServer(runtime.ServerOptions()...)
		spacev1.RegisterSpaceServiceServer(server, authority)
		go func() { _ = server.Serve(listener) }()
		defer server.Stop()
		conn, err := (spacecommunication.ClientConfig{Address: listener.Addr().String(), CAFile: caPath, CertFile: clientCert, KeyFile: clientKey}).Dial()
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()
		coordinator.Access = s2s.NewGRPCVoiceRoomAccessResolver(spacev1.NewSpaceServiceClient(conn), issuer)
		check()
	}
	withColdSpaceVerifier(func() {
		require.Error(t, coordinator.ReconcileAll(ctx), "absent real issuer keys must keep recovery closed")
		require.False(t, ready.Load())
		require.Zero(t, authority.calls.Load(), "no unauthenticated domain decision may run")
	})
	publisher, err := principaljwks.Start(cfg, nil)
	require.NoError(t, err)
	defer func() { _ = publisher.Close() }()
	withColdSpaceVerifier(func() {
		require.NoError(t, coordinator.ReconcileAll(ctx))
		require.True(t, ready.Load())
		require.Positive(t, authority.calls.Load(), "nonempty recovery must cross real authentication")
	})
	recovered, err := calls.GetCall(ctx, admission.RoomID)
	require.NoError(t, err)
	require.Equal(t, admission.Generation, recovered.SpaceMedia[admission.ProfileID.String()].Generation)
	var committed, touched int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admissions WHERE operation_id=$1 AND state='COMMITTED' AND participant_state='ACTIVE'`, admission.OperationID).Scan(&committed))
	require.Equal(t, 1, committed)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE lease_token IS NOT NULL OR delivered_at IS NOT NULL`).Scan(&touched))
	require.Zero(t, touched, "startup key publication cannot activate the native roster relay")
}

func recoveryTLS(t *testing.T) (principaljwks.Config, string, string, string, string, string, *x509.CertPool) {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0600))
		return path
	}
	key := func() *rsa.PrivateKey {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		return k
	}
	caKey := key()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "recovery.test"}, IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caPath := write("ca.pem", caPEM)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	leaf := func(name string, usage x509.ExtKeyUsage) (string, string) {
		k := key()
		certificate := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
			KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		der, err := x509.CreateCertificate(rand.Reader, certificate, ca, &k.PublicKey, caKey)
		require.NoError(t, err)
		return write(name+".crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
			write(name+".key", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}))
	}
	voiceCert, voiceKey := leaf("voice", x509.ExtKeyUsageServerAuth)
	spaceCert, spaceKey := leaf("space", x509.ExtKeyUsageServerAuth)
	clientCert, clientKey := leaf("voice-client", x509.ExtKeyUsageClientAuth)
	return principaljwks.Config{KeyID: "current", SigningKey: key(), NextKeyID: "next", NextSigningKey: key(), TLSCert: voiceCert, TLSKey: voiceKey},
		caPath, clientCert, clientKey, spaceCert, spaceKey, roots
}

// The child runs the actual legacy production root, including its own TLS
// publisher, durable recovery, NATS compatibility consumers and health handler.
// Only dependency authorities are fixtures. No coordinator or readiness state
// is constructed in the child by this test.
func TestVoiceProcessRestartRecoversNonemptyAdmission(t *testing.T) {
	if os.Getenv("VOICE_RECOVERY_PROCESS_CHILD") == "1" {
		main()
		return
	}
	if testing.Short() {
		t.Skip("Linux/PostgreSQL/NATS production process integration")
	}
	ctx := t.Context()
	pool := compositionPostgres(t, ctx)
	a := compositionAdmission(t, ctx, spacemedia.NewPostgresAdmissionStore(pool))
	natsURL := compositionNATS(t, ctx)
	projection := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: projection.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	prefix := uuid.NewString() + ":"
	calls := store.NewRedisCallStore(rdb, prefix)
	cfg, ca, clientCert, clientKey, serverCert, serverKey, roots := recoveryTLS(t)
	reserve := func() string {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := l.Addr().String()
		require.NoError(t, l.Close())
		return addr
	}
	cfg.ListenAddr = reserve()
	healthAddr := reserve()
	writeKey := func(name string, key *rsa.PrivateKey) string {
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
		return path
	}
	certificate, err := tls.LoadX509KeyPair(serverCert, serverKey)
	require.NoError(t, err)
	dependency := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})))
	dependencies := &recoveryProcessAuthorities{}
	authv1.RegisterAuthServiceServer(dependency, dependencies)
	rolev1.RegisterRoleServiceServer(dependency, dependencies)
	dependencyListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = dependency.Serve(dependencyListener) }()
	t.Cleanup(dependency.Stop)
	// ConnString retains the original administrative DSN when the existing
	// composition fixture changes Config.Database for its disposable database.
	database := pool.Config().ConnConfig
	dsn := (&url.URL{Scheme: "postgres", User: url.UserPassword(database.User, database.Password),
		Host: net.JoinHostPort(database.Host, strconv.Itoa(int(database.Port))), Path: database.Database,
		RawQuery: "sslmode=disable"}).String()
	env := map[string]string{
		"VOICE_RECOVERY_PROCESS_CHILD": "1", "VOICE_DATABASE_URL": dsn,
		"VOICE_REDIS_ADDR": projection.Addr(), "VOICE_REDIS_PREFIX": prefix, "NATS_URL": natsURL,
		"LISTEN_ADDR": healthAddr, "VOICE_GRPC_LISTEN": "127.0.0.1:0",
		"VOICE_PRINCIPAL_PRIVATE_KEY_FILE": writeKey("current.pem", cfg.SigningKey), "VOICE_PRINCIPAL_KID": cfg.KeyID,
		"VOICE_PRINCIPAL_NEXT_PRIVATE_KEY_FILE": writeKey("next.pem", cfg.NextSigningKey), "VOICE_PRINCIPAL_NEXT_KID": cfg.NextKeyID,
		"VOICE_PRINCIPAL_JWKS_LISTEN_ADDR": cfg.ListenAddr, "VOICE_PRINCIPAL_JWKS_TLS_CERT_FILE": cfg.TLSCert,
		"VOICE_PRINCIPAL_JWKS_TLS_KEY_FILE": cfg.TLSKey,
		"VOICE_ROLE_GRPC_ADDR":              dependencyListener.Addr().String(), "VOICE_ROLE_TLS_CA_FILE": ca,
		"VOICE_ROLE_CLIENT_CERT_FILE": clientCert, "VOICE_ROLE_CLIENT_KEY_FILE": clientKey,
		"VOICE_AUTH_SESSION_FLOOR_GRPC_ADDR": dependencyListener.Addr().String(), "VOICE_AUTH_SESSION_FLOOR_TLS_CA_FILE": ca,
		"VOICE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME": "127.0.0.1", "VOICE_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE": clientCert,
		"VOICE_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE": clientKey,
		"VOICE_USER_PRINCIPAL_REPLAY_REDIS_ADDR":   projection.Addr(), "VOICE_USER_PRINCIPAL_TLS_CERT_FILE": serverCert,
		"VOICE_USER_PRINCIPAL_TLS_KEY_FILE": serverKey, "VOICE_USER_PRINCIPAL_CLIENT_CA_FILE": ca,
		"VOICE_USER_PRINCIPAL_GRPC_LISTEN": "127.0.0.1:0", "S2S_JWKS_CA_FILE": ca,
		"S2S_JWKS_CLIENT_CERT_FILE": clientCert, "S2S_JWKS_CLIENT_KEY_FILE": clientKey,
		"S2S_JWKS_URLS_JSON":                    `{"gateway":"https://` + cfg.ListenAddr + `/internal/v1/principal/jwks.json"}`,
		"VOICE_SPACE_COMMUNICATION_TLS_CA_FILE": ca, "VOICE_SPACE_COMMUNICATION_CLIENT_CERT_FILE": clientCert,
		"VOICE_SPACE_COMMUNICATION_CLIENT_KEY_FILE": clientKey,
	}
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}}}
	t.Cleanup(func() { client.CloseIdleConnections() })
	ready := func() bool {
		response, err := client.Get("http://" + healthAddr + "/ready")
		if err != nil {
			return false
		}
		defer response.Body.Close()
		return response.StatusCode == http.StatusOK
	}
	start := func(t *testing.T, overrides map[string]string) (*exec.Cmd, <-chan error, string) {
		t.Helper()
		executable, err := os.Executable()
		require.NoError(t, err)
		command := exec.Command(executable, "-test.run=^TestVoiceProcessRestartRecoversNonemptyAdmission$")
		values := make(map[string]string, len(env))
		for key, value := range env {
			values[key] = value
		}
		for key, value := range overrides {
			values[key] = value
		}
		for key, value := range values {
			command.Env = append(command.Env, key+"="+value)
		}
		output := filepath.Join(t.TempDir(), "voice.log")
		file, err := os.Create(output)
		require.NoError(t, err)
		command.Stdout, command.Stderr = file, file
		require.NoError(t, command.Start())
		done := make(chan error, 1)
		go func() { result := command.Wait(); _ = file.Close(); done <- result; close(done) }()
		t.Cleanup(func() {
			_ = command.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("owned Voice child did not exit")
			}
		})
		return command, done, output
	}
	// Every iteration creates a new real Space principal runtime: no key cache
	// can survive across the stopped Voice processes.
	space := func(t *testing.T, fail bool, badJWKS bool) (*recoveryProcessSpace, func()) {
		t.Helper()
		endpoint := cfg.ListenAddr
		if badJWKS {
			endpoint = reserve()
		}
		runtime, err := socialprincipal.New(ctx, socialprincipal.Config{Target: "space", Capability: "space-voice-communication",
			JWKSURLs:     map[string]string{"voice": "https://" + endpoint + "/internal/v1/principal/jwks.json"},
			RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second,
			ReplayAddr: miniredis.RunT(t).Addr(), JWKSCAFile: ca, TLSCertFile: serverCert, TLSKeyFile: serverKey,
			ListenAddr: "127.0.0.1:0", ClientCAFile: ca, ClientCAs: roots, JWKSClientCertFile: clientCert, JWKSClientKeyFile: clientKey})
		require.NoError(t, err)
		authority := &recoveryProcessSpace{entered: make(chan struct{}, 1), release: make(chan struct{}), fail: fail,
			spaceID: a.SpaceID.String(), profileID: a.ProfileID.String()}
		server := grpc.NewServer(runtime.ServerOptions()...)
		spacev1.RegisterSpaceServiceServer(server, authority)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		env["VOICE_SPACE_COMMUNICATION_GRPC_ADDR"] = listener.Addr().String()
		go func() { _ = server.Serve(listener) }()
		return authority, func() { server.Stop(); _ = runtime.Close() }
	}
	var firstPID int
	for _, name := range []string{"first_process", "restart_with_cold_space_cache"} {
		t.Run(name, func(t *testing.T) {
			authority, closeSpace := space(t, false, false)
			defer closeSpace()
			command, done, output := start(t, nil)
			if firstPID == 0 {
				firstPID = command.Process.Pid
			} else {
				require.NotEqual(t, firstPID, command.Process.Pid)
			}
			select {
			case <-authority.entered:
			case err := <-done:
				body, _ := os.ReadFile(output)
				t.Fatalf("production startup exited before authenticated recovery: %v %s", err, body)
			case <-time.After(15 * time.Second):
				t.Fatal("production recovery did not reach protected Space")
			}
			require.False(t, ready(), "readiness cannot open while full recovery awaits Space authority")
			response, err := client.Get("https://" + cfg.ListenAddr + "/internal/v1/principal/jwks.json")
			require.NoError(t, err, "real Voice JWKS must already serve during blocked reconciliation")
			var document map[string]json.RawMessage
			require.NoError(t, json.NewDecoder(response.Body).Decode(&document))
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusOK, response.StatusCode)
			encoded, err := json.Marshal(document)
			require.NoError(t, err)
			keys, err := principal.ParseJWKS(encoded)
			require.NoError(t, err)
			require.Len(t, keys, 2)
			require.True(t, cfg.SigningKey.PublicKey.Equal(keys[cfg.KeyID]))
			require.True(t, cfg.NextSigningKey.PublicKey.Equal(keys[cfg.NextKeyID]))
			close(authority.release)
			require.Eventually(t, ready, 10*time.Second, 20*time.Millisecond)
			recovered, err := calls.GetCall(ctx, a.RoomID)
			require.NoError(t, err)
			member := recovered.SpaceMedia[a.ProfileID.String()]
			require.Equal(t, a.Generation, member.Generation)
			require.Equal(t, a.Identity, member.Identity)
			require.Equal(t, a.OperationID.String(), member.AdmissionOperationID)
			require.Equal(t, a.SessionEpoch, member.Issued.SessionEpoch)
			require.Equal(t, a.AccessEpoch, member.Issued.AccessEpoch)
			require.Equal(t, a.PolicyEpoch, member.Issued.PolicyEpoch)
			require.Positive(t, authority.calls.Load())
			require.Positive(t, dependencies.floorCalls.Load())
			require.Positive(t, dependencies.grantCalls.Load())
			// SIGKILL models a lost process, not an in-memory coordinator reset.
			require.NoError(t, command.Process.Signal(syscall.SIGKILL))
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("owned Voice process did not stop")
			}
			body, err := os.ReadFile(output)
			require.NoError(t, err)
			require.NotContains(t, string(body), "WARNING: DATA RACE", "forced restart must not conceal a child race report")
		})
	}
	for _, failure := range []string{"missing_tls", "occupied_jwks", "space_authority_unavailable", "cold_jwks_unavailable", "auth_floor_unavailable", "role_grants_unavailable"} {
		t.Run(failure, func(t *testing.T) {
			dependencies.failFloor.Store(failure == "auth_floor_unavailable")
			dependencies.failGrants.Store(failure == "role_grants_unavailable")
			t.Cleanup(func() { dependencies.failFloor.Store(false); dependencies.failGrants.Store(false) })
			authority, closeSpace := space(t, failure == "space_authority_unavailable", failure == "cold_jwks_unavailable")
			defer closeSpace()
			close(authority.release)
			overrides := map[string]string{}
			if failure == "missing_tls" {
				overrides["VOICE_PRINCIPAL_JWKS_TLS_CERT_FILE"] = filepath.Join(t.TempDir(), "missing.crt")
			}
			if failure == "occupied_jwks" {
				occupied, err := net.Listen("tcp", cfg.ListenAddr)
				require.NoError(t, err)
				defer occupied.Close()
			}
			_, done, output := start(t, overrides)
			deadline := time.NewTimer(15 * time.Second)
			defer deadline.Stop()
			poll := time.NewTicker(10 * time.Millisecond)
			defer poll.Stop()
		waiting:
			for {
				select {
				case err := <-done:
					require.Error(t, err)
					break waiting
				case <-poll.C:
					require.False(t, ready(), "failed startup cannot transiently open readiness")
				case <-deadline.C:
					body, _ := os.ReadFile(output)
					t.Fatalf("failed startup did not exit: %s", body)
				}
			}
			body, err := os.ReadFile(output)
			require.NoError(t, err)
			marker := "Space media initial reconciliation:"
			if failure == "missing_tls" || failure == "occupied_jwks" {
				marker = "voice principal JWKS startup:"
			}
			require.Contains(t, string(body), marker, "negative proof must reach its intended startup boundary")
			require.False(t, ready(), "failed startup must never publish readiness")
			if failure == "missing_tls" || failure == "occupied_jwks" || failure == "cold_jwks_unavailable" {
				require.Zero(t, authority.calls.Load(), "no unauthenticated Space domain decision")
			}
		})
	}
	var committed, touched int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admissions WHERE operation_id=$1 AND state='COMMITTED' AND participant_state='ACTIVE'`, a.OperationID).Scan(&committed))
	require.Equal(t, 1, committed)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE lease_token IS NOT NULL OR delivered_at IS NOT NULL`).Scan(&touched))
	require.Zero(t, touched, "process restart cannot activate dormant A3 roster delivery")
}

type recoveryProcessAuthorities struct {
	authv1.UnimplementedAuthServiceServer
	rolev1.UnimplementedRoleServiceServer
	floorCalls, grantCalls atomic.Int32
	failFloor, failGrants  atomic.Bool
}

func (s *recoveryProcessAuthorities) GetVoiceSessionEpochFloor(context.Context, *authv1.GetVoiceSessionEpochFloorRequest) (*authv1.GetVoiceSessionEpochFloorResponse, error) {
	s.floorCalls.Add(1)
	if s.failFloor.Load() {
		return nil, status.Error(codes.Unavailable, "Auth floor unavailable")
	}
	return &authv1.GetVoiceSessionEpochFloorResponse{SessionEpochFloor: 3}, nil
}

func (s *recoveryProcessAuthorities) ResolveVoiceRoomGrants(context.Context, *rolev1.ResolveVoiceRoomGrantsRequest) (*rolev1.ResolveVoiceRoomGrantsResponse, error) {
	s.grantCalls.Add(1)
	if s.failGrants.Load() {
		return nil, status.Error(codes.Unavailable, "Role grants unavailable")
	}
	return &rolev1.ResolveVoiceRoomGrantsResponse{PolicyEpoch: 5, Grants: &rolev1.VoiceRoomGrants{CanJoin: true, CanSubscribe: true}}, nil
}

type recoveryProcessSpace struct {
	spacev1.UnimplementedSpaceServiceServer
	entered            chan struct{}
	release            chan struct{}
	fail               bool
	spaceID, profileID string
	calls              atomic.Int32
}

func (s *recoveryProcessSpace) ResolveVoiceRoomAccess(ctx context.Context, request *spacev1.ResolveVoiceRoomAccessRequest) (*spacev1.ResolveVoiceRoomAccessResponse, error) {
	s.calls.Add(1)
	if request.GetSpace().GetId() != s.spaceID || request.GetProfileId() != s.profileID {
		return nil, status.Error(codes.PermissionDenied, "unexpected recovered identity")
	}
	select {
	case s.entered <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if s.fail {
		return nil, status.Error(codes.Unavailable, "Space authority unavailable")
	}
	return &spacev1.ResolveVoiceRoomAccessResponse{ProtocolVersion: 1, SpaceId: s.spaceID, ProfileId: s.profileID,
		Active: true, Member: true, Discoverable: true, AccessEpoch: 4, EvaluatedAt: timestamppb.Now(), CommunicationState: spacev1.MemberCommunicationState_MEMBER_COMMUNICATION_STATE_CLEAR}, nil
}
