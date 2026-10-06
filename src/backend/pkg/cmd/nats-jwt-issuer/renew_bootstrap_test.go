package main

import (
	"crypto/sha256"
	"fmt"
	jwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"reflect"
	"testing"
	"time"
)

func pubHash(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
func renewalFixture(t *testing.T) ([]byte, string, string, []byte, string, string, *jwt.UserClaims, nkeys.KeyPair, nkeys.KeyPair) {
	t.Helper()
	op, _ := nkeys.CreateOperator()
	ap, _ := nkeys.CreateAccount()
	uk, _ := nkeys.CreateUser()
	opPub, _ := op.PublicKey()
	apPub, _ := ap.PublicKey()
	up, _ := uk.PublicKey()
	o := jwt.NewOperatorClaims(opPub)
	ot, _ := o.Encode(op)
	a := jwt.NewAccountClaims(apPub)
	at, _ := a.Encode(op)
	u := jwt.NewUserClaims(up)
	u.Name = "existing-bootstrap"
	u.Pub.Allow = []string{"$JS.API.STREAM.INFO.chat_events"}
	u.Sub.Allow = []string{"_INBOX.voice.bootstrap.reply.>"}
	u.Expires = time.Now().Add(time.Hour).Unix()
	u.Limits.NatsLimits.Payload = 65536
	token, _ := u.Encode(ap)
	seed, _ := uk.Seed()
	creds, _ := jwt.FormatUserConfig(token, seed)
	signer, _ := ap.Seed()
	return creds, at, ot, signer, pubHash(up), pubHash(apPub), u, ap, op
}
func TestRenewBootstrapPreservesIdentitySeedAndAllOtherClaims(t *testing.T) {
	old, at, ot, signer, actor, account, _, _, _ := renewalFixture(t)
	out, e := renewExistingBootstrap(old, at, ot, signer, actor, account)
	if e != nil {
		t.Fatal(e)
	}
	oldToken, _ := jwt.ParseDecoratedJWT(old)
	newToken, _ := jwt.ParseDecoratedJWT(out)
	before, _ := jwt.DecodeUserClaims(oldToken)
	after, _ := jwt.DecodeUserClaims(newToken)
	if len(after.Pub.Allow) != len(before.Pub.Allow)+4 {
		t.Fatal("exact permission delta missing")
	}
	after.Pub = before.Pub
	after.ID = before.ID
	after.IssuedAt = before.IssuedAt
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated claims changed")
	}
	b, _ := jwt.ParseDecoratedUserNKey(old)
	n, _ := jwt.ParseDecoratedUserNKey(out)
	bs, _ := b.Seed()
	ns, _ := n.Seed()
	if string(bs) != string(ns) {
		t.Fatal("user seed changed")
	}
}
func TestRenewBootstrapWrongSignerRefused(t *testing.T) {
	old, at, ot, _, actor, account, _, _, _ := renewalFixture(t)
	other, _ := nkeys.CreateAccount()
	seed, _ := other.Seed()
	if _, e := renewExistingBootstrap(old, at, ot, seed, actor, account); e == nil {
		t.Fatal("wrong signer accepted")
	}
}
func TestRenewBootstrapRevokedActorRefused(t *testing.T) {
	old, at, ot, signer, actor, account, u, _, op := renewalFixture(t)
	a, _ := jwt.DecodeAccountClaims(at)
	a.RevokeAt(u.Subject, time.Now().Add(time.Minute))
	at, _ = a.Encode(op)
	if _, e := renewExistingBootstrap(old, at, ot, signer, actor, account); e == nil {
		t.Fatal("revoked actor resurrected")
	}
}
func TestRenewBootstrapExpiredActorRefused(t *testing.T) {
	old, at, ot, signer, actor, account, u, ap, _ := renewalFixture(t)
	u.Expires = time.Now().Add(-time.Minute).Unix()
	token, _ := u.Encode(ap)
	key, _ := jwt.ParseDecoratedUserNKey(old)
	seed, _ := key.Seed()
	old, _ = jwt.FormatUserConfig(token, seed)
	if _, e := renewExistingBootstrap(old, at, ot, signer, actor, account); e == nil {
		t.Fatal("expired actor resurrected")
	}
}
func TestRenewBootstrapDenyPreservedAsVeto(t *testing.T) {
	old, at, ot, signer, actor, account, u, ap, _ := renewalFixture(t)
	u.Pub.Deny = []string{"$JS.API.STREAM.UPDATE.*"}
	token, _ := u.Encode(ap)
	key, _ := jwt.ParseDecoratedUserNKey(old)
	seed, _ := key.Seed()
	old, _ = jwt.FormatUserConfig(token, seed)
	if _, e := renewExistingBootstrap(old, at, ot, signer, actor, account); e == nil {
		t.Fatal("deny bypassed")
	}
}

func TestRenewBootstrapUnrestrictedOldActorIsNotNarrowed(t *testing.T) {
	old, at, ot, signer, actor, account, u, ap, _ := renewalFixture(t)
	u.Pub.Allow = nil
	token, _ := u.Encode(ap)
	key, _ := jwt.ParseDecoratedUserNKey(old)
	seed, _ := key.Seed()
	old, _ = jwt.FormatUserConfig(token, seed)
	if _, e := renewExistingBootstrap(old, at, ot, signer, actor, account); e == nil {
		t.Fatal("unrestricted existing permissions narrowed implicitly")
	}
}
func TestRenewBootstrapScopedSignerCannotBeBypassed(t *testing.T) {
	old, at, ot, _, actor, account, u, _, op := renewalFixture(t)
	a, _ := jwt.DecodeAccountClaims(at)
	sk, _ := nkeys.CreateAccount()
	sp, _ := sk.PublicKey()
	scope := jwt.NewUserScope()
	scope.Key = sp
	scope.Template.Pub.Allow = []string{"$JS.API.STREAM.INFO.chat_events"}
	scope.Template.Sub.Allow = []string{"_INBOX.voice.bootstrap.reply.>"}
	a.SigningKeys.AddScopedSigner(scope)
	at, _ = a.Encode(op)
	u.SetScoped(true)
	u.IssuerAccount = a.Subject
	token, _ := u.Encode(sk)
	key, _ := jwt.ParseDecoratedUserNKey(old)
	seed, _ := key.Seed()
	old, _ = jwt.FormatUserConfig(token, seed)
	ss, _ := sk.Seed()
	if _, e := renewExistingBootstrap(old, at, ot, ss, actor, account); e == nil {
		t.Fatal("scoped signer authority bypassed")
	}
}
