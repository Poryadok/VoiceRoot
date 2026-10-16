package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

type actorFixture struct {
	op, account, user, signer nkeys.KeyPair
	oc                        *jwt.OperatorClaims
	ac                        *jwt.AccountClaims
	uc                        *jwt.UserClaims
}

func actorHash(b []byte) string          { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func actorPublic(k nkeys.KeyPair) string { p, _ := k.PublicKey(); return p }
func newActorFixture(t *testing.T) *actorFixture {
	t.Helper()
	op, _ := nkeys.CreateOperator()
	a, _ := nkeys.CreateAccount()
	u, _ := nkeys.CreateUser()
	s, _ := nkeys.CreateAccount()
	f := &actorFixture{op: op, account: a, user: u, signer: s}
	t.Cleanup(func() { op.Wipe(); a.Wipe(); u.Wipe(); s.Wipe() })
	f.oc = jwt.NewOperatorClaims(actorPublic(op))
	f.ac = jwt.NewAccountClaims(actorPublic(a))
	f.uc = jwt.NewUserClaims(actorPublic(u))
	f.uc.Pub.Allow = jwt.StringList{"events.chat.*"}
	f.uc.Sub.Allow = jwt.StringList{"_INBOX.actor.>"}
	return f
}
func (f *actorFixture) inputs(t *testing.T) ([]byte, string, string, string, string) {
	t.Helper()
	o, e := f.oc.Encode(f.op)
	if e != nil {
		t.Fatal("synthetic operator encode")
	}
	a, e := f.ac.Encode(f.op)
	if e != nil {
		t.Fatal("synthetic account encode")
	}
	k := f.account
	if f.uc.IssuerAccount != "" {
		k = f.signer
	}
	u, e := f.uc.Encode(k)
	if e != nil {
		t.Fatal("synthetic user encode")
	}
	rawSeed, _ := f.user.Seed()
	seed := append([]byte(nil), rawSeed...)
	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()
	d, e := jwt.DecorateSeed(seed)
	if e != nil {
		t.Fatal("synthetic seed format")
	}
	c := append([]byte("-----BEGIN NATS USER JWT-----\n"+u+"\n------END NATS USER JWT------\n\n"), d...)
	return c, a, o, actorHash([]byte(actorPublic(f.user))), actorHash([]byte(actorPublic(f.account)))
}
func checkActor(t *testing.T, f *actorFixture, want bool) map[string]interface{} {
	t.Helper()
	c, a, o, u, ac := f.inputs(t)
	unchanged := append([]byte(nil), c...)
	b, e := verifyExistingActor(c, a, o, u, ac)
	if string(c) != string(unchanged) {
		t.Fatal("credentials mutated")
	}
	if !want {
		if e == nil || b != nil || e.Error() != "existing_actor_verification_rejected" {
			t.Fatal("expected sanitized rejection")
		}
		return nil
	}
	if e != nil {
		t.Fatal("valid synthetic chain rejected")
	}
	var r map[string]interface{}
	if json.Unmarshal(b, &r) != nil {
		t.Fatal("invalid receipt")
	}
	if r["signed_chain_verified"] != true || r["credential_sha256"] != actorHash(c) || r["actor_sha256"] != u || r["account_sha256"] != ac || r["operator_sha256"] != actorHash([]byte(o)) || r["account_jwt_sha256"] != actorHash([]byte(a)) {
		t.Fatal("receipt binding mismatch")
	}
	return r
}
func TestVerifyActorDirectAndUnscoped(t *testing.T) {
	f := newActorFixture(t)
	r := checkActor(t, f, true)
	e := r["effective"].(map[string]interface{})
	if e["response"] != nil || e["pub"].(map[string]interface{})["allow"].([]interface{})[0] != "events.chat.*" {
		t.Fatal("permission IR mismatch")
	}
	f.ac.SigningKeys.Add(actorPublic(f.signer))
	f.uc.IssuerAccount = actorPublic(f.account)
	checkActor(t, f, true)
}
func TestVerifyActorIdentityAndSignature(t *testing.T) {
	f := newActorFixture(t)
	c, a, o, u, ac := f.inputs(t)
	for _, v := range []struct {
		c           []byte
		a, o, u, ac string
	}{{c, a, o, strings.Repeat("0", 64), ac}, {c, a, o, u, strings.Repeat("0", 64)}, {c, a + "x", o, u, ac}, {c, a, o + "x", u, ac}, {append([]byte("invalid"), c...), a, o, u, ac}} {
		if b, e := verifyExistingActor(v.c, v.a, v.o, v.u, v.ac); e == nil || b != nil {
			t.Fatal("invalid identity/signature accepted")
		}
	}
	other, _ := nkeys.CreateUser()
	defer other.Wipe()
	seed, _ := other.Seed()
	token, _ := jwt.ParseDecoratedJWT(c)
	wrong, _ := jwt.FormatUserConfig(token, seed)
	if _, e := verifyExistingActor(wrong, a, o, u, ac); e == nil {
		t.Fatal("seed identity mismatch accepted")
	}
	f.uc.IssuerAccount = actorPublic(f.account)
	checkActor(t, f, false) // signer not enrolled
	g := newActorFixture(t)
	_, a2, o2, _, _ := g.inputs(t)
	for _, v := range []struct{ a, o string }{{a, o2}, {a2, o}, {a2, o2}} {
		if _, e := verifyExistingActor(c, v.a, v.o, u, ac); e == nil {
			t.Fatal("foreign signed chain accepted")
		}
	}
	changed := actorResign(t, token, f.account, func(m map[string]interface{}) { m["sub"] = actorPublic(other) })
	forged := append([]byte("-----BEGIN NATS USER JWT-----\n"+changed+"\n------END NATS USER JWT------\n\n"), c[strings.Index(string(c), "-----BEGIN USER NKEY SEED-----"):]...)
	if _, e := verifyExistingActor(forged, a, o, u, ac); e == nil {
		t.Fatal("signed foreign identity accepted")
	}
	for _, bad := range [][]byte{append(append([]byte(nil), c...), c...), []byte(strings.Repeat("x", (1<<20)+1))} {
		if _, e := verifyExistingActor(bad, a, o, u, ac); e == nil {
			t.Fatal("ambiguous or oversized creds accepted")
		}
	}
}
func TestVerifyActorFreshnessAndRevocation(t *testing.T) {
	for _, role := range []string{"user", "account", "operator"} {
		t.Run(role, func(t *testing.T) {
			f := newActorFixture(t)
			expiry := time.Now().Add(-time.Hour).Unix()
			switch role {
			case "user":
				f.uc.Expires = expiry
			case "account":
				f.ac.Expires = expiry
			case "operator":
				f.oc.Expires = expiry
			}
			checkActor(t, f, false)
		})
	}
	f := newActorFixture(t)
	f.uc.NotBefore = time.Now().Add(time.Hour).Unix()
	checkActor(t, f, false)
	for _, key := range []string{"user", "all"} {
		f = newActorFixture(t)
		f.ac.Revocations = jwt.RevocationList{}
		k := actorPublic(f.user)
		if key == "all" {
			k = jwt.All
		}
		f.ac.Revocations[k] = time.Now().Add(time.Hour).Unix()
		checkActor(t, f, false)
	}
	f = newActorFixture(t)
	f.ac.Revocations = jwt.RevocationList{actorPublic(f.user): time.Now().Add(-time.Hour).Unix()}
	checkActor(t, f, true)
}
func TestVerifyActorRestrictions(t *testing.T) {
	for _, kind := range []string{"scope", "response", "source", "times", "locale", "connection", "bearer", "defaults", "callout", "audience", "limits", "strict"} {
		t.Run(kind, func(t *testing.T) {
			f := newActorFixture(t)
			switch kind {
			case "scope":
				s := jwt.NewUserScope()
				s.Key = actorPublic(f.signer)
				f.ac.SigningKeys.AddScopedSigner(s)
				f.uc.SetScoped(true)
				f.uc.IssuerAccount = actorPublic(f.account)
			case "response":
				f.uc.Resp = &jwt.ResponsePermission{MaxMsgs: 1}
			case "source":
				f.uc.Src = jwt.CIDRList{"127.0.0.1/32"}
			case "times":
				f.uc.Times = []jwt.TimeRange{{Start: "00:00:00", End: "23:59:59"}}
			case "locale":
				f.uc.Locale = "UTC"
			case "connection":
				f.uc.AllowedConnectionTypes = jwt.StringList{"STANDARD"}
			case "bearer":
				f.uc.BearerToken = true
			case "defaults":
				f.ac.DefaultPermissions.Pub.Allow = jwt.StringList{">"}
			case "callout":
				f.ac.Authorization.AuthUsers = jwt.StringList{actorPublic(f.user)}
			case "audience":
				f.uc.Audience = "server"
			case "limits":
				f.uc.NatsLimits.Payload = 1
			case "strict":
				f.oc.StrictSigningKeyUsage = true
			}
			checkActor(t, f, false)
		})
	}
}

// Sign modified synthetic payloads directly: Encode omits empty lists and resets iat.
func actorResign(t *testing.T, token string, k nkeys.KeyPair, change func(map[string]interface{})) string {
	t.Helper()
	p := strings.Split(token, ".")
	raw, _ := base64.RawURLEncoding.DecodeString(p[1])
	var m map[string]interface{}
	json.Unmarshal(raw, &m)
	change(m)
	raw, _ = json.Marshal(m)
	p[1] = base64.RawURLEncoding.EncodeToString(raw)
	sig, _ := k.Sign([]byte(p[0] + "." + p[1]))
	p[2] = base64.RawURLEncoding.EncodeToString(sig)
	return strings.Join(p, ".")
}
func TestVerifyActorNilEmptyAndUnknown(t *testing.T) {
	f := newActorFixture(t)
	c, a, o, u, ac := f.inputs(t)
	token, _ := jwt.ParseDecoratedJWT(c)
	seed, _ := f.user.Seed()
	for _, empty := range []bool{false, true} {
		changed := actorResign(t, token, f.account, func(m map[string]interface{}) {
			n := m["nats"].(map[string]interface{})
			p := map[string]interface{}{}
			if empty {
				p["allow"] = []interface{}{}
				p["deny"] = []interface{}{}
			}
			n["pub"] = p
			n["sub"] = p
		})
		creds, _ := jwt.FormatUserConfig(changed, seed)
		b, e := verifyExistingActor(creds, a, o, u, ac)
		if e != nil {
			t.Fatal("nil/empty rejected")
		}
		var r map[string]interface{}
		json.Unmarshal(b, &r)
		for _, role := range []string{"pub", "sub"} {
			for _, list := range []string{"allow", "deny"} {
				v := r["effective"].(map[string]interface{})[role].(map[string]interface{})[list]
				if (v == nil) == empty {
					t.Fatal("nil/empty collapsed")
				}
			}
		}
	}
	for _, change := range []func(map[string]interface{}){func(m map[string]interface{}) { m["iat"] = time.Now().Add(time.Hour).Unix() }, func(m map[string]interface{}) { m["nats"].(map[string]interface{})["unmodelled_restriction"] = true }} {
		changed := actorResign(t, token, f.account, change)
		creds, _ := jwt.FormatUserConfig(changed, seed)
		if _, e := verifyExistingActor(creds, a, o, u, ac); e == nil {
			t.Fatal("unsupported/future claim accepted")
		}
	}
}
