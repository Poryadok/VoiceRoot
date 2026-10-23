package main

import (
	"strings"
	"testing"
)

func TestRolePermissionsClientRequiresCompleteProtectedConfig(t *testing.T) {
	names := []string{"ROLE_GRPC_ADDR", "MESSAGING_ROLE_TLS_CA_FILE", "MESSAGING_ROLE_TLS_SERVER_NAME", "MESSAGING_ROLE_CLIENT_CERT_FILE", "MESSAGING_ROLE_CLIENT_KEY_FILE"}
	for _, missing := range names {
		t.Run(missing, func(t *testing.T) {
			client, conn, err := newRolePermissionsClient(func(name string) string {
				if name == missing {
					return ""
				}
				return "configured"
			}, nil)
			if err == nil || !strings.Contains(err.Error(), missing) || client != nil || conn != nil {
				t.Fatal("partial Role transport admitted")
			}
		})
	}
	client, conn, err := newRolePermissionsClient(func(string) string { return "configured" }, nil)
	if err == nil || client != nil || conn != nil {
		t.Fatal("missing Messaging signing runtime admitted")
	}
	client, conn, err = newRolePermissionsClient(func(string) string { return "" }, nil)
	if err != nil || client != nil || conn != nil {
		t.Fatal("unconfigured transport should remain absent")
	}
}
