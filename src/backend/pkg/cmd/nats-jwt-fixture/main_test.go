package main

import (
	"os"
	"path/filepath"
	"slices"
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
	noAck, err := jwt.DecodeUserClaims(credsJWT(t, mustRead(t, filepath.Join(dest, "creds", "chat-noack.creds"))))
	if err != nil || strings.Contains(strings.Join(noAck.Pub.Allow, ","), "$JS.ACK.") {
		t.Fatal("chat-noack credential must omit every ACK permission")
	}
	if noAck.Resp != nil {
		t.Fatal("chat-noack credential must not inherit response permission that could mask ACK denial")
	}
	if got, want := strings.Join(noAck.Sub.Allow, ","), "_INBOX.voice.chat.noack"; got != want {
		t.Fatalf("chat-noack credential subscriptions = %q, want %q", got, want)
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
	limits := accountClaims.Limits
	if limits.Conn < 2 || limits.LeafNodeConn < 1 || limits.Subs < 2 || limits.Data < 1024 || limits.Payload < 1024 {
		t.Fatal("fixture application account must permit bounded app, leaf, and request/reply transport")
	}
	if limits.MemoryStorage <= 0 || limits.DiskStorage <= 0 || limits.Streams <= 0 || limits.Consumer <= 0 || limits.MaxAckPending <= 0 {
		t.Fatal("fixture application account must explicitly bound every required JetStream limit")
	}
	systemPublic, err := os.ReadFile(filepath.Join(dest, "system-account.public"))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(systemPublic)), "A") {
		t.Fatalf("fixture system account public key missing or invalid: %v", err)
	}
	if strings.TrimSpace(string(systemPublic)) == accountClaims.Subject {
		t.Fatal("fixture system and application accounts must be distinct")
	}
	if operatorClaims.SystemAccount != strings.TrimSpace(string(systemPublic)) {
		t.Fatal("fixture operator must declare the distinct system account")
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

func TestGenerateCanOmitResponsePermissionForExplicitAckProof(t *testing.T) {
	acl := fixtureACL()
	acl.Services["chat"] = serviceACL{Publish: []string{"$JS.ACK.chat_events.proof_chat.>"}, Subscribe: []string{"_INBOX.voice.chat.proof"}, NoResponse: true}
	dest := filepath.Join(t.TempDir(), "fixture")
	if err := generate(dest, acl); err != nil {
		t.Fatal(err)
	}
	claims, err := jwt.DecodeUserClaims(credsJWT(t, mustRead(t, filepath.Join(dest, "creds", "chat.creds"))))
	if err != nil || claims.Resp != nil || strings.Join(claims.Pub.Allow, ",") != "$JS.ACK.chat_events.proof_chat.>" {
		t.Fatal("proof chat must rely on its exact ACK grant without response permission")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
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

func TestCanonicalACLHasScopedRuntimeAndBootstrapGrants(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "deploy", "nats", "acl-intent.yaml")
	acl, err := loadACL(path)
	if err != nil {
		t.Fatal(err)
	}
	for service, grant := range acl.Services {
		if !grant.NoResponse {
			t.Errorf("%s may use bounded response permission to bypass exact ACK grant", service)
		}
		for _, subject := range grant.Publish {
			if strings.HasPrefix(subject, "$JS.API.CONSUMER.CREATE.") ||
				strings.HasPrefix(subject, "$JS.API.CONSUMER.DURABLE.CREATE.") ||
				strings.HasPrefix(subject, "$JS.API.CONSUMER.DELETE.") ||
				strings.HasPrefix(subject, "$JS.API.STREAM.CREATE.") ||
				strings.HasPrefix(subject, "$JS.API.STREAM.UPDATE.") ||
				strings.HasPrefix(subject, "$JS.API.STREAM.DELETE.") {
				t.Errorf("%s has JetStream mutation grant %s", service, subject)
			}
		}
		for _, subject := range grant.Subscribe {
			if strings.HasPrefix(subject, "_INBOX.") && strings.HasSuffix(subject, ".>") {
				if subject != "_INBOX.voice."+service+".>" && !(service == "auth" && subject == "_INBOX.voice.auth.requests.>") {
					t.Errorf("%s has cross-service reply inbox %s", service, subject)
				}
			}
		}
	}
	if !acl.Bootstrap.NoResponse {
		t.Error("bootstrap must not inherit response publish permission")
	}
	if !slices.Contains(acl.Services["auth"].Subscribe, "_INBOX.voice.auth.requests.>") || slices.Contains(acl.Services["auth"].Subscribe, "_INBOX.voice.auth.>") {
		t.Error("Auth reply inbox must be scoped to its JNATS requests prefix")
	}
	for service, grants := range map[string][]string{
		"chat":        {"$JS.API.CONSUMER.INFO.message_events.chat_message_activity", "$JS.ACK.message_events.chat_message_activity.>"},
		"messaging":   {"$JS.API.CONSUMER.INFO.message_events.messaging_delivery_ack", "$JS.API.CONSUMER.INFO.user_events.messaging_receipt_privacy", "$JS.ACK.message_events.messaging_delivery_ack.>", "$JS.ACK.user_events.messaging_receipt_privacy.>"},
		"bot":         {"$JS.API.CONSUMER.INFO.message_events.bot_message_events", "$JS.ACK.message_events.bot_message_events.>"},
		"matchmaking": {"$JS.API.CONSUMER.INFO.story_events.matchmaking_story_lfp_v2", "$JS.ACK.story_events.matchmaking_story_lfp_v2.>"},
		"space":       {"$JS.API.CONSUMER.INFO.subscription_events.space_subscription_entitlement", "$JS.ACK.subscription_events.space_subscription_entitlement.>"},
		"user":        {"$JS.API.CONSUMER.INFO.user_events.user-account-deletion-v1", "$JS.API.CONSUMER.MSG.NEXT.user_events.user-account-deletion-v1", "$JS.ACK.user_events.user-account-deletion-v1.>"},
	} {
		for _, subject := range grants {
			if !slices.Contains(acl.Services[service].Publish, subject) {
				t.Errorf("%s missing publish grant %s", service, subject)
			}
		}
	}
	for _, subject := range []string{
		"$JS.API.STREAM.CREATE.message_events",
		"$JS.API.CONSUMER.INFO.message_events.chat_message_activity",
		"$JS.API.CONSUMER.CREATE.message_events.chat_message_activity",
	} {
		if !slices.Contains(acl.Bootstrap.Publish, subject) {
			t.Errorf("bootstrap missing grant %s", subject)
		}
	}
}

func TestCanonicalJWTDoesNotUseResponsePermission(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "deploy", "nats", "acl-intent.yaml")
	acl, err := loadACL(path)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "fixture")
	if err := generate(dest, acl); err != nil {
		t.Fatal(err)
	}
	for _, name := range append(append([]string{}, serviceNames...), "bootstrap") {
		claims, err := jwt.DecodeUserClaims(credsJWT(t, mustRead(t, filepath.Join(dest, "creds", name+".creds"))))
		if err != nil {
			t.Fatal(err)
		}
		if claims.Resp != nil {
			t.Errorf("%s JWT grants response permission", name)
		}
	}
}

func TestGrantValidationRejectsBroadInboxAndManagementWildcard(t *testing.T) {
	for _, subject := range []string{"_INBOX.>", "_INBOX.voice.>", "$JS.API.>", "$JS.API.CONSUMER.>"} {
		if err := validateGrant(serviceACL{Subscribe: []string{subject}}); err == nil {
			t.Errorf("unsafe subscription %s accepted", subject)
		}
	}
	if err := validateGrant(serviceACL{Subscribe: []string{"_INBOX.voice.chat.>"}}); err != nil {
		t.Fatalf("scoped reply inbox should be valid: %v", err)
	}
}
