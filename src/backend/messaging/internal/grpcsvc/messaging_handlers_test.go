package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
)

type stubChatGuard struct {
	peer uuid.UUID
	err  error
}

func (s stubChatGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (s stubChatGuard) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	if s.err != nil {
		return uuid.Nil, s.err
	}
	return s.peer, nil
}

func (s stubChatGuard) OtherMemberProfileIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.peer == uuid.Nil {
		return nil, nil
	}
	return []uuid.UUID{s.peer}, nil
}

type stubProfiles struct {
	acct uuid.UUID
	err  error
}

func (s stubProfiles) AccountIDByProfileID(context.Context, uuid.UUID) (uuid.UUID, error) {
	if s.err != nil {
		return uuid.Nil, s.err
	}
	return s.acct, nil
}

type stubBlocks struct {
	blocked bool
	err     error
}

func (s stubBlocks) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.blocked, nil
}

type stubFiles struct {
	byID map[string]*filev1.FileMetadata
	err  error
}

func (s stubFiles) GetBulkMetadata(_ context.Context, req *filev1.GetBulkMetadataRequest, _ ...grpc.CallOption) (*filev1.GetBulkMetadataResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := map[string]*filev1.FileMetadata{}
	for _, id := range req.GetFileIds() {
		if m := s.byID[id]; m != nil {
			out[id] = m
		}
	}
	return &filev1.GetBulkMetadataResponse{BulkFileMetadata: &filev1.BulkFileMetadata{ByFileId: out}}, nil
}

func profileCtx(accountID, profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderUserID, accountID.String(),
		authctx.HeaderProfileID, profileID.String(),
	))
}

func TestCheckDMBlocksForSend(t *testing.T) {
	t.Parallel()
	chatID := uuid.New()
	selfProf := uuid.New()
	peerProf := uuid.New()
	selfAcct := uuid.New()
	peerAcct := uuid.New()

	t.Run("skipped when deps nil", func(t *testing.T) {
		t.Parallel()
		s := &MessagingGRPC{}
		require.NoError(t, s.checkDMBlocksForSend(context.Background(), chatID, selfProf))
	})

	t.Run("missing account", func(t *testing.T) {
		t.Parallel()
		s := &MessagingGRPC{
			ChatGuard:    stubChatGuard{peer: peerProf},
			UserProfiles: stubProfiles{acct: peerAcct},
			Blocks:       stubBlocks{},
		}
		err := s.checkDMBlocksForSend(context.Background(), chatID, selfProf)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("peer resolution internal", func(t *testing.T) {
		t.Parallel()
		s := &MessagingGRPC{
			ChatGuard:    stubChatGuard{err: errors.New("chat internal")},
			UserProfiles: stubProfiles{acct: peerAcct},
			Blocks:       stubBlocks{},
		}
		ctx := profileCtx(selfAcct, selfProf)
		err := s.checkDMBlocksForSend(ctx, chatID, selfProf)
		require.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("peer not member", func(t *testing.T) {
		t.Parallel()
		s := &MessagingGRPC{
			ChatGuard:    stubChatGuard{err: store.ErrNotChatMember},
			UserProfiles: stubProfiles{acct: peerAcct},
			Blocks:       stubBlocks{},
		}
		ctx := profileCtx(selfAcct, selfProf)
		err := s.checkDMBlocksForSend(ctx, chatID, selfProf)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("peer profile not found", func(t *testing.T) {
		t.Parallel()
		s := &MessagingGRPC{
			ChatGuard:    stubChatGuard{peer: peerProf},
			UserProfiles: stubProfiles{err: status.Error(codes.NotFound, "missing")},
			Blocks:       stubBlocks{},
		}
		ctx := profileCtx(selfAcct, selfProf)
		err := s.checkDMBlocksForSend(ctx, chatID, selfProf)
		require.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("blocks error", func(t *testing.T) {
		t.Parallel()
		s := &MessagingGRPC{
			ChatGuard:    stubChatGuard{peer: peerProf},
			UserProfiles: stubProfiles{acct: peerAcct},
			Blocks:       stubBlocks{err: errors.New("social down")},
		}
		ctx := profileCtx(selfAcct, selfProf)
		err := s.checkDMBlocksForSend(ctx, chatID, selfProf)
		require.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("blocked", func(t *testing.T) {
		t.Parallel()
		s := &MessagingGRPC{
			ChatGuard:    stubChatGuard{peer: peerProf},
			UserProfiles: stubProfiles{acct: peerAcct},
			Blocks:       stubBlocks{blocked: true},
		}
		ctx := profileCtx(selfAcct, selfProf)
		err := s.checkDMBlocksForSend(ctx, chatID, selfProf)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("allowed", func(t *testing.T) {
		t.Parallel()
		s := &MessagingGRPC{
			ChatGuard:    stubChatGuard{peer: peerProf},
			UserProfiles: stubProfiles{acct: peerAcct},
			Blocks:       stubBlocks{blocked: false},
		}
		ctx := profileCtx(selfAcct, selfProf)
		require.NoError(t, s.checkDMBlocksForSend(ctx, chatID, selfProf))
	})
}

func TestValidateAttachments(t *testing.T) {
	t.Parallel()
	chatID := uuid.New()
	fileID := uuid.New().String()
	dm := chatv1.ChatType_CHAT_TYPE_DM
	chatRef := &chatv1.ChatRef{Id: chatID.String(), Type: &dm}

	s := &MessagingGRPC{Files: stubFiles{byID: map[string]*filev1.FileMetadata{
		fileID: {
			Id: fileID, Status: "ready", FileType: "image", ScanResult: "clean", Chat: chatRef,
		},
	}}}

	n, err := s.validateAttachments(context.Background(), chatID, "[]")
	require.NoError(t, err)
	require.Equal(t, 0, n)

	_, err = s.validateAttachments(context.Background(), chatID, "not-json")
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	sNoFiles := &MessagingGRPC{}
	_, err = sNoFiles.validateAttachments(context.Background(), chatID, `[{"file_id":"`+fileID+`","type":"image"}]`)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = s.validateAttachments(context.Background(), chatID, `[{"file_id":"bad","type":"image"}]`)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = s.validateAttachments(context.Background(), chatID, `[{"file_id":"`+fileID+`","type":""}]`)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = s.validateAttachments(context.Background(), chatID, `[{"file_id":"`+uuid.New().String()+`","type":"image"}]`)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	wrongChat := uuid.New().String()
	sWrong := &MessagingGRPC{Files: stubFiles{byID: map[string]*filev1.FileMetadata{
		fileID: {Id: fileID, Status: "ready", FileType: "image", ScanResult: "clean", Chat: &chatv1.ChatRef{Id: wrongChat, Type: &dm}},
	}}}
	_, err = sWrong.validateAttachments(context.Background(), chatID, `[{"file_id":"`+fileID+`","type":"image"}]`)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	sDirty := &MessagingGRPC{Files: stubFiles{byID: map[string]*filev1.FileMetadata{
		fileID: {Id: fileID, Status: "ready", FileType: "image", ScanResult: "infected", Chat: chatRef},
	}}}
	_, err = sDirty.validateAttachments(context.Background(), chatID, `[{"file_id":"`+fileID+`","type":"image"}]`)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	sSkipped := &MessagingGRPC{Files: stubFiles{byID: map[string]*filev1.FileMetadata{
		fileID: {Id: fileID, Status: "ready", FileType: "image", ScanResult: "skipped", Chat: chatRef},
	}}}
	n, err = sSkipped.validateAttachments(context.Background(), chatID, `[{"file_id":"`+fileID+`","type":"image"}]`)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	sMismatch := &MessagingGRPC{Files: stubFiles{byID: map[string]*filev1.FileMetadata{
		fileID: {Id: fileID, Status: "ready", FileType: "video", ScanResult: "clean", Chat: chatRef},
	}}}
	_, err = sMismatch.validateAttachments(context.Background(), chatID, `[{"file_id":"`+fileID+`","type":"image"}]`)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	sDown := &MessagingGRPC{Files: stubFiles{err: errors.New("file svc")}}
	_, err = sDown.validateAttachments(context.Background(), chatID, `[{"file_id":"`+fileID+`","type":"image"}]`)
	require.Equal(t, codes.Internal, status.Code(err))

	audioID := uuid.New().String()
	sVoice := &MessagingGRPC{Files: stubFiles{byID: map[string]*filev1.FileMetadata{
		audioID: {Id: audioID, Status: "ready", FileType: "audio", ScanResult: "clean", Chat: chatRef},
	}}}
	n, err = sVoice.validateAttachments(context.Background(), chatID, `[{"file_id":"`+audioID+`","type":"voice_message"}]`)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}
