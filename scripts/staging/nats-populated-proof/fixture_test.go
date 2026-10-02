package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
)

// Disposable signing keys exist only inside tests. Never export this helper as
// production credential issuance or count its output as deployed evidence.
type testBundle struct {
	dir, acl, trust, pin string
	now                  time.Time
	m                    manifest
	files                map[string][]byte
	account, operator    nkeys.KeyPair
	witness              ed25519.PrivateKey
	p                    policy
}

func must[T any](v T, e error) T {
	if e != nil {
		panic("fixture preparation failed")
	}
	return v
}
func newBundle(t *testing.T) *testBundle {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "bundle")
	if os.Mkdir(dir, 0700) != nil {
		t.Fatal("fixture directory failed")
	}
	b := &testBundle{dir: dir, acl: filepath.Join(root, "acl.yaml"), trust: filepath.Join(root, "approved-trust.json"), now: time.Now().UTC().Truncate(time.Second), files: map[string][]byte{}}
	raw := must(os.ReadFile("../../../deploy/nats/acl-intent.yaml"))
	if yaml.Unmarshal(raw, &b.p) != nil {
		t.Fatal("fixture ACL failed")
	}
	writeTest(t, b.acl, raw)
	b.operator = must(nkeys.CreateOperator())
	b.account = must(nkeys.CreateAccount())
	sys := must(nkeys.CreateAccount())
	t.Cleanup(func() { b.operator.Wipe(); b.account.Wipe(); sys.Wipe() })
	opPub := must(b.operator.PublicKey())
	appPub := must(b.account.PublicKey())
	sysPub := must(sys.PublicKey())
	op := jwt.NewOperatorClaims(opPub)
	op.SystemAccount = sysPub
	b.files["operator.jwt"] = []byte(must(op.Encode(b.operator)))
	app := jwt.NewAccountClaims(appPub)
	app.Limits = jwt.OperatorLimits{AccountLimits: jwt.AccountLimits{Conn: 64, LeafNodeConn: 64}, NatsLimits: jwt.NatsLimits{Subs: 1024, Data: 64 << 20, Payload: 1 << 20}, JetStreamLimits: jwt.JetStreamLimits{MemoryStorage: 64 << 20, DiskStorage: 512 << 20, Streams: 64, Consumer: 512, MaxAckPending: 4096}}
	b.files["account.jwt"] = []byte(must(app.Encode(b.operator)))
	b.files["system-account.jwt"] = []byte(must(jwt.NewAccountClaims(sysPub).Encode(b.operator)))
	b.files["account.public"] = []byte(appPub)
	b.files["system-account.public"] = []byte(sysPub)
	for _, s := range append(append([]string{}, services...), "bootstrap") {
		g := b.p.Services[s]
		if s == "bootstrap" {
			g = b.p.Bootstrap
		}
		b.files[s+".creds"] = testCredential(t, b.account, g, nil)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("fixture TLS failed")
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "voice-nats"}, DNSNames: []string{"voice-nats"}, NotBefore: b.now.Add(-time.Hour), NotAfter: b.now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der := must(x509.CreateCertificate(rand.Reader, cert, cert, pub, key))
	b.files["tls.crt"] = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	b.files["ca.crt"] = b.files["tls.crt"]
	b.files["tls.key"] = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: must(x509.MarshalPKCS8PrivateKey(key))})
	b.files["hub.conf"] = []byte(hubConfig(b.files))
	b.files["leaf.conf"] = []byte(leafConfig)
	witnessPub, witnessKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("fixture witness failed")
	}
	b.witness = witnessKey
	b.m = manifest{Schema: schema, Kind: "deployed", Namespace: "voice-staging", Generation: "r20261002test", CapturedAt: b.now, ReleaseSHA: strings.Repeat("a", 40), ACLSHA: hash(raw), ClusterUID: "test-cluster", HubPodUID: "test-pod", HubImage: natsImage, LeafImage: natsImage, Mounts: map[string]mountedSource{}, Files: map[string]string{}}
	trust := trustPolicy{PublicKey: base64.StdEncoding.EncodeToString(witnessPub), ClusterUID: b.m.ClusterUID, HubPodUID: b.m.HubPodUID, Generation: b.m.Generation, ReleaseSHA: b.m.ReleaseSHA, ACLSHA: b.m.ACLSHA}
	trustBytes := must(json.Marshal(trust))
	writeTest(t, b.trust, trustBytes)
	b.pin = hash(trustBytes)
	b.save(t)
	return b
}
func writeTest(t *testing.T, path string, raw []byte) {
	t.Helper()
	if os.WriteFile(path, raw, 0600) != nil {
		t.Fatal("fixture write failed")
	}
}
func (b *testBundle) save(t *testing.T) {
	t.Helper()
	for n, raw := range b.files {
		b.m.Files[n] = hash(raw)
		b.m.Mounts[n] = mountedSource{Resource: "test-source", UID: "test-uid", ResourceVersion: "1", Key: n, Immutable: true}
		writeTest(t, filepath.Join(b.dir, n), raw)
	}
	raw := must(json.Marshal(b.m))
	writeTest(t, filepath.Join(b.dir, "manifest.json"), raw)
	writeTest(t, filepath.Join(b.dir, "manifest.sig"), []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(b.witness, raw))))
}
func testCredential(t *testing.T, account nkeys.KeyPair, g grant, mutate func(*jwt.UserClaims)) []byte {
	t.Helper()
	u := must(nkeys.CreateUser())
	defer u.Wipe()
	c := jwt.NewUserClaims(must(u.PublicKey()))
	c.Pub.Allow = g.Publish
	c.Sub.Allow = g.Subscribe
	if len(g.Publish) == 0 {
		c.Pub.Deny = []string{">"}
	}
	if len(g.Subscribe) == 0 {
		c.Sub.Deny = []string{">"}
	}
	if mutate != nil {
		mutate(c)
	}
	token := must(c.Encode(account))
	seed := must(u.Seed())
	return []byte("-----BEGIN NATS USER JWT-----\n" + token + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n")
}
func TestVerifiedMountedBundle(t *testing.T) {
	b := newBundle(t)
	if _, e := loadInput(b.dir, b.acl, b.trust, b.pin, false, b.now); e != nil {
		t.Fatalf("valid signed disposable bundle: %v", e)
	}
}
func TestBundleRefusesUnapprovedInputs(t *testing.T) {
	cases := []struct {
		name, code string
		change     func(*testing.T, *testBundle)
	}{
		{"unpinned witness", "trust_policy_pin_mismatch", func(t *testing.T, b *testBundle) { b.pin = strings.Repeat("0", 64) }},
		{"self attested witness", "trust_policy_inside_bundle", func(t *testing.T, b *testBundle) {
			raw := must(os.ReadFile(b.trust))
			b.trust = filepath.Join(b.dir, "trust.json")
			writeTest(t, b.trust, raw)
		}},
		{"wrong active generation", "trusted_provenance_mismatch", func(t *testing.T, b *testBundle) { b.m.Generation = "r20261001old"; b.save(t) }},
		{"stale witness", "provenance_stale", func(t *testing.T, b *testBundle) { b.m.CapturedAt = b.now.Add(-16 * time.Minute); b.save(t) }},
		{"unapproved runtime", "runtime_provenance_mismatch", func(t *testing.T, b *testBundle) { b.m.HubImage = "nats:latest"; b.save(t) }},
		{"external leaf route", "mounted_config_not_supported", func(t *testing.T, b *testBundle) {
			b.files["leaf.conf"] = []byte(strings.ReplaceAll(leafConfig, "voice-nats:7422", "production.example:7422"))
			b.save(t)
		}},
		{"mutable credential source", "credential_source_not_immutable", func(t *testing.T, b *testBundle) {
			s := b.m.Mounts["realtime.creds"]
			s.Immutable = false
			b.m.Mounts["realtime.creds"] = s
			raw := must(json.Marshal(b.m))
			writeTest(t, filepath.Join(b.dir, "manifest.json"), raw)
			writeTest(t, filepath.Join(b.dir, "manifest.sig"), []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(b.witness, raw))))
		}},
		{"expired credentials", "service_validity_invalid", func(t *testing.T, b *testBundle) {
			b.files["realtime.creds"] = testCredential(t, b.account, b.p.Services["realtime"], func(c *jwt.UserClaims) { c.Expires = b.now.Add(5 * time.Minute).Unix() })
			b.save(t)
		}},
		{"wrong APP issuer", "service_trust_invalid", func(t *testing.T, b *testBundle) {
			a := must(nkeys.CreateAccount())
			defer a.Wipe()
			b.files["realtime.creds"] = testCredential(t, a, b.p.Services["realtime"], nil)
			b.save(t)
		}},
		{"broad response permission", "current_grant_mismatch", func(t *testing.T, b *testBundle) {
			b.files["realtime.creds"] = testCredential(t, b.account, b.p.Services["realtime"], func(c *jwt.UserClaims) { c.Resp = &jwt.ResponsePermission{MaxMsgs: 1, Expires: time.Second} })
			b.save(t)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBundle(t)
			tc.change(t, b)
			_, e := loadInput(b.dir, b.acl, b.trust, b.pin, false, b.now)
			if e == nil || e.Error() != tc.code {
				t.Fatalf("want %s; got %v", tc.code, e)
			}
		})
	}
}
func TestEachNewGrantIsRequired(t *testing.T) {
	missing := map[string][]string{"realtime": {"$JS.API.CONSUMER.INFO.social_events.rt_realtime1_friend_removed", "$JS.ACK.social_events.rt_realtime1_friend_removed.>", "_INBOX.voice.realtime1.friend_removed"}, "bootstrap": {"$JS.API.STREAM.UPDATE.social_events", "$JS.API.CONSUMER.INFO.social_events.rt_realtime1_friend_removed", "$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed"}}
	for service, subjects := range missing {
		for _, subject := range subjects {
			t.Run(service+"/"+subject, func(t *testing.T) {
				b := newBundle(t)
				g := b.p.Services[service]
				if service == "bootstrap" {
					g = b.p.Bootstrap
				}
				remove := func(a []string) []string {
					var out []string
					for _, s := range a {
						if s != subject {
							out = append(out, s)
						}
					}
					return out
				}
				g.Publish = remove(g.Publish)
				g.Subscribe = remove(g.Subscribe)
				b.files[service+".creds"] = testCredential(t, b.account, g, nil)
				b.save(t)
				_, e := loadInput(b.dir, b.acl, b.trust, b.pin, false, b.now)
				if e == nil || e.Error() != "current_grant_mismatch" {
					t.Fatalf("old grant set must fail: %v", e)
				}
			})
		}
	}
}
func TestEveryProfileRejectsBroaderPublishAndSubscribe(t *testing.T) {
	for _, service := range append(append([]string{}, services...), "bootstrap") {
		for _, direction := range []string{"pub", "sub"} {
			t.Run(service+"/"+direction, func(t *testing.T) {
				b := newBundle(t)
				g := b.p.Services[service]
				if service == "bootstrap" {
					g = b.p.Bootstrap
				}
				if direction == "pub" {
					g.Publish = append(append([]string{}, g.Publish...), "unauthorized.business.event")
				} else {
					g.Subscribe = append(append([]string{}, g.Subscribe...), "_INBOX.voice.foreign.>")
				}
				b.files[service+".creds"] = testCredential(t, b.account, g, nil)
				b.save(t)
				_, e := loadInput(b.dir, b.acl, b.trust, b.pin, false, b.now)
				if e == nil || e.Error() != "current_grant_mismatch" {
					t.Fatalf("broader %s grant for %s accepted: %v", direction, service, e)
				}
			})
		}
	}
}
func TestRevokedUserCannotPass(t *testing.T) {
	b := newBundle(t)
	token := must(nkeys.ParseDecoratedJWT(b.files["realtime.creds"]))
	user := must(jwt.DecodeUserClaims(token))
	app := must(jwt.DecodeAccountClaims(string(b.files["account.jwt"])))
	app.Revocations = jwt.RevocationList{user.Subject: b.now.Unix()}
	b.files["account.jwt"] = []byte(must(app.Encode(b.operator)))
	b.files["hub.conf"] = []byte(hubConfig(b.files))
	b.save(t)
	_, e := loadInput(b.dir, b.acl, b.trust, b.pin, false, b.now)
	if e == nil || e.Error() != "service_revoked" {
		t.Fatalf("revoked exact user accepted: %v", e)
	}
}
