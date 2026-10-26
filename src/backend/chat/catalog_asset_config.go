package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"voice/backend/chat/internal/catalogassets"
)

type catalogOwnerConfig struct{ Address, CA, Name, Cert, Key string }
type catalogAssetConfig struct {
	Listener         catalogassets.ListenerConfig
	Auth, User, File catalogOwnerConfig
	ActorKeysFile    string
}

func catalogRequiredSuffixes() []string {
	names := []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "GATEWAY_JWKS_URL", "MESSAGING_JWKS_URL", "FILE_JWKS_URL", "JWKS_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE", "REPLAY_REDIS_ADDR", "ACTOR_KEYS_FILE"}
	for _, owner := range []string{"AUTH", "USER", "FILE"} {
		for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "TLS_CLIENT_CERT_FILE", "TLS_CLIENT_KEY_FILE"} {
			names = append(names, owner+"_"+suffix)
		}
	}
	return names
}
func loadCatalogAssetConfig(lookup func(string) (string, bool)) (catalogAssetConfig, bool, error) {
	var c catalogAssetConfig
	if lookup == nil {
		return c, false, errors.New("catalog environment reader required")
	}
	values := map[string]string{}
	enabled := false
	for _, suffix := range append(catalogRequiredSuffixes(), "REPLAY_REDIS_PASSWORD") {
		value, present := lookup("CHAT_CATALOG_ASSET_" + suffix)
		enabled = enabled || present
		values[suffix] = value
	}
	if !enabled {
		return c, false, nil
	}
	for _, suffix := range catalogRequiredSuffixes() {
		if values[suffix] == "" || values[suffix] != strings.TrimSpace(values[suffix]) {
			return c, true, errors.New("complete CHAT_CATALOG_ASSET_ protected listener, actor keys and owner transports required")
		}
	}
	c.Listener = catalogassets.ListenerConfig{Listen: values["GRPC_LISTEN"], CertFile: values["TLS_CERT_FILE"], KeyFile: values["TLS_KEY_FILE"], ClientCAFile: values["CLIENT_CA_FILE"], GatewayJWKS: values["GATEWAY_JWKS_URL"], MessagingJWKS: values["MESSAGING_JWKS_URL"], FileJWKS: values["FILE_JWKS_URL"], JWKSCAFile: values["JWKS_CA_FILE"], JWKSClientCertFile: values["JWKS_CLIENT_CERT_FILE"], JWKSClientKeyFile: values["JWKS_CLIENT_KEY_FILE"], RedisAddr: values["REPLAY_REDIS_ADDR"], RedisPassword: values["REPLAY_REDIS_PASSWORD"]}
	c.ActorKeysFile = values["ACTOR_KEYS_FILE"]
	owner := func(name string) catalogOwnerConfig {
		return catalogOwnerConfig{values[name+"_GRPC_ADDR"], values[name+"_TLS_CA_FILE"], values[name+"_TLS_SERVER_NAME"], values[name+"_TLS_CLIENT_CERT_FILE"], values[name+"_TLS_CLIENT_KEY_FILE"]}
	}
	c.Auth, c.User, c.File = owner("AUTH"), owner("USER"), owner("FILE")
	return c, true, c.Listener.Validate()
}

func readCatalogActorKeys(path string) (*catalogassets.ActorKeyring, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("catalog actor key file unavailable")
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return nil, errors.New("invalid catalog actor key file")
	}
	data, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil || len(data) > 65536 {
		return nil, errors.New("invalid catalog actor key file")
	}
	var document struct {
		ActiveKeyID string `json:"active_key_id"`
		Keys        []struct {
			ID     string `json:"id"`
			Secret string `json:"secret_base64"`
		} `json:"keys"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&document) != nil {
		return nil, errors.New("invalid catalog actor key document")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, errors.New("invalid catalog actor key document")
	}
	secrets := map[string][]byte{}
	for _, entry := range document.Keys {
		if _, exists := secrets[entry.ID]; exists {
			return nil, errors.New("duplicate catalog actor key")
		}
		secret, err := base64.StdEncoding.Strict().DecodeString(entry.Secret)
		if err != nil || len(secret) != 32 || base64.StdEncoding.EncodeToString(secret) != entry.Secret {
			return nil, errors.New("invalid catalog actor key material")
		}
		secrets[entry.ID] = secret
	}
	return catalogassets.NewActorKeyring(document.ActiveKeyID, secrets)
}
