package gisprincipal

import (
	"errors"
	"os"
	"strings"
	chatv1 "voice.app/voice/chat/v1"
)

const ScopeAuthorityMethod = chatv1.ChatScopeAuthorityService_GetSpaceChatScopeAuthority_FullMethodName

// Each caller gets a separate trust/listener/replay configuration. Neither
// scope listener exposes lifecycle mutations or Federation snapshots.
func ScopeAuthorityConfigFromEnv(issuer string) (Config, string, bool, error) {
	if issuer != "role" && issuer != "space" {
		return Config{}, "", false, errors.New("invalid Chat scope authority issuer")
	}
	prefix := "CHAT_SCOPE_" + strings.ToUpper(issuer) + "_"
	get := func(key string) string { return strings.TrimSpace(os.Getenv(prefix + key)) }
	listen := get("GRPC_LISTEN")
	cfg := Config{Issuer: issuer, Audience: Audience, AllowedMethods: []string{ScopeAuthorityMethod},
		TLSCertFile: get("TLS_CERT_FILE"), TLSKeyFile: get("TLS_KEY_FILE"), ClientCAFile: get("CLIENT_CA_FILE"),
		JWKSURL: get("JWKS_URL"), JWKSCAFile: get("JWKS_CA_FILE"), JWKSClientCertFile: get("JWKS_CLIENT_CERT_FILE"), JWKSClientKeyFile: get("JWKS_CLIENT_KEY_FILE"),
		ReplayAddr: get("REPLAY_REDIS_ADDR"), ReplayPassword: os.Getenv(prefix + "REPLAY_REDIS_PASSWORD")}
	configured := false
	for _, key := range []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "JWKS_URL", "JWKS_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD"} {
		if _, set := os.LookupEnv(prefix + key); set {
			configured = true
		}
	}
	if !configured {
		return Config{}, "", false, nil
	}
	if listen == "" || cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" || cfg.ClientCAFile == "" || cfg.JWKSURL == "" || cfg.JWKSCAFile == "" || cfg.ReplayAddr == "" || (cfg.JWKSClientCertFile == "") != (cfg.JWKSClientKeyFile == "") {
		return Config{}, "", true, errors.New("Chat scope authority principal configuration is incomplete")
	}
	return cfg, listen, true, nil
}
