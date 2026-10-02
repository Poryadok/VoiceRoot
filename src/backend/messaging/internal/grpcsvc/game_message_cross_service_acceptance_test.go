package grpcsvc

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
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/gameprotocol"
	"voice/backend/messaging/internal/principalgrpc"
	"voice/backend/messaging/internal/principalruntime"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

// TestT16ComposeCrossServiceAcceptance runs only in the hosted Linux Compose
// gate. It keeps the Messaging gRPC interceptor and processor real, and uses
// live Auth and GIS HTTP services plus their migrated PostgreSQL stores.
func TestT16ComposeCrossServiceAcceptance(t *testing.T) {
	if os.Getenv("T16_ACCEPTANCE") != "1" {
		t.Skip("requires the hosted T16 Compose fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	authDB := t16Pool(t, ctx, "T16_AUTH_DATABASE_URL")
	gisDB := t16Pool(t, ctx, "T16_GIS_DATABASE_URL")
	gisRuntimeDB := t16Pool(t, ctx, "T16_GIS_RUNTIME_DATABASE_URL")
	userDB := t16Pool(t, ctx, "T16_USER_DATABASE_URL")
	chatDB := t16Pool(t, ctx, "T16_CHAT_DATABASE_URL")
	messagingDB := t16Pool(t, ctx, "T16_MESSAGING_DATABASE_URL")

	fixture := seedT16AcceptFixture(t, ctx, authDB, gisDB, userDB, chatDB)
	seedT16GISOwnerSession(t, ctx, fixture)
	waitT16AuthGISRecovery(t, ctx, authDB, fixture)
	assertT16AuthDeviceAuthorityFailsClosed(t, ctx, authDB, gisDB, fixture)
	workloadNonceCount := countT16GISWorkloadNonces(t, ctx)
	assertion, authorityProof := requestT16AuthDeviceAuthority(t, ctx, fixture)
	// Auth admission reads GIS policy once, then its production binding-authority reader
	// calls GIS once; both requests must traverse the production WorkloadProof verifier.
	require.Equal(t, workloadNonceCount+2, countT16GISWorkloadNonces(t, ctx),
		"successful Auth device-authority issuance must traverse GIS's production WorkloadProof verifier for policy and binding authority")
	noncesAfterIssuance := countT16GISWorkloadNonces(t, ctx)
	require.Equal(t, assertion, replayT16AuthDeviceAuthority(t, ctx, fixture, authorityProof),
		"exact Auth device-authority proof retry must return the retained assertion")
	require.Equal(t, noncesAfterIssuance, countT16GISWorkloadNonces(t, ctx),
		"exact Auth device-authority proof retry must not call GIS again")

	root := requiredT16Env(t, "T16_FIXTURE_DIR")
	tlsDir := filepath.Join(root, "tls")
	caFile := filepath.Join(tlsDir, "ca.crt")
	messagingCert := filepath.Join(tlsDir, "messaging-client.crt")
	messagingKey := filepath.Join(tlsDir, "messaging-client.key")
	authBase := requiredT16Env(t, "T16_AUTH_MTLS_BASE_URL")
	gameAuthKeys, err := s2s.NewGameAuthKeys(s2s.GameAuthKeysConfig{
		JWKSURL:     authBase + "/api/v1/auth/.well-known/principal-jwks.json",
		TLSCertFile: messagingCert, TLSKeyFile: messagingKey, CAFile: caFile,
	})
	require.NoError(t, err)
	t.Cleanup(gameAuthKeys.Close)
	permitClient, err := s2s.NewGameMessageExecutionPermitClient(s2s.GameMessageExecutionPermitConfig{
		Endpoint:    authBase + "/api/v1/auth/sdk/game-message/execution-permits",
		TLSCertFile: messagingCert, TLSKeyFile: messagingKey, CAFile: caFile,
	})
	require.NoError(t, err)
	mappingClient, err := s2s.NewGameResourceMappingAuthorizationClient(s2s.GameResourceMappingAuthorizationConfig{
		Endpoint:    requiredT16Env(t, "T16_GIS_MTLS_BASE_URL") + "/internal/v1/game-integrations/resource-mappings/authorize-chat",
		TLSCertFile: messagingCert, TLSKeyFile: messagingKey, CAFile: caFile,
		WorkloadKeyBase64: requiredT16Env(t, "T16_MESSAGING_WORKLOAD_KEY_B64"),
	})
	require.NoError(t, err)

	storeMessages := &store.MessagesStore{Pool: messagingDB}
	chatGuard := &t16BlockingChatGuard{ChatGuard: &store.SQLChatGuard{Pool: chatDB}, entered: make(chan struct{}), release: make(chan struct{})}
	mappingAuthority := &t16DiagnosticResourceMappingAuthority{
		delegate: mappingClient, runtimeDB: gisRuntimeDB, t: t,
	}
	processor := &VerifiedGameMessageProcessor{Store: storeMessages, AuthKeys: gameAuthKeys,
		Permits: permitClient, Bindings: &AuthBackedGameBindingAuthority{
			Chats: chatGuard, ResourceMappings: mappingAuthority,
		}}
	diagnosticProcessor := &t16DiagnosticGameMessageProcessor{delegate: processor, t: t}
	diagnosticProcessor.probeAuthJWKS = func() {
		probeT16AuthPrincipalJWKS(t, ctx, authBase, messagingCert, messagingKey, caFile)
	}
	verifier, gatewayIssuer := t16GatewayPrincipalRuntime(t, ctx)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(principalgrpc.ApplyGameMessageUnaryInterceptor(verifier)))
	messagingv1.RegisterMessagingServiceServer(grpcServer, &MessagingGRPC{Messages: storeMessages, GameMessages: diagnosticProcessor})
	listener := bufconn.Listen(1 << 20)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)
	conn, err := grpc.NewClient("passthrough:///t16-messaging", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := messagingv1.NewMessagingServiceClient(conn)

	request := &messagingv1.ApplyGameMessageRequest{CompactJws: fixture.compactMessage, DeviceAuthorityAssertion: assertion}
	requestHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	newT16Request := func(current t16AcceptFixture) (*messagingv1.ApplyGameMessageRequest, string) {
		freshAssertion, _ := requestT16AuthDeviceAuthority(t, ctx, current)
		freshRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: current.compactMessage, DeviceAuthorityAssertion: freshAssertion}
		freshHash, hashErr := principal.RequestHash(freshRequest)
		require.NoError(t, hashErr)
		return freshRequest, freshHash
	}
	apply := func(callRequest *messagingv1.ApplyGameMessageRequest, tokenIssuer *principal.Issuer, audience, rpc, signedHash string) (*messagingv1.ApplyGameMessageResponse, error) {
		id := uuid.NewString()
		servicePrincipal, issueErr := tokenIssuer.IssueService(principal.ServiceInput{
			Audience: audience, RPC: rpc, RequestID: id, RequestHash: signedHash,
		})
		if issueErr != nil {
			return nil, issueErr
		}
		callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+servicePrincipal, "x-request-id", id))
		return client.ApplyGameMessage(callCtx, callRequest)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	wrongIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "untrusted-gateway", KeyID: "gateway-t16-wrong", PrivateKey: otherKey})
	require.NoError(t, err)
	for _, test := range []struct {
		name, audience, rpc, hash string
		issuer                    *principal.Issuer
	}{
		{name: "wrong issuer", audience: "messaging", rpc: messagingv1.MessagingService_ApplyGameMessage_FullMethodName, hash: requestHash, issuer: wrongIssuer},
		{name: "wrong service", audience: "auth", rpc: messagingv1.MessagingService_ApplyGameMessage_FullMethodName, hash: requestHash, issuer: gatewayIssuer},
		{name: "wrong RPC", audience: "messaging", rpc: "/voice.messaging.v1.Other/Call", hash: requestHash, issuer: gatewayIssuer},
		{name: "wrong protobuf hash", audience: "messaging", rpc: messagingv1.MessagingService_ApplyGameMessage_FullMethodName, hash: strings.Repeat("0", 64), issuer: gatewayIssuer},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, callErr := apply(request, test.issuer, test.audience, test.rpc, test.hash)
			require.Error(t, callErr)
		})
	}
	assertNoT16ExecutionSideEffects(t, ctx, authDB, gisDB, messagingDB, fixture)

	// GIS must deny absent, foreign-application, foreign-environment,
	// foreign-binding, and altered-chat mappings before Auth issues any permit.
	for _, wrongScope := range []struct {
		name             string
		wrongApplication bool
	}{
		{name: "wrong application", wrongApplication: true},
		{name: "wrong environment"},
	} {
		t.Run(wrongScope.name+" mapping", func(t *testing.T) {
			_, deleteErr := gisDB.Exec(ctx, `DELETE FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND chat_id=$3`, fixture.appID, fixture.envID, fixture.chatID)
			require.NoError(t, deleteErr)
			wrongAppID, wrongEnvID, seedErr := seedT16WrongScopeResourceMapping(ctx, gisDB, fixture, wrongScope.wrongApplication)
			require.NoError(t, seedErr)
			wrongScopeRequest, wrongScopeHash := newT16Request(fixture)
			workloadNoncesBeforeMapping := countT16GISWorkloadNonces(t, ctx)
			_, callErr := apply(wrongScopeRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, wrongScopeHash)
			require.Error(t, callErr, "mapping for a different application/environment must fail closed")
			require.Equal(t, workloadNoncesBeforeMapping+1, countT16GISWorkloadNonces(t, ctx), "scope-mismatch denial must reach GIS exactly once")
			assertNoT16ExecutionSideEffects(t, ctx, authDB, gisDB, messagingDB, fixture)
			_, cleanupErr := gisDB.Exec(ctx, `DELETE FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key='t16-wrong-scope'`, wrongAppID, wrongEnvID)
			require.NoError(t, cleanupErr)
			_, cleanupErr = gisDB.Exec(ctx, `DELETE FROM environments WHERE application_id=$1 AND id=$2`, wrongAppID, wrongEnvID)
			require.NoError(t, cleanupErr)
			if wrongScope.wrongApplication {
				_, cleanupErr = gisDB.Exec(ctx, `DELETE FROM applications WHERE id=$1`, wrongAppID)
				require.NoError(t, cleanupErr)
			}
		})
	}
	require.NoError(t, seedT16ResourceMapping(ctx, gisDB, fixture))

	_, err = gisDB.Exec(ctx, `DELETE FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND chat_id=$3`, fixture.appID, fixture.envID, fixture.chatID)
	require.NoError(t, err)
	missingMappingRequest, missingMappingHash := newT16Request(fixture)
	diagnosticProcessor.watchAssertion(missingMappingRequest.GetDeviceAuthorityAssertion())
	workloadNoncesBeforeMapping := countT16GISWorkloadNonces(t, ctx)
	_, err = apply(missingMappingRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, missingMappingHash)
	diagnosticProcessor.clearWatchedAssertion()
	require.Error(t, err, "absent mapping must fail closed")
	require.Equal(t, workloadNoncesBeforeMapping+1, countT16GISWorkloadNonces(t, ctx), "absent mapping denial must reach GIS exactly once")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, gisDB, messagingDB, fixture)
	require.NoError(t, seedT16ResourceMapping(ctx, gisDB, fixture))

	_, err = gisDB.Exec(ctx, `DELETE FROM game_resource_binding_chats WHERE application_id=$1 AND environment_id=$2 AND binding_id=$3 AND chat_id=$4`, fixture.appID, fixture.envID, fixture.bindingID, fixture.chatID)
	require.NoError(t, err)
	foreignBindingID := uuid.New()
	_, err = gisDB.Exec(ctx, `INSERT INTO player_bindings(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision)
VALUES($1,$2,$3,'google','hmac-sha256-v1:t16-foreign:`+strings.Repeat("f", 64)+`',$4,$5,$6,$7,'active',1)`, foreignBindingID, fixture.appID, fixture.envID,
		fixture.sourceAccountID, fixture.sourceActorID, fixture.targetProfileID, fixture.deviceID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `INSERT INTO game_resource_binding_chats(application_id,environment_id,binding_id,chat_id,roster_revision,status,lease_expires_at) VALUES($1,$2,$3,$4,1,'active',now()+interval '1 hour')`, fixture.appID, fixture.envID, foreignBindingID, fixture.chatID)
	require.NoError(t, err)
	foreignMappingRequest, foreignMappingHash := newT16Request(fixture)
	workloadNoncesBeforeMapping = countT16GISWorkloadNonces(t, ctx)
	_, err = apply(foreignMappingRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, foreignMappingHash)
	require.Error(t, err, "foreign binding mapping must fail closed")
	require.Equal(t, workloadNoncesBeforeMapping+1, countT16GISWorkloadNonces(t, ctx), "foreign binding denial must reach GIS exactly once")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, gisDB, messagingDB, fixture)
	_, err = gisDB.Exec(ctx, `DELETE FROM game_resource_binding_chats WHERE application_id=$1 AND environment_id=$2 AND binding_id=$3 AND chat_id=$4`, fixture.appID, fixture.envID, foreignBindingID, fixture.chatID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `INSERT INTO game_resource_binding_chats(application_id,environment_id,binding_id,chat_id,roster_revision,status,lease_expires_at) VALUES($1,$2,$3,$4,1,'active',now()+interval '1 hour')`, fixture.appID, fixture.envID, fixture.bindingID, fixture.chatID)
	require.NoError(t, err)

	_, err = gisDB.Exec(ctx, `UPDATE game_resource_mappings SET chat_id=$1,resource_id=$1 WHERE application_id=$2 AND environment_id=$3 AND chat_id=$4`, uuid.New(), fixture.appID, fixture.envID, fixture.chatID)
	require.NoError(t, err)
	alteredMappingRequest, alteredMappingHash := newT16Request(fixture)
	workloadNoncesBeforeMapping = countT16GISWorkloadNonces(t, ctx)
	_, err = apply(alteredMappingRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, alteredMappingHash)
	require.Error(t, err, "altered chat mapping must fail closed")
	require.Equal(t, workloadNoncesBeforeMapping+1, countT16GISWorkloadNonces(t, ctx), "altered chat denial must reach GIS exactly once")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, gisDB, messagingDB, fixture)
	require.NoError(t, seedT16ResourceMapping(ctx, gisDB, fixture))

	positiveMappingRequest, positiveMappingHash := newT16Request(fixture)
	var seededMappingRevision int64
	err = gisDB.QueryRow(ctx, `SELECT m.mapping_revision
		FROM game_resource_binding_chats r
		JOIN applications a ON a.id=r.application_id AND a.status IN ('sandbox','active')
		JOIN environments e ON e.id=r.environment_id AND e.application_id=r.application_id AND e.status='active'
		JOIN player_bindings b ON b.application_id=r.application_id AND b.environment_id=r.environment_id
			AND b.binding_id=r.binding_id AND b.status='active'
		JOIN game_resource_mappings m ON m.application_id=r.application_id AND m.environment_id=r.environment_id
			AND m.resource_kind='chat' AND m.chat_id=r.chat_id AND m.status='active'
		WHERE r.application_id=$1 AND r.environment_id=$2 AND r.binding_id=$3 AND r.chat_id=$4
		AND r.status='active' AND r.lease_expires_at > clock_timestamp()`,
		fixture.appID, fixture.envID, fixture.bindingID, fixture.chatID).Scan(&seededMappingRevision)
	require.NoError(t, err, "GIS exact app/environment/binding/chat predicate must match the seeded active tuple")
	require.Positive(t, seededMappingRevision)
	var runtimeUser, runtimeDatabase, runtimeSearchPath string
	err = gisRuntimeDB.QueryRow(ctx, `SELECT current_user, current_database(), current_setting('search_path')`).
		Scan(&runtimeUser, &runtimeDatabase, &runtimeSearchPath)
	require.NoError(t, err)
	require.Equal(t, "gameintegration_runtime", runtimeUser)
	require.Equal(t, "game_integration_db", runtimeDatabase)
	var runtimeMappingRevision int64
	err = gisRuntimeDB.QueryRow(ctx, `SELECT m.mapping_revision
		FROM game_resource_binding_chats r
		JOIN applications a ON a.id=r.application_id AND a.status IN ('sandbox','active')
		JOIN environments e ON e.id=r.environment_id AND e.application_id=r.application_id AND e.status='active'
		JOIN player_bindings b ON b.application_id=r.application_id AND b.environment_id=r.environment_id
			AND b.binding_id=r.binding_id AND b.status='active'
		JOIN game_resource_mappings m ON m.application_id=r.application_id AND m.environment_id=r.environment_id
			AND m.resource_kind='chat' AND m.chat_id=r.chat_id AND m.status='active'
		WHERE r.application_id=$1 AND r.environment_id=$2 AND r.binding_id=$3 AND r.chat_id=$4
		AND r.status='active' AND r.lease_expires_at > clock_timestamp()`,
		fixture.appID, fixture.envID, fixture.bindingID, fixture.chatID).Scan(&runtimeMappingRevision)
	require.NoError(t, err, "GIS runtime role must see the exact active app/environment/binding/chat tuple")
	require.Equal(t, seededMappingRevision, runtimeMappingRevision,
		"GIS runtime role and fixture owner must observe the same mapping revision; search_path=%s", runtimeSearchPath)
	workloadNoncesBeforePositive := countT16GISWorkloadNonces(t, ctx)
	mappingCallsBeforePositive := mappingAuthority.calls.Load()
	diagnosticProcessor.watchAssertion(positiveMappingRequest.GetDeviceAuthorityAssertion())
	response, err := apply(positiveMappingRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, positiveMappingHash)
	diagnosticProcessor.clearWatchedAssertion()
	workloadNonceDelta := countT16GISWorkloadNonces(t, ctx) - workloadNoncesBeforePositive
	t.Logf("T16 positive request returned err=%v GIS workload nonce delta=%d", err, workloadNonceDelta)
	if err != nil {
		var authPermits, gisPermits, messageCompletions int
		require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1`, fixture.operationID).Scan(&authPermits))
		require.NoError(t, gisDB.QueryRow(ctx, `SELECT count(*) FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, fixture.operationID).Scan(&gisPermits))
		require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1`, fixture.operationID).Scan(&messageCompletions))
		t.Logf("T16 positive failure state: auth_permits=%d gis_permits=%d messaging_completions=%d", authPermits, gisPermits, messageCompletions)
	}
	require.NoError(t, err, "the real Auth -> GIS -> Messaging path should accept the exact linked chat")
	require.Equal(t, int64(1), mappingAuthority.calls.Load()-mappingCallsBeforePositive,
		"the exact linked-chat request must traverse GIS mapping authorization exactly once")
	require.Equal(t, 5, workloadNonceDelta,
		"positive request should use one mapping proof, two Auth policy preflight proofs, one permit issue proof, and one completion proof")
	require.Equal(t, fixture.messageID.String(), response.GetMessage().GetId())
	require.Equal(t, fixture.chatID.String(), response.GetMessage().GetChat().GetId())
	var authPermitID, gisPermitID uuid.UUID
	var authOutcome string
	var authReceipt []byte
	var authCompletedAt time.Time
	require.NoError(t, authDB.QueryRow(ctx, `SELECT permit_jti,gis_permit_id,completion_outcome,completion_receipt,completed_at
		FROM sdk_game_message_execution_permits WHERE operation_id=$1`, fixture.operationID).
		Scan(&authPermitID, &gisPermitID, &authOutcome, &authReceipt, &authCompletedAt))
	require.NotEqual(t, uuid.Nil, authPermitID)
	require.NotEqual(t, uuid.Nil, gisPermitID)
	require.Equal(t, "committed", authOutcome)
	require.NotEmpty(t, authReceipt)
	require.JSONEq(t, `{"outcome":"committed","operation_id":"`+fixture.operationID.String()+
		`","permit_id":"`+gisPermitID.String()+`","status":"completed"}`, string(authReceipt))
	require.False(t, authCompletedAt.IsZero())
	var gisPermitStatus string
	var gisCompletedAt time.Time
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT status,completion_at FROM player_binding_execution_permits
		WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, fixture.operationID).
		Scan(&gisPermitStatus, &gisCompletedAt))
	require.Equal(t, "committed", gisPermitStatus)
	require.False(t, gisCompletedAt.IsZero())
	var messagingOutcome, messagingStatus string
	var messagingCompletedAt time.Time
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT outcome,status,completed_at FROM game_message_execution_permit_completions
		WHERE operation_id=$1`, fixture.operationID).Scan(&messagingOutcome, &messagingStatus, &messagingCompletedAt))
	require.Equal(t, "committed", messagingOutcome)
	require.Equal(t, "completed", messagingStatus)
	require.False(t, messagingCompletedAt.IsZero())

	// Auth's retained committed receipt makes an exact completion replay
	// read-only: it returns the same validated receipt without a fresh GIS call.
	workloadNoncesBeforeCompletionReplay := countT16GISWorkloadNonces(t, ctx)
	require.NoError(t, permitClient.Complete(ctx, authPermitID, gisPermitID, fixture.operationID, "committed"))
	var replayedAuthOutcome string
	var replayedAuthReceipt []byte
	var replayedAuthCompletedAt time.Time
	require.NoError(t, authDB.QueryRow(ctx, `SELECT completion_outcome,completion_receipt,completed_at
		FROM sdk_game_message_execution_permits WHERE operation_id=$1`, fixture.operationID).
		Scan(&replayedAuthOutcome, &replayedAuthReceipt, &replayedAuthCompletedAt))
	require.Equal(t, authOutcome, replayedAuthOutcome)
	require.Equal(t, authReceipt, replayedAuthReceipt)
	require.Equal(t, authCompletedAt, replayedAuthCompletedAt)
	var replayedGISStatus, replayedMessagingOutcome, replayedMessagingStatus string
	var replayedGISCompletedAt, replayedMessagingCompletedAt time.Time
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT status,completion_at FROM player_binding_execution_permits
		WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, fixture.operationID).
		Scan(&replayedGISStatus, &replayedGISCompletedAt))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT outcome,status,completed_at FROM game_message_execution_permit_completions
		WHERE operation_id=$1`, fixture.operationID).
		Scan(&replayedMessagingOutcome, &replayedMessagingStatus, &replayedMessagingCompletedAt))
	require.Equal(t, gisPermitStatus, replayedGISStatus)
	require.Equal(t, gisCompletedAt, replayedGISCompletedAt)
	require.Equal(t, messagingOutcome, replayedMessagingOutcome)
	require.Equal(t, messagingStatus, replayedMessagingStatus)
	require.Equal(t, messagingCompletedAt, replayedMessagingCompletedAt)
	require.Equal(t, workloadNoncesBeforeCompletionReplay, countT16GISWorkloadNonces(t, ctx),
		"exact Auth completion replay must not call GIS or consume a new WorkloadProof nonce")

	// A lost gRPC response is retried with a fresh Gateway principal. Messaging
	// must return the exact durable receipt without another permit or message.
	retryID := uuid.NewString()
	retryPrincipal, err := gatewayIssuer.IssueService(principal.ServiceInput{
		Audience: "messaging", RPC: messagingv1.MessagingService_ApplyGameMessage_FullMethodName,
		RequestID: retryID, RequestHash: positiveMappingHash,
	})
	require.NoError(t, err)
	retryCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+retryPrincipal, "x-request-id", retryID))
	var authPermitsBeforeRetry, gisPermitsBeforeRetry, permitCompletionsBeforeRetry int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1`, fixture.operationID).Scan(&authPermitsBeforeRetry))
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT count(*) FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, fixture.operationID).Scan(&gisPermitsBeforeRetry))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1`, fixture.operationID).Scan(&permitCompletionsBeforeRetry))
	workloadNoncesBeforeMessageRetry := countT16GISWorkloadNonces(t, ctx)
	retried, err := client.ApplyGameMessage(retryCtx, positiveMappingRequest)
	require.NoError(t, err)
	require.True(t, proto.Equal(response, retried), "exact operation retry must return the complete same durable response")
	require.Equal(t, workloadNoncesBeforeMessageRetry, countT16GISWorkloadNonces(t, ctx),
		"exact message retry must not make fresh GIS calls or consume a WorkloadProof nonce")
	var authPermitsAfterRetry, gisPermitsAfterRetry, permitCompletionsAfterRetry int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1`, fixture.operationID).Scan(&authPermitsAfterRetry))
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT count(*) FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, fixture.operationID).Scan(&gisPermitsAfterRetry))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1`, fixture.operationID).Scan(&permitCompletionsAfterRetry))
	require.Equal(t, authPermitsBeforeRetry, authPermitsAfterRetry)
	require.Equal(t, gisPermitsBeforeRetry, gisPermitsAfterRetry)
	require.Equal(t, permitCompletionsBeforeRetry, permitCompletionsAfterRetry)
	var revisions int
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, fixture.messageID).Scan(&revisions))
	require.Equal(t, 1, revisions)
	var permitCompletions int
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1 AND outcome='committed'`, fixture.operationID).Scan(&permitCompletions))
	require.Equal(t, 1, permitCompletions)

	// Chat membership is intentionally checked after Auth/GIS issues the permit.
	// The failure must therefore leave a durable aborted completion and no message.
	chatFailure := fixture
	chatFailure.operationID, chatFailure.messageID = uuid.New(), newT16MessageID(t)
	chatFailure.compactMessage = signT16Message(t, chatFailure, fixture.deviceKey, time.Now().UTC())
	chatAssertion, _ := requestT16AuthDeviceAuthority(t, ctx, chatFailure)
	chatRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: chatFailure.compactMessage, DeviceAuthorityAssertion: chatAssertion}
	chatHash, err := principal.RequestHash(chatRequest)
	require.NoError(t, err)
	_, err = chatDB.Exec(ctx, `DELETE FROM chat_members WHERE chat_id=$1 AND profile_id=$2`, fixture.chatID, fixture.targetProfileID)
	require.NoError(t, err)
	_, err = apply(chatRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, chatHash)
	require.Error(t, err, "non-member chat use must be denied")
	_, err = chatDB.Exec(ctx, `INSERT INTO chat_members(chat_id,profile_id,role) VALUES($1,$2,'member')`, fixture.chatID, fixture.targetProfileID)
	require.NoError(t, err)
	var abortedPermits, abortedCompletions, deniedMessages int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1 AND completion_outcome='aborted'`, chatFailure.operationID).Scan(&abortedPermits))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1 AND outcome='aborted' AND completed_at IS NOT NULL`, chatFailure.operationID).Scan(&abortedCompletions))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, chatFailure.messageID).Scan(&deniedMessages))
	require.Equal(t, 1, abortedPermits)
	require.Equal(t, 1, abortedCompletions)
	require.Zero(t, deniedMessages)

	// A syntactically valid attachment reaches the File provenance boundary only
	// after the permit. This harness intentionally has no File service, so the
	// processor must fail closed and terminalize that permit as aborted.
	fileFailure := fixture
	fileFailure.operationID, fileFailure.messageID = uuid.New(), newT16MessageID(t)
	fileFailure.compactMessage = signT16MessageWithAttachments(t, fileFailure, fixture.deviceKey, time.Now().UTC(), []map[string]any{{
		"byte_length": 17, "content_sha256": strings.Repeat("a", 64), "file_id": uuid.NewString(), "media_type": "text/plain", "object_revision": 1,
	}})
	fileAssertion, _ := requestT16AuthDeviceAuthority(t, ctx, fileFailure)
	fileRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: fileFailure.compactMessage, DeviceAuthorityAssertion: fileAssertion}
	fileHash, err := principal.RequestHash(fileRequest)
	require.NoError(t, err)
	_, err = apply(fileRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, fileHash)
	require.Error(t, err, "File provenance must fail closed when unavailable")
	var fileAbortedPermits, fileAbortedCompletions, fileDeniedMessages int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1 AND completion_outcome='aborted'`, fileFailure.operationID).Scan(&fileAbortedPermits))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1 AND outcome='aborted' AND completed_at IS NOT NULL`, fileFailure.operationID).Scan(&fileAbortedCompletions))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, fileFailure.messageID).Scan(&fileDeniedMessages))
	require.Equal(t, 1, fileAbortedPermits)
	require.Equal(t, 1, fileAbortedCompletions)
	require.Zero(t, fileDeniedMessages)

	// Hold a real Messaging operation at the Chat check after its real Auth/GIS
	// permit is durable. A second real Auth-issued permit has no Messaging receipt
	// and withholds completion so GIS must exercise its explicit expiry path.
	revokeRace := fixture
	revokeRace.operationID, revokeRace.messageID = uuid.New(), newT16MessageID(t)
	revokeRace.compactMessage = signT16Message(t, revokeRace, fixture.deviceKey, time.Now().UTC())
	revokeAssertion, _ := requestT16AuthDeviceAuthority(t, ctx, revokeRace)
	revokeRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: revokeRace.compactMessage, DeviceAuthorityAssertion: revokeAssertion}
	revokeHash, err := principal.RequestHash(revokeRequest)
	require.NoError(t, err)
	chatGuard.arm()
	messageResult := make(chan error, 1)
	go func() {
		_, callErr := apply(revokeRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, revokeHash)
		messageResult <- callErr
	}()
	select {
	case <-chatGuard.entered:
	case <-ctx.Done():
		t.Fatal("real Messaging operation did not reach the post-permit Chat barrier")
	}
	pendingOperationID := uuid.New()
	pendingMutation := []byte("T16 explicit expiry with no receipt")
	pendingPermit, err := permitClient.Issue(ctx, gameprotocol.DeviceAuthority{AssertionJWS: assertion}, pendingOperationID, pendingMutation)
	require.NoError(t, err, "Auth must issue the second live GIS permit")
	pendingPermitRetry, err := permitClient.Issue(ctx, gameprotocol.DeviceAuthority{AssertionJWS: assertion}, pendingOperationID, pendingMutation)
	require.NoError(t, err, "exact retry after a lost permit response must replay the live Auth/GIS receipt")
	require.Equal(t, pendingPermit, pendingPermitRetry, "exact permit retry must return the same signed durable result")
	var pendingAuthRows, pendingGISRows int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1`, pendingOperationID).Scan(&pendingAuthRows))
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT count(*) FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, pendingOperationID).Scan(&pendingGISRows))
	require.Equal(t, 1, pendingAuthRows, "Auth retry must retain one durable permit")
	require.Equal(t, 1, pendingGISRows, "GIS retry must retain one durable permit")
	var pendingExpiryBeforeMS int64
	var pendingExpiryAfter time.Time
	require.NoError(t, authDB.QueryRow(ctx, `SELECT expires_at_ms FROM sdk_game_message_execution_permits WHERE operation_id=$1`, pendingOperationID).Scan(&pendingExpiryBeforeMS))
	pendingExpiryBefore := time.UnixMilli(pendingExpiryBeforeMS).UTC()
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT expires_at FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, pendingOperationID).Scan(&pendingExpiryAfter))
	require.Equal(t, pendingExpiryBefore, pendingExpiryAfter.Truncate(time.Millisecond).UTC(), "Auth and GIS must persist the same fixed permit expiry to the signed millisecond precision")
	var pendingPermitStatus string
	var pendingExpiresAt time.Time
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT status,expires_at FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, pendingOperationID).Scan(&pendingPermitStatus, &pendingExpiresAt))
	require.Equal(t, "issued", pendingPermitStatus)

	ownerToken := t16OwnerAccessToken(t, fixture)
	revokeStarted := time.Now()
	revokeIdempotencyKey := uuid.NewString()
	type revokeResult struct {
		status int
		body   string
		err    error
	}
	revokeDone := make(chan revokeResult, 1)
	go func() {
		for attempt := 0; attempt < 8; attempt++ {
			req, requestErr := http.NewRequestWithContext(ctx, http.MethodDelete,
				os.Getenv("T16_GIS_BASE_URL")+"/api/v1/game-integrations/bindings/"+fixture.bindingID.String(),
				strings.NewReader(`{"expected_revision":1}`))
			if requestErr != nil {
				revokeDone <- revokeResult{err: requestErr}
				return
			}
			req.Header.Set("Authorization", "Bearer "+ownerToken)
			req.Header.Set("Content-Type", "application/json")
			// All retries use the exact same idempotency key.
			req.Header.Set("Idempotency-Key", revokeIdempotencyKey)
			response, requestErr := http.DefaultClient.Do(req)
			if requestErr != nil {
				revokeDone <- revokeResult{err: requestErr}
				return
			}
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
			_ = response.Body.Close()
			if readErr != nil {
				revokeDone <- revokeResult{err: readErr}
				return
			}
			if response.StatusCode == http.StatusOK {
				revokeDone <- revokeResult{status: response.StatusCode, body: string(body)}
				return
			}
			if response.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), "BINDING_REVOCATION_PENDING") {
				revokeDone <- revokeResult{status: response.StatusCode, body: string(body)}
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		revokeDone <- revokeResult{err: errors.New("GIS owner revoke did not finish after bounded exact retries")}
	}()

	var revokingAt time.Time
	var t0Observed time.Time
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		var bindingStatus string
		err = gisDB.QueryRow(ctx, `SELECT status,updated_at FROM player_bindings WHERE binding_id=$1`, fixture.bindingID).Scan(&bindingStatus, &revokingAt)
		require.NoError(t, err)
		if bindingStatus == "revoking" {
			t0Observed = time.Now()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.False(t, t0Observed.IsZero(), "GIS must durably commit active→revoking before permit drain")
	// A new Auth/GIS permit is forbidden once the durable GIS binding transition
	// has begun, even while permits issued before t0 are still draining.
	postRevokeOperationID := uuid.New()
	_, err = permitClient.Issue(ctx, gameprotocol.DeviceAuthority{AssertionJWS: assertion}, postRevokeOperationID, []byte("T16 must deny issue after durable revoking"))
	require.Error(t, err, "new Auth/GIS permit issue must fail after GIS durably enters revoking")
	var postRevokeAuthRows, postRevokeGISRows int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1`, postRevokeOperationID).Scan(&postRevokeAuthRows))
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT count(*) FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, postRevokeOperationID).Scan(&postRevokeGISRows))
	require.Zero(t, postRevokeAuthRows, "revoking binding must not persist a new Auth permit")
	require.Zero(t, postRevokeGISRows, "revoking binding must not persist a new GIS permit")
	var pendingStatusDuringRevoke string
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT status FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, pendingOperationID).Scan(&pendingStatusDuringRevoke))
	require.Equal(t, "issued", pendingStatusDuringRevoke, "preexisting pending GIS permit must still be draining before releasing Messaging")
	chatGuard.releaseBarrier()
	select {
	case err = <-messageResult:
		require.NoError(t, err, "the already-issued permit may finish during revoke drain")
	case <-ctx.Done():
		t.Fatal("pre-revoke Messaging operation did not reach a terminal result")
	}
	var revokeOutcome revokeResult
	select {
	case revokeOutcome = <-revokeDone:
	case <-ctx.Done():
		t.Fatal("GIS owner revoke did not return")
	}
	require.NoError(t, revokeOutcome.err, revokeOutcome.body)
	require.Equal(t, http.StatusOK, revokeOutcome.status, revokeOutcome.body)
	var commitStatus, expiryStatus string
	var commitAt, expiryAt time.Time
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT status,completion_at FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, revokeRace.operationID).Scan(&commitStatus, &commitAt))
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT status,completion_at FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, pendingOperationID).Scan(&expiryStatus, &expiryAt))
	require.Equal(t, "committed", commitStatus)
	require.Equal(t, "expired", expiryStatus, "GIS must expire the withheld permit after its lease and 500ms drain grace")
	require.False(t, expiryAt.Before(pendingExpiresAt.Add(500*time.Millisecond)))
	lastTerminalAt := commitAt
	if expiryAt.After(lastTerminalAt) {
		lastTerminalAt = expiryAt
	}
	gisDrain := lastTerminalAt.Sub(revokingAt)
	require.LessOrEqual(t, gisDrain, 4250*time.Millisecond, "GIS persisted revoking→last-terminal must meet the 4.25s limit")
	monotonicRequestToTerminal := time.Since(revokeStarted)
	t.Logf("T16 revoke timing: GIS durable revoking_at(updated_at)=%s, last_terminal_at=%s, GIS drain=%s, DELETE request-to-last-terminal monotonic=%s, request-to-observed-t0=%s",
		revokingAt.Format(time.RFC3339Nano), lastTerminalAt.Format(time.RFC3339Nano), gisDrain, monotonicRequestToTerminal, t0Observed.Sub(revokeStarted))

	// A retained exact receipt is read-only after revoke; a distinct operation
	// with the old binding proof is denied before a new permit or write.
	beforeRetainedRetry := countT16Permits(t, ctx, authDB, gisDB, fixture.bindingID, revokeRace.operationID)
	retained, err := apply(revokeRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, revokeHash)
	require.NoError(t, err)
	require.Equal(t, revokeRace.messageID.String(), retained.GetMessage().GetId())
	require.Equal(t, beforeRetainedRetry, countT16Permits(t, ctx, authDB, gisDB, fixture.bindingID, revokeRace.operationID), "receipt retry after revoke must not call Auth/GIS again")
	oldAuthorityOperation := fixture
	oldAuthorityOperation.operationID, oldAuthorityOperation.messageID = uuid.New(), newT16MessageID(t)
	oldAuthorityOperation.compactMessage = signT16Message(t, oldAuthorityOperation, fixture.deviceKey, time.Now().UTC())
	oldAuthorityRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: oldAuthorityOperation.compactMessage, DeviceAuthorityAssertion: assertion}
	oldAuthorityHash, err := principal.RequestHash(oldAuthorityRequest)
	require.NoError(t, err)
	_, err = apply(oldAuthorityRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, oldAuthorityHash)
	require.Error(t, err, "old-binding proof must not authorize a new operation after GIS revoke")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, gisDB, messagingDB, oldAuthorityOperation)

	// Model a durable owner-approved relink without provider credentials: GIS
	// commits a new binding ID/authority revision, and the current Auth grant,
	// consent, and device authority revision are updated before the new use.
	// The monotonic target starts after both service stores report the relink.
	restored, restoreT0 := restoreT16Binding(t, ctx, authDB, gisDB, fixture)
	restoredMapping := seedT16BindingChatMapping(ctx, gisDB, restored)
	require.NoError(t, restoredMapping)
	restoreOperation := restored
	restoreOperation.operationID, restoreOperation.messageID = uuid.New(), newT16MessageID(t)
	restoreOperation.compactMessage = signT16Message(t, restoreOperation, restored.deviceKey, time.Now().UTC())
	newAuthority, _ := requestT16AuthDeviceAuthority(t, ctx, restoreOperation)
	restoreRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: restoreOperation.compactMessage, DeviceAuthorityAssertion: newAuthority}
	restoreHash, err := principal.RequestHash(restoreRequest)
	require.NoError(t, err)
	restoreResponse, err := apply(restoreRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, restoreHash)
	require.NoError(t, err, "fresh Auth authority for the new binding should authorize use")
	restoredAt := time.Now()
	require.LessOrEqual(t, restoredAt.Sub(restoreT0), 5*time.Second, "provisional restore-to-fresh-use target")
	require.Equal(t, restoreOperation.messageID.String(), restoreResponse.GetMessage().GetId())
	t.Logf("T16 provisional restore timing: durable new binding+Auth grant t0=%s, first accepted fresh-authority use t1=%s, monotonic delta=%s",
		restoreT0.Format(time.RFC3339Nano), restoredAt.Format(time.RFC3339Nano), restoredAt.Sub(restoreT0))

	// The assertion issued before relink remains tied to the revoked binding ID
	// and cannot authorize a new write after the new binding is active.
	oldAfterRestore := restored
	oldAfterRestore.operationID, oldAfterRestore.messageID = uuid.New(), newT16MessageID(t)
	oldAfterRestore.compactMessage = signT16Message(t, oldAfterRestore, restored.deviceKey, time.Now().UTC())
	oldAfterRestoreRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: oldAfterRestore.compactMessage, DeviceAuthorityAssertion: assertion}
	oldAfterRestoreHash, err := principal.RequestHash(oldAfterRestoreRequest)
	require.NoError(t, err)
	_, err = apply(oldAfterRestoreRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, oldAfterRestoreHash)
	require.Error(t, err, "pre-relink Auth assertion for the old binding must remain denied")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, gisDB, messagingDB, oldAfterRestore)
}

type t16GISUnavailableFixture struct {
	SessionToken    string    `json:"session_token"`
	Proof           string    `json:"proof"`
	DeviceID        uuid.UUID `json:"device_id"`
	AppID           uuid.UUID `json:"app_id"`
	EnvID           uuid.UUID `json:"environment_id"`
	BindingID       uuid.UUID `json:"binding_id"`
	ChallengeID     uuid.UUID `json:"challenge_id"`
	ExchangeID      uuid.UUID `json:"exchange_id"`
	SourceAccountID uuid.UUID `json:"source_account_id"`
	TargetAccountID uuid.UUID `json:"target_account_id"`
	OwnerProfileID  uuid.UUID `json:"owner_profile_id"`
	TargetProfileID uuid.UUID `json:"target_profile_id"`
	ChatID          uuid.UUID `json:"chat_id"`
}

// TestT16PrepareAuthGISUnavailableFixture persists one synthetic Auth identity
// before the workflow stops GIS. A later test process uses the same live Auth
// endpoint and disposable databases, without substituting either service.
func TestT16PrepareAuthGISUnavailableFixture(t *testing.T) {
	if os.Getenv("T16_ACCEPTANCE") != "1" {
		t.Skip("requires the hosted T16 Compose fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	authDB := t16Pool(t, ctx, "T16_AUTH_DATABASE_URL")
	gisDB := t16Pool(t, ctx, "T16_GIS_DATABASE_URL")
	userDB := t16Pool(t, ctx, "T16_USER_DATABASE_URL")
	chatDB := t16Pool(t, ctx, "T16_CHAT_DATABASE_URL")
	fixture := seedT16AcceptFixture(t, ctx, authDB, gisDB, userDB, chatDB)
	_, err := gisDB.Exec(ctx, `DELETE FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND chat_id=$3`, fixture.appID, fixture.envID, fixture.chatID)
	require.NoError(t, err)
	prepared := t16GISUnavailableFixture{SessionToken: fixture.sessionToken,
		Proof: t16AuthDeviceAuthorityProof(t, fixture, fixture.deviceID), DeviceID: fixture.deviceID,
		AppID: fixture.appID, EnvID: fixture.envID, BindingID: fixture.bindingID, ChallengeID: fixture.challengeID,
		ExchangeID: fixture.bindingOperationID, SourceAccountID: fixture.sourceAccountID, TargetAccountID: fixture.targetAccountID,
		OwnerProfileID: fixture.ownerProfileID, TargetProfileID: fixture.targetProfileID, ChatID: fixture.chatID}
	data, err := json.Marshal(prepared)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(requiredT16Env(t, "T16_FIXTURE_DIR"), "auth-gis-unavailable.json"), data, 0o600)
	require.NoError(t, err)
}

// TestT16AuthDeviceAuthorityRejectsUnavailableGIS runs after the workflow has
// stopped only GIS. Auth remains the production container and must fail closed
// before it persists an authority receipt when its real GIS client cannot dial.
func TestT16AuthDeviceAuthorityRejectsUnavailableGIS(t *testing.T) {
	if os.Getenv("T16_ACCEPTANCE") != "1" {
		t.Skip("requires the hosted T16 Compose fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	authDB := t16Pool(t, ctx, "T16_AUTH_DATABASE_URL")
	fixturePath := filepath.Join(requiredT16Env(t, "T16_FIXTURE_DIR"), "auth-gis-unavailable.json")
	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	var fixture t16GISUnavailableFixture
	require.NoError(t, json.Unmarshal(data, &fixture))
	t.Cleanup(func() { cleanupT16GISUnavailableFixture(t, fixture, fixturePath) })
	beforeNonces := countT16GISWorkloadNonces(t, ctx)
	beforeIssues := countT16DeviceAuthorityIssues(t, ctx, authDB, fixture.DeviceID)
	status, body := postT16AuthDeviceAuthorityBearerResponse(t, ctx, fixture.SessionToken, fixture.Proof)
	require.Equal(t, http.StatusUnauthorized, status, body)
	require.Equal(t, beforeNonces, countT16GISWorkloadNonces(t, ctx), "unavailable GIS must not consume a workload nonce")
	require.Equal(t, beforeIssues, countT16DeviceAuthorityIssues(t, ctx, authDB, fixture.DeviceID), "Auth must not persist an assertion when GIS is unavailable")
}

func cleanupT16GISUnavailableFixture(t *testing.T, fixture t16GISUnavailableFixture, fixturePath string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	authDB := t16Pool(t, ctx, "T16_AUTH_DATABASE_URL")
	gisDB := t16Pool(t, ctx, "T16_GIS_DATABASE_URL")
	userDB := t16Pool(t, ctx, "T16_USER_DATABASE_URL")
	chatDB := t16Pool(t, ctx, "T16_CHAT_DATABASE_URL")
	defer authDB.Close()
	defer gisDB.Close()
	defer userDB.Close()
	defer chatDB.Close()
	_, err := chatDB.Exec(ctx, `DELETE FROM chat_members WHERE chat_id=$1`, fixture.ChatID)
	require.NoError(t, err)
	_, err = chatDB.Exec(ctx, `DELETE FROM chats WHERE id=$1`, fixture.ChatID)
	require.NoError(t, err)
	_, err = userDB.Exec(ctx, `DELETE FROM profiles WHERE id IN ($1,$2)`, fixture.OwnerProfileID, fixture.TargetProfileID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `DELETE FROM game_resource_binding_chats WHERE binding_id=$1`, fixture.BindingID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `DELETE FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2`, fixture.AppID, fixture.EnvID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `DELETE FROM player_binding_exchange_operations WHERE operation_id=$1`, fixture.ExchangeID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `DELETE FROM player_binding_challenges WHERE challenge_id=$1`, fixture.ChallengeID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `DELETE FROM player_bindings WHERE binding_id=$1`, fixture.BindingID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `DELETE FROM environments WHERE id=$1`, fixture.EnvID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `DELETE FROM applications WHERE id=$1`, fixture.AppID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM sdk_device_authority_issues WHERE device_id=$1`, fixture.DeviceID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM sdk_game_message_grants WHERE binding_id=$1`, fixture.BindingID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM sdk_linked_sessions WHERE request_id IN (SELECT request_id FROM sdk_authorizations WHERE game_binding_id=$1)`, fixture.BindingID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM sdk_authorizations WHERE game_binding_id=$1`, fixture.BindingID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM sdk_sessions WHERE device_id=$1`, fixture.DeviceID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM sdk_device_keys WHERE device_id=$1`, fixture.DeviceID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM sdk_devices WHERE device_id=$1`, fixture.DeviceID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM sdk_identities WHERE application_id=$1 AND environment_id=$2`, fixture.AppID, fixture.EnvID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `DELETE FROM accounts WHERE id IN ($1,$2)`, fixture.SourceAccountID, fixture.TargetAccountID)
	require.NoError(t, err)
	require.NoError(t, os.Remove(fixturePath))
}

func assertNoT16ExecutionSideEffects(t *testing.T, ctx context.Context, authDB, gisDB, messagingDB *pgxpool.Pool, fixture t16AcceptFixture) {
	t.Helper()
	var authPermits, gisPermits, messages, completions int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1`, fixture.operationID).Scan(&authPermits))
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT count(*) FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, fixture.operationID).Scan(&gisPermits))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, fixture.messageID).Scan(&messages))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1`, fixture.operationID).Scan(&completions))
	require.Zero(t, authPermits, "denial must precede Auth permit issuance")
	require.Zero(t, gisPermits, "denial must precede GIS permit issuance")
	require.Zero(t, messages, "denial must precede message commit")
	require.Zero(t, completions, "mapping denial must not create completion")
}

func countT16Permits(t *testing.T, ctx context.Context, authDB, gisDB *pgxpool.Pool, bindingID, operationID uuid.UUID) int {
	t.Helper()
	var authPermits, gisPermits int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1`, operationID).Scan(&authPermits))
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT count(*) FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, bindingID, operationID).Scan(&gisPermits))
	require.Equal(t, authPermits, gisPermits, "Auth and GIS must retain the same permit count")
	return authPermits
}

type t16BlockingChatGuard struct {
	ChatGuard
	armed   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

// t16DiagnosticGameMessageProcessor delegates to the real processor and logs
// only the underlying error for the one assertion selected by the acceptance
// test. It adds no authorization behavior or substitute provider seam.
type t16DiagnosticGameMessageProcessor struct {
	delegate      GameMessageProcessor
	t             *testing.T
	probeAuthJWKS func()
	mu            sync.Mutex
	watch         string
}

type t16DiagnosticResourceMappingAuthority struct {
	delegate interface {
		AuthorizeAppBindingChat(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error
	}
	runtimeDB *pgxpool.Pool
	t         *testing.T
	calls     atomic.Int64
}

func (d *t16DiagnosticResourceMappingAuthority) AuthorizeAppBindingChat(
	ctx context.Context, authority gameprotocol.DeviceAuthority, message gameprotocol.Message,
) error {
	call := d.calls.Add(1)
	err := d.delegate.AuthorizeAppBindingChat(ctx, authority, message)
	d.t.Logf("T16 GIS mapping call=%d allowed=%t", call, err == nil)
	if err != nil {
		var runtimeRevision int64
		runtimeErr := d.runtimeDB.QueryRow(ctx, `SELECT m.mapping_revision
			FROM game_resource_binding_chats r
			JOIN applications a ON a.id=r.application_id AND a.status IN ('sandbox','active')
			JOIN environments e ON e.id=r.environment_id AND e.application_id=r.application_id AND e.status='active'
			JOIN player_bindings b ON b.application_id=r.application_id AND b.environment_id=r.environment_id
				AND b.binding_id=r.binding_id AND b.status='active'
			JOIN game_resource_mappings m ON m.application_id=r.application_id AND m.environment_id=r.environment_id
				AND m.resource_kind='chat' AND m.chat_id=r.chat_id AND m.status='active'
			WHERE r.application_id=$1 AND r.environment_id=$2 AND r.binding_id=$3 AND r.chat_id=$4
			AND r.status='active' AND r.lease_expires_at > clock_timestamp()`,
			authority.ApplicationID, authority.EnvironmentID, authority.BindingID, message.ChatID).
			Scan(&runtimeRevision)
		d.t.Logf("T16 diagnostic: real GIS mapping client rejected call=%d tuple app=%s env=%s binding=%s messageApp=%s messageEnv=%s messageBinding=%s chat=%s: %v",
			call,
			authority.ApplicationID, authority.EnvironmentID, authority.BindingID,
			message.ApplicationID, message.EnvironmentID, message.BindingID, message.ChatID, err)
		d.t.Logf("T16 diagnostic: actual request tuple under GIS runtime role revision=%d query_error=%v",
			runtimeRevision, runtimeErr)
	}
	return err
}

func (p *t16DiagnosticGameMessageProcessor) watchAssertion(assertion string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.watch = assertion
}

func (p *t16DiagnosticGameMessageProcessor) clearWatchedAssertion() {
	p.watchAssertion("")
}

func (p *t16DiagnosticGameMessageProcessor) ProcessGameMessage(ctx context.Context, compact, authority string) (*store.MessageRow, error) {
	row, err := p.delegate.ProcessGameMessage(ctx, compact, authority)
	if err != nil {
		p.mu.Lock()
		watched := authority != "" && authority == p.watch
		p.mu.Unlock()
		if watched {
			p.t.Logf("T16 diagnostic: real game message processor rejected selected request: %v", err)
			if strings.Contains(err.Error(), "auth principal JWKS request failed") && p.probeAuthJWKS != nil {
				p.probeAuthJWKS()
			}
		}
	}
	return row, err
}

func probeT16AuthPrincipalJWKS(t *testing.T, ctx context.Context, baseURL, certFile, keyFile, caFile string) {
	t.Helper()
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Logf("T16 diagnostic: Auth JWKS mTLS probe could not load Messaging identity: %v", err)
		return
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Logf("T16 diagnostic: Auth JWKS mTLS probe client certificate is invalid: %v", err)
		return
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		t.Logf("T16 diagnostic: Auth JWKS mTLS probe could not read CA: %v", err)
		return
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Log("T16 diagnostic: Auth JWKS mTLS probe CA contains no certificates")
		return
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots,
		Certificates: []tls.Certificate{certificate},
		GetClientCertificate: func(request *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			issuerMatch := false
			for _, acceptableCA := range request.AcceptableCAs {
				if string(acceptableCA) == string(leaf.RawIssuer) {
					issuerMatch = true
					break
				}
			}
			t.Logf("T16 diagnostic: Auth requested client certificate; acceptable_ca_count=%d issuer_match=%t",
				len(request.AcceptableCAs), issuerMatch)
			return &certificate, nil
		}}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/auth/.well-known/principal-jwks.json", nil)
	if err != nil {
		t.Logf("T16 diagnostic: Auth JWKS mTLS probe request construction failed: %v", err)
		return
	}
	response, err := client.Do(request)
	if err != nil {
		t.Logf("T16 diagnostic: Auth JWKS mTLS probe transport error: %T: %v", err, err)
		return
	}
	defer response.Body.Close()
	t.Logf("T16 diagnostic: Auth JWKS mTLS probe returned HTTP %d", response.StatusCode)
}

func (g *t16BlockingChatGuard) arm() { g.armed.Store(true) }

func (g *t16BlockingChatGuard) releaseBarrier() { close(g.release) }

func (g *t16BlockingChatGuard) EnsureMember(ctx context.Context, chatID, profileID uuid.UUID) error {
	if g.armed.CompareAndSwap(true, false) {
		close(g.entered)
		select {
		case <-g.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return g.ChatGuard.EnsureMember(ctx, chatID, profileID)
}

func t16OwnerAccessToken(t *testing.T, fixture t16AcceptFixture) string {
	t.Helper()
	keyPath := requiredT16Env(t, "T16_AUTH_JWT_PRIVATE_KEY_FILE")
	keyPEM, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	key, err := parseT16RSAKeyPEM(keyPEM)
	require.NoError(t, err)
	now := time.Now().UTC()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "local-key", "typ": "JWT"})
	require.NoError(t, err)
	claims, err := json.Marshal(map[string]any{"iss": "voice-auth", "aud": "voice-client", "sub": fixture.sourceAccountID.String(),
		"user_id": fixture.sourceAccountID.String(), "profile_id": fixture.ownerProfileID.String(), "roles": []string{},
		"subscription_tier": "free", "account_type": "regular", "session_epoch": 1, "jti": uuid.NewString(),
		"iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix()})
	require.NoError(t, err)
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func parseT16RSAKeyPEM(keyPEM []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, errors.New("missing PEM private key")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("PEM private key is not RSA")
		}
		return key, nil
	default:
		return nil, errors.New("unsupported PEM private key type")
	}
}

func TestT16RSAKeyPEMFormats(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	parsedPKCS1, err := parseT16RSAKeyPEM(pkcs1)
	require.NoError(t, err)
	require.Equal(t, key.N, parsedPKCS1.N)

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	pkcs8 := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})
	parsedPKCS8, err := parseT16RSAKeyPEM(pkcs8)
	require.NoError(t, err)
	require.Equal(t, key.N, parsedPKCS8.N)

	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecdsaPKCS8Bytes, err := x509.MarshalPKCS8PrivateKey(ecdsaKey)
	require.NoError(t, err)
	ecdsaPKCS8 := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecdsaPKCS8Bytes})
	_, err = parseT16RSAKeyPEM(ecdsaPKCS8)
	require.Error(t, err)
}

func seedT16ResourceMapping(ctx context.Context, gisDB *pgxpool.Pool, fixture t16AcceptFixture) error {
	_, err := gisDB.Exec(ctx, `INSERT INTO game_resource_mappings(mapping_id,application_id,environment_id,external_key,resource_kind,resource_id,chat_id,chat_operation_id,chat_request_hash,status,mapping_revision)
VALUES($1,$2,$3,'t16-synthetic-chat','chat',$4,$4,$5,'sha256:`+strings.Repeat("e", 64)+`','active',1) ON CONFLICT (application_id,environment_id,external_key) DO UPDATE SET resource_id=EXCLUDED.resource_id,chat_id=EXCLUDED.chat_id,status='active',mapping_revision=game_resource_mappings.mapping_revision+1`, uuid.New(), fixture.appID, fixture.envID, fixture.chatID, uuid.New())
	return err
}

func seedT16WrongScopeResourceMapping(ctx context.Context, gisDB *pgxpool.Pool, fixture t16AcceptFixture, wrongApplication bool) (uuid.UUID, uuid.UUID, error) {
	appID := fixture.appID
	if wrongApplication {
		appID = uuid.New()
		if _, err := gisDB.Exec(ctx, `INSERT INTO applications(id,owner_account_id,name,status,revision) VALUES($1,$2,'T16 wrong-scope synthetic','sandbox',1)`, appID, fixture.sourceAccountID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
	}
	envID := uuid.New()
	kind := "sandbox"
	if !wrongApplication {
		// The fixture application's sandbox environment already exists.
		kind = "production"
	}
	if _, err := gisDB.Exec(ctx, `INSERT INTO environments(id,application_id,kind,status,provider_policy,redirect_uris,allowed_origins,revision)
	VALUES($1,$2,$3,'active',$4,'["https://voice.test/callback"]','[]',1)`, envID, appID, kind,
		`{"providers":["google"],"player_scopes":["game.chat.send"]}`); err != nil {
		return appID, uuid.Nil, err
	}
	_, err := gisDB.Exec(ctx, `INSERT INTO game_resource_mappings(mapping_id,application_id,environment_id,external_key,resource_kind,resource_id,chat_id,chat_operation_id,chat_request_hash,status,mapping_revision)
VALUES($1,$2,$3,'t16-wrong-scope','chat',$4,$4,$5,'sha256:`+strings.Repeat("e", 64)+`','active',1)`, uuid.New(), appID, envID, fixture.chatID, uuid.New())
	return appID, envID, err
}

func seedT16BindingChatMapping(ctx context.Context, gisDB *pgxpool.Pool, fixture t16AcceptFixture) error {
	_, err := gisDB.Exec(ctx, `INSERT INTO game_resource_binding_chats(application_id,environment_id,binding_id,chat_id,roster_revision,status,lease_expires_at)
VALUES($1,$2,$3,$4,1,'active',now()+interval '1 hour')`, fixture.appID, fixture.envID, fixture.bindingID, fixture.chatID)
	return err
}

func restoreT16Binding(t *testing.T, ctx context.Context, authDB, gisDB *pgxpool.Pool, prior t16AcceptFixture) (t16AcceptFixture, time.Time) {
	t.Helper()
	restored := prior
	restored.bindingID = uuid.New()
	restored.bindingOperationID = uuid.New()
	authorityRevision := int64(2)
	_, err := gisDB.Exec(ctx, `INSERT INTO player_bindings(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision)
VALUES($1,$2,$3,'google','hmac-sha256-v1:t16-restored:`+strings.Repeat("9", 64)+`',$4,$5,$6,$7,'active',$8)`, restored.bindingID, restored.appID, restored.envID,
		restored.sourceAccountID, restored.sourceActorID, restored.targetProfileID, restored.deviceID, authorityRevision)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `UPDATE sdk_authorizations SET game_binding_id=$1,game_binding_status='active',game_binding_authority_revision=$2,game_binding_operation_id=$3
WHERE game_binding_id=$4 AND source_account_id=$5 AND application_id=$6 AND environment_id=$7`, restored.bindingID, authorityRevision, restored.bindingOperationID,
		prior.bindingID, prior.sourceAccountID, prior.appID, prior.envID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `UPDATE sdk_game_message_grants SET binding_id=$1,authority_revision=$2,status='active',updated_at=now()
WHERE binding_id=$3 AND application_id=$4 AND environment_id=$5`, restored.bindingID, authorityRevision, prior.bindingID, restored.appID, restored.envID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `UPDATE sdk_devices SET authority_revision=$1 WHERE device_id=$2 AND account_id=$3`, authorityRevision, restored.deviceID, restored.sourceAccountID)
	require.NoError(t, err)
	var relinkCommittedAt time.Time
	err = gisDB.QueryRow(ctx, `UPDATE player_bindings SET updated_at=clock_timestamp() WHERE binding_id=$1 RETURNING updated_at`, restored.bindingID).Scan(&relinkCommittedAt)
	require.NoError(t, err)
	return restored, relinkCommittedAt
}

type t16AcceptFixture struct {
	appID, envID, sourceAccountID, sourceActorID, bindingID, deviceID, keyID uuid.UUID
	targetAccountID, targetProfileID, chatID, messageID, operationID         uuid.UUID
	ownerProfileID, challengeID, bindingOperationID                          uuid.UUID
	authorityRevision                                                        int64
	sessionToken                                                             string
	deviceKey                                                                *ecdsa.PrivateKey
	compactMessage                                                           string
}

func seedT16AcceptFixture(t *testing.T, ctx context.Context, authDB, gisDB, userDB, chatDB *pgxpool.Pool) t16AcceptFixture {
	t.Helper()
	fixture := t16AcceptFixture{appID: uuid.MustParse("00000000-0000-4000-8000-000000000016"),
		envID: uuid.MustParse("00000000-0000-4000-8000-000000000017"), sourceAccountID: uuid.New(), sourceActorID: uuid.New(),
		bindingID: uuid.New(), deviceID: uuid.New(), keyID: uuid.New(), targetAccountID: uuid.New(), targetProfileID: uuid.New(),
		chatID: uuid.New(), messageID: newT16MessageID(t), operationID: uuid.New(), ownerProfileID: uuid.New(), challengeID: uuid.New(), bindingOperationID: uuid.New()}
	fixture.authorityRevision = 1
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	fixture.deviceKey = deviceKey
	publicJWK, thumbprint := t16PublicJWK(t, deviceKey)
	fixture.sessionToken = base64.RawURLEncoding.EncodeToString(randomT16Bytes(t, 32))
	accessHash := t16SHA256(fixture.sessionToken)
	linkedToken := base64.RawURLEncoding.EncodeToString(randomT16Bytes(t, 32))
	linkedHash := t16SHA256(linkedToken)
	now := time.Now().UTC().Truncate(time.Millisecond)
	keyExpiry := now.Add(90 * 24 * time.Hour)
	requestID := uuid.New()
	challengeID, bindingOperationID := fixture.challengeID, fixture.bindingOperationID
	grantID := uuid.New()
	profileRevision := int64(1)

	for _, accountID := range []uuid.UUID{fixture.sourceAccountID, fixture.targetAccountID} {
		_, err = authDB.Exec(ctx, `INSERT INTO accounts(id,password_hash,type,status) VALUES($1,'t16-synthetic','regular','active')`, accountID)
		require.NoError(t, err)
	}
	for _, record := range []struct {
		account, profile uuid.UUID
		username         string
	}{
		{fixture.sourceAccountID, fixture.ownerProfileID, "t16owner" + fixture.sourceAccountID.String()[:8]},
		{fixture.targetAccountID, fixture.targetProfileID, "t16target" + fixture.targetAccountID.String()[:8]},
	} {
		_, err = userDB.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,$3,'0001','T16 synthetic')`,
			record.profile, record.account, record.username)
		require.NoError(t, err)
	}
	_, err = authDB.Exec(ctx, `INSERT INTO sdk_identities(account_id,actor_id,application_id,environment_id,issuer,provider_subject,ownership_generation,status,created_at)
VALUES($1,$2,$3,$4,'google',$6,1,'active',$5)`, fixture.sourceAccountID, fixture.sourceActorID, fixture.appID, fixture.envID, now,
		"t16-subject-"+fixture.sourceAccountID.String())
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `INSERT INTO sdk_devices(device_id,account_id,thumbprint,public_jwk,authority_revision) VALUES($1,$2,$3,$4,1)`,
		fixture.deviceID, fixture.sourceAccountID, thumbprint, string(publicJWK))
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `INSERT INTO sdk_device_keys(key_id,device_id,application_id,environment_id,public_jwk,key_thumbprint,generation,not_before,not_after,status)
VALUES($1,$2,$3,$4,$5,$6,1,$7,$8,'active')`, fixture.keyID, fixture.deviceID, fixture.appID, fixture.envID, string(publicJWK), thumbprint, now.Add(-time.Second), keyExpiry)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `INSERT INTO sdk_sessions(token_hash,account_id,device_id,ownership_generation,game_subject,expires_at)
VALUES($1,$2,$3,1,'t16-synthetic-player',$4)`, accessHash, fixture.sourceAccountID, fixture.deviceID, now.Add(time.Hour))
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `INSERT INTO sdk_authorizations(request_id,source_account_id,device_id,source_session_hash,source_generation,
application_id,environment_id,idempotency_key,request_hash,redirect_uri,code_challenge,client_state,scopes,policy_revision,display_name,
expires_at,target_account_id,target_profile_id,target_epoch,profile_revision,consumed_at,game_binding_id,game_binding_status,
game_binding_authority_revision,game_binding_challenge_id,game_binding_operation_id,game_binding_intent,game_binding_challenge_nonce,
game_binding_consent_revision)
VALUES($1,$2,$3,$4,1,$5,$6,$7,repeat('a',64),'https://voice.test/callback',repeat('b',43),'t16','game.chat.send',1,
'T16 synthetic',$8,$9,$10,1,$11,$12,$13,'active',1,$14,$15,true,repeat('c',43),1)`,
		requestID, fixture.sourceAccountID, fixture.deviceID, accessHash, fixture.appID, fixture.envID, uuid.New(), now.Add(time.Hour),
		fixture.targetAccountID, fixture.targetProfileID, profileRevision, now, fixture.bindingID, challengeID, bindingOperationID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `INSERT INTO sdk_linked_sessions(token_hash,request_id,expires_at) VALUES($1,$2,$3)`, linkedHash, requestID, now.Add(time.Hour))
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `UPDATE sdk_authorizations SET game_binding_consent_revision=(SELECT consent_revision FROM sdk_linked_sessions WHERE request_id=$1) WHERE request_id=$1`, requestID)
	require.NoError(t, err)
	_, err = authDB.Exec(ctx, `INSERT INTO sdk_game_message_grants(grant_id,authorization_request_id,application_id,environment_id,target_account_id,target_profile_id,
target_epoch,binding_id,consent_revision,scopes,policy_revision,profile_revision,status,authority_revision,created_at,updated_at)
SELECT $1,$2,$3,$4,$5,$6,1,$7,consent_revision,'game.chat.send',1,$8,'active',1,$9,$9 FROM sdk_linked_sessions WHERE request_id=$2`,
		grantID, requestID, fixture.appID, fixture.envID, fixture.targetAccountID, fixture.targetProfileID, fixture.bindingID, profileRevision, now)
	require.NoError(t, err)

	_, err = gisDB.Exec(ctx, `INSERT INTO applications(id,owner_account_id,name,status,revision) VALUES($1,$2,'T16 synthetic','sandbox',1)`, fixture.appID, fixture.sourceAccountID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `INSERT INTO environments(id,application_id,kind,status,provider_policy,redirect_uris,allowed_origins,revision)
VALUES($1,$2,'sandbox','active',$3,'["https://voice.test/callback"]','[]',1)`, fixture.envID, fixture.appID,
		`{"providers":["google"],"player_scopes":["game.chat.send"]}`)
	require.NoError(t, err)
	digest := strings.Repeat("d", 64)
	_, err = gisDB.Exec(ctx, `INSERT INTO player_bindings(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision)
VALUES($1,$2,$3,'google','hmac-sha256-v1:t16:`+digest+`',$4,$5,$6,$7,'active',1)`, fixture.bindingID, fixture.appID, fixture.envID,
		fixture.sourceAccountID, fixture.sourceActorID, fixture.targetProfileID, fixture.deviceID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `INSERT INTO player_binding_challenges(challenge_id,nonce,application_id,environment_id,provider,redirect_uri_sha256,pkce_challenge,device_key_id,device_key_thumbprint,operation_id,expires_at,status,consumed_at,request_sha256)
VALUES($1,$2,$3,$4,'google',repeat('1',64),repeat('2',43),$5,$6,$7,now()+interval '1 hour','consumed',now(),repeat('3',64))`, fixture.challengeID, uuid.NewString(), fixture.appID, fixture.envID, fixture.keyID, thumbprint, fixture.bindingOperationID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `INSERT INTO player_binding_exchange_operations(operation_id,challenge_id,request_sha256,status,handoff_jws,auth_claim_id,assertion_jti,binding_id)
VALUES($1,$2,repeat('3',64),'created','t16-synthetic-handoff',$3,$4,$5)`, fixture.bindingOperationID, fixture.challengeID, uuid.New(), uuid.New(), fixture.bindingID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `INSERT INTO game_resource_mappings(mapping_id,application_id,environment_id,external_key,resource_kind,resource_id,chat_id,chat_operation_id,chat_request_hash,status,mapping_revision)
VALUES($1,$2,$3,'t16-synthetic-chat','chat',$4,$4,$5,'sha256:`+strings.Repeat("e", 64)+`','active',1)`, uuid.New(), fixture.appID, fixture.envID, fixture.chatID, uuid.New())
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `INSERT INTO game_resource_binding_chats(application_id,environment_id,binding_id,chat_id,roster_revision,status,lease_expires_at)
VALUES($1,$2,$3,$4,1,'active',$5)`, fixture.appID, fixture.envID, fixture.bindingID, fixture.chatID, now.Add(time.Hour))
	require.NoError(t, err)
	_, err = chatDB.Exec(ctx, `INSERT INTO chats(id,type,creator_profile_id) VALUES($1,'dm',$2)`, fixture.chatID, fixture.targetProfileID)
	require.NoError(t, err)
	_, err = chatDB.Exec(ctx, `INSERT INTO chat_members(chat_id,profile_id,role) VALUES($1,$2,'member')`, fixture.chatID, fixture.targetProfileID)
	require.NoError(t, err)
	fixture.compactMessage = signT16Message(t, fixture, deviceKey, now)
	return fixture
}

// Probe the same Auth JVM that observed the GIS outage. Docker health alone does
// not establish recovery of its HTTP client (including cached DNS failures).
// This enrollment challenge cannot mint a session or device-authority assertion.
func waitT16AuthGISRecovery(t *testing.T, ctx context.Context, authDB *pgxpool.Pool, fixture t16AcceptFixture) {
	t.Helper()
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	jwk, _ := t16PublicJWK(t, fixture.deviceKey)
	id, err := probeT16AuthGISReadiness(probeCtx, &http.Client{Timeout: 2 * time.Second}, requiredT16Env(t, "T16_AUTH_BASE_URL"), fixture.appID, fixture.envID, string(jwk), 250*time.Millisecond)
	require.NoError(t, err)
	// Delete only the probe's enrollment challenge before nonce/receipt baselines.
	result, err := authDB.Exec(ctx, `DELETE FROM sdk_challenges WHERE challenge_id=$1 AND application_id=$2 AND environment_id=$3`, id, fixture.appID, fixture.envID)
	require.NoError(t, err)
	require.EqualValues(t, 1, result.RowsAffected())
}

func probeT16AuthGISReadiness(ctx context.Context, client *http.Client, baseURL string, application, environment uuid.UUID, jwk string, interval time.Duration) (uuid.UUID, error) {
	body, err := json.Marshal(map[string]string{"applicationId": application.String(), "environmentId": environment.String(), "devicePublicJwk": jwk})
	if err != nil {
		return uuid.Nil, errors.New("Auth GIS readiness request encoding failed")
	}
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/auth/sdk/challenges", strings.NewReader(string(body)))
		if err != nil {
			return uuid.Nil, errors.New("Auth GIS readiness request creation failed")
		}
		request.Header.Set("Content-Type", "application/json")
		response, callErr := client.Do(request)
		if callErr == nil {
			data, readErr := io.ReadAll(io.LimitReader(response.Body, 16*1024))
			_ = response.Body.Close()
			if readErr != nil {
				return uuid.Nil, errors.New("Auth GIS readiness response read failed")
			}
			if response.StatusCode == http.StatusOK {
				var challenge struct {
					ChallengeID uuid.UUID `json:"challengeId"`
				}
				if json.Unmarshal(data, &challenge) != nil || challenge.ChallengeID == uuid.Nil {
					return uuid.Nil, errors.New("Auth GIS readiness response has no valid challenge")
				}
				return challenge.ChallengeID, nil
			}
			if response.StatusCode != http.StatusUnauthorized && response.StatusCode != http.StatusServiceUnavailable {
				return uuid.Nil, fmt.Errorf("Auth GIS readiness unexpected HTTP status %d", response.StatusCode)
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return uuid.Nil, errors.New("Auth GIS readiness deadline reached")
		case <-timer.C:
		}
	}
}

func requestT16AuthDeviceAuthority(t *testing.T, ctx context.Context, fixture t16AcceptFixture) (string, string) {
	t.Helper()
	proof := t16AuthDeviceAuthorityProof(t, fixture, fixture.deviceID)
	return postT16AuthDeviceAuthority(t, ctx, fixture, proof), proof
}

func t16AuthDeviceAuthorityProof(t *testing.T, fixture t16AcceptFixture, claimedDeviceID uuid.UUID) string {
	t.Helper()
	now := time.Now().UTC()
	claims := map[string]any{"version": 1, "audience": "voice.game-message", "request_id": uuid.NewString(),
		"application_id": fixture.appID.String(), "environment_id": fixture.envID.String(), "device_id": claimedDeviceID.String(), "issued_at": now.UnixMilli()}
	body, err := json.Marshal(claims)
	require.NoError(t, err)
	header, err := json.Marshal(map[string]string{"alg": "ES256", "kid": fixture.keyID.String(), "typ": "voice.game-device-authority-request+jws"})
	require.NoError(t, err)
	return signT16ES256(t, fixture.deviceKey, header, body)
}

func replayT16AuthDeviceAuthority(t *testing.T, ctx context.Context, fixture t16AcceptFixture, proof string) string {
	t.Helper()
	return postT16AuthDeviceAuthority(t, ctx, fixture, proof)
}

func postT16AuthDeviceAuthority(t *testing.T, ctx context.Context, fixture t16AcceptFixture, proof string) string {
	t.Helper()
	status, body := postT16AuthDeviceAuthorityResponse(t, ctx, fixture, proof)
	require.Equal(t, http.StatusOK, status, body)
	return body
}

func postT16AuthDeviceAuthorityResponse(t *testing.T, ctx context.Context, fixture t16AcceptFixture, proof string) (int, string) {
	t.Helper()
	return postT16AuthDeviceAuthorityBearerResponse(t, ctx, fixture.sessionToken, proof)
}

func postT16AuthDeviceAuthorityBearerResponse(t *testing.T, ctx context.Context, sessionToken, proof string) (int, string) {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requiredT16Env(t, "T16_AUTH_BASE_URL")+"/api/v1/auth/sdk/device-authority", strings.NewReader(proof))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+sessionToken)
	request.Header.Set("Content-Type", "text/plain")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024))
	require.NoError(t, err)
	return response.StatusCode, string(data)
}

func assertT16AuthDeviceAuthorityFailsClosed(t *testing.T, ctx context.Context, authDB, gisDB *pgxpool.Pool, fixture t16AcceptFixture) {
	t.Helper()
	assertDenied := func(t *testing.T, name, proof string, expectedGISNonceDelta int) {
		t.Helper()
		beforeNonces := countT16GISWorkloadNonces(t, ctx)
		beforeIssues := countT16DeviceAuthorityIssues(t, ctx, authDB, fixture.deviceID)
		status, body := postT16AuthDeviceAuthorityResponse(t, ctx, fixture, proof)
		require.Equal(t, http.StatusUnauthorized, status, "%s: %s", name, body)
		require.Equal(t, beforeNonces+expectedGISNonceDelta, countT16GISWorkloadNonces(t, ctx), "%s GIS WorkloadProof calls", name)
		require.Equal(t, beforeIssues, countT16DeviceAuthorityIssues(t, ctx, authDB, fixture.deviceID), "%s must not persist an Auth authority receipt", name)
	}

	t.Run("device-authority rejects a valid proof bound to a different device", func(t *testing.T) {
		assertDenied(t, "device mismatch", t16AuthDeviceAuthorityProof(t, fixture, uuid.New()), 0)
	})
	t.Run("device-authority rejects a bad signed proof before GIS", func(t *testing.T) {
		proof := t16AuthDeviceAuthorityProof(t, fixture, fixture.deviceID)
		parts := strings.Split(proof, ".")
		require.Len(t, parts, 3)
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		require.NoError(t, err)
		signature[0] ^= 0x80
		parts[2] = base64.RawURLEncoding.EncodeToString(signature)
		assertDenied(t, "bad signature", strings.Join(parts, "."), 1)
	})
	t.Run("device-authority rejects when there is no active Auth grant", func(t *testing.T) {
		commandTag, err := authDB.Exec(ctx, `UPDATE sdk_game_message_grants SET status='revoked',authority_revision=authority_revision+1
WHERE binding_id=$1 AND status='active'`, fixture.bindingID)
		require.NoError(t, err)
		require.EqualValues(t, 1, commandTag.RowsAffected())
		t.Cleanup(func() {
			_, restoreErr := authDB.Exec(ctx, `UPDATE sdk_game_message_grants SET status='active',authority_revision=1
WHERE binding_id=$1 AND status='revoked'`, fixture.bindingID)
			require.NoError(t, restoreErr)
		})
		assertDenied(t, "no active grant", t16AuthDeviceAuthorityProof(t, fixture, fixture.deviceID), 1)
	})
	t.Run("device-authority rejects an ambiguous active Auth grant", func(t *testing.T) {
		requestID, bindingID, linkedHash, targetProfileID := uuid.New(), uuid.New(), randomT16TokenHash(t), uuid.New()
		t.Cleanup(func() {
			_, cleanupErr := gisDB.Exec(ctx, `DELETE FROM player_bindings WHERE binding_id=$1`, bindingID)
			require.NoError(t, cleanupErr)
			_, cleanupErr = authDB.Exec(ctx, `DELETE FROM sdk_game_message_grants WHERE binding_id=$1`, bindingID)
			require.NoError(t, cleanupErr)
			_, cleanupErr = authDB.Exec(ctx, `DELETE FROM sdk_linked_sessions WHERE request_id=$1`, requestID)
			require.NoError(t, cleanupErr)
			_, cleanupErr = authDB.Exec(ctx, `DELETE FROM sdk_authorizations WHERE request_id=$1`, requestID)
			require.NoError(t, cleanupErr)
		})
		_, err := authDB.Exec(ctx, `INSERT INTO sdk_authorizations(request_id,source_account_id,device_id,source_session_hash,source_generation,
application_id,environment_id,idempotency_key,request_hash,redirect_uri,code_challenge,client_state,scopes,policy_revision,display_name,
expires_at,target_account_id,target_profile_id,target_epoch,profile_revision,consumed_at,game_binding_id,game_binding_status,
game_binding_authority_revision,game_binding_intent,game_binding_consent_revision)
SELECT $1,source_account_id,device_id,source_session_hash,source_generation,application_id,environment_id,$2,request_hash,
redirect_uri,code_challenge,client_state,scopes,policy_revision,display_name,expires_at,$3,$4,target_epoch,profile_revision,
consumed_at,$5,'active',1,false,NULL FROM sdk_authorizations WHERE game_binding_id=$6`,
			requestID, uuid.New(), fixture.sourceAccountID, targetProfileID, bindingID, fixture.bindingID)
		require.NoError(t, err)
		_, err = authDB.Exec(ctx, `INSERT INTO sdk_linked_sessions(token_hash,request_id,expires_at)
SELECT $1,$2,expires_at FROM sdk_linked_sessions WHERE request_id=(SELECT request_id FROM sdk_authorizations WHERE game_binding_id=$3)`, linkedHash, requestID, fixture.bindingID)
		require.NoError(t, err)
		_, err = authDB.Exec(ctx, `UPDATE sdk_authorizations SET game_binding_consent_revision=(SELECT consent_revision FROM sdk_linked_sessions WHERE request_id=$1) WHERE request_id=$1`, requestID)
		require.NoError(t, err)
		_, err = authDB.Exec(ctx, `INSERT INTO sdk_game_message_grants(grant_id,authorization_request_id,application_id,environment_id,target_account_id,target_profile_id,
target_epoch,binding_id,consent_revision,scopes,policy_revision,profile_revision,status,authority_revision,created_at,updated_at)
SELECT $1,$2,application_id,environment_id,$3,$4,target_epoch,$5,
  (SELECT consent_revision FROM sdk_linked_sessions WHERE request_id=$2),scopes,policy_revision,profile_revision,'active',1,now(),now()
FROM sdk_game_message_grants WHERE binding_id=$6 AND status='active'`, uuid.New(), requestID, fixture.sourceAccountID, targetProfileID, bindingID, fixture.bindingID)
		require.NoError(t, err)
		_, err = gisDB.Exec(ctx, `INSERT INTO player_bindings(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision)
SELECT $1,application_id,environment_id,'google','hmac-sha256-v1:t16-ambiguous:`+strings.Repeat("a", 64)+`',account_id,actor_id,profile_id,device_id,'active',1
FROM player_bindings WHERE binding_id=$2`, bindingID, fixture.bindingID)
		require.NoError(t, err)
		assertDenied(t, "ambiguous active grant", t16AuthDeviceAuthorityProof(t, fixture, fixture.deviceID), 1)
	})
	t.Run("device-authority rejects a GIS-revoked binding", func(t *testing.T) {
		_, err := gisDB.Exec(ctx, `UPDATE player_bindings SET status='revoked',authority_revision=authority_revision+1 WHERE binding_id=$1`, fixture.bindingID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, restoreErr := gisDB.Exec(ctx, `UPDATE player_bindings SET status='active',authority_revision=1 WHERE binding_id=$1`, fixture.bindingID)
			require.NoError(t, restoreErr)
		})
		assertDenied(t, "GIS binding revoked", t16AuthDeviceAuthorityProof(t, fixture, fixture.deviceID), 2)
	})
}

func randomT16TokenHash(t *testing.T) string {
	t.Helper()
	return hex.EncodeToString(randomT16Bytes(t, 32))
}

func countT16DeviceAuthorityIssues(t *testing.T, ctx context.Context, authDB *pgxpool.Pool, deviceID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_device_authority_issues WHERE device_id=$1`, deviceID).Scan(&count))
	return count
}

func signT16Message(t *testing.T, fixture t16AcceptFixture, key *ecdsa.PrivateKey, now time.Time) string {
	return signT16MessageWithAttachments(t, fixture, key, now, nil)
}

func newT16MessageID(t *testing.T) uuid.UUID {
	t.Helper()
	messageID, err := uuid.NewV7()
	require.NoError(t, err)
	return messageID
}

func signT16MessageWithAttachments(t *testing.T, fixture t16AcceptFixture, key *ecdsa.PrivateKey, now time.Time, attachments []map[string]any) string {
	t.Helper()
	issuedAt := now.UTC().Truncate(time.Second)
	expiresAt := issuedAt.Add(5 * time.Minute)
	content := []byte("T16 synthetic message")
	contentHash := sha256.Sum256(content)
	var manifest any
	var manifestHash any
	if attachments != nil {
		manifestJSON, err := json.Marshal(attachments)
		require.NoError(t, err)
		digest := sha256.Sum256(manifestJSON)
		manifest = base64.RawURLEncoding.EncodeToString(manifestJSON)
		manifestHash = hex.EncodeToString(digest[:])
	}
	claims := map[string]any{"version": 1, "operation": "create", "audience": "voice.game-message",
		"application_id": fixture.appID.String(), "environment_id": fixture.envID.String(),
		"account_id": fixture.sourceAccountID.String(), "actor_id": fixture.sourceActorID.String(), "binding_id": fixture.bindingID.String(),
		"device_id": fixture.deviceID.String(), "operation_id": fixture.operationID.String(), "authority_revision": fixture.authorityRevision,
		"chat_id": fixture.chatID.String(), "message_id": fixture.messageID.String(), "revision": 1,
		"previous_revision_hash": nil, "issued_at": issuedAt.Format(time.RFC3339),
		"expires_at": expiresAt.Format(time.RFC3339), "content_type": "text/plain",
		"content_b64": base64.RawURLEncoding.EncodeToString(content), "content_sha256": hex.EncodeToString(contentHash[:]),
		"attachment_manifest_b64": manifest, "attachment_manifest_sha256": manifestHash}
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	header, err := json.Marshal(map[string]string{"alg": "ES256", "kid": fixture.keyID.String(), "typ": "voice.game-message+jws"})
	require.NoError(t, err)
	return signT16ES256(t, key, header, payload)
}

func signT16ES256(t *testing.T, key *ecdsa.PrivateKey, header, payload []byte) string {
	t.Helper()
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	require.NoError(t, err)
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func t16PublicJWK(t *testing.T, key *ecdsa.PrivateKey) ([]byte, string) {
	t.Helper()
	x := base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32)))
	y := base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32)))
	jwk, err := json.Marshal(map[string]string{"kty": "EC", "crv": "P-256", "x": x, "y": y})
	require.NoError(t, err)
	thumbInput, err := json.Marshal(map[string]string{"crv": "P-256", "kty": "EC", "x": x, "y": y})
	require.NoError(t, err)
	digest := sha256.Sum256(thumbInput)
	return jwk, base64.RawURLEncoding.EncodeToString(digest[:])
}

func t16GatewayPrincipalRuntime(t *testing.T, ctx context.Context) (*principalruntime.Runtime, *principal.Issuer) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "gateway-t16", PrivateKey: key})
	require.NoError(t, err)
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "T16 test JWKS CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	serverCert := t16LeafCert(t, ca, caKey, "localhost", []string{"localhost"}, nil, x509.ExtKeyUsageServerAuth, 2)
	clientCert := t16LeafCert(t, ca, caKey, "t16-runtime-client", nil, nil, x509.ExtKeyUsageClientAuth, 3)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	keySet := []principal.JWKSKey{{KeyID: "gateway-t16", PublicKey: &key.PublicKey}}
	mux := http.NewServeMux()
	mux.Handle("/.well-known/principal-jwks.json", principal.JWKSHandlerKeys(keySet))
	server := httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	dir := t.TempDir()
	caPath, certPath, keyPath := filepath.Join(dir, "ca.crt"), filepath.Join(dir, "client.crt"), filepath.Join(dir, "client.key")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600))
	writeT16TLS(t, certPath, keyPath, clientCert)
	jwksURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1) + "/.well-known/principal-jwks.json"
	runtime, err := principalruntime.NewForIssuer(ctx, principalruntime.Config{JWKSURL: jwksURL, TLSCertFile: certPath,
		TLSKeyFile: keyPath, CAFile: caPath, RedisURL: requiredT16Env(t, "T16_REDIS_URL")}, "gateway")
	require.NoError(t, err)
	t.Cleanup(func() { _ = runtime.Close() })
	return runtime, issuer
}

func t16LeafCert(t *testing.T, ca *x509.Certificate, caKey *rsa.PrivateKey, cn string, dns, ips []string,
	usage x509.ExtKeyUsage, serial int64) tls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var ipAddrs []net.IP
	for _, raw := range ips {
		ipAddrs = append(ipAddrs, net.ParseIP(raw))
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: dns, IPAddresses: ipAddrs,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return cert
}

func writeT16TLS(t *testing.T, certPath, keyPath string, pair tls.Certificate) {
	t.Helper()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
}

func t16Pool(t *testing.T, ctx context.Context, name string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, requiredT16Env(t, name))
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)
	return pool
}

func seedT16GISOwnerSession(t *testing.T, ctx context.Context, fixture t16AcceptFixture) {
	t.Helper()
	options, err := redis.ParseURL(requiredT16Env(t, "T16_REDIS_URL"))
	require.NoError(t, err)
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Set(ctx, "auth:session:min_epoch:"+fixture.sourceAccountID.String(), "1", 0).Err())
}

func countT16GISWorkloadNonces(t *testing.T, ctx context.Context) int {
	t.Helper()
	options, err := redis.ParseURL(requiredT16Env(t, "T16_REDIS_URL"))
	require.NoError(t, err)
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	var count int
	var cursor uint64
	for {
		keys, next, scanErr := client.Scan(ctx, cursor, "gameintegration:auth-workload-nonce:*", 100).Result()
		require.NoError(t, scanErr)
		count += len(keys)
		cursor = next
		if cursor == 0 {
			return count
		}
	}
}

func requiredT16Env(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	require.NotEmpty(t, value, "%s is required", name)
	return value
}

func randomT16Bytes(t *testing.T, count int) []byte {
	t.Helper()
	result := make([]byte, count)
	_, err := rand.Read(result)
	require.NoError(t, err)
	return result
}

func t16SHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
