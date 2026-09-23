// nats-jwt-fixture makes disposable, non-production JWT credentials for local
// Compose and staging rehearsal. It never prints generated private material.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

var serviceNames = []string{"analytics", "auth", "bot", "chat", "file", "gateway", "matchmaking", "messaging", "moderation", "notification", "realtime", "role", "search", "social", "space", "story", "subscription", "user", "voice"}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: nats-jwt-fixture DESTINATION")
		os.Exit(2)
	}
	if err := generate(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "nats JWT fixture generation failed")
		os.Exit(1)
	}
	fmt.Println("NATS JWT fixture generated")
}

func generate(dest string) error {
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

	account, err := nkeys.CreateAccount()
	if err != nil {
		return err
	}
	accountPub, _ := account.PublicKey()
	accountClaim := jwt.NewAccountClaims(accountPub)
	accountJWT, err := accountClaim.Encode(op)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dest, "operator.jwt"), opJWT); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dest, "account.jwt"), accountJWT); err != nil {
		return err
	}

	var intent strings.Builder
	intent.WriteString("# Generated fixture ACL intent. Activation must replace this with subject-specific grants.\nservices:\n")
	for _, name := range serviceNames {
		user, err := nkeys.CreateUser()
		if err != nil {
			return err
		}
		pub, _ := user.PublicKey()
		claim := jwt.NewUserClaims(pub)
		claim.Name = "voice-" + name
		// Deliberately narrow placeholders: no general JetStream management grant.
		claim.Pub.Allow = []string{"voice." + name + ".>"}
		claim.Sub.Allow = []string{"voice." + name + ".>"}
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
		fmt.Fprintf(&intent, "  %s:\n    publish: [\"voice.%s.>\"]\n    subscribe: [\"voice.%s.>\"]\n", name, name, name)
	}
	if err := writeFile(filepath.Join(dest, "acl-intent.yaml"), intent.String()); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func writeFile(path, contents string) error { return os.WriteFile(path, []byte(contents+"\n"), 0600) }
