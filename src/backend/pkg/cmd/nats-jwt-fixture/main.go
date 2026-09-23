// nats-jwt-fixture makes disposable, non-production JWT credentials for local
// Compose and staging rehearsal. It never prints generated private material.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
)

var serviceNames = []string{"analytics", "auth", "bot", "chat", "file", "gateway", "matchmaking", "messaging", "moderation", "notification", "realtime", "role", "search", "social", "space", "story", "subscription", "user", "voice"}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: nats-jwt-fixture ACL_MANIFEST DESTINATION")
		os.Exit(2)
	}
	acl, err := loadACL(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "nats JWT fixture ACL is invalid")
		os.Exit(1)
	}
	if err := generate(os.Args[2], acl); err != nil {
		fmt.Fprintln(os.Stderr, "nats JWT fixture generation failed")
		os.Exit(1)
	}
	fmt.Println("NATS JWT fixture generated")
}

type serviceACL struct {
	Publish   []string `yaml:"publish"`
	Subscribe []string `yaml:"subscribe"`
}

type aclDocument struct {
	Version   int                   `yaml:"version"`
	Services  map[string]serviceACL `yaml:"services"`
	Bootstrap serviceACL            `yaml:"bootstrap"`
}

func loadACL(path string) (aclDocument, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return aclDocument{}, err
	}
	var acl aclDocument
	decoder := yaml.NewDecoder(strings.NewReader(string(contents)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&acl); err != nil {
		return aclDocument{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return aclDocument{}, errors.New("ACL must contain exactly one document")
		}
		return aclDocument{}, err
	}
	return acl, validateACL(acl)
}

func validateACL(acl aclDocument) error {
	if acl.Version != 1 || len(acl.Services) != len(serviceNames) {
		return errors.New("ACL must specify version one and every service exactly once")
	}
	for _, name := range serviceNames {
		grant, ok := acl.Services[name]
		if !ok {
			return fmt.Errorf("ACL missing service %s", name)
		}
		if err := validateGrant(grant); err != nil {
			return fmt.Errorf("ACL service %s: %w", name, err)
		}
	}
	if err := validateGrant(acl.Bootstrap); err != nil {
		return fmt.Errorf("ACL bootstrap: %w", err)
	}
	return nil
}

func validateGrant(grant serviceACL) error {
	if len(grant.Publish) == 0 && len(grant.Subscribe) == 0 {
		return errors.New("at least one permission is required")
	}
	seen := make(map[string]struct{}, len(grant.Publish)+len(grant.Subscribe))
	for _, subject := range grant.Publish {
		if !safePublishSubject(subject) {
			return fmt.Errorf("unsafe subject %q", subject)
		}
		if _, duplicate := seen[subject]; duplicate {
			return fmt.Errorf("duplicate subject %q", subject)
		}
		seen[subject] = struct{}{}
	}
	for _, subject := range grant.Subscribe {
		if !safeSubscribeSubject(subject) {
			return fmt.Errorf("unsafe subject %q", subject)
		}
		if _, duplicate := seen[subject]; duplicate {
			return fmt.Errorf("duplicate subject %q", subject)
		}
		seen[subject] = struct{}{}
	}
	return nil
}

// JetStream acknowledgement subjects contain server-assigned delivery tokens.
// A fixed stream/durable prefix with a terminal wildcard is the narrowest
// viable user grant: the client cannot ACK a neighbouring durable or access
// the JetStream management API.
func safePublishSubject(subject string) bool {
	if subject == "" || strings.ContainsAny(subject, " \t\r\n*") || subject == "$JS.API.>" {
		return false
	}
	if !strings.Contains(subject, ">") {
		return true
	}
	if !strings.HasPrefix(subject, "$JS.ACK.") || !strings.HasSuffix(subject, ".>") || strings.Count(subject, ">") != 1 {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(subject, ".>"), ".")
	return len(parts) == 4 && parts[2] != "" && parts[3] != ""
}

func safeSubscribeSubject(subject string) bool {
	return subject != "" && !strings.ContainsAny(subject, " \t\r\n>*") && subject != "$JS.API.>"
}

func generate(dest string, acl aclDocument) error {
	if err := validateACL(acl); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("destination must not exist: %s", dest)
	}
	if err := os.MkdirAll(filepath.Join(dest, "creds"), 0700); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(dest)
		}
	}()

	op, err := nkeys.CreateOperator()
	if err != nil {
		return err
	}
	opPub, _ := op.PublicKey()
	opClaim := jwt.NewOperatorClaims(opPub)
	opJWT, err := opClaim.Encode(op)
	if err != nil {
		return err
	}

	// The NATS system account is deliberately independent from the application
	// account. JetStream application streams must never run in SYS, and SYS has
	// no service or bootstrap credentials.
	systemAccount, err := nkeys.CreateAccount()
	if err != nil {
		return err
	}
	systemAccountPub, _ := systemAccount.PublicKey()
	systemAccountClaim := jwt.NewAccountClaims(systemAccountPub)
	systemAccountJWT, err := systemAccountClaim.Encode(op)
	if err != nil {
		return err
	}

	account, err := nkeys.CreateAccount()
	if err != nil {
		return err
	}
	accountPub, _ := account.PublicKey()
	accountClaim := jwt.NewAccountClaims(accountPub)
	accountClaim.Limits = jwt.OperatorLimits{
		AccountLimits: jwt.AccountLimits{Conn: 64, LeafNodeConn: 64},
		NatsLimits:    jwt.NatsLimits{Subs: 1024, Data: 64 << 20, Payload: 1 << 20},
		JetStreamLimits: jwt.JetStreamLimits{
			MemoryStorage: 64 << 20,
			DiskStorage:   512 << 20,
			Streams:       64,
			Consumer:      512,
			MaxAckPending: 4096,
		},
	}
	accountJWT, err := accountClaim.Encode(op)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dest, "operator.jwt"), opJWT); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dest, "system-account.jwt"), systemAccountJWT); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dest, "system-account.public"), systemAccountPub); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dest, "account.jwt"), accountJWT); err != nil {
		return err
	}
	// Public account identity is required by a MEMORY resolver's preload map.
	// It is intentionally separate from the JWT/creds secret material.
	if err := writeFile(filepath.Join(dest, "account.public"), accountPub); err != nil {
		return err
	}

	for _, name := range serviceNames {
		grant := acl.Services[name]
		user, err := nkeys.CreateUser()
		if err != nil {
			return err
		}
		pub, _ := user.PublicKey()
		claim := jwt.NewUserClaims(pub)
		claim.Name = "voice-" + name
		applyGrant(claim, grant)
		userJWT, err := claim.Encode(account)
		if err != nil {
			return err
		}
		seed, err := user.Seed()
		if err != nil {
			return err
		}
		creds := "-----BEGIN NATS USER JWT-----\n" + userJWT + "\n------END NATS USER JWT------\n\n" +
			"************************* IMPORTANT *************************\n" +
			"NKEY Seed printed below can be used to sign and prove identity.\n" +
			"NKEYs are sensitive and should be treated as secrets.\n" +
			"-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
		if err := writeFile(filepath.Join(dest, "creds", name+".creds"), creds); err != nil {
			return err
		}
	}
	if err := writeCredential(filepath.Join(dest, "creds", "bootstrap.creds"), "voice-nats-bootstrap", acl.Bootstrap, account); err != nil {
		return err
	}
	intent, err := yaml.Marshal(acl)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dest, "acl-intent.yaml"), string(intent)); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func writeCredential(path, name string, grant serviceACL, account nkeys.KeyPair) error {
	user, err := nkeys.CreateUser()
	if err != nil {
		return err
	}
	pub, _ := user.PublicKey()
	claim := jwt.NewUserClaims(pub)
	claim.Name = name
	applyGrant(claim, grant)
	userJWT, err := claim.Encode(account)
	if err != nil {
		return err
	}
	seed, err := user.Seed()
	if err != nil {
		return err
	}
	creds := "-----BEGIN NATS USER JWT-----\n" + userJWT + "\n------END NATS USER JWT------\n\n" +
		"************************* IMPORTANT *************************\n" +
		"NKEY Seed printed below can be used to sign and prove identity.\n" +
		"NKEYs are sensitive and should be treated as secrets.\n" +
		"-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	return writeFile(path, creds)
}

func applyGrant(claim *jwt.UserClaims, grant serviceACL) {
	claim.Pub.Allow = append([]string(nil), grant.Publish...)
	claim.Sub.Allow = append([]string(nil), grant.Subscribe...)
	// JetStream INFO, bind and publish operations use an ephemeral reply inbox.
	// Response permissions permit only replies to a request made by this user,
	// avoiding a broad `_INBOX.>` subscription grant.
	claim.Resp = &jwt.ResponsePermission{MaxMsgs: 16, Expires: time.Second}
	if len(claim.Pub.Allow) == 0 {
		claim.Pub.Deny = []string{">"}
	}
	if len(claim.Sub.Allow) == 0 {
		claim.Sub.Deny = []string{">"}
	}
}

func writeFile(path, contents string) error { return os.WriteFile(path, []byte(contents+"\n"), 0600) }
