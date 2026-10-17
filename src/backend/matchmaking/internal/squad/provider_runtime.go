package squad

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

var principalKIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var projectedSecretTimestampPattern = regexp.MustCompile(`^\.\.\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}\.\d{9}$`)

type providerRuntimeConfig struct {
	ChatAddr, VoiceAddr                    string
	ChatCA, VoiceCA, ClientCert, ClientKey string
	ChatServerName, VoiceServerName        string
	SigningKeysDir, ActiveKID              string
}

// LoadProtectedProviderWorker constructs only the dedicated MatchSquad mTLS
// clients. A partial configuration is an error; absent configuration disables
// the worker so callers can fail closed without falling back to public RPCs.
func LoadProtectedProviderWorker(matchStore *store.MatchStore, getenv func(string) string) (*MatchSquadProviderWorker, func() error, bool, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	values := map[string]string{
		"MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR":    getenv("MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR"),
		"MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR":   getenv("MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR"),
		"MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE":      getenv("MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE"),
		"MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE":     getenv("MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE"),
		"MATCHMAKING_MATCH_SQUAD_CLIENT_CERT_FILE":  getenv("MATCHMAKING_MATCH_SQUAD_CLIENT_CERT_FILE"),
		"MATCHMAKING_MATCH_SQUAD_CLIENT_KEY_FILE":   getenv("MATCHMAKING_MATCH_SQUAD_CLIENT_KEY_FILE"),
		"MATCHMAKING_MATCH_SQUAD_CHAT_SERVER_NAME":  getenv("MATCHMAKING_MATCH_SQUAD_CHAT_SERVER_NAME"),
		"MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME": getenv("MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME"),
		"MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR":    getenv("MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR"),
		"MATCHMAKING_PRINCIPAL_ACTIVE_KID":          getenv("MATCHMAKING_PRINCIPAL_ACTIVE_KID"),
		"MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR":    getenv("MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR"),
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE":  getenv("MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE"),
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE":   getenv("MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE"),
	}
	configured := false
	for name, value := range values {
		value = strings.TrimSpace(value)
		values[name] = value
		if value != "" {
			configured = true
		}
	}
	if !configured {
		return nil, func() error { return nil }, false, nil
	}
	for name, value := range values {
		if value == "" {
			return nil, nil, true, fmt.Errorf("required MatchSquad provider configuration %s is missing", name)
		}
	}
	config := providerRuntimeConfig{
		ChatAddr: values["MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR"], VoiceAddr: values["MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR"],
		ChatCA: values["MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE"], VoiceCA: values["MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE"],
		ClientCert: values["MATCHMAKING_MATCH_SQUAD_CLIENT_CERT_FILE"], ClientKey: values["MATCHMAKING_MATCH_SQUAD_CLIENT_KEY_FILE"],
		ChatServerName: values["MATCHMAKING_MATCH_SQUAD_CHAT_SERVER_NAME"], VoiceServerName: values["MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME"],
		SigningKeysDir: values["MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR"], ActiveKID: values["MATCHMAKING_PRINCIPAL_ACTIVE_KID"],
	}
	issuer, err := loadMatchmakingIssuer(config.SigningKeysDir, config.ActiveKID)
	if err != nil {
		return nil, nil, true, err
	}
	certificate, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
	if err != nil {
		return nil, nil, true, errors.New("matchsquad provider client TLS identity is invalid")
	}
	chatCredentials, err := providerTLSCredentials(config.ChatCA, config.ChatServerName, certificate)
	if err != nil {
		return nil, nil, true, fmt.Errorf("chat MatchSquad mTLS configuration: %w", err)
	}
	voiceCredentials, err := providerTLSCredentials(config.VoiceCA, config.VoiceServerName, certificate)
	if err != nil {
		return nil, nil, true, fmt.Errorf("voice MatchSquad mTLS configuration: %w", err)
	}
	chatConn, err := grpc.NewClient(grpcclient.DialTarget(config.ChatAddr), grpc.WithTransportCredentials(chatCredentials))
	if err != nil {
		return nil, nil, true, errors.New("chat MatchSquad mTLS client could not be constructed")
	}
	voiceConn, err := grpc.NewClient(grpcclient.DialTarget(config.VoiceAddr), grpc.WithTransportCredentials(voiceCredentials))
	if err != nil {
		_ = chatConn.Close()
		return nil, nil, true, errors.New("voice MatchSquad mTLS client could not be constructed")
	}
	worker := &MatchSquadProviderWorker{
		Store: matchStore, Issuer: issuer,
		Chat: chatv1.NewMatchSquadChatServiceClient(chatConn), Voice: callsv1.NewMatchSquadVoiceServiceClient(voiceConn),
	}
	closeConnections := func() error {
		chatErr := chatConn.Close()
		voiceErr := voiceConn.Close()
		return errors.Join(chatErr, voiceErr)
	}
	return worker, closeConnections, true, nil
}

// PrincipalJWKSListener is the HTTPS publisher for Matchmaking's own signed
// principal keys. Its config is opt-in as one complete profile; the handler
// contains public keys only and is intended for workload-TLS-protected access.
type PrincipalJWKSListener struct {
	ListenAddr string
	TLSConfig  *tls.Config
	Handler    http.Handler
	Enabled    bool
}

// LoadPrincipalJWKSListener validates the complete signing and TLS profile
// before exposing the active and next public keys through the internal route.
func LoadPrincipalJWKSListener(getenv func(string) string) (PrincipalJWKSListener, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	names := []string{
		"MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR",
		"MATCHMAKING_PRINCIPAL_ACTIVE_KID",
		"MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR",
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE",
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE",
	}
	values := make(map[string]string, len(names))
	configured := false
	for _, name := range names {
		value := strings.TrimSpace(getenv(name))
		values[name] = value
		configured = configured || value != ""
	}
	if !configured {
		return PrincipalJWKSListener{}, nil
	}
	for _, name := range names {
		if values[name] == "" {
			return PrincipalJWKSListener{}, fmt.Errorf("partial Matchmaking principal JWKS configuration: %s is missing", name)
		}
	}
	if _, _, err := net.SplitHostPort(values["MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR"]); err != nil {
		return PrincipalJWKSListener{}, errors.New("Matchmaking principal JWKS listen address is invalid")
	}
	keys, err := loadMatchmakingPublicKeys(values["MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR"], values["MATCHMAKING_PRINCIPAL_ACTIVE_KID"])
	if err != nil {
		return PrincipalJWKSListener{}, err
	}
	certificate, err := tls.LoadX509KeyPair(values["MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE"], values["MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE"])
	if err != nil {
		return PrincipalJWKSListener{}, errors.New("Matchmaking principal JWKS TLS identity is invalid")
	}
	mux := http.NewServeMux()
	mux.Handle("/internal/v1/principal/jwks.json", principal.JWKSHandlerKeys(keys))
	return PrincipalJWKSListener{
		ListenAddr: values["MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR"],
		TLSConfig:  &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}},
		Handler:    mux,
		Enabled:    true,
	}, nil
}

func providerTLSCredentials(caFile, serverName string, certificate tls.Certificate) (credentials.TransportCredentials, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, errors.New("CA certificate unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("CA file contains no certificates")
	}
	return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName, Certificates: []tls.Certificate{certificate}}), nil
}

func loadMatchmakingIssuer(directory, activeKID string) (*principal.Issuer, error) {
	keys, err := loadMatchmakingSigningKeys(directory, activeKID)
	if err != nil {
		return nil, err
	}
	return principal.NewIssuer(principal.IssuerConfig{Issuer: "matchmaking", KeyID: activeKID, PrivateKey: keys[activeKID]})
}

func loadMatchmakingPublicKeys(directory, activeKID string) ([]principal.JWKSKey, error) {
	keys, err := loadMatchmakingSigningKeys(directory, activeKID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	for kid := range keys {
		ids = append(ids, kid)
	}
	sort.Strings(ids)
	publicKeys := make([]principal.JWKSKey, 0, len(ids))
	for _, kid := range ids {
		publicKeys = append(publicKeys, principal.JWKSKey{KeyID: kid, PublicKey: &keys[kid].PublicKey})
	}
	return publicKeys, nil
}

func loadMatchmakingSigningKeys(directory, activeKID string) (map[string]*rsa.PrivateKey, error) {
	directory, activeKID = strings.TrimSpace(directory), strings.TrimSpace(activeKID)
	if directory == "" || !principalKIDPattern.MatchString(activeKID) {
		return nil, errors.New("matchmaking principal signing configuration is invalid")
	}
	canonicalDir, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, errors.New("matchmaking principal signing directory is unavailable")
	}
	info, err := os.Stat(canonicalDir)
	if err != nil || !info.IsDir() {
		return nil, errors.New("matchmaking principal signing path is not a directory")
	}
	entries, err := os.ReadDir(canonicalDir)
	if err != nil {
		return nil, errors.New("matchmaking principal signing directory is unreadable")
	}
	keys := make(map[string]*rsa.PrivateKey, 2)
	var projectedDataTarget string
	var projectedVersionDirectories []string
	for _, entry := range entries {
		if entry.Name() == "..data" {
			if entry.Type()&os.ModeSymlink == 0 {
				return nil, errors.New("matchmaking principal projected data link is invalid")
			}
			target, err := filepath.EvalSymlinks(filepath.Join(canonicalDir, entry.Name()))
			if err != nil {
				return nil, errors.New("matchmaking principal projected data link is unavailable")
			}
			rel, err := filepath.Rel(canonicalDir, target)
			if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, errors.New("matchmaking principal projected data link resolves outside its directory")
			}
			if !projectedSecretTimestampPattern.MatchString(filepath.Base(target)) {
				return nil, errors.New("matchmaking principal projected data link target is invalid")
			}
			targetInfo, err := os.Stat(target)
			if err != nil || !targetInfo.IsDir() {
				return nil, errors.New("matchmaking principal projected data link target is invalid")
			}
			projectedDataTarget = filepath.Base(target)
			continue
		}
		if projectedSecretTimestampPattern.MatchString(entry.Name()) {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return nil, errors.New("matchmaking principal projected version directory is invalid")
			}
			projectedVersionDirectories = append(projectedVersionDirectories, entry.Name())
			continue
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pem") {
			return nil, errors.New("matchmaking principal key directory must contain only PEM rotation keys")
		}
		kid := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if !principalKIDPattern.MatchString(kid) {
			return nil, errors.New("matchmaking principal key ID is invalid")
		}
		path, err := filepath.EvalSymlinks(filepath.Join(canonicalDir, entry.Name()))
		if err != nil {
			return nil, errors.New("matchmaking principal key file is unavailable")
		}
		rel, err := filepath.Rel(canonicalDir, path)
		if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, errors.New("matchmaking principal key resolves outside its directory")
		}
		fileInfo, err := os.Stat(path)
		if err != nil || !fileInfo.Mode().IsRegular() {
			return nil, errors.New("matchmaking principal key is not a regular file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, errors.New("matchmaking principal key is unreadable")
		}
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
			return nil, errors.New("matchmaking principal key must be one PKCS#8 PEM block")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		key, ok := parsed.(*rsa.PrivateKey)
		if err != nil || !ok || key.Validate() != nil || key.N.BitLen() < 2048 {
			return nil, errors.New("matchmaking principal key must be valid RSA of at least 2048 bits")
		}
		keys[kid] = key
	}
	if projectedDataTarget != "" && (len(projectedVersionDirectories) != 1 || projectedVersionDirectories[0] != projectedDataTarget) {
		return nil, errors.New("matchmaking principal projected version directory does not match its data link")
	}
	if projectedDataTarget == "" && len(projectedVersionDirectories) != 0 {
		return nil, errors.New("matchmaking principal projected version directory has no data link")
	}
	if len(keys) != 2 || keys[activeKID] == nil {
		return nil, errors.New("matchmaking principal key directory must contain current and next keys")
	}
	active := keys[activeKID]
	for kid, key := range keys {
		if kid != activeKID && key.N.Cmp(active.N) == 0 {
			return nil, errors.New("matchmaking principal rotation keys must have distinct public keys")
		}
	}
	return keys, nil
}
