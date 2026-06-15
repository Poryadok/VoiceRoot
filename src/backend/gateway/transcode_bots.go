package main

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func (t *transcoder) serveBots(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t.clients.bot == nil {
		return false
	}

	if strings.HasPrefix(rest, "me/") {
		ctx := withBotGRPCMetadata(r)
		meRest := strings.TrimPrefix(rest, "me/")
		switch {
		case r.Method == http.MethodGet && meRest == "interactions/poll":
			stream, err := t.clients.bot.PollEvents(ctx, &botv1.PollEventsRequest{})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			var events []*botv1.BotEvent
			for {
				msg, err := stream.Recv()
				if err != nil {
					break
				}
				if msg.GetBotEvent() != nil {
					events = append(events, msg.GetBotEvent())
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{"events": events})
			return true

		case r.Method == http.MethodPost && meRest == "interactions/complete":
			req := &botv1.CompleteInteractionRequest{}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			_, err := t.clients.bot.CompleteInteraction(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true

		case r.Method == http.MethodPost && meRest == "interactions/defer":
			req := &botv1.DeferResponseRequest{}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			_, err := t.clients.bot.DeferResponse(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true

		case r.Method == http.MethodPost && meRest == "messages":
			req := &botv1.SendBotMessageRequest{}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			resp, err := t.clients.bot.SendBotMessage(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true

		case r.Method == http.MethodPatch && strings.HasPrefix(meRest, "messages/"):
			messageID := strings.TrimPrefix(meRest, "messages/")
			messageID = strings.Trim(messageID, "/")
			req := &botv1.EditBotMessageRequest{MessageId: messageID}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			req.MessageId = messageID
			resp, err := t.clients.bot.EditBotMessage(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		}
		return false
	}

	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodPost && rest == "":
		req := &botv1.RegisterBotRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.bot.RegisterBot(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "":
		resp, err := t.clients.bot.ListBots(ctx, &botv1.ListBotsRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "manifest/validate":
		req := &botv1.ValidateManifestRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.bot.ValidateManifest(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "interactions":
		req := &botv1.ExecuteSlashInteractionRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.bot.ExecuteSlashInteraction(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "commands":
		chatID := queryFirst(r, "chat_id")
		chatType := queryFirst(r, "chat_type")
		if chatType == "" {
			chatType = "CHAT_TYPE_CHANNEL"
		}
		ref := &chatv1.ChatRef{Id: chatID, Type: chatTypePtr(chatTypeToEnum(chatType))}
		resp, err := t.clients.bot.ListSlashCommandsForChat(ctx, &botv1.ListSlashCommandsForChatRequest{Chat: ref})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/manifest"):
		botID := strings.TrimSuffix(rest, "/manifest")
		botID = strings.Trim(botID, "/")
		req := &botv1.ApplyManifestRequest{BotId: botID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.BotId = botID
		resp, err := t.clients.bot.ApplyManifest(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/token/regenerate"):
		botID := strings.TrimSuffix(rest, "/token/regenerate")
		botID = strings.Trim(botID, "/")
		resp, err := t.clients.bot.RegenerateToken(ctx, &botv1.RegenerateTokenRequest{BotId: botID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "autocomplete":
		req := &botv1.AutocompleteSlashOptionRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.bot.AutocompleteSlashOption(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && strings.HasPrefix(rest, "spaces/") && strings.HasSuffix(rest, "/installed"):
		spaceID := strings.TrimSuffix(strings.TrimPrefix(rest, "spaces/"), "/installed")
		spaceID = strings.Trim(spaceID, "/")
		resp, err := t.clients.bot.ListInstalledBots(ctx, &botv1.ListInstalledBotsRequest{SpaceId: spaceID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case strings.HasSuffix(rest, "/webhook"):
		botID := strings.TrimSuffix(rest, "/webhook")
		botID = strings.Trim(botID, "/")
		switch r.Method {
		case http.MethodGet:
			resp, err := t.clients.bot.GetWebhookURL(ctx, &botv1.GetWebhookURLRequest{BotId: botID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		case http.MethodPatch:
			req := &botv1.SetWebhookURLRequest{BotId: botID}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			req.BotId = botID
			_, err := t.clients.bot.SetWebhookURL(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		}
		return false

	case r.Method == http.MethodDelete && strings.Contains(rest, "/spaces/") && !strings.HasSuffix(rest, "/install"):
		parts := strings.Split(strings.Trim(rest, "/"), "/")
		if len(parts) == 3 && parts[1] == "spaces" {
			_, err := t.clients.bot.UninstallBotFromSpace(ctx, &botv1.UninstallBotFromSpaceRequest{
				BotId:   parts[0],
				SpaceId: parts[2],
			})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		}
		return false

	case r.Method == http.MethodGet && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.bot.GetBot(ctx, &botv1.GetBotRequest{BotId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest != "" && !strings.Contains(rest, "/"):
		req := &botv1.UpdateBotRequest{BotId: rest}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.BotId = rest
		resp, err := t.clients.bot.UpdateBot(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest != "" && !strings.Contains(rest, "/"):
		_, err := t.clients.bot.DeleteBot(ctx, &botv1.DeleteBotRequest{BotId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && strings.Contains(rest, "/spaces/") && strings.HasSuffix(rest, "/install"):
		parts := strings.Split(strings.Trim(rest, "/"), "/")
		if len(parts) < 4 {
			return false
		}
		botID := parts[0]
		spaceID := parts[2]
		req := &botv1.InstallBotInSpaceRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.BotId = botID
		req.SpaceId = spaceID
		resp, err := t.clients.bot.InstallBotInSpace(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		return false
	}
}

func chatTypePtr(t chatv1.ChatType) *chatv1.ChatType {
	v := t
	return &v
}

func chatTypeToEnum(raw string) chatv1.ChatType {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "CHAT_TYPE_GROUP", "GROUP":
		return chatv1.ChatType_CHAT_TYPE_GROUP
	case "CHAT_TYPE_CHANNEL", "CHANNEL":
		return chatv1.ChatType_CHAT_TYPE_CHANNEL
	case "CHAT_TYPE_DM", "DM":
		return chatv1.ChatType_CHAT_TYPE_DM
	default:
		return chatv1.ChatType_CHAT_TYPE_CHANNEL
	}
}

func withBotGRPCMetadata(r *http.Request) context.Context {
	ctx := r.Context()
	token := botBearerToken(r)
	if token != "" {
		md := metadata.Pairs("x-voice-bot-token", token)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}

func botBearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) > 4 && strings.EqualFold(h[:4], "bot ") {
		return strings.TrimSpace(h[4:])
	}
	return ""
}
