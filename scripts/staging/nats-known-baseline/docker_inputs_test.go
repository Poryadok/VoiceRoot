//go:build linuxintegration

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
)

// Disposable signing keys are test-only, generated inside the owned local
// Linux test volume. They never enroll or alter deployed auth identities.
func TestDockerFixtureInputs(t *testing.T) {
	base := os.Getenv("KNOWN_NATS_TEST_INPUTS")
	if !strings.HasPrefix(base, "/var/lib/docker/volumes/voice-known-test-") || !strings.HasSuffix(base, "/_data") {
		t.Fatal("owned Docker test volume input required")
	}
	must := func(err error) {
		if err != nil {
			t.Fatal("test input preparation failed")
		}
	}
	write := func(name string, b []byte) {
		p := filepath.Join(base, name)
		must(os.WriteFile(p, b, 0440))
		must(os.Chown(p, 0, 65532))
	}
	must(os.MkdirAll(filepath.Join(base, "inputs"), 0750))
	must(os.Chown(filepath.Join(base, "inputs"), 0, 65532))
	op, err := nkeys.CreateOperator()
	must(err)
	defer op.Wipe()
	app, err := nkeys.CreateAccount()
	must(err)
	defer app.Wipe()
	sys, err := nkeys.CreateAccount()
	must(err)
	defer sys.Wipe()
	opPub, err := op.PublicKey()
	must(err)
	appPub, err := app.PublicKey()
	must(err)
	sysPub, err := sys.PublicKey()
	must(err)
	oc := jwt.NewOperatorClaims(opPub)
	oc.SystemAccount = sysPub
	operator, err := oc.Encode(op)
	must(err)
	ac := jwt.NewAccountClaims(appPub)
	ac.Limits = jwt.OperatorLimits{AccountLimits: jwt.AccountLimits{Conn: 64, LeafNodeConn: 64},
		NatsLimits:      jwt.NatsLimits{Subs: 1024, Data: 64 << 20, Payload: 1 << 20},
		JetStreamLimits: jwt.JetStreamLimits{MemoryStorage: 8 << 20, DiskStorage: 32 << 20, Streams: 64, Consumer: 512, MaxAckPending: 4096}}
	account, err := ac.Encode(op)
	must(err)
	system, err := jwt.NewAccountClaims(sysPub).Encode(op)
	must(err)
	write("server.conf", []byte(fmt.Sprintf("host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\nmax_control_line: 32768\noperator: %q\nsystem_account: %q\nresolver: MEMORY\nresolver_preload: { %q: %q, %q: %q }\njetstream: { store_dir: /data, max_file_store: 64MB, max_memory_store: 16MB }\n", operator, sysPub, appPub, account, sysPub, system)))
	type grant struct {
		Publish   []string `yaml:"publish"`
		Subscribe []string `yaml:"subscribe"`
	}
	var policy struct {
		Bootstrap grant            `yaml:"bootstrap"`
		Services  map[string]grant `yaml:"services"`
	}
	raw, err := os.ReadFile("../../../deploy/nats/acl-intent.yaml")
	must(err)
	must(yaml.Unmarshal(raw, &policy))
	for _, role := range []string{"bootstrap", "social", "realtime"} {
		g := policy.Services[role]
		if role == "bootstrap" {
			g = policy.Bootstrap
		}
		if len(g.Publish) == 0 || len(g.Subscribe) == 0 {
			t.Fatal("selected role source grant missing")
		}
		user, err := nkeys.CreateUser()
		must(err)
		pub, err := user.PublicKey()
		must(err)
		uc := jwt.NewUserClaims(pub)
		uc.Pub.Allow = g.Publish
		uc.Sub.Allow = g.Subscribe
		token, err := uc.Encode(app)
		must(err)
		seed, err := user.Seed()
		must(err)
		write("inputs/"+role+".creds", []byte("-----BEGIN NATS USER JWT-----\n"+token+"\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n"+string(seed)+"\n------END USER NKEY SEED------\n"))
		user.Wipe()
	}
	for _, role := range []string{"realtime", "notification", "analytics-chat", "search"} {
		raw, err := os.ReadFile("testdata/deployed-bootstrap-20261004/" + role + "-bootstrap.sh")
		must(err)
		if !strings.HasPrefix(string(raw), "#!/bin/sh\n") {
			t.Fatal("deployed bootstrap script missing")
		}
		write("bootstrap-"+role+".sh", raw)
	}
}
