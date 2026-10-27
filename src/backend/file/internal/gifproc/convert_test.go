package gifproc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func fixture(t *testing.T, delays []int, disposal []byte) []byte {
	t.Helper()
	pal := color.Palette{color.Transparent, color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	g := &gif.GIF{Config: image.Config{ColorModel: pal, Width: 65, Height: 49}, Delay: delays, Disposal: disposal}
	for i := range delays {
		r := image.Rect(0, 0, 65, 49)
		if i > 0 {
			r = image.Rect(16, 16, 48, 40)
		}
		if i == 2 {
			r = image.Rect(24, 24, 40, 36)
		}
		if i == 3 {
			r = image.Rect(16, 16, 24, 24)
		}
		f := image.NewPaletted(r, pal)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				f.SetColorIndex(x, y, uint8(i%3+1))
			}
		}
		if i == 0 {
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					f.SetColorIndex(x, y, 0)
				}
			}
		}
		g.Image = append(g.Image, f)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRejectInvalidAndBoundedInput(t *testing.T) {
	raw := fixture(t, []int{20}, []byte{gif.DisposalNone})
	oversized := append([]byte(nil), raw...)
	binary.LittleEndian.PutUint16(oversized[6:8], 4097)
	malformed := append([]byte(nil), raw...)
	malformed = malformed[:len(malformed)-3]
	duration := fixture(t, []int{12001}, []byte{gif.DisposalNone})
	frames := fixture(t, make([]int, 601), make([]byte, 601))
	canvasBudget := fixture(t, make([]int, 5), make([]byte, 5))
	binary.LittleEndian.PutUint16(canvasBudget[6:8], 4096)
	binary.LittleEndian.PutUint16(canvasBudget[8:10], 4096)
	for _, tc := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"not GIF", []byte("not a GIF"), ErrInvalid}, {"truncated", malformed, ErrInvalid},
		{"dimensions", oversized, ErrTooLarge}, {"duration", duration, ErrTooLarge}, {"frames", frames, ErrTooLarge},
		{"cumulative logical canvas", canvasBudget, ErrTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Convert(context.Background(), tc.raw)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Convert(ctx, raw); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestRealCodecRunningProcessCancellation(t *testing.T) {
	codecAvailable(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	path := filepath.Join(t.TempDir(), "cancelled.mp4")
	err := runOutput(ctx, path, maxOutputBytes, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=64x64:rate=100", "-t", "3600", "-c:v", "libx264", "-threads", "1", path)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("running process cancellation: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("child was not promptly reaped: %s", time.Since(start))
	}
}

func TestOutputByteLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.mp4")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate((5 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = readCapped(path); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized derivative: %v", err)
	}
}

func TestRealCodecOutputCapIsPermanent(t *testing.T) {
	codecAvailable(t)
	path := filepath.Join(t.TempDir(), "capped.mp4")
	// Exercise the actual output supervisor with a smaller test budget. No fake
	// binary: ffmpeg creates an ordinary long H264 MP4 and exceeds that budget.
	err := runOutput(context.Background(), path, 8<<10, "-nostdin", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=256x256:rate=30", "-t", "20", "-an", "-c:v", "libx264", "-threads", "1", "-preset", "veryfast", "-bf", "0", "-movflags", "+faststart", path)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("cap must be permanent, not a successful truncated movie or transient duration error: %v", err)
	}
	// A genuine codec invocation error below the cap must remain retryable.
	err = runOutput(context.Background(), filepath.Join(t.TempDir(), "unsupported.mp4"), 5<<20, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=16x16", "-t", "1", "-c:v", "voice_nonexistent_codec", filepath.Join(t.TempDir(), "bad.mp4"))
	if err == nil || errors.Is(err, ErrTooLarge) || errors.Is(err, ErrInvalid) {
		t.Fatalf("codec failure must be retryable: %v", err)
	}
}

func codecAvailable(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			if os.Getenv("GIFPROC_REAL_CODEC") == "1" {
				t.Fatal(err)
			}
			t.Skip("real codec required; see README.md")
		}
	}
}

func command(t *testing.T, name string, args ...string) []byte {
	t.Helper()
	b, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v: %s", name, err, b)
	}
	return b
}

func TestRealCodecAnimationTimingAndDisposal(t *testing.T) {
	codecAvailable(t)
	// Positive centisecond delays must remain variable, including the last hold.
	raw := fixture(t, []int{20, 30, 50, 10}, []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalBackground, gif.DisposalNone})
	out, err := Convert(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 66 || out.Height != 50 || out.DurationSeconds != 2 {
		t.Fatalf("metadata: %+v", struct{ W, H, D int32 }{out.Width, out.Height, out.DurationSeconds})
	}
	if len(out.Full) == 0 || len(out.Full) > 5<<20 || len(out.Thumbnail) == 0 || len(out.Thumbnail) > 5<<20 {
		t.Fatal("output byte cap")
	}
	if !bytes.HasPrefix(out.Thumbnail, []byte("RIFF")) || string(out.Thumbnail[8:12]) != "WEBP" {
		t.Fatal("thumbnail is not WebP")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "full.mp4")
	if err := os.WriteFile(path, out.Full, 0600); err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Streams []struct {
			CodecName     string `json:"codec_name"`
			CodecType     string `json:"codec_type"`
			PixFmt        string `json:"pix_fmt"`
			Width, Height int
		}
		Format struct{ Duration string }
	}
	if err := json.Unmarshal(command(t, "ffprobe", "-v", "error", "-show_streams", "-show_format", "-of", "json", path), &probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.Streams) != 1 || probe.Streams[0].CodecName != "h264" || probe.Streams[0].CodecType != "video" || probe.Streams[0].PixFmt != "yuv420p" {
		t.Fatalf("unexpected streams: %+v", probe.Streams)
	}
	duration, _ := strconv.ParseFloat(probe.Format.Duration, 64)
	if math.Abs(duration-1.1) > .025 {
		t.Fatalf("duration %f want 1.1", duration)
	}
	var frames struct {
		Frames []struct {
			PTS string `json:"best_effort_timestamp_time"`
		}
	}
	if err := json.Unmarshal(command(t, "ffprobe", "-v", "error", "-show_frames", "-select_streams", "v:0", "-show_entries", "frame=best_effort_timestamp_time", "-of", "json", path), &frames); err != nil {
		t.Fatal(err)
	}
	if len(frames.Frames) < 4 {
		t.Fatalf("frames: %d", len(frames.Frames))
	}
	for i, want := range []float64{0, .2, .5, 1} {
		got, _ := strconv.ParseFloat(frames.Frames[i].PTS, 64)
		if math.Abs(got-want) > .001 {
			t.Fatalf("frame%d pts %f want %f", i, got, want)
		}
	}
	// Decode actual encoded samples, not just headers. Check disposal previous,
	// background, preserved red outside partial frames and deliberate black matte.
	pixels := command(t, "ffmpeg", "-v", "error", "-i", path, "-f", "rawvideo", "-pix_fmt", "rgb24", "-vsync", "0", "pipe:1")
	stride := 66 * 50 * 3
	if len(pixels) < 4*stride {
		t.Fatal("missing decoded frames")
	}
	sample := func(f, x, y int) (byte, byte, byte) {
		p := f*stride + (y*66+x)*3
		return pixels[p], pixels[p+1], pixels[p+2]
	}
	r, g, b := sample(0, 0, 0)
	if int(r)+int(g)+int(b) > 100 {
		t.Fatalf("transparent matte: %d %d %d", r, g, b)
	}
	for _, tc := range []struct{ f, x, y, channel int }{{0, 30, 30, 0}, {1, 30, 30, 1}, {2, 30, 30, 2}, {2, 18, 30, 0}, {3, 8, 8, 0}} {
		r, g, b = sample(tc.f, tc.x, tc.y)
		v := []byte{r, g, b}
		if v[tc.channel] < 170 {
			t.Fatalf("sample %+v RGB %v", tc, v)
		}
	}
	r, g, b = sample(3, 30, 30)
	if int(r)+int(g)+int(b) > 60 {
		t.Fatalf("background disposal did not clear: %d %d %d", r, g, b)
	}
	if bytes.Index(out.Full, []byte("moov")) > bytes.Index(out.Full, []byte("mdat")) {
		t.Fatal("MP4 is not faststart")
	}
}

func TestOpaqueLogicalBackgroundDisposal(t *testing.T) {
	pal := color.Palette{color.White, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	f0 := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	for i := range f0.Pix {
		f0.Pix[i] = 1
	}
	f1 := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	for i := range f1.Pix {
		f1.Pix[i] = 2
	}
	g := &gif.GIF{Config: image.Config{Width: 8, Height: 8, ColorModel: pal}, BackgroundIndex: 0, Image: []*image.Paletted{f0, f1}, Delay: []int{10, 10}, Disposal: []byte{gif.DisposalBackground, gif.DisposalNone}}
	dir := t.TempDir()
	if err := writeFrames(context.Background(), dir, g); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "000001.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	r, gr, b, _ := img.At(6, 6).RGBA()
	if r != 65535 || gr != 65535 || b != 65535 {
		t.Fatalf("opaque logical background lost: %d %d %d", r, gr, b)
	}
}

func TestRealCodecZeroDelayAndSingleFrame(t *testing.T) {
	codecAvailable(t)
	for _, delay := range []int{0, 1, 100} {
		out, err := Convert(context.Background(), fixture(t, []int{delay}, []byte{gif.DisposalNone}))
		if err != nil {
			t.Fatal(err)
		}
		if out.DurationSeconds != 1 {
			t.Fatalf("delay%d duration%d", delay, out.DurationSeconds)
		}
	}
}

func TestUnavailableCodecIsRetryableAndCleansTemp(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	_, err := Convert(context.Background(), fixture(t, []int{20}, []byte{gif.DisposalNone}))
	if err == nil || errors.Is(err, ErrInvalid) || errors.Is(err, ErrTooLarge) {
		t.Fatalf("want retryable dependency error: %v", err)
	}
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) != 0 {
		t.Fatalf("temporary files leaked: %v %v", entries, e)
	}
}
