package gifproc

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The real os/exec pipe path must obey the cap, including io.Copy optimizations.
func TestSubprocessCaptureBound(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			out, runErr := run(ctx, exe, "-test.run=^TestSubprocessCaptureHelper$", "--", "gif-capture-helper="+stream)
			if ctx.Err() != nil {
				t.Fatalf("child did not drain and exit: %v", ctx.Err())
			}
			if stream == "stdout" {
				if runErr != nil {
					t.Fatal(runErr)
				}
				if len(out) != maxLogBytes || !bytes.Equal(out, bytes.Repeat([]byte{'Z'}, maxLogBytes)) {
					t.Fatalf("retained stdout bytes=%d, want capped %d", len(out), maxLogBytes)
				}
				return
			}
			var exit *exec.ExitError
			if !errors.As(runErr, &exit) || exit.ExitCode() != 7 {
				t.Fatalf("expected genuine child exit7, got %v", runErr)
			}
			if len(runErr.Error()) > maxLogBytes+len(exe)+128 || strings.Contains(runErr.Error(), "uncapped-tail") {
				t.Fatalf("retained stderr error bytes=%d exceed bounded diagnostic", len(runErr.Error()))
			}
			if len(out) != 0 {
				t.Fatalf("failed command returned stdout bytes=%d", len(out))
			}
		})
	}
}

func TestSubprocessCaptureHelper(t *testing.T) {
	if len(os.Args) == 0 || !strings.HasPrefix(os.Args[len(os.Args)-1], "gif-capture-helper=") {
		return
	}
	stream := strings.TrimPrefix(os.Args[len(os.Args)-1], "gif-capture-helper=")
	output := os.Stdout
	code := 0
	if stream == "stderr" {
		output, code = os.Stderr, 7
	}
	if _, err := output.Write(append(bytes.Repeat([]byte{'Z'}, maxLogBytes*4), []byte("uncapped-tail")...)); err != nil {
		os.Exit(8)
	}
	os.Exit(code)
}
