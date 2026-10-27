package filepreview

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
)

func TestPDFDocumentThumbnailSnapshot(t *testing.T) {
	s := &eventsv1.FilePreviewSnapshot{ProtocolVersion: 1, FileId: uuid.NewString(), MetadataRevision: 1, Status: "ready", FileType: "document", SizeBytes: 100, ScanResult: "clean", HasThumbnail: true}
	require.NoError(t, Validate(s))
	width := int32(320)
	s.Width = &width
	require.Error(t, Validate(s))
	s.Width = nil
	s.FileType = "other"
	require.Error(t, Validate(s))
}
