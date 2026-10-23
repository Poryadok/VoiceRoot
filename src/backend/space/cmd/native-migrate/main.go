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
	mode := flag.String("mode", "provision", "provision new target, or verify retained completed controller evidence")
	dir := flag.String("migrations-dir", "/migrations", "selected immutable Space SQL directory")
	driver := flag.String("migrate", "/usr/local/bin/migrate", "pinned migrate executable")
	prior := flag.String("prior-evidence", "", "actual completed JSON-lines deployment evidence (verify only)")
	flag.Parse()
	c := nativeinstaller.Config{Mode: *mode, MigrationsDir: *dir, MigratePath: *driver, AdminURL: os.Getenv("SPACE_NATIVE_ADMIN_URL"), Output: os.Stdout, RequiredBuildSHA256: os.Getenv("SPACE_NATIVE_REQUIRED_SOURCE_SHA256")}
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected installer arguments")
		os.Exit(2)
	}
	if *prior != "" {
		f, err := os.Open(*prior)
		if err != nil {
			fmt.Fprintln(os.Stderr, "retained evidence unavailable")
			os.Exit(1)
		}
		c.Prior, err = readReceiptAndClose(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := nativeinstaller.Run(ctx, c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
