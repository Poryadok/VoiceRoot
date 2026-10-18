package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

func serviceRenewalFixture(t *testing.T, role string) ([]byte, string, string, []byte, string, string, grant, *jwt.UserClaims, nkeys.KeyPair) {
	t.Helper()
	old, at, ot, signer, actor, account, u, ap, _ := renewalFixture(t)
	acl, err := readPolicy("../../../../../deploy/nats/acl-intent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	g := acl.Services[role]
	u.Name = "voice-" + role
	u.NatsLimits = jwt.NewUserClaims(u.Subject).NatsLimits
	u.Pub.Allow = nil
	for _, subject := range g.Publish {
		if !containsServiceSubject(serviceRenewSubjects[role], subject) {
			u.Pub.Allow = append(u.Pub.Allow, subject)
		}
	}
	u.Sub.Allow = append([]string(nil), g.Subscribe...)
	u.Pub.Deny = []string{"unrelated.denied"}
	u.Sub.Deny = []string{"unrelated.denied"}
	return serviceReencode(t, old, u, ap), at, ot, signer, actor, account, g, u, ap
}

func serviceReencode(t *testing.T, old []byte, u *jwt.UserClaims, signer nkeys.KeyPair) []byte {
	t.Helper()
	k, err := jwt.ParseDecoratedUserNKey(old)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Wipe()
	seed, _ := k.Seed()
	token, err := u.Encode(signer)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jwt.FormatUserConfig(token, seed)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRenewServicePreservesClaimsSeedAndExactGrant(t *testing.T) {
	for _, role := range []string{"role", "space", "voice", "messaging"} {
		t.Run(role, func(t *testing.T) {
			old, at, ot, signer, actor, account, g, _, _ := serviceRenewalFixture(t, role)
			out, err := renewExistingService(old, at, ot, signer, actor, account, role, g)
			if err != nil {
				t.Fatal(err)
			}
			bt, _ := jwt.ParseDecoratedJWT(old)
			nt, _ := jwt.ParseDecoratedJWT(out)
			before, _ := jwt.DecodeUserClaims(bt)
			after, _ := jwt.DecodeUserClaims(nt)
			if !sameServiceSubjects(after.Pub.Allow, g.Publish) {
				t.Fatal("canonical publish grant not reached")
			}
			after.Pub.Allow = before.Pub.Allow
			after.ID, after.IssuedAt = before.ID, before.IssuedAt
			if !reflect.DeepEqual(before, after) {
				t.Fatal("unrelated signed claims changed")
			}
			bk, _ := jwt.ParseDecoratedUserNKey(old)
			nk, _ := jwt.ParseDecoratedUserNKey(out)
			bs, _ := bk.Seed()
			ns, _ := nk.Seed()
			if !bytes.Equal(bs, ns) {
				t.Fatal("seed changed")
			}
			again, err := renewExistingService(out, at, ot, signer, actor, account, role, g)
			if err != nil || !bytes.Equal(again, out) {
				t.Fatal("already current actor was reissued")
			}
		})
	}
}

func TestRenewVoiceParticipantGrantPreservesDenyVeto(t *testing.T) {
	for _, deny := range []string{"voice.participant_joined", "voice.participant_left", "voice.roster_membership_changed", "voice.*", "voice.>"} {
		old, at, ot, signer, actor, account, g, u, ap := serviceRenewalFixture(t, "voice")
		u.Pub.Deny = []string{deny}
		old = serviceReencode(t, old, u, ap)
		if output, err := renewExistingService(old, at, ot, signer, actor, account, "voice", g); err == nil || len(output) != 0 {
			t.Fatalf("participant deny %q bypassed", deny)
		}
	}
}

func TestMusicRenewalAddsOnlyInfoAndAckAndRetainsExistingInbox(t *testing.T) {
	old, at, ot, signer, actor, account, g, u, ap := serviceRenewalFixture(t, "messaging")
	u.Sub.Allow = []string{"_INBOX.voice.messaging.>"}
	old = serviceReencode(t, old, u, ap)
	out, err := renewExistingService(old, at, ot, signer, actor, account, "messaging", g)
	if err != nil {
		t.Fatal(err)
	}
	oldToken, _ := jwt.ParseDecoratedJWT(old)
	newToken, _ := jwt.ParseDecoratedJWT(out)
	before, _ := jwt.DecodeUserClaims(oldToken)
	after, _ := jwt.DecodeUserClaims(newToken)
	var delta []string
	for _, subject := range after.Pub.Allow {
		if !containsServiceSubject(before.Pub.Allow, subject) {
			delta = append(delta, subject)
		}
	}
	if !sameServiceSubjects(delta, []string{"$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata", "$JS.ACK.file_events.messaging_music_metadata.>"}) || !reflect.DeepEqual(before.Sub, after.Sub) {
		t.Fatal("Music renewal changed consumer ownership or subscription authority")
	}
	for _, deny := range []string{"$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata", "$JS.ACK.file_events.messaging_music_metadata.1.2.3"} {
		claims := *before
		claims.Pub.Deny = []string{deny}
		invalid := serviceReencode(t, old, &claims, ap)
		if result, err := renewExistingService(invalid, at, ot, signer, actor, account, "messaging", g); err == nil || len(result) != 0 {
			t.Fatal("Music deny bypassed")
		}
	}
}

func TestRenewServiceRejectsAuthorityChanges(t *testing.T) {
	for _, scenario := range []string{"role", "name", "signer", "actor", "account", "extra", "missing", "subscription", "response", "deny_exact_ack", "deny_wildcard", "duplicate", "limits"} {
		t.Run(scenario, func(t *testing.T) {
			old, at, ot, signer, actor, account, g, u, ap := serviceRenewalFixture(t, "voice")
			role := "voice"
			switch scenario {
			case "role":
				role = "gateway"
			case "name":
				u.Name = "voice-role"
			case "signer":
				key, _ := nkeys.CreateAccount()
				signer, _ = key.Seed()
			case "actor":
				actor = account
			case "account":
				account = actor
			case "extra":
				g.Publish = append(g.Publish, "$JS.API.>")
			case "missing":
				g.Publish = g.Publish[1:]
			case "subscription":
				g.Subscribe = append(g.Subscribe, "other.events")
			case "response":
				g.NoResponse = false
			case "deny_exact_ack":
				u.Pub.Deny = []string{"$JS.ACK.chat_events.voice_space_media_chat.1.2.3"}
			case "deny_wildcard":
				u.Pub.Deny = []string{"$JS.ACK.*.>"}
			case "duplicate":
				g.Publish = append(g.Publish, g.Publish[0])
			case "limits":
				u.NatsLimits.Payload = 65536
			}
			old = serviceReencode(t, old, u, ap)
			if out, err := renewExistingService(old, at, ot, signer, actor, account, role, g); err == nil || len(out) != 0 {
				t.Fatal("invalid authority emitted credentials")
			}
		})
	}
}

func TestRenewServicePreservesWildcardCoveredSubscriptions(t *testing.T) {
	old, at, ot, signer, actor, account, g, u, ap := serviceRenewalFixture(t, "voice")
	u.Sub.Allow = []string{"_INBOX.voice.voice.>"}
	old = serviceReencode(t, old, u, ap)
	out, err := renewExistingService(old, at, ot, signer, actor, account, "voice", g)
	if err != nil {
		t.Fatal("existing wildcard already covers source inboxes:", err)
	}
	beforeToken, _ := jwt.ParseDecoratedJWT(old)
	afterToken, _ := jwt.ParseDecoratedJWT(out)
	before, _ := jwt.DecodeUserClaims(beforeToken)
	after, _ := jwt.DecodeUserClaims(afterToken)
	if !sameServiceSubjects(after.Pub.Allow, g.Publish) {
		t.Fatal("canonical publish grant not reached")
	}
	after.Pub.Allow = before.Pub.Allow
	after.ID, after.IssuedAt = before.ID, before.IssuedAt
	if !reflect.DeepEqual(before, after) {
		t.Fatal("covered source inboxes changed existing signed claims")
	}
	for _, scenario := range []string{"missing_wildcard", "unrelated_allow", "denied_inbox"} {
		t.Run(scenario, func(t *testing.T) {
			claims := *before
			switch scenario {
			case "missing_wildcard":
				claims.Sub.Allow = []string{"_INBOX.voice.voice.space_media_chat", "_INBOX.voice.voice.space_media_role"}
			case "unrelated_allow":
				claims.Sub.Allow = append(append([]string(nil), before.Sub.Allow...), "other.events")
			case "denied_inbox":
				claims.Sub.Deny = []string{"_INBOX.voice.voice.space_media_chat"}
			}
			invalid := serviceReencode(t, old, &claims, ap)
			if result, err := renewExistingService(invalid, at, ot, signer, actor, account, "voice", g); err == nil || len(result) != 0 {
				t.Fatal("incompatible subscription authority emitted credentials")
			}
		})
	}
}

func TestRenewServicePrivatePipe(t *testing.T) {
	old, at, ot, signer, actor, account, g, _, _ := serviceRenewalFixture(t, "role")
	request := map[string]any{"schema": "voice-nats-existing-service-renewal-v1", "old_creds": string(old), "account_jwt": at, "operator_jwt": ot, "signer_seed": string(signer), "actor_sha256": actor, "account_sha256": account, "service": "role", "publish": g.Publish, "subscribe": g.Subscribe, "no_response": true}
	raw, _ := json.Marshal(request)
	var output bytes.Buffer
	if err := renewServicePipe(bytes.NewReader(raw), &output); err != nil || output.Len() == 0 {
		t.Fatal("valid private renewal rejected")
	}
	request["extra_grant"] = ">"
	raw, _ = json.Marshal(request)
	for _, invalid := range [][]byte{raw, []byte(`{} {}`), bytes.Repeat([]byte(" "), 1048577)} {
		output.Reset()
		if err := renewServicePipe(bytes.NewReader(invalid), &output); err == nil || output.Len() != 0 {
			t.Fatal("invalid request emitted credentials")
		}
	}
}
