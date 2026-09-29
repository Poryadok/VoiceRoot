// nats-proof-credential issues one offline, short-lived APP user credential for
// the bounded staging JetStream proof. It never prints a JWT or NKey seed.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

const maximumTTL = 2 * time.Hour

var generationPattern = regexp.MustCompile(`^r[0-9]{8}[a-z0-9]{0,8}$`)
var credentialPattern = regexp.MustCompile(`(?s)\A-----BEGIN NATS USER JWT-----\n([A-Za-z0-9_\-.]+)\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n([A-Z0-9]+)\n------END USER NKEY SEED------\n?\z`)

var exactPublish = []string{
	"social.friend_request",
	"$JS.API.STREAM.INFO.social_events",
	"$JS.API.STREAM.MSG.GET.social_events",
	"$JS.API.STREAM.MSG.DELETE.social_events",
}
var exactSubscribe = []string{"_INBOX.voice.nats-proof.>"}

var secretKeys = map[string][]string{
	"voice-nats-operator":              {"operator.jwt", "account.jwt", "system-account.jwt", "account.public", "system-account.public"},
	"voice-nats-hub-tls":               {"tls.crt", "tls.key", "ca.crt"},
	"voice-nats-bootstrap-credentials": {"bootstrap.creds"},
	"voice-nats-service-credentials":   {"analytics.creds", "auth.creds", "bot.creds", "chat.creds", "file.creds", "gateway.creds", "matchmaking.creds", "messaging.creds", "moderation.creds", "notification.creds", "realtime.creds", "role.creds", "search.creds", "social.creds", "space.creds", "story.creds", "subscription.creds", "user.creds", "voice.creds"},
}

type proofOptions struct {
	Namespace, Generation                      string
	AccountSeed, AccountPublic, Bundle, Output string
	Credential                                 string
	TTL, MinValidity                           time.Duration
}

type secretList struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Items      []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Type       string `json:"type"`
		Metadata   struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Immutable *bool             `json:"immutable"`
		Data      map[string]string `json:"data"`
	} `json:"items"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	flags := flag.NewFlagSet("nats-proof-credential", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	namespace := flags.String("namespace", "", "staging namespace")
	generation := flags.String("generation", "", "active NATS generation")
	seed := flags.String("account-seed", "", "protected APP account seed")
	bundle := flags.String("bundle", "", "protected fixed or versioned Secret List")
	public := flags.String("account-public", "", "APP account public key instead of bundle")
	output := flags.String("output", "", "fresh protected credential file")
	credential := flags.String("credential", "", "protected credential for read-only validation")
	ttl := flags.Duration("ttl", time.Hour, "credential lifetime, at most 2h")
	minimum := flags.Duration("check-min-validity", 0, "read-only preflight minimum remaining validity")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "NATS proof credential arguments are invalid")
		return 2
	}
	opts := proofOptions{Namespace: *namespace, Generation: *generation, AccountSeed: *seed, AccountPublic: *public, Bundle: *bundle, Output: *output, Credential: *credential, TTL: *ttl, MinValidity: *minimum}
	if *credential != "" || *minimum != 0 {
		if *credential == "" || *minimum <= 0 || *seed != "" || *public != "" || *output != "" {
			fmt.Fprintln(stderr, "NATS proof credential preflight arguments are invalid")
			return 2
		}
		if err := checkProofCredential(opts, now); err != nil {
			fmt.Fprintln(stderr, "NATS proof credential preflight failed; no values printed")
			return 1
		}
		fmt.Fprintln(stdout, "proof_credential_valid=true")
		return 0
	}
	expiry, err := issueProofCredential(opts, now)
	if err != nil {
		fmt.Fprintln(stderr, "NATS proof credential issuance failed; no values printed")
		return 1
	}
	fmt.Fprintf(stdout, "expires_at=%s\n", expiry.UTC().Format(time.RFC3339))
	return 0
}

func validateTarget(opts proofOptions) error {
	if opts.Namespace != "voice-staging" || opts.Generation != "legacy" && !generationPattern.MatchString(opts.Generation) {
		return errors.New("invalid staging generation")
	}
	return nil
}

func protectedRead(path string, maxBytes int64) ([]byte, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("protected path is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		return nil, errors.New("protected input is unavailable")
	}
	if runtime.GOOS == "linux" && info.Mode().Perm() != 0600 && info.Mode().Perm() != 0400 {
		return nil, errors.New("protected input mode is invalid")
	}
	return os.ReadFile(path)
}

func bundlePublic(opts proofOptions) (string, error) {
	contents, err := protectedRead(opts.Bundle, 1<<20)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	var bundle secretList
	if decoder.Decode(&bundle) != nil || decoder.Decode(new(any)) != io.EOF || bundle.APIVersion != "v1" || bundle.Kind != "List" || len(bundle.Items) != len(secretKeys) {
		return "", errors.New("NATS Secret List is invalid")
	}
	seen := make(map[string]bool, len(secretKeys))
	accountPublic := ""
	for _, item := range bundle.Items {
		base := item.Metadata.Name
		if opts.Generation != "legacy" {
			base = strings.TrimSuffix(base, "-"+opts.Generation)
		}
		wantKeys, known := secretKeys[base]
		wantName := base
		if opts.Generation != "legacy" {
			wantName += "-" + opts.Generation
		}
		if !known || seen[base] || item.Metadata.Name != wantName || item.Metadata.Namespace != opts.Namespace || item.APIVersion != "v1" || item.Kind != "Secret" || item.Type != "Opaque" {
			return "", errors.New("NATS Secret identity is invalid")
		}
		if opts.Generation != "legacy" && (item.Immutable == nil || !*item.Immutable) || opts.Generation == "legacy" && item.Immutable != nil && *item.Immutable {
			return "", errors.New("NATS Secret immutability is invalid")
		}
		if len(item.Data) != len(wantKeys) {
			return "", errors.New("NATS Secret data keys are invalid")
		}
		for _, key := range wantKeys {
			encoded, ok := item.Data[key]
			if !ok || encoded == "" {
				return "", errors.New("NATS Secret data keys are invalid")
			}
			decoded, decodeErr := base64.StdEncoding.Strict().DecodeString(encoded)
			if decodeErr != nil || len(decoded) == 0 || base64.StdEncoding.EncodeToString(decoded) != encoded {
				return "", errors.New("NATS Secret data is invalid")
			}
			if base == "voice-nats-operator" && key == "account.public" {
				accountPublic = string(decoded)
			}
		}
		seen[base] = true
	}
	if len(seen) != len(secretKeys) || !nkeys.IsValidPublicAccountKey(accountPublic) {
		return "", errors.New("APP account public key is invalid")
	}
	return accountPublic, nil
}

func accountPublic(opts proofOptions, requireBundle bool) (string, error) {
	if requireBundle || opts.Bundle != "" {
		if opts.Bundle == "" || opts.AccountPublic != "" {
			return "", errors.New("APP account source is ambiguous")
		}
		return bundlePublic(opts)
	}
	if !nkeys.IsValidPublicAccountKey(opts.AccountPublic) {
		return "", errors.New("APP account public key is invalid")
	}
	return opts.AccountPublic, nil
}

func issueProofCredential(opts proofOptions, now time.Time) (expiry time.Time, resultErr error) {
	if err := validateTarget(opts); err != nil {
		return time.Time{}, err
	}
	if opts.TTL < time.Second || opts.TTL > maximumTTL || opts.Output == "" || opts.AccountSeed == "" || opts.Credential != "" || opts.MinValidity != 0 {
		return time.Time{}, errors.New("proof issuance arguments are invalid")
	}
	if !filepath.IsAbs(opts.Output) || filepath.Clean(opts.Output) != opts.Output {
		return time.Time{}, errors.New("output path is invalid")
	}
	parent, err := os.Stat(filepath.Dir(opts.Output))
	if err != nil || !parent.IsDir() || runtime.GOOS == "linux" && parent.Mode().Perm() != 0700 {
		return time.Time{}, errors.New("output parent is not protected")
	}
	public, err := accountPublic(opts, false)
	if err != nil {
		return time.Time{}, err
	}
	seedBytes, err := protectedRead(opts.AccountSeed, 256)
	if err != nil {
		return time.Time{}, err
	}
	account, err := nkeys.FromSeed([]byte(strings.TrimSpace(string(seedBytes))))
	if err != nil {
		return time.Time{}, errors.New("APP account seed is invalid")
	}
	defer account.Wipe()
	seedPublic, err := account.PublicKey()
	if err != nil || seedPublic != public {
		return time.Time{}, errors.New("APP account seed does not match public key")
	}
	user, err := nkeys.CreateUser()
	if err != nil {
		return time.Time{}, errors.New("proof user cannot be created")
	}
	defer user.Wipe()
	userPublic, err := user.PublicKey()
	if err != nil {
		return time.Time{}, errors.New("proof user is invalid")
	}
	claim := jwt.NewUserClaims(userPublic)
	claim.Name = "voice-nats-proof"
	claim.IssuedAt = now.Unix()
	expiry = now.Add(opts.TTL).Truncate(time.Second)
	claim.Expires = expiry.Unix()
	claim.Pub.Allow = append([]string(nil), exactPublish...)
	claim.Sub.Allow = append([]string(nil), exactSubscribe...)
	token, err := claim.Encode(account)
	if err != nil {
		return time.Time{}, errors.New("proof JWT cannot be signed")
	}
	userSeed, err := user.Seed()
	if err != nil {
		return time.Time{}, errors.New("proof user seed is unavailable")
	}
	contents := []byte("-----BEGIN NATS USER JWT-----\n" + token + "\n------END NATS USER JWT------\n\n" +
		"-----BEGIN USER NKEY SEED-----\n" + string(userSeed) + "\n------END USER NKEY SEED------\n")
	if err := validateProofContents(contents, public, time.Second, now); err != nil {
		return time.Time{}, fmt.Errorf("generated proof credential is invalid: %w", err)
	}
	file, err := os.OpenFile(opts.Output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return time.Time{}, errors.New("proof output already exists or is unavailable")
	}
	defer func() {
		if resultErr != nil {
			_ = os.Remove(opts.Output)
		}
	}()
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return time.Time{}, errors.New("proof output write failed")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return time.Time{}, errors.New("proof output sync failed")
	}
	if err := file.Close(); err != nil {
		return time.Time{}, errors.New("proof output close failed")
	}
	return expiry, nil
}

func checkProofCredential(opts proofOptions, now time.Time) error {
	if err := validateTarget(opts); err != nil {
		return err
	}
	if opts.MinValidity <= 0 || opts.MinValidity > maximumTTL || opts.Credential == "" || opts.Bundle == "" || opts.AccountPublic != "" {
		return errors.New("proof preflight arguments are invalid")
	}
	public, err := accountPublic(opts, true)
	if err != nil {
		return err
	}
	contents, err := protectedRead(opts.Credential, 16<<10)
	if err != nil {
		return err
	}
	return validateProofContents(contents, public, opts.MinValidity, now)
}

func validateProofContents(contents []byte, accountPublic string, minimum time.Duration, now time.Time) error {
	parts := credentialPattern.FindSubmatch(contents)
	if len(parts) != 3 {
		return errors.New("proof credentials are malformed")
	}
	token := string(parts[1])
	claims, err := jwt.DecodeUserClaims(token)
	if err != nil || claims.Issuer != accountPublic || !nkeys.IsValidPublicUserKey(claims.Subject) {
		return errors.New("proof JWT claims are invalid")
	}
	if err := verifySignature(token, accountPublic); err != nil {
		return errors.New("proof JWT signature is invalid")
	}
	user, err := nkeys.FromSeed(parts[2])
	if err != nil {
		return errors.New("proof user seed is invalid")
	}
	defer user.Wipe()
	userPublic, err := user.PublicKey()
	if err != nil || userPublic != claims.Subject {
		return errors.New("proof user seed does not match JWT subject")
	}
	if !exactSubjects(claims.Pub.Allow, exactPublish) || !exactSubjects(claims.Sub.Allow, exactSubscribe) || claims.Resp != nil || len(claims.Pub.Deny) != 0 || len(claims.Sub.Deny) != 0 {
		return errors.New("proof JWT grants are invalid")
	}
	if claims.IssuedAt <= 0 || claims.Expires <= claims.IssuedAt || claims.Expires-claims.IssuedAt > int64(maximumTTL/time.Second) || claims.IssuedAt > now.Unix()+30 || claims.NotBefore > now.Unix() || time.Unix(claims.Expires, 0).Sub(now) < minimum {
		return errors.New("proof JWT lifetime is invalid")
	}
	return nil
}

func exactSubjects(actual, want []string) bool {
	if len(actual) != len(want) {
		return false
	}
	seen := make(map[string]bool, len(actual))
	for _, subject := range actual {
		if seen[subject] {
			return false
		}
		seen[subject] = true
	}
	for _, subject := range want {
		if !seen[subject] {
			return false
		}
	}
	return true
}

func verifySignature(token, public string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("JWT structure is invalid")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}
	key, err := nkeys.FromPublicKey(public)
	if err != nil {
		return err
	}
	return key.Verify([]byte(parts[0]+"."+parts[1]), signature)
}
