package imgproc

import (
	"bytes"
	"context"
	"encoding/base64"
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

func TestProcessor_ProcessImage_DecodableFormats(t *testing.T) {
	// Keep JPEG as static bytes: importing image/jpeg in a test would register
	// its decoder and hide a missing production decoder registration.
	jpegBytes, err := base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAMCAgMCAgMDAwMEAwMEBQgFBQQEBQoHBwYIDAoMDAsKCwsNDhIQDQ4RDgsLEBYQERMUFRUVDA8XGBYUGBIUFRT/2wBDAQMEBAUEBQkFBQkUDQsNFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBT/wAARCAADAAQDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwDwyiiivxw/rM//2Q==")
	require.NoError(t, err)
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	var pngBytes bytes.Buffer
	require.NoError(t, png.Encode(&pngBytes, img))
	webpBytes, err := encodeWebP(img, defaultFullWebPQuality)
	require.NoError(t, err)

	for _, tc := range []struct {
		name        string
		data        []byte
		full, thumb image.Point
	}{
		{"JPEG", jpegBytes, image.Pt(4, 3), image.Pt(4, 3)},
		{"PNG", pngBytes.Bytes(), image.Pt(640, 480), image.Pt(320, 240)},
		{"WebP", webpBytes, image.Pt(640, 480), image.Pt(320, 240)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const key = "attachments/test/input"
			mem := &memObjectStore{objects: map[string][]byte{key: tc.data}}
			out, err := (Processor{Reader: mem, Writer: mem, Deleter: mem}).ProcessImage(context.Background(), store.FileRow{ID: uuid.New(), R2Key: key, SizeBytes: int64(len(tc.data))})
			require.NoError(t, err)
			require.Equal(t, int32(tc.full.X), out.Width)
			require.Equal(t, int32(tc.full.Y), out.Height)
			for objectKey, dimensions := range map[string]image.Point{out.ConvertedR2Key: tc.full, out.ThumbnailR2Key: tc.thumb} {
				require.Equal(t, "image/webp", mem.contentTypes[objectKey])
				require.True(t, isWebP(mem.objects[objectKey]))
				decoded, format, err := image.Decode(bytes.NewReader(mem.objects[objectKey]))
				require.NoError(t, err)
				require.Equal(t, "webp", format)
				require.Equal(t, dimensions, decoded.Bounds().Size())
			}
		})
	}
}

func TestProcessor_ProcessImage_MalformedInputDoesNotWrite(t *testing.T) {
	const key = "attachments/test/broken"
	for _, raw := range [][]byte{nil, []byte("not an image"), []byte("RIFF\x00\x00\x00\x00WEBP")} {
		mem := &memObjectStore{objects: map[string][]byte{key: raw}}
		_, err := (Processor{Reader: mem, Writer: mem, Deleter: mem}).ProcessImage(context.Background(), store.FileRow{ID: uuid.New(), R2Key: key, SizeBytes: int64(len(raw))})
		require.Error(t, err)
		require.Empty(t, mem.putKeys)
		require.Empty(t, mem.deletedKeys)
	}
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
