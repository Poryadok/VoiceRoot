package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"
)

type stickerPackStoreFake struct {
	profileID    uuid.UUID
	uninstallErr error
}

func (f *stickerPackStoreFake) ListInstalledStickerPacks(_ context.Context, id uuid.UUID) ([]store.StickerPackRow, error) {
	f.profileID = id
	return []store.StickerPackRow{}, nil
}
func (f *stickerPackStoreFake) GetStickerPack(context.Context, uuid.UUID, uuid.UUID) (*store.StickerPackRow, error) {
	return nil, store.ErrStickerPackNotFound
}
func (f *stickerPackStoreFake) InstallStickerPack(context.Context, uuid.UUID, uuid.UUID) (*store.StickerPackRow, error) {
	return nil, store.ErrStickerPackForbidden
}
func (f *stickerPackStoreFake) UninstallStickerPack(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return f.uninstallErr
}

func stickerContext(profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, profileID.String()))
}

func TestListInstalledStickerPacks_UsesActiveProfileOnly(t *testing.T) {
	profileID := uuid.New()
	fake := &stickerPackStoreFake{}
	service := &ChatGRPC{StickerPacks: fake}
	_, err := service.ListInstalledStickerPacks(stickerContext(profileID), &chatv1.ListInstalledStickerPacksRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if fake.profileID != profileID {
		t.Fatalf("profile = %s, want active %s", fake.profileID, profileID)
	}
}

func TestUninstallStickerPack_RejectsSystemPack(t *testing.T) {
	fake := &stickerPackStoreFake{uninstallErr: store.ErrSystemStickerPack}
	service := &ChatGRPC{StickerPacks: fake}
	_, err := service.UninstallStickerPack(stickerContext(uuid.New()), &chatv1.UninstallStickerPackRequest{PackId: uuid.NewString()})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code = %s, want %s (%v)", status.Code(err), codes.PermissionDenied, err)
	}
}
