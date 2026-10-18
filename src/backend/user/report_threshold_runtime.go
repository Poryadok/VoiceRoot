package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/user/internal/grpcsvc"
	"voice/backend/user/internal/reportaudience"
	authv1 "voice/backend/user/pb/voice/auth/v1"
)

const reportAuthorityPrefix = "USER_REPORT_THRESHOLD_PRINCIPAL_"
const userPrincipalJWKSPath = "/.well-known/principal-jwks.json"

type reportAuthorityRuntime struct {
	server    *grpc.Server
	listener  net.Listener
	principal *socialprincipal.Runtime
	auth      *grpc.ClientConn
	jwks      http.Handler
}

// newReportAuthorityRuntime has no insecure or ordinary-listener fallback.
// Any partial configured serving set is a startup error, rather than a lower denominator.
func newReportAuthorityRuntime(ctx context.Context, pool *pgxpool.Pool, options []grpc.ServerOption) (result *reportAuthorityRuntime, returnErr error) {
	cfg, enabled, err := socialprincipal.LoadFromEnvWithAudience("user", "moderation-report-threshold", reportAuthorityPrefix, ":9466")
	if err != nil {
		return nil, err
	}
	fields := []string{"AUTH_ADDR", "AUTH_CA_FILE", "AUTH_TLS_SERVER_NAME", "SIGNING_KID", "SIGNING_KEY_FILE", "NEXT_SIGNING_KID", "NEXT_SIGNING_KEY_FILE"}
	if !enabled {
		for _, field := range fields {
			if _, set := os.LookupEnv(reportAuthorityPrefix + field); set {
				return nil, errors.New("partial User report authority configuration")
			}
		}
		return nil, nil
	}
	if pool == nil {
		return nil, errors.New("User report authority requires database")
	}
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		values[field] = strings.TrimSpace(os.Getenv(reportAuthorityPrefix + field))
		if values[field] == "" {
			return nil, errors.New("complete User report authority configuration required")
		}
	}
	validKID := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	if !validKID.MatchString(values["SIGNING_KID"]) || !validKID.MatchString(values["NEXT_SIGNING_KID"]) || values["SIGNING_KID"] == values["NEXT_SIGNING_KID"] {
		return nil, errors.New("invalid User report authority rotation keys")
	}
	current, err := principal.LoadRSAPrivateKeyFile(values["SIGNING_KEY_FILE"])
	if err != nil {
		return nil, err
	}
	next, err := principal.LoadRSAPrivateKeyFile(values["NEXT_SIGNING_KEY_FILE"])
	if err != nil {
		return nil, err
	}
	if current.N.BitLen() < 2048 || next.N.BitLen() < 2048 || current.N.Cmp(next.N) == 0 {
		return nil, errors.New("distinct strong User report authority rotation keys required")
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: values["SIGNING_KID"], PrivateKey: current})
	if err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(values["AUTH_CA_FILE"])
	if err != nil {
		return nil, err
	}
	ca := x509.NewCertPool()
	if !ca.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("invalid Auth audience CA")
	}
	identity, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, err
	}
	result = &reportAuthorityRuntime{}
	owned := result
	defer func() {
		if returnErr != nil {
			owned.Close()
			result = nil
		}
	}()
	result.auth, err = grpc.NewClient(grpcclient.DialTarget(values["AUTH_ADDR"]), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: ca, ServerName: values["AUTH_TLS_SERVER_NAME"], Certificates: []tls.Certificate{identity}})))
	if err != nil {
		return nil, err
	}
	result.principal, err = socialprincipal.New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	result.listener, err = net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	serverOptions := append([]grpc.ServerOption{}, options...)
	serverOptions = append(serverOptions, result.principal.ServerOptions()...)
	result.server = grpc.NewServer(serverOptions...)
	grpcsvc.RegisterReportThresholdServer(result.server, reportaudience.Service{Source: reportaudience.PostgresSource{Pool: pool}, Classifier: reportaudience.AuthClient{Client: authv1.NewAuthServiceClient(result.auth), Issuer: issuer}})
	result.jwks = principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: values["SIGNING_KID"], PublicKey: &current.PublicKey}, {KeyID: values["NEXT_SIGNING_KID"], PublicKey: &next.PublicKey}})
	go func() { _ = result.server.Serve(result.listener) }()
	return result, nil
}

func (r *reportAuthorityRuntime) Close() {
	if r == nil {
		return
	}
	if r.server != nil {
		r.server.Stop()
	}
	if r.listener != nil {
		_ = r.listener.Close()
	}
	if r.principal != nil {
		_ = r.principal.Close()
	}
	if r.auth != nil {
		_ = r.auth.Close()
	}
}

func (r *reportAuthorityRuntime) HTTP(next http.Handler) http.Handler {
	if r == nil {
		return next
	}
	mux := http.NewServeMux()
	mux.Handle(userPrincipalJWKSPath, r.jwks)
	mux.Handle("/", next)
	return mux
}
