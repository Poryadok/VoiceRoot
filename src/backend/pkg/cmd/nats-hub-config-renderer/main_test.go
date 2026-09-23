package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

func TestRenderRejectsEmptyInputs(t *testing.T) {
	if _, err := render(configInput{}); err == nil {
		t.Fatal("renderer must fail closed when hub JWT inputs are absent")
	}
}

func TestWriteAtomicUsesOwnerReadOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not preserve POSIX file modes")
	}
	path := filepath.Join(t.TempDir(), "rendered", "nats.conf")
	if err := writeAtomic(path, "resolver: MEMORY\n"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0400 {
		t.Fatalf("rendered mode = %v, err = %v", info.Mode(), err)
	}
}

func TestRenderAndWriteRemovesConfigWhenValidationFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rendered", "nats.conf")
	err := renderAndWrite(validInput(t), path, func(string) error { return errors.New("invalid") })
	if err == nil {
		t.Fatal("renderer must fail when config validation fails")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("invalid rendered config must be removed, stat error = %v", statErr)
	}
}

func TestRenderPreloadsOnlyDistinctSignedAccounts(t *testing.T) {
	in := validInput(t)
	config, err := render(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(config, "__NATS_") || !strings.Contains(config, in.AppPublic+": ") || !strings.Contains(config, in.SystemPublic+": ") {
		t.Fatalf("resolver preload markers were not replaced exactly: %q", config)
	}
	if !strings.Contains(config, "system_account: "+in.SystemPublic) {
		t.Fatal("system account must be the distinct SYS public key")
	}
}

func TestRenderRejectsModifiedAccountSignature(t *testing.T) {
	in := validInput(t)
	in.AppJWT = in.AppJWT[:len(in.AppJWT)-1] + "x"
	if _, err := render(in); err == nil {
		t.Fatal("modified account JWT signature must fail closed")
	}
}

func validInput(t *testing.T) configInput {
	t.Helper()
	op, err := nkeys.CreateOperator()
	if err != nil {
		t.Fatal(err)
	}
	opPublic, _ := op.PublicKey()
	systemKey, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	systemPublic, _ := systemKey.PublicKey()
	opClaim := jwt.NewOperatorClaims(opPublic)
	opClaim.SystemAccount = systemPublic
	opJWT, err := opClaim.Encode(op)
	if err != nil {
		t.Fatal(err)
	}
	appKey, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	appPublic, _ := appKey.PublicKey()
	app := jwt.NewAccountClaims(appPublic)
	app.Limits = jwt.OperatorLimits{JetStreamLimits: jwt.JetStreamLimits{MemoryStorage: 1, DiskStorage: 1, Streams: 1, Consumer: 1, MaxAckPending: 1}}
	appJWT, err := app.Encode(op)
	if err != nil {
		t.Fatal(err)
	}
	systemJWT, err := jwt.NewAccountClaims(systemPublic).Encode(op)
	if err != nil {
		t.Fatal(err)
	}
	return configInput{
		OperatorJWT: opJWT, AppJWT: appJWT, SystemJWT: systemJWT,
		AppPublic: appPublic, SystemPublic: systemPublic,
		Template: "resolver_preload: {\n__NATS_APP_RESOLVER_PRELOAD__\n__NATS_SYS_RESOLVER_PRELOAD__\n}\nsystem_account: __NATS_SYSTEM_ACCOUNT__\n",
	}
}
