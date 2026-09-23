package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/jwt/v2"
)

func TestGenerateCreatesDistinctServiceCredentialsWithoutBroadJetStreamAPI(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "fixture")
	if err := generate(dest, fixtureACL()); err != nil {
		t.Fatal(err)
	}
	for _, name := range serviceNames {
		contents, err := os.ReadFile(filepath.Join(dest, "creds", name+".creds"))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(contents), "BEGIN NATS USER JWT") || !strings.Contains(string(contents), "BEGIN USER NKEY SEED") {
			t.Fatalf("%s is not a NATS creds file", name)
		}
		claims, err := jwt.DecodeUserClaims(credsJWT(t, string(contents)))
		if err != nil {
			t.Fatalf("decode %s claims: %v", name, err)
		}
		if got, want := claims.Pub.Allow, fixtureACL().Services[name].Publish; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s publish grants = %v, want %v", name, got, want)
		}
		if claims.Resp == nil || claims.Resp.MaxMsgs != 16 {
			t.Fatalf("%s must have bounded request/reply permission", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "creds", "bootstrap.creds")); err != nil {
		t.Fatalf("bootstrap credential missing: %v", err)
	}
	operatorJWT, err := os.ReadFile(filepath.Join(dest, "operator.jwt"))
	if err != nil {
		t.Fatal(err)
	}
	operatorClaims, err := jwt.DecodeOperatorClaims(string(operatorJWT))
	if err != nil {
		t.Fatal(err)
	}
	if accountPublic, err := os.ReadFile(filepath.Join(dest, "account.public")); err != nil || !strings.HasPrefix(strings.TrimSpace(string(accountPublic)), "A") {
		t.Fatalf("fixture account public key missing or invalid: %v", err)
	}
	accountJWT, err := os.ReadFile(filepath.Join(dest, "account.jwt"))
	if err != nil {
		t.Fatal(err)
	}
	accountClaims, err := jwt.DecodeAccountClaims(string(accountJWT))
	if err != nil || !accountClaims.Limits.IsJSEnabled() {
		t.Fatalf("fixture account must explicitly enable JetStream: %v", err)
	}
	if accountClaims.Issuer != operatorClaims.Subject {
		t.Fatal("fixture application account must be operator-signed")
	}
	if accountClaims.Limits.Conn < 2 || accountClaims.Limits.LeafNodeConn < 1 {
		t.Fatal("fixture application account must permit app and leaf connections")
	}
	systemPublic, err := os.ReadFile(filepath.Join(dest, "system-account.public"))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(systemPublic)), "A") {
		t.Fatalf("fixture system account public key missing or invalid: %v", err)
	}
	if strings.TrimSpace(string(systemPublic)) == accountClaims.Subject {
		t.Fatal("fixture system and application accounts must be distinct")
	}
	systemJWT, err := os.ReadFile(filepath.Join(dest, "system-account.jwt"))
	if err != nil {
		t.Fatal(err)
	}
	systemClaims, err := jwt.DecodeAccountClaims(string(systemJWT))
	if err != nil || systemClaims.Limits.IsJSEnabled() {
		t.Fatalf("fixture system account must not enable JetStream: %v", err)
	}
	if systemClaims.Issuer != operatorClaims.Subject {
		t.Fatal("fixture system account must be operator-signed")
	}
	if _, err := os.Stat(filepath.Join(dest, "creds", "system.creds")); !os.IsNotExist(err) {
		t.Fatalf("system account must not have service credentials: %v", err)
	}
	contract, err := os.ReadFile(filepath.Join(dest, "acl-intent.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contract), "$JS.API.>") {
		t.Fatal("fixture must not grant a broad JetStream API wildcard")
	}
	if generated, err := loadACL(filepath.Join(dest, "acl-intent.yaml")); err != nil || len(generated.Services) != len(serviceNames) || len(generated.Bootstrap.Publish) == 0 {
		t.Fatalf("generated seed-free ACL intent must round-trip: %v", err)
	}
	if err := generate(dest, fixtureACL()); err == nil {
		t.Fatal("existing destination must be refused")
	}
}

func TestLoadACLRejectsUnknownAndMultipleDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acl.yaml")
	if err := os.WriteFile(path, []byte("version: 1\npublsih: [bad]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadACL(path); err == nil {
		t.Fatal("unknown ACL fields must fail")
	}
	if err := os.WriteFile(path, []byte("version: 1\n---\nversion: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadACL(path); err == nil {
		t.Fatal("multiple ACL documents must fail")
	}
}

func credsJWT(t *testing.T, creds string) string {
	t.Helper()
	const begin = "-----BEGIN NATS USER JWT-----\n"
	const end = "\n------END NATS USER JWT------"
	start := strings.Index(creds, begin)
	finish := strings.Index(creds, end)
	if start < 0 || finish < start {
		t.Fatal("credential does not contain a user JWT")
	}
	return creds[start+len(begin) : finish]
}

func TestGenerateRejectsWildcardAndBroadJetStreamPermissions(t *testing.T) {
	for _, mutate := range []func(*aclDocument){
		func(acl *aclDocument) {
			acl.Services["auth"] = serviceACL{Publish: []string{"user.>"}, Subscribe: acl.Services["auth"].Subscribe}
		},
		func(acl *aclDocument) {
			acl.Services["auth"] = serviceACL{Publish: acl.Services["auth"].Publish, Subscribe: []string{"$JS.API.>"}}
		},
		func(acl *aclDocument) { delete(acl.Services, "voice") },
	} {
		acl := fixtureACL()
		mutate(&acl)
		if err := generate(filepath.Join(t.TempDir(), "fixture"), acl); err == nil {
			t.Fatal("unsafe or incomplete ACL must be rejected")
		}
	}
}

func TestGeneratePermitsOnlyExactDurableJetStreamAckWildcard(t *testing.T) {
	acl := fixtureACL()
	acl.Services["notification"] = serviceACL{
		Publish:   []string{"$JS.ACK.message_events.notif_msg_v2.>"},
		Subscribe: []string{"_INBOX.voice.notification.message"},
	}
	if err := generate(filepath.Join(t.TempDir(), "fixture"), acl); err != nil {
		t.Fatalf("fixed durable acknowledgement grant must be allowed: %v", err)
	}
	for _, subject := range []string{"$JS.ACK.message_events.>", "$JS.ACK.message_events.notif_msg_v2.extra.>", "$JS.API.CONSUMER.INFO.message_events.>"} {
		acl := fixtureACL()
		acl.Services["notification"] = serviceACL{Publish: []string{subject}}
		if err := generate(filepath.Join(t.TempDir(), "fixture"), acl); err == nil {
			t.Fatalf("unsafe wildcard %q must be rejected", subject)
		}
	}
}

func fixtureACL() aclDocument {
	services := make(map[string]serviceACL, len(serviceNames))
	for _, name := range serviceNames {
		services[name] = serviceACL{Publish: []string{name + ".fixture.published"}, Subscribe: []string{name + ".fixture.received"}}
	}
	return aclDocument{
		Version:   1,
		Services:  services,
		Bootstrap: serviceACL{Publish: []string{"$JS.API.STREAM.INFO.fixture"}, Subscribe: []string{"$JS.API.STREAM.INFO.fixture.response"}},
	}
}
