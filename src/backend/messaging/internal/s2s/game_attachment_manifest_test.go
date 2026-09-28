package s2s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/gameprotocol"
)

type gameFileMetadataClient struct {
	file       *filev1.FileMetadata
	err        error
	profileIDs []string
}

func (c *gameFileMetadataClient) GetFileMetadata(ctx context.Context, req *filev1.GetFileMetadataRequest, _ ...grpc.CallOption) (*filev1.GetFileMetadataResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	c.profileIDs = append(c.profileIDs, md.Get("x-voice-profile-id")...)
	if c.err != nil {
		return nil, c.err
	}
	if c.file == nil || req.GetFileId() != c.file.GetId() {
		return nil, errors.New("file missing")
	}
	return &filev1.GetFileMetadataResponse{FileMetadata: c.file}, nil
}

func TestGameAttachmentManifestVerifierUsesResolvedProfileAndMatchesImmutableMetadata(t *testing.T) {
	fileID, trustedProfile, untrustedProfile := uuid.New(), uuid.New(), uuid.New()
	client := &gameFileMetadataClient{file: &filev1.FileMetadata{
		Id: fileID.String(), ObjectRevision: 1, SizeBytes: 5,
		Sha256Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		MimeType:   "image/png", Status: "ready", ScanResult: "clean",
	}}
	verifier := &GameAttachmentManifestVerifier{Client: client, Clock: func() time.Time { return time.Unix(100, 0) }}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", untrustedProfile.String(), "x-voice-user-id", untrustedProfile.String()))
	err := verifier.VerifyGameAttachmentManifest(ctx, trustedProfile, uuid.New(), []gameprotocol.Attachment{{
		FileID: fileID, ObjectRevision: 1, ByteLength: 5, ContentSHA256: client.file.GetSha256Hash(), MediaType: "image/png",
	}})
	require.NoError(t, err)
	require.Equal(t, []string{trustedProfile.String()}, client.profileIDs)
}

func TestGameAttachmentManifestVerifierFailsClosedOnMismatchOrFileUnavailable(t *testing.T) {
	fileID, profileID := uuid.New(), uuid.New()
	attachment := gameprotocol.Attachment{FileID: fileID, ObjectRevision: 1, ByteLength: 5,
		ContentSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", MediaType: "image/png"}
	base := &filev1.FileMetadata{Id: fileID.String(), ObjectRevision: 1, SizeBytes: 5,
		Sha256Hash: attachment.ContentSHA256, MimeType: "image/png", Status: "ready", ScanResult: "clean"}
	for _, tc := range []struct {
		name string
		file *filev1.FileMetadata
		err  error
	}{
		{name: "wrong revision", file: func() *filev1.FileMetadata {
			f := proto.Clone(base).(*filev1.FileMetadata)
			f.ObjectRevision = 2
			return f
		}()},
		{name: "wrong digest", file: func() *filev1.FileMetadata {
			f := proto.Clone(base).(*filev1.FileMetadata)
			f.Sha256Hash = "bad"
			return f
		}()},
		{name: "not ready", file: func() *filev1.FileMetadata {
			f := proto.Clone(base).(*filev1.FileMetadata)
			f.Status = "deleted"
			return f
		}()},
		{name: "infected", file: func() *filev1.FileMetadata {
			f := proto.Clone(base).(*filev1.FileMetadata)
			f.ScanResult = "infected"
			return f
		}()},
		{name: "service unavailable", err: errors.New("File unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &gameFileMetadataClient{file: tc.file, err: tc.err}
			verifier := &GameAttachmentManifestVerifier{Client: client}
			require.Error(t, verifier.VerifyGameAttachmentManifest(context.Background(), profileID, uuid.New(), []gameprotocol.Attachment{attachment}))
		})
	}
}
