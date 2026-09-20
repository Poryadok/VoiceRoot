package main

import (
	"context"
	"net/http"
	"strings"

	chatv1 "voice.app/voice/chat/v1"
)

func (t *transcoder) serveStickerPackRoutes(w http.ResponseWriter, r *http.Request, ctx context.Context, rest string) bool {
	// serveNamespace has already removed /api/v1/sticker-packs; accepting the
	// prefix as well keeps this helper directly testable.
	rest = strings.Trim(strings.TrimPrefix(rest, "sticker-packs"), "/")
	switch {
	case r.Method == http.MethodGet && rest == "":
		resp, err := t.clients.chat.ListInstalledStickerPacks(ctx, &chatv1.ListInstalledStickerPacksRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	case r.Method == http.MethodGet && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.chat.GetStickerPack(ctx, &chatv1.GetStickerPackRequest{PackId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/install"):
		id := strings.Trim(strings.TrimSuffix(rest, "/install"), "/")
		if id == "" || strings.Contains(id, "/") {
			return false
		}
		resp, err := t.clients.chat.InstallStickerPack(ctx, &chatv1.InstallStickerPackRequest{PackId: id})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	case r.Method == http.MethodDelete && rest != "" && !strings.Contains(rest, "/"):
		_, err := t.clients.chat.UninstallStickerPack(ctx, &chatv1.UninstallStickerPackRequest{PackId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	default:
		return false
	}
}
