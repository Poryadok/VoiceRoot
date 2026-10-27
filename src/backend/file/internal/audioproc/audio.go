// Package audioproc extracts bounded display metadata from admitted plaintext
// audio. Encrypted uploads must bypass this processor at the File boundary.
package audioproc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const MaxBytes = 10 * 1024 * 1024
const maxProbeBytes = 64 * 1024

var ErrSize = errors.New("audio_size_invalid")
var ErrAudio = errors.New("audio_metadata_invalid")
var probeSlots = make(chan struct{}, 2)

type Metadata struct {
	Title           string
	Artist          string
	Album           string
	DurationSeconds int32
}

type Extractor struct{ Binary string }

// Extract never returns stderr, filenames or tool errors to callers. The fixed
// demuxer/protocol lists exclude playlists and network protocols. The admitted
// MP3/FLAC/Ogg/WAVE formats match the client's bounded pull-stream decoder.
func (p Extractor) Extract(ctx context.Context, raw []byte) (Metadata, error) {
	if len(raw) == 0 || len(raw) > MaxBytes {
		return Metadata{}, ErrSize
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case probeSlots <- struct{}{}:
		defer func() { <-probeSlots }()
	case <-ctx.Done():
		return Metadata{}, ErrAudio
	}
	f, err := os.CreateTemp("", "voice-audio-*")
	if err != nil {
		return Metadata{}, ErrAudio
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(raw); err != nil {
		_ = f.Close()
		return Metadata{}, ErrAudio
	}
	if err = f.Close(); err != nil {
		return Metadata{}, ErrAudio
	}
	binary := p.Binary
	if binary == "" {
		binary = "ffprobe"
	}
	cmd := exec.CommandContext(ctx, binary, "-v", "error", "-max_alloc", "16777216", "-threads", "1", "-protocol_whitelist", "file", "-format_whitelist", "mp3,flac,ogg,wav", "-show_entries", "stream=codec_type,codec_name,duration:stream_disposition=attached_pic:format=duration:format_tags=title,artist,album", "-of", "json", f.Name())
	var out limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil || out.overflow {
		return Metadata{}, ErrAudio
	}
	return parseProbe(out.Bytes())
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	overflow bool
}

// Keep Buffer private: a promoted ReadFrom would let exec's io.Copy bypass
// Write and retain unbounded probe stdout.
func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }
func (b *limitedBuffer) Len() int      { return b.buffer.Len() }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxProbeBytes {
		b.overflow = true
		return 0, ErrAudio
	}
	return b.buffer.Write(p)
}

func parseProbe(raw []byte) (Metadata, error) {
	if len(raw) > maxProbeBytes {
		return Metadata{}, ErrAudio
	}
	var probe struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Duration    string `json:"duration"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
		Format struct {
			Duration string            `json:"duration"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		return Metadata{}, ErrAudio
	}
	audio := false
	duration := probe.Format.Duration
	for _, stream := range probe.Streams {
		if stream.CodecType == "audio" {
			switch stream.CodecName {
			case "mp3", "flac", "vorbis", "opus", "pcm_u8", "pcm_s16le", "pcm_s24le", "pcm_s32le", "pcm_f32le", "pcm_f64le":
			default:
				return Metadata{}, ErrAudio
			}
			audio = true
			if duration == "" {
				duration = stream.Duration
			}
			continue
		}
		if stream.CodecType != "video" || stream.Disposition.AttachedPic != 1 {
			return Metadata{}, ErrAudio
		}
	}
	seconds, err := strconv.ParseFloat(duration, 64)
	if !audio || err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > math.MaxInt32 {
		return Metadata{}, ErrAudio
	}
	m := Metadata{DurationSeconds: int32(math.Ceil(seconds))}
	seen := map[string]bool{}
	for key, value := range probe.Format.Tags {
		key = strings.ToLower(key)
		if key != "title" && key != "artist" && key != "album" {
			continue
		}
		if seen[key] || len(value) > 1024 {
			return Metadata{}, ErrAudio
		}
		seen[key] = true
		value = strings.TrimSpace(value)
		for _, r := range value {
			if unicode.IsControl(r) {
				return Metadata{}, ErrAudio
			}
		}
		switch key {
		case "title":
			m.Title = value
		case "artist":
			m.Artist = value
		case "album":
			m.Album = value
		}
	}
	return m, nil
}
