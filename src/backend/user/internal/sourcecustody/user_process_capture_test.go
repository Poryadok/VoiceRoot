package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	identityv1 "voice.app/voice/identityerasure/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
)

// This runs the actual User executable and its protected capture listener
// against real TLS PostgreSQL and a signed custody stream. Auth's transport is
// a fixture, so it is not evidence of a complete deployed Ban or Auth custody.
func TestUserProcessServesQualifiedSpaceCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable TLS PostgreSQL and a User process")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	integrationtest.ConfigureDockerTesting()
	_, control := recoveryTLSDatabase(t, ctx, "user-process-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "user-process-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, err := CreateClosedSource(ctx, control, admin, request)
	require.NoError(t, err)
	migrations, keys := migrationFixture(t)
	require.NoError(t, InitializeClosedSource(ctx, control, admin, request, physical, migrations, keys))
	_, err = admin.Exec(ctx, `CREATE ROLE custody_app NOLOGIN PASSWORD 'fixture_only'; GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO custody_app`)
	require.NoError(t, err)
	isolateWriterDatabases(t, admin, physical.Database)
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	sourceURL, err := url.Parse(adminURL)
	require.NoError(t, err)
	sourceURL.Path = "/" + physical.Database
	source, err := openPinnedPostgres(ctx, sourceURL.String())
	require.NoError(t, err)
	_, err = source.Exec(ctx, `GRANT USAGE ON SCHEMA public TO custody_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO custody_app;
GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO custody_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO custody_app;
GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO custody_app`)
	require.NoError(t, err)
	profile, account, actorProfile, actorAccount := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if os.Getenv("VOICE_USER_CAPTURE_TEST_SERVE") == "1" {
		actorProfile, err = uuid.Parse(os.Getenv("VOICE_CAPTURE_ACTOR_PROFILE"))
		require.NoError(t, err)
		actorAccount, err = uuid.Parse(os.Getenv("VOICE_CAPTURE_ACTOR_ACCOUNT"))
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, actorProfile)
		require.NotEqual(t, uuid.Nil, actorAccount)
	}
	_, err = source.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'captureprocess','1234','Capture process')`, profile, account)
	require.NoError(t, err)
	_, err = source.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'captureactor','1234','Capture actor')`, actorProfile, actorAccount)
	require.NoError(t, err)
	source.Close()
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	source, err = openPinnedPostgres(ctx, sourceURL.String())
	require.NoError(t, err)
	defer source.Close()
	var binding Binding
	var digest []byte
	require.NoError(t, control.QueryRow(ctx, `SELECT epoch,source_id,history_generation,user_source_generation,keyset_digest FROM user_custody_state WHERE singleton`).Scan(&binding.Epoch, &binding.SourceID, &binding.HistoryGeneration, &binding.UserSourceGeneration, &digest))
	copy(binding.KeysetDigest[:], digest)
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	defer func() { _ = controller.Shutdown(context.Background()) }()
	challenge, err := controller.ActivationChallenge()
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var controlSystem, controlDatabase string
	var controlOID uint32
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&controlSystem, &controlDatabase, &controlOID))
	now := time.Now()
	claim := OpenAttestation{Challenge: challenge, ControlSystem: controlSystem, ControlDatabase: controlDatabase, ControlDatabaseOID: controlOID, Binding: binding, Physical: physical, WriterRole: "custody_app", SourceEvidenceDigest: [32]byte{3}, WriterManifestDigest: [32]byte{1}, AuthEvidenceDigest: [32]byte{2}, GuardCutover: true, IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(4 * time.Minute).Unix()}
	claim, err = SignOpenAttestation(claim, private)
	require.NoError(t, err)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.ActivateQualifiedGuards(ctx, admin, source, keys, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))

	dir := t.TempDir()
	pki := processTLS(t, dir, "user-process")
	clientCertPath, clientKeyPath := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client.key")
	require.NoError(t, os.WriteFile(clientCertPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pki.client.Certificate[0]}), 0600))
	clientDER, err := x509.MarshalPKCS8PrivateKey(pki.client.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(clientKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientDER}), 0600))
	serverCert, err := tls.LoadX509KeyPair(pki.serverCert, pki.serverKey)
	require.NoError(t, err)
	reserve := func() string {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, e)
		address := l.Addr().String()
		require.NoError(t, l.Close())
		return address
	}
	userJWKSAddress, captureAddress, ordinaryAddress, httpAddress := reserve(), reserve(), reserve(), reserve()
	userKeyDir := filepath.Join(dir, "user-signers")
	require.NoError(t, os.Mkdir(userKeyDir, 0700))
	for _, id := range []string{"current", "next"} {
		key, e := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, e)
		encoded, e := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, e)
		require.NoError(t, os.WriteFile(filepath.Join(userKeyDir, id+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600))
	}
	keyDir := filepath.Join(dir, "identity-keys")
	require.NoError(t, os.Mkdir(keyDir, 0700))
	identityKey := make([]byte, 32)
	for i := range identityKey {
		identityKey[i] = 0x71
	}
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "v1.key"), identityKey, 0600))
	fingerprint := sha256.Sum256(identityKey)
	manifest, err := json.Marshal(map[string]any{"purpose": "voice.user.identity-association.v1", "revision": 1, "active_version": 1, "versions": []any{map[string]any{"version": 1, "sha256": fmt.Sprintf("%x", fingerprint), "file": "v1.key"}}})
	require.NoError(t, err)
	manifestPath := filepath.Join(dir, "identity-manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, manifest, 0600))

	// A real mTLS stream server verifies each User signature against User's
	// running JWKS. Replay storage here is an in-memory test fixture.
	jwksHTTP := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots, ServerName: "localhost"}}, Timeout: 2 * time.Second}
	defer jwksHTTP.CloseIdleConnections()
	resolver := principal.NewJWKSResolver(func(fetchCtx context.Context, issuer string) ([]byte, error) {
		if issuer != "user" {
			return nil, ErrUnavailable
		}
		req, e := http.NewRequestWithContext(fetchCtx, http.MethodGet, "https://"+userJWKSAddress+"/.well-known/jwks.json", nil)
		if e != nil {
			return nil, e
		}
		response, e := jwksHTTP.Do(req)
		if e != nil {
			return nil, e
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, ErrUnavailable
		}
		return io.ReadAll(io.LimitReader(response.Body, 65537))
	})
	var replayMu sync.Mutex
	seen := map[string]bool{}
	stream, err := NewStreamServer(controller, StreamConfig{KeyResolver: resolver.Resolve, ReplayGuard: func(_ context.Context, issuer, id string, _ time.Time) error {
		replayMu.Lock()
		defer replayMu.Unlock()
		key := issuer + ":" + id
		if seen[key] {
			return ErrUnavailable
		}
		seen[key] = true
		return nil
	}, Heartbeat: 100 * time.Millisecond, RequireAttested: true})
	require.NoError(t, err)
	custodyListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	custodyServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})))
	userv1.RegisterUserSourceCustodyServiceServer(custodyServer, stream)
	go func() { _ = custodyServer.Serve(custodyListener) }()
	defer custodyServer.Stop()
	defer custodyListener.Close()

	// The protected Auth fixture verifies the real User signature and request
	// binding. Its returned parent facts do not assert Auth source continuity.
	authListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	authVerifier := func(callCtx context.Context, request any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		message, ok := request.(proto.Message)
		meta, metaErr := principal.IncomingMetadata(callCtx)
		if !ok || metaErr != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid fixture principal")
		}
		hash, hashErr := principal.RequestHash(message)
		if hashErr != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid fixture principal")
		}
		verified, verifyErr := principal.VerifyService(callCtx, meta.BearerToken, principal.VerifyConfig{ExpectedIssuer: "user", ExpectedAudience: "auth", ExpectedRPC: info.FullMethod, ExpectedRequestID: meta.RequestID, ExpectedRequestHash: hash, KeyResolver: resolver.Resolve})
		if verifyErr != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid fixture principal")
		}
		return next(principal.WithVerified(callCtx, verified), request)
	}
	authServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})), grpc.UnaryInterceptor(authVerifier))
	identityv1.RegisterAuthUserIdentityAuthorityServiceServer(authServer, &userAuthContractServer{})
	go func() { _ = authServer.Serve(authListener) }()
	defer authServer.Stop()
	defer authListener.Close()
	ordinaryAuthListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ordinaryAuth := grpc.NewServer()
	go func() { _ = ordinaryAuth.Serve(ordinaryAuthListener) }()
	defer ordinaryAuth.Stop()
	defer ordinaryAuthListener.Close()

	spaceKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	spaceNext, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(id string, key *rsa.PublicKey) map[string]string {
		return map[string]string{"kty": "RSA", "kid": id, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
	}
	spaceJWKS, err := json.Marshal(map[string]any{"keys": []any{jwk("current", &spaceKey.PublicKey), jwk("next", &spaceNext.PublicKey)}})
	require.NoError(t, err)
	spaceJWKSService := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(spaceJWKS) }))
	defer spaceJWKSService.Close()
	spaceCA := filepath.Join(dir, "space-jwks-ca.pem")
	require.NoError(t, os.WriteFile(spaceCA, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: spaceJWKSService.Certificate().Raw}), 0600))
	replay := miniredis.RunT(t)
	appURL := *sourceURL
	appURL.User = url.UserPassword("custody_app", "fixture_only")
	values := map[string]string{
		"VOICE_RELEASE_PROFILE": "development", "DATABASE_URL": appURL.String(), "AUTH_GRPC_ADDR": ordinaryAuthListener.Addr().String(), "USER_GRPC_ADDR": ordinaryAddress, "LISTEN_ADDR": httpAddress, "USER_REDIS_ADDR": replay.Addr(),
		"USER_CUSTODY_ADDR": custodyListener.Addr().String(), "USER_CUSTODY_TLS_CA_FILE": pki.caFile, "USER_CUSTODY_TLS_SERVER_NAME": "localhost", "USER_CUSTODY_TLS_CERT_FILE": clientCertPath, "USER_CUSTODY_TLS_KEY_FILE": clientKeyPath, "USER_CUSTODY_SIGNING_KEYS_DIR": userKeyDir, "USER_CUSTODY_ACTIVE_KID": "current", "USER_CUSTODY_HEARTBEAT_TIMEOUT": "1s",
		"USER_CUSTODY_EPOCH": "1", "USER_CUSTODY_SOURCE_ID": binding.SourceID, "USER_CUSTODY_HISTORY_GENERATION": binding.HistoryGeneration.String(), "USER_CUSTODY_USER_SOURCE_GENERATION": binding.UserSourceGeneration.String(), "USER_CUSTODY_KEYSET_DIGEST": fmt.Sprintf("%x", binding.KeysetDigest), "USER_IDENTITY_KEY_MANIFEST": manifestPath, "USER_IDENTITY_KEYS_DIR": keyDir,
		"USER_PRINCIPAL_JWKS_LISTEN": userJWKSAddress, "USER_PRINCIPAL_JWKS_TLS_CERT_FILE": pki.serverCert, "USER_PRINCIPAL_JWKS_TLS_KEY_FILE": pki.serverKey,
		"USER_IDENTITY_AUTH_GRPC_ADDR": authListener.Addr().String(), "USER_IDENTITY_AUTH_TLS_SERVER_NAME": "localhost", "USER_IDENTITY_AUTH_TLS_CA_FILE": pki.caFile, "USER_IDENTITY_AUTH_TLS_CERT_FILE": clientCertPath, "USER_IDENTITY_AUTH_TLS_KEY_FILE": clientKeyPath,
		"USER_SPACE_CAPTURE_PRINCIPAL_GRPC_LISTEN": captureAddress, "USER_SPACE_CAPTURE_PRINCIPAL_TLS_CERT_FILE": pki.serverCert, "USER_SPACE_CAPTURE_PRINCIPAL_TLS_KEY_FILE": pki.serverKey, "USER_SPACE_CAPTURE_PRINCIPAL_CLIENT_CA_FILE": pki.caFile, "USER_SPACE_CAPTURE_PRINCIPAL_REPLAY_REDIS_ADDR": replay.Addr(), "USER_SPACE_CAPTURE_PRINCIPAL_JWKS_CA_FILE": spaceCA, "USER_SPACE_CAPTURE_PRINCIPAL_JWKS_CLIENT_CERT_FILE": clientCertPath, "USER_SPACE_CAPTURE_PRINCIPAL_JWKS_CLIENT_KEY_FILE": clientKeyPath,
		"S2S_JWKS_URLS_JSON": fmt.Sprintf(`{"space":%q}`, spaceJWKSService.URL),
	}
	binary := filepath.Join(dir, "user.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../..")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)
	command := exec.CommandContext(ctx, binary)
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(strings.ToUpper(name), "PG") || strings.HasPrefix(name, "USER_") || strings.HasPrefix(name, "S2S_") || name == "DATABASE_URL" || name == "AUTH_GRPC_ADDR" || name == "SPACE_GRPC_ADDR" || name == "VOICE_RELEASE_PROFILE" || name == "LISTEN_ADDR" || name == "NATS_URL" {
			continue
		}
		command.Env = append(command.Env, item)
	}
	for name, value := range values {
		command.Env = append(command.Env, name+"="+value)
	}
	logFile, err := os.Create(filepath.Join(dir, "user.log"))
	require.NoError(t, err)
	defer logFile.Close()
	command.Stdout, command.Stderr = logFile, logFile
	require.NoError(t, command.Start())
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	joined := false
	defer func() {
		if !joined {
			_ = command.Process.Kill()
			<-done
		}
	}()
	connection, err := grpc.NewClient(captureAddress, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots, ServerName: "localhost", Certificates: []tls.Certificate{pki.client}})))
	require.NoError(t, err)
	defer connection.Close()
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: spaceKey})
	require.NoError(t, err)
	rpc := userv1.NewUserServiceClient(connection)
	requestCapture := &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: uuid.NewString(), SpaceId: uuid.NewString(), Purpose: 2, Cohort: &userv1.SpaceAccountCaptureCohort{Entries: []*userv1.SpaceAccountCaptureEntry{{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{PublicationId: uuid.NewString()}}}}}}
	requestCapture.CohortDigest = spaceaccountcapture.CohortDigest(requestCapture.Cohort)
	call := func() (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		id := uuid.NewString()
		hash, e := principal.RequestHash(requestCapture)
		if e != nil {
			return nil, e
		}
		token, e := spaceIssuer.IssueService(principal.ServiceInput{Audience: "user", RPC: userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName, RequestID: id, RequestHash: hash})
		if e != nil {
			return nil, e
		}
		callCtx, stop := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 3*time.Second)
		defer stop()
		return rpc.CaptureSpaceMemberAccountBindings(callCtx, requestCapture)
	}
	var response *userv1.CaptureSpaceMemberAccountBindingsResponse
	var callErr error
	require.Eventually(t, func() bool {
		select {
		case <-done:
			joined = true
			return true
		default:
		}
		response, callErr = call()
		return callErr == nil
	}, 45*time.Second, 250*time.Millisecond, "User did not admit capture; last error: %v", callErr)
	if joined {
		_ = logFile.Sync()
		output, _ := os.ReadFile(logFile.Name())
		t.Fatalf("User exited before admitting capture: %s", output)
	}
	require.NoError(t, spaceaccountcapture.ValidateResponse(requestCapture, response))
	require.Len(t, response.Results, 1)
	require.Equal(t, account.String(), response.Results[0].AccountId)
	require.Equal(t, userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED, response.Results[0].Outcome)
	ordinaryConnection, err := grpc.NewClient(ordinaryAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer ordinaryConnection.Close()
	birthCtx, stopBirth := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-user-id", account.String(), "x-voice-subscription-tier", "premium")), 5*time.Second)
	created, err := userv1.NewUserServiceClient(ordinaryConnection).CreateProfile(birthCtx, &userv1.CreateProfileRequest{DisplayName: "Guarded birth"})
	stopBirth()
	require.NoError(t, err, "User's actual guarded writer must call the protected Auth fixture")
	birthProfile, err := uuid.Parse(created.GetProfile().GetId())
	require.NoError(t, err)
	var origin string
	require.NoError(t, source.QueryRow(ctx, `SELECT a.capture_origin FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, birthProfile).Scan(&origin))
	require.Equal(t, "BIRTH", origin)
	requestCapture.OperationId = uuid.NewString()
	requestCapture.Cohort.Entries[0].ProfileId = birthProfile.String()
	requestCapture.CohortDigest = spaceaccountcapture.CohortDigest(requestCapture.Cohort)
	response, err = call()
	require.NoError(t, err)
	require.Equal(t, account.String(), response.Results[0].AccountId)
	require.Equal(t, userv1.SpaceAccountAssociationOrigin_SPACE_ACCOUNT_ASSOCIATION_ORIGIN_BIRTH, response.Results[0].AssociationOrigin)
	primaryAccount, primaryProfile := uuid.New(), uuid.NewString()
	primaryCtx, stopPrimary := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "auth")), 5*time.Second)
	primary, err := userv1.NewUserServiceClient(ordinaryConnection).EnsurePrimaryProfile(primaryCtx, &userv1.EnsurePrimaryProfileRequest{AccountId: primaryAccount.String(), ProfileId: &primaryProfile, DisplayHint: "primarybirth"})
	stopPrimary()
	require.NoError(t, err, "the actual primary-profile writer must also use guarded birth")
	require.Equal(t, primaryProfile, primary.GetProfile().GetId())
	require.NoError(t, source.QueryRow(ctx, `SELECT a.capture_origin FROM profiles p JOIN identity_associations a ON a.association_id=p.identity_association_id WHERE p.id=$1`, primaryProfile).Scan(&origin))
	require.Equal(t, "BIRTH", origin)
	if os.Getenv("VOICE_USER_CAPTURE_TEST_SERVE") == "1" {
		spaceDER, err := x509.MarshalPKCS8PrivateKey(spaceKey)
		require.NoError(t, err)
		spaceKeyPath := filepath.Join(dir, "space-signing-key.pem")
		require.NoError(t, os.WriteFile(spaceKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: spaceDER}), 0600))
		fixture, err := json.Marshal(map[string]string{"address": captureAddress, "ca": pki.caFile, "cert": clientCertPath, "key": clientKeyPath, "space_signer": spaceKeyPath, "profile": profile.String(), "birth_profile": birthProfile.String(), "account": account.String(), "actor_profile": actorProfile.String(), "actor_account": actorAccount.String()})
		require.NoError(t, err)
		fmt.Printf("VOICE_CAPTURE_READY %s\n", fixture)
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}
	require.NoError(t, controller.CloseAdmission(ctx))
	_, err = call()
	require.Error(t, err, "capture must fail after custody closes")
}
