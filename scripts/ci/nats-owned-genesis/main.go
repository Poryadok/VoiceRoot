// Test-only key generator. It does not access Kubernetes, production credentials,
// or establish runtime acceptance. The caller owns a private disposable directory
// and must remove its exact contents after success, failure, timeout, or SIGKILL.
// Failure cleanup below is best effort and is not a substitute for caller custody.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	jwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
)

type grant struct {
	Publish    []string `yaml:"publish"`
	Subscribe  []string `yaml:"subscribe"`
	NoResponse bool     `yaml:"no_response"`
}
type policy struct {
	Version   int              `yaml:"version"`
	Services  map[string]grant `yaml:"services"`
	Bootstrap grant            `yaml:"bootstrap"`
}

const aclSHA256 = "5cee12a03b98b1ad8f609f288f0f3dd03a5b10cfa42ff4a8787657637cee8fc3"

func public(k nkeys.KeyPair) string {
	s, e := k.PublicKey()
	if e != nil {
		panic(e)
	}
	return s
}
func key(f func() (nkeys.KeyPair, error)) nkeys.KeyPair {
	k, e := f()
	if e != nil {
		panic(e)
	}
	return k
}
func encoded(c interface {
	Encode(nkeys.KeyPair) (string, error)
}, k nkeys.KeyPair) string {
	s, e := c.Encode(k)
	if e != nil {
		panic(e)
	}
	return s
}

type output struct {
	dir     string
	created []string
}

func (o *output) write(name string, b []byte) {
	path := filepath.Join(o.dir, name)
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		panic(e)
	}
	o.created = append(o.created, path)
	if _, e = f.Write(b); e != nil {
		f.Close()
		panic(e)
	}
	if e = f.Close(); e != nil {
		panic(e)
	}
}
func (o *output) cleanupOnFailure() {
	if problem := recover(); problem != nil {
		for i := len(o.created) - 1; i >= 0; i-- {
			_ = os.Remove(o.created[i])
		}
		panic(problem)
	}
}

func privateDirectory(dir string) {
	if os.Geteuid() != 0 || filepath.Clean(dir) != dir {
		panic(errors.New("unsafe private output directory"))
	}
	resolved, e := filepath.EvalSymlinks(dir)
	if e != nil || resolved != dir {
		panic(errors.New("unsafe private output directory"))
	}
	for path := dir; ; path = filepath.Dir(path) {
		info, e := os.Lstat(path)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
			panic(errors.New("unsafe private output directory"))
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 || (path == dir && info.Mode().Perm() != 0700) {
			panic(errors.New("unsafe private output directory"))
		}
		if path == "/" {
			break
		}
	}
}
func run() {
	if len(os.Args) != 3 || !filepath.IsAbs(os.Args[1]) || !filepath.IsAbs(os.Args[2]) {
		panic(errors.New("private output and ACL paths required"))
	}
	dir := os.Args[1]
	privateDirectory(dir)
	entries, e := os.ReadDir(dir)
	if e != nil || len(entries) != 0 {
		panic(errors.New("output directory must be empty"))
	}
	out := &output{dir: dir}
	defer out.cleanupOnFailure()
	raw, e := os.ReadFile(os.Args[2])
	if e != nil || len(raw) > 1<<20 {
		panic(errors.New("invalid ACL source"))
	}
	aclHash := sha256.Sum256(raw)
	if hex.EncodeToString(aclHash[:]) != aclSHA256 {
		panic(errors.New("ACL source changed"))
	}
	var p policy
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if e = dec.Decode(&p); e != nil || p.Version != 1 || len(p.Services) == 0 {
		panic(errors.New("invalid ACL policy"))
	}
	if _, exists := p.Services["bootstrap"]; exists {
		panic(errors.New("duplicate bootstrap role"))
	}
	p.Services["bootstrap"] = p.Bootstrap
	op := key(nkeys.CreateOperator)
	app := key(nkeys.CreateAccount)
	system := key(nkeys.CreateAccount)
	opPub, appPub, systemPub := public(op), public(app), public(system)
	operator := jwt.NewOperatorClaims(opPub)
	operator.SystemAccount = systemPub
	appClaim := jwt.NewAccountClaims(appPub)
	appClaim.Limits = jwt.OperatorLimits{
		AccountLimits:   jwt.AccountLimits{Conn: 64, LeafNodeConn: 64},
		NatsLimits:      jwt.NatsLimits{Subs: 1024, Data: 64 << 20, Payload: 1 << 20},
		JetStreamLimits: jwt.JetStreamLimits{MemoryStorage: 1 << 30, DiskStorage: 1 << 30, Streams: 100, Consumer: 100, MaxAckPending: 10000},
	}
	out.write("operator.jwt", []byte(encoded(operator, op)))
	out.write("account.jwt", []byte(encoded(appClaim, op)))
	out.write("system-account.jwt", []byte(encoded(jwt.NewAccountClaims(systemPub), op)))
	out.write("account.public", []byte(appPub))
	out.write("system-account.public", []byte(systemPub))
	signer, e := app.Seed()
	if e != nil {
		panic(e)
	}
	out.write("fixture-account.seed", signer)
	names := make([]string, 0, len(p.Services))
	for name := range p.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	users := map[string]string{}
	for _, name := range names {
		g := p.Services[name]
		if name == "" || filepath.Base(name) != name || !g.NoResponse || len(g.Publish) == 0 || len(g.Subscribe) == 0 {
			panic(errors.New("invalid role grant"))
		}
		user := key(nkeys.CreateUser)
		pub := public(user)
		users[name] = pub
		claim := jwt.NewUserClaims(pub)
		claim.Name = "voice-" + name
		claim.Pub.Allow = g.Publish
		claim.Sub.Allow = g.Subscribe
		claim.Expires = time.Now().Add(time.Hour).Unix()
		token := encoded(claim, app)
		seed, e := user.Seed()
		if e != nil {
			panic(e)
		}
		creds, e := jwt.FormatUserConfig(token, seed)
		if e != nil {
			panic(e)
		}
		out.write(name+".creds", creds)
	}
	if users["bootstrap"] == "" {
		panic(errors.New("bootstrap role missing"))
	}
	id := make([]byte, 16)
	if _, e = rand.Read(id); e != nil {
		panic(e)
	}
	id[6] = (id[6] & 15) | 64
	id[8] = (id[8] & 63) | 128
	realm := fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	descriptor := map[string]any{"schema": "voice-owned-nats-genesis-v1", "realm_id": realm, "generation": "r20260930a4", "operator": opPub, "account": appPub, "system_account": systemPub, "users": users, "acl_sha256": aclSHA256}
	data, e := json.Marshal(descriptor)
	if e != nil {
		panic(e)
	}
	out.write("genesis.json", data)
	sum := sha256.Sum256(data)
	fmt.Println("genesis_sha256=" + hex.EncodeToString(sum[:]))
}
func main() {
	defer func() {
		if recover() != nil {
			fmt.Fprintln(os.Stderr, "owned genesis creation failed")
			os.Exit(1)
		}
	}()
	run()
}
