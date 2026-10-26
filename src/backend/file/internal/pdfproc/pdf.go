// Package pdfproc renders only the first page of bounded plaintext PDFs.
package pdfproc

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"io"
	"os"
	"os/exec"
	"time"
)

const MaxBytes = 10 * 1024 * 1024
const MaxThumbnailBytes = 1024 * 1024
const MaxEdge = 320

var ErrInvalidPDF = errors.New("invalid PDF preview")
var renderSlots = make(chan struct{}, 1)

type Renderer struct{ Binary string }

// Render uses fixed first-page and pixel limits. It neither executes PDF actions
// nor follows document links. Tool stderr and filenames never reach the caller.
func (p Renderer) Render(ctx context.Context, raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw) > MaxBytes || !bytes.HasPrefix(raw, []byte("%PDF-")) {
		return nil, ErrInvalidPDF
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case renderSlots <- struct{}{}:
		defer func() { <-renderSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp("", "voice-pdf-*.pdf")
	if err != nil {
		return nil, errors.New("PDF temporary storage unavailable")
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(raw); err != nil {
		_ = f.Close()
		return nil, errors.New("PDF temporary storage unavailable")
	}
	if err = f.Close(); err != nil {
		return nil, errors.New("PDF temporary storage unavailable")
	}
	binary := p.Binary
	if binary == "" {
		binary = "pdftoppm"
	}
	cmd := exec.CommandContext(ctx, binary, "-f", "1", "-l", "1", "-singlefile", "-scale-to", "320", "-png", f.Name())
	var out limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err = cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, ErrInvalidPDF
		}
		return nil, errors.New("PDF renderer unavailable")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(out.Bytes()))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > MaxEdge || cfg.Height > MaxEdge {
		return nil, ErrInvalidPDF
	}
	return out.Bytes(), nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > MaxThumbnailBytes {
		return 0, ErrInvalidPDF
	}
	return b.Buffer.Write(p)
}
