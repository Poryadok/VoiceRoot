// Package sessionfloor reads Auth's current account epoch without retaining
// positive authority between decisions.
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

const RPC = authv1.AuthService_GetRoleSessionEpochFloor_FullMethodName

type AuthClient interface {
	GetRoleSessionEpochFloor(context.Context, *authv1.GetRoleSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetRoleSessionEpochFloorResponse, error)
}

type Client struct {
	service  AuthClient
	issuer   *principal.Issuer
	deadline time.Duration
}

func New(service AuthClient, issuer *principal.Issuer) *Client {
	return &Client{service: service, issuer: issuer, deadline: 2 * time.Second}
}

func (c *Client) CurrentFloor(ctx context.Context, accountID string) (int64, error) {
	unavailable := status.Error(codes.Unavailable, "Auth session epoch floor unavailable")
	if c == nil || c.service == nil || c.issuer == nil || ctx == nil || c.deadline <= 0 || c.deadline > 2*time.Second {
		return 0, unavailable
	}
	id, err := uuid.Parse(accountID)
	if err != nil || id == uuid.Nil || id.String() != accountID {
		return 0, status.Error(codes.Unauthenticated, "invalid delegated identity")
	}
	req := &authv1.GetRoleSessionEpochFloorRequest{AccountId: accountID}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return 0, unavailable
	}
	requestID := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: RPC, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return 0, unavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	reply, err := c.service.GetRoleSessionEpochFloor(callCtx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return 0, status.Error(codes.Unauthenticated, "delegated account is unavailable")
		}
		return 0, unavailable
	}
	if reply == nil || reply.SessionEpochFloor <= 0 || len(reply.ProtoReflect().GetUnknown()) != 0 {
		return 0, unavailable
	}
	return reply.SessionEpochFloor, nil
}

func (c *Client) RequireCurrent(ctx context.Context, accountID string, sessionEpoch int64) error {
	if sessionEpoch <= 0 {
		return status.Error(codes.Unauthenticated, "invalid delegated session epoch")
	}
	floor, err := c.CurrentFloor(ctx, accountID)
	if err != nil {
		return err
	}
	if sessionEpoch < floor {
		return status.Error(codes.Unauthenticated, "delegated session epoch is stale")
	}
	return nil
}

// Config has no insecure or implicit Voice-service fallback.
type Config struct {
	Address, CAFile, ServerName, ClientCertFile, ClientKeyFile string
}

func ConfigFromEnv() (Config, bool, error) {
	names := []string{"ROLE_AUTH_SESSION_FLOOR_GRPC_ADDR", "ROLE_AUTH_SESSION_FLOOR_TLS_CA_FILE", "ROLE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME", "ROLE_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE", "ROLE_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE"}
	values := make([]string, len(names))
	enabled := false
	for i, name := range names {
		value, present := os.LookupEnv(name)
		enabled = enabled || present
		values[i] = value
	}
	if !enabled {
		return Config{}, false, nil
	}
	config := Config{Address: values[0], CAFile: values[1], ServerName: values[2], ClientCertFile: values[3], ClientKeyFile: values[4]}
	if !config.complete() {
		return Config{}, true, errors.New("Role Auth session-floor TLS configuration is incomplete")
	}
	return config, true, nil
}

func (c Config) complete() bool {
	for _, value := range []string{c.Address, c.CAFile, c.ServerName, c.ClientCertFile, c.ClientKeyFile} {
		if value == "" || strings.TrimSpace(value) != value {
			return false
		}
	}
	return true
}

func (c Config) Dial(issuer *principal.Issuer) (*Client, *grpc.ClientConn, error) {
	if issuer == nil || !c.complete() {
		return nil, nil, errors.New("Role Auth session-floor client configuration is incomplete")
	}
	caPEM, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, nil, errors.New("Role Auth session-floor CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, nil, errors.New("Role Auth session-floor CA invalid")
	}
	cert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientKeyFile)
	if err != nil {
		return nil, nil, errors.New("Role Auth session-floor client certificate unavailable")
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(c.Address), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: c.ServerName, Certificates: []tls.Certificate{cert}})))
	if err != nil {
		return nil, nil, errors.New("Role Auth session-floor transport unavailable")
	}
	return New(authv1.NewAuthServiceClient(conn), issuer), conn, nil
}

var _ principal.SessionEpochChecker = (*Client)(nil).RequireCurrent
