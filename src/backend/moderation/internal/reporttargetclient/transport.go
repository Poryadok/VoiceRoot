package reporttargetclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"strings"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

// DialFromEnv fails partial configuration closed. Every endpoint is distinct
// from ordinary RPC clients and disables automatic retry of replay-bound calls.
func DialFromEnv(issuer *principal.Issuer) (*Client, func(), error) {
	const prefix = "MODERATION_REPORT_TARGETS_"
	cert, err := tls.LoadX509KeyPair(os.Getenv(prefix+"CLIENT_CERT_FILE"), os.Getenv(prefix+"CLIENT_KEY_FILE"))
	if err != nil {
		return nil, nil, err
	}
	ca, err := os.ReadFile(os.Getenv(prefix + "TLS_CA_FILE"))
	if err != nil {
		return nil, nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, nil, errors.New("report owner CA invalid")
	}
	connections := map[string]grpc.ClientConnInterface{}
	var conns []*grpc.ClientConn
	cleanup := func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}
	for target, owner := range map[string]string{"user": "USER", "message": "MESSAGING", "space": "SPACE", "story": "STORY"} {
		addr := os.Getenv(prefix + owner + "_GRPC_ADDR")
		server := os.Getenv(prefix + owner + "_TLS_SERVER_NAME")
		if addr == "" || server == "" || addr != strings.TrimSpace(addr) || server != strings.TrimSpace(server) {
			cleanup()
			return nil, nil, errors.New("report owner transport incomplete")
		}
		conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: server, Certificates: []tls.Certificate{cert}})), grpc.WithDisableRetry())
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		conns = append(conns, conn)
		connections[target] = conn
	}
	return &Client{Issuer: issuer, Connections: connections}, cleanup, nil
}
