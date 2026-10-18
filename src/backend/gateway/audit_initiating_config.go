package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/grpcclient"
)

var auditInitiatingServices = [...]string{"ROLE", "CHAT", "BOT_ADMIN"}

type auditInitiatingTransport struct {
	service string
	conn    *grpc.ClientConn
	err     error
}

func auditInitiatingClientConfigFromEnv(service string) (lifecycleClientConfig, bool, error) {
	if service != "ROLE" && service != "CHAT" && service != "BOT_ADMIN" {
		return lifecycleClientConfig{}, false, fmt.Errorf("unknown initiating transport service %q", service)
	}
	var values [5]string
	active := false
	for i, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		value, present := os.LookupEnv("GATEWAY_" + service + "_AUDIT_" + suffix)
		active = active || present
		values[i] = value
	}
	c := lifecycleClientConfig{values[0], values[1], values[2], values[3], values[4]}
	if !active {
		return c, false, nil
	}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return c, true, fmt.Errorf("gateway %s initiating TLS configuration is incomplete", service)
		}
	}
	_, err := c.tlsConfig()
	return c, true, err
}

func (c *grpcClients) configureAuditInitiatingTransports() {
	for _, service := range auditInitiatingServices {
		cfg, enabled, err := auditInitiatingClientConfigFromEnv(service)
		if !enabled && err == nil {
			continue
		}
		transport := auditInitiatingTransport{service: service, err: err}
		if err == nil {
			transport.conn, transport.err = cfg.dial()
			if transport.err == nil {
				c.connections = append(c.connections, transport.conn)
				switch service {
				case "ROLE":
					c.roleAudit = rolev1.NewRoleServiceClient(transport.conn)
				case "CHAT":
					c.chatAudit = chatv1.NewChatServiceClient(transport.conn)
				case "BOT_ADMIN":
					c.botAuditAdmin = botv1.NewBotServiceClient(transport.conn)
				}
			}
		}
		c.auditInitiatingTransports = append(c.auditInitiatingTransports, transport)
	}
}

func (c *grpcClients) waitAuditInitiatingReady(ctx context.Context) error {
	for _, transport := range c.auditInitiatingTransports {
		if transport.err != nil {
			return fmt.Errorf("%s initiating transport: %w", transport.service, transport.err)
		}
		if transport.conn == nil {
			return fmt.Errorf("%s initiating connection is not configured", transport.service)
		}
		transport.conn.Connect()
		if err := grpcclient.WaitForReady(ctx, transport.conn); err != nil {
			return fmt.Errorf("%s initiating readiness: %w", transport.service, err)
		}
	}
	return nil
}
