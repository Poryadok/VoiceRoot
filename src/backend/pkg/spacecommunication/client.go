package spacecommunication

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type ClientConfig struct{ Address, CAFile, CertFile, KeyFile string }

func LoadClientConfig(lookup func(string) (string, bool), prefix string) (ClientConfig, bool, error) {
	configured := false
	getenv := func(suffix string) string {
		value, set := lookup(prefix + suffix)
		configured = configured || set
		return strings.TrimSpace(value)
	}
	cfg := ClientConfig{Address: getenv("GRPC_ADDR"), CAFile: getenv("TLS_CA_FILE"), CertFile: getenv("CLIENT_CERT_FILE"), KeyFile: getenv("CLIENT_KEY_FILE")}
	count := 0
	for _, value := range []string{cfg.Address, cfg.CAFile, cfg.CertFile, cfg.KeyFile} {
		if value != "" {
			count++
		}
	}
	if !configured {
		return cfg, false, nil
	}
	if count != 4 {
		return cfg, true, errors.New("space communication client requires address, CA, client certificate and key")
	}
	return cfg, true, nil
}

func (c ClientConfig) Dial() (*grpc.ClientConn, error) {
	if c.Address == "" || c.CAFile == "" || c.CertFile == "" || c.KeyFile == "" {
		return nil, errors.New("incomplete Space communication transport")
	}
	pem, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("space communication CA has no certificates")
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(grpcclient.DialTarget(c.Address), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}})))
}

// Sign replaces all prior metadata and issues a fresh exact request credential.
// The caller owns the shared admission deadline through the subsequent commit.
func Sign(ctx context.Context, issuer *principal.Issuer, method string, req proto.Message) (context.Context, error) {
	if issuer == nil {
		return nil, errors.New("communication principal issuer unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, err
	}
	requestID := uuid.NewString()
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, err
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}
