package squad

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/principal"
)

const (
	chatTeardownRPC  = "/voice.chat.v1.MatchSquadChatService/TeardownMatchSquadChat"
	voiceTeardownRPC = "/voice.calls.v1.MatchSquadVoiceService/TeardownMatchSquadRoom"
	chatCompactRPC   = "/voice.chat.v1.MatchSquadChatService/CompactMatchSquadChat"
	voiceCompactRPC  = "/voice.calls.v1.MatchSquadVoiceService/CompactMatchSquadRoom"
)

var errProviderContract = errors.New("protected provider contract mismatch")

// MatchSquadProviderWorker dispatches only persisted, request-bound commands to
// the dedicated protected provider clients. The clients must use the provider
// mTLS listeners; this worker never falls back to the public service clients.
type MatchSquadProviderWorker struct {
	Store  *store.MatchStore
	Chat   chatv1.MatchSquadChatServiceClient
	Voice  callsv1.MatchSquadVoiceServiceClient
	Issuer *principal.Issuer
}

func (w *MatchSquadProviderWorker) RunTeardownOnce(ctx context.Context, limit int) (int, error) {
	if w == nil || w.Store == nil || w.Issuer == nil || w.Chat == nil || w.Voice == nil {
		return 0, errors.New("MatchSquad protected provider worker unavailable")
	}
	items, err := w.Store.ListPendingMatchSquadTeardownParticipants(ctx, limit)
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures []error
	for _, item := range items {
		if err := w.Store.SetMatchSquadTeardownParticipantState(ctx, item.AggregateID, item.Provider, "IN_FLIGHT"); err != nil {
			failures = append(failures, fmt.Errorf("claim %s MatchSquad teardown participant: %w", item.Provider, err))
			continue
		}
		receiptID, receiptBytes, err := w.teardown(ctx, item)
		if err != nil {
			nextState := "RETRYABLE_FAILURE"
			if errors.Is(err, errProviderContract) {
				nextState = "CONTRACT_MISMATCH"
			}
			if stateErr := w.Store.SetMatchSquadTeardownParticipantState(ctx, item.AggregateID, item.Provider, nextState); stateErr != nil {
				failures = append(failures, fmt.Errorf("record %s MatchSquad teardown failure: %w", item.Provider, stateErr))
			}
			failures = append(failures, fmt.Errorf("dispatch %s MatchSquad teardown: %w", item.Provider, err))
			continue
		}
		if _, err := w.Store.RecordMatchSquadTeardownReceipt(ctx, item.AggregateID, item.Provider, receiptID, receiptBytes); err != nil {
			if errors.Is(err, store.ErrMatchSquadConflict) {
				if stateErr := w.Store.SetMatchSquadTeardownParticipantState(ctx, item.AggregateID, item.Provider, "CONTRACT_MISMATCH"); stateErr != nil {
					failures = append(failures, fmt.Errorf("record %s MatchSquad receipt mismatch: %w", item.Provider, stateErr))
				}
				failures = append(failures, fmt.Errorf("validate %s MatchSquad teardown receipt: %w", item.Provider, err))
				continue
			}
			failures = append(failures, fmt.Errorf("record %s MatchSquad teardown receipt: %w", item.Provider, err))
			continue
		}
		completed++
	}
	return completed, errors.Join(failures...)
}

func (w *MatchSquadProviderWorker) teardown(ctx context.Context, item store.MatchSquadTeardownParticipant) (uuid.UUID, []byte, error) {
	switch item.Provider {
	case "chat":
		request := new(chatv1.TeardownMatchSquadChatRequest)
		if err := unmarshalFrozenRequest(item.Request, item.RequestHash, request); err != nil {
			return uuid.Nil, nil, fmt.Errorf("%w: invalid stored Chat teardown request: %v", errProviderContract, err)
		}
		if request.GetTeardownOperationId() != item.OperationID.String() || request.GetMatchId() == "" {
			return uuid.Nil, nil, fmt.Errorf("%w: stored Chat teardown binding is invalid", errProviderContract)
		}
		callCtx, err := w.callContext(ctx, "chat", chatTeardownRPC, item.OperationID, request)
		if err != nil {
			return uuid.Nil, nil, err
		}
		response, err := w.Chat.TeardownMatchSquadChat(callCtx, request)
		if err != nil {
			return uuid.Nil, nil, err
		}
		if response == nil || response.GetReceipt() == nil {
			return uuid.Nil, nil, fmt.Errorf("%w: Chat teardown returned no receipt", errProviderContract)
		}
		raw, err := deterministicBytes(response.GetReceipt())
		if err != nil {
			return uuid.Nil, nil, err
		}
		receiptID, err := uuid.Parse(response.GetReceipt().GetReceiptId())
		if err != nil {
			return uuid.Nil, nil, fmt.Errorf("%w: Chat teardown receipt ID is invalid", errProviderContract)
		}
		return receiptID, raw, nil
	case "voice":
		request := new(callsv1.TeardownMatchSquadRoomRequest)
		if err := unmarshalFrozenRequest(item.Request, item.RequestHash, request); err != nil {
			return uuid.Nil, nil, fmt.Errorf("%w: invalid stored Voice teardown request: %v", errProviderContract, err)
		}
		if request.GetTeardownOperationId() != item.OperationID.String() || request.GetMatchId() == "" {
			return uuid.Nil, nil, fmt.Errorf("%w: stored Voice teardown binding is invalid", errProviderContract)
		}
		callCtx, err := w.callContext(ctx, "voice", voiceTeardownRPC, item.OperationID, request)
		if err != nil {
			return uuid.Nil, nil, err
		}
		response, err := w.Voice.TeardownMatchSquadRoom(callCtx, request)
		if err != nil {
			return uuid.Nil, nil, err
		}
		if response == nil || response.GetReceipt() == nil {
			return uuid.Nil, nil, fmt.Errorf("%w: Voice teardown returned no receipt", errProviderContract)
		}
		raw, err := deterministicBytes(response.GetReceipt())
		if err != nil {
			return uuid.Nil, nil, err
		}
		receiptID, err := uuid.Parse(response.GetReceipt().GetReceiptId())
		if err != nil {
			return uuid.Nil, nil, fmt.Errorf("%w: Voice teardown receipt ID is invalid", errProviderContract)
		}
		return receiptID, raw, nil
	default:
		return uuid.Nil, nil, errors.New("unknown MatchSquad provider")
	}
}

func (w *MatchSquadProviderWorker) RunCompactionOnce(ctx context.Context, limit int) (int, error) {
	if w == nil || w.Store == nil || w.Issuer == nil || w.Chat == nil || w.Voice == nil {
		return 0, errors.New("MatchSquad protected provider worker unavailable")
	}
	due, err := w.Store.ListDueMatchSquadCompactions(ctx, limit)
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures []error
	for _, pending := range due {
		intent, err := w.Store.PrepareMatchSquadCompaction(ctx, pending.AggregateID, pending.Provider)
		if err != nil {
			failures = append(failures, fmt.Errorf("prepare %s MatchSquad compaction: %w", pending.Provider, err))
			continue
		}
		var receiptID uuid.UUID
		var receiptBytes []byte
		switch intent.Provider {
		case "chat":
			request := new(chatv1.CompactMatchSquadChatRequest)
			if err := unmarshalFrozenRequest(intent.RequestBytes, intent.RequestHash, request); err != nil {
				failures = append(failures, fmt.Errorf("%w: invalid stored Chat compaction request: %v", errProviderContract, err))
				continue
			}
			if request.GetOperationId() != intent.OperationID.String() || request.GetTeardownAggregateId() != intent.AggregateID.String() {
				failures = append(failures, fmt.Errorf("%w: stored Chat compaction binding is invalid", errProviderContract))
				continue
			}
			callCtx, err := w.callContext(ctx, "chat", chatCompactRPC, intent.OperationID, request)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			response, err := w.Chat.CompactMatchSquadChat(callCtx, request)
			if err != nil {
				failures = append(failures, fmt.Errorf("dispatch Chat MatchSquad compaction: %w", err))
				continue
			}
			if response == nil || response.GetReceipt() == nil {
				failures = append(failures, fmt.Errorf("%w: Chat compaction returned no receipt", errProviderContract))
				continue
			}
			receiptBytes, err = deterministicBytes(response.GetReceipt())
			if err == nil {
				receiptID, err = uuid.Parse(response.GetReceipt().GetReceiptId())
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("%w: invalid Chat compaction receipt ID", errProviderContract))
				continue
			}
		case "voice":
			request := new(callsv1.CompactMatchSquadRoomRequest)
			if err := unmarshalFrozenRequest(intent.RequestBytes, intent.RequestHash, request); err != nil {
				failures = append(failures, fmt.Errorf("%w: invalid stored Voice compaction request: %v", errProviderContract, err))
				continue
			}
			if request.GetOperationId() != intent.OperationID.String() || request.GetTeardownAggregateId() != intent.AggregateID.String() {
				failures = append(failures, fmt.Errorf("%w: stored Voice compaction binding is invalid", errProviderContract))
				continue
			}
			callCtx, err := w.callContext(ctx, "voice", voiceCompactRPC, intent.OperationID, request)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			response, err := w.Voice.CompactMatchSquadRoom(callCtx, request)
			if err != nil {
				failures = append(failures, fmt.Errorf("dispatch Voice MatchSquad compaction: %w", err))
				continue
			}
			if response == nil || response.GetReceipt() == nil {
				failures = append(failures, fmt.Errorf("%w: Voice compaction returned no receipt", errProviderContract))
				continue
			}
			receiptBytes, err = deterministicBytes(response.GetReceipt())
			if err == nil {
				receiptID, err = uuid.Parse(response.GetReceipt().GetReceiptId())
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("%w: invalid Voice compaction receipt ID", errProviderContract))
				continue
			}
		default:
			failures = append(failures, fmt.Errorf("unknown MatchSquad compaction provider %q", intent.Provider))
			continue
		}
		if _, err := w.Store.RecordMatchSquadCompactionReceipt(ctx, intent.AggregateID, intent.Provider, receiptID, receiptBytes); err != nil {
			failures = append(failures, fmt.Errorf("record %s MatchSquad compaction receipt: %w", intent.Provider, err))
			continue
		}
		completed++
	}
	return completed, errors.Join(failures...)
}

func (w *MatchSquadProviderWorker) callContext(ctx context.Context, audience, rpc string, operationID uuid.UUID, request proto.Message) (context.Context, error) {
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, fmt.Errorf("hash protected provider request: %w", err)
	}
	token, err := w.Issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: operationID.String(), RequestHash: hash})
	if err != nil {
		return nil, fmt.Errorf("issue protected provider principal: %w", err)
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operationID.String())), nil
}

func unmarshalFrozenRequest(raw, expectedHash []byte, message proto.Message) error {
	if len(raw) == 0 || len(expectedHash) != 32 {
		return errors.New("stored provider request is incomplete")
	}
	if err := proto.Unmarshal(raw, message); err != nil {
		return fmt.Errorf("decode stored provider request: %w", err)
	}
	if len(message.ProtoReflect().GetUnknown()) != 0 {
		return errors.New("stored provider request has unknown fields")
	}
	encoded, err := deterministicBytes(message)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if !bytes.Equal(encoded, raw) || !bytes.Equal(digest[:], expectedHash) {
		return errors.New("stored provider request does not match its immutable digest")
	}
	return nil
}

func deterministicBytes(message proto.Message) ([]byte, error) {
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("encode protected provider message: %w", err)
	}
	return encoded, nil
}
