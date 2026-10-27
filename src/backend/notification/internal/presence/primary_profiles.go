package presence

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"os"
	"strings"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

var ErrPrimaryUnavailable = errors.New("authoritative primary profile unavailable")

type PrimaryProfilesConfig struct{ Address, ServerName, CAFile, ClientCertFile, ClientKeyFile string }

// PrimaryProfiles uses a separate mTLS connection and the already configured
// Notification workload signer. It never falls back to internal caller headers.
type PrimaryProfiles struct {
	client userv1.UserServiceClient
	issuer *principal.Issuer
	conn   *grpc.ClientConn
}

func (c *GRPCChecker) NewPrimaryProfiles(cfg PrimaryProfilesConfig) (*PrimaryProfiles, error) {
	if c == nil || c.issuer == nil {
		return nil, ErrPrimaryUnavailable
	}
	for _, value := range []string{cfg.Address, cfg.ServerName, cfg.CAFile, cfg.ClientCertFile, cfg.ClientKeyFile} {
		if strings.TrimSpace(value) == "" {
			return nil, errors.New("complete primary profile mTLS configuration required")
		}
	}
	ca, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, errors.New("primary profile CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errors.New("primary profile CA invalid")
	}
	cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
	if err != nil {
		return nil, errors.New("primary profile client identity unavailable")
	}
	transport := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: cfg.ServerName, Certificates: []tls.Certificate{cert}})
	conn, err := grpc.NewClient(grpcclient.DialTarget(cfg.Address), grpc.WithTransportCredentials(transport))
	if err != nil {
		return nil, err
	}
	return &PrimaryProfiles{client: userv1.NewUserServiceClient(conn), issuer: c.issuer, conn: conn}, nil
}

func (c *GRPCChecker) PrimaryProfilesFromEnv(lookup func(string) (string, bool)) (*PrimaryProfiles, error) {
	names := []string{"USER_NOTIFICATION_PRIMARY_GRPC_ADDR", "USER_NOTIFICATION_PRIMARY_TLS_SERVER_NAME", "USER_NOTIFICATION_PRIMARY_TLS_CA_FILE", "USER_NOTIFICATION_PRIMARY_CLIENT_CERT_FILE", "USER_NOTIFICATION_PRIMARY_CLIENT_KEY_FILE"}
	values := make([]string, len(names))
	enabled := false
	if lookup == nil {
		return nil, errors.New("primary profile environment unavailable")
	}
	for i, name := range names {
		value, present := lookup(name)
		values[i] = strings.TrimSpace(value)
		enabled = enabled || present
	}
	if !enabled {
		return nil, nil
	}
	return c.NewPrimaryProfiles(PrimaryProfilesConfig{Address: values[0], ServerName: values[1], CAFile: values[2], ClientCertFile: values[3], ClientKeyFile: values[4]})
}

func (r *PrimaryProfiles) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}
	return r.conn.Close()
}

// PrimaryProfileForAccount returns exactly the existing User-owned primary.
// Missing/no-primary/deleted and unavailable responses cannot choose a recipient.
// The caller retries authority failure within its reminder window only.
func (r *PrimaryProfiles) PrimaryProfileForAccount(ctx context.Context, account uuid.UUID) (uuid.UUID, error) {
	if r == nil || r.client == nil || r.issuer == nil || account == uuid.Nil {
		return uuid.Nil, ErrPrimaryUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req := &userv1.ResolvePrimaryProfileIDsRequest{AccountIds: []string{account.String()}}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return uuid.Nil, ErrPrimaryUnavailable
	}
	requestID := uuid.NewString()
	token, err := r.issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: userv1.UserService_ResolvePrimaryProfileIDs_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return uuid.Nil, ErrPrimaryUnavailable
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	response, err := r.client.ResolvePrimaryProfileIDs(ctx, req)
	if err != nil {
		return uuid.Nil, err
	}
	if response == nil || len(response.PrimaryProfileIds) != 1 || len(response.ProtoReflect().GetUnknown()) != 0 {
		return uuid.Nil, ErrPrimaryUnavailable
	}
	raw, ok := response.PrimaryProfileIds[account.String()]
	if !ok {
		return uuid.Nil, ErrPrimaryUnavailable
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, ErrPrimaryUnavailable
	}
	return id, nil
}
