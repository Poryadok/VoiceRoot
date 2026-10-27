package audioproc

import (
	"context"
	"io"
	"strconv"
	"strings"
	"testing"
)

// File's music contract admits audio only and extracts optional tags rather
// than accepting caller-supplied display metadata.
func TestProbeMetadata(t *testing.T) {
	got, err := parseProbe([]byte(`{"streams":[{"codec_type":"audio","codec_name":"mp3","duration":"12.25"}],"format":{"duration":"12.25","tags":{"TITLE":"Track","artist":"Artist","album":"Album"}}}`))
	if err != nil || got.Title != "Track" || got.Artist != "Artist" || got.Album != "Album" || got.DurationSeconds != 13 {
		t.Fatalf("metadata = %#v, %v", got, err)
	}
	untagged, err := parseProbe([]byte(`{"streams":[{"codec_type":"audio","codec_name":"mp3"}],"format":{"duration":"1"}}`))
	if err != nil || untagged.Title != "" || untagged.Artist != "" || untagged.Album != "" || untagged.DurationSeconds != 1 {
		t.Fatalf("untagged = %#v, %v", untagged, err)
	}
}

func TestProbeRejectsNonAudioAndInvalidDuration(t *testing.T) {
	for _, raw := range []string{
		`{`,
		`{"streams":[],"format":{"duration":"1"}}`,
		`{"streams":[{"codec_type":"video"}],"format":{"duration":"1"}}`,
		`{"streams":[{"codec_type":"audio","codec_name":"mp3"},{"codec_type":"video"}],"format":{"duration":"1"}}`,
		`{"streams":[{"codec_type":"audio","codec_name":"mp3"}],"format":{"duration":"NaN"}}`,
		`{"streams":[{"codec_type":"audio","codec_name":"mp3"}],"format":{"duration":"-1"}}`,
		`{"streams":[{"codec_type":"audio","codec_name":"mp3"}],"format":{"duration":"1e30"}}`,
	} {
		if _, err := parseProbe([]byte(raw)); err == nil {
			t.Fatalf("accepted invalid media probe: %s", raw)
		}
	}
}

func TestProbeBoundsUntrustedTags(t *testing.T) {
	raw := `{"streams":[{"codec_type":"audio","codec_name":"mp3"}],"format":{"duration":"1","tags":{"title":"` + strings.Repeat("x", 1025) + `"}}}`
	if _, err := parseProbe([]byte(raw)); err == nil {
		t.Fatal("accepted oversized tag")
	}
}

func TestProbePermitsEmbeddedCoverButNotVideo(t *testing.T) {
	_, err := parseProbe([]byte(`{"streams":[{"codec_type":"audio","codec_name":"mp3"},{"codec_type":"video","disposition":{"attached_pic":1}}],"format":{"duration":"2"}}`))
	if err != nil {
		t.Fatalf("embedded album cover rejected: %v", err)
	}
}

func TestProbeOutputLimit(t *testing.T) {
	var b limitedBuffer
	if _, err := b.Write(make([]byte, maxProbeBytes+1)); err == nil || !b.overflow || b.Len() != 0 {
		t.Fatal("oversized process output was retained")
	}
}

func TestProbeOutputCopyLimit(t *testing.T) {
	for _, size := range []int{maxProbeBytes, maxProbeBytes + 1, 4 * maxProbeBytes} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			var out limitedBuffer
			// exec copies stdout from its pipe with io.Copy. Hide WriterTo so
			// this exercises the same destination-side interface dispatch.
			source := struct{ io.Reader }{strings.NewReader(strings.Repeat("x", size))}
			_, err := io.Copy(&out, source)
			if size <= maxProbeBytes {
				if err != nil || out.overflow || out.Len() != size {
					t.Fatalf("at cap: retained=%d overflow=%v error=%v", out.Len(), out.overflow, err)
				}
			} else if err != ErrAudio || !out.overflow || out.Len() > maxProbeBytes {
				t.Fatalf("above cap: retained=%d overflow=%v error=%v", out.Len(), out.overflow, err)
			}
		})
	}
}

func TestProbeRejectsUnsupportedCodecInsideAdmittedContainer(t *testing.T) {
	// A WAVE/Ogg container alone does not establish pull-decoder compatibility.
	for _, codec := range []string{"aac", "speex", "ac3", "", "unknown"} {
		raw := `{"streams":[{"codec_type":"audio","codec_name":"` + codec + `"}],"format":{"duration":"1"}}`
		if _, err := parseProbe([]byte(raw)); err != ErrAudio {
			t.Fatalf("codec %q: got %v", codec, err)
		}
	}
	for _, codec := range []string{"mp3", "flac", "vorbis", "opus", "pcm_u8", "pcm_s16le", "pcm_s24le", "pcm_s32le", "pcm_f32le", "pcm_f64le"} {
		raw := `{"streams":[{"codec_type":"audio","codec_name":"` + codec + `"}],"format":{"duration":"1"}}`
		if _, err := parseProbe([]byte(raw)); err != nil {
			t.Fatalf("codec %q: got %v", codec, err)
		}
	}
}

func TestExtractorRejectsSizeBeforeExecuting(t *testing.T) {
	// An invalid binary proves admission fails independently of the tool.
	p := Extractor{Binary: "/does-not-exist"}
	for _, raw := range [][]byte{nil, make([]byte, MaxBytes+1)} {
		if _, err := p.Extract(context.Background(), raw); err != ErrSize {
			t.Fatalf("size admission error = %v", err)
		}
	}
}
