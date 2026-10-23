package main

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/pkg/principal"
)

// newRolePermissionsClient has no plaintext or unsigned compatibility path.
func newRolePermissionsClient(getenv func(string) string, issuer *principal.Issuer) (*s2s.GRPCRolePermissions, *grpc.ClientConn, error) {
	names := []string{"ROLE_GRPC_ADDR", "MESSAGING_ROLE_TLS_CA_FILE", "MESSAGING_ROLE_TLS_SERVER_NAME", "MESSAGING_ROLE_CLIENT_CERT_FILE", "MESSAGING_ROLE_CLIENT_KEY_FILE"}
	values := make([]string, len(names))
	configured := false
	for i, name := range names {
		values[i] = strings.TrimSpace(getenv(name))
		configured = configured || values[i] != ""
	}
	if !configured {
		return nil, nil, nil
	}
	for i, value := range values {
		if value == "" {
			return nil, nil, fmt.Errorf("Role permission client requires %s", names[i])
		}
	}
	if issuer == nil {
		return nil, nil, errors.New("Role permission client requires Messaging signing keys and protected JWKS publisher")
	}
	conn, err := dialManagedChatPurgeOwner(values[0], values[1], values[2], values[3], values[4])
	if err != nil {
		return nil, nil, err
	}
	return &s2s.GRPCRolePermissions{Client: rolev1.NewRoleServiceClient(conn), Issuer: issuer}, conn, nil
}
