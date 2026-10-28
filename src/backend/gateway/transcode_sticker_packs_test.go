package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
)

type recordingStickerPacks struct {
	chatv1.UnimplementedChatServiceServer
	listCalled    bool
	getLast       *chatv1.GetStickerPackRequest
	installLast   *chatv1.InstallStickerPackRequest
	uninstallLast *chatv1.UninstallStickerPackRequest
	err           error
}

func (s *recordingStickerPacks) ListInstalledStickerPacks(context.Context, *chatv1.ListInstalledStickerPacksRequest) (*chatv1.ListInstalledStickerPacksResponse, error) {
	s.listCalled = true
	return &chatv1.ListInstalledStickerPacksResponse{Packs: []*chatv1.StickerPack{{Id: "pack-1", Title: "First pack"}}}, s.err
}

func (s *recordingStickerPacks) GetStickerPack(_ context.Context, req *chatv1.GetStickerPackRequest) (*chatv1.GetStickerPackResponse, error) {
	s.getLast = req
	return &chatv1.GetStickerPackResponse{Pack: &chatv1.StickerPack{Id: req.GetPackId(), Title: "First pack"}}, s.err
}

func (s *recordingStickerPacks) InstallStickerPack(_ context.Context, req *chatv1.InstallStickerPackRequest) (*chatv1.InstallStickerPackResponse, error) {
	s.installLast = req
	return &chatv1.InstallStickerPackResponse{Pack: &chatv1.StickerPack{Id: req.GetPackId(), Title: "First pack"}}, s.err
}

func (s *recordingStickerPacks) UninstallStickerPack(_ context.Context, req *chatv1.UninstallStickerPackRequest) (*chatv1.UninstallStickerPackResponse, error) {
	s.uninstallLast = req
	return &chatv1.UninstallStickerPackResponse{}, s.err
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
	assertStickerPackJSON(t, list.Body.Bytes(), "packs")
	get := performRequest(h, http.MethodGet, "/api/v1/sticker-packs/pack-1", "", headers)
	if get.Code != http.StatusOK || grpcRec.getLast.GetPackId() != "pack-1" {
		t.Fatalf("get = %d, req=%+v", get.Code, grpcRec.getLast)
	}
	assertStickerPackJSON(t, get.Body.Bytes(), "pack")
	install := performRequest(h, http.MethodPost, "/api/v1/sticker-packs/pack-1/install", "", headers)
	if install.Code != http.StatusOK || grpcRec.installLast.GetPackId() != "pack-1" {
		t.Fatalf("install = %d, req=%+v", install.Code, grpcRec.installLast)
	}
	assertStickerPackJSON(t, install.Body.Bytes(), "pack")
	uninstall := performRequest(h, http.MethodDelete, "/api/v1/sticker-packs/pack-1", "", headers)
	if uninstall.Code != http.StatusNoContent || grpcRec.uninstallLast.GetPackId() != "pack-1" {
		t.Fatalf("uninstall = %d, req=%+v", uninstall.Code, grpcRec.uninstallLast)
	}
	if uninstall.Body.Len() != 0 {
		t.Fatalf("uninstall body = %q, want empty", uninstall.Body.String())
	}
}

func assertStickerPackJSON(t *testing.T, body []byte, field string) {
	t.Helper()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("response JSON = %q: %v", body, err)
	}
	pack := payload[field]
	if field == "packs" {
		var packs []json.RawMessage
		if err := json.Unmarshal(pack, &packs); err != nil || len(packs) != 1 {
			t.Fatalf("packs = %s, err = %v", pack, err)
		}
		pack = packs[0]
	}
	var got struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(pack, &got); err != nil || got.ID != "pack-1" || got.Title != "First pack" {
		t.Fatalf("pack = %s, decoded = %+v, err = %v", pack, got, err)
	}
}

func TestTranscodeStickerPacksGRPCErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, method, path string
		code               codes.Code
		wantHTTP           int
		wantError          string
	}{
		{"list unauthenticated", http.MethodGet, "/api/v1/sticker-packs", codes.Unauthenticated, http.StatusUnauthorized, "unauthenticated"},
		{"get missing", http.MethodGet, "/api/v1/sticker-packs/pack-1", codes.NotFound, http.StatusNotFound, "not_found"},
		{"install forbidden", http.MethodPost, "/api/v1/sticker-packs/pack-1/install", codes.PermissionDenied, http.StatusForbidden, "permission_denied"},
		{"uninstall system pack", http.MethodDelete, "/api/v1/sticker-packs/pack-1", codes.PermissionDenied, http.StatusForbidden, "permission_denied"},
		{"invalid pack ID", http.MethodGet, "/api/v1/sticker-packs/not-a-uuid", codes.InvalidArgument, http.StatusBadRequest, "invalid_argument"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingStickerPacks{err: status.Error(tc.code, "sticker pack rejected")}
			conn, cleanup := startBufconnChatConn(t, rec)
			t.Cleanup(cleanup)
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"}},
				transcoder:  &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
			})
			resp := performRequest(h, tc.method, tc.path, "", map[string]string{"Authorization": "Bearer valid-user-token"})
			if resp.Code != tc.wantHTTP || resp.Header().Get("X-Voice-GRPC-Code") != tc.code.String() {
				t.Fatalf("status = %d, grpc code = %q, want %d/%s; body = %s", resp.Code, resp.Header().Get("X-Voice-GRPC-Code"), tc.wantHTTP, tc.code, resp.Body.String())
			}
			var body struct {
				ErrorCode string `json:"error_code"`
				Message   string `json:"message"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil || body.ErrorCode != tc.wantError || body.Message != "sticker pack rejected" {
				t.Fatalf("error body = %s, decoded = %+v, err = %v", resp.Body.String(), body, err)
			}
		})
	}
}

func TestTranscodeStickerPacksRejectsMalformedPathsWithoutRPC(t *testing.T) {
	t.Parallel()
	rec := &recordingStickerPacks{}
	conn, cleanup := startBufconnChatConn(t, rec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"}},
		transcoder:  &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})
	headers := map[string]string{"Authorization": "Bearer valid-user-token"}
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/sticker-packs/pack-1/extra"},
		{http.MethodPost, "/api/v1/sticker-packs/pack-1/extra/install"},
		{http.MethodDelete, "/api/v1/sticker-packs/pack-1/extra"},
	} {
		resp := performRequest(h, tc.method, tc.path, "", headers)
		if resp.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want 404", tc.method, tc.path, resp.Code)
		}
	}
	if rec.listCalled || rec.getLast != nil || rec.installLast != nil || rec.uninstallLast != nil {
		t.Fatalf("malformed routes reached Chat RPC: %+v", rec)
	}
}
