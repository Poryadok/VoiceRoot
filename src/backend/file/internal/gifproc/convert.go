// Package gifproc converts bounded, validated GIF bytes into silent MP4 and WebP.
package gifproc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const Version = "gif-mp4-v1"
const maxOutputBytes = 5 << 20
const maxLogBytes = 16 << 10

// ErrInvalid and ErrTooLarge are permanent rejections, recognizable with errors.Is.
var ErrInvalid = errors.New("invalid GIF")
var ErrTooLarge = errors.New("GIF resource budget exceeded")

type Output struct {
	Full, Thumbnail []byte
	Width, Height   int32
	DurationSeconds int32
}

// Convert never fetches network input. Filesystem, subprocess and context errors
// do not wrap permanent rejection sentinels, allowing the durable worker to retry.
func Convert(ctx context.Context, raw []byte) (Output, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}
	duration, err := preflight(ctx, raw)
	if err != nil {
		return Output{}, err
	}
	decoded, err := gif.DecodeAll(bytes.NewReader(raw))
	if err != nil {
		return Output{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if err = ctx.Err(); err != nil {
		return Output{}, err
	}
	dir, err := os.MkdirTemp("", "voice-gif-")
	if err != nil {
		return Output{}, fmt.Errorf("GIF temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err = writeFrames(ctx, dir, decoded); err != nil {
		return Output{}, err
	}
	fullPath := filepath.Join(dir, "full.mp4")
	width := (decoded.Config.Width + 1) &^ 1
	height := (decoded.Config.Height + 1) &^ 1
	// CRF controls quality; the output cap is checked independently. A constrained
	// bitrate budget and output supervisor limit disk growth. A 10ms terminal frame
	// in ffconcat supplies the last GIF hold instead of dropping its duration.
	bitrate := int64(maxOutputBytes-65536) * 8 * 100 / int64(duration+1)
	// The manifest and relative filenames are generated here, never from input.
	// concat requires safe=0 for the per-image centisecond framerate option.
	args := []string{
		"-nostdin", "-v", "error", "-y", "-threads", "1", "-protocol_whitelist", "file",
		"-f", "concat", "-safe", "0", "-i", filepath.Join(dir, "frames.txt"),
		"-map", "0:v:0", "-an", "-sn", "-dn", "-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2:0:0:black,format=yuv420p",
		"-c:v", "libx264", "-threads", "1", "-preset", "veryfast", "-crf", "23",
		"-maxrate", strconv.FormatInt(bitrate, 10), "-bufsize", strconv.FormatInt(bitrate, 10),
		"-fps_mode", "vfr", "-enc_time_base", "1:100", "-video_track_timescale", "10000",
		"-movflags", "+faststart", "-map_metadata", "-1", "-bf", "0", fullPath,
	}
	if err = runOutput(ctx, fullPath, maxOutputBytes, args...); err != nil {
		return Output{}, err
	}
	full, err := readCapped(fullPath)
	if err != nil {
		return Output{}, err
	}
	if err = verifyMP4(ctx, fullPath, width, height, duration); err != nil {
		return Output{}, err
	}
	thumbPath := filepath.Join(dir, "thumbnail.webp")
	if err = runOutput(ctx, thumbPath, maxOutputBytes, "-nostdin", "-v", "error", "-y", "-threads", "1", "-protocol_whitelist", "file", "-i", filepath.Join(dir, "000000.png"), "-frames:v", "1", "-vf", "scale='min(320,iw)':'min(320,ih)':force_original_aspect_ratio=decrease", "-c:v", "libwebp", "-quality", "80", "-an", "-map_metadata", "-1", thumbPath); err != nil {
		return Output{}, err
	}
	thumb, err := readCapped(thumbPath)
	if err != nil {
		return Output{}, err
	}
	return Output{Full: full, Thumbnail: thumb, Width: int32(width), Height: int32(height), DurationSeconds: int32((duration + 99) / 100)}, nil
}

// preflight walks GIF block framing without allocating decoded pixels. Bounding
// full logical canvases per frame also bounds compositor/PNG disk work when a
// malicious file uses thousands of tiny subrectangles on a huge canvas.
func preflight(ctx context.Context, b []byte) (int, error) {
	if len(b) > 200<<20 {
		return 0, ErrTooLarge
	}
	if len(b) < 13 || string(b[:6]) != "GIF87a" && string(b[:6]) != "GIF89a" {
		return 0, ErrInvalid
	}
	u16 := func(p []byte) int { return int(binary.LittleEndian.Uint16(p)) }
	w, h := u16(b[6:8]), u16(b[8:10])
	if w == 0 || h == 0 {
		return 0, ErrInvalid
	}
	area := int64(w) * int64(h)
	if w > 4096 || h > 4096 || area > 16<<20 {
		return 0, ErrTooLarge
	}
	p := 13
	if b[10]&0x80 != 0 {
		p += 3 * (1 << ((b[10] & 7) + 1))
	}
	skipBlocks := func() bool {
		for p < len(b) {
			n := int(b[p])
			p++
			if n == 0 {
				return true
			}
			if n > len(b)-p {
				return false
			}
			p += n
		}
		return false
	}
	frames, total, delay := 0, 0, 0
	for p < len(b) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		tag := b[p]
		p++
		switch tag {
		case 0x3b:
			if frames == 0 || p != len(b) {
				return 0, ErrInvalid
			}
			return total, nil
		case 0x21:
			if p >= len(b) {
				return 0, ErrInvalid
			}
			kind := b[p]
			p++
			if kind == 0xf9 {
				if len(b)-p < 6 || b[p] != 4 || b[p+5] != 0 {
					return 0, ErrInvalid
				}
				delay = u16(b[p+2 : p+4])
				p += 6
			} else if !skipBlocks() {
				return 0, ErrInvalid
			}
		case 0x2c:
			if len(b)-p < 9 {
				return 0, ErrInvalid
			}
			x, y, fw, fh := u16(b[p:p+2]), u16(b[p+2:p+4]), u16(b[p+4:p+6]), u16(b[p+6:p+8])
			packed := b[p+8]
			p += 9
			if fw == 0 || fh == 0 || x+fw > w || y+fh > h {
				return 0, ErrInvalid
			}
			frames++
			if frames > 600 || area*int64(frames) > 64<<20 {
				return 0, ErrTooLarge
			}
			if delay == 0 {
				delay = 10
			}
			total += delay
			delay = 0
			if total > 12000 {
				return 0, ErrTooLarge
			}
			if packed&0x80 != 0 {
				p += 3 * (1 << ((packed & 7) + 1))
			}
			if p >= len(b) {
				return 0, ErrInvalid
			}
			p++ // LZW minimum code size; decoder validates it.
			if !skipBlocks() {
				return 0, ErrInvalid
			}
		default:
			return 0, ErrInvalid
		}
	}
	return 0, ErrInvalid
}

func writeFrames(ctx context.Context, dir string, g *gif.GIF) error {
	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, image.NewUniform(background(g, g.Image[0])), image.Point{}, draw.Src)
	var manifest strings.Builder
	manifest.WriteString("ffconcat version 1.0\n")
	for i, frame := range g.Image {
		if err := ctx.Err(); err != nil {
			return err
		}
		var previous *image.RGBA
		disposal := byte(0)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			previous = image.NewRGBA(bounds)
			copy(previous.Pix, canvas.Pix)
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		// Use an opaque black matte only in the encoded snapshot. The working canvas
		// retains alpha so later transparent overlays and disposal remain correct.
		opaque := image.NewRGBA(bounds)
		draw.Draw(opaque, bounds, image.NewUniform(color.Black), image.Point{}, draw.Src)
		draw.Draw(opaque, bounds, canvas, image.Point{}, draw.Over)
		name := fmt.Sprintf("%06d.png", i)
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("GIF frame create: %w", err)
		}
		encodeErr := png.Encode(f, opaque)
		closeErr := f.Close()
		if encodeErr != nil {
			return fmt.Errorf("GIF frame encode: %w", encodeErr)
		}
		if closeErr != nil {
			return closeErr
		}
		delay := g.Delay[i]
		if delay == 0 {
			delay = 10
		}
		fmt.Fprintf(&manifest, "file %s\noption framerate 100\nduration %.2f\n", name, float64(delay)/100)
		switch disposal {
		case gif.DisposalPrevious:
			canvas = previous
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.NewUniform(background(g, frame)), image.Point{}, draw.Src)
		}
	}
	// A final repeat gives the concat demuxer an explicit endpoint for the last
	// duration; its one-tick packet is included and verified within 20ms tolerance.
	fmt.Fprintf(&manifest, "file %06d.png\noption framerate 100\n", len(g.Image)-1)
	if err := os.WriteFile(filepath.Join(dir, "frames.txt"), []byte(manifest.String()), 0600); err != nil {
		return fmt.Errorf("GIF frame manifest: %w", err)
	}
	return nil
}

// A transparent GIF disposes to transparency; opaque GIFs use the logical
// screen background from the global palette, independent of local palettes.
func background(g *gif.GIF, frame *image.Paletted) color.Color {
	for _, c := range frame.Palette {
		_, _, _, a := c.RGBA()
		if a == 0 {
			return color.Transparent
		}
	}
	if pal, ok := g.Config.ColorModel.(color.Palette); ok && int(g.BackgroundIndex) < len(pal) {
		return pal[g.BackgroundIndex]
	}
	return color.Transparent
}

// A named buffer prevents io.Copy from finding a promoted ReadFrom method and
// bypassing Write. Excess bytes are drained without being retained.
type limitedBuffer struct{ buffer bytes.Buffer }

func (b *limitedBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *limitedBuffer) String() string { return b.buffer.String() }

func runOutput(ctx context.Context, path string, maxBytes int, args ...string) error {
	// Do not use ffmpeg -fs: it can exit successfully with a truncated movie
	// below the requested cap, making duration validation look retryable forever.
	child, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	done, watched := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watched)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		check := func() bool {
			info, err := os.Stat(path)
			if err == nil && info.Size() > int64(maxBytes) {
				cancel(ErrTooLarge)
				return true
			}
			return false
		}
		for {
			select {
			case <-done:
				check()
				return // Also inspect outputs that finish between ticks.
			case <-child.Done():
				return
			case <-ticker.C:
				if check() {
					return
				}
			}
		}
	}()
	_, err := run(child, "ffmpeg", args...)
	close(done)
	<-watched
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(context.Cause(child), ErrTooLarge) {
		return ErrTooLarge
	}
	return err
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if room := maxLogBytes - b.buffer.Len(); room > 0 {
		if len(p) > room {
			p = p[:room]
		}
		_, _ = b.buffer.Write(p)
	}
	return n, nil
}

func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	var stdout, stderr limitedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("GIF %s failed: %w: %s", name, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func readCapped(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("GIF output open: %w", err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, maxOutputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("GIF output read: %w", err)
	}
	if len(b) > maxOutputBytes {
		return nil, ErrTooLarge
	}
	if len(b) == 0 {
		return nil, errors.New("GIF codec produced empty output")
	}
	return b, nil
}

func verifyMP4(ctx context.Context, path string, w, h, duration int) error {
	b, err := run(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file", "-show_entries", "stream=codec_name,codec_type,pix_fmt,width,height:format=duration", "-of", "json", path)
	if err != nil {
		return err
	}
	var p struct {
		Streams []struct {
			CodecName     string `json:"codec_name"`
			CodecType     string `json:"codec_type"`
			PixFmt        string `json:"pix_fmt"`
			Width, Height int
		}
		Format struct{ Duration string }
	}
	if err = json.Unmarshal(b, &p); err != nil {
		return fmt.Errorf("GIF output probe: %w", err)
	}
	seconds, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || math.Abs(seconds-float64(duration)/100) > .021 || len(p.Streams) != 1 {
		return fmt.Errorf("GIF output failed duration/stream verification: got %.4fs, expected %.4fs, streams %d", seconds, float64(duration)/100, len(p.Streams))
	}
	s := p.Streams[0]
	if s.CodecType != "video" || s.CodecName != "h264" || s.PixFmt != "yuv420p" || s.Width != w || s.Height != h {
		return errors.New("GIF output failed codec verification")
	}
	return nil
}
