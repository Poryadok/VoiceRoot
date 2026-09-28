package s2s

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/gameprotocol"
)

// GameAttachmentManifestVerifier reads every attachment through File's
// profile-scoped metadata API, using only the profile resolved by T16.
type GameAttachmentManifestVerifier struct {
	Client fileGameMetadataClient
	Clock  func() time.Time
}

type fileGameMetadataClient interface {
	GetFileMetadata(context.Context, *filev1.GetFileMetadataRequest, ...grpc.CallOption) (*filev1.GetFileMetadataResponse, error)
}

func (v *GameAttachmentManifestVerifier) VerifyGameAttachmentManifest(ctx context.Context, profileID, _ uuid.UUID, attachments []gameprotocol.Attachment) error {
	if v == nil || v.Client == nil || profileID == uuid.Nil || len(attachments) == 0 {
		return errors.New("File game attachment verifier unavailable")
	}
	now := time.Now().UTC()
	if v.Clock != nil {
		now = v.Clock().UTC()
	}
	for _, attachment := range attachments {
		ctx := profileMetadataContext(ctx, profileID)
		response, err := v.Client.GetFileMetadata(ctx, &filev1.GetFileMetadataRequest{FileId: attachment.FileID.String()})
		if err != nil || response == nil || response.GetFileMetadata() == nil {
			return errors.New("File metadata unavailable")
		}
		file := response.GetFileMetadata()
		if file.GetId() != attachment.FileID.String() || file.GetObjectRevision() != uint64(attachment.ObjectRevision) ||
			file.GetSizeBytes() != attachment.ByteLength || file.GetSha256Hash() != attachment.ContentSHA256 ||
			file.GetMimeType() != attachment.MediaType || file.GetStatus() != "ready" || file.GetScanResult() != "clean" {
			return errors.New("File metadata does not match signed attachment manifest")
		}
		if file.GetExpiresAt() != nil && !file.GetExpiresAt().AsTime().After(now) {
			return errors.New("File attachment has expired")
		}
	}
	return nil
}

func profileMetadataContext(ctx context.Context, profileID uuid.UUID) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	forwarded := md.Copy()
	forwarded.Delete("x-voice-profile-id")
	forwarded.Delete("x-voice-user-id")
	forwarded.Set("x-voice-profile-id", profileID.String())
	return metadata.NewOutgoingContext(ctx, forwarded)
}
