package main

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type config struct {
	ListenAddr       string
	DatabaseURL      string
	RedisAddr        string
	JWKSURL          string
	JWTIssuer        string
	JWTAudience      string
	OperatorAccounts map[uuid.UUID]struct{}
	CredentialKey    []byte
	AuthWorkloadKey  []byte
}

func loadConfig(getenv func(string) string) (config, error) {
	if getenv == nil {
		return config{}, fmt.Errorf("environment reader not configured")
	}
	c := config{
		ListenAddr:  strings.TrimSpace(getenv("LISTEN_ADDR")),
		DatabaseURL: strings.TrimSpace(getenv("DATABASE_URL")),
		RedisAddr:   strings.TrimSpace(getenv("GAME_INTEGRATION_REDIS_ADDR")),
		JWKSURL:     strings.TrimSpace(getenv("GAME_INTEGRATION_JWKS_URL")),
		JWTIssuer:   strings.TrimSpace(getenv("GAME_INTEGRATION_JWT_ISSUER")),
		JWTAudience: strings.TrimSpace(getenv("GAME_INTEGRATION_JWT_AUDIENCE")),
	}
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	for _, required := range []struct {
		name  string
		value string
	}{
		{"DATABASE_URL", c.DatabaseURL},
		{"GAME_INTEGRATION_REDIS_ADDR", c.RedisAddr},
		{"GAME_INTEGRATION_JWKS_URL", c.JWKSURL},
		{"GAME_INTEGRATION_JWT_ISSUER", c.JWTIssuer},
		{"GAME_INTEGRATION_JWT_AUDIENCE", c.JWTAudience},
	} {
		if required.value == "" {
			return config{}, fmt.Errorf("%s is required", required.name)
		}
	}
	c.OperatorAccounts = make(map[uuid.UUID]struct{})
	for _, raw := range strings.Split(getenv("GAME_INTEGRATION_OPERATOR_ACCOUNT_IDS"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return config{}, fmt.Errorf("invalid GAME_INTEGRATION_OPERATOR_ACCOUNT_IDS")
		}
		c.OperatorAccounts[id] = struct{}{}
	}
	if raw := strings.TrimSpace(getenv("GAME_INTEGRATION_CREDENTIAL_KEY_B64")); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return config{}, fmt.Errorf("invalid GAME_INTEGRATION_CREDENTIAL_KEY_B64")
		}
		c.CredentialKey = key
	}
	if raw := strings.TrimSpace(getenv("GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64")); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return config{}, fmt.Errorf("invalid GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64")
		}
		c.AuthWorkloadKey = key
	}
	return c, nil
}
