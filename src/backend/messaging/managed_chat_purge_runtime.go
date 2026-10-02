package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	filev1 "voice.app/voice/file/v1"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/messaging/internal/grpcsvc"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/grpcclient"
)

type managedChatPurgeRuntime struct {
	Coordinator *grpcsvc.ManagedChatPurgeCoordinator
	JWKS        *http.Server
	Connections []*grpc.ClientConn
}

func (r *managedChatPurgeRuntime) Close() {
	if r == nil {
		return
	}
	if r.JWKS != nil {
		_ = r.JWKS.Close()
	}
	for _, conn := range r.Connections {
		_ = conn.Close()
	}
}

func newManagedChatPurgeRuntime(ctx context.Context, pool *pgxpool.Pool) (*managedChatPurgeRuntime, error) {
	config := managedChatPurgeRuntimeConfigFromEnv(os.Getenv)
	issuer, handler, enabled, err := loadManagedChatPurgePrincipal()
	if err != nil {
		return nil, err
	}
	if !enabled {
		if config.configured() {
			return nil, errors.New("managed chat purge TLS/owner configuration requires the signing-key configuration")
		}
		return nil, nil
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, errors.New("managed chat purge requires Messaging PostgreSQL")
	}
	serverCert, err := tls.LoadX509KeyPair(config.JWKSCertFile, config.JWKSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("Messaging purge JWKS certificate: %w", err)
	}
	clientCAPEM, err := os.ReadFile(config.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("Messaging purge JWKS client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, errors.New("Messaging purge JWKS client CA contains no certificates")
	}
	listener, err := net.Listen("tcp", config.JWKSListen)
	if err != nil {
		return nil, fmt.Errorf("Messaging purge JWKS listen: %w", err)
	}
	runtime := &managedChatPurgeRuntime{JWKS: &http.Server{Handler: handler, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}}}
	go func() {
		if serveErr := runtime.JWKS.ServeTLS(listener, "", ""); serveErr != nil && serveErr != http.ErrServerClosed {
			// Serve failures are observable through health checks and client retries.
			fmt.Fprintf(os.Stderr, "Messaging purge JWKS server exited: %v\n", serveErr)
		}
	}()
	fileConn, err := dialManagedChatPurgeOwner(config.FileAddr, config.FileCA, config.FileServerName, config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("File managed chat purge client: %w", err)
	}
	runtime.Connections = append(runtime.Connections, fileConn)
	searchConn, err := dialManagedChatPurgeOwner(config.SearchAddr, config.SearchCA, config.SearchServerName, config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("Search managed chat purge client: %w", err)
	}
	runtime.Connections = append(runtime.Connections, searchConn)
	runtime.Coordinator = &grpcsvc.ManagedChatPurgeCoordinator{
		Store: &store.MessagesStore{Pool: pool}, Files: filev1.NewFileServiceClient(fileConn),
		Search: searchv1.NewSearchServiceClient(searchConn), Issuer: issuer,
	}
	return runtime, nil
}

func (c managedChatPurgeRuntimeConfig) configured() bool {
	return c.JWKSListen != "" || c.JWKSCertFile != "" || c.JWKSKeyFile != "" || c.ClientCAFile != "" ||
		c.ClientCertFile != "" || c.ClientKeyFile != "" || c.FileAddr != "" || c.FileCA != "" || c.FileServerName != "" ||
		c.SearchAddr != "" || c.SearchCA != "" || c.SearchServerName != ""
}

type managedChatPurgeRuntimeConfig struct {
	JWKSListen, JWKSCertFile, JWKSKeyFile, ClientCAFile string
	ClientCertFile, ClientKeyFile                       string
	FileAddr, FileCA, FileServerName                    string
	SearchAddr, SearchCA, SearchServerName              string
}

func managedChatPurgeRuntimeConfigFromEnv(getenv func(string) string) managedChatPurgeRuntimeConfig {
	return managedChatPurgeRuntimeConfig{
		JWKSListen:       strings.TrimSpace(getenv("MESSAGING_PURGE_PRINCIPAL_JWKS_LISTEN")),
		JWKSCertFile:     strings.TrimSpace(getenv("MESSAGING_PURGE_PRINCIPAL_TLS_CERT_FILE")),
		JWKSKeyFile:      strings.TrimSpace(getenv("MESSAGING_PURGE_PRINCIPAL_TLS_KEY_FILE")),
		ClientCAFile:     strings.TrimSpace(getenv("MESSAGING_PURGE_PRINCIPAL_CLIENT_CA_FILE")),
		ClientCertFile:   strings.TrimSpace(getenv("MESSAGING_PURGE_PRINCIPAL_CLIENT_CERT_FILE")),
		ClientKeyFile:    strings.TrimSpace(getenv("MESSAGING_PURGE_PRINCIPAL_CLIENT_KEY_FILE")),
		FileAddr:         strings.TrimSpace(getenv("FILE_MANAGED_CHAT_PURGE_GRPC_ADDR")),
		FileCA:           strings.TrimSpace(getenv("FILE_MANAGED_CHAT_PURGE_TLS_CA_FILE")),
		FileServerName:   strings.TrimSpace(getenv("FILE_MANAGED_CHAT_PURGE_TLS_SERVER_NAME")),
		SearchAddr:       strings.TrimSpace(getenv("SEARCH_MANAGED_CHAT_PURGE_GRPC_ADDR")),
		SearchCA:         strings.TrimSpace(getenv("SEARCH_MANAGED_CHAT_PURGE_TLS_CA_FILE")),
		SearchServerName: strings.TrimSpace(getenv("SEARCH_MANAGED_CHAT_PURGE_TLS_SERVER_NAME")),
	}
}

func (c managedChatPurgeRuntimeConfig) validate() error {
	for name, value := range map[string]string{
		"MESSAGING_PURGE_PRINCIPAL_JWKS_LISTEN":      c.JWKSListen,
		"MESSAGING_PURGE_PRINCIPAL_TLS_CERT_FILE":    c.JWKSCertFile,
		"MESSAGING_PURGE_PRINCIPAL_TLS_KEY_FILE":     c.JWKSKeyFile,
		"MESSAGING_PURGE_PRINCIPAL_CLIENT_CA_FILE":   c.ClientCAFile,
		"MESSAGING_PURGE_PRINCIPAL_CLIENT_CERT_FILE": c.ClientCertFile,
		"MESSAGING_PURGE_PRINCIPAL_CLIENT_KEY_FILE":  c.ClientKeyFile,
		"FILE_MANAGED_CHAT_PURGE_GRPC_ADDR":          c.FileAddr,
		"FILE_MANAGED_CHAT_PURGE_TLS_CA_FILE":        c.FileCA,
		"FILE_MANAGED_CHAT_PURGE_TLS_SERVER_NAME":    c.FileServerName,
		"SEARCH_MANAGED_CHAT_PURGE_GRPC_ADDR":        c.SearchAddr,
		"SEARCH_MANAGED_CHAT_PURGE_TLS_CA_FILE":      c.SearchCA,
		"SEARCH_MANAGED_CHAT_PURGE_TLS_SERVER_NAME":  c.SearchServerName,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("managed chat purge runtime requires %s", name)
		}
	}
	return nil
}

func dialManagedChatPurgeOwner(address, caFile, serverName, certFile, keyFile string) (*grpc.ClientConn, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("owner CA contains no certificates")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(address), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName, Certificates: []tls.Certificate{cert}})))
	if err != nil {
		return nil, err
	}
	// The Search process depends on Messaging's health check. Keep these
	// connections lazy so that the startup graph cannot deadlock; RPCs retry
	// through gRPC's resolver after both services have started.
	return conn, nil
}
