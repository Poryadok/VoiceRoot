package pdfproc

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// Two pages with different aspect ratios prove the first-page-only contract.
func fixturePDF() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << >> >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 200] /Resources << >> >>",
	}
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

func TestRendererFirstPage(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm unavailable; runtime image must provide Poppler")
	}
	raw := fixturePDF()
	first, err := (Renderer{}).Render(context.Background(), raw)
	require.NoError(t, err)
	cfg, err := png.DecodeConfig(bytes.NewReader(first))
	require.NoError(t, err)
	require.Equal(t, 320, cfg.Width)
	require.Equal(t, 160, cfg.Height)
	again, err := (Renderer{}).Render(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, first, again)
	largePage := bytes.Replace(raw, []byte("200 100"), []byte("900 900"), 1)
	large, err := (Renderer{}).Render(context.Background(), largePage)
	require.NoError(t, err)
	cfg, err = png.DecodeConfig(bytes.NewReader(large))
	require.NoError(t, err)
	require.LessOrEqual(t, cfg.Width, MaxEdge)
	require.LessOrEqual(t, cfg.Height, MaxEdge)
}

func TestRendererRejectsInvalidAndOversized(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte("not a PDF"), bytes.Repeat([]byte("x"), MaxBytes+1)} {
		_, err := (Renderer{}).Render(context.Background(), raw)
		require.ErrorIs(t, err, ErrInvalidPDF)
	}
	if _, err := exec.LookPath("pdftoppm"); err == nil {
		_, err = (Renderer{}).Render(context.Background(), []byte("%PDF-1.4\nbroken"))
		require.ErrorIs(t, err, ErrInvalidPDF)
	}
}

func TestRendererCancellationAndMissingToolAreRetryable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (Renderer{}).Render(ctx, fixturePDF())
	require.ErrorIs(t, err, context.Canceled)
	_, err = (Renderer{Binary: "voice-missing-pdf-renderer"}).Render(context.Background(), fixturePDF())
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrInvalidPDF)
}

func TestOutputBound(t *testing.T) {
	var out limitedBuffer
	_, err := out.Write(make([]byte, MaxThumbnailBytes+1))
	require.Error(t, err)
	require.Zero(t, out.Len())
}
