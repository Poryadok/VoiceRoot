package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	jwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"reflect"
	"strings"
)

var bootstrapRenewSubjects = []string{"$JS.API.STREAM.UPDATE.chat_events", "$JS.API.STREAM.UPDATE.social_events", "$JS.API.CONSUMER.INFO.social_events.rt_realtime1_friend_removed", "$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed"}

func renewalFailure() error { return errors.New("existing_bootstrap_renewal_refused") }
func renewalSubjectMatch(pattern, subject string) bool {
	a, b := strings.Split(pattern, "."), strings.Split(subject, ".")
	for i, t := range a {
		if t == ">" {
			return i == len(a)-1 && i < len(b)
		}
		if i >= len(b) || (t != "*" && t != b[i]) {
			return false
		}
	}
	return len(a) == len(b)
}

// renewExistingBootstrap never creates keys, changes the account JWT or removes
// denies. The human-root CLI must first validate protected input/source custody.
func renewExistingBootstrap(old []byte, accountToken, operatorToken string, signerSeed []byte, actorHash, accountHash string) ([]byte, error) {
	if len(old) == 0 || len(old) > 262144 || len(accountToken) > 262144 || len(operatorToken) > 262144 || len(signerSeed) > 60 {
		return nil, renewalFailure()
	}
	token, e := jwt.ParseDecoratedJWT(old)
	if e != nil {
		return nil, renewalFailure()
	}
	u, e := jwt.DecodeUserClaims(token)
	if e != nil {
		return nil, renewalFailure()
	}
	a, e := jwt.DecodeAccountClaims(accountToken)
	if e != nil {
		return nil, renewalFailure()
	}
	o, e := jwt.DecodeOperatorClaims(operatorToken)
	if e != nil {
		return nil, renewalFailure()
	}
	for _, claim := range []jwt.Claims{u, a, o} {
		vr := jwt.CreateValidationResults()
		claim.Validate(vr)
		if vr.IsBlocking(true) {
			return nil, renewalFailure()
		}
	}
	if !o.IsSelfSigned() || !o.DidSign(a) || !a.DidSign(u) || a.IsClaimRevoked(u) {
		return nil, renewalFailure()
	}
	hash := func(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
	if hash(u.Subject) != actorHash || hash(a.Subject) != accountHash {
		return nil, renewalFailure()
	}
	if scope, ok := a.SigningKeys.GetScope(u.Issuer); ok && scope != nil {
		return nil, renewalFailure()
	}
	signer, e := nkeys.FromSeed([]byte(strings.TrimSpace(string(signerSeed))))
	if e != nil {
		return nil, renewalFailure()
	}
	defer signer.Wipe()
	sp, e := signer.PublicKey()
	if e != nil || sp != u.Issuer || !nkeys.IsValidPublicAccountKey(sp) {
		return nil, renewalFailure()
	}
	user, e := jwt.ParseDecoratedUserNKey(old)
	if e != nil {
		return nil, renewalFailure()
	}
	defer user.Wipe()
	up, e := user.PublicKey()
	if e != nil || up != u.Subject {
		return nil, renewalFailure()
	}
	if len(u.Pub.Allow) == 0 {
		return nil, renewalFailure()
	}
	for _, subject := range bootstrapRenewSubjects {
		for _, deny := range u.Pub.Deny {
			if renewalSubjectMatch(deny, subject) {
				return nil, renewalFailure()
			}
		}
		found := false
		for _, allow := range u.Pub.Allow {
			if allow == subject {
				found = true
			}
		}
		if !found {
			u.Pub.Allow = append(u.Pub.Allow, subject)
		}
	}
	// Encode changes only issuance ID/time/signature; expiration, response policy,
	// connection limits and every other existing claim are retained.
	renewed, e := u.Encode(signer)
	if e != nil {
		return nil, renewalFailure()
	}
	if !renewalOnlyPubDelta(token, renewed) {
		return nil, renewalFailure()
	}
	seed, e := user.Seed()
	if e != nil {
		return nil, renewalFailure()
	}
	result, e := jwt.FormatUserConfig(renewed, seed)
	for i := range seed {
		seed[i] = 0
	}
	if e != nil {
		return nil, renewalFailure()
	}
	return result, nil
}

func renewalOnlyPubDelta(oldToken, newToken string) bool {
	decode := func(token string) map[string]any {
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			return nil
		}
		b, e := base64.RawURLEncoding.DecodeString(parts[1])
		if e != nil {
			return nil
		}
		var row map[string]any
		if json.Unmarshal(b, &row) != nil {
			return nil
		}
		return row
	}
	before, after := decode(oldToken), decode(newToken)
	if before == nil || after == nil {
		return false
	}
	for _, row := range []map[string]any{before, after} {
		delete(row, "iat")
		delete(row, "jti")
	}
	bn, bok := before["nats"].(map[string]any)
	an, aok := after["nats"].(map[string]any)
	if !bok || !aok {
		return false
	}
	bp, bok := bn["pub"].(map[string]any)
	ap, aok := an["pub"].(map[string]any)
	if !bok || !aok {
		return false
	}
	ap["allow"] = bp["allow"]
	return reflect.DeepEqual(before, after)
}
