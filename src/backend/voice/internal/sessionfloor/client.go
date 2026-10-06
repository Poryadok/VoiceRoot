package sessionfloor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

const (
	RPC      = "/voice.auth.v1.AuthService/GetVoiceSessionEpochFloor"
	Audience = "auth"
)

type Client struct {
	service  authv1.AuthServiceClient
	issuer   *principal.Issuer
	deadline time.Duration
}

type Config struct {
	Address, CAFile, ServerName, ClientCertFile, ClientKeyFile string
}

func ConfigFromEnv() (Config, bool, error) {
	config := Config{
		Address:        strings.TrimSpace(os.Getenv("VOICE_AUTH_SESSION_FLOOR_GRPC_ADDR")),
		CAFile:         strings.TrimSpace(os.Getenv("VOICE_AUTH_SESSION_FLOOR_TLS_CA_FILE")),
		ServerName:     strings.TrimSpace(os.Getenv("VOICE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME")),
		ClientCertFile: strings.TrimSpace(os.Getenv("VOICE_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE")),
		ClientKeyFile:  strings.TrimSpace(os.Getenv("VOICE_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE")),
	}
	values := []string{config.Address, config.CAFile, config.ServerName, config.ClientCertFile, config.ClientKeyFile}
	active := false
	for _, value := range values {
		active = active || value != ""
	}
	if !active {
		return Config{}, false, nil
	}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return Config{}, true, errors.New("Voice Auth session-floor TLS configuration is incomplete")
		}
	}
	return config, true, nil
}

func (c Config) Dial(issuer *principal.Issuer) (*Client, *grpc.ClientConn, error) {
	if issuer == nil || strings.TrimSpace(c.Address) == "" || strings.TrimSpace(c.ServerName) == "" {
		return nil, nil, errors.New("Voice Auth session-floor client configuration is incomplete")
	}
	caPEM, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, nil, errors.New("Voice Auth session-floor CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, nil, errors.New("Voice Auth session-floor CA invalid")
	}
	certificate, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientKeyFile)
	if err != nil {
		return nil, nil, errors.New("Voice Auth session-floor client certificate unavailable")
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(c.Address), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: c.ServerName,
		Certificates: []tls.Certificate{certificate},
	})))
	if err != nil {
		return nil, nil, errors.New("Voice Auth session-floor transport unavailable")
	}
	return New(authv1.NewAuthServiceClient(conn), issuer), conn, nil
}

func New(service authv1.AuthServiceClient, issuer *principal.Issuer) *Client {
	return &Client{service: service, issuer: issuer, deadline: 2 * time.Second}
}

// RequireCurrent fails closed unless Auth confirms that the credential's epoch
// is at least the current durable account epoch and retained floor.
func (c *Client) RequireCurrent(ctx context.Context, accountID string, sessionEpoch int64) error {
	if c == nil || c.service == nil || c.issuer == nil || ctx == nil || sessionEpoch <= 0 {
		return status.Error(codes.Unavailable, "Auth session epoch floor unavailable")
	}
	parsed, err := uuid.Parse(accountID)
	if err != nil || parsed.String() != accountID {
		return status.Error(codes.Unauthenticated, "invalid delegated identity")
	}
	request := &authv1.GetVoiceSessionEpochFloorRequest{AccountId: accountID}
	requestID := uuid.NewString()
	hash, err := principal.RequestHash(request)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid Auth floor request")
	}
	token, err := c.issuer.IssueService(principal.ServiceInput{
		Audience: Audience, RPC: RPC, RequestID: requestID, RequestHash: hash,
	})
	if err != nil {
		return status.Error(codes.Unavailable, "Auth session epoch floor unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs(
		"authorization", "Bearer "+token,
		"x-request-id", requestID,
	))
	response, err := c.service.GetVoiceSessionEpochFloor(callCtx, request)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return status.Error(codes.Unauthenticated, "delegated account is unavailable")
		}
		return status.Error(codes.Unavailable, "Auth session epoch floor unavailable")
	}
	floor := response.GetSessionEpochFloor()
	if floor <= 0 {
		return status.Error(codes.Unavailable, "Auth session epoch floor unavailable")
	}
	if sessionEpoch < floor {
		return status.Error(codes.Unauthenticated, "delegated session epoch is stale")
	}
	return nil
}

func CanonicalAccountID(value string) bool {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil && parsed.String() == value
}

var _ principal.SessionEpochChecker = (*Client)(nil).RequireCurrent
