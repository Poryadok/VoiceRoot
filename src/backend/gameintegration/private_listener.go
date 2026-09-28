package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"

	"voice/backend/gameintegration/internal/httpapi"
	"voice/backend/pkg/httpserver"
)

func newMessagingPrivateServer(cfg config, handler http.Handler) (*http.Server, net.Listener, error) {
	certificate, err := tls.LoadX509KeyPair(cfg.MessagingTLSCertFile, cfg.MessagingTLSKeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("load Messaging TLS certificate: %w", err)
	}
	caPEM, err := os.ReadFile(cfg.MessagingClientCAFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read Messaging client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return nil, nil, fmt.Errorf("messaging client CA contains no certificates")
	}
	tlsConfig := httpapi.MessagingMTLSConfig(clientCAs)
	tlsConfig.Certificates = []tls.Certificate{certificate}
	tcpListener, err := net.Listen("tcp", cfg.MessagingListenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("bind Messaging private listener: %w", err)
	}
	server := &http.Server{Addr: cfg.MessagingListenAddr,
		Handler:   httpserver.Wrap(handler, httpserver.NewLogger("gameintegration-messaging-private")),
		TLSConfig: tlsConfig}
	httpserver.ApplyHTTPServerTimeouts(server)
	return server, tls.NewListener(tcpListener, tlsConfig), nil
}
