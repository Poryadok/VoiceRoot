package rolegrant

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

const callTimeout = 2 * time.Second

type Config struct {
	Address, CAFile, ClientCertFile, ClientKeyFile string
}

func LoadFromEnv(getenv func(string) string) (Config, bool, error) {
	if getenv == nil {
		return Config{}, false, errors.New("environment reader is required")
	}
	cfg := Config{
		Address:        strings.TrimSpace(getenv("VOICE_ROLE_GRPC_ADDR")),
		CAFile:         strings.TrimSpace(getenv("VOICE_ROLE_TLS_CA_FILE")),
		ClientCertFile: strings.TrimSpace(getenv("VOICE_ROLE_CLIENT_CERT_FILE")),
		ClientKeyFile:  strings.TrimSpace(getenv("VOICE_ROLE_CLIENT_KEY_FILE")),
	}
	values := []string{cfg.Address, cfg.CAFile, cfg.ClientCertFile, cfg.ClientKeyFile}
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
		return Config{}, true, errors.New("Voice-to-Role mTLS client requires address, CA, client certificate, and key")
	}
	return cfg, true, nil
}

type Checker struct {
	client rolev1.RoleServiceClient
	issuer *principal.Issuer
}

func NewChecker(client rolev1.RoleServiceClient, issuer *principal.Issuer) (*Checker, error) {
	if client == nil || issuer == nil {
		return nil, errors.New("Role client and Voice principal issuer are required")
	}
	return &Checker{client: client, issuer: issuer}, nil
}

func New(config Config, issuer *principal.Issuer) (*Checker, *grpc.ClientConn, error) {
	if issuer == nil {
		return nil, nil, errors.New("Voice principal issuer is required")
	}
	if strings.TrimSpace(config.Address) == "" {
		return nil, nil, errors.New("Role gRPC address is required")
	}
	caPEM, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read Role TLS CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, nil, errors.New("Role TLS CA contains no certificates")
	}
	clientCert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("load Voice Role client certificate: %w", err)
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(config.Address), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{clientCert},
	})))
	if err != nil {
		return nil, nil, fmt.Errorf("dial Role grant checker: %w", err)
	}
	checker, err := NewChecker(rolev1.NewRoleServiceClient(conn), issuer)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return checker, conn, nil
}

func (c *Checker) CheckGameSessionGrant(ctx context.Context, applicationID, environmentID, sessionID, voiceRoomID, profileID string) error {
	if c == nil || c.client == nil || c.issuer == nil {
		return status.Error(codes.Unavailable, "Role grant checker is unavailable")
	}
	ids := []string{applicationID, environmentID, sessionID, voiceRoomID, profileID}
	for _, value := range ids {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return status.Error(codes.PermissionDenied, "managed game session identity is invalid")
		}
	}
	request := &rolev1.CheckGameSessionGrantRequest{
		ApplicationId: applicationID, EnvironmentId: environmentID, SessionId: sessionID,
		VoiceRoomId: voiceRoomID, ProfileId: profileID,
	}
	requestID := uuid.NewString()
	requestHash, err := principal.RequestHash(request)
	if err != nil {
		return status.Error(codes.Internal, "Role grant request could not be bound")
	}
	token, err := c.issuer.IssueService(principal.ServiceInput{
		Audience: "role", RPC: rolev1.RoleService_CheckGameSessionGrant_FullMethodName,
		RequestID: requestID, RequestHash: requestHash,
	})
	if err != nil {
		return status.Error(codes.Internal, "Voice principal could not be issued")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	callCtx = metadata.AppendToOutgoingContext(callCtx, "authorization", "Bearer "+token, "x-request-id", requestID)
	response, err := c.client.CheckGameSessionGrant(callCtx, request)
	if err != nil {
		return err
	}
	if response == nil || !response.GetAllowed() {
		return status.Error(codes.PermissionDenied, "managed game session grant denied")
	}
	return nil
}
