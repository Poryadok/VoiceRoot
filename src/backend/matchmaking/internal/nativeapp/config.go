package nativeapp

import (
	"strings"
	w "voice/backend/pkg/mmadmissionwire"
)

// These are mandatory dependencies of the fixed native executable. No absent
// profile or SDK flag can select a compatibility implementation.
var requiredProfileKeys = []string{
	"MATCHMAKING_REDIS_ADDR", "NATS_URL",
	"MATCHMAKING_SPACE_PRINCIPAL_GRPC_LISTEN", "MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE", "MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE", "MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE",
	"MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR", "S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE",
	"MATCHMAKING_VOICE_PARTY_SNAPSHOT_GRPC_ADDR", "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CA_FILE", "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_CERT_FILE", "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_KEY_FILE", "MATCHMAKING_VOICE_PARTY_SNAPSHOT_SERVER_NAME",
	"MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR", "MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR", "MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE", "MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE", "MATCHMAKING_MATCH_SQUAD_CLIENT_CERT_FILE", "MATCHMAKING_MATCH_SQUAD_CLIENT_KEY_FILE", "MATCHMAKING_MATCH_SQUAD_CHAT_SERVER_NAME", "MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME",
	"MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR", "MATCHMAKING_PRINCIPAL_ACTIVE_KID", "MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR", "MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE", "MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE",
}

type nativeConfig struct {
	descriptor    w.DescriptorV1
	rawDescriptor []byte
	values        map[string]string
}

func resolveConfig(getenv func(string) string, descriptor func() ([]byte, error)) (nativeConfig, error) {
	if getenv == nil || descriptor == nil {
		return nativeConfig{}, errUnavailable
	}
	cfg := nativeConfig{values: make(map[string]string)}
	for _, key := range append([]string{"DATABASE_URL"}, requiredProfileKeys...) {
		value := strings.TrimSpace(getenv(key))
		if value == "" {
			return nativeConfig{}, errUnavailable
		}
		cfg.values[key] = value
	}
	// Optional operational policies are captured once as well. Factories get
	// this immutable resolver rather than rereading environment during recovery.
	for _, key := range []string{"MATCHMAKING_REDIS_PASSWORD", "MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD", "S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN", "LISTEN_ADDR", "MATCHMAKING_GRPC_LISTEN", "NATS_CREDS_FILE", "NATS_CA_FILE", "NATS_CLIENT_CERT_FILE", "NATS_CLIENT_KEY_FILE", "NATS_SERVER_NAME"} {
		cfg.values[key] = getenv(key)
	}
	raw, err := descriptor()
	if err != nil || w.Decode(raw, &cfg.descriptor) != nil || len(cfg.descriptor.AdmittedArgv) != 1 || cfg.descriptor.AdmittedArgv[0] != "/service-native" {
		return nativeConfig{}, errUnavailable
	}
	cfg.rawDescriptor = append([]byte(nil), raw...)
	return cfg, nil
}
