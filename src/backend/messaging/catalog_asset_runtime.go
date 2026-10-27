package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/grpcsvc"
	"voice/backend/pkg/grpcclient"
)

type catalogAssetEndpoint struct{ address, ca, serverName, cert, key string }
type catalogAssetConfig struct{ file, chat catalogAssetEndpoint }

// Presence, including an explicitly empty value, opts into the complete trust
// configuration. Ordinary FILE_GRPC_ADDR/CHAT_GRPC_ADDR are never consulted.
func loadCatalogAssetConfig(lookup func(string) (string, bool)) (catalogAssetConfig, bool, error) {
	var config catalogAssetConfig
	enabled := false
	for _, owner := range []struct {
		name     string
		endpoint *catalogAssetEndpoint
	}{{"FILE", &config.file}, {"CHAT", &config.chat}} {
		fields := []*string{&owner.endpoint.address, &owner.endpoint.ca, &owner.endpoint.serverName, &owner.endpoint.cert, &owner.endpoint.key}
		for index, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "TLS_CLIENT_CERT_FILE", "TLS_CLIENT_KEY_FILE"} {
			value, present := lookup("MESSAGING_CATALOG_ASSET_" + owner.name + "_" + suffix)
			enabled = enabled || present
			*fields[index] = strings.TrimSpace(value)
		}
	}
	if !enabled {
		return config, false, nil
	}
	for _, owner := range []catalogAssetEndpoint{config.file, config.chat} {
		if owner.address == "" || owner.ca == "" || owner.serverName == "" || owner.cert == "" || owner.key == "" {
			return config, true, errors.New("messaging catalog asset clients require complete File and Chat dedicated mTLS configuration")
		}
	}
	return config, true, nil
}

type catalogAssetRuntime struct {
	Coordinator *grpcsvc.CatalogAssetCoordinator
	connections []*grpc.ClientConn
}

// Activate only after the existing protected File lifecycle client is wired.
// A typed catalog source must never enter the legacy no-reference SQL path.
func attachCatalogAssetRuntime(service *grpcsvc.MessagingGRPC, runtime *catalogAssetRuntime) error {
	if runtime == nil {
		return nil
	}
	if service == nil || service.AttachmentReferences == nil || service.AttachmentReferences.Files == nil || service.AttachmentReferences.Issuer == nil {
		return errors.New("catalog asset admission requires protected File reference acquisition")
	}
	if runtime.Coordinator == nil || runtime.Coordinator.Issuer != service.AttachmentReferences.Issuer {
		return errors.New("catalog asset and reference acquisition require the same Messaging issuer")
	}
	service.CatalogAssets = runtime.Coordinator
	return nil
}

func (r *catalogAssetRuntime) Close() {
	if r == nil {
		return
	}
	for _, conn := range r.connections {
		_ = conn.Close()
	}
}

func newCatalogAssetRuntime(lookup func(string) (string, bool), messaging *managedChatPurgeRuntime) (*catalogAssetRuntime, error) {
	config, enabled, err := loadCatalogAssetConfig(lookup)
	if err != nil || !enabled {
		return nil, err
	}
	// This runtime constructs its issuer with the fixed Messaging identity and
	// publishes the corresponding rotation keyset over its protected JWKS listener.
	if messaging == nil || messaging.Issuer == nil || messaging.JWKS == nil {
		return nil, errors.New("catalog asset clients require Messaging signing keys and protected JWKS publisher")
	}
	runtime := &catalogAssetRuntime{}
	file, err := config.file.dial()
	if err != nil {
		return nil, fmt.Errorf("catalog File mTLS client: %w", err)
	}
	runtime.connections = append(runtime.connections, file)
	chat, err := config.chat.dial()
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("catalog Chat mTLS client: %w", err)
	}
	runtime.connections = append(runtime.connections, chat)
	runtime.Coordinator = &grpcsvc.CatalogAssetCoordinator{Files: filev1.NewCatalogAssetServiceClient(file), Chat: chatv1.NewCatalogAssetServiceClient(chat), Issuer: messaging.Issuer}
	return runtime, nil
}

func (c catalogAssetEndpoint) dial() (*grpc.ClientConn, error) {
	caPEM, err := os.ReadFile(c.ca)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("catalog authority CA contains no certificates")
	}
	cert, err := tls.LoadX509KeyPair(c.cert, c.key)
	if err != nil {
		return nil, err
	}
	// Lazy dialing avoids a startup dependency cycle. Each coordinator RPC has its
	// own two-second deadline and freshly bound principal; no ordinary fallback.
	return grpc.NewClient(grpcclient.DialTarget(c.address), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: c.serverName, Certificates: []tls.Certificate{cert}})), grpc.WithDisableRetry())
}
