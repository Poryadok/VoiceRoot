package main

import (
	"bytes"
	"encoding/json"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRetainedSignedGrantChain(t *testing.T) {
	now := time.Now()
	op, _ := nkeys.CreateOperator()
	defer op.Wipe()
	opPublic, _ := op.PublicKey()
	account, _ := nkeys.CreateAccount()
	defer account.Wipe()
	accountPublic, _ := account.PublicKey()
	user, _ := nkeys.CreateUser()
	defer user.Wipe()
	userPublic, _ := user.PublicKey()
	operator := jwt.NewOperatorClaims(opPublic)
	opToken, e := operator.Encode(op)
	if e != nil {
		t.Fatal(e)
	}
	ac := jwt.NewAccountClaims(accountPublic)
	ac.Limits = jwt.OperatorLimits{JetStreamLimits: jwt.JetStreamLimits{MemoryStorage: 1 << 20, DiskStorage: 1 << 20, Streams: 16, Consumer: 16}}
	accountToken, e := ac.Encode(op)
	if e != nil {
		t.Fatal(e)
	}
	uc := jwt.NewUserClaims(userPublic)
	uc.Pub.Allow = []string{"$JS.API.CONSUMER.INFO.chat_events.c1", "$JS.ACK.chat_events.c1.>"}
	uc.Sub.Allow = []string{"_INBOX.story.>"}
	token, e := uc.Encode(account)
	if e != nil {
		t.Fatal(e)
	}
	seed, _ := user.Seed()
	creds := "-----BEGIN NATS USER JWT-----\n" + token + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	acl, _ := yaml.Marshal(retainedPolicy{Version: 1, Services: map[string]retainedPolicyGrant{"story": {Publish: uc.Pub.Allow, Subscribe: uc.Sub.Allow, NoResponse: true}}})
	original := retainedGrantInput{ACL: string(acl), Operator: opToken, Account: accountToken, AccountPublic: accountPublic, Credentials: map[string]string{"story": creds}}
	if os.Getenv("RETAINED_GRANT_TEST_FIXTURE") == "/fixture/signed-input.json" {
		raw, _ := json.Marshal(original)
		if e := os.WriteFile("/fixture/signed-input.json", raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	got, e := verifyRetainedGrants(original, now)
	if e != nil || got.AccountPublic != accountPublic || got.Roles["story"].UserPublic != userPublic || strings.Join(got.Roles["story"].Publish.Deny, ",") != strings.Join(uc.Pub.Deny, ",") {
		t.Fatalf("signed effective grant lost: %v", e)
	}
	input, _ := json.Marshal(original)
	var output bytes.Buffer
	if e := retainedGrants(bytes.NewReader(input), &output); e != nil {
		t.Fatalf("ROOT bounded input mode: %v", e)
	}
	var actual retainedGrantOutput
	if e := json.Unmarshal(output.Bytes(), &actual); e != nil || actual.Roles["story"].CredentialSHA256 != retainedHash(creds) {
		t.Fatal("private input byte binding lost")
	}
	for _, name := range []string{"bad-signature", "wrong-account", "expired", "revoked", "seed-mismatch", "unknown-role", "duplicate-user", "acl-mismatch"} {
		t.Run(name, func(t *testing.T) {
			in := original
			in.Credentials = map[string]string{"story": creds}
			switch name {
			case "bad-signature":
				in.Account = in.Account[:len(in.Account)-1] + "!"
			case "wrong-account":
				in.AccountPublic = opPublic
			case "expired":
				if _, e := verifyRetainedGrants(in, now.Add(24*time.Hour)); e != nil {
					t.Fatal("non-expiring baseline must remain valid")
				}
				uc.Expires = now.Add(5 * time.Minute).Unix()
				x, _ := uc.Encode(account)
				in.Credentials["story"] = strings.Replace(creds, token, x, 1)
				uc.Expires = 0
			case "revoked":
				ac.Revocations = jwt.RevocationList{userPublic: now.Add(time.Hour).Unix()}
				in.Account, _ = ac.Encode(op)
				ac.Revocations = nil
			case "seed-mismatch":
				other, _ := nkeys.CreateUser()
				defer other.Wipe()
				s, _ := other.Seed()
				in.Credentials["story"] = strings.Replace(creds, string(seed), string(s), 1)
			case "unknown-role":
				in.Credentials = map[string]string{"foreign": creds}
			case "duplicate-user":
				in.Credentials["social"] = creds
			case "acl-mismatch":
				in.ACL = strings.Replace(in.ACL, "chat_events.c1", "other_events.c1", 1)
			}
			if _, e := verifyRetainedGrants(in, now); e == nil {
				t.Fatal("untrusted grant admitted")
			}
		})
	}
}
