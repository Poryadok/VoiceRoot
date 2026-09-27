package controlledgame

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

const callbackContentType = "application/vnd.voice.game-command+json;version=1"
const maxCallbackBodyBytes = 64 * 1024

type CredentialStatus string

const (
	CredentialCurrent CredentialStatus = "current"
	CredentialOverlap CredentialStatus = "overlap"
	CredentialRevoked CredentialStatus = "revoked"
)

type SigningCredential struct {
	KeyID          string
	AppID          string
	EnvironmentID  string
	InstallationID string
	Secret         []byte
	Status         CredentialStatus
	NotAfter       time.Time
}

type EffectApplier func(context.Context, pgx.Tx, []byte) ([]byte, error)

type commandAcceptor interface {
	accept(context.Context, callbackCommand, []byte, PermitAuthority, BindingAuthorizer, EffectApplier) ([]byte, bool, error)
}

type HandlerConfig struct {
	Store           commandAcceptor
	PermitAuthority PermitAuthority
	Authorize       BindingAuthorizer
	Credentials     map[string]SigningCredential
	Clock           func() time.Time
	Apply           EffectApplier
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
		body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, maxCallbackBodyBytes+1))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) || tooLarge == nil && len(body) > maxCallbackBodyBytes {
				http.Error(response, "callback body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(response, "invalid callback body", http.StatusBadRequest)
			return
		}
		if len(body) > maxCallbackBodyBytes {
			http.Error(response, "callback body too large", http.StatusRequestEntityTooLarge)
			return
		}
		command, err := parseCallbackCommand(body)
		if err != nil {
			http.Error(response, "invalid command envelope", http.StatusUnprocessableEntity)
			return
		}
		credential, knownKey := config.Credentials[keyID]
		now := config.Clock()
		if !knownKey || !credentialUsable(credential, keyID, command, now) {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := authenticateRequest(request.Method, request.URL.EscapedPath(), timestamp, keyID, body, signature, credential.Secret, now); err != nil {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		if config.Store == nil || config.PermitAuthority == nil || config.Authorize == nil || config.Apply == nil {
			http.Error(response, "receiver unavailable", http.StatusInternalServerError)
			return
		}
		result, replay, err := config.Store.accept(request.Context(), command, body, config.PermitAuthority, config.Authorize,
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
			if errors.Is(err, errCommandExpired) || errors.Is(err, errPermitWindowExpired) {
				http.Error(response, "command expired", http.StatusGone)
				return
			}
			if errors.Is(err, errPermitDenied) || errors.Is(err, errAuthorizationDenied) {
				http.Error(response, "command not authorized", http.StatusForbidden)
				return
			}
			if errors.Is(err, errPermitUnavailable) {
				http.Error(response, "authority unavailable", http.StatusServiceUnavailable)
				return
			}
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

func credentialUsable(credential SigningCredential, keyID string, command callbackCommand, now time.Time) bool {
	if credential.KeyID != keyID || credential.AppID != command.AppID || credential.EnvironmentID != command.EnvironmentID ||
		credential.InstallationID != command.InstallationID || len(credential.Secret) < 32 {
		return false
	}
	switch credential.Status {
	case CredentialCurrent:
		return true
	case CredentialOverlap:
		return !credential.NotAfter.IsZero() && now.Before(credential.NotAfter)
	default:
		return false
	}
}

func oneShotEffectKey(command callbackCommand) []byte {
	canonicalTuple := "voice-controlled-game-effect-v1\n" + command.InstallationID + "\n" + command.MessageID + "\n" + command.ActionID + "\n" + command.ActorProof.ProfileID
	digest := sha256.Sum256([]byte(canonicalTuple))
	return digest[:]
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
