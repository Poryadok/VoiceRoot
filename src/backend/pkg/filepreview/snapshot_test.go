package filepreview

import (
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

func TestClosedRedactedPreviewContract(t *testing.T) {
	base := &eventsv1.FilePreviewSnapshot{ProtocolVersion: 1, FileId: uuid.NewString(), MetadataRevision: 1, Status: "ready", FileType: "image", SizeBytes: 42, ScanResult: "clean", Width: proto.Int32(20)}
	require.NoError(t, Validate(base))
	cases := map[string]func(*eventsv1.FilePreviewSnapshot){
		"zero_revision":        func(s *eventsv1.FilePreviewSnapshot) { s.MetadataRevision = 0 },
		"overflow":             func(s *eventsv1.FilePreviewSnapshot) { s.MetadataRevision = math.MaxUint64 },
		"encrypted":            func(s *eventsv1.FilePreviewSnapshot) { s.IsE2E = true },
		"music":                func(s *eventsv1.FilePreviewSnapshot) { s.UploadIntent = "music" },
		"unimplemented_intent": func(s *eventsv1.FilePreviewSnapshot) { s.UploadIntent = "sticker" },
		"unknown_status":       func(s *eventsv1.FilePreviewSnapshot) { s.Status = "maybe_ready" },
		"infected":             func(s *eventsv1.FilePreviewSnapshot) { s.ScanResult = "infected" },
		"negative_numeric":     func(s *eventsv1.FilePreviewSnapshot) { s.Width = proto.Int32(-1) },
		"tombstone_leak":       func(s *eventsv1.FilePreviewSnapshot) { s.Status = "deleted" },
		"processing_leak":      func(s *eventsv1.FilePreviewSnapshot) { s.Status = "processing" },
		"unknown_field":        func(s *eventsv1.FilePreviewSnapshot) { s.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01}) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := proto.Clone(base).(*eventsv1.FilePreviewSnapshot)
			mutate(s)
			require.Error(t, Validate(s))
		})
	}
	event := &eventsv1.FileProcessed{FileId: base.FileId, Status: base.Status, Preview: base}
	require.NoError(t, ValidateEvent(event))
	event.ConvertedR2Key = proto.String("private/key")
	require.Error(t, ValidateEvent(event))
	event.ConvertedR2Key = nil
	event.Music = &eventsv1.FileMusicMetadata{}
	require.Error(t, ValidateEvent(event))
	event.Music = nil
	event.FileId = uuid.NewString()
	require.Error(t, ValidateEvent(event))
}
