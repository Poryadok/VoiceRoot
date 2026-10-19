package nativeapp

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"strings"
	"voice/backend/matchmaking/internal/rosterconsume"
	w "voice/backend/pkg/mmadmissionwire"
)

type configurationContracts struct{ Config, Credentials, Upstream, Broker w.Hash }

// These are mismatch detectors over actual resolved inputs, not admission or
// readiness. Secret bytes and input hashes never enter public responses/logs.
func observeConfigurationContracts(cfg nativeConfig) (configurationContracts, error) {
	settings := make(map[string]string)
	credentials := make(map[string]w.Hash)
	upstream := make(map[string]string)
	broker := make(map[string]string)
	for key, value := range cfg.values {
		if key == "DATABASE_URL" || strings.Contains(key, "PASSWORD") {
			credentials[key] = w.Hash(sha256.Sum256([]byte(value)))
			continue
		}
		settings[key] = value
		if strings.HasSuffix(key, "_FILE") {
			hash, err := hashContractFile(value)
			if err != nil {
				return configurationContracts{}, errUnavailable
			}
			credentials[key] = hash
		}
		if strings.HasPrefix(key, "MATCHMAKING_VOICE_PARTY_SNAPSHOT_") || strings.HasPrefix(key, "MATCHMAKING_MATCH_SQUAD_") || strings.HasPrefix(key, "S2S_") {
			upstream[key] = value
		}
		if strings.HasPrefix(key, "NATS_") {
			broker[key] = value
		}
	}
	// The strict issuer factory separately validates the projected-secret layout,
	// exactly two distinct RSA keys and the active key. No recursive traversal.
	entries, err := os.ReadDir(cfg.values["MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR"])
	if err != nil {
		return configurationContracts{}, errUnavailable
	}
	keys := 0
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".pem") {
			continue
		}
		hash, err := hashContractFile(filepath.Join(cfg.values["MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR"], entry.Name()))
		if err != nil {
			return configurationContracts{}, errUnavailable
		}
		credentials["issuer:"+entry.Name()] = hash
		keys++
	}
	if keys != 2 {
		return configurationContracts{}, errUnavailable
	}
	broker["stream"] = "matchmaking_events;FILE;LIMITS;168h;mm.match_completed,mm.match_found,mm.player_banned,mm.rating_submitted,mm.search_cancelled,mm.search_nudge,mm.search_started,mm.search_timeout"
	broker["roster"] = rosterconsume.Stream + ";" + rosterconsume.Subject + ";" + rosterconsume.Durable + ";preprovisioned-pull;commit-before-ack"
	return configurationContracts{
		Config:      codeContractHash("voice.mm.native.resolved-config.v1", settings),
		Credentials: codeContractHash("voice.mm.native.credential-inputs.v1", credentials),
		Upstream:    codeContractHash("voice.mm.native.upstream-profile.v1", upstream),
		Broker:      codeContractHash("voice.mm.native.broker-profile.v1", broker),
	}, nil
}

func hashContractFile(path string) (w.Hash, error) {
	if path == "" {
		return w.Hash{}, errUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return w.Hash{}, errUnavailable
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 1<<20 {
		return w.Hash{}, errUnavailable
	}
	hash := sha256.New()
	if count, err := io.Copy(hash, io.LimitReader(file, (1<<20)+1)); err != nil || count > 1<<20 {
		return w.Hash{}, errUnavailable
	}
	var result w.Hash
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func checkConfigurationContracts(cfg nativeConfig) (w.Hash, error) {
	observed, err := observeConfigurationContracts(cfg)
	d := cfg.descriptor
	if err != nil || observed.Config != d.ConfigContractHash || observed.Credentials != d.CredentialRefsManifestHash || observed.Upstream != d.UpstreamContractManifestHash || observed.Broker != d.BrokerContractManifestHash {
		return w.Hash{}, errUnavailable
	}
	return codeContractHash("voice.mm.native.prepared-contracts.v1", struct {
		Configuration     configurationContracts
		Factories, Routes w.Hash
	}{observed, d.Binding.SourceFactoryManifestHash, d.Binding.FreshRouteManifestHash}), nil
}
