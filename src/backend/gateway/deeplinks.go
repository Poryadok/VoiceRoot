package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

type resolveDeepLinkResponse struct {
	Kind        string `json:"kind"`
	SpaceID     string `json:"space_id,omitempty"`
	ChatID      string `json:"chat_id,omitempty"`
	VoiceRoomID string `json:"voice_room_id,omitempty"`
	MessageID   string `json:"message_id,omitempty"`
	InviteCode  string `json:"invite_code,omitempty"`
	Username    string `json:"username,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	AppURI      string `json:"app_uri,omitempty"`
	WebPath     string `json:"web_path,omitempty"`
}

func publicWebOrigin() string {
	if v := strings.TrimSpace(os.Getenv("GATEWAY_PUBLIC_WEB_ORIGIN")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://voice.gg"
}

func (g *gateway) handleDeepLinkPublic(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.Trim(path, "/")
	if path == "" {
		return false
	}

	target, err := ParseDeepLinkURL("https://voice.gg/" + path)
	if err != nil {
		return false
	}

	switch target.Kind {
	case DeepLinkKindInvite, DeepLinkKindSpace, DeepLinkKindSpaceChat, DeepLinkKindVoiceRoom,
		DeepLinkKindSpaceMessage, DeepLinkKindChat, DeepLinkKindChatMessage,
		DeepLinkKindProfile, DeepLinkKindDM:
		g.writeDeepLinkBridgeHTML(w, target)
		return true
	default:
		return false
	}
}

func (g *gateway) writeDeepLinkBridgeHTML(w http.ResponseWriter, target DeepLinkTarget) {
	appURI := deepLinkAppURI(target)
	webPath := deepLinkWebPath(target)
	webURL := publicWebOrigin() + webPath
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Voice</title>
<script>
(function(){
  var app=%q, web=%q;
  try { window.location.href = app; } catch (e) {}
  setTimeout(function(){ window.location.href = web; }, 300);
})();
</script></head>
<body><p><a href=%q>Open in Voice</a></p></body></html>`, appURI, webURL, webURL)
}

func deepLinkAppURI(t DeepLinkTarget) string {
	switch t.Kind {
	case DeepLinkKindInvite:
		return "voice://invite/" + t.InviteCode
	case DeepLinkKindSpace:
		return "voice://s/" + t.SpaceID
	case DeepLinkKindSpaceChat:
		return "voice://s/" + t.SpaceID + "/c/" + t.ChatID
	case DeepLinkKindVoiceRoom:
		return "voice://s/" + t.SpaceID + "/v/" + t.VoiceRoomID
	case DeepLinkKindSpaceMessage:
		return "voice://s/" + t.SpaceID + "/c/" + t.ChatID + "/m/" + t.MessageID
	case DeepLinkKindChat:
		return "voice://ch/" + t.ChatID
	case DeepLinkKindChatMessage:
		return "voice://ch/" + t.ChatID + "/m/" + t.MessageID
	case DeepLinkKindProfile:
		return "voice://u/" + t.Username
	case DeepLinkKindDM:
		return "voice://dm/" + t.UserID
	default:
		return "voice://"
	}
}

func deepLinkWebPath(t DeepLinkTarget) string {
	switch t.Kind {
	case DeepLinkKindInvite:
		return "/invite/" + t.InviteCode
	case DeepLinkKindSpace:
		return "/s/" + t.SpaceID
	case DeepLinkKindSpaceChat:
		return "/s/" + t.SpaceID + "/c/" + t.ChatID
	case DeepLinkKindVoiceRoom:
		return "/s/" + t.SpaceID + "/v/" + t.VoiceRoomID
	case DeepLinkKindSpaceMessage:
		return "/s/" + t.SpaceID + "/c/" + t.ChatID + "/m/" + t.MessageID
	case DeepLinkKindChat:
		return "/ch/" + t.ChatID
	case DeepLinkKindChatMessage:
		return "/ch/" + t.ChatID + "/m/" + t.MessageID
	case DeepLinkKindProfile:
		return "/u/" + t.Username
	case DeepLinkKindDM:
		return "/dm/" + t.UserID
	default:
		return "/"
	}
}

func (g *gateway) handleWellKnown(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/.well-known/apple-app-site-association":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"applinks":{"apps":[],"details":[{"appID":"TEAMID.gg.voice.app","paths":["/invite/*","/s/*","/ch/*","/u/*","/dm/*"]}]}}`))
		return true
	case "/.well-known/assetlinks.json":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"gg.voice.app","sha256_cert_fingerprints":["PLACEHOLDER"]}}]`))
		return true
	default:
		return false
	}
}

func (t *transcoder) serveLinks(w http.ResponseWriter, r *http.Request, rest string) bool {
	if r.Method != http.MethodGet || rest != "resolve" {
		return false
	}
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing url parameter"})
		return true
	}
	target, err := ParseDeepLinkURL(rawURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid deep link url"})
		return true
	}
	ctx := withGRPCMetadata(r.Context(), r)
	resp, err := t.resolveDeepLink(ctx, target)
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	writeJSON(w, http.StatusOK, resp)
	return true
}

func (t *transcoder) resolveDeepLink(ctx context.Context, target DeepLinkTarget) (resolveDeepLinkResponse, error) {
	out := resolveDeepLinkResponse{
		Kind:        string(target.Kind),
		SpaceID:     target.SpaceID,
		ChatID:      target.ChatID,
		VoiceRoomID: target.VoiceRoomID,
		MessageID:   target.MessageID,
		InviteCode:  target.InviteCode,
		Username:    target.Username,
		UserID:      target.UserID,
		AppURI:      deepLinkAppURI(target),
		WebPath:     deepLinkWebPath(target),
	}

	switch target.Kind {
	case DeepLinkKindInvite:
		if t.clients.space == nil {
			return out, status.Error(codes.Unavailable, "space unavailable")
		}
		resp, err := t.clients.space.GetInvite(ctx, &spacev1.GetInviteRequest{Code: target.InviteCode})
		if err != nil {
			return out, err
		}
		out.SpaceID = resp.GetInvite().GetSpaceId()
		out.InviteCode = resp.GetInvite().GetCode()

	case DeepLinkKindSpace:
		if t.clients.space == nil {
			return out, status.Error(codes.Unavailable, "space unavailable")
		}
		if _, err := t.clients.space.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: target.SpaceID}); err != nil {
			return out, err
		}

	case DeepLinkKindSpaceChat, DeepLinkKindSpaceMessage:
		if t.clients.space == nil || t.clients.chat == nil {
			return out, status.Error(codes.Unavailable, "upstream unavailable")
		}
		if _, err := t.clients.space.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: target.SpaceID}); err != nil {
			return out, err
		}
		if _, err := t.clients.chat.GetChat(ctx, &chatv1.GetChatRequest{ChatId: target.ChatID}); err != nil {
			return out, err
		}

	case DeepLinkKindVoiceRoom:
		if t.clients.space == nil {
			return out, status.Error(codes.Unavailable, "space unavailable")
		}
		tree, err := t.clients.space.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: target.SpaceID})
		if err != nil {
			return out, err
		}
		found := false
		for _, room := range tree.GetVoiceRooms() {
			if room.GetId() == target.VoiceRoomID {
				found = true
				break
			}
		}
		if !found {
			return out, status.Error(codes.NotFound, "voice room not found")
		}

	case DeepLinkKindChat, DeepLinkKindChatMessage:
		if t.clients.chat == nil {
			return out, status.Error(codes.Unavailable, "chat unavailable")
		}
		if _, err := t.clients.chat.GetChat(ctx, &chatv1.GetChatRequest{ChatId: target.ChatID}); err != nil {
			return out, err
		}

	case DeepLinkKindProfile:
		if t.clients.user == nil {
			return out, status.Error(codes.Unavailable, "user unavailable")
		}
		if _, err := t.clients.user.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_Username{Username: target.Username},
		}); err != nil {
			return out, err
		}

	case DeepLinkKindDM:
		if t.clients.user == nil {
			return out, status.Error(codes.Unavailable, "user unavailable")
		}
		if _, err := t.clients.user.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_ProfileId{ProfileId: target.UserID},
		}); err != nil {
			return out, err
		}

	default:
		return out, status.Error(codes.InvalidArgument, "unsupported deep link kind")
	}

	return out, nil
}
