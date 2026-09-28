package sessionowners

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

const callTimeout = 2 * time.Second

type Config struct {
	ChatAddress, ChatCA, ChatClientCert, ChatClientKey     string
	VoiceAddress, VoiceCA, VoiceClientCert, VoiceClientKey string
	RoleAddress, RoleCA, RoleClientCert, RoleClientKey     string
}

type Clients struct {
	chat        chatv1.GameIntegrationChatServiceClient
	voice       callsv1.GameSessionProvisioningServiceClient
	role        rolev1.RoleServiceClient
	issuer      *principal.Issuer
	connections []*grpc.ClientConn
}

func LoadFromEnv(getenv func(string) string) (Config, bool, error) {
	if getenv == nil {
		return Config{}, false, errors.New("environment reader is required")
	}
	config := Config{
		ChatAddress: strings.TrimSpace(getenv("GIS_CHAT_GRPC_ADDR")), ChatCA: strings.TrimSpace(getenv("GIS_CHAT_TLS_CA_FILE")),
		ChatClientCert: strings.TrimSpace(getenv("GIS_CHAT_CLIENT_CERT_FILE")), ChatClientKey: strings.TrimSpace(getenv("GIS_CHAT_CLIENT_KEY_FILE")),
		VoiceAddress: strings.TrimSpace(getenv("GIS_VOICE_GRPC_ADDR")), VoiceCA: strings.TrimSpace(getenv("GIS_VOICE_TLS_CA_FILE")),
		VoiceClientCert: strings.TrimSpace(getenv("GIS_VOICE_CLIENT_CERT_FILE")), VoiceClientKey: strings.TrimSpace(getenv("GIS_VOICE_CLIENT_KEY_FILE")),
		RoleAddress: strings.TrimSpace(getenv("GIS_ROLE_GRPC_ADDR")), RoleCA: strings.TrimSpace(getenv("GIS_ROLE_TLS_CA_FILE")),
		RoleClientCert: strings.TrimSpace(getenv("GIS_ROLE_CLIENT_CERT_FILE")), RoleClientKey: strings.TrimSpace(getenv("GIS_ROLE_CLIENT_KEY_FILE")),
	}
	values := []string{config.ChatAddress, config.ChatCA, config.ChatClientCert, config.ChatClientKey,
		config.VoiceAddress, config.VoiceCA, config.VoiceClientCert, config.VoiceClientKey,
		config.RoleAddress, config.RoleCA, config.RoleClientCert, config.RoleClientKey}
	configured := 0
	for _, value := range values {
		if value != "" {
			configured++
		}
	}
	if configured == 0 {
		return Config{}, false, nil
	}
	if configured != len(values) {
		return Config{}, true, errors.New("GIS Chat, Voice, and Role owner mTLS clients must be configured together")
	}
	return config, true, nil
}

func New(config Config, issuer *principal.Issuer) (*Clients, error) {
	if issuer == nil {
		return nil, errors.New("GIS principal issuer is required")
	}
	client := &Clients{issuer: issuer}
	chatConn, err := dialMTLS(config.ChatAddress, config.ChatCA, config.ChatClientCert, config.ChatClientKey)
	if err != nil {
		return nil, fmt.Errorf("dial Chat owner: %w", err)
	}
	client.connections = append(client.connections, chatConn)
	voiceConn, err := dialMTLS(config.VoiceAddress, config.VoiceCA, config.VoiceClientCert, config.VoiceClientKey)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("dial Voice owner: %w", err)
	}
	client.connections = append(client.connections, voiceConn)
	roleConn, err := dialMTLS(config.RoleAddress, config.RoleCA, config.RoleClientCert, config.RoleClientKey)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("dial Role owner: %w", err)
	}
	client.connections = append(client.connections, roleConn)
	client.chat = chatv1.NewGameIntegrationChatServiceClient(chatConn)
	client.voice = callsv1.NewGameSessionProvisioningServiceClient(voiceConn)
	client.role = rolev1.NewRoleServiceClient(roleConn)
	return client, nil
}

func dialMTLS(address, caFile, certFile, keyFile string) (*grpc.ClientConn, error) {
	if strings.TrimSpace(address) == "" {
		return nil, errors.New("owner gRPC address is required")
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("owner TLS CA contains no certificates")
	}
	clientCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{clientCert}}
	return grpc.NewClient(grpcclient.DialTarget(address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
}

func (c *Clients) Close() error {
	var first error
	for _, connection := range c.connections {
		if err := connection.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (c *Clients) Adapters() registry.SessionOwnerAdapters {
	return registry.SessionOwnerAdapters{
		CreateChat: c.createChat, SyncChatRoster: c.syncChatRoster, ProvisionVoice: c.provisionVoice,
		ApplyRoleGrants: c.applyRole, CloseVoice: c.closeVoice, RevokeRoleGrants: c.revokeRole,
	}
}

func (c *Clients) createChat(ctx context.Context, owner registry.SessionOwnerRequest) (registry.SessionOwnerReceipt, error) {
	request, ok := owner.Proto.(*chatv1.ProvisionManagedChatRequest)
	if !ok || request.GetOperationId() != owner.OperationID.String() {
		return registry.SessionOwnerReceipt{}, errors.New("invalid Chat create owner request")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	callCtx, err := c.authContext(callCtx, "chat", chatv1.GameIntegrationChatService_ProvisionManagedChat_FullMethodName, request, request.GetOperationId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	response, err := c.chat.ProvisionManagedChat(callCtx, request)
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	chatID, err := parseUUID(response.GetChatId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	receiptID, err := parseUUID(response.GetReceiptId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	if response.GetRequestHash() != owner.RequestHash {
		return registry.SessionOwnerReceipt{}, errors.New("chat receipt request hash mismatch")
	}
	return registry.SessionOwnerReceipt{ResourceID: chatID, ReceiptID: receiptID, RequestHash: response.GetRequestHash()}, nil
}

func (c *Clients) syncChatRoster(ctx context.Context, owner registry.SessionOwnerRequest) (registry.SessionOwnerReceipt, error) {
	request, ok := owner.Proto.(*chatv1.SyncManagedChatMembersRequest)
	if !ok || request.GetOperationId() != owner.OperationID.String() {
		return registry.SessionOwnerReceipt{}, errors.New("invalid Chat roster owner request")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	callCtx, err := c.authContext(callCtx, "chat", chatv1.GameIntegrationChatService_SyncManagedChatMembers_FullMethodName, request, request.GetOperationId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	response, err := c.chat.SyncManagedChatMembers(callCtx, request)
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	receiptID, err := parseUUID(response.GetReceiptId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	chatID, err := parseUUID(request.GetChatId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	if response.GetRequestHash() != owner.RequestHash {
		return registry.SessionOwnerReceipt{}, errors.New("chat roster receipt request hash mismatch")
	}
	return registry.SessionOwnerReceipt{ResourceID: chatID, ReceiptID: receiptID, RequestHash: response.GetRequestHash()}, nil
}

func (c *Clients) provisionVoice(ctx context.Context, owner registry.SessionOwnerRequest) (registry.SessionOwnerReceipt, error) {
	request, ok := owner.Proto.(*callsv1.ProvisionGameSessionRoomRequest)
	if !ok || request.GetOperationId() != owner.OperationID.String() {
		return registry.SessionOwnerReceipt{}, errors.New("invalid Voice provision owner request")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	callCtx, err := c.authContext(callCtx, "voice", callsv1.GameSessionProvisioningService_ProvisionGameSessionRoom_FullMethodName, request, request.GetOperationId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	response, err := c.voice.ProvisionGameSessionRoom(callCtx, request)
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	roomID, err := parseUUID(response.GetRoomId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	receiptID, err := parseUUID(response.GetVoiceCreationReceiptId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	if !hashBytesMatch(response.GetRequestHash(), owner.RequestHash) {
		return registry.SessionOwnerReceipt{}, errors.New("Voice provision receipt request hash mismatch")
	}
	return registry.SessionOwnerReceipt{ResourceID: roomID, ReceiptID: receiptID, RequestHash: owner.RequestHash}, nil
}

func (c *Clients) applyRole(ctx context.Context, owner registry.SessionOwnerRequest) (registry.SessionOwnerReceipt, error) {
	request, ok := owner.Proto.(*rolev1.ApplyGameSessionGrantsRequest)
	if !ok || request.GetOperationId() != owner.OperationID.String() {
		return registry.SessionOwnerReceipt{}, errors.New("invalid Role apply owner request")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	callCtx, err := c.authContext(callCtx, "role", rolev1.RoleService_ApplyGameSessionGrants_FullMethodName, request, request.GetOperationId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	response, err := c.role.ApplyGameSessionGrants(callCtx, request)
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	return roleReceipt(response.GetReceipt(), owner.RequestHash, request.GetVoiceRoomId())
}

func (c *Clients) closeVoice(ctx context.Context, owner registry.SessionOwnerRequest) (registry.SessionOwnerReceipt, error) {
	request, ok := owner.Proto.(*callsv1.CloseGameSessionRoomRequest)
	if !ok || request.GetOperationId() != owner.OperationID.String() {
		return registry.SessionOwnerReceipt{}, errors.New("invalid Voice close owner request")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	callCtx, err := c.authContext(callCtx, "voice", callsv1.GameSessionProvisioningService_CloseGameSessionRoom_FullMethodName, request, request.GetOperationId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	response, err := c.voice.CloseGameSessionRoom(callCtx, request)
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	roomID, err := parseUUID(response.GetRoomId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	receiptID, err := parseUUID(response.GetCloseReceiptId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	if !hashBytesMatch(response.GetRequestHash(), owner.RequestHash) {
		return registry.SessionOwnerReceipt{}, errors.New("Voice close receipt request hash mismatch")
	}
	return registry.SessionOwnerReceipt{ResourceID: roomID, ReceiptID: receiptID, RequestHash: owner.RequestHash}, nil
}

func (c *Clients) revokeRole(ctx context.Context, owner registry.SessionOwnerRequest) (registry.SessionOwnerReceipt, error) {
	request, ok := owner.Proto.(*rolev1.RevokeGameSessionGrantsRequest)
	if !ok || request.GetOperationId() != owner.OperationID.String() {
		return registry.SessionOwnerReceipt{}, errors.New("invalid Role revoke owner request")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	callCtx, err := c.authContext(callCtx, "role", rolev1.RoleService_RevokeGameSessionGrants_FullMethodName, request, request.GetOperationId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	response, err := c.role.RevokeGameSessionGrants(callCtx, request)
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	return roleReceipt(response.GetReceipt(), owner.RequestHash, owner.VoiceRoomID.String())
}

func (c *Clients) authContext(ctx context.Context, audience, method string, request proto.Message, requestID string) (context.Context, error) {
	requestHash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	if requestID == "" || requestHash == "" {
		return nil, errors.New("principal request binding is incomplete")
	}
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: requestHash})
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", requestID), nil
}

func roleReceipt(receipt *rolev1.GameSessionGrantReceipt, expectedHash, resource string) (registry.SessionOwnerReceipt, error) {
	if receipt == nil || !hashBytesMatch(receipt.GetRequestSha256(), expectedHash) {
		return registry.SessionOwnerReceipt{}, errors.New("Role grant receipt request hash mismatch")
	}
	receiptID, err := parseUUID(receipt.GetReceiptId())
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	resourceID, err := parseUUID(resource)
	if err != nil {
		return registry.SessionOwnerReceipt{}, err
	}
	return registry.SessionOwnerReceipt{ResourceID: resourceID, ReceiptID: receiptID, RequestHash: expectedHash}, nil
}

func hashBytesMatch(raw []byte, expected string) bool {
	if len(raw) != sha256.Size {
		return false
	}
	return "sha256:"+hex.EncodeToString(raw) == expected
}

func parseUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, errors.New("owner returned invalid receipt UUID")
	}
	return id, nil
}
