package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"
)

func (s *ChatGRPC) ListInstalledStickerPacks(ctx context.Context, _ *chatv1.ListInstalledStickerPacksRequest) (*chatv1.ListInstalledStickerPacksResponse, error) {
	id, err := s.stickerProfile(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.StickerPacks.ListInstalledStickerPacks(ctx, id)
	if err != nil {
		return nil, stickerError(err)
	}
	out := make([]*chatv1.StickerPack, 0, len(rows))
	for _, p := range rows {
		out = append(out, stickerPackToProto(p))
	}
	return &chatv1.ListInstalledStickerPacksResponse{Packs: out}, nil
}
func (s *ChatGRPC) GetStickerPack(ctx context.Context, req *chatv1.GetStickerPackRequest) (*chatv1.GetStickerPackResponse, error) {
	id, err := s.stickerProfile(ctx)
	if err != nil {
		return nil, err
	}
	pack, err := parseUUIDField("pack_id", req.GetPackId())
	if err != nil {
		return nil, err
	}
	row, err := s.StickerPacks.GetStickerPack(ctx, id, pack)
	if err != nil {
		return nil, stickerError(err)
	}
	return &chatv1.GetStickerPackResponse{Pack: stickerPackToProto(*row)}, nil
}
func (s *ChatGRPC) InstallStickerPack(ctx context.Context, req *chatv1.InstallStickerPackRequest) (*chatv1.InstallStickerPackResponse, error) {
	id, err := s.stickerProfile(ctx)
	if err != nil {
		return nil, err
	}
	pack, err := parseUUIDField("pack_id", req.GetPackId())
	if err != nil {
		return nil, err
	}
	row, err := s.StickerPacks.InstallStickerPack(ctx, id, pack)
	if err != nil {
		return nil, stickerError(err)
	}
	return &chatv1.InstallStickerPackResponse{Pack: stickerPackToProto(*row)}, nil
}
func (s *ChatGRPC) UninstallStickerPack(ctx context.Context, req *chatv1.UninstallStickerPackRequest) (*chatv1.UninstallStickerPackResponse, error) {
	id, err := s.stickerProfile(ctx)
	if err != nil {
		return nil, err
	}
	pack, err := parseUUIDField("pack_id", req.GetPackId())
	if err != nil {
		return nil, err
	}
	if err = s.StickerPacks.UninstallStickerPack(ctx, id, pack); err != nil {
		return nil, stickerError(err)
	}
	return &chatv1.UninstallStickerPackResponse{}, nil
}
func (s *ChatGRPC) stickerProfile(ctx context.Context) (uuid.UUID, error) {
	if s == nil || s.StickerPacks == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "sticker-pack persistence not configured")
	}
	id, ok := authctx.ProfileID(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	return id, nil
}
func stickerError(err error) error {
	switch {
	case errors.Is(err, store.ErrStickerPackNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, store.ErrStickerPackForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, store.ErrSystemStickerPack):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
func stickerPackToProto(p store.StickerPackRow) *chatv1.StickerPack {
	out := &chatv1.StickerPack{Id: p.ID.String(), Title: p.Title, IsSystem: p.IsSystem, IsPremium: p.IsPremium, StickerCount: p.StickerCount, SortOrder: p.SortOrder}
	if p.ThumbFileID != nil {
		v := p.ThumbFileID.String()
		out.ThumbFileId = &v
	}
	if p.CreatorProfileID != nil {
		v := p.CreatorProfileID.String()
		out.CreatorProfileId = &v
	}
	for _, s := range p.Stickers {
		x := &chatv1.Sticker{Id: s.ID.String(), FileId: s.FileID.String(), SortOrder: s.SortOrder, Width: s.Width, Height: s.Height}
		if s.Emoji != nil {
			x.Emoji = s.Emoji
		}
		out.Stickers = append(out.Stickers, x)
	}
	return out
}
