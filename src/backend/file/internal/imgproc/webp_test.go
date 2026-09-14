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

	grpcsvc "voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/store"
)

type memObjectStore struct {
	objects      map[string][]byte
	contentTypes map[string]string
	putKeys      []string
	deletedKeys  []string
	failPutAt    int
}

func (m *memObjectStore) ReadObject(_ context.Context, key string, _ int64) ([]byte, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, errString("missing key")
	}
	return data, nil
}

func (m *memObjectStore) PutObject(_ context.Context, key, contentType string, data []byte) error {
	m.putKeys = append(m.putKeys, key)
	if m.failPutAt > 0 && len(m.putKeys) == m.failPutAt {
		return errString("put failed")
	}
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	if m.contentTypes == nil {
		m.contentTypes = make(map[string]string)
	}
	m.objects[key] = append([]byte(nil), data...)
	m.contentTypes[key] = contentType
	return nil
}

func (m *memObjectStore) DeleteObject(_ context.Context, key string) error {
	m.deletedKeys = append(m.deletedKeys, key)
	delete(m.objects, key)
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

	proc := Processor{Reader: mem, Writer: mem, Deleter: mem}
	out, err := proc.ProcessImage(context.Background(), row)
	require.NoError(t, err)
	require.Equal(t, int32(640), out.Width)
	require.Equal(t, int32(480), out.Height)
	require.Contains(t, out.ConvertedR2Key, "full.webp")
	require.Contains(t, out.ThumbnailR2Key, "thumb.webp")
	require.NotEmpty(t, mem.objects[out.ConvertedR2Key])
	require.NotEmpty(t, mem.objects[out.ThumbnailR2Key])
	require.Equal(t, "image/webp", mem.contentTypes[out.ConvertedR2Key])
	require.Equal(t, "image/webp", mem.contentTypes[out.ThumbnailR2Key])
	require.True(t, isWebP(mem.objects[out.ConvertedR2Key]))
	require.True(t, isWebP(mem.objects[out.ThumbnailR2Key]))
	require.LessOrEqual(t, len(mem.objects[out.ConvertedR2Key]), maxProcessedImageBytes)
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

	proc := Processor{Reader: mem, Writer: mem, Deleter: mem}
	out, err := proc.ProcessImage(context.Background(), row)
	require.NoError(t, err)
	require.Equal(t, int32(64), out.Width)
	require.Equal(t, int32(48), out.Height)
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

func TestEncodeWebPWithinCap_AcceptsExactBoundaryAndRejectsOverflow(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	encoder := func(_ image.Image, _ float32) ([]byte, error) { return make([]byte, 10), nil }
	encoded, err := encodeWebPWithinCap(img, 85, 10, encoder)
	require.NoError(t, err)
	require.Len(t, encoded, 10)

	_, err = encodeWebPWithinCap(img, 85, 9, encoder)
	require.ErrorIs(t, err, grpcsvc.ErrProcessedOutputTooLarge)
}

func TestProcessor_ProcessImage_RejectsOversizedDerivativesBeforeWrites(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var raw bytes.Buffer
	require.NoError(t, png.Encode(&raw, img))
	mem := &memObjectStore{objects: map[string][]byte{"attachments/test/large.png": raw.Bytes()}}
	proc := Processor{
		Reader:  mem,
		Writer:  mem,
		Deleter: mem,
		encode: func(_ image.Image, _ float32) ([]byte, error) {
			return make([]byte, maxProcessedImageBytes+1), nil
		},
	}
	_, err := proc.ProcessImage(context.Background(), store.FileRow{ID: uuid.New(), R2Key: "attachments/test/large.png", SizeBytes: int64(raw.Len())})
	require.ErrorIs(t, err, grpcsvc.ErrProcessedOutputTooLarge)
	require.Empty(t, mem.putKeys)
	require.Empty(t, mem.deletedKeys)
}

func TestProcessor_ProcessImage_CleansBothDeterministicKeysAfterPartialWrite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var raw bytes.Buffer
	require.NoError(t, png.Encode(&raw, img))
	mem := &memObjectStore{objects: map[string][]byte{"attachments/test/partial.png": raw.Bytes()}, failPutAt: 2}
	fileID := uuid.New()
	proc := Processor{Reader: mem, Writer: mem, Deleter: mem}
	_, err := proc.ProcessImage(context.Background(), store.FileRow{ID: fileID, R2Key: "attachments/test/partial.png", SizeBytes: int64(raw.Len())})
	require.Error(t, err)
	require.Equal(t, []string{"processed/" + fileID.String() + "/full.webp", "processed/" + fileID.String() + "/thumb.webp"}, mem.deletedKeys)
	require.NotContains(t, mem.objects, "processed/"+fileID.String()+"/full.webp")
}
