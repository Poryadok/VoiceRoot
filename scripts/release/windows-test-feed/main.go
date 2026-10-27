// windows-test-feed prepares disposable A5 update bundles, never installs them.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxInstaller = 512 << 20

type manifest struct {
	Schema    int    `json:"schema_version"`
	Signing   string `json:"signing"`
	Source    string `json:"source_sha"`
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	URL       string `json:"installer_url"`
	SHA       string `json:"installer_sha256"`
	Length    int    `json:"installer_length"`
	Signature string `json:"installer_ed25519"`
	FeedSHA   string `json:"appcast_sha256"`
}

var sourcePattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func digest(b []byte) string      { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func encoded(b []byte) string     { return base64.StdEncoding.EncodeToString(b) }
func canonical(m manifest) []byte { b, _ := json.Marshal(m); return append(b, '\n') }

func sandboxBase(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return false
	}
	if u.Hostname() != "127.0.0.1" && u.Hostname() != "[::1]" && u.Hostname() != "::1" {
		return false
	}
	return u.Path == "" && !strings.HasSuffix(raw, "/") && u.Opaque == ""
}

func feed(m manifest) []byte {
	// Fixed shape prevents unsigned installer arguments or extra feed entries.
	escape := func(s string) string { var b bytes.Buffer; _ = xml.EscapeText(&b, []byte(s)); return b.String() }
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
<channel><title>Voice disposable Windows test feed</title><item><title>Version %s</title>
<enclosure url="%s" sparkle:version="%s" sparkle:os="windows-x64" sparkle:edSignature="%s" length="%d" type="application/octet-stream" />
</item></channel></rss>
`, escape(m.Version), escape(m.URL), escape(m.Version), escape(m.Signature), m.Length))
}

func readRegular(path string, max int64) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > max {
		return nil, errors.New("invalid regular file size or type")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(st, opened) {
		return nil, errors.New("file changed while opening")
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if len(b) < 1 || int64(len(b)) > max {
		return nil, errors.New("invalid file size")
	}
	return b, nil
}

func build(installer, out, version, source, base string, key ed25519.PrivateKey) (string, error) {
	if !sourcePattern.MatchString(source) || source == strings.Repeat("0", 40) || !versionPattern.MatchString(version) || !sandboxBase(base) || len(key) != ed25519.PrivateKeySize {
		return "", errors.New("invalid source, numeric version, loopback base or key")
	}
	b, err := readRegular(installer, maxInstaller)
	if err != nil {
		return "", err
	}
	m := manifest{Schema: 1, Signing: "disposable-test-ed25519", Source: source, Version: version, Platform: "windows-x64", SHA: digest(b), Length: len(b), Signature: encoded(ed25519.Sign(key, b))}
	m.URL = base + "/" + m.SHA + "/VoiceSetup.exe"
	appcast := feed(m)
	m.FeedSHA = digest(appcast)
	raw := canonical(m)
	// Reserve the hash directory exclusively. Failure never changes an existing bundle.
	if err = os.MkdirAll(out, 0700); err != nil {
		return "", err
	}
	destination := filepath.Join(out, m.SHA)
	if err = os.Mkdir(destination, 0700); err != nil {
		return "", err
	}
	// A failed publication retains its reserved directory for inspection, never reusable.
	for _, member := range []struct {
		name string
		body []byte
	}{
		{"VoiceSetup.exe", b}, {"appcast.xml", appcast}, {"manifest.json", raw},
		{"manifest.sig", []byte(encoded(ed25519.Sign(key, raw)) + "\n")},
	} {
		path := filepath.Join(destination, member.name)
		f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return "", e
		}
		_, e = f.Write(member.body)
		closeErr := f.Close()
		if e != nil {
			return "", e
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	if err = verify(destination, source, key.Public().(ed25519.PublicKey)); err != nil {
		return "", err
	}
	return destination, nil
}

func verify(bundle, source string, public ed25519.PublicKey) error {
	invalid := errors.New("invalid test bundle")
	if !sourcePattern.MatchString(source) || source == strings.Repeat("0", 40) || len(public) != ed25519.PublicKeySize {
		return invalid
	}
	raw, err := readRegular(filepath.Join(bundle, "manifest.json"), 16384)
	if err != nil {
		return invalid
	}
	sigText, err := readRegular(filepath.Join(bundle, "manifest.sig"), 256)
	if err != nil {
		return invalid
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	if err != nil || !ed25519.Verify(public, raw, sig) {
		return invalid
	}
	var m manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&m) != nil || !bytes.Equal(raw, canonical(m)) {
		return invalid
	}
	if m.Schema != 1 || m.Signing != "disposable-test-ed25519" || m.Source != source || !versionPattern.MatchString(m.Version) || m.Platform != "windows-x64" || !digestPattern.MatchString(m.SHA) || m.Length < 1 || m.Length > maxInstaller {
		return invalid
	}
	suffix := "/" + m.SHA + "/VoiceSetup.exe"
	if !strings.HasSuffix(m.URL, suffix) || !sandboxBase(strings.TrimSuffix(m.URL, suffix)) || filepath.Base(filepath.Clean(bundle)) != m.SHA {
		return invalid
	}
	b, err := readRegular(filepath.Join(bundle, "VoiceSetup.exe"), maxInstaller)
	if err != nil || digest(b) != m.SHA || len(b) != m.Length {
		return invalid
	}
	artifactSig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || !ed25519.Verify(public, b, artifactSig) {
		return invalid
	}
	appcast, err := readRegular(filepath.Join(bundle, "appcast.xml"), 16384)
	if err != nil || digest(appcast) != m.FeedSHA || !bytes.Equal(appcast, feed(m)) {
		return invalid
	}
	return nil
}

func run(args []string, output io.Writer) error {
	if len(args) < 1 {
		return errors.New("expected keygen, build or verify")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	keyPath := flags.String("key", "", "path to disposable test seed (base64)")
	publicText := flags.String("public-key", "", "independently pinned public key (base64)")
	installer := flags.String("installer", "", "already built Windows installer")
	out := flags.String("out", "", "feed root directory")
	version := flags.String("version", "", "numeric x.y.z version")
	source := flags.String("source-sha", "", "independently resolved exact source SHA")
	base := flags.String("base-url", "", "loopback HTTP origin, e.g. http://127.0.0.1:18948")
	bundle := flags.String("bundle", "", "immutable hash directory")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return errors.New("invalid arguments")
	}
	switch args[0] {
	case "keygen":
		if *keyPath == "" {
			return errors.New("key path required")
		}
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(*keyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = f.WriteString(encoded(private.Seed()) + "\n")
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		_, err = fmt.Fprintln(output, encoded(public))
		return err
	case "build":
		if *out == "" || *keyPath == "" {
			return errors.New("output and disposable key path required")
		}
		text, err := readRegular(*keyPath, 256)
		if err != nil {
			return errors.New("cannot read test key")
		}
		seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(text)))
		if err != nil || len(seed) != ed25519.SeedSize {
			return errors.New("invalid test seed")
		}
		path, err := build(*installer, *out, *version, *source, *base, ed25519.NewKeyFromSeed(seed))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, path)
		return err
	case "verify":
		public, err := base64.StdEncoding.DecodeString(*publicText)
		if err != nil {
			return errors.New("invalid pinned public key")
		}
		if err = verify(*bundle, *source, ed25519.PublicKey(public)); err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, `{"result":"PASS","scope":"offline test package integrity only"}`)
		return err
	default:
		return errors.New("expected keygen, build or verify")
	}
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "windows-test-feed:", err)
		os.Exit(1)
	}
}
