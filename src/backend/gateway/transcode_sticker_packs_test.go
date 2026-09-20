package main

import (
	"context"
	"net/http"
	"testing"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingStickerPacks struct {
	chatv1.UnimplementedChatServiceServer
	listCalled    bool
	getLast       *chatv1.GetStickerPackRequest
	installLast   *chatv1.InstallStickerPackRequest
	uninstallLast *chatv1.UninstallStickerPackRequest
}

func (s *recordingStickerPacks) ListInstalledStickerPacks(context.Context, *chatv1.ListInstalledStickerPacksRequest) (*chatv1.ListInstalledStickerPacksResponse, error) {
	s.listCalled = true
	return &chatv1.ListInstalledStickerPacksResponse{}, nil
}

func (s *recordingStickerPacks) GetStickerPack(_ context.Context, req *chatv1.GetStickerPackRequest) (*chatv1.GetStickerPackResponse, error) {
	s.getLast = req
	return &chatv1.GetStickerPackResponse{}, nil
}

func (s *recordingStickerPacks) InstallStickerPack(_ context.Context, req *chatv1.InstallStickerPackRequest) (*chatv1.InstallStickerPackResponse, error) {
	s.installLast = req
	return &chatv1.InstallStickerPackResponse{}, nil
}

func (s *recordingStickerPacks) UninstallStickerPack(_ context.Context, req *chatv1.UninstallStickerPackRequest) (*chatv1.UninstallStickerPackResponse, error) {
	s.uninstallLast = req
	return &chatv1.UninstallStickerPackResponse{}, nil
}

func TestTranscodeStickerPacks(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingStickerPacks{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"}},
		transcoder:  &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})
	headers := map[string]string{"Authorization": "Bearer valid-user-token"}

	list := performRequest(h, http.MethodGet, "/api/v1/sticker-packs", "", headers)
	if list.Code != http.StatusOK || !grpcRec.listCalled {
		t.Fatalf("list = %d, called=%v", list.Code, grpcRec.listCalled)
	}
	get := performRequest(h, http.MethodGet, "/api/v1/sticker-packs/pack-1", "", headers)
	if get.Code != http.StatusOK || grpcRec.getLast.GetPackId() != "pack-1" {
		t.Fatalf("get = %d, req=%+v", get.Code, grpcRec.getLast)
	}
	install := performRequest(h, http.MethodPost, "/api/v1/sticker-packs/pack-1/install", "", headers)
	if install.Code != http.StatusOK || grpcRec.installLast.GetPackId() != "pack-1" {
		t.Fatalf("install = %d, req=%+v", install.Code, grpcRec.installLast)
	}
	uninstall := performRequest(h, http.MethodDelete, "/api/v1/sticker-packs/pack-1", "", headers)
	if uninstall.Code != http.StatusNoContent || grpcRec.uninstallLast.GetPackId() != "pack-1" {
		t.Fatalf("uninstall = %d, req=%+v", uninstall.Code, grpcRec.uninstallLast)
	}
}
