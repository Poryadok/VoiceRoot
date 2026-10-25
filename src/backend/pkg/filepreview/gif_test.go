package filepreview

import (
	"testing"
	eventsv1 "voice.app/voice/events/v1"
)

func TestGIFPreviewVersionCompatibility(t *testing.T) {
	s := &eventsv1.FilePreviewSnapshot{ProtocolVersion: 2, FileId: "ad7e1e5b-05a1-40b4-a647-c99dbb029e74", MetadataRevision: 1, Status: "ready", FileType: "video", UploadIntent: "gif", SizeBytes: 1024, ScanResult: "skipped", HasConverted: true, HasThumbnail: true}
	if err := Validate(s); err != nil {
		t.Fatalf("GIF v2 preview rejected: %v", err)
	}
	s.ProtocolVersion = 1
	if Validate(s) == nil {
		t.Fatal("v1 must continue rejecting GIF intent")
	}
	s.ProtocolVersion = 2
	s.FileType = "other"
	if Validate(s) == nil {
		t.Fatal("GIF output must be video")
	}
	s.FileType = "video"
	s.HasConverted = false
	if Validate(s) == nil {
		t.Fatal("GIF ready output must have converted representation")
	}
}
