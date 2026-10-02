package main

import (
	"fmt"
	"strconv"
	"strings"
	commonv1 "voice.app/voice/common/v1"
)

type lifecycleOwnerConfig struct {
	ID                            commonv1.ParticipantId
	Audience, Address, ServerName string
}
type lifecycleRuntimeConfig struct {
	CAFile, CertFile, KeyFile, RoleCertFile, RoleKeyFile, Mode, DevTombstoneKeyFile string
	Owners                                                                          []lifecycleOwnerConfig
}

func loadLifecycleRuntimeConfig(getenv func(string) string) (lifecycleRuntimeConfig, bool, error) {
	c := lifecycleRuntimeConfig{}
	raw := strings.TrimSpace(getenv("SPACE_LIFECYCLE_ENABLED"))
	enabled := false
	if raw != "" {
		var err error
		enabled, err = strconv.ParseBool(raw)
		if err != nil {
			return c, false, fmt.Errorf("invalid SPACE_LIFECYCLE_ENABLED")
		}
	}
	if !enabled {
		return c, false, nil
	}
	c.CAFile = strings.TrimSpace(getenv("SPACE_LIFECYCLE_TLS_CA_FILE"))
	c.CertFile = strings.TrimSpace(getenv("SPACE_LIFECYCLE_CLIENT_CERT_FILE"))
	c.KeyFile = strings.TrimSpace(getenv("SPACE_LIFECYCLE_CLIENT_KEY_FILE"))
	c.RoleCertFile = strings.TrimSpace(getenv("SPACE_ROLE_CLIENT_CERT_FILE"))
	c.RoleKeyFile = strings.TrimSpace(getenv("SPACE_ROLE_CLIENT_KEY_FILE"))
	c.Mode = strings.TrimSpace(getenv("SPACE_LIFECYCLE_MODE"))
	c.DevTombstoneKeyFile = strings.TrimSpace(getenv("SPACE_LIFECYCLE_DEV_TOMBSTONE_KEY_FILE"))
	if c.CAFile == "" || c.CertFile == "" || c.KeyFile == "" || c.RoleCertFile == "" || c.RoleKeyFile == "" {
		return c, false, fmt.Errorf("space lifecycle requires dedicated CA and both client certificate/key pairs")
	}
	if c.Mode != "development" || c.DevTombstoneKeyFile == "" {
		return c, false, fmt.Errorf("space lifecycle requires a purpose-specific HMAC provider; local key files are permitted only in explicit development mode")
	}
	names := []string{"role", "chat", "messaging", "file", "voice", "matchmaking", "search", "subscription", "bot", "notification"}
	for index, name := range names {
		prefix := "SPACE_LIFECYCLE_" + strings.ToUpper(name)
		address := strings.TrimSpace(getenv(prefix + "_GRPC_ADDR"))
		if address == "" || strings.Contains(address, "://") && !strings.HasPrefix(address, "dns:///") {
			return c, false, fmt.Errorf("space lifecycle requires %s_GRPC_ADDR for its protected mTLS endpoint", prefix)
		}
		serverName := strings.TrimSpace(getenv(prefix + "_TLS_SERVER_NAME"))
		if serverName == "" {
			serverName = name
		}
		c.Owners = append(c.Owners, lifecycleOwnerConfig{ID: commonv1.ParticipantId(index + 1), Audience: name, Address: address, ServerName: serverName})
	}
	return c, true, nil
}
