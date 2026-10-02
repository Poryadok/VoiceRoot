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
	"voice/backend/pkg/grpcclient"
)

type spacePurgeOwnerConfig struct {
	MessagingAddr, MessagingCA, MessagingServerName string
	FileAddr, FileCA, FileServerName                string
	ClientCert, ClientKey                           string
}

func spacePurgeOwnerConfigFromEnv(getenv func(string) string) spacePurgeOwnerConfig {
	return spacePurgeOwnerConfig{
		MessagingAddr:       strings.TrimSpace(getenv("CHAT_SPACE_PURGE_MESSAGING_GRPC_ADDR")),
		MessagingCA:         strings.TrimSpace(getenv("CHAT_SPACE_PURGE_MESSAGING_TLS_CA_FILE")),
		MessagingServerName: strings.TrimSpace(getenv("CHAT_SPACE_PURGE_MESSAGING_TLS_SERVER_NAME")),
		FileAddr:            strings.TrimSpace(getenv("CHAT_SPACE_PURGE_FILE_GRPC_ADDR")),
		FileCA:              strings.TrimSpace(getenv("CHAT_SPACE_PURGE_FILE_TLS_CA_FILE")),
		FileServerName:      strings.TrimSpace(getenv("CHAT_SPACE_PURGE_FILE_TLS_SERVER_NAME")),
		ClientCert:          strings.TrimSpace(getenv("CHAT_SPACE_PURGE_CLIENT_CERT_FILE")),
		ClientKey:           strings.TrimSpace(getenv("CHAT_SPACE_PURGE_CLIENT_KEY_FILE")),
	}
}

func (c spacePurgeOwnerConfig) configured() bool {
	return c.MessagingAddr != "" || c.MessagingCA != "" || c.MessagingServerName != "" || c.FileAddr != "" || c.FileCA != "" || c.FileServerName != "" || c.ClientCert != "" || c.ClientKey != ""
}

func (c spacePurgeOwnerConfig) validate() error {
	for name, value := range map[string]string{
		"CHAT_SPACE_PURGE_MESSAGING_GRPC_ADDR":       c.MessagingAddr,
		"CHAT_SPACE_PURGE_MESSAGING_TLS_CA_FILE":     c.MessagingCA,
		"CHAT_SPACE_PURGE_MESSAGING_TLS_SERVER_NAME": c.MessagingServerName,
		"CHAT_SPACE_PURGE_FILE_GRPC_ADDR":            c.FileAddr,
		"CHAT_SPACE_PURGE_FILE_TLS_CA_FILE":          c.FileCA,
		"CHAT_SPACE_PURGE_FILE_TLS_SERVER_NAME":      c.FileServerName,
		"CHAT_SPACE_PURGE_CLIENT_CERT_FILE":          c.ClientCert,
		"CHAT_SPACE_PURGE_CLIENT_KEY_FILE":           c.ClientKey,
	} {
		if value == "" {
			return fmt.Errorf("Chat Space purge runtime requires %s", name)
		}
	}
	return nil
}

func dialSpacePurgeOwner(address, caFile, serverName, certFile, keyFile string) (*grpc.ClientConn, error) {
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
	return grpc.NewClient(grpcclient.DialTarget(address), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName, Certificates: []tls.Certificate{cert},
	})))
}
