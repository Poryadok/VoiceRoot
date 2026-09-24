package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

func TestIssueWritesProtectedFourSecretRestoreList(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("0600 issuance requires Linux")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	cert, key, ca := testTLS(t, parent, "voice-nats")
	dest := filepath.Join(parent, "issued")
	if err := issue(canonicalACL(), dest, "voice-staging", cert, key, ca); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"signing-seeds/operator.seed", "signing-seeds/app-account.seed", "signing-seeds/system-account.seed",
		"voice-nats-operator.yaml", "voice-nats-bootstrap-credentials.yaml",
		"voice-nats-service-credentials.yaml", "voice-nats-hub-tls.yaml", "secrets.json",
	} {
		info, err := os.Stat(filepath.Join(dest, path))
		if err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("%s mode = %o, want 0600", path, info.Mode().Perm())
		}
	}
	data, err := os.ReadFile(filepath.Join(dest, "secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Kind  string `json:"kind"`
		Items []struct {
			Metadata struct{ Name, Namespace string } `json:"metadata"`
			Type     string                           `json:"type"`
			Data     map[string]string                `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}
	if list.Kind != "List" || len(list.Items) != 4 {
		t.Fatalf("restore List has kind %s and %d items", list.Kind, len(list.Items))
	}
	expected := map[string][]string{
		"voice-nats-operator":              {"operator.jwt", "account.jwt", "system-account.jwt", "account.public", "system-account.public"},
		"voice-nats-bootstrap-credentials": {"bootstrap.creds"},
		"voice-nats-service-credentials":   {"analytics.creds", "auth.creds", "bot.creds", "chat.creds", "file.creds", "gateway.creds", "matchmaking.creds", "messaging.creds", "moderation.creds", "notification.creds", "realtime.creds", "role.creds", "search.creds", "social.creds", "space.creds", "story.creds", "subscription.creds", "user.creds", "voice.creds"},
		"voice-nats-hub-tls":               {"tls.crt", "tls.key", "ca.crt"},
	}
	secretData := map[string]map[string]string{}
	for _, item := range list.Items {
		if item.Metadata.Namespace != "voice-staging" || item.Type != "Opaque" {
			t.Errorf("%s has wrong namespace/type", item.Metadata.Name)
		}
		keys, ok := expected[item.Metadata.Name]
		if !ok {
			t.Errorf("unexpected secret %s", item.Metadata.Name)
			continue
		}
		secretData[item.Metadata.Name] = item.Data
		if len(item.Data) != len(keys) {
			t.Errorf("%s has %d keys, want %d", item.Metadata.Name, len(item.Data), len(keys))
		}
		for _, key := range keys {
			if item.Data[key] == "" {
				t.Errorf("%s missing nonempty data %s", item.Metadata.Name, key)
			}
		}
		delete(expected, item.Metadata.Name)
	}
	if len(expected) != 0 {
		t.Errorf("missing Secrets: %v", expected)
	}
	acl, err := readPolicy(canonicalACL())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range serviceNames {
		contents, err := base64.StdEncoding.DecodeString(secretData["voice-nats-service-credentials"][name+".creds"])
		if err != nil {
			t.Fatal(err)
		}
		claims, err := jwt.DecodeUserClaims(credentialJWT(t, string(contents)))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(claims.Pub.Allow, acl.Services[name].Publish) || !slices.Equal(claims.Sub.Allow, acl.Services[name].Subscribe) {
			t.Errorf("%s JWT permissions differ from reviewed ACL", name)
		}
		if claims.Resp != nil {
			t.Errorf("%s JWT grants response permission outside the exact publish ACL", name)
		}
	}
	bootstrapCreds, err := base64.StdEncoding.DecodeString(secretData["voice-nats-bootstrap-credentials"]["bootstrap.creds"])
	if err != nil {
		t.Fatal(err)
	}
	bootstrapClaims, err := jwt.DecodeUserClaims(credentialJWT(t, string(bootstrapCreds)))
	if err != nil {
		t.Fatal(err)
	}
	if bootstrapClaims.Resp != nil {
		t.Error("bootstrap JWT grants response permission outside the exact publish ACL")
	}
	for _, tc := range []struct{ seed, jwtKey string }{{"operator.seed", "operator.jwt"}, {"app-account.seed", "account.jwt"}, {"system-account.seed", "system-account.jwt"}} {
		seed, err := os.ReadFile(filepath.Join(dest, "signing-seeds", tc.seed))
		if err != nil {
			t.Fatal(err)
		}
		pair, err := nkeys.FromSeed([]byte(strings.TrimSpace(string(seed))))
		if err != nil {
			t.Fatal(err)
		}
		public, err := pair.PublicKey()
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := base64.StdEncoding.DecodeString(secretData["voice-nats-operator"][tc.jwtKey])
		if err != nil {
			t.Fatal(err)
		}
		var subject string
		if tc.seed == "operator.seed" {
			claims, err := jwt.DecodeOperatorClaims(string(encoded))
			if err != nil {
				t.Fatal(err)
			}
			subject = claims.Subject
		} else {
			claims, err := jwt.DecodeAccountClaims(string(encoded))
			if err != nil {
				t.Fatal(err)
			}
			subject = claims.Subject
		}
		if public != subject {
			t.Errorf("%s does not sign its exported JWT", tc.seed)
		}
	}
	if err := issue(canonicalACL(), dest, "voice-staging", cert, key, ca); err == nil {
		t.Fatal("issuer overwrote an existing destination")
	}
}

func credentialJWT(t *testing.T, creds string) string {
	t.Helper()
	const begin = "-----BEGIN NATS USER JWT-----\n"
	const end = "\n------END NATS USER JWT------"
	start, stop := strings.Index(creds, begin), strings.Index(creds, end)
	if start < 0 || stop <= start {
		t.Fatal("invalid .creds format")
	}
	return creds[start+len(begin) : stop]
}

func TestIssueRejectsInvalidTLSAndACLWithoutOutput(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("issuer is intentionally Linux-only")
	}
	parent := t.TempDir()
	wrongCert, wrongKey, ca := testTLS(t, parent, "wrong-host")
	for _, tc := range []struct {
		name, namespace, cert, key, ca string
		acl                            string
	}{
		{"wrong SAN", "voice-staging", wrongCert, wrongKey, ca, canonicalACL()},
		{"wrong namespace", "default", wrongCert, wrongKey, ca, canonicalACL()},
		{"invalid ACL", "voice-staging", wrongCert, wrongKey, ca, filepath.Join(parent, "missing-acl.yaml")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := filepath.Join(parent, tc.name)
			if err := issue(tc.acl, dest, tc.namespace, tc.cert, tc.key, tc.ca); err == nil {
				t.Fatal("invalid issuance succeeded")
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Errorf("failed issuance left output: %v", err)
			}
		})
	}
}

func TestPolicyRejectsServiceMutationAndBroadInbox(t *testing.T) {
	base, err := os.ReadFile(canonicalACL())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readPolicy(canonicalACL()); err != nil {
		t.Fatalf("canonical policy rejected: %v", err)
	}
	for _, tc := range []struct{ old, replacement string }{
		{"$JS.API.CONSUMER.INFO.message_events.chat_message_activity", "$JS.API.CONSUMER.CREATE.message_events.chat_message_activity"},
		{"$JS.API.CONSUMER.INFO.message_events.chat_message_activity", "$JS.API.STREAM.PURGE.message_events"},
		{"$JS.API.CONSUMER.INFO.message_events.chat_message_activity", "$JS.API.CONSUMER.PAUSE.message_events.chat_message_activity"},
		{"_INBOX.voice.chat.>", "_INBOX.>"},
		{"_INBOX.voice.chat.>", "_INBOX.voice.messaging.>"},
		{"_INBOX.voice.chat.chat_message_activity", "_INBOX.voice.messaging.foreign"},
	} {
		contents := strings.Replace(string(base), tc.old, tc.replacement, 1)
		path := filepath.Join(t.TempDir(), "unsafe.yaml")
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readPolicy(path); err == nil {
			t.Errorf("unsafe ACL grant %q accepted", tc.replacement)
		}
	}
}

func TestIssueRejectsUnsafeParentMode(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("issuer is intentionally Linux-only")
	}
	parent := t.TempDir()
	cert, key, ca := testTLS(t, parent, "voice-nats")
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0700)
	dest := filepath.Join(parent, "issued")
	if err := issue(canonicalACL(), dest, "voice-staging", cert, key, ca); err == nil {
		t.Fatal("issuer accepted group/world-readable parent")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("failed issuance left output: %v", err)
	}
}

func TestTLSRequiresExactVoiceNatsSANAndTrustedChain(t *testing.T) {
	dir := t.TempDir()
	cert, key, ca := testTLS(t, dir, "voice-nats")
	if _, _, _, err := readAndVerifyTLS(cert, key, ca); err != nil {
		t.Fatal(err)
	}
	wrongCert, wrongKey, wrongCA := testTLS(t, dir, "wrong-host")
	if _, _, _, err := readAndVerifyTLS(wrongCert, wrongKey, wrongCA); err == nil {
		t.Error("wrong SAN accepted")
	}
	if _, _, _, err := readAndVerifyTLS(cert, key, wrongCA); err == nil {
		t.Error("untrusted chain accepted")
	}
}

func canonicalACL() string {
	return filepath.Join("..", "..", "..", "..", "..", "deploy", "nats", "acl-intent.yaml")
}

func testTLS(t *testing.T, dir, host string) (string, string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: host}, DNSNames: []string{host}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, keyPath, ca := filepath.Join(dir, host+".crt"), filepath.Join(dir, host+".key"), filepath.Join(dir, host+"-ca.crt")
	for path, content := range map[string][]byte{cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), keyPath: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pemKey}), ca: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})} {
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Contains(template.DNSNames, host) {
		t.Fatal("test TLS fixture missing SAN")
	}
	return cert, keyPath, ca
}
