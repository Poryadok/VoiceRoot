package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voice/backend/space/internal/nativeinstaller"
)

func main() {
	mode := flag.String("mode", "provision", "provision, verify, inspect-legacy24, upgrade-legacy24, or verify-legacy24")
	dir := flag.String("migrations-dir", "/migrations", "selected immutable Space SQL directory")
	driver := flag.String("migrate", "/usr/local/bin/migrate", "pinned migrate executable")
	prior := flag.String("prior-evidence", "", "actual completed JSON-lines deployment evidence (verify only)")
	origin := flag.String("origin-evidence", "", "private original legacy24 sidecar; created exclusively by upgrade, retained for verify")
	operation := flag.String("operation-id", "", "owning controller operation correlation (legacy24 upgrade only)")
	snapshot := flag.String("snapshot-id", "", "owning controller completed backup snapshot correlation (legacy24 upgrade only)")
	flag.Parse()
	c := nativeinstaller.Config{Mode: *mode, MigrationsDir: *dir, MigratePath: *driver, AdminURL: os.Getenv("SPACE_NATIVE_ADMIN_URL"), Output: os.Stdout, RequiredBuildSHA256: os.Getenv("SPACE_NATIVE_REQUIRED_SOURCE_SHA256")}
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected installer arguments")
		os.Exit(2)
	}
	legacy := *mode == "upgrade-legacy24" || *mode == "verify-legacy24" || *mode == "inspect-legacy24"
	if (!legacy && (*origin != "" || *operation != "" || *snapshot != "")) ||
		(*mode == "upgrade-legacy24" && *prior != "") ||
		(*mode == "inspect-legacy24" && (*origin != "" || *operation != "" || *snapshot != "" || *prior != "")) ||
		(*mode == "verify-legacy24" && (*operation != "" || *snapshot != "" || *prior == "")) {
		fmt.Fprintln(os.Stderr, "evidence arguments do not match the explicit installer mode")
		os.Exit(2)
	}
	var legacyPrior *nativeinstaller.Legacy24Receipt
	if *prior != "" {
		f, err := os.Open(*prior)
		if err != nil {
			fmt.Fprintln(os.Stderr, "retained evidence unavailable")
			os.Exit(1)
		}
		if *mode == "verify-legacy24" {
			legacyPrior, err = readLegacy24ReceiptAndClose(f)
		} else {
			c.Prior, err = readReceiptAndClose(f)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var err error
	switch *mode {
	case "inspect-legacy24":
		c.Mode = ""
		err = nativeinstaller.InspectLegacy24(ctx, c)
	case "upgrade-legacy24":
		c.Mode = ""
		err = nativeinstaller.RunLegacy24Upgrade(ctx, nativeinstaller.Legacy24Config{Config: c, OperationID: *operation, SnapshotID: *snapshot, OriginEvidencePath: *origin})
	case "verify-legacy24":
		c.Mode = ""
		err = nativeinstaller.VerifyLegacy24Upgrade(ctx, nativeinstaller.Legacy24VerifyConfig{Config: c, PriorLegacy24: legacyPrior, OriginEvidencePath: *origin})
	default:
		err = nativeinstaller.Run(ctx, c)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readLegacy24ReceiptAndClose(in io.ReadCloser) (receipt *nativeinstaller.Legacy24Receipt, err error) {
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, evidenceCloseFailure{cause: closeErr})
			receipt = nil
		}
	}()
	return nativeinstaller.ReadLegacy24Receipt(in)
}

type evidenceCloseFailure struct{ cause error }

func (e evidenceCloseFailure) Error() string { return "retained evidence close failed" }
func (e evidenceCloseFailure) Unwrap() error { return e.cause }

func readReceiptAndClose(in io.ReadCloser) (receipt *nativeinstaller.Receipt, err error) {
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, evidenceCloseFailure{cause: closeErr})
			receipt = nil
		}
	}()
	return nativeinstaller.ReadReceipt(in)
}
