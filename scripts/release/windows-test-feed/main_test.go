package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	installer := filepath.Join(dir, "input.exe")
	// Deliberately synthetic bytes; never claims a runnable Windows installer.
	if err := os.WriteFile(installer, []byte("synthetic installer v1"), 0600); err != nil {
		t.Fatal(err)
	}
	bundle, err := build(installer, filepath.Join(dir, "feed"), "1.2.3", strings.Repeat("a", 40), "http://127.0.0.1:18948", private)
	if err != nil {
		t.Fatal(err)
	}
	return bundle, private, public
}

func TestSignedBundleBindsInstallerFeedSourceAndExternalKey(t *testing.T) {
	bundle, _, public := fixture(t)
	if err := verify(bundle, strings.Repeat("a", 40), public); err != nil {
		t.Fatal(err)
	}
	if err := verify(bundle, strings.Repeat("b", 40), public); err == nil {
		t.Fatal("accepted wrong release source")
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if err := verify(bundle, strings.Repeat("a", 40), other); err == nil {
		t.Fatal("trusted bundle-supplied identity")
	}
}

func TestEveryPublishedMemberRejectsTampering(t *testing.T) {
	for _, name := range []string{"VoiceSetup.exe", "appcast.xml", "manifest.json", "manifest.sig"} {
		t.Run(name, func(t *testing.T) {
			bundle, _, public := fixture(t)
			path := filepath.Join(bundle, name)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			b[len(b)/2] ^= 1
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			if err := verify(bundle, strings.Repeat("a", 40), public); err == nil {
				t.Fatal("accepted modified member")
			}
		})
	}
}

func TestImmutablePublicationNeverOverwritesAndLeavesSourceAlone(t *testing.T) {
	bundle, private, _ := fixture(t)
	before, _ := os.ReadFile(filepath.Join(bundle, "VoiceSetup.exe"))
	_, err := build(filepath.Join(bundle, "VoiceSetup.exe"), filepath.Dir(bundle), "1.2.4", strings.Repeat("a", 40), "http://127.0.0.1:18948", private)
	if err == nil {
		t.Fatal("overwrote immutable namespace")
	}
	after, _ := os.ReadFile(filepath.Join(bundle, "VoiceSetup.exe"))
	if string(before) != string(after) {
		t.Fatal("changed existing installer")
	}
}

func TestInvalidInputsPublishNothing(t *testing.T) {
	_, private, _ := fixture(t)
	for _, base := range []string{"http://example.com", "https://prod.example.com", "http://127.0.0.1.evil", "http://user@127.0.0.1", "http://127.0.0.1/?secret=x", "file:///tmp"} {
		dir := t.TempDir()
		installer := filepath.Join(dir, "in.exe")
		_ = os.WriteFile(installer, []byte("fixture"), 0600)
		out := filepath.Join(dir, "out")
		if _, err := build(installer, out, "1.2.3", strings.Repeat("a", 40), base, private); err == nil {
			t.Errorf("accepted base %s", base)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("invalid input published output")
		}
	}
}

func TestFeedUsesEdDSAInstallerBytesAndWindowsArchitecture(t *testing.T) {
	bundle, _, public := fixture(t)
	raw, err := os.ReadFile(filepath.Join(bundle, "appcast.xml"))
	if err != nil {
		t.Fatal(err)
	}
	feed := string(raw)
	for _, expected := range []string{"sparkle:edSignature=", "sparkle:os=\"windows-x64\"", "sparkle:version=\"1.2.3\"", "http://127.0.0.1:18948/"} {
		if !strings.Contains(feed, expected) {
			t.Errorf("missing %s", expected)
		}
	}
	if strings.Contains(feed, "dsaSignature") {
		t.Fatal("legacy DSA signature")
	}
	if err := verify(bundle, strings.Repeat("a", 40), public); err != nil {
		t.Fatal(err)
	}
}

func TestSignedMalformedManifestAndMetadataSubstitutionReject(t *testing.T) {
	for _, mutate := range []func(*manifest){
		func(m *manifest) { m.Signing = "production" },
		func(m *manifest) { m.Schema = 2 },
		func(m *manifest) { m.Platform = "windows-arm64" },
		func(m *manifest) { m.URL = "http://example.com/" + m.SHA + "/VoiceSetup.exe" },
		func(m *manifest) { m.Length++ },
		func(m *manifest) { m.Signature = encoded(make([]byte, 64)) },
		func(m *manifest) { m.Version = "../escape" },
	} {
		bundle, private, public := fixture(t)
		raw, _ := os.ReadFile(filepath.Join(bundle, "manifest.json"))
		var m manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		mutate(&m)
		raw = canonical(m)
		_ = os.WriteFile(filepath.Join(bundle, "manifest.json"), raw, 0600)
		_ = os.WriteFile(filepath.Join(bundle, "manifest.sig"), []byte(encoded(ed25519.Sign(private, raw))), 0600)
		if err := verify(bundle, strings.Repeat("a", 40), public); err == nil {
			t.Fatal("accepted invalid signed metadata")
		}
	}
}

func TestCLIKeygenBuildVerifyAndExclusiveKey(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "test.seed")
	var output bytes.Buffer
	if err := run([]string{"keygen", "--key", key}, &output); err != nil {
		t.Fatal(err)
	}
	public := strings.TrimSpace(output.String())
	output.Reset()
	if err := run([]string{"keygen", "--key", key}, &output); err == nil {
		t.Fatal("overwrote test identity")
	}
	installer := filepath.Join(dir, "synthetic.exe")
	_ = os.WriteFile(installer, []byte("fixture"), 0600)
	if err := run([]string{"build", "--key", key, "--installer", installer, "--out", filepath.Join(dir, "feed"), "--version", "1.2.3", "--source-sha", strings.Repeat("a", 40), "--base-url", "http://127.0.0.1:18948"}, &output); err != nil {
		t.Fatal(err)
	}
	bundle := strings.TrimSpace(output.String())
	output.Reset()
	if err := run([]string{"verify", "--bundle", bundle, "--public-key", public, "--source-sha", strings.Repeat("a", 40)}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "offline test package integrity only") {
		t.Fatal("missing qualification boundary")
	}
}
