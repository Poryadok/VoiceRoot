// nats-jwt-issuer creates persistent, protected staging/production Secret
// material from the reviewed ACL. It never emits private material to stdout.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"gopkg.in/yaml.v3"
)

var serviceNames = []string{"analytics", "auth", "bot", "chat", "file", "gateway", "matchmaking", "messaging", "moderation", "notification", "realtime", "role", "search", "social", "space", "story", "subscription", "user", "voice"}

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

type secret struct {
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   secretMetadata    `json:"metadata" yaml:"metadata"`
	Type       string            `json:"type" yaml:"type"`
	Data       map[string]string `json:"data" yaml:"data"`
}

type secretMetadata struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

func main() {
	namespace := flag.String("namespace", "", "voice-staging or voice-prod")
	cert := flag.String("tls-cert", "", "TLS certificate PEM with voice-nats SAN")
	key := flag.String("tls-key", "", "matching TLS private key PEM")
	ca := flag.String("tls-ca", "", "TLS CA PEM")
	flag.Parse()
	if flag.NArg() != 2 || *namespace == "" || *cert == "" || *key == "" || *ca == "" {
		fmt.Fprintln(os.Stderr, "usage: nats-jwt-issuer --namespace voice-staging --tls-cert CERT --tls-key KEY --tls-ca CA ACL_MANIFEST FRESH_ABSOLUTE_DESTINATION")
		os.Exit(2)
	}
	if err := issue(flag.Arg(0), flag.Arg(1), *namespace, *cert, *key, *ca); err != nil {
		fmt.Fprintln(os.Stderr, "NATS JWT issuance failed; no credentials printed")
		os.Exit(1)
	}
	fmt.Println("NATS JWT Secrets issued in protected destination")
}

func issue(aclPath, dest, namespace, certPath, keyPath, caPath string) error {
	if runtime.GOOS != "linux" {
		return errors.New("issuer requires Linux filesystem permissions")
	}
	if namespace != "voice-staging" && namespace != "voice-prod" {
		return errors.New("unsupported namespace")
	}
	if !filepath.IsAbs(dest) || filepath.Clean(dest) != dest {
		return errors.New("destination must be a clean absolute path")
	}
	if _, err := os.Lstat(dest); err == nil || !errors.Is(err, os.ErrNotExist) {
		return errors.New("destination already exists or cannot be checked")
	}
	acl, err := readPolicy(aclPath)
	if err != nil {
		return err
	}
	certPEM, keyPEM, caPEM, err := readAndVerifyTLS(certPath, keyPath, caPath)
	if err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(dest), ".voice-nats-issue-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := os.Chmod(staging, 0700); err != nil {
		return err
	}
	secrets, err := issueJWTs(staging, namespace, acl)
	if err != nil {
		return err
	}
	secrets = append(secrets, makeSecret("voice-nats-hub-tls", namespace, map[string][]byte{"tls.crt": certPEM, "tls.key": keyPEM, "ca.crt": caPEM}))
	for _, item := range secrets {
		contents, err := yaml.Marshal(item)
		if err != nil {
			return err
		}
		if err := writeProtected(filepath.Join(staging, item.Metadata.Name+".yaml"), contents); err != nil {
			return err
		}
	}
	list, err := json.MarshalIndent(struct {
		APIVersion string   `json:"apiVersion"`
		Kind       string   `json:"kind"`
		Items      []secret `json:"items"`
	}{"v1", "List", secrets}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeProtected(filepath.Join(staging, "secrets.json"), append(list, '\n')); err != nil {
		return err
	}
	if _, err := os.Lstat(dest); err == nil || !errors.Is(err, os.ErrNotExist) {
		return errors.New("destination appeared during issuance")
	}
	return os.Rename(staging, dest)
}

func readPolicy(path string) (policy, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return policy{}, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(contents)))
	decoder.KnownFields(true)
	var acl policy
	if err := decoder.Decode(&acl); err != nil {
		return policy{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return policy{}, errors.New("ACL must contain exactly one document")
	}
	if acl.Version != 1 || len(acl.Services) != len(serviceNames) {
		return policy{}, errors.New("ACL must have version one and exactly the deployed service set")
	}
	for _, name := range serviceNames {
		g, ok := acl.Services[name]
		if !ok || !validGrant(g, name, false) {
			return policy{}, fmt.Errorf("invalid grant for %s", name)
		}
	}
	if !validGrant(acl.Bootstrap, "bootstrap", true) {
		return policy{}, errors.New("invalid bootstrap grant")
	}
	return acl, nil
}

func validGrant(g grant, owner string, bootstrap bool) bool {
	if len(g.Publish)+len(g.Subscribe) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, subject := range g.Publish {
		if seen[subject] || subject == "" || strings.ContainsAny(subject, " \t\r\n*") || strings.HasSuffix(subject, ".>") && !validAck(subject) {
			return false
		}
		if strings.Contains(subject, ">") && !validAck(subject) {
			return false
		}
		if !bootstrap && (strings.HasPrefix(subject, "$JS.API.STREAM.CREATE.") || strings.HasPrefix(subject, "$JS.API.STREAM.UPDATE.") || strings.HasPrefix(subject, "$JS.API.STREAM.DELETE.") || strings.HasPrefix(subject, "$JS.API.CONSUMER.CREATE.") || strings.HasPrefix(subject, "$JS.API.CONSUMER.DURABLE.CREATE.") || strings.HasPrefix(subject, "$JS.API.CONSUMER.DELETE.")) {
			return false
		}
		seen[subject] = true
	}
	for _, subject := range g.Subscribe {
		if seen[subject] || subject == "" || strings.ContainsAny(subject, " \t\r\n*") {
			return false
		}
		if strings.Contains(subject, ">") {
			allowed := subject == "_INBOX.voice."+owner+".>" || owner == "auth" && subject == "_INBOX.voice.auth.requests.>" || bootstrap && subject == "_INBOX.voice.bootstrap.reply.>"
			if !allowed {
				return false
			}
		}
		seen[subject] = true
	}
	return true
}

func validAck(subject string) bool {
	if !strings.HasPrefix(subject, "$JS.ACK.") || !strings.HasSuffix(subject, ".>") || strings.Count(subject, ">") != 1 {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(subject, ".>"), ".")
	return len(parts) == 4 && parts[2] != "" && parts[3] != ""
}

func readAndVerifyTLS(certPath, keyPath, caPath string) ([]byte, []byte, []byte, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, nil, err
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, nil, nil, err
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil || len(pair.Certificate) == 0 {
		return nil, nil, nil, errors.New("TLS certificate and key do not match")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil || !slices.Contains(leaf.DNSNames, "voice-nats") {
		return nil, nil, nil, errors.New("TLS certificate lacks voice-nats DNS SAN")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, nil, nil, errors.New("TLS CA is invalid")
	}
	intermediates := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, nil, nil, err
		}
		intermediates.AddCert(cert)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "voice-nats", Roots: roots, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		return nil, nil, nil, errors.New("TLS chain or server name verification failed")
	}
	return certPEM, keyPEM, caPEM, nil
}

func issueJWTs(dir, namespace string, acl policy) ([]secret, error) {
	seedDir := filepath.Join(dir, "signing-seeds")
	if err := os.Mkdir(seedDir, 0700); err != nil {
		return nil, err
	}
	op, err := nkeys.CreateOperator()
	if err != nil {
		return nil, err
	}
	app, err := nkeys.CreateAccount()
	if err != nil {
		return nil, err
	}
	system, err := nkeys.CreateAccount()
	if err != nil {
		return nil, err
	}
	for path, pair := range map[string]nkeys.KeyPair{"operator.seed": op, "app-account.seed": app, "system-account.seed": system} {
		seed, err := pair.Seed()
		if err != nil {
			return nil, err
		}
		if err := writeProtected(filepath.Join(seedDir, path), append(seed, '\n')); err != nil {
			return nil, err
		}
	}
	opPub, _ := op.PublicKey()
	appPub, _ := app.PublicKey()
	systemPub, _ := system.PublicKey()
	opClaim := jwt.NewOperatorClaims(opPub)
	opClaim.SystemAccount = systemPub
	opJWT, err := opClaim.Encode(op)
	if err != nil {
		return nil, err
	}
	systemClaim := jwt.NewAccountClaims(systemPub)
	systemJWT, err := systemClaim.Encode(op)
	if err != nil {
		return nil, err
	}
	appClaim := jwt.NewAccountClaims(appPub)
	appClaim.Limits = jwt.OperatorLimits{
		AccountLimits: jwt.AccountLimits{Conn: 64, LeafNodeConn: 64},
		NatsLimits:    jwt.NatsLimits{Subs: 1024, Data: 64 << 20, Payload: 1 << 20},
		JetStreamLimits: jwt.JetStreamLimits{
			MemoryStorage: 64 << 20, DiskStorage: 512 << 20,
			Streams: 64, Consumer: 512, MaxAckPending: 4096,
		},
	}
	appJWT, err := appClaim.Encode(op)
	if err != nil {
		return nil, err
	}
	operator := makeSecret("voice-nats-operator", namespace, map[string][]byte{
		"operator.jwt": []byte(opJWT), "account.jwt": []byte(appJWT), "system-account.jwt": []byte(systemJWT),
		"account.public": []byte(appPub), "system-account.public": []byte(systemPub),
	})
	services := map[string][]byte{}
	for _, name := range serviceNames {
		creds, err := credential("voice-"+name, acl.Services[name], app)
		if err != nil {
			return nil, err
		}
		services[name+".creds"] = creds
	}
	bootstrapCreds, err := credential("voice-nats-bootstrap", acl.Bootstrap, app)
	if err != nil {
		return nil, err
	}
	return []secret{operator, makeSecret("voice-nats-bootstrap-credentials", namespace, map[string][]byte{"bootstrap.creds": bootstrapCreds}), makeSecret("voice-nats-service-credentials", namespace, services)}, nil
}

func credential(name string, permissions grant, account nkeys.KeyPair) ([]byte, error) {
	user, err := nkeys.CreateUser()
	if err != nil {
		return nil, err
	}
	pub, _ := user.PublicKey()
	claim := jwt.NewUserClaims(pub)
	claim.Name = name
	claim.Pub.Allow = append([]string(nil), permissions.Publish...)
	claim.Sub.Allow = append([]string(nil), permissions.Subscribe...)
	if !permissions.NoResponse {
		claim.Resp = &jwt.ResponsePermission{MaxMsgs: 16, Expires: time.Second}
	}
	if len(claim.Pub.Allow) == 0 {
		claim.Pub.Deny = []string{">"}
	}
	if len(claim.Sub.Allow) == 0 {
		claim.Sub.Deny = []string{">"}
	}
	token, err := claim.Encode(account)
	if err != nil {
		return nil, err
	}
	seed, err := user.Seed()
	if err != nil {
		return nil, err
	}
	return []byte("-----BEGIN NATS USER JWT-----\n" + token + "\n------END NATS USER JWT------\n\n" +
		"-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"), nil
}

func makeSecret(name, namespace string, entries map[string][]byte) secret {
	data := make(map[string]string, len(entries))
	for key, contents := range entries {
		data[key] = base64.StdEncoding.EncodeToString(contents)
	}
	return secret{APIVersion: "v1", Kind: "Secret", Metadata: secretMetadata{Name: name, Namespace: namespace}, Type: "Opaque", Data: data}
}

func writeProtected(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
