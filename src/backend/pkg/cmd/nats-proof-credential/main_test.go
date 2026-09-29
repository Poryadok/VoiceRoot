package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

var proofPublish = []string{
	"social.friend_request",
	"$JS.API.STREAM.INFO.social_events",
	"$JS.API.STREAM.MSG.GET.social_events",
	"$JS.API.STREAM.MSG.DELETE.social_events",
}

var proofSubscribe = []string{"_INBOX.voice.nats-proof.>"}

func TestIssueProofCredentialExactGrantsAndShortExpiry(t *testing.T) {
	requireLinuxProofHost(t)
	account, public, seedPath, bundlePath := proofFixture(t, "r20260930a1")
	_ = account
	output := protectedOutputPath(t)
	now := time.Now().Truncate(time.Second)
	opts := proofOptions{Namespace: "voice-staging", Generation: "r20260930a1", AccountSeed: seedPath, Bundle: bundlePath, Output: output, TTL: time.Hour}
	expiry, err := issueProofCredential(opts, now)
	if err != nil {
		t.Fatal(err)
	}
	if !expiry.Equal(now.Add(time.Hour)) {
		t.Fatalf("expiry = %v, want %v", expiry, now.Add(time.Hour))
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" {
		info, err := os.Stat(output)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("credential mode = %o, want 0600", info.Mode().Perm())
		}
	}
	token := proofJWT(t, contents)
	claims, err := jwt.DecodeUserClaims(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Issuer != public || claims.Expires != expiry.Unix() {
		t.Fatalf("issuer or expiry differs from APP account contract")
	}
	if !sameGrantSet(claims.Pub.Allow, proofPublish) || !sameGrantSet(claims.Sub.Allow, proofSubscribe) || claims.Resp != nil {
		t.Fatalf("proof permissions differ from exact publish/subscribe grant")
	}
	for _, subject := range claims.Pub.Allow {
		if strings.ContainsAny(subject, "*>") || strings.Contains(subject, "CREATE") || strings.Contains(subject, "PURGE") {
			t.Fatalf("broad or persistent mutation grant: %q", subject)
		}
	}
	check := proofOptions{Namespace: "voice-staging", Generation: "r20260930a1", Bundle: bundlePath, Credential: output, MinValidity: 30 * time.Minute}
	if err := checkProofCredential(check, now); err != nil {
		t.Fatalf("issued proof credential failed read-only preflight: %v", err)
	}
	if err := checkProofCredential(check, now.Add(31*time.Minute)); err == nil {
		t.Fatal("proof credential with less than 30m remaining passed preflight")
	}
	if err := issueProofCredentialWithoutOverwrite(opts, now); err != nil {
		t.Fatal(err)
	}
}

func TestIssueProofCredentialWithPublicKeyInsteadOfBundle(t *testing.T) {
	requireLinuxProofHost(t)
	_, public, seedPath, _ := proofFixture(t, "legacy")
	output := protectedOutputPath(t)
	now := time.Now().Truncate(time.Second)
	opts := proofOptions{Namespace: "voice-staging", Generation: "legacy", AccountSeed: seedPath, AccountPublic: public, Output: output, TTL: time.Hour}
	if _, err := issueProofCredential(opts, now); err != nil {
		t.Fatal(err)
	}
	if _, err := jwt.DecodeUserClaims(proofJWT(t, mustRead(t, output))); err != nil {
		t.Fatalf("public-key mode wrote invalid proof credential: %v", err)
	}
}

func issueProofCredentialWithoutOverwrite(opts proofOptions, now time.Time) error {
	before, err := os.ReadFile(opts.Output)
	if err != nil {
		return err
	}
	if _, err := issueProofCredential(opts, now); err == nil {
		return errors.New("existing credential output was overwritten")
	}
	after, err := os.ReadFile(opts.Output)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return errors.New("existing credential output changed after collision")
	}
	return nil
}

func TestProofCredentialRejectsIdentityAndDurationErrorsBeforeOutput(t *testing.T) {
	requireLinuxProofHost(t)
	_, public, seedPath, bundlePath := proofFixture(t, "legacy")
	_, _, otherSeed, otherBundle := proofFixture(t, "legacy")
	now := time.Now().Truncate(time.Second)
	for _, tc := range []struct {
		name string
		edit func(*proofOptions)
	}{
		{"wrong namespace", func(o *proofOptions) { o.Namespace = "voice-prod" }},
		{"invalid generation", func(o *proofOptions) { o.Generation = "../r20260930" }},
		{"bundle generation mismatch", func(o *proofOptions) { o.Generation = "r20260930a1" }},
		{"wrong account seed", func(o *proofOptions) { o.AccountSeed = otherSeed }},
		{"wrong bundle account", func(o *proofOptions) { o.Bundle = otherBundle }},
		{"missing identity", func(o *proofOptions) { o.Bundle = ""; o.AccountPublic = "" }},
		{"ambiguous identity", func(o *proofOptions) { o.AccountPublic = public }},
		{"zero TTL", func(o *proofOptions) { o.TTL = 0 }},
		{"over two hours", func(o *proofOptions) { o.TTL = 2*time.Hour + time.Second }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := protectedOutputPath(t)
			opts := proofOptions{Namespace: "voice-staging", Generation: "legacy", AccountSeed: seedPath, Bundle: bundlePath, Output: output, TTL: time.Hour}
			tc.edit(&opts)
			if _, err := issueProofCredential(opts, now); err == nil {
				t.Fatal("unsafe issuance succeeded")
			}
			if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed issuance left output: %v", err)
			}
		})
	}
}

func TestReadOnlyPreflightRejectsBroadOrUntrustedCredentials(t *testing.T) {
	requireLinuxProofHost(t)
	account, _, seedPath, bundlePath := proofFixture(t, "r20260930a1")
	_, _, _, wrongBundle := proofFixture(t, "r20260930a1")
	now := time.Now().Truncate(time.Second)
	output := protectedOutputPath(t)
	opts := proofOptions{Namespace: "voice-staging", Generation: "r20260930a1", AccountSeed: seedPath, Bundle: bundlePath, Output: output, TTL: time.Hour}
	if _, err := issueProofCredential(opts, now); err != nil {
		t.Fatal(err)
	}
	check := proofOptions{Namespace: "voice-staging", Generation: "r20260930a1", Bundle: bundlePath, Credential: output, MinValidity: 30 * time.Minute}
	check.Bundle = wrongBundle
	if err := checkProofCredential(check, now); err == nil {
		t.Fatal("credential signed by another APP account passed preflight")
	}
	check.Bundle = bundlePath
	for _, tc := range []struct {
		name, credential string
	}{
		{"broad publish", signedProofCreds(t, account, now, append(append([]string{}, proofPublish...), "$JS.API.STREAM.CREATE.social_events"), proofSubscribe)},
		{"broad inbox", signedProofCreds(t, account, now, proofPublish, []string{"_INBOX.>"})},
		{"wrong cleanup stream", signedProofCreds(t, account, now, []string{"social.friend_request", "$JS.API.STREAM.INFO.social_events", "$JS.API.STREAM.MSG.GET.social_events", "$JS.API.STREAM.MSG.DELETE.message_events"}, proofSubscribe)},
		{"excessive lifetime", signedProofCredsWithTTL(t, account, now, proofPublish, proofSubscribe, 24*time.Hour)},
		{"tampered signature", tamperedProofCreds(t, mustRead(t, output))},
		{"mismatched user seed", mismatchedProofSeed(t, mustRead(t, output))},
		{"malformed delimiter", "not a credential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.creds")
			if err := os.WriteFile(path, []byte(tc.credential), 0600); err != nil {
				t.Fatal(err)
			}
			check.Credential = path
			if err := checkProofCredential(check, now); err == nil {
				t.Fatal("unsafe credential passed read-only preflight")
			}
		})
	}
}

func TestProofCredentialCLIPrintsOnlyExpiryMetadata(t *testing.T) {
	requireLinuxProofHost(t)
	_, _, seedPath, bundlePath := proofFixture(t, "legacy")
	output := protectedOutputPath(t)
	now := time.Now().Truncate(time.Second)
	var stdout, stderr bytes.Buffer
	args := []string{"--namespace", "voice-staging", "--generation", "legacy", "--account-seed", seedPath, "--bundle", bundlePath, "--ttl", "60m", "--output", output}
	if code := run(args, &stdout, &stderr, now); code != 0 {
		t.Fatalf("CLI failed: %s", stderr.String())
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "expires_at=") || strings.Contains(stdout.String(), string(contents)) || strings.Contains(stdout.String(), "BEGIN NATS USER JWT") {
		t.Fatal("CLI did not emit only safe expiry metadata")
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--namespace", "voice-staging", "--generation", "legacy", "--bundle", bundlePath, "--credential", output, "--check-min-validity", "30m"}, &stdout, &stderr, now); code != 0 {
		t.Fatalf("CLI read-only preflight failed: %s", stderr.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), string(contents)) || strings.Contains(stdout.String()+stderr.String(), "BEGIN NATS USER JWT") {
		t.Fatal("preflight printed credential content")
	}
}

func TestProtectedReadAcceptsOwnerOnlyModesOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("POSIX permission contract")
	}
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("inert"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []os.FileMode{0400, 0600} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		if _, err := protectedRead(path, 32); err != nil {
			t.Fatalf("owner-only mode %o rejected: %v", mode, err)
		}
	}
	for _, mode := range []os.FileMode{0440, 0644, 0700} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		if _, err := protectedRead(path, 32); err == nil {
			t.Fatalf("mode %o accepted", mode)
		}
	}
}

func protectedOutputPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "proof.creds")
}

func requireLinuxProofHost(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("proof credential operations require Linux filesystem permissions")
	}
}

func TestProofCredentialRejectsNonLinuxHost(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-Linux refusal contract")
	}
	account, _, seedPath, bundlePath := proofFixture(t, "r20260930a1")
	now := time.Now().Truncate(time.Second)
	output := protectedOutputPath(t)
	issue := proofOptions{Namespace: "voice-staging", Generation: "r20260930a1", AccountSeed: seedPath, Bundle: bundlePath, Output: output, TTL: time.Hour}
	if _, err := issueProofCredential(issue, now); err == nil {
		t.Fatal("proof credential issuance succeeded on non-Linux host")
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-Linux issuance wrote output: %v", err)
	}
	credential := filepath.Join(t.TempDir(), "proof.creds")
	if err := os.WriteFile(credential, []byte(signedProofCreds(t, account, now, proofPublish, proofSubscribe)), 0600); err != nil {
		t.Fatal(err)
	}
	check := proofOptions{Namespace: "voice-staging", Generation: "r20260930a1", Bundle: bundlePath, Credential: credential, MinValidity: 30 * time.Minute}
	if err := checkProofCredential(check, now); err == nil {
		t.Fatal("proof credential preflight succeeded on non-Linux host")
	}
}

func proofFixture(t *testing.T, generation string) (nkeys.KeyPair, string, string, string) {
	t.Helper()
	account, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	public, err := account.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := account.Seed()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	seedPath := filepath.Join(dir, "app-account.seed")
	if err := os.WriteFile(seedPath, append(seed, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	items := []map[string]any{}
	serviceKeys := map[string]string{}
	for _, service := range strings.Fields("analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice") {
		serviceKeys[service+".creds"] = base64.StdEncoding.EncodeToString([]byte("inert"))
	}
	operatorKeys := map[string]string{}
	for _, key := range []string{"operator.jwt", "account.jwt", "system-account.jwt", "account.public", "system-account.public"} {
		operatorKeys[key] = base64.StdEncoding.EncodeToString([]byte("inert"))
	}
	operatorKeys["account.public"] = base64.StdEncoding.EncodeToString([]byte(public))
	keysBySecret := map[string]map[string]string{
		"voice-nats-operator":              operatorKeys,
		"voice-nats-hub-tls":               {"tls.crt": base64.StdEncoding.EncodeToString([]byte("inert")), "tls.key": base64.StdEncoding.EncodeToString([]byte("inert")), "ca.crt": base64.StdEncoding.EncodeToString([]byte("inert"))},
		"voice-nats-bootstrap-credentials": {"bootstrap.creds": base64.StdEncoding.EncodeToString([]byte("inert"))},
		"voice-nats-service-credentials":   serviceKeys,
	}
	for _, base := range []string{"voice-nats-operator", "voice-nats-hub-tls", "voice-nats-bootstrap-credentials", "voice-nats-service-credentials"} {
		itemName := base
		if generation != "legacy" {
			itemName += "-" + generation
		}
		data := keysBySecret[base]
		item := map[string]any{"apiVersion": "v1", "kind": "Secret", "type": "Opaque", "metadata": map[string]string{"name": itemName, "namespace": "voice-staging"}, "data": data}
		if generation != "legacy" {
			item["immutable"] = true
		}
		items = append(items, item)
	}
	bundlePath := filepath.Join(dir, "bundle.json")
	contents, err := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "List", "items": items})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundlePath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	return account, public, seedPath, bundlePath
}

func proofJWT(t *testing.T, contents []byte) string {
	t.Helper()
	const begin = "-----BEGIN NATS USER JWT-----\n"
	const end = "\n------END NATS USER JWT------"
	start := strings.Index(string(contents), begin)
	stop := strings.Index(string(contents), end)
	if start < 0 || stop <= start {
		t.Fatal("malformed NATS credentials")
	}
	return string(contents[start+len(begin) : stop])
}

func signedProofCreds(t *testing.T, account nkeys.KeyPair, now time.Time, publish, subscribe []string) string {
	return signedProofCredsWithTTL(t, account, now, publish, subscribe, time.Hour)
}

func signedProofCredsWithTTL(t *testing.T, account nkeys.KeyPair, now time.Time, publish, subscribe []string, ttl time.Duration) string {
	t.Helper()
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	public, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	claim := jwt.NewUserClaims(public)
	claim.IssuedAt = now.Unix()
	claim.Expires = now.Add(ttl).Unix()
	claim.Pub.Allow = publish
	claim.Sub.Allow = subscribe
	token, err := claim.Encode(account)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	return "-----BEGIN NATS USER JWT-----\n" + token + "\n------END NATS USER JWT------\n\n" +
		"-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func sameGrantSet(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	actual = append([]string(nil), actual...)
	expected = append([]string(nil), expected...)
	slices.Sort(actual)
	slices.Sort(expected)
	return slices.Equal(actual, expected)
}

func tamperedProofCreds(t *testing.T, contents []byte) string {
	t.Helper()
	token := proofJWT(t, contents)
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(parts[2]) == 0 {
		t.Fatal("fixture JWT is malformed")
	}
	signature := []byte(parts[2])
	if signature[0] == 'A' {
		signature[0] = 'B'
	} else {
		signature[0] = 'A'
	}
	parts[2] = string(signature)
	return strings.Replace(string(contents), token, strings.Join(parts, "."), 1)
}

func mismatchedProofSeed(t *testing.T, contents []byte) string {
	t.Helper()
	other, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := other.Seed()
	if err != nil {
		t.Fatal(err)
	}
	const begin = "-----BEGIN USER NKEY SEED-----\n"
	const end = "\n------END USER NKEY SEED------"
	text := string(contents)
	start, stop := strings.Index(text, begin), strings.Index(text, end)
	if start < 0 || stop <= start {
		t.Fatal("fixture credentials lack user seed section")
	}
	return text[:start+len(begin)] + string(seed) + text[stop:]
}
