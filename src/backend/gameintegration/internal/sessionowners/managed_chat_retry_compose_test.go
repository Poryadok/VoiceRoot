package sessionowners

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/principal"
)

const t30ComposeGate = "VOICE_T30_COMPOSE_CHAT_RETRY"

func TestT30ComposeChatProvisionLostResponseRetriesRealReceipt(t *testing.T) {
	if os.Getenv(t30ComposeGate) != "1" {
		t.Skip("requires the isolated Phase0 Compose Chat and GIS services")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	gisPool := composeTestPool(t, ctx, "DATABASE_URL")
	chatPool := composeTestPool(t, ctx, "CHAT_DATABASE_URL")
	applicationID := composeTestUUID(t, "T31_APPLICATION_ID")
	environmentID := composeTestUUID(t, "T31_ENVIRONMENT_ID")
	principalContext := registry.SessionPrincipal{
		ApplicationID: applicationID,
		EnvironmentID: environmentID,
		Scopes:        []string{"game.sessions.manage"},
	}

	config := Config{
		ChatAddress:    strings.TrimSpace(os.Getenv("GIS_CHAT_GRPC_ADDR")),
		ChatCA:         strings.TrimSpace(os.Getenv("GIS_CHAT_TLS_CA_FILE")),
		ChatClientCert: strings.TrimSpace(os.Getenv("GIS_CHAT_CLIENT_CERT_FILE")),
		ChatClientKey:  strings.TrimSpace(os.Getenv("GIS_CHAT_CLIENT_KEY_FILE")),
	}
	chatConn, err := dialMTLS(config.ChatAddress, config.ChatCA, config.ChatClientCert, config.ChatClientKey)
	require.NoError(t, err)
	t.Cleanup(func() { _ = chatConn.Close() })
	// NewClient is lazy. Establish the real mTLS transport as fixture setup,
	// before the production adapter starts its unchanged two-second RPC budget.
	// This sends no warmup mutation and does not retry an uncertain owner effect.
	require.NoError(t, waitForComposeChatConnection(ctx, chatConn), "Chat owner transport must be ready before response-loss injection")
	issuer := composeTestIssuer(t)
	chatClient := &dropFirstProvisionResponse{
		GameIntegrationChatServiceClient: chatv1.NewGameIntegrationChatServiceClient(chatConn),
	}
	clients := &Clients{chat: chatClient, issuer: issuer}
	orchestrator := registry.NewSessionOrchestrator(&registry.Store{Pool: gisPool}, clients.Adapters())

	externalKey := "t30-lost-response-" + uuid.NewString()
	accepted, err := orchestrator.CreateSession(ctx, principalContext, registry.CreateSessionInput{
		OperationID: uuid.New(), Kind: "match", ExternalKey: externalKey, DisplayName: "T30 receipt retry",
		RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{uuid.New()},
	})
	require.NoError(t, err)
	var mappingOperationsBefore int
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM game_resource_operations WHERE application_id=$1 AND environment_id=$2`, applicationID, environmentID).Scan(&mappingOperationsBefore))

	_, err = orchestrator.AdvanceOne(ctx, accepted.OperationID)
	require.Error(t, err, "the real Chat RPC committed before the test wrapper dropped its first successful response")
	require.Len(t, chatClient.requests, 1)
	require.Len(t, chatClient.responses, 1,
		"the underlying Chat RPC must succeed before the wrapper injects response loss; actual gRPC status=%s, error=%v",
		chatClient.firstRPCErrorCode, err)
	require.Equal(t, codes.Unavailable, status.Code(err), "the test wrapper must return its simulated response-loss status")
	firstRequest := chatClient.requests[0]
	firstResponse := chatClient.responses[0]
	require.NotEmpty(t, firstResponse.GetChatId())
	require.Equal(t, firstRequest.GetOperationId(), firstResponse.GetReceiptId())
	firstHash, err := principal.RequestHash(firstRequest)
	require.NoError(t, err)
	require.NotEmpty(t, firstHash)
	require.Equal(t, firstHash, firstResponse.GetRequestHash())
	require.False(t, firstResponse.GetReplayed())

	chatID, chatReceiptID, chatRequestHash := assertCommittedChatReceipt(t, ctx, chatPool, applicationID, environmentID, externalKey, firstRequest.GetOperationId())
	var mappingCount, mappingOperationCount, ownerReceiptCount, outboxCount int
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, applicationID, environmentID, externalKey).Scan(&mappingCount))
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM game_resource_operations WHERE application_id=$1 AND environment_id=$2`, applicationID, environmentID).Scan(&mappingOperationCount))
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE operation_id=$1 AND stage='chat_create'`, accepted.OperationID).Scan(&ownerReceiptCount))
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM gis_session_outbox WHERE session_id=$1`, accepted.SessionID).Scan(&outboxCount))
	require.Zero(t, mappingCount)
	require.Equal(t, mappingOperationsBefore, mappingOperationCount, "GIS has no successful mapping operation receipt before reconciling Chat")
	require.Zero(t, ownerReceiptCount)
	require.Zero(t, outboxCount)
	pending, err := orchestrator.GetOperation(ctx, principalContext, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.Status)
	require.Equal(t, "provisioning", pending.SessionStatus)
	require.Equal(t, "accepted", pending.Stage)
	require.Nil(t, pending.ChatID)
	require.Nil(t, pending.ChatCreateReceiptID)
	require.Nil(t, pending.ActiveEventID)

	advanced, err := orchestrator.AdvanceOne(ctx, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "chat_ready", advanced.Stage)
	require.Len(t, chatClient.requests, 2)
	retryRequest := chatClient.requests[1]
	retryBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(retryRequest)
	require.NoError(t, err)
	firstBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(firstRequest)
	require.NoError(t, err)
	require.Equal(t, firstRequest.GetOperationId(), retryRequest.GetOperationId(), "GIS retries the same deterministic Chat operation")
	require.Equal(t, firstBytes, retryBytes, "GIS retries byte-identical complete protobuf requests")
	retryHash, err := principal.RequestHash(retryRequest)
	require.NoError(t, err)
	require.Equal(t, firstHash, retryHash, "full-protobuf request hash remains stable")
	require.Len(t, chatClient.responses, 2)
	require.False(t, chatClient.responses[0].GetReplayed())
	require.True(t, chatClient.responses[1].GetReplayed(), "the actual Chat handler returns its persisted receipt on retry")
	replayedChatID, replayedReceiptID, replayedRequestHash := assertCommittedChatReceipt(t, ctx, chatPool, applicationID, environmentID, externalKey, retryRequest.GetOperationId())
	require.Equal(t, chatID, replayedChatID)
	require.Equal(t, chatReceiptID, replayedReceiptID)
	require.Equal(t, chatRequestHash, replayedRequestHash)
	require.Equal(t, chatID, advancedValue(t, advanced.ChatID))
	require.Equal(t, chatReceiptID, advancedValue(t, advanced.ChatCreateReceiptID))
	require.Equal(t, firstHash, chatRequestHash)
	require.Equal(t, firstRequest.GetOperationId(), chatReceiptID.String())

	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, applicationID, environmentID, externalKey).Scan(&mappingCount))
	require.Equal(t, 1, mappingCount)
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM game_resource_operations WHERE application_id=$1 AND environment_id=$2`, applicationID, environmentID).Scan(&mappingOperationCount))
	require.Equal(t, mappingOperationsBefore+1, mappingOperationCount)
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts WHERE operation_id=$1 AND stage='chat_create' AND owner_operation_id=$2 AND resource_id=$3 AND receipt_id=$4`, accepted.OperationID, uuid.MustParse(firstRequest.GetOperationId()), chatID, chatReceiptID).Scan(&ownerReceiptCount))
	require.Equal(t, 1, ownerReceiptCount)
	var mappedResourceID, mappedChatID, mappedChatOperationID uuid.UUID
	var mappedRequestHash string
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT resource_id,chat_id,chat_operation_id,chat_request_hash FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND external_key=$3`, applicationID, environmentID, externalKey).Scan(&mappedResourceID, &mappedChatID, &mappedChatOperationID, &mappedRequestHash))
	require.Equal(t, chatID, mappedResourceID)
	require.Equal(t, chatID, mappedChatID)
	require.Equal(t, uuid.MustParse(firstRequest.GetOperationId()), mappedChatOperationID)
	require.Equal(t, firstHash, mappedRequestHash)
	require.Nil(t, advanced.ActiveEventID)
	require.NoError(t, gisPool.QueryRow(ctx, `SELECT count(*) FROM gis_session_outbox WHERE session_id=$1`, accepted.SessionID).Scan(&outboxCount))
	require.Zero(t, outboxCount)
	t.Logf("T30 actual Chat receipt retry passed: session_id=%s chat_operation_id=%s chat_id=%s receipt_id=%s request_hash=%s gis_mapping_rows=%d chat_effect_rows=%d", accepted.SessionID, firstRequest.GetOperationId(), chatID, chatReceiptID, firstHash, mappingCount, 1)
}

func waitForComposeChatConnection(ctx context.Context, conn *grpc.ClientConn) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	conn.Connect()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.Shutdown {
			return errors.New("Chat owner connection closed before becoming ready")
		}
		if !conn.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
}

func TestComposeChatConnectionReadiness(t *testing.T) {
	t.Run("connects lazy transport without an application RPC", func(t *testing.T) {
		listener := bufconn.Listen(1 << 20)
		server := grpc.NewServer() // No application service or mutation is registered.
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(func() { server.Stop(); _ = listener.Close() })
		conn, err := grpc.NewClient("passthrough:///compose-chat-ready", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		require.Equal(t, connectivity.Idle, conn.GetState())
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		require.NoError(t, waitForComposeChatConnection(ctx, conn))
		require.Equal(t, connectivity.Ready, conn.GetState())
	})
	t.Run("unavailable transport stays bounded by fixture context", func(t *testing.T) {
		conn, err := grpc.NewClient("passthrough:///compose-chat-unavailable", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return nil, errors.New("fixture transport unavailable")
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, waitForComposeChatConnection(ctx, conn), context.DeadlineExceeded)
	})
	t.Run("closed transport cannot pass readiness", func(t *testing.T) {
		conn, err := grpc.NewClient("passthrough:///compose-chat-closed", grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		require.NoError(t, conn.Close())
		require.ErrorContains(t, waitForComposeChatConnection(t.Context(), conn), "connection closed")
	})
}

type dropFirstProvisionResponse struct {
	chatv1.GameIntegrationChatServiceClient
	requests          []*chatv1.ProvisionManagedChatRequest
	responses         []*chatv1.ProvisionManagedChatResponse
	firstRPCErrorCode string
}

func (c *dropFirstProvisionResponse) ProvisionManagedChat(ctx context.Context, request *chatv1.ProvisionManagedChatRequest, options ...grpc.CallOption) (*chatv1.ProvisionManagedChatResponse, error) {
	c.requests = append(c.requests, proto.Clone(request).(*chatv1.ProvisionManagedChatRequest))
	response, err := c.GameIntegrationChatServiceClient.ProvisionManagedChat(ctx, request, options...)
	if err != nil {
		if len(c.requests) == 1 {
			c.firstRPCErrorCode = status.Code(err).String()
		}
		return nil, err
	}
	c.responses = append(c.responses, proto.Clone(response).(*chatv1.ProvisionManagedChatResponse))
	if len(c.responses) == 1 {
		return nil, status.Error(codes.Unavailable, "simulated response loss after committed Chat receipt")
	}
	return response, nil
}

func assertCommittedChatReceipt(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationID, environmentID uuid.UUID, externalKey, operationID string) (uuid.UUID, uuid.UUID, string) {
	t.Helper()
	parsedOperationID, err := uuid.Parse(operationID)
	require.NoError(t, err)
	var chatID, receiptID uuid.UUID
	var requestHash string
	require.NoError(t, pool.QueryRow(ctx, `SELECT chat_id,request_hash FROM managed_chat_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND method='create'`, applicationID, environmentID, parsedOperationID).Scan(&chatID, &requestHash))
	require.NoError(t, pool.QueryRow(ctx, `SELECT operation_id FROM managed_chat_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, applicationID, environmentID, parsedOperationID).Scan(&receiptID))
	require.Equal(t, parsedOperationID, receiptID, "Chat receipt_id is the durable operation UUID")
	var managedChatCount, operationCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chats WHERE managed_by_application_id=$1 AND managed_environment_id=$2 AND external_chat_key=$3 AND id=$4`, applicationID, environmentID, externalKey, chatID).Scan(&managedChatCount))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM managed_chat_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`, applicationID, environmentID, parsedOperationID).Scan(&operationCount))
	require.Equal(t, 1, managedChatCount)
	require.Equal(t, 1, operationCount)
	return chatID, receiptID, requestHash
}

func composeTestPool(t *testing.T, ctx context.Context, variable string) *pgxpool.Pool {
	t.Helper()
	connectionString := strings.TrimSpace(os.Getenv(variable))
	require.NotEmpty(t, connectionString, "%s is required by the Compose acceptance runner", variable)
	pool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)
	return pool
}

func composeTestUUID(t *testing.T, variable string) uuid.UUID {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(variable))
	parsed, err := uuid.Parse(value)
	require.NoError(t, err, "%s must be a UUID", variable)
	require.Equal(t, parsed.String(), value, "%s must be canonical", variable)
	return parsed
}

func composeTestIssuer(t *testing.T) *principal.Issuer {
	t.Helper()
	keyPath := strings.TrimSpace(os.Getenv("GAME_INTEGRATION_PRINCIPAL_PRIVATE_KEY_FILE"))
	keyID := strings.TrimSpace(os.Getenv("GAME_INTEGRATION_PRINCIPAL_KID"))
	keyPEM, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	require.True(t, ok, "GIS signer must be an RSA key")
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gameintegration", KeyID: keyID, PrivateKey: privateKey})
	require.NoError(t, err)
	return issuer
}

func advancedValue(t *testing.T, value *uuid.UUID) uuid.UUID {
	t.Helper()
	if value == nil {
		t.Fatal(errors.New("expected a persisted Chat UUID"))
	}
	return *value
}
