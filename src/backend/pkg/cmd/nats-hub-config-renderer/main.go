// nats-hub-config-renderer validates hub identity material and renders the
// MEMORY resolver configuration without placing account JWTs in a ConfigMap.
package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

type configInput struct {
	OperatorJWT  string
	AppJWT       string
	SystemJWT    string
	AppPublic    string
	SystemPublic string
	Template     string
}

func writeAtomic(path, contents string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("renderer output directory unavailable")
	}
	tmp, err := os.CreateTemp(dir, ".nats.conf-")
	if err != nil {
		return errors.New("renderer output unavailable")
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0400); err != nil {
		tmp.Close()
		return errors.New("renderer output mode unavailable")
	}
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		return errors.New("renderer output unavailable")
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return errors.New("renderer output unavailable")
	}
	if err := tmp.Close(); err != nil {
		return errors.New("renderer output unavailable")
	}
	if err := os.Rename(tmpName, path); err != nil {
		return errors.New("renderer output unavailable")
	}
	if err := os.Chmod(path, 0400); err != nil {
		return errors.New("renderer output mode unavailable")
	}
	return nil
}

func renderAndWrite(in configInput, path string, validate func(string) error) error {
	contents, err := render(in)
	if err != nil {
		return err
	}
	if err := writeAtomic(path, contents); err != nil {
		return err
	}
	if err := validate(path); err != nil {
		_ = os.Remove(path)
		return errors.New("rendered hub config is invalid")
	}
	return nil
}

func validateNATSConfig(path string) error {
	cmd := exec.Command("/nats-server", "-t", "-c", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return errors.New("nats config validation failed")
	}
	return nil
}

func render(in configInput) (string, error) {
	if in.OperatorJWT == "" || in.AppJWT == "" || in.SystemJWT == "" || in.AppPublic == "" || in.SystemPublic == "" {
		return "", errors.New("hub identity material is incomplete")
	}
	appPublic, systemPublic := strings.TrimSpace(in.AppPublic), strings.TrimSpace(in.SystemPublic)
	if appPublic == systemPublic || !strings.HasPrefix(appPublic, "A") || !strings.HasPrefix(systemPublic, "A") {
		return "", errors.New("hub account identities are invalid")
	}
	op, err := jwt.DecodeOperatorClaims(strings.TrimSpace(in.OperatorJWT))
	if err != nil {
		return "", errors.New("hub operator JWT is invalid")
	}
	if err := verifyJWT(strings.TrimSpace(in.OperatorJWT), op.Subject); err != nil || invalidClaims(op) {
		return "", errors.New("hub operator JWT is invalid")
	}
	app, err := jwt.DecodeAccountClaims(strings.TrimSpace(in.AppJWT))
	if err != nil || invalidClaims(app) || verifyJWT(strings.TrimSpace(in.AppJWT), op.Subject) != nil || app.Subject != appPublic || app.Issuer != op.Subject || !app.Limits.IsJSEnabled() {
		return "", errors.New("hub application account JWT is invalid")
	}
	sys, err := jwt.DecodeAccountClaims(strings.TrimSpace(in.SystemJWT))
	if err != nil || invalidClaims(sys) || verifyJWT(strings.TrimSpace(in.SystemJWT), op.Subject) != nil || op.SystemAccount != systemPublic || sys.Subject != systemPublic || sys.Issuer != op.Subject || sys.Limits.IsJSEnabled() {
		return "", errors.New("hub system account JWT is invalid")
	}
	if strings.Count(in.Template, "__NATS_APP_RESOLVER_PRELOAD__") != 1 || strings.Count(in.Template, "__NATS_SYS_RESOLVER_PRELOAD__") != 1 || strings.Count(in.Template, "__NATS_SYSTEM_ACCOUNT__") != 1 {
		return "", errors.New("hub config template is invalid")
	}
	out := strings.ReplaceAll(in.Template, "__NATS_APP_RESOLVER_PRELOAD__", fmt.Sprintf("%s: %q", appPublic, strings.TrimSpace(in.AppJWT)))
	out = strings.ReplaceAll(out, "__NATS_SYS_RESOLVER_PRELOAD__", fmt.Sprintf("%s: %q", systemPublic, strings.TrimSpace(in.SystemJWT)))
	return strings.ReplaceAll(out, "__NATS_SYSTEM_ACCOUNT__", systemPublic), nil
}

type validatable interface{ Validate(*jwt.ValidationResults) }

func invalidClaims(c validatable) bool {
	v := jwt.CreateValidationResults()
	c.Validate(v)
	return v.IsBlocking(true)
}

func verifyJWT(token, signer string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("invalid JWT")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}
	key, err := nkeys.FromPublicKey(signer)
	if err != nil {
		return err
	}
	return key.Verify([]byte(parts[0]+"."+parts[1]), sig)
}

func main() {
	if len(os.Args) != 8 {
		os.Exit(2)
	}
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return string(b)
	}
	err := renderAndWrite(configInput{OperatorJWT: read(os.Args[1]), AppJWT: read(os.Args[2]), SystemJWT: read(os.Args[3]), AppPublic: read(os.Args[4]), SystemPublic: read(os.Args[5]), Template: read(os.Args[6])}, os.Args[7], validateNATSConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nats hub config rendering failed")
		os.Exit(1)
	}
}
