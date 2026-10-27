package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	filev1 "voice.app/voice/file/v1"
)

func TestAPIProcessedGIFCannotBypassMessagingIntentRules(t *testing.T) {
	id := uuid.NewString()
	s := &MessagingGRPC{Files: stubFiles{byID: map[string]*filev1.FileMetadata{id: {Id: id, Status: "ready", FileType: "video", ScanResult: "skipped", Intent: filev1.UploadIntent_UPLOAD_INTENT_GIF}}}}
	for _, kind := range []string{"video", "gif", "sticker"} {
		_, err := s.validateFileBackedAttachments(context.Background(), nil, []messageAttachment{{FileID: id, Type: kind}})
		require.Equal(t, codes.FailedPrecondition, status.Code(err), "API GIF processing must not activate send/forward by relabeling attachment type")
	}
}
