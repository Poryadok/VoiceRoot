package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/store"
)

type audioPreviewStub struct {
	filev1.FileServiceClient
	responses map[string]*filev1.ResolveMessageAttachmentPreviewResponse
	err       error
}

func (s *audioPreviewStub) ResolveMessageAttachmentPreview(_ context.Context, r *filev1.ResolveMessageAttachmentPreviewRequest, _ ...grpc.CallOption) (*filev1.ResolveMessageAttachmentPreviewResponse, error) {
	return s.responses[r.Reference.FileId], s.err
}

func TestAudioPreviewPerSlotAndFailClosed(t *testing.T) {
	audioID, imageID, owner := uuid.NewString(), uuid.NewString(), uuid.NewString()
	ref := func(id string) *filev1.FileReferenceKey {
		return &filev1.FileReferenceKey{FileId: id, OwnerId: owner, OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}
	}
	audioRef, imageRef := ref(audioID), ref(imageID)
	audio := &filev1.ResolveMessageAttachmentPreviewResponse{Reference: audioRef, AudioNoGenericPreview: &filev1.AudioNoGenericPreview{ProtocolVersion: 1, FileId: audioID, Status: "ready"}}
	wire, err := proto.Marshal(audio)
	require.NoError(t, err)
	decoded := &filev1.ResolveMessageAttachmentPreviewResponse{}
	require.NoError(t, proto.Unmarshal(wire, decoded))
	require.Nil(t, decoded.GetSnapshot(), "the old snapshot-only consumer still denies AUDIO")
	require.Equal(t, audioID, decoded.GetAudioNoGenericPreview().GetFileId())
	image := &filev1.ResolveMessageAttachmentPreviewResponse{Reference: imageRef, Snapshot: &eventsv1.FilePreviewSnapshot{ProtocolVersion: 1, FileId: imageID, MetadataRevision: 1, Status: "ready", FileType: "image", ScanResult: "clean", SizeBytes: 42}}
	stub := &audioPreviewStub{responses: map[string]*filev1.ResolveMessageAttachmentPreviewResponse{audioID: audio, imageID: image}}
	c := &AttachmentReferenceCoordinator{Files: stub, Issuer: testSpaceFileProducer(t).Issuer}
	request := &filev1.AcquireFileReferencesRequest{References: []*filev1.FileReferenceKey{audioRef, imageRef}}
	row := func() store.MessageRow {
		return store.MessageRow{ContentType: "voice", AttachmentsJSON: `[{"file_id":"` + audioID + `","type":"voice_message"},{"file_id":"` + imageID + `","type":"image"},{"file_id":"` + audioID + `","type":"audio"}]`}
	}
	r := row()
	require.NoError(t, c.PreparePreview(context.Background(), &r, request))
	require.Len(t, r.FilePreviewBindings, 1)
	require.Equal(t, 1, r.FilePreviewBindings[0].Slot)
	outerUnknown := proto.Clone(audio).(*filev1.ResolveMessageAttachmentPreviewResponse)
	outerUnknown.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	stub.responses[audioID] = outerUnknown
	r = row()
	require.NoError(t, c.PreparePreview(context.Background(), &r, request))
	stub.responses[audioID] = audio
	for name, mutate := range map[string]func(*filev1.ResolveMessageAttachmentPreviewResponse){
		"both":            func(v *filev1.ResolveMessageAttachmentPreviewResponse) { v.Snapshot = image.Snapshot },
		"wrong reference": func(v *filev1.ResolveMessageAttachmentPreviewResponse) { v.Reference = imageRef },
		"wrong id":        func(v *filev1.ResolveMessageAttachmentPreviewResponse) { v.AudioNoGenericPreview.FileId = imageID },
		"unknown version": func(v *filev1.ResolveMessageAttachmentPreviewResponse) { v.AudioNoGenericPreview.ProtocolVersion = 2 },
		"unknown status":  func(v *filev1.ResolveMessageAttachmentPreviewResponse) { v.AudioNoGenericPreview.Status = "other" },
		"unknown nested": func(v *filev1.ResolveMessageAttachmentPreviewResponse) {
			v.AudioNoGenericPreview.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := proto.Clone(audio).(*filev1.ResolveMessageAttachmentPreviewResponse)
			mutate(bad)
			stub.responses[audioID] = bad
			r := row()
			require.Equal(t, codes.DataLoss, status.Code(c.PreparePreview(context.Background(), &r, request)))
			stub.responses[audioID] = audio
		})
	}
	stub.err = status.Error(codes.FailedPrecondition, "unsupported")
	r = row()
	require.Equal(t, codes.FailedPrecondition, status.Code(c.PreparePreview(context.Background(), &r, request)))
	stub.err = nil
	stub.responses[audioID] = nil
	r = row()
	require.Equal(t, codes.DataLoss, status.Code(c.PreparePreview(context.Background(), &r, request)))
	stub.responses[audioID] = &filev1.ResolveMessageAttachmentPreviewResponse{Reference: audioRef}
	r = row()
	require.Equal(t, codes.DataLoss, status.Code(c.PreparePreview(context.Background(), &r, request)))
	processing := proto.Clone(audio).(*filev1.ResolveMessageAttachmentPreviewResponse)
	processing.AudioNoGenericPreview.Status = "processing"
	stub.responses[audioID] = processing
	r = row()
	require.Equal(t, codes.FailedPrecondition, status.Code(c.PreparePreview(context.Background(), &r, request)))
	r = row()
	require.NoError(t, c.preparePreviews(context.Background(), &r, request, false))
	require.Len(t, r.FilePreviewBindings, 1)
	stub.responses[audioID] = audio
	only := store.MessageRow{ContentType: "voice", AttachmentsJSON: `[{"file_id":"` + audioID + `","type":"voice_message"}]`}
	require.NoError(t, c.PreparePreview(context.Background(), &only, &filev1.AcquireFileReferencesRequest{References: []*filev1.FileReferenceKey{audioRef}}))
	require.NotNil(t, only.FilePreviewBindings)
	require.Empty(t, only.FilePreviewBindings)
	legacy := &filev1.ResolveMessageAttachmentPreviewResponse{Reference: audioRef, Snapshot: &eventsv1.FilePreviewSnapshot{ProtocolVersion: 1, FileId: audioID, MetadataRevision: 1, Status: "ready", FileType: "audio", ScanResult: "clean", SizeBytes: 42}}
	stub.responses[audioID] = legacy
	only = store.MessageRow{ContentType: "voice", AttachmentsJSON: `[{"file_id":"` + audioID + `","type":"voice_message"}]`}
	require.NoError(t, c.PreparePreview(context.Background(), &only, &filev1.AcquireFileReferencesRequest{References: []*filev1.FileReferenceKey{audioRef}}))
	require.Len(t, only.FilePreviewBindings, 1)
	stub.responses[audioID] = audio
	wrongType := store.MessageRow{ContentType: "voice", AttachmentsJSON: `[{"file_id":"` + audioID + `","type":"image"}]`}
	require.Equal(t, codes.FailedPrecondition, status.Code(c.PreparePreview(context.Background(), &wrongType, &filev1.AcquireFileReferencesRequest{References: []*filev1.FileReferenceKey{audioRef}})))
}
