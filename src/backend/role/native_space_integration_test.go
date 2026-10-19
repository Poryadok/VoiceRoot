//go:build role_space_native

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/memberauthority"
	"voice/backend/role/internal/principalruntime"
)

type nativeSpaceReply struct {
	SpaceID, Address string
	Binding          *memberadmission.EnsureBinding
	Published        bool
}

type nativeFixtureLog struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *nativeFixtureLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}
func (b *nativeFixtureLog) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

type nativeTrackedObserver struct {
	client *memberauthority.Client
	calls  atomic.Int64
}

func (o *nativeTrackedObserver) ObserveMember(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (memberauthority.Observation, error) {
	o.calls.Add(1)
	return o.client.ObserveMember(ctx, req)
}

func nativeServiceContext(t *testing.T, key *rsa.PrivateKey, method string, request proto.Message) context.Context {
	return nativeWorkloadContext(t, key, "space", method, request)
}

func nativeWorkloadContext(t *testing.T, key *rsa.PrivateKey, issuer, method string, request proto.Message) context.Context {
	t.Helper()
	transport := sourceCallContext(t, key, issuer, method, uuid.NewString(), request)
	md, ok := metadata.FromOutgoingContext(transport)
	require.True(t, ok)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return metadata.NewOutgoingContext(ctx, md)
}

func TestMemberAdmissionPositiveProtectedReceipt(t *testing.T) {
	runNativeMemberProtocol(t, false)
}

// This is the deliberately still-red full interactive cutover oracle. The
// receipt producer's passing test must not be relabeled as this completion.
func TestNativeAssignmentNeedsQualifiedOperationPublication(t *testing.T) {
	runNativeMemberProtocol(t, true)
}

func runNativeMemberProtocol(t *testing.T, requireInteractive bool) {
	ctx, roles := admissionListenerStore(t)
	build := exec.CommandContext(ctx, "sh", "testdata/build_native_space_fixture.sh")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, "/tmp/role-native-space-migrations")
	fixture.Run(true, "goto", "34")

	// Real Redis serves both runtime replay guards; no in-memory replay stand.
	redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		require.NoError(t, redis.Terminate(cleanup))
	})
	redisHost, err := redis.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redis.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	redisAddress := redisHost + ":" + redisPort.Port()

	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
	}
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk("current", current), jwk("next", next)}}))
	}))
	t.Cleanup(jwks.Close)
	dir := t.TempDir()
	certFile, keyFile, clientCA := filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key"), filepath.Join(dir, "client.crt")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.TLS.Certificates[0].Certificate[0]})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(jwks.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(183), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &clientKey.PublicKey, clientKey)
	require.NoError(t, err)
	clientPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(clientCA, clientPEM, 0600))
	clientDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	require.NoError(t, err)
	clientPair, err := tls.X509KeyPair(clientPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientDER}))
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	transport := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{clientPair}})

	process := exec.CommandContext(ctx, "/tmp/role-native-space-fixture")
	stdin, err := process.StdinPipe()
	require.NoError(t, err)
	stdout, err := process.StdoutPipe()
	require.NoError(t, err)
	var stderr nativeFixtureLog
	process.Stderr = &stderr
	require.NoError(t, process.Start())
	encoder, decoder := json.NewEncoder(stdin), json.NewDecoder(stdout)
	t.Cleanup(func() {
		_ = encoder.Encode(map[string]string{"Action": "stop"})
		_ = stdin.Close()
		require.NoError(t, process.Wait(), stderr.String())
	})
	owner, profile, account, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, encoder.Encode(map[string]any{"DSN": fixture.Pool.Config().ConnString(), "Owner": owner, "Profile": profile, "Account": account, "Operation": operation, "Config": map[string]any{"ListenAddr": "127.0.0.1:0", "CertFile": certFile, "KeyFile": keyFile, "ClientCAFile": clientCA, "ReplayAddr": redisAddress, "JWKSCAFile": certFile, "JWKSURLs": map[string]string{"role": jwks.URL}, "RefreshAfter": time.Minute, "HardExpiry": 2 * time.Minute, "UnknownKIDCooldown": time.Second}}))
	var created nativeSpaceReply
	require.NoError(t, decoder.Decode(&created), stderr.String())
	space := uuid.MustParse(created.SpaceID)
	fixture.Run(true, "up")
	require.NoError(t, encoder.Encode(map[string]string{"Action": "prepare"}))
	var preparing nativeSpaceReply
	require.NoError(t, decoder.Decode(&preparing), stderr.String())
	require.NotNil(t, preparing.Binding)
	spaceConn, err := grpc.NewClient(preparing.Address, grpc.WithTransportCredentials(transport))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, spaceConn.Close()) })
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "current", PrivateKey: current})
	require.NoError(t, err)
	authority := &memberauthority.Client{Space: spacev1.NewSpaceServiceClient(spaceConn), Issuer: issuer}
	trackedAuthority := &nativeTrackedObserver{client: authority}
	require.NoError(t, roles.BootstrapSpaceRoles(ctx, space, owner))
	ownerAccount := uuid.NewString()
	var floor atomic.Int64
	floor.Store(7)
	runtime, err := principalruntime.NewWithSessionEpochChecker(ctx, principalruntime.Config{ListenAddr: "127.0.0.1:0", TLSCertFile: certFile, TLSKeyFile: keyFile, ClientCAFile: clientCA, ReplayAddr: redisAddress, JWKSCAFile: certFile, JWKSURLs: map[string]string{"space": jwks.URL, "voice": jwks.URL, "gameintegration": jwks.URL, "gateway": jwks.URL, "bot": jwks.URL}, BotLifecycleEnabled: true, DelegatedUserEnabled: true, RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second}, func(_ context.Context, account string, epoch int64) error {
		if account != ownerAccount || epoch < floor.Load() {
			return errors.New("stale session")
		}
		return nil
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	legacy, protected := newRoleGRPCServers(nil, &grpcsvc.RoleGRPC{Store: roles, MemberAuthority: trackedAuthority}, runtime)
	legacy.Stop()
	_, address := sourceListenerClient(t, protected, transport)
	roleConn, err := grpc.NewClient(address, grpc.WithTransportCredentials(transport))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, roleConn.Close()) })
	client := rolev1.NewRoleServiceClient(roleConn)
	b := preparing.Binding
	request := &rolev1.EnsureDefaultMemberRoleRequest{ProtocolVersion: b.ProtocolVersion, SpaceId: b.SpaceID, ProfileId: b.ProfileID, MembershipGeneration: b.MembershipGeneration, AdmissionOperationId: b.AdmissionOperationID, AdmissionBindingSha256: b.AdmissionBindingSHA256}
	call := func(req *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
		return client.EnsureDefaultMemberRole(nativeServiceContext(t, current, rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName, req), req)
	}
	response, err := call(request)
	require.NoError(t, err, "real protected Space PREPARING must yield a committed inert Role receipt")
	receipt := response.GetReceipt()
	require.NotNil(t, receipt)
	digest, err := memberadmission.EnsureReceiptFingerprint(*b, receipt.RoleId)
	require.NoError(t, err)
	require.Equal(t, digest[:], receipt.ReceiptSha256)
	raw, err := roles.GetRawMemberRoleInventory(ctx, space, profile)
	require.NoError(t, err)
	require.Empty(t, raw, "inert Role preparation is not publication")
	replay, err := call(request)
	require.NoError(t, err)
	require.Equal(t, receipt, replay.Receipt)
	require.NoError(t, encoder.Encode(map[string]any{"Action": "publish", "RoleID": receipt.RoleId, "Receipt": receipt.ReceiptSha256}))
	var published nativeSpaceReply
	require.NoError(t, decoder.Decode(&published), stderr.String())
	require.True(t, published.Published)
	observed, err := authority.ObserveMember(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: b.SpaceID, ProfileId: b.ProfileID})
	require.NoError(t, err)
	observedCtx, err := memberauthority.WithObservations(ctx, observed)
	require.NoError(t, err)
	qualified, err := roles.GetQualifiedMemberRoles(observedCtx, space, profile)
	require.NoError(t, err)
	require.Len(t, qualified, 1)
	require.Equal(t, receipt.RoleId, qualified[0].ID.String())
	memberRequest := &rolev1.GetMemberRolesRequest{SpaceId: b.SpaceID, ProfileId: b.ProfileID}
	memberResponse, err := client.GetMemberRoles(nativeServiceContext(t, current, rolev1.RoleService_GetMemberRoles_FullMethodName, memberRequest), memberRequest)
	require.NoError(t, err, "actual native public caller must collect protected Space evidence outside Role locks")
	require.Len(t, memberResponse.GetRoleList().GetRoles(), 1)
	voiceRequest := &rolev1.ResolveVoiceRoomGrantsRequest{SpaceId: b.SpaceID, ProfileId: b.ProfileID, VoiceRoomId: uuid.NewString()}
	voiceResponse, err := client.ResolveVoiceRoomGrants(nativeWorkloadContext(t, current, "voice", rolev1.RoleService_ResolveVoiceRoomGrants_FullMethodName, voiceRequest), voiceRequest)
	require.NoError(t, err, "derived Voice decision must collect actual native membership before taking the Role fence")
	require.NotZero(t, voiceResponse.GetPolicyEpoch())
	require.True(t, voiceResponse.GetGrants().GetCanJoin(), "published default Member's qualified Voice grant remains usable")
	botDecision := &rolev1.CheckPermissionRequest{SpaceId: b.SpaceID, ProfileId: owner.String(), PermissionName: "SPACE_MANAGE_BOTS"}
	botContext := nativeWorkloadContext(t, current, "bot", rolev1.RoleService_CheckPermission_FullMethodName, botDecision)
	botResponse, err := client.CheckPermission(botContext, botDecision)
	require.NoError(t, err, "exact Bot installation decision must use the protected qualified receiver")
	require.True(t, botResponse.GetAllowed())
	_, err = client.CheckPermission(botContext, botDecision)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "Bot decision transport replay must be rejected")
	botMemberDecision := proto.Clone(botDecision).(*rolev1.CheckPermissionRequest)
	botMemberDecision.ProfileId = b.ProfileID
	botResponse, err = client.CheckPermission(nativeWorkloadContext(t, current, "bot", rolev1.RoleService_CheckPermission_FullMethodName, botMemberDecision), botMemberDecision)
	require.NoError(t, err)
	require.False(t, botResponse.GetAllowed(), "ordinary qualified Member is not a Bot administrator")
	botScopedDecision := proto.Clone(botDecision).(*rolev1.CheckPermissionRequest)
	room := uuid.NewString()
	botScopedDecision.VoiceRoomId = &room
	_, err = client.CheckPermission(nativeWorkloadContext(t, current, "bot", rolev1.RoleService_CheckPermission_FullMethodName, botScopedDecision), botScopedDecision)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "Bot management accepts no client-selected node")
	botWrongPurpose := proto.Clone(botDecision).(*rolev1.CheckPermissionRequest)
	botWrongPurpose.PermissionName = "SPACE_MANAGE_SETTINGS"
	_, err = client.CheckPermission(nativeWorkloadContext(t, current, "bot", rolev1.RoleService_CheckPermission_FullMethodName, botWrongPurpose), botWrongPurpose)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "Bot cannot borrow the Space settings purpose")
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: current})
	require.NoError(t, err)
	delegated := func(method string, request proto.Message) context.Context {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "role", RPC: method, RequestID: id, RequestHash: hash, AccountID: ownerAccount, ProfileID: owner.String(), SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		call, cancel := context.WithTimeout(ctx, 5*time.Second)
		t.Cleanup(cancel)
		return metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	create := &rolev1.CreateRoleRequest{SpaceId: b.SpaceID, Name: "native interactive", Position: 1, PermissionsMask: 1}
	custom, err := client.CreateRole(delegated(rolev1.RoleService_CreateRole_FullMethodName, create), create)
	require.NoError(t, err)
	assign := &rolev1.AssignRoleRequest{SpaceId: b.SpaceID, ProfileId: b.ProfileID, RoleId: custom.GetRole().GetId()}
	_, err = client.AssignRole(delegated(rolev1.RoleService_AssignRole_FullMethodName, assign), assign)
	if requireInteractive {
		require.NoError(t, err)
		memberResponse, err = client.GetMemberRoles(nativeServiceContext(t, current, rolev1.RoleService_GetMemberRoles_FullMethodName, memberRequest), memberRequest)
		require.NoError(t, err, "a successful native assignment must create qualified source provenance rather than an unresolved raw tuple")
		require.Len(t, memberResponse.GetRoleList().GetRoles(), 2)
	} else {
		require.Equal(t, codes.Unavailable, status.Code(err), "missing accepted operation core cannot ACK a raw legacy write")
		inventory, err := roles.GetRawMemberRoleInventory(ctx, space, profile)
		require.NoError(t, err)
		require.Empty(t, inventory)
	}
	// Both fixed Space decision branches consume actual native authority. Owner
	// proof is its own Bootstrap evidence plus Space membership/ownership, never
	// a same-name or raw Admin shortcut. Ordinary Member gets neither bit.
	for _, permission := range []string{"SPACE_APPROVE_JOIN_REQUESTS", "SPACE_MANAGE_SETTINGS"} {
		for _, actor := range []uuid.UUID{owner, profile} {
			decision := &rolev1.CheckPermissionRequest{SpaceId: b.SpaceID, ProfileId: actor.String(), PermissionName: permission}
			result, err := client.CheckPermission(sourceCallContext(t, current, "space", rolev1.RoleService_CheckPermission_FullMethodName, uuid.NewString(), decision), decision)
			require.NoError(t, err)
			require.Equal(t, actor == owner, result.Allowed)
		}
	}
	// Same operation with a changed body cannot replace the immutable receipt.
	changed := *request
	changed.AdmissionBindingSha256 = append([]byte(nil), request.AdmissionBindingSha256...)
	changed.AdmissionBindingSha256[0] ^= 1
	_, err = call(&changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	_, err = client.EnsureDefaultMemberRole(context.Background(), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	remove := &rolev1.RemoveMemberRolesRequest{ProtocolVersion: 1, SpaceId: b.SpaceID, ProfileId: b.ProfileID, RemovalGenerationFloor: b.MembershipGeneration, RemovalOperationId: uuid.NewString(), RemovalBindingSha256: make([]byte, 32)}
	removeCall := func() (*rolev1.RemoveMemberRolesResponse, error) {
		return client.RemoveMemberRoles(nativeServiceContext(t, current, rolev1.RoleService_RemoveMemberRoles_FullMethodName, remove), remove)
	}
	removed, err := removeCall()
	require.NoError(t, err)
	require.NotNil(t, removed.Receipt)
	removedAgain, err := removeCall()
	require.NoError(t, err)
	require.Equal(t, removed.Receipt, removedAgain.Receipt)
	observed, err = authority.ObserveMember(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: b.SpaceID, ProfileId: b.ProfileID})
	require.NoError(t, err)
	observedCtx, err = memberauthority.WithObservations(ctx, observed)
	require.NoError(t, err)
	qualified, err = roles.GetQualifiedMemberRoles(observedCtx, space, profile)
	require.NoError(t, err)
	require.Empty(t, qualified, "independent local ordinary floor defeats a still-published old Space binding")
	replay, err = call(request)
	require.NoError(t, err)
	require.Equal(t, receipt, replay.Receipt)
	var count int
	require.NoError(t, roles.Pool.QueryRow(ctx, `SELECT count(*) FROM role_member_admission_preparations WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count, "historical Ensure cannot repair removed preparation")
	floor.Store(8)
	beforeAuthority := trackedAuthority.calls.Load()
	_, err = client.CreateRole(delegated(rolev1.RoleService_CreateRole_FullMethodName, create), create)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "fresh transport must validate the current session before any owner read or saved response")
	require.Equal(t, beforeAuthority, trackedAuthority.calls.Load())
}
