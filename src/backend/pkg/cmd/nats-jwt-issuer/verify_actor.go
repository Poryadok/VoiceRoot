package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

var errExistingActor = errors.New("existing_actor_verification_rejected")

type existingPermission struct {
	Allow jwt.StringList `json:"allow"`
	Deny  jwt.StringList `json:"deny"`
}
type existingEffective struct {
	Pub      existingPermission `json:"pub"`
	Sub      existingPermission `json:"sub"`
	Response interface{}        `json:"response"`
}

func existingHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Verify, never issue. The caller owns operator trust, capture freshness, live
// configuration binding and separate broker authentication. JWT decoding checks
// signatures, not whether the supplied operator is an approved trust anchor.

func verifyExistingActor(credentials []byte, accountToken, operatorToken, actorSHA, accountSHA string) ([]byte, error) {
	const maxInput = 1 << 20
	if len(credentials) == 0 || len(credentials) > maxInput || len(accountToken) == 0 || len(accountToken) > maxInput || len(operatorToken) == 0 || len(operatorToken) > maxInput {
		return nil, errExistingActor
	}
	// Require exactly one decorated user JWT and seed; no ambiguous additional
	// identity blocks or raw-token fallback from the library's permissive parser.
	if !bytes.HasPrefix(bytes.TrimSpace(credentials), []byte("-----BEGIN NATS USER JWT-----\n")) && !bytes.HasPrefix(bytes.TrimSpace(credentials), []byte("-----BEGIN NATS USER JWT-----\r\n")) {
		return nil, errExistingActor
	}
	for _, marker := range []string{"BEGIN NATS USER JWT", "END NATS USER JWT", "BEGIN USER NKEY SEED", "END USER NKEY SEED"} {
		if bytes.Count(credentials, []byte(marker)) != 1 {
			return nil, errExistingActor
		}
	}
	if bytes.Count(credentials, []byte("BEGIN ")) != 2 || bytes.Count(credentials, []byte("END ")) != 2 {
		return nil, errExistingActor
	}
	userToken, err := jwt.ParseDecoratedJWT(credentials)
	if err != nil {
		return nil, errExistingActor
	}
	user, err := jwt.DecodeUserClaims(userToken)
	if err != nil {
		return nil, errExistingActor
	}
	account, err := jwt.DecodeAccountClaims(accountToken)
	if err != nil {
		return nil, errExistingActor
	}
	operator, err := jwt.DecodeOperatorClaims(operatorToken)
	if err != nil {
		return nil, errExistingActor
	}
	// Refuse unmodelled signed fields/version rather than silently ignoring them.
	for _, v := range []struct {
		token  string
		claims jwt.Claims
	}{{userToken, user}, {accountToken, account}, {operatorToken, operator}} {
		chunks := strings.Split(v.token, ".")
		payload, e := base64.RawURLEncoding.DecodeString(chunks[1])
		if e != nil {
			return nil, errExistingActor
		}
		d := json.NewDecoder(bytes.NewReader(payload))
		d.DisallowUnknownFields()
		if d.Decode(v.claims) != nil {
			return nil, errExistingActor
		}
		vr := jwt.CreateValidationResults()
		v.claims.Validate(vr)
		c := v.claims.Claims()
		now := time.Now().Unix()
		if vr.IsBlocking(true) || c.IssuedAt <= 0 || c.IssuedAt > now || c.NotBefore < 0 || c.Expires < 0 || (c.Expires != 0 && c.Expires <= now) || c.Audience != "" {
			return nil, errExistingActor
		}
	}
	if user.Version != 2 || account.Version != 2 || operator.Version != 2 || !nkeys.IsValidPublicUserKey(user.Subject) || !nkeys.IsValidPublicAccountKey(account.Subject) || !nkeys.IsValidPublicOperatorKey(operator.Subject) || !operator.IsSelfSigned() || !operator.DidSign(account) || !account.DidSign(user) {
		return nil, errExistingActor
	}
	if user.Issuer == account.Subject {
		if user.IssuerAccount != "" && user.IssuerAccount != account.Subject {
			return nil, errExistingActor
		}
	} else {
		scope, listed := account.SigningKeys.GetScope(user.Issuer)
		if !listed || scope != nil || user.IssuerAccount != account.Subject {
			return nil, errExistingActor
		}
	}
	if account.IsClaimRevoked(user) || existingHash([]byte(user.Subject)) != actorSHA || existingHash([]byte(account.Subject)) != accountSHA {
		return nil, errExistingActor
	}
	if user.Resp != nil || user.BearerToken || len(user.Src) != 0 || len(user.Times) != 0 || user.Locale != "" || len(user.AllowedConnectionTypes) != 0 || !user.NatsLimits.IsUnlimited() || !reflect.DeepEqual(account.DefaultPermissions, jwt.Permissions{}) || !reflect.DeepEqual(account.Authorization, jwt.ExternalAuthorization{}) {
		return nil, errExistingActor
	}
	key, err := jwt.ParseDecoratedNKey(credentials)
	if err != nil {
		return nil, errExistingActor
	}
	defer key.Wipe()
	pub, err := key.PublicKey()
	if err != nil || pub != user.Subject || !nkeys.IsValidPublicUserKey(pub) {
		return nil, errExistingActor
	}
	return json.Marshal(struct {
		Effective  existingEffective `json:"effective"`
		Actor      string            `json:"actor_sha256"`
		Account    string            `json:"account_sha256"`
		Credential string            `json:"credential_sha256"`
		Operator   string            `json:"operator_sha256"`
		AccountJWT string            `json:"account_jwt_sha256"`
		ActorJWT   string            `json:"actor_jwt_sha256"`
		Verified   bool              `json:"signed_chain_verified"`
	}{existingEffective{existingPermission{user.Pub.Allow, user.Pub.Deny}, existingPermission{user.Sub.Allow, user.Sub.Deny}, nil}, actorSHA, accountSHA, existingHash(credentials), existingHash([]byte(operatorToken)), existingHash([]byte(accountToken)), existingHash([]byte(userToken)), true})
}
