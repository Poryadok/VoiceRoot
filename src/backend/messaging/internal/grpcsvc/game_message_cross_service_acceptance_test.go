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
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	userDB := t16Pool(t, ctx, "T16_USER_DATABASE_URL")
	chatDB := t16Pool(t, ctx, "T16_CHAT_DATABASE_URL")
	messagingDB := t16Pool(t, ctx, "T16_MESSAGING_DATABASE_URL")

	fixture := seedT16AcceptFixture(t, ctx, authDB, gisDB, userDB, chatDB)
	seedT16GISOwnerSession(t, ctx, fixture)
	workloadNonceCount := countT16GISWorkloadNonces(t, ctx)
	assertion := requestT16AuthDeviceAuthority(t, ctx, fixture)
	require.Equal(t, workloadNonceCount+1, countT16GISWorkloadNonces(t, ctx),
		"successful Auth device-authority issuance must traverse GIS's production WorkloadProof verifier and authority handler")

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
	processor := &VerifiedGameMessageProcessor{Store: storeMessages, AuthKeys: gameAuthKeys,
		Permits: permitClient, Bindings: &AuthBackedGameBindingAuthority{
			Chats: chatGuard, ResourceMappings: mappingClient,
		}}
	verifier, gatewayIssuer := t16GatewayPrincipalRuntime(t, ctx)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(principalgrpc.ApplyGameMessageUnaryInterceptor(verifier)))
	messagingv1.RegisterMessagingServiceServer(grpcServer, &MessagingGRPC{Messages: storeMessages, GameMessages: processor})
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
	assertNoT16ExecutionSideEffects(t, ctx, authDB, messagingDB, fixture)

	// GIS must deny absent, foreign-binding, and altered-chat mappings before
	// Auth issues any execution permit.
	_, err = gisDB.Exec(ctx, `DELETE FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND chat_id=$3`, fixture.appID, fixture.envID, fixture.chatID)
	require.NoError(t, err)
	_, err = apply(request, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, requestHash)
	require.Error(t, err, "absent mapping must fail closed")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, messagingDB, fixture)
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
	_, err = apply(request, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, requestHash)
	require.Error(t, err, "foreign binding mapping must fail closed")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, messagingDB, fixture)
	_, err = gisDB.Exec(ctx, `DELETE FROM game_resource_binding_chats WHERE application_id=$1 AND environment_id=$2 AND binding_id=$3 AND chat_id=$4`, fixture.appID, fixture.envID, foreignBindingID, fixture.chatID)
	require.NoError(t, err)
	_, err = gisDB.Exec(ctx, `INSERT INTO game_resource_binding_chats(application_id,environment_id,binding_id,chat_id,roster_revision,status,lease_expires_at) VALUES($1,$2,$3,$4,1,'active',now()+interval '1 hour')`, fixture.appID, fixture.envID, fixture.bindingID, fixture.chatID)
	require.NoError(t, err)

	_, err = gisDB.Exec(ctx, `UPDATE game_resource_mappings SET chat_id=$1,resource_id=$1 WHERE application_id=$2 AND environment_id=$3 AND chat_id=$4`, uuid.New(), fixture.appID, fixture.envID, fixture.chatID)
	require.NoError(t, err)
	_, err = apply(request, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, requestHash)
	require.Error(t, err, "altered chat mapping must fail closed")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, messagingDB, fixture)
	require.NoError(t, seedT16ResourceMapping(ctx, gisDB, fixture))

	response, err := apply(request, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, requestHash)
	require.NoError(t, err, "the real Auth -> GIS -> Messaging path should accept the exact linked chat")
	require.Equal(t, fixture.messageID.String(), response.GetMessage().GetId())
	require.Equal(t, fixture.chatID.String(), response.GetMessage().GetDisplayChatId())

	// A lost gRPC response is retried with a fresh Gateway principal. Messaging
	// must return the exact durable receipt without another permit or message.
	retryID := uuid.NewString()
	retryPrincipal, err := gatewayIssuer.IssueService(principal.ServiceInput{
		Audience: "messaging", RPC: messagingv1.MessagingService_ApplyGameMessage_FullMethodName,
		RequestID: retryID, RequestHash: requestHash,
	})
	require.NoError(t, err)
	retryCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+retryPrincipal, "x-request-id", retryID))
	retried, err := client.ApplyGameMessage(retryCtx, request)
	require.NoError(t, err)
	require.Equal(t, response.GetMessage().GetId(), retried.GetMessage().GetId())
	var revisions int
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, fixture.messageID).Scan(&revisions))
	require.Equal(t, 1, revisions)
	var permitCompletions int
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1 AND outcome='committed'`, fixture.operationID).Scan(&permitCompletions))
	require.Equal(t, 1, permitCompletions)

	// Chat membership is intentionally checked after Auth/GIS issues the permit.
	// The failure must therefore leave a durable aborted completion and no message.
	chatFailure := fixture
	chatFailure.operationID, chatFailure.messageID = uuid.New(), uuid.New()
	chatFailure.compactMessage = signT16Message(t, chatFailure, fixture.deviceKey, time.Now().UTC())
	chatAssertion := requestT16AuthDeviceAuthority(t, ctx, chatFailure)
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
	fileFailure.operationID, fileFailure.messageID = uuid.New(), uuid.New()
	fileFailure.compactMessage = signT16MessageWithAttachments(t, fileFailure, fixture.deviceKey, time.Now().UTC(), []map[string]any{{
		"byte_length": 17, "content_sha256": strings.Repeat("a", 64), "file_id": uuid.NewString(), "media_type": "text/plain", "object_revision": 1,
	}})
	fileAssertion := requestT16AuthDeviceAuthority(t, ctx, fileFailure)
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
	revokeRace.operationID, revokeRace.messageID = uuid.New(), uuid.New()
	revokeRace.compactMessage = signT16Message(t, revokeRace, fixture.deviceKey, time.Now().UTC())
	revokeAssertion := requestT16AuthDeviceAuthority(t, ctx, revokeRace)
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
	var pendingExpiryBefore, pendingExpiryAfter time.Time
	require.NoError(t, authDB.QueryRow(ctx, `SELECT expires_at FROM sdk_game_message_execution_permits WHERE operation_id=$1`, pendingOperationID).Scan(&pendingExpiryBefore))
	require.NoError(t, gisDB.QueryRow(ctx, `SELECT expires_at FROM player_binding_execution_permits WHERE binding_id=$1 AND operation_id=$2`, fixture.bindingID, pendingOperationID).Scan(&pendingExpiryAfter))
	require.Equal(t, pendingExpiryBefore, pendingExpiryAfter, "Auth and GIS must persist the same fixed permit expiry")
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
	oldAuthorityOperation.operationID, oldAuthorityOperation.messageID = uuid.New(), uuid.New()
	oldAuthorityOperation.compactMessage = signT16Message(t, oldAuthorityOperation, fixture.deviceKey, time.Now().UTC())
	oldAuthorityRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: oldAuthorityOperation.compactMessage, DeviceAuthorityAssertion: assertion}
	oldAuthorityHash, err := principal.RequestHash(oldAuthorityRequest)
	require.NoError(t, err)
	_, err = apply(oldAuthorityRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, oldAuthorityHash)
	require.Error(t, err, "old-binding proof must not authorize a new operation after GIS revoke")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, messagingDB, oldAuthorityOperation)

	// Model a durable owner-approved relink without provider credentials: GIS
	// commits a new binding ID/authority revision, and the current Auth grant,
	// consent, and device authority revision are updated before the new use.
	// The monotonic target starts after both service stores report the relink.
	restored, restoreT0 := restoreT16Binding(t, ctx, authDB, gisDB, fixture)
	restoredMapping := seedT16BindingChatMapping(ctx, gisDB, restored)
	require.NoError(t, restoredMapping)
	restoreOperation := restored
	restoreOperation.operationID, restoreOperation.messageID = uuid.New(), uuid.New()
	restoreOperation.compactMessage = signT16Message(t, restoreOperation, restored.deviceKey, time.Now().UTC())
	newAuthority := requestT16AuthDeviceAuthority(t, ctx, restoreOperation)
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
	oldAfterRestore.operationID, oldAfterRestore.messageID = uuid.New(), uuid.New()
	oldAfterRestore.compactMessage = signT16Message(t, oldAfterRestore, restored.deviceKey, time.Now().UTC())
	oldAfterRestoreRequest := &messagingv1.ApplyGameMessageRequest{CompactJws: oldAfterRestore.compactMessage, DeviceAuthorityAssertion: assertion}
	oldAfterRestoreHash, err := principal.RequestHash(oldAfterRestoreRequest)
	require.NoError(t, err)
	_, err = apply(oldAfterRestoreRequest, gatewayIssuer, "messaging", messagingv1.MessagingService_ApplyGameMessage_FullMethodName, oldAfterRestoreHash)
	require.Error(t, err, "pre-relink Auth assertion for the old binding must remain denied")
	assertNoT16ExecutionSideEffects(t, ctx, authDB, messagingDB, oldAfterRestore)
}

func assertNoT16ExecutionSideEffects(t *testing.T, ctx context.Context, authDB, messagingDB *pgxpool.Pool, fixture t16AcceptFixture) {
	t.Helper()
	var permits, messages, completions int
	require.NoError(t, authDB.QueryRow(ctx, `SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=$1`, fixture.operationID).Scan(&permits))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_revisions WHERE message_id=$1`, fixture.messageID).Scan(&messages))
	require.NoError(t, messagingDB.QueryRow(ctx, `SELECT count(*) FROM game_message_execution_permit_completions WHERE operation_id=$1`, fixture.operationID).Scan(&completions))
	require.Zero(t, permits, "denial must precede Auth permit issuance")
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
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
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

func seedT16ResourceMapping(ctx context.Context, gisDB *pgxpool.Pool, fixture t16AcceptFixture) error {
	_, err := gisDB.Exec(ctx, `INSERT INTO game_resource_mappings(mapping_id,application_id,environment_id,external_key,resource_kind,resource_id,chat_id,chat_operation_id,chat_request_hash,status,mapping_revision)
VALUES($1,$2,$3,'t16-synthetic-chat','chat',$4,$4,$5,'sha256:`+strings.Repeat("e", 64)+`','active',1) ON CONFLICT (application_id,environment_id,external_key) DO UPDATE SET resource_id=EXCLUDED.resource_id,chat_id=EXCLUDED.chat_id,status='active',mapping_revision=game_resource_mappings.mapping_revision+1`, uuid.New(), fixture.appID, fixture.envID, fixture.chatID, uuid.New())
	return err
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
		chatID: uuid.New(), messageID: uuid.New(), operationID: uuid.New(), ownerProfileID: uuid.New(), challengeID: uuid.New(), bindingOperationID: uuid.New()}
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
	_, err = gisDB.Exec(ctx, `INSERT INTO player_binding_challenges(challenge_id,nonce,application_id,environment_id,provider,redirect_uri_sha256,pkce_challenge,device_key_id,device_key_thumbprint,operation_id,expires_at,status,consumed_at)
VALUES($1,$2,$3,$4,'google',repeat('1',64),repeat('2',43),$5,$6,$7,now()+interval '1 hour','consumed',now())`, fixture.challengeID, uuid.NewString(), fixture.appID, fixture.envID, fixture.keyID, thumbprint, fixture.bindingOperationID)
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

func requestT16AuthDeviceAuthority(t *testing.T, ctx context.Context, fixture t16AcceptFixture) string {
	t.Helper()
	now := time.Now().UTC()
	claims := map[string]any{"version": 1, "audience": "voice.game-message", "request_id": uuid.NewString(),
		"application_id": fixture.appID.String(), "environment_id": fixture.envID.String(), "device_id": fixture.deviceID.String(), "issued_at": now.UnixMilli()}
	body, err := json.Marshal(claims)
	require.NoError(t, err)
	header, err := json.Marshal(map[string]string{"alg": "ES256", "kid": fixture.keyID.String(), "typ": "voice.game-device-authority-request+jws"})
	require.NoError(t, err)
	proof := signT16ES256(t, fixture.deviceKey, header, body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requiredT16Env(t, "T16_AUTH_BASE_URL")+"/api/v1/auth/sdk/device-authority", strings.NewReader(proof))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+fixture.sessionToken)
	request.Header.Set("Content-Type", "text/plain")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode, string(data))
	return string(data)
}

func signT16Message(t *testing.T, fixture t16AcceptFixture, key *ecdsa.PrivateKey, now time.Time) string {
	return signT16MessageWithAttachments(t, fixture, key, now, nil)
}

func signT16MessageWithAttachments(t *testing.T, fixture t16AcceptFixture, key *ecdsa.PrivateKey, now time.Time, attachments []map[string]any) string {
	t.Helper()
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
		"previous_revision_hash": nil, "issued_at": now.Add(-time.Second).UTC().Truncate(time.Second).Format(time.RFC3339),
		"expires_at": now.Add(5 * time.Minute).UTC().Truncate(time.Second).Format(time.RFC3339), "content_type": "text/plain",
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
