package main

// This verifier uses the same direct operator/account/user trust chain as
// nats-populated-proof/input.go. Unsupported signing scopes/routes fail closed.
// ROOT supplies captured opaque tokens; no user request can supply this input.
import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

type retainedGrantInput struct {
	ACL           string            `json:"acl"`
	Operator      string            `json:"operator"`
	Account       string            `json:"account"`
	AccountPublic string            `json:"account_public"`
	Credentials   map[string]string `json:"credentials"`
}
type retainedGrant struct {
	CredentialSHA256 string         `json:"credential_sha256"`
	TokenSHA256      string         `json:"token_sha256"`
	UserPublic       string         `json:"user_public"`
	Publish          jwt.Permission `json:"publish"`
	Subscribe        jwt.Permission `json:"subscribe"`
	Expires          int64          `json:"expires"`
}
type retainedGrantOutput struct {
	ACLSHA256      string                   `json:"acl_sha256"`
	Schema         string                   `json:"schema"`
	OperatorSHA256 string                   `json:"operator_sha256"`
	AccountSHA256  string                   `json:"account_sha256"`
	AccountPublic  string                   `json:"account_public"`
	Roles          map[string]retainedGrant `json:"roles"`
}
type retainedPolicyGrant struct {
	Publish    []string `yaml:"publish"`
	Subscribe  []string `yaml:"subscribe"`
	NoResponse bool     `yaml:"no_response"`
}
type retainedPolicy struct {
	Version   int                            `yaml:"version"`
	Services  map[string]retainedPolicyGrant `yaml:"services"`
	Bootstrap retainedPolicyGrant            `yaml:"bootstrap"`
}

func retainedSet(a, b []string) bool {
	a = sortedRoles(a)
	b = sortedRoles(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func retainedHash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func retainedSignature(token, key string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[2])
	if e != nil {
		return false
	}
	public, e := nkeys.FromPublicKey(key)
	return e == nil && public.Verify([]byte(parts[0]+"."+parts[1]), sig) == nil
}
func retainedValid(c interface{ Validate(*jwt.ValidationResults) }) bool {
	result := jwt.CreateValidationResults()
	c.Validate(result)
	return !result.IsBlocking(true)
}
func verifyRetainedGrants(in retainedGrantInput, now time.Time) (retainedGrantOutput, error) {
	fail := func() (retainedGrantOutput, error) {
		return retainedGrantOutput{}, fixedError("retained_grant_trust_invalid")
	}
	var policy retainedPolicy
	decoder := yaml.NewDecoder(strings.NewReader(in.ACL))
	decoder.KnownFields(true)
	if len(in.ACL) > 256<<10 || decoder.Decode(&policy) != nil || decoder.Decode(new(any)) != io.EOF || policy.Version != 1 {
		return fail()
	}
	op, e := jwt.DecodeOperatorClaims(in.Operator)
	if e != nil || !retainedValid(op) || !retainedSignature(in.Operator, op.Subject) {
		return fail()
	}
	account, e := jwt.DecodeAccountClaims(in.Account)
	if e != nil || !retainedValid(account) || account.Issuer != op.Subject || account.Subject != in.AccountPublic ||
		!retainedSignature(in.Account, op.Subject) || !account.Limits.IsJSEnabled() ||
		len(account.Imports) > 0 || len(account.Exports) > 0 || len(account.Mappings) > 0 || account.HasExternalAuthorization() {
		return fail()
	}
	if len(in.Credentials) == 0 || len(in.Credentials) > 20 {
		return fail()
	}
	out := retainedGrantOutput{ACLSHA256: retainedHash(in.ACL), Schema: "voice-signed-retained-grants-v1", OperatorSHA256: retainedHash(in.Operator), AccountSHA256: retainedHash(in.Account), AccountPublic: account.Subject, Roles: map[string]retainedGrant{}}
	allowed := strings.Fields("analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice bootstrap")
	users := map[string]bool{}
	for role, credential := range in.Credentials {
		rs := sortedRoles(allowed)
		i := sort.SearchStrings(rs, role)
		if i >= len(rs) || rs[i] != role || len(credential) > 65536 {
			return fail()
		}
		token, e := nkeys.ParseDecoratedJWT([]byte(credential))
		if e != nil {
			return fail()
		}
		user, e := jwt.DecodeUserClaims(token)
		if e != nil || !retainedValid(user) || user.Issuer != account.Subject || !retainedSignature(token, account.Subject) || users[user.Subject] ||
			user.IssuedAt > now.Unix()+30 || user.NotBefore > now.Unix() || (user.Expires != 0 && user.Expires < now.Add(10*time.Minute).Unix()) ||
			user.Resp != nil || user.BearerToken || user.ProxyRequired {
			return fail()
		}
		for who, at := range account.Revocations {
			if (who == user.Subject || who == "*") && user.IssuedAt <= at {
				return fail()
			}
		}
		grant, ok := policy.Services[role]
		if !ok || !grant.NoResponse || !retainedSet(user.Pub.Allow, grant.Publish) || !retainedSet(user.Sub.Allow, grant.Subscribe) {
			return fail()
		}
		pubDeny, subDeny := []string(nil), []string(nil)
		if len(grant.Publish) == 0 {
			pubDeny = []string{">"}
		}
		if len(grant.Subscribe) == 0 {
			subDeny = []string{">"}
		}
		if !retainedSet(user.Pub.Deny, pubDeny) || !retainedSet(user.Sub.Deny, subDeny) {
			return fail()
		}
		key, e := nkeys.ParseDecoratedNKey([]byte(credential))
		if e != nil {
			return fail()
		}
		public, e := key.PublicKey()
		key.Wipe()
		if e != nil || public != user.Subject {
			return fail()
		}
		users[user.Subject] = true
		out.Roles[role] = retainedGrant{retainedHash(credential), retainedHash(token), user.Subject, user.Pub, user.Sub, user.Expires}
	}
	return out, nil
}
func sortedRoles(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
func retainedGrants(reader io.Reader, writer io.Writer) error {
	// Pure bounded token verification: no connection, file read, signer or seed
	// output. Like CLOSED state decoding, only the ROOT current-helper consumer
	// invokes this mode; it does not need a broker network namespace.
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return fixedError("retained_grant_caller_invalid")
	}
	d := json.NewDecoder(io.LimitReader(reader, 2<<20))
	d.DisallowUnknownFields()
	var in retainedGrantInput
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
		return fixedError("retained_grant_input_invalid")
	}
	out, e := verifyRetainedGrants(in, time.Now())
	if e != nil {
		return e
	}
	return json.NewEncoder(writer).Encode(out)
}
