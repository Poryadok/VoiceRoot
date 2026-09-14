package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/principal"
	socials2s "voice/backend/social/internal/s2s"
)

type socialPrincipalRuntime struct {
	Issuer      *principal.Issuer
	User        *socials2s.GRPCUserPrivacy
	Space       *socials2s.GRPCSpaceCoMembership
	JWKS        *http.Server
	connections []*grpc.ClientConn
}

func (r *socialPrincipalRuntime) Close() {
	if r == nil {
		return
	}
	for _, conn := range r.connections {
		_ = conn.Close()
	}
	if r.JWKS != nil {
		_ = r.JWKS.Close()
	}
}

func loadSocialPrincipalRuntime() (_ *socialPrincipalRuntime, err error) {
	issuer, jwks, err := loadSocialPrincipalIssuerFromEnv()
	if err != nil {
		return nil, err
	}
	required := false
	for _, name := range []string{"USER_GRPC_ADDR", "SPACE_GRPC_ADDR", "USER_PRINCIPAL_GRPC_ADDR", "USER_PRINCIPAL_TLS_CA_FILE", "USER_PRINCIPAL_TLS_SERVER_NAME", "SPACE_PRINCIPAL_GRPC_ADDR", "SPACE_PRINCIPAL_TLS_CA_FILE", "SPACE_PRINCIPAL_TLS_SERVER_NAME", "SOCIAL_PRINCIPAL_JWKS_LISTEN", "SOCIAL_PRINCIPAL_TLS_CERT_FILE", "SOCIAL_PRINCIPAL_TLS_KEY_FILE"} {
		if _, set := os.LookupEnv(name); set {
			required = true
		}
	}
	if issuer == nil {
		if required {
			return nil, fmt.Errorf("social protected dependencies require principal signing and TLS configuration")
		}
		return &socialPrincipalRuntime{}, nil
	}
	runtime := &socialPrincipalRuntime{Issuer: issuer}
	defer func() {
		if err != nil {
			runtime.Close()
		}
	}()
	certFile := strings.TrimSpace(os.Getenv("SOCIAL_PRINCIPAL_TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("SOCIAL_PRINCIPAL_TLS_KEY_FILE"))
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("social principal JWKS TLS certificate and key are required")
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("social principal JWKS TLS: %w", err)
	}
	addr := ":8443"
	if value, set := os.LookupEnv("SOCIAL_PRINCIPAL_JWKS_LISTEN"); set {
		addr = strings.TrimSpace(value)
		if addr == "" {
			return nil, fmt.Errorf("social principal JWKS listen address is empty")
		}
	}
	document, err := json.Marshal(jwks)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=30")
		_, _ = w.Write(document)
	})
	runtime.JWKS = &http.Server{Addr: addr, Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}}
	httpserver.ApplyHTTPServerTimeouts(runtime.JWKS)
	for _, service := range []string{"USER", "SPACE"} {
		conn, connErr := socialProtectedClient(service)
		if connErr != nil {
			return nil, connErr
		}
		if conn == nil {
			continue
		}
		runtime.connections = append(runtime.connections, conn)
		if service == "USER" {
			runtime.User = &socials2s.GRPCUserPrivacy{Client: userv1.NewUserServiceClient(conn), Issuer: issuer}
		} else {
			runtime.Space = socials2s.NewGRPCSpaceCoMembership(conn)
			runtime.Space.Issuer = issuer
		}
	}
	return runtime, nil
}

func socialProtectedClient(service string) (*grpc.ClientConn, error) {
	address := strings.TrimSpace(os.Getenv(service + "_PRINCIPAL_GRPC_ADDR"))
	ca := strings.TrimSpace(os.Getenv(service + "_PRINCIPAL_TLS_CA_FILE"))
	serverName := strings.TrimSpace(os.Getenv(service + "_PRINCIPAL_TLS_SERVER_NAME"))
	configured := false
	for _, suffix := range []string{"_GRPC_ADDR", "_PRINCIPAL_GRPC_ADDR", "_PRINCIPAL_TLS_CA_FILE", "_PRINCIPAL_TLS_SERVER_NAME"} {
		if _, set := os.LookupEnv(service + suffix); set {
			configured = true
		}
	}
	if !configured {
		return nil, nil
	}
	if address == "" || ca == "" || serverName == "" {
		return nil, fmt.Errorf("%s principal address, TLS CA and server name are required", service)
	}
	data, err := os.ReadFile(ca)
	if err != nil {
		return nil, fmt.Errorf("%s principal TLS CA: %w", service, err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("%s principal TLS CA has no certificates", service)
	}
	// Application retries must re-enter the adapter to mint a fresh jti. gRPC
	// transparent retries before processing remain safe; service-config retries
	// after a verifier consumed the credential must not reuse it.
	return grpc.NewClient(grpcclient.DialTarget(address), grpc.WithDisableRetry(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName})))
}
