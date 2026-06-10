package imgproc

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/png"
	"image/jpeg"
	"math"

	"golang.org/x/image/draw"

	grpcsvc "voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/store"
)

const thumbMaxEdge = 320

// ObjectWriter uploads processed bytes to object storage.
type ObjectWriter interface {
	PutObject(ctx context.Context, key, contentType string, data []byte) error
}

// ObjectReader reads uploaded object bytes from storage.
type ObjectReader interface {
	ReadObject(ctx context.Context, key string, maxBytes int64) ([]byte, error)
}

// Processor reads the original image and writes optimized full + thumbnail objects.
// Encodes as JPEG (pure Go, no CGO) with .webp key suffix for API compatibility with Phase 3 paths.
type Processor struct {
	Reader ObjectReader
	Writer ObjectWriter
}

func (p Processor) ProcessImage(ctx context.Context, row store.FileRow) (grpcsvc.ImageProcessingResult, error) {
	if p.Reader == nil || p.Writer == nil {
		return grpcsvc.ImageProcessingResult{}, errNotConfigured
	}
	raw, err := p.Reader.ReadObject(ctx, row.R2Key, row.SizeBytes)
	if err != nil {
		return grpcsvc.ImageProcessingResult{}, err
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return grpcsvc.ImageProcessingResult{}, err
	}
	bounds := src.Bounds()
	width := int32(bounds.Dx())
	height := int32(bounds.Dy())

	prefix := "processed/" + row.ID.String()
	fullKey := prefix + "/full.webp"
	thumbKey := prefix + "/thumb.webp"

	fullBytes, err := encodeJPEG(src, 85)
	if err != nil {
		return grpcsvc.ImageProcessingResult{}, err
	}
	if err := p.Writer.PutObject(ctx, fullKey, "image/jpeg", fullBytes); err != nil {
		return grpcsvc.ImageProcessingResult{}, err
	}

	thumb := resizeThumb(src, thumbMaxEdge)
	thumbBytes, err := encodeJPEG(thumb, 80)
	if err != nil {
		return grpcsvc.ImageProcessingResult{}, err
	}
	if err := p.Writer.PutObject(ctx, thumbKey, "image/jpeg", thumbBytes); err != nil {
		return grpcsvc.ImageProcessingResult{}, err
	}

	return grpcsvc.ImageProcessingResult{
		ConvertedR2Key: fullKey,
		ThumbnailR2Key: thumbKey,
		Width:          width,
		Height:         height,
	}, nil
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func resizeThumb(src image.Image, maxEdge int) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxEdge && h <= maxEdge {
		return src
	}
	scale := math.Min(float64(maxEdge)/float64(w), float64(maxEdge)/float64(h))
	nw := int(math.Max(1, math.Round(float64(w)*scale)))
	nh := int(math.Max(1, math.Round(float64(h)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}

var errNotConfigured = errString("image processor: reader and writer required")

type errString string

func (e errString) Error() string { return string(e) }
