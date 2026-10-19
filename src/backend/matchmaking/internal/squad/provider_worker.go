package squad

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/principal"
)

const (
	chatTeardownRPC  = "/voice.chat.v1.MatchSquadChatService/TeardownMatchSquadChat"
	voiceTeardownRPC = "/voice.calls.v1.MatchSquadVoiceService/TeardownMatchSquadRoom"
	chatCompactRPC   = "/voice.chat.v1.MatchSquadChatService/CompactMatchSquadChat"
	voiceCompactRPC  = "/voice.calls.v1.MatchSquadVoiceService/CompactMatchSquadRoom"
	chatCreateRPC    = "/voice.chat.v1.MatchSquadChatService/CreateMatchSquadChat"
	voiceCreateRPC   = "/voice.calls.v1.MatchSquadVoiceService/CreateMatchSquadRoom"
)

var errProviderContract = errors.New("protected provider contract mismatch")

// MatchSquadProviderWorker dispatches only persisted, request-bound commands to
// the dedicated protected provider clients. The clients must use the provider
// mTLS listeners; this worker never falls back to the public service clients.
type MatchSquadProviderWorker struct {
	Store          *store.MatchStore
	Chat           chatv1.MatchSquadChatServiceClient
	Voice          callsv1.MatchSquadVoiceServiceClient
	Issuer         *principal.Issuer
	PartyValidator partysource.ProvisionValidator
}

// Provision is the request-path entry to the durable provider saga. Every RPC
// follows a committed immutable request, and activation follows both receipts.
func (w *MatchSquadProviderWorker) Provision(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID) (string, string, error) {
	if w == nil || w.Store == nil || w.Issuer == nil || w.Chat == nil || w.Voice == nil || matchID == uuid.Nil {
		return "", "", errors.New("MatchSquad protected provider worker unavailable")
	}
	operation, err := w.Store.GetMatchSquadProvisioningOperation(ctx, matchID)
	if errors.Is(err, store.ErrMatchSquadPending) {
		candidate, proof, gateErr := w.partyGate().ValidateAttempt(ctx, matchID)
		if gateErr != nil {
			return "", "", gateErr
		}
		intent, buildErr := newProvisioningIntent(matchID, profileIDs)
		if buildErr != nil {
			return "", "", buildErr
		}
		operation, err = w.Store.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
		if errors.Is(err, store.ErrPlatformBanDenied) {
			return "", "", w.partyGate().ResetDeniedParties(ctx, candidate, err)
		}
		if errors.Is(err, store.ErrMatchSquadConflict) {
			// Another final accepter may have committed the same canonical match
			// manifest first. Reuse only that exact durable intent; never replace it.
			operation, err = w.Store.GetMatchSquadProvisioningOperation(ctx, matchID)
			if err == nil && (!bytes.Equal(operation.ParticipantManifestBytes, intent.ParticipantManifestBytes) ||
				!bytes.Equal(operation.ParticipantManifestHash, intent.ParticipantManifestHash)) {
				err = store.ErrMatchSquadConflict
			}
		}
	}
	if err != nil {
		return "", "", err
	}
	operation, err = w.provisionOperation(ctx, operation)
	if err != nil {
		return "", "", err
	}
	if operation.State != "active" {
		return "", "", store.ErrMatchSquadPending
	}
	if operation.ChatID == nil || operation.VoiceRoomID == nil {
		return "", "", fmt.Errorf("%w: active MatchSquad operation lacks resource IDs", errProviderContract)
	}
	return operation.VoiceRoomID.String(), operation.ChatID.String(), nil
}

func (w *MatchSquadProviderWorker) ProvisionAndActivate(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID) (store.Match, error) {
	if _, _, err := w.Provision(ctx, matchID, profileIDs); err != nil {
		return store.Match{}, err
	}
	match, err := w.Store.Get(ctx, matchID)
	if err != nil {
		return store.Match{}, err
	}
	if match.Status != store.MatchStatusActive || match.ChatID == nil || match.VoiceRoomID == nil {
		return store.Match{}, fmt.Errorf("%w: provisioning completed without active match resources", errProviderContract)
	}
	return match, nil
}

// RunProvisioningOnce resumes committed operations after an ambiguous response
// or process restart. Retries always use the exact request bytes already saved.
func (w *MatchSquadProviderWorker) RunProvisioningOnce(ctx context.Context, limit int) (int, error) {
	if w == nil || w.Store == nil || w.Issuer == nil || w.Chat == nil || w.Voice == nil {
		return 0, errors.New("MatchSquad protected provider worker unavailable")
	}
	items, err := w.Store.ListPendingMatchSquadProvisioningOperations(ctx, limit)
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures []error
	for _, item := range items {
		if _, err := w.provisionOperation(ctx, item); err != nil {
			failures = append(failures, fmt.Errorf("resume MatchSquad provisioning: %w", err))
			continue
		}
		completed++
	}
	// A process can die after the final proposal response commits and before
	// Provision() persists its immutable provider intent. Recover those matches
	// through the same idempotent entry point once the operation scan is done.
	missingIntent, err := w.Store.ListAcceptedMatchesMissingProvisioningIntent(ctx, limit)
	if err != nil {
		failures = append(failures, fmt.Errorf("list accepted matches without provisioning intent: %w", err))
	} else {
		for _, match := range missingIntent {
			if _, err := w.ProvisionAndActivate(ctx, match.ID, match.ProfileIDs()); err != nil {
				failures = append(failures, fmt.Errorf("recover accepted match %s provisioning: %w", match.ID, err))
				continue
			}
			completed++
		}
	}
	return completed, errors.Join(failures...)
}

func newProvisioningIntent(matchID uuid.UUID, profileIDs []uuid.UUID) (store.MatchSquadProvisioningIntent, error) {
	if matchID == uuid.Nil || len(profileIDs) == 0 {
		return store.MatchSquadProvisioningIntent{}, fmt.Errorf("%w: empty MatchSquad participant manifest", errProviderContract)
	}
	ids := append([]uuid.UUID(nil), profileIDs...)
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	manifest := make([]byte, 0, len(ids)*16)
	participants := make([]*chatv1.MatchSquadParticipant, 0, len(ids))
	for i, id := range ids {
		if id == uuid.Nil || (i > 0 && ids[i-1] == id) {
			return store.MatchSquadProvisioningIntent{}, fmt.Errorf("%w: invalid MatchSquad participant manifest", errProviderContract)
		}
		manifest = append(manifest, id[:]...)
		participants = append(participants, &chatv1.MatchSquadParticipant{ProfileId: id.String()})
	}
	chatOperationID, voiceOperationID := uuid.New(), uuid.New()
	request := &chatv1.CreateMatchSquadChatRequest{
		ProtocolVersion: 1, OperationId: chatOperationID.String(), MatchId: matchID.String(),
		Participants: participants, ParticipantManifestSha256: sha256Bytes(manifest),
	}
	requestBytes, err := deterministicBytes(request)
	if err != nil {
		return store.MatchSquadProvisioningIntent{}, err
	}
	return store.MatchSquadProvisioningIntent{
		MatchID: matchID, OperationID: uuid.New(), ParticipantManifestHash: sha256Bytes(manifest), ParticipantManifestBytes: manifest,
		ChatOperationID: chatOperationID, ChatRequestHash: sha256Bytes(requestBytes),
		ChatRequestBytes: requestBytes, VoiceOperationID: voiceOperationID,
	}, nil
}

func (w *MatchSquadProviderWorker) provisionOperation(ctx context.Context, operation store.MatchSquadProvisioningOperation) (store.MatchSquadProvisioningOperation, error) {
	// A scan item is discovery only; reload before deciding whether a forward
	// attempt or receipt-only settlement is still permitted.
	var err error
	operation, err = w.Store.GetMatchSquadProvisioningOperation(ctx, operation.MatchID)
	if err != nil {
		return operation, err
	}
	if operation.State == "active" {
		return operation, nil
	}
	if operation.State != "provisioning" && operation.State != "compensating" {
		return operation, store.ErrMatchSquadPending
	}
	dispatch, err := w.Store.GetMatchSquadDispatchState(ctx, operation.MatchID)
	if err != nil {
		return operation, err
	}
	if dispatch.MatchStatus == store.MatchStatusAbandoned && dispatch.Disposition == "LIVE" {
		if err := w.Store.AbandonMatch(ctx, operation.MatchID); err != nil {
			return operation, err
		}
		operation, err = w.Store.GetMatchSquadProvisioningOperation(ctx, operation.MatchID)
		if err != nil {
			return operation, err
		}
	}
	chatRequest := new(chatv1.CreateMatchSquadChatRequest)
	if err := unmarshalFrozenRequest(operation.ChatRequestBytes, operation.ChatRequestHash, chatRequest); err != nil ||
		chatRequest.GetOperationId() != operation.ChatOperationID.String() || chatRequest.GetMatchId() != operation.MatchID.String() ||
		!bytes.Equal(chatRequest.GetParticipantManifestSha256(), operation.ParticipantManifestHash) {
		return operation, fmt.Errorf("%w: stored Chat creation request is invalid", errProviderContract)
	}
	if operation.ChatReceiptID == nil {
		if err := w.authorizeForwardCreate(ctx, operation, "chat"); err != nil {
			return operation, err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		callCtx, err := w.callContext(attemptCtx, "chat", chatCreateRPC, operation.ChatOperationID, chatRequest)
		if err != nil {
			cancel()
			return operation, err
		}
		response, err := w.Chat.CreateMatchSquadChat(callCtx, chatRequest)
		cancel()
		if err != nil {
			return operation, err
		}
		if response == nil || response.GetReceipt() == nil {
			return operation, fmt.Errorf("%w: Chat create returned no receipt", errProviderContract)
		}
		receipt := response.GetReceipt()
		receiptBytes, err := deterministicBytes(receipt)
		if err != nil {
			return operation, err
		}
		receiptID, err := uuid.Parse(receipt.GetReceiptId())
		if err != nil {
			return operation, fmt.Errorf("%w: Chat receipt ID is invalid", errProviderContract)
		}
		chatID, err := uuid.Parse(receipt.GetChatId())
		if err != nil {
			return operation, fmt.Errorf("%w: Chat resource ID is invalid", errProviderContract)
		}
		voiceRequest, err := newVoiceCreateRequest(operation, receipt)
		if err != nil {
			return operation, err
		}
		voiceBytes, err := deterministicBytes(voiceRequest)
		if err != nil {
			return operation, err
		}
		operation, err = w.Store.RecordMatchSquadChatReceipt(ctx, operation.MatchID, receiptID, chatID, receiptBytes, sha256Bytes(voiceBytes), voiceBytes)
		if err != nil {
			return operation, err
		}
	}
	if operation.ChatReceiptID == nil || operation.ChatID == nil || len(operation.ChatReceiptBytes) == 0 || len(operation.VoiceRequestBytes) == 0 {
		return operation, fmt.Errorf("%w: Chat receipt and Voice request must be durable before Voice creation", errProviderContract)
	}
	voiceRequest := new(callsv1.CreateMatchSquadRoomRequest)
	if err := unmarshalFrozenRequest(operation.VoiceRequestBytes, operation.VoiceRequestHash, voiceRequest); err != nil ||
		voiceRequest.GetOperationId() != operation.VoiceOperationID.String() || voiceRequest.GetMatchId() != operation.MatchID.String() ||
		!bytes.Equal(voiceRequest.GetParticipantManifestSha256(), operation.ParticipantManifestHash) {
		return operation, fmt.Errorf("%w: stored Voice creation request is invalid", errProviderContract)
	}
	if operation.VoiceReceiptID == nil {
		if err := w.authorizeForwardCreate(ctx, operation, "voice"); err != nil {
			return operation, err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		callCtx, err := w.callContext(attemptCtx, "voice", voiceCreateRPC, operation.VoiceOperationID, voiceRequest)
		if err != nil {
			cancel()
			return operation, err
		}
		response, err := w.Voice.CreateMatchSquadRoom(callCtx, voiceRequest)
		cancel()
		if err != nil {
			// Once dispatch is possible, even a terminal status cannot prove
			// absence. Preserve the immutable issued marker for exact replay.
			return operation, err
		}
		if response == nil || response.GetReceipt() == nil {
			return operation, fmt.Errorf("%w: Voice create returned no receipt", errProviderContract)
		}
		receipt := response.GetReceipt()
		receiptBytes, err := deterministicBytes(receipt)
		if err != nil {
			return operation, err
		}
		receiptID, err := uuid.Parse(receipt.GetReceiptId())
		if err != nil {
			return operation, fmt.Errorf("%w: Voice receipt ID is invalid", errProviderContract)
		}
		roomID, err := uuid.Parse(receipt.GetRoomId())
		if err != nil {
			return operation, fmt.Errorf("%w: Voice resource ID is invalid", errProviderContract)
		}
		operation, err = w.Store.RecordMatchSquadVoiceReceipt(ctx, operation.MatchID, receiptID, roomID, receiptBytes)
		if err != nil {
			return operation, err
		}
	}
	if operation.VoiceReceiptID == nil || operation.VoiceRoomID == nil {
		return operation, fmt.Errorf("%w: Voice creation receipt was not durably recorded", errProviderContract)
	}
	dispatch, err = w.Store.GetMatchSquadDispatchState(ctx, operation.MatchID)
	if err != nil {
		return operation, err
	}
	if dispatch.Disposition == "ABANDONED" {
		// Persisted receipts materialize exact teardown independently. They do
		// not restore eligibility or make this operation publicly usable.
		return operation, store.ErrMatchSquadPending
	}
	candidate, proof, err := w.partyGate().ValidateAttempt(ctx, operation.MatchID)
	if err != nil {
		return operation, err
	}
	if _, err := w.Store.ActivateValidatedProvisionedMatch(ctx, candidate, proof, operation.OperationID); err != nil {
		if errors.Is(err, store.ErrPlatformBanDenied) {
			return operation, w.partyGate().ResetDeniedParties(ctx, candidate, err)
		}
		return operation, err
	}
	return w.Store.GetMatchSquadProvisioningOperation(ctx, operation.MatchID)
}

func (w *MatchSquadProviderWorker) partyGate() *PartySourceGate {
	return &PartySourceGate{Store: w.Store, Validator: w.PartyValidator}
}

func (w *MatchSquadProviderWorker) authorizeForwardCreate(ctx context.Context, operation store.MatchSquadProvisioningOperation, provider string) error {
	dispatch, err := w.Store.GetMatchSquadDispatchState(ctx, operation.MatchID)
	if err != nil {
		return err
	}
	if dispatch.Disposition == "ABANDONED" {
		marker := dispatch.Chat
		if provider == "voice" {
			marker = dispatch.Voice
		}
		if dispatch.OperationState != "compensating" || dispatch.MatchStatus != store.MatchStatusAbandoned || marker != "MAY_HAVE_CREATED" {
			return store.ErrMatchSquadPending
		}
		// Exact replay of a possible dispatch only obtains its original
		// receipt for cleanup. Never issue a previously NOT_ISSUED provider.
		return nil
	}
	candidate, proof, err := w.partyGate().ValidateAttempt(ctx, operation.MatchID)
	if err != nil {
		return err
	}
	_, err = w.Store.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, operation.OperationID, provider)
	if errors.Is(err, store.ErrPlatformBanDenied) {
		return w.partyGate().ResetDeniedParties(ctx, candidate, err)
	}
	return err
}

func isTerminalProviderCreateError(err error) bool {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}

func newVoiceCreateRequest(operation store.MatchSquadProvisioningOperation, chatReceipt *chatv1.MatchSquadChatReceipt) (*callsv1.CreateMatchSquadRoomRequest, error) {
	if chatReceipt == nil || len(operation.ParticipantManifestBytes) == 0 || len(operation.ParticipantManifestBytes)%16 != 0 {
		return nil, fmt.Errorf("%w: invalid Voice create dependencies", errProviderContract)
	}
	participants := make([]*chatv1.MatchSquadParticipant, 0, len(operation.ParticipantManifestBytes)/16)
	for offset := 0; offset < len(operation.ParticipantManifestBytes); offset += 16 {
		var id uuid.UUID
		copy(id[:], operation.ParticipantManifestBytes[offset:offset+16])
		if id == uuid.Nil {
			return nil, fmt.Errorf("%w: invalid participant manifest", errProviderContract)
		}
		participants = append(participants, &chatv1.MatchSquadParticipant{ProfileId: id.String()})
	}
	return &callsv1.CreateMatchSquadRoomRequest{ProtocolVersion: 1, OperationId: operation.VoiceOperationID.String(), MatchId: operation.MatchID.String(), Participants: participants,
		ParticipantManifestSha256: append([]byte(nil), operation.ParticipantManifestHash...), ChatCreationReceipt: proto.Clone(chatReceipt).(*chatv1.MatchSquadChatReceipt)}, nil
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
		claimed, ok, err := w.Store.ClaimMatchSquadTeardownParticipant(ctx, item, 30*time.Second)
		if err != nil {
			failures = append(failures, fmt.Errorf("claim %s MatchSquad teardown participant: %w", item.Provider, err))
			continue
		}
		if !ok {
			continue
		}
		receiptID, receiptBytes, err := w.teardown(ctx, claimed)
		if err != nil {
			if isTerminalProviderTeardownError(err) {
				if stateErr := w.Store.MarkMatchSquadTeardownContractMismatch(ctx, claimed, "contract_mismatch"); stateErr != nil {
					failures = append(failures, fmt.Errorf("record %s MatchSquad teardown contract mismatch: %w", item.Provider, stateErr))
				}
			} else if stateErr := w.Store.ScheduleMatchSquadTeardownRetry(ctx, claimed, matchSquadTeardownRetryDelay(claimed.AttemptCount, rand.Float64()), matchSquadTeardownErrorClass(err)); stateErr != nil {
				failures = append(failures, fmt.Errorf("schedule %s MatchSquad teardown retry: %w", item.Provider, stateErr))
			}
			failures = append(failures, fmt.Errorf("dispatch %s MatchSquad teardown: %w", item.Provider, err))
			continue
		}
		if _, err := w.Store.RecordMatchSquadTeardownReceipt(ctx, item.AggregateID, item.Provider, receiptID, receiptBytes); err != nil {
			if errors.Is(err, store.ErrMatchSquadConflict) {
				if stateErr := w.Store.MarkMatchSquadTeardownContractMismatch(ctx, claimed, "receipt_mismatch"); stateErr != nil {
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

func isTerminalProviderTeardownError(err error) bool {
	if errors.Is(err, errProviderContract) {
		return true
	}
	switch status.Code(err) {
	case codes.NotFound, codes.FailedPrecondition, codes.InvalidArgument:
		return true
	default:
		return false
	}
}

func matchSquadTeardownRetryDelay(attempt int, jitter float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if jitter < 0 {
		jitter = 0
	} else if jitter > 1 {
		jitter = 1
	}
	shift := attempt - 1
	if shift > 18 {
		shift = 18
	}
	base := time.Second * time.Duration(1<<shift)
	const capDelay = 5 * time.Minute
	if base > capDelay {
		base = capDelay
	}
	delay := base + time.Duration(float64(base/4)*jitter)
	if delay > capDelay {
		return capDelay
	}
	return delay
}

func matchSquadTeardownErrorClass(err error) string {
	if errors.Is(err, errProviderContract) {
		return "provider_contract"
	}
	switch status.Code(err) {
	case codes.Unavailable:
		return "unavailable"
	case codes.Unauthenticated:
		return "unauthenticated"
	case codes.PermissionDenied:
		return "permission_denied"
	case codes.DeadlineExceeded:
		return "deadline_exceeded"
	case codes.Internal:
		return "internal"
	default:
		return "transport_or_unknown"
	}
}

func matchSquadTeardownAttemptContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 10*time.Second)
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
		attemptCtx, cancel := matchSquadTeardownAttemptContext(ctx)
		defer cancel()
		callCtx, err := w.callContext(attemptCtx, "chat", chatTeardownRPC, item.OperationID, request)
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
		attemptCtx, cancel := matchSquadTeardownAttemptContext(ctx)
		defer cancel()
		callCtx, err := w.callContext(attemptCtx, "voice", voiceTeardownRPC, item.OperationID, request)
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

func sha256Bytes(value []byte) []byte {
	digest := sha256.Sum256(value)
	return append([]byte(nil), digest[:]...)
}
