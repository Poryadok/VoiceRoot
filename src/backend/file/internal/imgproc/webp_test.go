package imgproc

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/file/internal/store"
)

type memObjectStore struct {
	objects map[string][]byte
}

func (m *memObjectStore) ReadObject(_ context.Context, key string, _ int64) ([]byte, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, errString("missing key")
	}
	return data, nil
}

func (m *memObjectStore) PutObject(_ context.Context, key, _ string, data []byte) error {
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	m.objects[key] = append([]byte(nil), data...)
	return nil
}

func TestProcessor_ProcessImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for x := 0; x < 640; x++ {
		img.Set(x, 100, color.RGBA{R: 200, G: 50, B: 50, A: 255})
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	storeKey := "attachments/test/photo.png"
	mem := &memObjectStore{objects: map[string][]byte{storeKey: buf.Bytes()}}
	row := store.FileRow{
		ID:        uuid.MustParse("01020304-0506-0708-090a-0b0c0d0e0f10"),
		R2Key:     storeKey,
		SizeBytes: int64(buf.Len()),
	}

	proc := Processor{Reader: mem, Writer: mem}
	out, err := proc.ProcessImage(context.Background(), row)
	require.NoError(t, err)
	require.Equal(t, int32(640), out.Width)
	require.Equal(t, int32(480), out.Height)
	require.Contains(t, out.ConvertedR2Key, "full.webp")
	require.Contains(t, out.ThumbnailR2Key, "thumb.webp")
	require.NotEmpty(t, mem.objects[out.ConvertedR2Key])
	require.NotEmpty(t, mem.objects[out.ThumbnailR2Key])
}

func TestProcessor_ProcessImage_requiresReaderAndWriter(t *testing.T) {
	proc := Processor{}
	_, err := proc.ProcessImage(context.Background(), store.FileRow{})
	require.Error(t, err)
}

func TestProcessor_ProcessImage_smallImageKeepsThumbDimensions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	storeKey := "attachments/test/small.png"
	mem := &memObjectStore{objects: map[string][]byte{storeKey: buf.Bytes()}}
	row := store.FileRow{
		ID:        uuid.MustParse("01020304-0506-0708-090a-0b0c0d0e0f11"),
		R2Key:     storeKey,
		SizeBytes: int64(buf.Len()),
	}

	proc := Processor{Reader: mem, Writer: mem}
	out, err := proc.ProcessImage(context.Background(), row)
	require.NoError(t, err)
	require.Equal(t, int32(64), out.Width)
	require.Equal(t, int32(48), out.Height)
}
