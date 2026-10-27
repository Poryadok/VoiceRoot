package pdfproc

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The child exits before Go's flag parser sees Poppler's fixed argv. Production
// code uses the same exec.CommandContext path as the real renderer.
func init() {
	mode := os.Getenv("VOICE_PDF_TEST_CHILD")
	if mode == "" {
		return
	}
	if mode == "wait" {
		if err := os.WriteFile(os.Getenv("VOICE_PDF_TEST_READY"), []byte("started"), 0600); err != nil {
			os.Exit(99)
		}
		for {
			time.Sleep(time.Second)
		}
	}
	if mode == "kill" {
		process, err := os.FindProcess(os.Getpid())
		if err == nil {
			_ = process.Kill()
		}
		os.Exit(99)
	}
	if mode == "bad-output" {
		_, _ = os.Stdout.Write([]byte("not a PNG"))
		os.Exit(0)
	}
	if mode == "large-output" {
		_, _ = os.Stdout.Write(make([]byte, MaxThumbnailBytes+1))
		os.Exit(1)
	}
	code, err := strconv.Atoi(mode)
	if err != nil {
		os.Exit(99)
	}
	os.Exit(code)
}

func TestRendererDistinguishesPDFErrorsFromToolFailures(t *testing.T) {
	for _, tc := range []struct {
		mode    string
		invalid bool
	}{
		{"1", true}, {"3", true}, {"2", false}, {"99", false}, {"42", false}, {"kill", false}, {"bad-output", false}, {"large-output", false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			if tc.mode == "kill" && runtime.GOOS == "windows" {
				t.Skip("Windows TerminateProcess returns exit 1, indistinguishable from an explicit exit; production SIGKILL proof requires Linux")
			}
			t.Setenv("VOICE_PDF_TEST_CHILD", tc.mode)
			_, err := (Renderer{Binary: os.Args[0]}).Render(context.Background(), fixturePDF())
			require.Error(t, err)
			if tc.invalid {
				require.ErrorIs(t, err, ErrInvalidPDF)
			} else {
				require.NotErrorIs(t, err, ErrInvalidPDF)
			}
		})
	}
}

func TestRendererCancelsAnActiveChild(t *testing.T) {
	t.Setenv("VOICE_PDF_TEST_CHILD", "wait")
	ready := filepath.Join(t.TempDir(), "ready")
	t.Setenv("VOICE_PDF_TEST_READY", ready)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := (Renderer{Binary: os.Args[0]}).Render(ctx, fixturePDF()); result <- err }()
	require.Eventually(t, func() bool { _, err := os.Stat(ready); return err == nil }, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("active renderer did not settle after cancellation")
	}
}
