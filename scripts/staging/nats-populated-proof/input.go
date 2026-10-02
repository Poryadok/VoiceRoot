package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
)

const schema = "isolated-deployed-config-clone-v1"
const natsImage = "nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb"

var services = strings.Fields("analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice")
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var shaPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var generationPattern = regexp.MustCompile(`^r[0-9]{8}[a-z0-9]{0,8}$`)

type grant struct {
	Publish    []string `yaml:"publish"`
	Subscribe  []string `yaml:"subscribe"`
	NoResponse bool     `yaml:"no_response"`
}
type policy struct {
	Version   int              `yaml:"version"`
	Services  map[string]grant `yaml:"services"`
	Bootstrap grant            `yaml:"bootstrap"`
}

// Witness signs the exact manifest bytes. Its public key must come from an
// independently approved custody channel, never from the input being attested.
// File digests cover opaque bytes actually read from the named mounted sources.
type manifest struct {
	Schema     string                   `json:"schema"`
	Kind       string                   `json:"kind"`
	Namespace  string                   `json:"namespace"`
	Generation string                   `json:"generation"`
	CapturedAt time.Time                `json:"captured_at"`
	ReleaseSHA string                   `json:"release_sha"`
	ACLSHA     string                   `json:"acl_sha"`
	ClusterUID string                   `json:"cluster_uid"`
	HubPodUID  string                   `json:"hub_pod_uid"`
	HubImage   string                   `json:"hub_image"`
	LeafImage  string                   `json:"leaf_image"`
	Mounts     map[string]mountedSource `json:"mounts"`
	Files      map[string]string        `json:"files"`
}
type mountedSource struct {
	Resource        string `json:"resource"`
	UID             string `json:"uid"`
	ResourceVersion string `json:"resource_version"`
	Key             string `json:"key"`
	Immutable       bool   `json:"immutable"`
}
type verifiedInput struct {
	Manifest    manifest
	Files       map[string][]byte
	ACL         policy
	ManifestSHA string
}

// This policy is supplied through an independently approved channel. Its digest
// is pinned separately by the reviewer/operator; a bundle cannot attest itself.
type trustPolicy struct {
	PublicKey  string `json:"public_key"`
	ClusterUID string `json:"cluster_uid"`
	HubPodUID  string `json:"hub_pod_uid"`
	Generation string `json:"generation"`
	ReleaseSHA string `json:"release_sha"`
	ACLSHA     string `json:"acl_sha"`
}

func failure(code string) error { return errors.New(code) }
func hash(b []byte) string      { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func protectedRead(path string, max int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > max {
		return nil, failure("protected_file_invalid")
	}
	if runtime.GOOS == "linux" && info.Mode().Perm() != 0400 && info.Mode().Perm() != 0600 {
		return nil, failure("protected_file_mode")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, failure("protected_read_failed")
	}
	return b, nil
}
func decodeStrict(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return failure("manifest_invalid")
	}
	return nil
}
func requiredFiles() []string {
	names := strings.Fields("operator.jwt account.jwt system-account.jwt account.public system-account.public tls.crt tls.key ca.crt hub.conf leaf.conf bootstrap.creds")
	for _, s := range services {
		names = append(names, s+".creds")
	}
	return names
}
func validateInput(dir, acl, witness string, fixture bool) error {
	_, err := loadInput(dir, acl, witness, "", fixture, time.Now())
	return err
}
func loadInput(dir, acl, witness, trustSHA string, fixture bool, now time.Time) (*verifiedInput, error) {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || runtime.GOOS == "linux" && info.Mode().Perm() != 0700 {
		return nil, failure("protected_directory_invalid")
	}
	raw, err := protectedRead(filepath.Join(dir, "manifest.json"), 1<<20)
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := decodeStrict(raw, &m); err != nil {
		return nil, err
	}
	if m.Schema != schema || m.Namespace != "voice-staging" || !generationPattern.MatchString(m.Generation) || !shaPattern.MatchString(m.ReleaseSHA) || !digestPattern.MatchString(m.ACLSHA) {
		return nil, failure("provenance_identity_invalid")
	}
	if m.HubImage != natsImage || m.LeafImage != natsImage {
		return nil, failure("runtime_provenance_mismatch")
	}
	if m.Kind != "deployed" && m.Kind != "candidate" && m.Kind != "fixture" || fixture != (m.Kind == "fixture") {
		return nil, failure("evidence_kind_mismatch")
	}
	if now.Sub(m.CapturedAt) > 15*time.Minute || m.CapturedAt.After(now.Add(30*time.Second)) {
		return nil, failure("provenance_stale")
	}
	if !fixture {
		if m.ClusterUID == "" || m.HubPodUID == "" {
			return nil, failure("mounted_provenance_missing")
		}
		bundleAbs, err := filepath.Abs(dir)
		if err != nil {
			return nil, failure("trust_policy_location_invalid")
		}
		trustAbs, err := filepath.Abs(witness)
		if err != nil {
			return nil, failure("trust_policy_location_invalid")
		}
		rel, err := filepath.Rel(bundleAbs, trustAbs)
		if err != nil || rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, failure("trust_policy_inside_bundle")
		}
		pub, err := protectedRead(witness, 4096)
		if err != nil {
			return nil, failure("witness_key_missing")
		}
		if !digestPattern.MatchString(trustSHA) || hash(pub) != trustSHA {
			return nil, failure("trust_policy_pin_mismatch")
		}
		var trust trustPolicy
		if decodeStrict(pub, &trust) != nil || trust.ClusterUID != m.ClusterUID || trust.HubPodUID != m.HubPodUID || trust.Generation != m.Generation || trust.ReleaseSHA != m.ReleaseSHA || trust.ACLSHA != m.ACLSHA {
			return nil, failure("trusted_provenance_mismatch")
		}
		key, err := base64.StdEncoding.DecodeString(trust.PublicKey)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, failure("witness_key_invalid")
		}
		sig, err := protectedRead(filepath.Join(dir, "manifest.sig"), 256)
		if err != nil {
			return nil, failure("witness_missing")
		}
		signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
		if err != nil || !ed25519.Verify(ed25519.PublicKey(key), raw, signature) {
			return nil, failure("witness_signature_invalid")
		}
	}
	aclBytes, err := os.ReadFile(acl)
	if err != nil || hash(aclBytes) != m.ACLSHA {
		return nil, failure("acl_digest_mismatch")
	}
	var p policy
	decoder := yaml.NewDecoder(bytes.NewReader(aclBytes))
	decoder.KnownFields(true)
	if decoder.Decode(&p) != nil || decoder.Decode(new(any)) != io.EOF || p.Version != 1 || len(p.Services) != len(services) {
		return nil, failure("acl_manifest_invalid")
	}
	required := requiredFiles()
	if len(m.Files) != len(required) || len(m.Mounts) != len(required) {
		return nil, failure("mounted_file_set_mismatch")
	}
	out := &verifiedInput{Manifest: m, Files: map[string][]byte{}, ACL: p, ManifestSHA: hash(raw)}
	for _, name := range required {
		mount, ok := m.Mounts[name]
		if !ok || mount.Resource == "" || mount.UID == "" || mount.ResourceVersion == "" || mount.Key == "" || strings.ContainsAny(mount.Resource, "/\\") {
			return nil, failure("mounted_provenance_missing")
		}
		if strings.HasSuffix(name, ".creds") || strings.HasSuffix(name, ".jwt") || name == "tls.key" {
			if !mount.Immutable {
				return nil, failure("credential_source_not_immutable")
			}
		}
		b, err := protectedRead(filepath.Join(dir, name), 1<<20)
		if err != nil {
			return nil, err
		}
		if !digestPattern.MatchString(m.Files[name]) || hash(b) != m.Files[name] {
			return nil, failure("mounted_bytes_mismatch")
		}
		out.Files[name] = b
	}
	if err := verifyIdentities(out, now); err != nil {
		return nil, err
	}
	if err := verifyTLS(out, now); err != nil {
		return nil, err
	}
	if normalizeConfig(string(out.Files["hub.conf"])) != normalizeConfig(hubConfig(out.Files)) || normalizeConfig(string(out.Files["leaf.conf"])) != normalizeConfig(leafConfig) {
		return nil, failure("mounted_config_not_supported")
	}
	return out, nil
}

type validatable interface{ Validate(*jwt.ValidationResults) }

func claimsValid(c validatable) bool {
	v := jwt.CreateValidationResults()
	c.Validate(v)
	return !v.IsBlocking(true)
}
func verifySignature(token, public string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	key, err := nkeys.FromPublicKey(public)
	return err == nil && key.Verify([]byte(parts[0]+"."+parts[1]), sig) == nil
}
func sameSet(a, b []string) bool {
	a = slices.Clone(a)
	b = slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
func verifyIdentities(in *verifiedInput, now time.Time) error {
	opToken := strings.TrimSpace(string(in.Files["operator.jwt"]))
	appToken := strings.TrimSpace(string(in.Files["account.jwt"]))
	sysToken := strings.TrimSpace(string(in.Files["system-account.jwt"]))
	op, err := jwt.DecodeOperatorClaims(opToken)
	if err != nil || !claimsValid(op) || !verifySignature(opToken, op.Subject) {
		return failure("operator_trust_invalid")
	}
	app, err := jwt.DecodeAccountClaims(appToken)
	if err != nil || !claimsValid(app) || app.Issuer != op.Subject || !verifySignature(appToken, op.Subject) || app.Subject != strings.TrimSpace(string(in.Files["account.public"])) || !app.Limits.IsJSEnabled() {
		return failure("app_trust_invalid")
	}
	sys, err := jwt.DecodeAccountClaims(sysToken)
	if err != nil || !claimsValid(sys) || sys.Issuer != op.Subject || !verifySignature(sysToken, op.Subject) || sys.Subject != op.SystemAccount || sys.Subject == app.Subject || sys.Subject != strings.TrimSpace(string(in.Files["system-account.public"])) || sys.Limits.IsJSEnabled() {
		return failure("system_trust_invalid")
	}
	if len(app.Imports) > 0 || len(app.Exports) > 0 || len(sys.Imports) > 0 || len(sys.Exports) > 0 || len(app.Mappings) > 0 || len(sys.Mappings) > 0 || app.HasExternalAuthorization() || sys.HasExternalAuthorization() {
		return failure("account_routes_not_supported")
	}
	names := append(slices.Clone(services), "bootstrap")
	for _, name := range names {
		credential := in.Files[name+".creds"]
		token, err := nkeys.ParseDecoratedJWT(credential)
		if err != nil {
			return failure("service_credentials_invalid")
		}
		c, err := jwt.DecodeUserClaims(token)
		if err != nil || !claimsValid(c) || c.Issuer != app.Subject || !verifySignature(token, app.Subject) {
			return failure("service_trust_invalid")
		}
		key, err := nkeys.ParseDecoratedNKey(credential)
		if err != nil {
			return failure("service_seed_invalid")
		}
		public, err := key.PublicKey()
		key.Wipe()
		if err != nil || public != c.Subject {
			return failure("service_seed_mismatch")
		}
		if c.IssuedAt > now.Unix()+30 || c.NotBefore > now.Unix() || c.Expires != 0 && c.Expires < now.Add(10*time.Minute).Unix() {
			return failure("service_validity_invalid")
		}
		for revoked, at := range app.Revocations {
			if (revoked == c.Subject || revoked == "*") && c.IssuedAt <= at {
				return failure("service_revoked")
			}
		}
		g, ok := in.ACL.Services[name]
		if name == "bootstrap" {
			g = in.ACL.Bootstrap
			ok = true
		}
		if !ok || !g.NoResponse || c.Resp != nil || c.BearerToken || c.ProxyRequired || !sameSet(c.Pub.Allow, g.Publish) || !sameSet(c.Sub.Allow, g.Subscribe) {
			return failure("current_grant_mismatch")
		}
		wantPubDeny := []string(nil)
		wantSubDeny := []string(nil)
		if len(g.Publish) == 0 {
			wantPubDeny = []string{">"}
		}
		if len(g.Subscribe) == 0 {
			wantSubDeny = []string{">"}
		}
		if !sameSet(c.Pub.Deny, wantPubDeny) || !sameSet(c.Sub.Deny, wantSubDeny) {
			return failure("current_deny_mismatch")
		}
	}
	return nil
}
func verifyTLS(in *verifiedInput, now time.Time) error {
	pair, err := tls.X509KeyPair(in.Files["tls.crt"], in.Files["tls.key"])
	if err != nil || len(pair.Certificate) == 0 {
		return failure("tls_key_invalid")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return failure("tls_certificate_invalid")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(in.Files["ca.crt"]) {
		return failure("tls_ca_invalid")
	}
	intermediate := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return failure("tls_certificate_invalid")
		}
		intermediate.AddCert(c)
	}
	_, err = leaf.Verify(x509.VerifyOptions{DNSName: "voice-nats", Roots: roots, Intermediates: intermediate, CurrentTime: now})
	if err != nil {
		return failure("tls_trust_invalid")
	}
	return nil
}
func normalizeConfig(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		lines = append(lines, line)
	}
	return strings.Join(strings.Fields(strings.Join(lines, "\n")), " ")
}
func hubConfig(files map[string][]byte) string {
	return fmt.Sprintf(`server_name: voice-nats-hub
max_control_line: 32768
jetstream { store_dir: /data }
http: 8222
operator: /etc/nats/jwt/operator.jwt
resolver: MEMORY
resolver_preload: {
%s: %q
%s: %q
}
system_account: %s
leafnodes {
listen: 0.0.0.0:7422
tls {
cert_file: /etc/nats/tls/tls.crt
key_file: /etc/nats/tls/tls.key
ca_file: /etc/nats/tls/ca.crt
handshake_first: true
}
}`, strings.TrimSpace(string(files["account.public"])), strings.TrimSpace(string(files["account.jwt"])), strings.TrimSpace(string(files["system-account.public"])), strings.TrimSpace(string(files["system-account.jwt"])), strings.TrimSpace(string(files["system-account.public"])))
}

const leafConfig = `listen: 127.0.0.1:4222
default_js_domain: { "$G": "" }
leafnodes {
remotes = [{
urls: ["nats-leaf://voice-nats:7422"]
account: "$G"
credentials: $NATS_CREDS
tls { ca_file: /etc/nats/tls/ca.crt; handshake_first: true }
}]
}`
