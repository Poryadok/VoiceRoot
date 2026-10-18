package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/chatcreation"
)

var (
	ErrCompositeUnknown       = errors.New("composite outcome unknown")
	ErrCompositeConflict      = errors.New("composite binding conflict")
	ErrCompositeResultExpired = errors.New("COMPOSITE_RESULT_EXPIRED")
	ErrCompositeCorrupt       = errors.New("invalid persisted composite evidence")
)

// GetChatCreationOutcome is a private evidence read: it deliberately bypasses
// ordinary LIVE/ACL guards so authenticated Chat recovery can settle old work.
// Public callers must perform current disclosure authorization separately.
func (s *SpaceStore) GetChatCreationOutcome(ctx context.Context, spaceID, operationID uuid.UUID) (*spacev1.SpaceChatCreationOutcome, error) {
	if s == nil || s.Pool == nil || spaceID == uuid.Nil || operationID == uuid.Nil {
		return nil, ErrCompositeCorrupt
	}
	return loadChatCreationOutcome(ctx, s.db(), spaceID, operationID, nil)
}

func loadChatCreationOutcome(ctx context.Context, db spaceStoreDB, spaceID, operationID uuid.UUID, binding *spacev1.SpaceChatCreationBinding) (*spacev1.SpaceChatCreationOutcome, error) {
	var raw []byte
	var storedSpace, chat, node, audit uuid.UUID
	var version, chatType, state, failure int32
	var expired bool
	err := db.QueryRow(ctx, `SELECT f.space_id,f.protocol_version,f.chat_id,f.chat_type,f.node_id,f.audit_event_id,f.terminal_state,f.failure_code,
 COALESCE(f.full_evidence_expires_at<=clock_timestamp(),false),e.outcome_bytes
 FROM space_chat_creation_fences f LEFT JOIN space_chat_creation_evidence e USING(operation_id)
 WHERE f.operation_id=$1`, operationID).Scan(&storedSpace, &version, &chat, &chatType, &node, &audit, &state, &failure, &expired, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCompositeUnknown
	}
	if err != nil {
		return nil, err
	}
	if storedSpace != spaceID {
		return nil, ErrCompositeConflict
	}
	if binding != nil && (version != int32(binding.GetProtocolVersion()) || chat.String() != binding.GetChatId() || chatType != int32(binding.GetChatType()) || node.String() != binding.GetNodeId() || audit.String() != binding.GetAuditEventId()) {
		return nil, ErrCompositeConflict
	}
	if expired {
		return nil, ErrCompositeResultExpired
	}
	if len(raw) == 0 {
		return nil, ErrCompositeCorrupt
	}
	saved := new(spacev1.SpaceChatCreationOutcome)
	if proto.Unmarshal(raw, saved) != nil || chatcreation.ValidateOutcome(saved.GetBinding(), saved) != nil {
		return nil, ErrCompositeCorrupt
	}
	b := saved.GetBinding()
	if b.GetOperationId() != operationID.String() || b.GetSpaceId() != spaceID.String() || int32(b.GetProtocolVersion()) != version || b.GetChatId() != chat.String() || int32(b.GetChatType()) != chatType || b.GetNodeId() != node.String() || b.GetAuditEventId() != audit.String() || int32(saved.GetState()) != state || int32(saved.GetFailureCode()) != failure {
		return nil, ErrCompositeCorrupt
	}
	if binding != nil && !proto.Equal(binding, b) {
		return nil, ErrCompositeConflict
	}
	return saved, nil
}

func lockChatCreation(ctx context.Context, tx pgx.Tx, operationID, spaceID uuid.UUID) error {
	material := append([]byte("voice.space.chat-creation.operation.v1\x00"), operationID[:]...)
	hash := sha256.Sum256(material)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(hash[:4])), int32(binary.BigEndian.Uint32(hash[4:8]))); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID))
	return err
}

// CommitChatCreation is called only after fresh external authority checks. No
// transaction or local Space mutex is held across those network calls. An
// unavailable authority must never be translated into a terminal refusal.
// The transaction rechecks local fences/membership/cap and serializes the
// original node and outcome. It never invokes ordinary tree audit machinery.
func (s *SpaceStore) CommitChatCreation(ctx context.Context, binding *spacev1.SpaceChatCreationBinding, authorization codes.Code) (*spacev1.SpaceChatCreationOutcome, error) {
	if chatcreation.ValidateBinding(binding) != nil {
		return nil, chatcreation.ErrInvalid
	}
	if authorization != codes.OK && !chatcreation.TerminalFailure(authorization) {
		return nil, chatcreation.ErrInvalid
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrCompositeCorrupt
	}
	// Freeze caller-owned protobuf memory before any blocking transaction work.
	binding = proto.Clone(binding).(*spacev1.SpaceChatCreationBinding)
	spaceID, operationID := uuid.MustParse(binding.SpaceId), uuid.MustParse(binding.OperationId)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockChatCreation(ctx, tx, operationID, spaceID); err != nil {
		return nil, err
	}
	saved, err := loadChatCreationOutcome(ctx, tx, spaceID, operationID, binding)
	if err == nil {
		return saved, nil
	}
	if !errors.Is(err, ErrCompositeUnknown) {
		return nil, err
	}
	// A purged Space remains unusable after its owner tombstone expires. A new
	// operation cannot recreate the Space or convert absence into a saved abort.
	var purged bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_permanent_purge_fences WHERE space_id=$1)`, spaceID).Scan(&purged); err != nil {
		return nil, err
	}
	if purged {
		return nil, ErrLifecycleFrozen
	}
	if err = checkOwnershipJournalAvailable(ctx, tx, []uuid.UUID{spaceID}); err != nil {
		return nil, err
	}
	if err = checkLifecycleAvailable(ctx, tx, []uuid.UUID{spaceID}); err != nil {
		if !errors.Is(err, ErrLifecycleFrozen) {
			return nil, err
		}
		authorization = codes.FailedPrecondition
	}
	var exists, member, guestAllowed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1),EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2),COALESCE((SELECT allow_guests FROM spaces WHERE id=$1),false)`, spaceID, uuid.MustParse(binding.CreatorProfileId)).Scan(&exists, &member, &guestAllowed); err != nil {
		return nil, err
	}
	if !exists {
		authorization = codes.NotFound
	} else if !member || binding.GetUser() != nil && binding.GetUser().GetAccountType() == "guest" && !guestAllowed {
		authorization = codes.PermissionDenied
	}
	if authorization == codes.OK {
		cap, capErr := s.treeNodeCapTx(ctx, tx, spaceID)
		if capErr != nil {
			return nil, capErr
		}
		count, countErr := s.countTreeNodes(ctx, tx, spaceID)
		if countErr != nil {
			return nil, countErr
		}
		if count >= cap {
			authorization = codes.ResourceExhausted
		}
	}
	outcome := &spacev1.SpaceChatCreationOutcome{Binding: binding, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED, FailureCode: uint32(authorization)}
	if authorization == codes.OK {
		order, orderErr := s.nextTreeSortOrder(ctx, tx, spaceID, nil)
		if orderErr != nil {
			return nil, orderErr
		}
		_, err = tx.Exec(ctx, `INSERT INTO space_tree_nodes(id,space_id,kind,chat_id,sort_order,is_system,is_pinned) VALUES($1,$2,'text_chat',$3,$4,false,false)`, uuid.MustParse(binding.NodeId), spaceID, uuid.MustParse(binding.ChatId), order)
		if err != nil {
			return nil, err
		}
		outcome.State = spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED
		outcome.Node = &spacev1.SpaceTreeNode{Id: binding.NodeId, SpaceId: binding.SpaceId, Kind: "text_chat", SortOrder: order, LinkedChat: &chatv1.ChatRef{Id: binding.ChatId, Type: &binding.ChatType}}
	}
	if chatcreation.ValidateOutcome(binding, outcome) != nil {
		return nil, ErrCompositeCorrupt
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(outcome)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_chat_creation_fences(operation_id,protocol_version,space_id,chat_id,chat_type,node_id,audit_event_id,terminal_state,failure_code) VALUES($1,1,$2,$3,$4,$5,$6,$7,$8)`, operationID, spaceID, uuid.MustParse(binding.ChatId), int32(binding.ChatType), uuid.MustParse(binding.NodeId), uuid.MustParse(binding.AuditEventId), int32(outcome.State), int32(outcome.FailureCode))
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO space_chat_creation_evidence(operation_id,outcome_bytes) VALUES($1,$2)`, operationID, raw); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return outcome, nil
}
