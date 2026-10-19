// Package releasepolicy binds immutable owner capability policy to a build.
package releasepolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
)

const (
	Alpha       = "free-invite-alpha"
	Development = "development"
	MobilePush  = "mobile_push"
)

var capabilityIDs = []string{"phone_auth", "paid", MobilePush, "public_discovery", "unfinished_rich_types", "federation"}
var baselineIntents = []string{"text", "photo", "video", "document", "music"}
var restrictedIntents = []string{"voice", "sticker", "gif", "article", "location", "video_note", "send_silent", "schedule", "when_online"}

// Policy has no public mutable fields. Projections are copies, never authority.
type Policy struct {
	profile, source, digest string
	capabilities            map[string]string
	rich                    map[string]string
}

var errPolicy = errors.New("invalid or unbound release policy")

// Parse validates exact artifact bytes against independently supplied build and
// release bindings. It does not attest images, replicas or qualification results.
func Parse(data []byte, expectedDigest, buildSource string) (*Policy, error) {
	if len(data) == 0 || len(data) > 64*1024 || !lowerHex(expectedDigest, 64) || !lowerHex(buildSource, 40) {
		return nil, errPolicy
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expectedDigest {
		return nil, errPolicy
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := strictValue(decoder, 0)
	if err != nil {
		return nil, errPolicy
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, errPolicy
	}
	root, ok := value.(map[string]any)
	if !ok || !keys(root, []string{"schema_version", "profile", "source_sha", "capabilities", "rich_intents"}) || root["schema_version"] != json.Number("2") || root["profile"] != Alpha || root["source_sha"] != buildSource {
		return nil, errPolicy
	}
	capabilities, ok := states(root["capabilities"], capabilityIDs)
	if !ok {
		return nil, errPolicy
	}
	for _, state := range capabilities {
		if state != "disabled" {
			return nil, errPolicy
		}
	}
	intents := append(append([]string{}, baselineIntents...), restrictedIntents...)
	rich, ok := states(root["rich_intents"], intents)
	if !ok {
		return nil, errPolicy
	}
	for _, intent := range baselineIntents {
		if rich[intent] != "qualified" {
			return nil, errPolicy
		}
	}
	for _, state := range rich {
		if state != "qualified" && state != "disabled" {
			return nil, errPolicy
		}
	}
	return &Policy{profile: Alpha, source: buildSource, digest: expectedDigest, capabilities: capabilities, rich: rich}, nil
}

// Load requires an explicit local development profile or a bound mounted alpha
// artifact. readFile is supplied by the bootstrap and is called only once.
func Load(lookup func(string) (string, bool), buildSource string, readFile func(string) ([]byte, error)) (*Policy, error) {
	if lookup == nil {
		return nil, errPolicy
	}
	profile, _ := lookup("VOICE_RELEASE_PROFILE")
	path, pathSet := lookup("VOICE_RELEASE_POLICY_FILE")
	digest, digestSet := lookup("VOICE_RELEASE_POLICY_SHA256")
	if profile == Development {
		if pathSet || digestSet {
			return nil, errPolicy
		}
		return &Policy{profile: Development}, nil
	}
	if profile != Alpha || !pathSet || !digestSet || !filepath.IsAbs(path) || readFile == nil {
		return nil, errPolicy
	}
	data, err := readFile(path)
	if err != nil {
		return nil, errPolicy
	}
	return Parse(data, digest, buildSource)
}

func (p *Policy) Allows(capability string) bool {
	if p == nil || !known(capability, capabilityIDs) {
		return false
	}
	return p.profile == Development
}
func (p *Policy) Profile() string {
	if p == nil {
		return ""
	}
	return p.profile
}
func (p *Policy) SourceSHA() string {
	if p == nil {
		return ""
	}
	return p.source
}
func (p *Policy) SHA256() string {
	if p == nil {
		return ""
	}
	return p.digest
}
func (p *Policy) CapabilityStates() map[string]string {
	out := make(map[string]string, len(capabilityIDs))
	for _, id := range capabilityIDs {
		if p != nil && p.Allows(id) {
			out[id] = "development"
		} else {
			out[id] = "disabled"
		}
	}
	return out
}
func (p *Policy) RichIntentQualified(intent string) bool {
	if p == nil || (!known(intent, baselineIntents) && !known(intent, restrictedIntents)) {
		return false
	}
	return p.profile == Development || p.rich[intent] == "qualified"
}
func known(value string, list []string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
func lowerHex(value string, length int) bool {
	if len(value) != length || value == strings.Repeat("0", length) {
		return false
	}
	for _, ch := range value {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return false
		}
	}
	return true
}
func keys(value map[string]any, required []string) bool {
	if len(value) != len(required) {
		return false
	}
	for _, key := range required {
		if _, ok := value[key]; !ok {
			return false
		}
	}
	return true
}
func states(value any, required []string) (map[string]string, bool) {
	object, ok := value.(map[string]any)
	if !ok || !keys(object, required) {
		return nil, false
	}
	out := make(map[string]string, len(object))
	for key, value := range object {
		state, ok := value.(string)
		if !ok {
			return nil, false
		}
		out[key] = state
	}
	return out, true
}
func strictValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 4 {
		return nil, errPolicy
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); ok {
		if delimiter != '{' {
			return nil, errPolicy
		}
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errPolicy
			}
			if _, exists := object[key]; exists {
				return nil, errPolicy
			}
			value, err := strictValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, errPolicy
		}
		return object, nil
	}
	return token, nil
}
