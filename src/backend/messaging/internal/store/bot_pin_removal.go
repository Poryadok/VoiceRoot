package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	messagingv1 "voice.app/voice/messaging/v1"
)

var ErrBotPinRemovalBinding = errors.New("Bot pin-removal binding is invalid")
var ErrBotPinRemovalConflict = errors.New("Bot pin-removal operation conflicts with retained authority")
var ErrPinCleanupStale = errors.New("pin admission predates Bot cleanup")

type PinCleanupObservation struct {
	chatID, messageID, actorProfileID uuid.UUID
	revision                          int64
	valid                             bool
}
type pinCleanupObservationKey struct{}

func WithPinCleanupObservation(ctx context.Context, observation PinCleanupObservation) context.Context {
	return context.WithValue(ctx, pinCleanupObservationKey{}, observation)
}

func (s *PinsStore) ObservePinCleanupRevision(ctx context.Context, chatID, messageID uuid.UUID) (PinCleanupObservation, error) {
	if s == nil || s.Pool == nil {
		return PinCleanupObservation{}, ErrStoreNotConfigured
	}
	observation := PinCleanupObservation{chatID: chatID, messageID: messageID}
	err := s.Pool.QueryRow(ctx, `SELECT m.sender_profile_id,COALESCE(r.revision,0) FROM messages m LEFT JOIN bot_pin_removal_revisions r ON r.chat_id=m.chat_id AND r.actor_profile_id=m.sender_profile_id WHERE m.chat_id=$1 AND m.id=$2 AND m.deleted_at IS NULL`, chatID, messageID).Scan(&observation.actorProfileID, &observation.revision)
	if err != nil {
		return PinCleanupObservation{}, err
	}
	observation.valid = true
	return observation, nil
}

func requirePinCleanupObservation(ctx context.Context, tx pgx.Tx, chatID, messageID uuid.UUID) error {
	observation, ok := ctx.Value(pinCleanupObservationKey{}).(PinCleanupObservation)
	if !ok || !observation.valid || observation.chatID != chatID || observation.messageID != messageID || observation.actorProfileID == uuid.Nil || observation.revision < 0 {
		return ErrPinCleanupStale
	}
	var actor uuid.UUID
	var revision int64
	err := tx.QueryRow(ctx, `SELECT m.sender_profile_id,COALESCE(r.revision,0) FROM messages m LEFT JOIN bot_pin_removal_revisions r ON r.chat_id=m.chat_id AND r.actor_profile_id=m.sender_profile_id WHERE m.chat_id=$1 AND m.id=$2 AND m.deleted_at IS NULL`, chatID, messageID).Scan(&actor, &revision)
	if err != nil {
		return err
	}
	if actor != observation.actorProfileID || revision != observation.revision {
		return ErrPinCleanupStale
	}
	return nil
}

type botPinRequestBinding struct {
	body                                         []byte
	requestSHA, evidenceSHA                      [32]byte
	kind                                         string
	operation, decision, bot, actor, space, chat uuid.UUID
	attachments                                  []botPinAttachmentBinding
}

func botPinUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, ErrBotPinRemovalBinding
	}
	return id, nil
}

func bindBotPinRemovalRequest(request *messagingv1.ApplyBotChatPinRemovalRequest) (*botPinRequestBinding, error) {
	if request == nil || request.ProtocolVersion != 1 || len(request.ProtoReflect().GetUnknown()) != 0 || len(request.DecisionSha256) != 32 || len(request.FrozenScopeSha256) != 32 || (request.Purpose != "global_delete" && request.Purpose != "space_uninstall") {
		return nil, ErrBotPinRemovalBinding
	}
	b := &botPinRequestBinding{}
	for _, field := range []struct {
		value  string
		target *uuid.UUID
	}{{request.OperationId, &b.operation}, {request.DecisionId, &b.decision}, {request.BotId, &b.bot}, {request.ActorProfileId, &b.actor}, {request.SpaceId, &b.space}, {request.ChatId, &b.chat}} {
		id, err := botPinUUID(field.value)
		if err != nil {
			return nil, err
		}
		*field.target = id
	}
	var evidence proto.Message
	switch e := request.Evidence.(type) {
	case *messagingv1.ApplyBotChatPinRemovalRequest_Native:
		if e == nil || e.Native == nil || e.Native.Generation == 0 || e.Native.Generation > math.MaxInt64 {
			return nil, ErrBotPinRemovalBinding
		}
		if _, err := botPinUUID(e.Native.InstallationId); err != nil {
			return nil, err
		}
		if _, err := botPinUUID(e.Native.InstallOperationId); err != nil {
			return nil, err
		}
		evidence, b.kind = e.Native, "native"
	case *messagingv1.ApplyBotChatPinRemovalRequest_LegacyInstallation:
		if e == nil || e.LegacyInstallation == nil || len(e.LegacyInstallation.SnapshotSha256) != 32 {
			return nil, ErrBotPinRemovalBinding
		}
		if _, err := botPinUUID(e.LegacyInstallation.InstallationId); err != nil {
			return nil, err
		}
		if _, err := botPinUUID(e.LegacyInstallation.SnapshotId); err != nil {
			return nil, err
		}
		evidence, b.kind = e.LegacyInstallation, "legacy_installation"
	case *messagingv1.ApplyBotChatPinRemovalRequest_OrphanWhitelist:
		if e == nil || request.Purpose != "global_delete" || e.OrphanWhitelist == nil || len(e.OrphanWhitelist.SnapshotSha256) != 32 {
			return nil, ErrBotPinRemovalBinding
		}
		if _, err := botPinUUID(e.OrphanWhitelist.SnapshotId); err != nil {
			return nil, err
		}
		evidence, b.kind = e.OrphanWhitelist, "orphan_whitelist"
	default:
		return nil, ErrBotPinRemovalBinding
	}
	if len(evidence.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrBotPinRemovalBinding
	}
	evidenceBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(evidence)
	if err != nil {
		return nil, ErrBotPinRemovalBinding
	}
	b.evidenceSHA = sha256.Sum256(evidenceBytes)
	b.body, err = (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return nil, ErrBotPinRemovalBinding
	}
	b.requestSHA = sha256.Sum256(b.body)
	// Requests without lifetime identity are eligible only for exact historical
	// replay; they cannot create a new cleanup or bridge into another operation.
	if len(request.Attachments) > 0 {
		// Later source/coverage decisions must read the exact frozen body, not
		// pointers into the caller's protobuf after SQL or provider waits.
		owned := new(messagingv1.ApplyBotChatPinRemovalRequest)
		if proto.Unmarshal(b.body, owned) != nil {
			return nil, ErrBotPinRemovalBinding
		}
		b.attachments, err = bindBotPinAttachments(owned)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

type botPinReceiptQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readBotPinRemovalReceipt(ctx context.Context, q botPinReceiptQuerier, b *botPinRequestBinding) (*messagingv1.ApplyBotChatPinRemovalResponse, error) {
	var request, requestSHA, response, responseSHA []byte
	err := q.QueryRow(ctx, `SELECT request_bytes,request_sha256,response_bytes,response_sha256 FROM bot_pin_removal_receipts WHERE operation_id=$1`, b.operation).Scan(&request, &requestSHA, &response, &responseSHA)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(request, b.body) || !bytes.Equal(requestSHA, b.requestSHA[:]) {
		return nil, ErrBotPinRemovalConflict
	}
	digest := sha256.Sum256(response)
	if !bytes.Equal(digest[:], responseSHA) {
		return nil, ErrBotPinRemovalBinding
	}
	var decoded messagingv1.ApplyBotChatPinRemovalResponse
	if proto.Unmarshal(response, &decoded) != nil || decoded.Receipt == nil || len(decoded.ProtoReflect().GetUnknown()) != 0 || len(decoded.Receipt.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrBotPinRemovalBinding
	}
	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&decoded)
	if err != nil || !bytes.Equal(canonical, response) {
		return nil, ErrBotPinRemovalBinding
	}
	var original messagingv1.ApplyBotChatPinRemovalRequest
	if proto.Unmarshal(b.body, &original) != nil {
		return nil, ErrBotPinRemovalBinding
	}
	r := decoded.Receipt
	if _, err := botPinUUID(r.ReceiptId); err != nil {
		return nil, err
	}
	if r.ProtocolVersion != 1 || r.OperationId != original.OperationId || r.DecisionId != original.DecisionId || r.Purpose != original.Purpose || r.BotId != original.BotId || r.ActorProfileId != original.ActorProfileId || r.SpaceId != original.SpaceId || r.ChatId != original.ChatId || r.EvidenceKind != b.kind || !bytes.Equal(r.EvidenceSha256, b.evidenceSHA[:]) || !bytes.Equal(r.FrozenScopeSha256, original.FrozenScopeSha256) || !bytes.Equal(r.RequestSha256, b.requestSHA[:]) || r.Participant != "messaging" || r.Outcome != "COMPLETED" || r.RemovedPinCount > math.MaxInt64 || r.CompletedAt == nil || r.CompletedAt.CheckValid() != nil || r.CompletedAt.Seconds <= 0 || len(r.CompletedAt.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrBotPinRemovalBinding
	}
	if err = validateBotPinTargetReceipts(b, &decoded); err != nil {
		return nil, err
	}
	return &decoded, nil
}

// ApplyBotChatPinRemoval consumes the private receiver's authenticated Bot
// source assertion. It is not a public actor- or chat-selected mutation grant.
func (s *PinsStore) ApplyBotChatPinRemoval(ctx context.Context, request *messagingv1.ApplyBotChatPinRemovalRequest) (*messagingv1.ApplyBotChatPinRemovalResponse, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrStoreNotConfigured
	}
	b, err := bindBotPinRemovalRequest(request)
	if err != nil {
		return nil, err
	}
	// Use the exact authenticated canonical body after SQL waits, independently
	// of the caller's mutable protobuf object.
	request = new(messagingv1.ApplyBotChatPinRemovalRequest)
	if proto.Unmarshal(b.body, request) != nil {
		return nil, ErrBotPinRemovalBinding
	}
	prior, err := readBotPinRemovalReceipt(ctx, s.Pool, b)
	if err == nil {
		return prior, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if len(b.attachments) == 0 {
		return nil, ErrBotPinRemovalBinding
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Retry historical receipts after the CHAT wait, before any new SPACE
	// lifecycle decision. Exact replay never performs another cleanup.
	if err = lockMessageMutation(ctx, tx, b.chat, nil, nil); err != nil {
		return nil, err
	}
	prior, err = readBotPinRemovalReceipt(ctx, tx, b)
	if err == nil {
		return prior, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	targetReceipts := make([]*messagingv1.BotChatPinRemovalTargetReceipt, len(b.attachments))
	var uncovered []int
	for index, target := range b.attachments {
		covered, coverageErr := readBotPinAttachmentCoverage(ctx, tx, b, target)
		if coverageErr == nil {
			targetReceipts[index] = covered
			continue
		}
		if !missingBotPinCoverage(coverageErr) {
			return nil, coverageErr
		}
		if target.attachment.CoveringOperationId != "" {
			return nil, ErrBotPinRemovalBinding
		}
		uncovered = append(uncovered, index)
	}
	if len(uncovered) > 0 {
		if err = lockMessageMutation(ctx, tx, b.chat, &b.space, nil); err != nil {
			return nil, err
		}
		for _, index := range uncovered {
			if err = s.qualifyUncoveredBotPinAttachment(ctx, b, b.attachments[index]); err != nil {
				return nil, err
			}
		}
	}
	var scopeSpace *uuid.UUID
	if request.Purpose == "space_uninstall" {
		scopeSpace = &b.space
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_pin_removal_decisions(decision_id,decision_sha256,purpose,bot_id,actor_profile_id,scope_space_id) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(decision_id) DO NOTHING`, b.decision, request.DecisionSha256, request.Purpose, b.bot, b.actor, scopeSpace)
	if err != nil {
		return nil, err
	}
	var savedSHA []byte
	var purpose string
	var bot, actor uuid.UUID
	var savedSpace *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT decision_sha256,purpose,bot_id,actor_profile_id,scope_space_id FROM bot_pin_removal_decisions WHERE decision_id=$1 FOR UPDATE`, b.decision).Scan(&savedSHA, &purpose, &bot, &actor, &savedSpace)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(savedSHA, request.DecisionSha256) || purpose != request.Purpose || bot != b.bot || actor != b.actor || (scopeSpace == nil) != (savedSpace == nil) || (scopeSpace != nil && *scopeSpace != *savedSpace) {
		return nil, ErrBotPinRemovalConflict
	}
	// Both directions are immutable. The unique Bot key also serializes first
	// bindings arriving concurrently through different CHAT guards.
	_, err = tx.Exec(ctx, `INSERT INTO bot_pin_actor_bindings(actor_profile_id,bot_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, b.actor, b.bot)
	if err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, `SELECT bot_id FROM bot_pin_actor_bindings WHERE actor_profile_id=$1 FOR UPDATE`, b.actor).Scan(&bot)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBotPinRemovalConflict
	}
	if err != nil {
		return nil, err
	}
	if bot != b.bot {
		return nil, ErrBotPinRemovalConflict
	}
	var revision int64
	var removedCount int64
	if len(uncovered) > 0 {
		err = tx.QueryRow(ctx, `INSERT INTO bot_pin_removal_revisions(chat_id,actor_profile_id,revision) VALUES($1,$2,1) ON CONFLICT(chat_id,actor_profile_id) DO UPDATE SET revision=bot_pin_removal_revisions.revision+1 WHERE bot_pin_removal_revisions.revision<9223372036854775807 RETURNING revision`, b.chat, b.actor).Scan(&revision)
		if err != nil {
			return nil, err
		}
		removed, removeErr := tx.Exec(ctx, `DELETE FROM pins p USING messages m WHERE p.message_id=m.id AND p.chat_id=$1 AND m.chat_id=$1 AND m.sender_profile_id=$2`, b.chat, b.actor)
		if removeErr != nil {
			return nil, removeErr
		}
		removedCount = removed.RowsAffected()
		for _, index := range uncovered {
			target := b.attachments[index]
			targetReceipts[index] = &messagingv1.BotChatPinRemovalTargetReceipt{AttachmentId: target.id.String(), EffectTargetSha256: target.digest[:], CleanupRevision: uint64(revision)}
		}
	}
	receiptID, completed := uuid.New(), time.Now().UTC()
	response := &messagingv1.ApplyBotChatPinRemovalResponse{Receipt: &messagingv1.BotChatPinRemovalReceipt{ProtocolVersion: 1, ReceiptId: receiptID.String(), OperationId: request.OperationId, DecisionId: request.DecisionId, Purpose: request.Purpose, BotId: request.BotId, ActorProfileId: request.ActorProfileId, SpaceId: request.SpaceId, EvidenceKind: b.kind, EvidenceSha256: b.evidenceSHA[:], FrozenScopeSha256: request.FrozenScopeSha256, RequestSha256: b.requestSHA[:], Participant: "messaging", Outcome: "COMPLETED", CompletedAt: timestamppb.New(completed), ChatId: request.ChatId, RemovedPinCount: uint64(removedCount), Targets: targetReceipts, ExecutionCleanupRevision: uint64(revision)}}
	responseBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, err
	}
	responseSHA := sha256.Sum256(responseBytes)
	_, err = tx.Exec(ctx, `INSERT INTO bot_pin_removal_receipts(operation_id,decision_id,bot_id,actor_profile_id,space_id,chat_id,request_bytes,request_sha256,response_bytes,response_sha256,receipt_id,completed_at,cleanup_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, b.operation, b.decision, b.bot, b.actor, b.space, b.chat, b.body, b.requestSHA[:], responseBytes, responseSHA[:], receiptID, completed, revision)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			return nil, ErrBotPinRemovalConflict
		}
		return nil, err
	}
	for _, index := range uncovered {
		target := b.attachments[index]
		if _, err = tx.Exec(ctx, `INSERT INTO bot_pin_removal_attachment_coverage(attachment_id,projection_bytes,effect_target_sha256,origin_operation_id,cleanup_revision) VALUES($1,$2,$3,$4,$5)`, target.id, target.projection, target.digest[:], b.operation, revision); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}
