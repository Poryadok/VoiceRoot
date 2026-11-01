package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	jwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
)

func TestFreshKeyMaterialBindsClaimsAndDecoratedSeed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"genesis", dir, "/workspace/deploy/nats/acl-intent.yaml"}
	run()
	read := func(name string) []byte {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var genesis struct {
		Account  string            `json:"account"`
		Operator string            `json:"operator"`
		System   string            `json:"system_account"`
		Users    map[string]string `json:"users"`
		ACL      string            `json:"acl_sha256"`
	}
	if e := json.Unmarshal(read("genesis.json"), &genesis); e != nil {
		t.Fatal(e)
	}
	op, e := jwt.DecodeOperatorClaims(string(read("operator.jwt")))
	if e != nil || op.Subject != genesis.Operator || op.SystemAccount != genesis.System {
		t.Fatal("operator binding invalid")
	}
	account, e := jwt.DecodeAccountClaims(string(read("account.jwt")))
	if e != nil || account.Subject != genesis.Account || account.Issuer != genesis.Operator {
		t.Fatal("account binding invalid")
	}
	if account.Limits.AccountLimits != (jwt.AccountLimits{Conn: 64, LeafNodeConn: 64}) ||
		account.Limits.NatsLimits != (jwt.NatsLimits{Subs: 1024, Data: 64 << 20, Payload: 1 << 20}) {
		t.Fatal("fixture account and NATS limits changed")
	}
	system, e := jwt.DecodeAccountClaims(string(read("system-account.jwt")))
	if e != nil || system.Subject != genesis.System || system.Issuer != genesis.Operator {
		t.Fatal("system binding invalid")
	}
	if genesis.ACL != aclSHA256 || len(genesis.Users) < 2 {
		t.Fatal("genesis policy or role set invalid")
	}
	seed, e := nkeys.FromSeed(read("fixture-account.seed"))
	if e != nil || public(seed) != genesis.Account {
		t.Fatal("fixture signer does not match account")
	}
	raw, e := os.ReadFile("/workspace/deploy/nats/acl-intent.yaml")
	if e != nil {
		t.Fatal(e)
	}
	var policy policy
	if e = yaml.Unmarshal(raw, &policy); e != nil {
		t.Fatal(e)
	}
	policy.Services["bootstrap"] = policy.Bootstrap
	for role, pub := range genesis.Users {
		creds := read(role + ".creds")
		token, e := jwt.ParseDecoratedJWT(creds)
		if e != nil {
			t.Fatal("decorated JWT invalid")
		}
		user, e := jwt.DecodeUserClaims(token)
		if e != nil || user.Subject != pub || user.Issuer != genesis.Account || user.Name != "voice-"+role {
			t.Fatal("actor binding invalid")
		}
		want, found := policy.Services[role]
		if !found || !reflect.DeepEqual([]string(user.Pub.Allow), want.Publish) || !reflect.DeepEqual([]string(user.Sub.Allow), want.Subscribe) || user.Resp != nil {
			t.Fatal("actor permission differs from reviewed policy")
		}
		pair, e := jwt.ParseDecoratedUserNKey(creds)
		if e != nil || public(pair) != pub {
			t.Fatal("seed does not match actor")
		}
	}
}

func TestChangedACLRefusesBeforeCreatingKeys(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	bad := filepath.Join(t.TempDir(), "acl.yaml")
	if e := os.WriteFile(bad, []byte("version: 1\n"), 0600); e != nil {
		t.Fatal(e)
	}
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"genesis", dir, bad}
	defer func() {
		if recover() == nil {
			t.Fatal("changed ACL accepted")
		}
		rows, e := os.ReadDir(dir)
		if e != nil || len(rows) != 0 {
			t.Fatal("keys created before refusal")
		}
	}()
	run()
}

func TestUnsafeDirectoryCustodyRefused(t *testing.T) {
	root := t.TempDir()
	for _, kind := range []string{"foreign-owner", "writable-parent", "symlink-parent"} {
		t.Run(kind, func(t *testing.T) {
			parent := filepath.Join(root, kind)
			if e := os.Mkdir(parent, 0700); e != nil {
				t.Fatal(e)
			}
			dir := filepath.Join(parent, "private")
			if e := os.Mkdir(dir, 0700); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "foreign-owner":
				if e := os.Chown(dir, 65532, 65532); e != nil {
					t.Fatal(e)
				}
			case "writable-parent":
				if e := os.Chmod(parent, 0777); e != nil {
					t.Fatal(e)
				}
			case "symlink-parent":
				link := filepath.Join(root, "linked")
				if e := os.Symlink(parent, link); e != nil {
					t.Fatal(e)
				}
				dir = filepath.Join(link, "private")
			}
			defer func() {
				if recover() == nil {
					t.Fatal("unsafe directory accepted")
				}
				rows, e := os.ReadDir(filepath.Join(parent, "private"))
				if e != nil || len(rows) != 0 {
					t.Fatal("unexpected key material")
				}
			}()
			privateDirectory(dir)
		})
	}
}

func TestFailureRemovesOnlyFilesCreatedByGenerator(t *testing.T) {
	dir := t.TempDir()
	out := &output{dir: dir}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("failure not propagated")
			}
		}()
		defer out.cleanupOnFailure()
		out.write("fixture-account.seed", []byte("synthetic"))
		panic("injected failure")
	}()
	rows, e := os.ReadDir(dir)
	if e != nil || len(rows) != 0 {
		t.Fatal("partial output not removed")
	}
}
