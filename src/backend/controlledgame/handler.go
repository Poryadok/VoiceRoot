package controlledgame

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

const callbackContentType = "application/vnd.voice.game-command+json;version=1"

type EffectApplier func(context.Context, pgx.Tx, []byte) ([]byte, error)

type HandlerConfig struct {
	Store *PostgresStore
	Keys  map[string][]byte
	Clock func() time.Time
	Apply EffectApplier
}

type callbackCommand struct {
	ActionID        string                     `json:"action_id"`
	ActorProof      callbackActorProof         `json:"actor_proof"`
	AppID           string                     `json:"app_id"`
	Arguments       map[string]json.RawMessage `json:"arguments"`
	BindingRevision int64                      `json:"binding_revision"`
	CardRevision    int64                      `json:"card_revision"`
	CommandID       string                     `json:"command_id"`
	EnvironmentID   string                     `json:"environment_id"`
	ExpiresAt       int64                      `json:"expires_at"`
	InstallationID  string                     `json:"installation_id"`
	InvocationID    string                     `json:"invocation_id"`
	IssuedAt        int64                      `json:"issued_at"`
	MessageID       string                     `json:"message_id"`
	OperationID     string                     `json:"operation_id"`
	SchemaVersion   int                        `json:"schema_version"`
	StateVersion    string                     `json:"state_version"`
}

type callbackActorProof struct {
	ProfileID string `json:"profile_id"`
	ProofID   string `json:"proof_id"`
}

type callbackResult struct {
	CommandID     string `json:"command_id"`
	OperationID   string `json:"operation_id"`
	ResultID      string `json:"result_id"`
	SchemaVersion int    `json:"schema_version"`
	StateVersion  string `json:"state_version"`
	Status        string `json:"status"`
	Summary       string `json:"summary"`
}

func NewHandler(config HandlerConfig) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL == nil || request.URL.RawPath != "" || request.URL.Path != callbackPath || request.URL.RawQuery != "" || request.URL.ForceQuery {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		contentType, okContent := singleHeader(request.Header, "Content-Type")
		keyID, okKey := singleHeader(request.Header, "X-Voice-Key-Id")
		timestamp, okTimestamp := singleHeader(request.Header, "X-Voice-Timestamp")
		signature, okSignature := singleHeader(request.Header, "X-Voice-Signature")
		if !okContent || !okKey || !okTimestamp || !okSignature || contentType != callbackContentType {
			http.Error(response, "invalid callback headers", http.StatusBadRequest)
			return
		}
		if config.Clock == nil {
			http.Error(response, "receiver unavailable", http.StatusInternalServerError)
			return
		}
		key, knownKey := config.Keys[keyID]
		if !knownKey {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(response, "invalid callback body", http.StatusBadRequest)
			return
		}
		if err := authenticateRequest(request.Method, request.URL.EscapedPath(), timestamp, keyID, body, signature, key, config.Clock()); err != nil {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		if config.Store == nil || config.Apply == nil {
			http.Error(response, "receiver unavailable", http.StatusInternalServerError)
			return
		}
		command, err := parseCallbackCommand(body)
		if err != nil {
			http.Error(response, "invalid command envelope", http.StatusUnprocessableEntity)
			return
		}
		result, replay, err := config.Store.accept(request.Context(), command.CommandID, command.OperationID, body,
			func(ctx context.Context, tx pgx.Tx, commandBody []byte) ([]byte, error) {
				resultBody, applyErr := config.Apply(ctx, tx, commandBody)
				if applyErr != nil {
					return nil, applyErr
				}
				if validateCallbackResult(resultBody, command) != nil {
					return nil, errors.New("effect returned invalid command result")
				}
				return resultBody, nil
			})
		if err != nil {
			if errors.Is(err, errCommandConflict) {
				http.Error(response, "command conflict", http.StatusConflict)
				return
			}
			http.Error(response, "receiver unavailable", http.StatusInternalServerError)
			return
		}
		_ = replay // The persisted result body is identical for original and replay.
		response.Header().Set("Content-Type", callbackContentType)
		response.Header().Set("Cache-Control", "no-store")
		response.WriteHeader(http.StatusAccepted)
		_, _ = response.Write(result)
	})
}

func singleHeader(headers http.Header, name string) (string, bool) {
	values := headers.Values(name)
	if len(values) != 1 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

func parseCallbackCommand(body []byte) (callbackCommand, error) {
	var command callbackCommand
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&command); err != nil {
		return callbackCommand{}, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return callbackCommand{}, errInvalidRequest
	}
	ids := []string{command.ActionID, command.CommandID, command.EnvironmentID, command.AppID,
		command.InstallationID, command.InvocationID, command.MessageID, command.OperationID,
		command.ActorProof.ProfileID}
	for _, id := range ids {
		if !canonicalUUID(id) {
			return callbackCommand{}, errInvalidRequest
		}
	}
	if command.SchemaVersion != 1 || command.ActorProof.ProofID == "" || command.Arguments == nil ||
		command.BindingRevision < 0 || command.CardRevision < 0 || command.StateVersion == "" ||
		command.IssuedAt < 0 || command.ExpiresAt <= command.IssuedAt {
		return callbackCommand{}, errInvalidRequest
	}
	return command, nil
}

func validateCallbackResult(body []byte, command callbackCommand) error {
	canonical, err := canonicalJSON(body)
	if err != nil || !bytes.Equal(canonical, body) {
		return errInvalidRequest
	}
	var result callbackResult
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errInvalidRequest
	}
	if result.SchemaVersion != 1 || !canonicalUUID(result.ResultID) || result.CommandID != command.CommandID ||
		result.OperationID != command.OperationID || result.StateVersion == "" || result.Status == "" || result.Summary == "" {
		return errInvalidRequest
	}
	return nil
}
