package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

func main() {
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, "SFU fixture initialization failed")
		os.Exit(1)
	}
	fmt.Println("SFU fixture initialized; credentials retained in owned private volume")
}

func prepare() error {
	dir := os.Getenv("VOICE_SFU_FIXTURE_DIR")
	if dir == "" {
		return fmt.Errorf("fixture directory required")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	nodeID := uuid.NewString()
	if err = os.WriteFile(filepath.Join(dir, "node-id"), []byte(nodeID), 0644); err != nil {
		return err
	}
	var secretBytes [32]byte
	if _, err = rand.Read(secretBytes[:]); err != nil {
		return err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes[:])
	trust := map[string]any{"issuer": "fixture-master", "environment": "sandbox", "node_id": nodeID, "keys": map[string]string{"fixture-1": base64.RawURLEncoding.EncodeToString(public)}}
	parameters := map[string]string{"node_id": nodeID, "api_key": "fixture-media", "api_secret": secret, "private_key": base64.RawURLEncoding.EncodeToString(private)}
	for name, value := range map[string]any{"trust.json": trust, "parameters.json": parameters} {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if name == "parameters.json" {
			mode = 0600
		}
		if err = os.WriteFile(filepath.Join(dir, name), raw, mode); err != nil {
			return err
		}
	}
	config := fmt.Sprintf("port: 7880\nrtc:\n  tcp_port: 7881\n  udp_port: 7882\n  use_external_ip: false\nkeys:\n  fixture-media: %s\nlogging:\n  level: error\n", secret)
	if err = os.WriteFile(filepath.Join(dir, "livekit.yaml"), []byte(config), 0644); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(dir, "authority"), 0755)
}
