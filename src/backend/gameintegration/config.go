package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/authbinding"
	"voice/backend/gameintegration/internal/botproof"
)

type config struct {
	ListenAddr                   string
	DatabaseURL                  string
	RedisAddr                    string
	JWKSURL                      string
	JWTIssuer                    string
	JWTAudience                  string
	OperatorAccounts             map[uuid.UUID]struct{}
	CredentialKey                []byte
	AuthWorkloadKey              []byte
	BotAuthorityURL              string
	BotWorkloadKey               []byte
	AuthBindingClient            *authbinding.Client
	MessagingListenAddr          string
	MessagingTLSCertFile         string
	MessagingTLSKeyFile          string
	MessagingClientCAFile        string
	MessagingWorkloadKey         []byte
	MessagingPreviousWorkloadKey []byte
	MessagingPreviousKeyUntil    time.Time
}

func loadConfig(getenv func(string) string) (config, error) {
	if getenv == nil {
		return config{}, fmt.Errorf("environment reader not configured")
	}
	c := config{
		ListenAddr:      strings.TrimSpace(getenv("LISTEN_ADDR")),
		DatabaseURL:     strings.TrimSpace(getenv("DATABASE_URL")),
		RedisAddr:       strings.TrimSpace(getenv("GAME_INTEGRATION_REDIS_ADDR")),
		JWKSURL:         strings.TrimSpace(getenv("GAME_INTEGRATION_JWKS_URL")),
		JWTIssuer:       strings.TrimSpace(getenv("GAME_INTEGRATION_JWT_ISSUER")),
		JWTAudience:     strings.TrimSpace(getenv("GAME_INTEGRATION_JWT_AUDIENCE")),
		BotAuthorityURL: strings.TrimSpace(getenv("BOT_INTERNAL_URL")),
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
	key, err := botproof.DecodeWorkloadKey(getenv("GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64"))
	if err != nil {
		return config{}, err
	}
	c.BotWorkloadKey = key
	if (c.BotAuthorityURL == "") != (len(c.BotWorkloadKey) == 0) {
		return config{}, fmt.Errorf("BOT_INTERNAL_URL and GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64 must be configured together")
	}
	authBindingValues := []struct{ name, value string }{
		{"GIS_AUTH_GAME_BINDING_BASE_URL", strings.TrimSpace(getenv("GIS_AUTH_GAME_BINDING_BASE_URL"))},
		{"GIS_AUTH_GAME_BINDING_CA_FILE", strings.TrimSpace(getenv("GIS_AUTH_GAME_BINDING_CA_FILE"))},
		{"GIS_AUTH_GAME_BINDING_CLIENT_CERT_FILE", strings.TrimSpace(getenv("GIS_AUTH_GAME_BINDING_CLIENT_CERT_FILE"))},
		{"GIS_AUTH_GAME_BINDING_CLIENT_KEY_FILE", strings.TrimSpace(getenv("GIS_AUTH_GAME_BINDING_CLIENT_KEY_FILE"))},
	}
	configured := 0
	for _, field := range authBindingValues {
		if field.value != "" {
			configured++
		}
	}
	if configured != 0 && configured != len(authBindingValues) {
		return config{}, fmt.Errorf("GIS_AUTH_GAME_BINDING_BASE_URL, CA, client cert and client key must be configured together")
	}
	if configured == len(authBindingValues) {
		client, err := authbinding.NewClient(authBindingValues[0].value, authBindingValues[1].value,
			authBindingValues[2].value, authBindingValues[3].value)
		if err != nil {
			return config{}, fmt.Errorf("invalid GIS Auth game-binding client configuration")
		}
		c.AuthBindingClient = client
	}
	messagingValues := []struct{ name, value string }{
		{"GAME_INTEGRATION_MESSAGING_LISTEN_ADDR", strings.TrimSpace(getenv("GAME_INTEGRATION_MESSAGING_LISTEN_ADDR"))},
		{"GAME_INTEGRATION_MESSAGING_TLS_CERT_FILE", strings.TrimSpace(getenv("GAME_INTEGRATION_MESSAGING_TLS_CERT_FILE"))},
		{"GAME_INTEGRATION_MESSAGING_TLS_KEY_FILE", strings.TrimSpace(getenv("GAME_INTEGRATION_MESSAGING_TLS_KEY_FILE"))},
		{"GAME_INTEGRATION_MESSAGING_CLIENT_CA_FILE", strings.TrimSpace(getenv("GAME_INTEGRATION_MESSAGING_CLIENT_CA_FILE"))},
		{"GAME_INTEGRATION_MESSAGING_WORKLOAD_KEY_B64", strings.TrimSpace(getenv("GAME_INTEGRATION_MESSAGING_WORKLOAD_KEY_B64"))},
	}
	messagingConfigured := 0
	for _, value := range messagingValues {
		if value.value != "" {
			messagingConfigured++
		}
	}
	previousRaw := strings.TrimSpace(getenv("GAME_INTEGRATION_MESSAGING_PREVIOUS_WORKLOAD_KEY_B64"))
	previousUntilRaw := strings.TrimSpace(getenv("GAME_INTEGRATION_MESSAGING_PREVIOUS_KEY_UNTIL"))
	if (previousRaw == "") != (previousUntilRaw == "") {
		return config{}, fmt.Errorf("messaging mTLS previous key and deadline must be configured together")
	}
	if messagingConfigured != 0 && messagingConfigured != len(messagingValues) {
		return config{}, fmt.Errorf("messaging mTLS listener, certificate, CA and workload key must be configured together")
	}
	if messagingConfigured == len(messagingValues) {
		c.MessagingListenAddr = messagingValues[0].value
		c.MessagingTLSCertFile, c.MessagingTLSKeyFile, c.MessagingClientCAFile = messagingValues[1].value, messagingValues[2].value, messagingValues[3].value
		key, err := decodeMessagingWorkloadKey(messagingValues[4].value)
		if err != nil {
			return config{}, err
		}
		c.MessagingWorkloadKey = key
		if previousRaw != "" {
			previous, err := decodeMessagingWorkloadKey(previousRaw)
			if err != nil {
				return config{}, fmt.Errorf("invalid GAME_INTEGRATION_MESSAGING_PREVIOUS_WORKLOAD_KEY_B64")
			}
			if string(previous) == string(key) {
				return config{}, fmt.Errorf("messaging workload keys must be distinct")
			}
			until, err := time.Parse(time.RFC3339, previousUntilRaw)
			if err != nil || until.Before(time.Now()) || until.After(time.Now().Add(5*time.Minute)) {
				return config{}, fmt.Errorf("messaging previous key deadline must be within the five-minute overlap")
			}
			c.MessagingPreviousWorkloadKey, c.MessagingPreviousKeyUntil = previous, until
		}
	} else if previousRaw != "" {
		return config{}, fmt.Errorf("messaging previous key requires a configured Messaging mTLS listener")
	}
	return c, nil
}

func decodeMessagingWorkloadKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("invalid GAME_INTEGRATION_MESSAGING_WORKLOAD_KEY_B64")
	}
	return key, nil
}
