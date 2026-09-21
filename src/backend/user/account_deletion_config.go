package main

import (
	"fmt"
	"strings"
)

type accountDeletionConsumerConfig struct {
	Enabled         bool
	CredentialsFile string
	NATSURL         string
}

// accountDeletionConsumerConfigFromEnv makes activation opt-in. The dedicated
// credential is intentionally mandatory so User never consumes Auth authority
// through the ordinary anonymous/shared NATS connection.
func accountDeletionConsumerConfigFromEnv(getenv func(string) string) (accountDeletionConsumerConfig, error) {
	enabled := strings.TrimSpace(getenv("USER_ACCOUNT_DELETE_CONSUMER_ENABLED")) == "true"
	if !enabled {
		return accountDeletionConsumerConfig{}, nil
	}
	credentialsFile := strings.TrimSpace(getenv("USER_ACCOUNT_DELETE_NATS_CREDS_FILE"))
	if credentialsFile == "" {
		return accountDeletionConsumerConfig{}, fmt.Errorf("USER_ACCOUNT_DELETE_NATS_CREDS_FILE is required when USER_ACCOUNT_DELETE_CONSUMER_ENABLED=true")
	}
	natsURL := strings.TrimSpace(getenv("NATS_URL"))
	if natsURL == "" {
		return accountDeletionConsumerConfig{}, fmt.Errorf("NATS_URL is required when USER_ACCOUNT_DELETE_CONSUMER_ENABLED=true")
	}
	return accountDeletionConsumerConfig{Enabled: true, CredentialsFile: credentialsFile, NATSURL: natsURL}, nil
}
