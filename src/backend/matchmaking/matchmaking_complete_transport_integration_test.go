package main

import (
	"context"
	"crypto"
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
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/grpcsvc"
	"voice/backend/matchmaking/internal/spaceprincipal"
	"voice/backend/matchmaking/internal/store"
)

const gatewayCompleteJWKSPath = "/.well-known/voice-principal-jwks.json"

func TestGatewayCompleteMatchUsesProtectedMMListenerAndPersistsActorLeave(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL and Redis fixtures")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	redisAddr := startMatchSquadRedis(t, ctx)
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = redisClient.Close() })

	accountID := uuid.New()
	profileA, profileB := uuid.New(), uuid.New()
	require.NoError(t, redisClient.Set(ctx, "auth:session:min_epoch:"+accountID.String(), "2", 0).Err())
	matchID := seedCompleteMatchFixture(t, ctx, pool, profileA, profileB)

	fixtureDir := t.TempDir()
	caCert, caKey, caPEM := newMatchSquadTestCA(t)
	caFile := filepath.Join(fixtureDir, "fixture-ca.pem")
	require.NoError(t, os.WriteFile(caFile, caPEM, 0o600))
	mmCert, mmKey := newMatchSquadTestLeaf(t, caCert, caKey, "matchmaking.fixture", nil, x509.ExtKeyUsageServerAuth)
	mmCertFile, mmKeyFile := writeMatchSquadTestPair(t, fixtureDir, "mm-server", mmCert, mmKey)
	clientCert, clientKey := newMatchSquadTestLeaf(t, caCert, caKey, "gateway.fixture", nil, x509.ExtKeyUsageClientAuth)
	clientCertFile, clientKeyFile := writeMatchSquadTestPair(t, fixtureDir, "gateway-client", clientCert, clientKey)
	jwksCert, jwksKey := newMatchSquadTestLeaf(t, caCert, caKey, "gateway-jwks.fixture", []net.IP{net.ParseIP("127.0.0.1")}, x509.ExtKeyUsageServerAuth)
	keyA, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyB, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	principalDir := filepath.Join(fixtureDir, "principal-keys")
	require.NoError(t, os.Mkdir(principalDir, 0o700))
	writeMatchSquadRSAPrivateKey(t, filepath.Join(principalDir, "gateway-current.pem"), keyA)
	writeMatchSquadRSAPrivateKey(t, filepath.Join(principalDir, "gateway-next.pem"), keyB)

	authKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	authJWKS := httptest.NewServer(matchSquadJWKSHandler("auth-test", &authKey.PublicKey))
	t.Cleanup(authJWKS.Close)

	mmListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	mmAddress := mmListener.Addr().String()
	gatewayBinary := buildMatchSquadGateway(t, ctx, fixtureDir)
	gatewayCurrentAddress := startMatchSquadGateway(t, ctx, gatewayBinary, reserveMatchSquadTCPPort(t), mmAddress, redisAddr, caFile, clientCertFile, clientKeyFile, principalDir, authJWKS.URL, "gateway-current")
	gatewayNextAddress := startMatchSquadGateway(t, ctx, gatewayBinary, reserveMatchSquadTCPPort(t), mmAddress, redisAddr, caFile, clientCertFile, clientKeyFile, principalDir, authJWKS.URL, "gateway-next")
	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != gatewayCompleteJWKSPath || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		response, err := http.Get("http://" + gatewayCurrentAddress + gatewayCompleteJWKSPath)
		if err != nil {
			http.Error(w, "fixture JWKS unavailable", http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	proxy.TLS = &tls.Config{Certificates: []tls.Certificate{mustLoadMatchSquadTestPair(t, jwksCert, jwksKey)}}
	proxy.StartTLS()
	t.Cleanup(proxy.Close)

	principalConfig := spaceprincipal.Config{
		ListenAddr: "127.0.0.1:0", TLSCertFile: mmCertFile, TLSKeyFile: mmKeyFile,
		ClientCAFile: caFile, JWKSCAFile: caFile, GatewayJWKSURL: proxy.URL + gatewayCompleteJWKSPath,
		ReplayAddr: redisAddr, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute,
		UnknownKIDCooldown: time.Second,
	}
	mmPrincipalRuntime, err := spaceprincipal.New(ctx, principalConfig)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mmPrincipalRuntime.Close() })
	mmGRPC := grpc.NewServer(mmPrincipalRuntime.ServerOptions()...)
	matchmakingService := &grpcsvc.MatchmakingGRPC{Matches: &store.MatchStore{Pool: pool}}
	dropReplyService := &dropFirstCompleteMatchReply{MatchmakingServiceServer: matchmakingService}
	matchmakingv1.RegisterMatchmakingServiceServer(mmGRPC, dropReplyService)
	mmServeErr := make(chan error, 1)
	go func() { mmServeErr <- mmGRPC.Serve(mmListener) }()
	t.Cleanup(func() {
		mmGRPC.Stop()
		select {
		case <-mmServeErr:
		case <-time.After(2 * time.Second):
			t.Error("protected Matchmaking listener did not stop")
		}
	})
	ordinaryListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ordinaryGRPC := grpc.NewServer(grpc.ChainUnaryInterceptor(spaceprincipal.OrdinaryUnaryInterceptor()))
	matchmakingv1.RegisterMatchmakingServiceServer(ordinaryGRPC, matchmakingService)
	ordinaryServeErr := make(chan error, 1)
	go func() { ordinaryServeErr <- ordinaryGRPC.Serve(ordinaryListener) }()
	t.Cleanup(func() {
		ordinaryGRPC.Stop()
		select {
		case <-ordinaryServeErr:
		case <-time.After(2 * time.Second):
			t.Error("ordinary Matchmaking listener did not stop")
		}
	})

	operationID := uuid.NewString()
	client := &http.Client{Timeout: 10 * time.Second}
	request := func(target, targetMatch string, epoch int64, profileID uuid.UUID, operation string, expires time.Time) (*http.Response, error) {
		t.Helper()
		body, err := json.Marshal(map[string]string{"operation_id": operation})
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+target+"/api/v1/matchmaking/matches/"+targetMatch+"/complete", strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+signMatchSquadUserJWTUntil(t, authKey, accountID, profileID.String(), epoch, expires))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Profile-ID", profileB.String()) // Must never replace the verified actor.
		return client.Do(req)
	}

	stale, err := request(gatewayCurrentAddress, matchID.String(), 1, profileA, operationID, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	stale.Body.Close()
	require.Equal(t, http.StatusUnauthorized, stale.StatusCode)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileA, false)

	// Missing and malformed session floors are fail-closed before MM is called.
	require.NoError(t, redisClient.Del(ctx, "auth:session:min_epoch:"+accountID.String()).Err())
	missingFloor, err := request(gatewayCurrentAddress, matchID.String(), 1, profileA, operationID, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	missingFloor.Body.Close()
	require.NotEqual(t, http.StatusOK, missingFloor.StatusCode)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileA, false)
	require.NoError(t, redisClient.Set(ctx, "auth:session:min_epoch:"+accountID.String(), "not-an-epoch", 0).Err())
	corruptFloor, err := request(gatewayCurrentAddress, matchID.String(), 1, profileA, operationID, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	corruptFloor.Body.Close()
	require.NotEqual(t, http.StatusOK, corruptFloor.StatusCode)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileA, false)
	require.NoError(t, redisClient.Set(ctx, "auth:session:min_epoch:"+accountID.String(), "1", 0).Err())

	expired, err := request(gatewayCurrentAddress, matchID.String(), 1, profileA, operationID, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	expired.Body.Close()
	require.NotEqual(t, http.StatusOK, expired.StatusCode)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileA, false)

	badBodyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+gatewayCurrentAddress+"/api/v1/matchmaking/matches/"+matchID.String()+"/complete", strings.NewReader(`{"operation_id":`))
	require.NoError(t, err)
	badBodyReq.Header.Set("Authorization", "Bearer "+signMatchSquadUserJWT(t, authKey, accountID, profileA.String(), 1))
	badBodyReq.Header.Set("Content-Type", "application/json")
	badBody, err := client.Do(badBodyReq)
	require.NoError(t, err)
	badBody.Body.Close()
	require.Equal(t, http.StatusBadRequest, badBody.StatusCode)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileA, false)

	// The MM commit succeeds, but its first RPC reply is intentionally lost.
	// A second Gateway instance with the rotated active key retries the same
	// actor operation against shared replay and operation stores.
	first, err := request(gatewayCurrentAddress, matchID.String(), 1, profileA, operationID, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	first.Body.Close()
	require.NotEqual(t, http.StatusOK, first.StatusCode)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileA, true)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileB, false)

	retry, err := request(gatewayNextAddress, matchID.String(), 1, profileA, operationID, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	retryBody, err := io.ReadAll(retry.Body)
	require.NoError(t, err)
	retry.Body.Close()
	require.Equalf(t, http.StatusOK, retry.StatusCode, "same actor/operation retry returned HTTP %d", retry.StatusCode)
	replay, err := request(gatewayCurrentAddress, matchID.String(), 1, profileA, operationID, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	replayBody, err := io.ReadAll(replay.Body)
	require.NoError(t, err)
	replay.Body.Close()
	require.Equal(http.StatusOK, replay.StatusCode)
	require.JSONEq(t, string(retryBody), string(replayBody), "cross-Gateway replay must return the persisted actor outcome")
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileA, true)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileB, false)

	// A different actor is authorized independently only if it is a participant.
	wrongActor, err := request(gatewayNextAddress, matchID.String(), 1, uuid.New(), operationID, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	wrongActor.Body.Close()
	require.NotEqual(t, http.StatusOK, wrongActor.StatusCode)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileB, false)

	// Reusing the actor-scoped operation key with a different request binding
	// (another match ID) conflicts without changing that match.
	profileC := uuid.New()
	otherMatchID := seedCompleteMatchFixture(t, ctx, pool, profileA, profileC)
	rebound, err := request(gatewayNextAddress, otherMatchID.String(), 1, profileA, operationID, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	rebound.Body.Close()
	require.NotEqual(t, http.StatusOK, rebound.StatusCode)
	assertCompleteMatchParticipantState(t, ctx, pool, otherMatchID, profileA, false)

	// The valid client JWT is the wrong principal kind for this listener, and
	// the ordinary listener must reject the protected method before the handler.
	clientPair, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	protectedConn, err := grpc.DialContext(ctx, mmAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: roots, ServerName: "matchmaking.fixture", Certificates: []tls.Certificate{clientPair}})),
		grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { _ = protectedConn.Close() })
	grpcClient := matchmakingv1.NewMatchmakingServiceClient(protectedConn)
	wrongKindCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+signMatchSquadUserJWT(t, authKey, accountID, profileB.String(), 1), "x-request-id", uuid.NewString())
	_, err = grpcClient.CompleteMatch(wrongKindCtx, &matchmakingv1.CompleteMatchRequest{MatchId: matchID.String(), OperationId: uuid.NewString()})
	require.Error(t, err)
	wrongMethodCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+signMatchSquadUserJWT(t, authKey, accountID, profileB.String(), 1), "x-request-id", uuid.NewString())
	_, err = grpcClient.ListGames(wrongMethodCtx, &matchmakingv1.ListGamesRequest{})
	require.Error(t, err)
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileB, false)

	ordinaryConn, err := grpc.DialContext(ctx, ordinaryListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { _ = ordinaryConn.Close() })
	_, err = matchmakingv1.NewMatchmakingServiceClient(ordinaryConn).CompleteMatch(ctx, &matchmakingv1.CompleteMatchRequest{MatchId: matchID.String(), OperationId: uuid.NewString()})
	require.Equal(t, codes.Unavailable, status.Code(err))
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileB, false)

	// A stalled Redis floor read times out before the protected service is
	// reached. Use a separate Gateway with an accept-and-stall Redis endpoint.
	stalledRedisAddr := startStalledMatchSquadRedis(t)
	stalledGateway := startMatchSquadGateway(t, ctx, gatewayBinary, reserveMatchSquadTCPPort(t), mmAddress, stalledRedisAddr, caFile, clientCertFile, clientKeyFile, principalDir, authJWKS.URL, "gateway-next")
	shortClient := &http.Client{Timeout: 2 * time.Second}
	stalledReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+stalledGateway+"/api/v1/matchmaking/matches/"+matchID.String()+"/complete", strings.NewReader(`{"operation_id":"`+uuid.NewString()+`"}`))
	require.NoError(t, err)
	stalledReq.Header.Set("Authorization", "Bearer "+signMatchSquadUserJWT(t, authKey, accountID, profileB.String(), 1))
	stalledReq.Header.Set("Content-Type", "application/json")
	stalledResponse, stalledErr := shortClient.Do(stalledReq)
	if stalledResponse != nil {
		stalledResponse.Body.Close()
	}
	require.Error(t, stalledErr, "stalled session floor must not authorize the request")
	assertCompleteMatchParticipantState(t, ctx, pool, matchID, profileB, false)

	select {
	case err := <-mmServeErr:
		require.NoError(t, err)
	default:
	}
}

func startMatchSquadRedis(t *testing.T, ctx context.Context) string {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"},
			WaitingFor: wait.ForListeningPort("6379/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := container.Terminate(cleanupCtx); err != nil {
			t.Errorf("terminate MatchSquad Redis fixture: %v", err)
		}
	})
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	return net.JoinHostPort(host, port.Port())
}

func startStalledMatchSquadRedis(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(io.Discard, conn)
			}()
		}
	}()
	return listener.Addr().String()
}

func seedCompleteMatchFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, profileA, profileB uuid.UUID) uuid.UUID {
	t.Helper()
	games, err := (&store.GameStore{Pool: pool}).List(ctx, store.ListGamesParams{PageSize: 1, Status: store.StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, games.Games)
	sessions := &store.SessionStore{Pool: pool}
	matchStore := &store.MatchStore{Pool: pool}
	deadline := time.Now().UTC().Add(30 * time.Minute)
	criteria := `{"region":"eu"}`
	sessionA, err := sessions.Create(ctx, store.CreateSessionParams{ProfileID: profileA, GameID: games.Games[0].ID, Mode: "Duo", Criteria: criteria, TimeoutAt: deadline})
	require.NoError(t, err)
	sessionB, err := sessions.Create(ctx, store.CreateSessionParams{ProfileID: profileB, GameID: games.Games[0].ID, Mode: "Duo", Criteria: criteria, TimeoutAt: deadline})
	require.NoError(t, err)
	proposal, err := matchStore.CreateProposal(ctx, store.CreateProposalParams{GameID: games.Games[0].ID, Mode: "Duo", Region: "eu", Sessions: []store.ProposalSession{{SessionID: sessionA.ID, ProfileID: profileA}, {SessionID: sessionB.ID, ProfileID: profileB}}})
	require.NoError(t, err)
	_, err = matchStore.SetProposalResponse(ctx, proposal.Match.ID, profileA, store.ProposalResponseAccepted)
	require.NoError(t, err)
	_, err = matchStore.SetProposalResponse(ctx, proposal.Match.ID, profileB, store.ProposalResponseAccepted)
	require.NoError(t, err)
	active, err := matchStore.ActivateMatch(ctx, proposal.Match.ID, "fixture-voice", "fixture-chat")
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusActive, active.Status)
	return proposal.Match.ID
}

func assertCompleteMatchParticipantState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, matchID, profileID uuid.UUID, wantLeft bool) {
	t.Helper()
	match, err := (&store.MatchStore{Pool: pool}).Get(ctx, matchID)
	require.NoError(t, err)
	require.Equal(t, wantLeft, match.HasLeft(profileID))
}

type dropFirstCompleteMatchReply struct {
	matchmakingv1.MatchmakingServiceServer
	once sync.Once
}

func (s *dropFirstCompleteMatchReply) CompleteMatch(ctx context.Context, req *matchmakingv1.CompleteMatchRequest) (*matchmakingv1.CompleteMatchResponse, error) {
	response, err := s.MatchmakingServiceServer.CompleteMatch(ctx, req)
	if err != nil {
		return response, err
	}
	first := false
	s.once.Do(func() { first = true })
	if first {
		return nil, status.Error(codes.Unavailable, "fixture dropped the committed CompleteMatch reply")
	}
	return response, nil
}

func buildMatchSquadGateway(t *testing.T, ctx context.Context, fixtureDir string) string {
	t.Helper()
	projectRoot, err := matchmakingRepoRoot()
	require.NoError(t, err)
	binary := filepath.Join(fixtureDir, "gateway-fixture")
	buildCtx, cancelBuild := context.WithTimeout(ctx, 90*time.Second)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(projectRoot, "src", "backend", "gateway")
	build.Stdout, build.Stderr = io.Discard, io.Discard
	require.NoError(t, build.Run(), "build the actual Gateway module for the private transport fixture")
	return binary
}

func startMatchSquadGateway(t *testing.T, ctx context.Context, binary string, gatewayPort int, mmAddress, redisAddr, caFile, clientCertFile, clientKeyFile, principalDir, authJWKSURL, activeKID string) string {
	t.Helper()
	projectRoot, err := matchmakingRepoRoot()
	require.NoError(t, err)

	listenAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(gatewayPort))
	cmd := exec.CommandContext(ctx, binary)
	cmd.Dir = filepath.Join(projectRoot, "src", "backend", "gateway")
	cmd.Env = matchSquadGatewayEnvironment(os.Environ(), map[string]string{
		"LISTEN_ADDR":                                   listenAddr,
		"GATEWAY_JWKS_URL":                              authJWKSURL,
		"GATEWAY_JWT_ISSUER":                            "match-squad-auth-fixture",
		"GATEWAY_JWT_AUDIENCE":                          "voice-client",
		"GATEWAY_SESSION_EPOCH_STRICT":                  "true",
		"GATEWAY_REDIS_ADDR":                            redisAddr,
		"GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR":            principalDir,
		"GATEWAY_PRINCIPAL_ACTIVE_KID":                  activeKID,
		"GATEWAY_MATCHMAKING_COMPLETE_GRPC_ADDR":        mmAddress,
		"GATEWAY_MATCHMAKING_COMPLETE_TLS_CA_FILE":      caFile,
		"GATEWAY_MATCHMAKING_COMPLETE_TLS_SERVER_NAME":  "matchmaking.fixture",
		"GATEWAY_MATCHMAKING_COMPLETE_CLIENT_CERT_FILE": clientCertFile,
		"GATEWAY_MATCHMAKING_COMPLETE_CLIENT_KEY_FILE":  clientKeyFile,
	})
	if err := cmd.Start(); err != nil {
		t.Fatalf("start actual Gateway process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	deadline := time.Now().Add(25 * time.Second)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		response, err := client.Get("http://" + listenAddr + "/health")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return listenAddr
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("fixture context ended while starting Gateway")
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Fatal("actual Gateway did not become healthy")
	return ""
}

func matchSquadGatewayEnvironment(existing []string, values map[string]string) []string {
	filtered := make([]string, 0, len(existing)+len(values))
	for _, entry := range existing {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GATEWAY_") || strings.HasPrefix(name, "MATCHMAKING_") || strings.HasPrefix(name, "S2S_") || name == "DATABASE_URL" || name == "LISTEN_ADDR" {
			continue
		}
		filtered = append(filtered, entry)
	}
	for name, value := range values {
		if value != "" {
			filtered = append(filtered, name+"="+value)
		}
	}
	return filtered
}

func matchmakingRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("locate Matchmaking transport fixture")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..")), nil
}

func reserveMatchSquadTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func newMatchSquadTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "MatchSquad private test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func newMatchSquadTestLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, name string, ips []net.IP, usage x509.ExtKeyUsage) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name}, IPAddresses: ips, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

func writeMatchSquadTestPair(t *testing.T, dir, stem string, cert *x509.Certificate, key *ecdsa.PrivateKey) (string, string) {
	t.Helper()
	certFile, keyFile := filepath.Join(dir, stem+".crt"), filepath.Join(dir, stem+".key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}), 0o600))
	return certFile, keyFile
}

func mustLoadMatchSquadTestPair(t *testing.T, cert *x509.Certificate, key *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair
}

func writeMatchSquadRSAPrivateKey(t *testing.T, path string, key *rsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))
}

func matchSquadJWKSHandler(kid string, key *rsa.PublicKey) http.Handler {
	document := struct {
		Keys []map[string]string `json:"keys"`
	}{Keys: []map[string]string{{
		"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
	encoded, _ := json.Marshal(document)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(encoded)
	})
}

func signMatchSquadUserJWT(t *testing.T, key *rsa.PrivateKey, accountID uuid.UUID, profileID string, epoch int64) string {
	return signMatchSquadUserJWTUntil(t, key, accountID, profileID, epoch, time.Now().Add(5*time.Minute))
}

func signMatchSquadUserJWTUntil(t *testing.T, key *rsa.PrivateKey, accountID uuid.UUID, profileID string, epoch int64, expires time.Time) string {
	t.Helper()
	claims := map[string]any{
		"iss": "match-squad-auth-fixture", "aud": "voice-client", "sub": accountID.String(),
		"user_id": accountID.String(), "profile_id": profileID, "account_type": "regular",
		"session_epoch": epoch, "jti": uuid.NewString(), "exp": expires.Unix(),
	}
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "auth-test", "typ": "JWT"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
