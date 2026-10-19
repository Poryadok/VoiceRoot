package releasepolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

const testSource = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func alphaBytes(t *testing.T) []byte {
	t.Helper()
	raw := map[string]any{
		"schema_version": 2, "profile": "free-invite-alpha", "source_sha": testSource,
		"capabilities": map[string]string{"phone_auth": "disabled", "paid": "disabled", "mobile_push": "disabled", "public_discovery": "disabled", "unfinished_rich_types": "disabled", "federation": "disabled"},
		"rich_intents": map[string]string{"text": "qualified", "photo": "qualified", "video": "qualified", "document": "qualified", "music": "qualified", "voice": "disabled", "sticker": "disabled", "gif": "disabled", "article": "disabled", "location": "disabled", "video_note": "disabled", "send_silent": "disabled", "schedule": "disabled", "when_online": "disabled"},
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func checksum(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func TestAlphaBindingAndImmutableProjection(t *testing.T) {
	data := alphaBytes(t)
	p, err := Parse(data, checksum(data), testSource)
	if err != nil {
		t.Fatal(err)
	}
	if p.Allows("mobile_push") || p.Allows("unknown") || p.Profile() != "free-invite-alpha" {
		t.Fatal("alpha authority did not deny restricted action")
	}
	states := p.CapabilityStates()
	states["mobile_push"] = "enabled"
	if p.Allows("mobile_push") {
		t.Fatal("projection mutated serving authority")
	}
	if !p.RichIntentQualified("music") || !p.RichIntentQualified("document") || p.RichIntentQualified("sticker") || p.RichIntentQualified("future") {
		t.Fatal("finite basic/Music/rich table changed")
	}
}

func TestAlphaRejectsWrongBindingAndMalformedPolicy(t *testing.T) {
	valid := alphaBytes(t)
	for _, data := range [][]byte{
		bytes.Replace(valid, []byte(`"schema_version":2`), []byte(`"schema_version":1`), 1),
		bytes.Replace(valid, []byte(`"paid":"disabled"`), []byte(`"paid":"enabled"`), 1),
		bytes.Replace(valid, []byte(`"mobile_push":"disabled",`), nil, 1),
		bytes.Replace(valid, []byte(`"text":"qualified"`), []byte(`"text":"disabled"`), 1),
		bytes.Replace(valid, []byte(`"source_sha":`), []byte(`"source_sha":"`+testSource+`","source_sha":`), 1),
		append(append([]byte{}, valid...), []byte(` {}`)...),
		[]byte(`null`), []byte(`{"schema_version":2,"unknown":"secret"}`),
	} {
		if _, err := Parse(data, checksum(data), testSource); err == nil {
			t.Errorf("malformed policy accepted")
		}
	}
	for _, binding := range [][2]string{{"", testSource}, {strings.Repeat("b", 64), testSource}, {checksum(valid), ""}, {checksum(valid), strings.Repeat("b", 40)}} {
		if _, err := Parse(valid, binding[0], binding[1]); err == nil {
			t.Fatal("missing/wrong external binding accepted")
		}
	}
}

func TestDevelopmentIsExplicitAndCannotQualify(t *testing.T) {
	look := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) { v, ok := values[key]; return v, ok }
	}
	if _, err := Load(look(nil), "", nil); err == nil {
		t.Fatal("missing mode silently became development")
	}
	p, err := Load(look(map[string]string{"VOICE_RELEASE_PROFILE": "development"}), "", nil)
	if err != nil || p.Profile() != "development" || !p.Allows("mobile_push") || p.Allows("unknown") {
		t.Fatalf("explicit development: %v", err)
	}
	if _, err := Load(look(map[string]string{"VOICE_RELEASE_PROFILE": "development", "VOICE_RELEASE_POLICY_FILE": "shadow"}), "", nil); err == nil {
		t.Fatal("development shadows a mounted release artifact")
	}
}

func TestLoadUsesExactArtifactAndBuildIdentity(t *testing.T) {
	data := alphaBytes(t)
	values := map[string]string{"VOICE_RELEASE_PROFILE": "free-invite-alpha", "VOICE_RELEASE_POLICY_FILE": "/private/policy.json", "VOICE_RELEASE_POLICY_SHA256": checksum(data)}
	readCalls := 0
	read := func(path string) ([]byte, error) {
		readCalls++
		if path != "/private/policy.json" {
			t.Fatal("wrong policy path")
		}
		return data, nil
	}
	p, err := Load(func(key string) (string, bool) { v, ok := values[key]; return v, ok }, testSource, read)
	if err != nil || p.Allows("mobile_push") || readCalls != 1 {
		t.Fatalf("actual bootstrap parse: %v", err)
	}
}
