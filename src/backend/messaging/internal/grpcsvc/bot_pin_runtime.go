package grpcsvc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/botpinprincipal"
	"voice/backend/messaging/internal/principalruntime"
	"voice/backend/messaging/internal/store"
)

type BotPinCleanupConfig struct {
	ListenAddr, TLSCertFile, TLSKeyFile, ClientCAFile                          string
	JWKSURL, JWKSCAFile, JWKSClientCertFile, JWKSClientKeyFile, ReplayRedisURL string
}

func BotPinCleanupConfigFromEnv(getenv func(string) string) BotPinCleanupConfig {
	value := func(suffix string) string { return strings.TrimSpace(getenv("MESSAGING_BOT_PIN_CLEANUP_" + suffix)) }
	return BotPinCleanupConfig{ListenAddr: value("GRPC_LISTEN"), TLSCertFile: value("TLS_CERT_FILE"), TLSKeyFile: value("TLS_KEY_FILE"), ClientCAFile: value("CLIENT_CA_FILE"), JWKSURL: value("JWKS_URL"), JWKSCAFile: value("JWKS_CA_FILE"), JWKSClientCertFile: value("JWKS_CLIENT_CERT_FILE"), JWKSClientKeyFile: value("JWKS_CLIENT_KEY_FILE"), ReplayRedisURL: value("REPLAY_REDIS_URL")}
}

type BotPinCleanupRuntime struct {
	server     *grpc.Server
	listener   net.Listener
	principal  *principalruntime.Runtime
	once       sync.Once
	serveError chan error
}

func StartBotPinCleanupServer(ctx context.Context, config BotPinCleanupConfig, pins *store.PinsStore) (*BotPinCleanupRuntime, error) {
	fields := []string{config.ListenAddr, config.TLSCertFile, config.TLSKeyFile, config.ClientCAFile, config.JWKSURL, config.JWKSCAFile, config.JWKSClientCertFile, config.JWKSClientKeyFile, config.ReplayRedisURL}
	configured := 0
	for _, value := range fields {
		if strings.TrimSpace(value) != "" {
			configured++
		}
	}
	if configured == 0 {
		return nil, nil
	}
	if configured != len(fields) || pins == nil || pins.Pool == nil || pins.BotPinChatScope == nil {
		return nil, errors.New("complete Bot pin cleanup mTLS/JWKS/replay/current-Chat-store configuration required")
	}
	certificate, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		return nil, errors.New("Bot pin cleanup server certificate unavailable")
	}
	caPEM, err := os.ReadFile(config.ClientCAFile)
	if err != nil {
		return nil, errors.New("Bot pin cleanup client CA unavailable")
	}
	clientRoots := x509.NewCertPool()
	if !clientRoots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Bot pin cleanup client CA invalid")
	}
	verifier, err := principalruntime.NewForIssuer(ctx, principalruntime.Config{JWKSURL: config.JWKSURL, TLSCertFile: config.JWKSClientCertFile, TLSKeyFile: config.JWKSClientKeyFile, CAFile: config.JWKSCAFile, RedisURL: config.ReplayRedisURL}, "bot")
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", config.ListenAddr)
	if err != nil {
		_ = verifier.Close()
		return nil, errors.New("Bot pin cleanup listener unavailable")
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots})), grpc.UnaryInterceptor(botpinprincipal.UnaryServerInterceptor(verifier)), grpc.StreamInterceptor(botpinprincipal.DenyStreams))
	service := messagingv1.MessagingService_ServiceDesc
	service.Methods = nil
	service.Streams = nil
	for _, method := range messagingv1.MessagingService_ServiceDesc.Methods {
		if method.MethodName == "ApplyBotChatPinRemoval" || method.MethodName == "GetBotPurgeChatCoverage" {
			service.Methods = append(service.Methods, method)
		}
	}
	server.RegisterService(&service, &BotPinCleanupGRPC{Pins: pins, Coverage: &store.MessagesStore{Pool: pins.Pool}})
	runtime := &BotPinCleanupRuntime{server: server, listener: listener, principal: verifier, serveError: make(chan error, 1)}
	go func() { runtime.serveError <- server.Serve(listener); close(runtime.serveError) }()
	return runtime, nil
}

func (r *BotPinCleanupRuntime) Address() string {
	if r == nil || r.listener == nil {
		return ""
	}
	return r.listener.Addr().String()
}
func (r *BotPinCleanupRuntime) Errors() <-chan error {
	if r == nil {
		return nil
	}
	return r.serveError
}
func (r *BotPinCleanupRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() { r.server.Stop(); _ = r.listener.Close(); _ = r.principal.Close() })
	return nil
}
