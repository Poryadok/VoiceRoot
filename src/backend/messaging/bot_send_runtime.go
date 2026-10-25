package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"reflect"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	botv1 "voice.app/voice/bot/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/messaging/internal/botsendprincipal"
	"voice/backend/messaging/internal/grpcsvc"
	"voice/backend/pkg/principal"
)

type botSendRuntime struct {
	server      *grpc.Server
	listener    net.Listener
	connections []*grpc.ClientConn
	errors      chan error
}

func (r *botSendRuntime) Close() {
	if r != nil {
		r.server.Stop()
		_ = r.listener.Close()
		for _, c := range r.connections {
			_ = c.Close()
		}
	}
}

func newBotSendRuntime(getenv func(string) string, service *grpcsvc.MessagingGRPC, verifier botsendprincipal.Verifier, issuer *principal.Issuer) (*botSendRuntime, error) {
	names := []string{"MESSAGING_BOT_SEND_GRPC_LISTEN", "MESSAGING_BOT_SEND_TLS_CERT_FILE", "MESSAGING_BOT_SEND_TLS_KEY_FILE", "MESSAGING_BOT_SEND_CLIENT_CA_FILE"}
	for _, prefix := range []string{"MESSAGING_BOT_SEND_AUTHORITY_", "MESSAGING_BOT_SEND_SPACE_"} {
		for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
			names = append(names, prefix+suffix)
		}
	}
	values := make([]string, len(names))
	configured := 0
	for i, name := range names {
		values[i] = strings.TrimSpace(getenv(name))
		if values[i] != "" {
			configured++
		}
	}
	if configured == 0 {
		return nil, nil
	}
	if configured != len(names) || service == nil || service.Messages == nil || service.ChatThreadPolicy == nil || verifier == nil || (reflect.ValueOf(verifier).Kind() == reflect.Ptr && reflect.ValueOf(verifier).IsNil()) || issuer == nil {
		return nil, errors.New("complete Bot send mTLS, owner clients, signer and verifier required")
	}
	certificate, err := tls.LoadX509KeyPair(values[1], values[2])
	if err != nil {
		return nil, errors.New("bot send server certificate unavailable")
	}
	raw, err := os.ReadFile(values[3])
	if err != nil {
		return nil, errors.New("bot send client CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(raw) {
		return nil, errors.New("bot send client CA invalid")
	}
	bot, err := dialManagedChatPurgeOwner(values[4], values[5], values[6], values[7], values[8])
	if err != nil {
		return nil, err
	}
	space, err := dialManagedChatPurgeOwner(values[9], values[10], values[11], values[12], values[13])
	if err != nil {
		_ = bot.Close()
		return nil, err
	}
	listener, err := net.Listen("tcp", values[0])
	if err != nil {
		_ = bot.Close()
		_ = space.Close()
		return nil, err
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})), grpc.UnaryInterceptor(botsendprincipal.UnaryServerInterceptor(verifier)), grpc.StreamInterceptor(botsendprincipal.DenyStreams))
	descriptor := messagingv1.MessagingService_ServiceDesc
	descriptor.Methods = nil
	descriptor.Streams = nil
	for _, method := range messagingv1.MessagingService_ServiceDesc.Methods {
		if method.MethodName == "SendBotMessage" {
			descriptor.Methods = append(descriptor.Methods, method)
		}
	}
	server.RegisterService(&descriptor, &grpcsvc.BotSendGRPC{Messaging: service, Authority: &grpcsvc.BotSendAuthority{Bot: botv1.NewBotServiceClient(bot), Space: spacev1.NewSpaceServiceClient(space), Issuer: issuer, Scope: service.ChatThreadPolicy}})
	runtime := &botSendRuntime{server: server, listener: listener, connections: []*grpc.ClientConn{bot, space}, errors: make(chan error, 1)}
	go func() { runtime.errors <- server.Serve(listener); close(runtime.errors) }()
	return runtime, nil
}
