package audioproc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
	if _, err := b.Write(make([]byte, maxProbeBytes+1)); err == nil || !b.overflow || b.buffer.Len() != 0 {
		t.Fatal("oversized process output was retained")
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

func TestExtractorBoundsProcessOutput(t *testing.T) {
	// A real child and pipe exercise exec's io.Copy optimizations. Calling Write
	// directly does not catch a promoted bytes.Buffer.ReadFrom bypassing the cap.
	dir := t.TempDir()
	source := filepath.Join(dir, "ffprobe.go")
	binary := filepath.Join(dir, "ffprobe.exe")
	const helper = `package main
import ("io"; "os")
func main() {
	f, err := os.Open(os.Getenv("AUDIO_TEST_OUTPUT")); if err != nil { os.Exit(2) }
	defer f.Close()
	if _, err := io.Copy(os.Stdout, f); err != nil { os.Exit(3) }
	if err := os.WriteFile(os.Getenv("AUDIO_TEST_COMPLETED"), []byte(os.Args[len(os.Args)-1]), 0600); err != nil { os.Exit(4) }
}`
	if err := os.WriteFile(source, []byte(helper), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-o", binary, source).CombinedOutput(); err != nil {
		t.Fatalf("build fake ffprobe: %v: %s", err, out)
	}
	const valid = `{"streams":[{"codec_type":"audio","codec_name":"mp3"}],"format":{"duration":"1","tags":{"title":"Track"}}}`
	for _, tc := range []struct {
		name string
		size int
	}{
		{"valid", len(valid)},
		{"exact limit", maxProbeBytes},
		{"oversized", 8 * 1024 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "stdout")
			marker := filepath.Join(t.TempDir(), "completed")
			if err := os.WriteFile(output, []byte(valid+strings.Repeat(" ", tc.size-len(valid))), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("AUDIO_TEST_OUTPUT", output)
			t.Setenv("AUDIO_TEST_COMPLETED", marker)
			got, err := (Extractor{Binary: binary}).Extract(context.Background(), []byte("audio fixture"))
			inputPath, markerErr := os.ReadFile(marker)
			if tc.size > maxProbeBytes {
				if err != ErrAudio || got != (Metadata{}) {
					t.Fatalf("oversized metadata = %#v, %v", got, err)
				}
				if !os.IsNotExist(markerErr) {
					t.Fatalf("child completed oversized stdout instead of stopping at capture limit: marker=%q, err=%v", inputPath, markerErr)
				}
				return
			}
			if err != nil || got.Title != "Track" || got.DurationSeconds != 1 {
				t.Fatalf("metadata = %#v, %v", got, err)
			}
			if markerErr != nil {
				t.Fatalf("child did not complete admitted stdout: %v", markerErr)
			}
			if _, err := os.Stat(string(inputPath)); !os.IsNotExist(err) {
				t.Fatalf("temporary input was retained: %v", err)
			}
		})
	}
}
