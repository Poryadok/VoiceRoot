package grpcsvc

import (
	"context"
	"encoding/json"
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
			botID.String(), chatID.String(), cmdName, optionName, strings.TrimSpace(req.GetFocusedValue()),
		)
		if choices, ok := s.Hub.GetAutocompleteChoices(cacheKey); ok {
			return autocompleteChoicesToProto(choices), nil
		}
		requestID := s.Hub.NextAutocompleteRequestID()
		s.Hub.RegisterAutocomplete(requestID, cacheKey)
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
		}
		if _, err := s.Store.EnqueueEvent(ctx, botID, "autocomplete", payload, ""); err != nil {
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
	choices, err := webhook.DeliverAutocompletePOST(ctx, s.HTTPClient, strings.TrimSpace(*botRow.WebhookURL), botRow.WebhookSecret, payload, dispatch.DefaultTimeout())
	if err != nil {
		if s.Events != nil {
			_ = s.Events.PublishWebhookFailed(ctx, botID.String(), "autocomplete", err.Error())
		}
		return &botv1.AutocompleteSlashOptionResponse{}, nil
	}
	out := make([]*botv1.AutocompleteChoice, 0, len(choices))
	for _, ch := range choices {
		out = append(out, &botv1.AutocompleteChoice{Name: ch.Name, Value: ch.Value})
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
	if s.Hub == nil || !s.Hub.CompleteAutocomplete(requestID, botRow.ID.String(), choices) {
		return nil, status.Error(codes.NotFound, "unknown autocomplete request")
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
