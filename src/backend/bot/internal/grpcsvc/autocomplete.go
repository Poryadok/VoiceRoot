package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/store"
	"voice/backend/bot/internal/webhook"

	botv1 "voice.app/voice/bot/v1"
)

func (s *BotGRPC) AutocompleteSlashOption(ctx context.Context, req *botv1.AutocompleteSlashOptionRequest) (*botv1.AutocompleteSlashOptionResponse, error) {
	invoker, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	allowed, err := s.Store.IsChatWhitelisted(ctx, botID, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !allowed {
		return nil, status.Error(codes.PermissionDenied, "bot not enabled in chat")
	}
	if err := s.requireInvokerMembership(ctx, chatID, invoker); err != nil {
		return nil, err
	}
	botRow, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	if err := s.requireScope(botRow, "TEXT_CHAT_SEND_MESSAGES"); err != nil {
		return nil, err
	}
	if !botRow.IsPollingMode && (botRow.WebhookURL == nil || strings.TrimSpace(*botRow.WebhookURL) == "") {
		return nil, status.Error(codes.FailedPrecondition, "webhook is not configured")
	}
	if chatRefIsDM(req.GetChat()) {
		if err := s.requireScope(botRow, "DM_SEND"); err != nil {
			return nil, err
		}
	}
	if !s.isBotOnline(ctx, botID) {
		// Mirror ExecuteSlashInteraction: offline bots cannot serve autocomplete (bots.md status).
		return &botv1.AutocompleteSlashOptionResponse{}, nil
	}
	cmdName := strings.TrimPrefix(strings.TrimSpace(req.GetCommandName()), "/")
	optionName := strings.TrimSpace(req.GetOptionName())
	if cmdName == "" || optionName == "" {
		return nil, status.Error(codes.InvalidArgument, "command_name and option_name required")
	}
	commands, err := s.Store.ListCommands(ctx, botID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var params []manifestOption
	found := false
	for _, c := range commands {
		if c.Name != cmdName {
			continue
		}
		found = true
		_ = json.Unmarshal([]byte(c.Parameters), &params)
		break
	}
	if !found {
		return nil, status.Error(codes.NotFound, "unknown command")
	}
	autocomplete := false
	for _, o := range params {
		if o.Name == optionName {
			if o.Type != "string" {
				return nil, status.Error(codes.InvalidArgument, "only string options support autocomplete")
			}
			autocomplete = o.Autocomplete
			break
		}
	}
	if !autocomplete {
		return nil, status.Error(codes.InvalidArgument, "option is not autocomplete")
	}
	if botRow.IsPollingMode || botRow.WebhookURL == nil || strings.TrimSpace(*botRow.WebhookURL) == "" {
		if !store.DevPollingEnabled() {
			return nil, status.Error(codes.FailedPrecondition, "development polling is disabled")
		}
		cacheKey := dispatch.AutocompleteCacheKey(
			botID.String(), chatID.String(), cmdName, optionName, strings.TrimSpace(req.GetFocusedValue()), botRow.SessionEpoch,
		)
		var cached []dispatch.AutocompleteChoice
		var found bool
		err = s.withBotSession(ctx, botRow, chatID, func(call context.Context, tx pgx.Tx, current *store.BotRow) error {
			if e := s.requireScope(current, "TEXT_CHAT_SEND_MESSAGES"); e != nil {
				return e
			}
			if chatRefIsDM(req.GetChat()) {
				if e := s.requireScope(current, "DM_SEND"); e != nil {
					return e
				}
			}
			if e := s.requireInvokerMembership(call, chatID, invoker); e != nil {
				return e
			}
			space, held := store.GuardedChatSpace(call)
			if !held {
				return status.Error(codes.NotFound, "autocomplete not found")
			}
			var raw []byte
			e := tx.QueryRow(call, `SELECT payload->'autocomplete_choices' FROM bot_event_log WHERE bot_id=$1 AND session_epoch=$2 AND event_type='autocomplete' AND delivery_status='delivered' AND payload->>'cache_key'=$3 AND space_id=$4 AND payload->>'invoker_profile_id'=$5 AND payload ? 'autocomplete_choices' ORDER BY created_at DESC LIMIT 1`, botID, botRow.SessionEpoch, cacheKey, space, invoker.String()).Scan(&raw)
			if errors.Is(e, pgx.ErrNoRows) {
				return nil
			}
			if e != nil {
				return e
			}
			if e = json.Unmarshal(raw, &cached); e != nil {
				return e
			}
			found = true
			return nil
		})
		if err != nil {
			return nil, err
		}
		if found {
			return autocompleteChoicesToProto(cached), nil
		}
		requestID := s.Hub.NextAutocompleteRequestID()
		s.Hub.RegisterAutocomplete(requestID, cacheKey, botRow.Session())
		var options map[string]any
		_ = json.Unmarshal([]byte(req.GetOptionsJson()), &options)
		payload := map[string]any{
			"type":               "autocomplete",
			"request_id":         requestID,
			"command_name":       cmdName,
			"option_name":        optionName,
			"focused_value":      req.GetFocusedValue(),
			"options":            options,
			"chat_id":            chatID.String(),
			"chat_type":          req.GetChat().GetType().String(),
			"invoker_profile_id": invoker.String(),
			"cache_key":          cacheKey,
		}
		if _, err := s.Store.EnqueueEvent(ctx, botRow.Session(), "autocomplete", payload, requestID); err != nil {
			s.Hub.CancelAutocomplete(requestID)
			return nil, status.Error(codes.Unavailable, "autocomplete could not be accepted")
		}
		pending := true
		return &botv1.AutocompleteSlashOptionResponse{Pending: &pending}, nil
	}
	var options map[string]any
	_ = json.Unmarshal([]byte(req.GetOptionsJson()), &options)
	payload := webhook.InteractionPayload{
		Type:             "autocomplete",
		CommandName:      cmdName,
		OptionName:       optionName,
		FocusedValue:     req.GetFocusedValue(),
		Options:          options,
		ChatID:           chatID.String(),
		ChatType:         req.GetChat().GetType().String(),
		InvokerProfileID: invoker.String(),
	}
	out := make([]*botv1.AutocompleteChoice, 0)
	requestID := s.Hub.NextAutocompleteRequestID()
	_, err = s.Store.EnqueueEvent(ctx, botRow.Session(), "autocomplete", map[string]any{"request_id": requestID, "chat_id": chatID.String(), "chat_type": req.GetChat().GetType().String(), "invoker_profile_id": invoker.String(), "command_name": cmdName, "option_name": optionName, "focused_value": req.GetFocusedValue(), "options": options}, requestID)
	if err != nil {
		return nil, requestSessionError(ctx, err)
	}
	err = s.withAutocompleteSession(ctx, botRow, requestID, func(call context.Context, tx pgx.Tx, current *store.BotRow, _ map[string]any) error {
		if current.IsPollingMode || current.WebhookURL == nil {
			return status.Error(codes.NotFound, "autocomplete not found")
		}
		client := webhook.GuardedClient(s.HTTPClient, func(request context.Context) error { return s.Store.CheckGuardedSession(request, current.Session()) })
		choices, e := webhook.DeliverAutocompletePOST(call, client, strings.TrimSpace(*current.WebhookURL), current.WebhookSecret, payload, dispatch.DefaultTimeout())
		state := "delivered"
		if e != nil {
			state = "failed"
		} else {
			for _, choice := range choices {
				out = append(out, &botv1.AutocompleteChoice{Name: choice.Name, Value: choice.Value})
			}
		}
		_, e = tx.Exec(call, `UPDATE bot_event_log SET delivery_status=$3,delivered_at=clock_timestamp() WHERE bot_id=$1 AND interaction_token=$2`, botID, requestID, state)
		return e
	})
	if err != nil {
		return nil, err
	}
	return &botv1.AutocompleteSlashOptionResponse{Choices: out}, nil
}

func (s *BotGRPC) CompleteAutocomplete(ctx context.Context, req *botv1.CompleteAutocompleteRequest) (*botv1.CompleteAutocompleteResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if !botRow.IsPollingMode || !store.DevPollingEnabled() {
		return nil, status.Error(codes.FailedPrecondition, "development polling is disabled")
	}
	requestID := strings.TrimSpace(req.GetRequestId())
	if requestID == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id required")
	}
	choices := make([]dispatch.AutocompleteChoice, 0, len(req.GetChoices()))
	for _, ch := range req.GetChoices() {
		choices = append(choices, dispatch.AutocompleteChoice{
			Name:  strings.TrimSpace(ch.GetName()),
			Value: strings.TrimSpace(ch.GetValue()),
		})
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	err = s.withAutocompleteSession(ctx, botRow, requestID, func(call context.Context, tx pgx.Tx, current *store.BotRow, binding map[string]any) error {
		if _, completed := binding["autocomplete_choices"]; completed {
			return status.Error(codes.NotFound, "autocomplete not found")
		}
		if !current.IsPollingMode || !store.DevPollingEnabled() {
			return status.Error(codes.FailedPrecondition, "development polling is disabled")
		}
		raw, e := json.Marshal(choices)
		if e != nil {
			return e
		}
		_, e = tx.Exec(call, `UPDATE bot_event_log SET payload=jsonb_set(payload,'{autocomplete_choices}',$3::jsonb),delivery_status='delivered',delivered_at=clock_timestamp() WHERE bot_id=$1 AND interaction_token=$2`, current.ID, requestID, string(raw))
		if e == nil && s.Hub != nil {
			s.Hub.CompleteAutocomplete(requestID, current.ID.String(), current.Session(), choices)
		}
		return e
	})
	if err != nil {
		return nil, err
	}
	return &botv1.CompleteAutocompleteResponse{}, nil
}

func autocompleteChoicesToProto(choices []dispatch.AutocompleteChoice) *botv1.AutocompleteSlashOptionResponse {
	out := make([]*botv1.AutocompleteChoice, 0, len(choices))
	for _, ch := range choices {
		out = append(out, &botv1.AutocompleteChoice{Name: ch.Name, Value: ch.Value})
	}
	return &botv1.AutocompleteSlashOptionResponse{Choices: out}
}
