package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	eventsv1 "voice.app/voice/events/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spacemutationlock"
)

var (
	ErrSpaceCompositeEvidence = errors.New("Space composite binding or outcome conflicts")
	ErrSpaceCompositeExpired  = errors.New("COMPOSITE_RESULT_EXPIRED")
	ErrSpaceCompositeDrain    = errors.New("Space composite drain is incomplete")
	ErrSpaceCompositeLease    = errors.New("Space composite worker lease lost")
)

func checkSpaceCompositePurgeReady(ctx context.Context, tx pgx.Tx, space uuid.UUID) error {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_space_composite_operations WHERE space_id=$1 AND state='PREPARED')
		OR EXISTS(SELECT 1 FROM chat_space_composite_outbox o JOIN chat_space_composite_operations p USING(operation_id)
		WHERE p.space_id=$1 AND (o.eligibility='HELD' OR (o.eligibility='READY' AND o.delivered_at IS NULL)))`, space).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return ErrSpaceCompositeDrain
	}
	return nil
}

type SpaceCompositeOperation struct {
	Binding     *spacev1.SpaceChatCreationBinding
	Response    *chatv1.CreateChatResponse
	State       string
	FailureCode codes.Code
	LegacyChat  *ChatRow
}

type SpaceCompositeClaim struct {
	Operation *SpaceCompositeOperation
	Token     uuid.UUID
}

type SpaceCompositeEffect struct {
	OperationID uuid.UUID
	EventID     uuid.UUID
	Kind        string
	Payload     []byte
	Token       uuid.UUID
}

func (s *DMStore) ClaimSpaceCompositeEffect(ctx context.Context, lease time.Duration) (*SpaceCompositeEffect, error) {
	if lease <= 0 || lease > 5*time.Minute {
		return nil, ErrSpaceCompositeLease
	}
	tx, err := beginSpaceComposite(ctx, s)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	e := &SpaceCompositeEffect{Token: uuid.New()}
	err = tx.QueryRow(ctx, `SELECT o.operation_id,o.event_id,o.effect_kind,o.payload FROM chat_space_composite_outbox o
		JOIN chat_space_composite_operations p USING(operation_id)
		WHERE p.state='COMPLETE' AND o.eligibility='READY' AND o.delivered_at IS NULL
		AND o.next_attempt_at<=clock_timestamp() AND (o.lease_token IS NULL OR o.lease_expires_at<=clock_timestamp())
		ORDER BY o.next_attempt_at,o.event_id FOR UPDATE OF o SKIP LOCKED LIMIT 1`).Scan(&e.OperationID, &e.EventID, &e.Kind, &e.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(e.Payload) == 0 {
		return nil, ErrSpaceCompositeEvidence
	}
	_, err = tx.Exec(ctx, `UPDATE chat_space_composite_outbox SET lease_token=$2,lease_expires_at=clock_timestamp()+$3*interval '1 millisecond',
		attempt_count=attempt_count+1 WHERE event_id=$1`, e.EventID, e.Token, lease.Milliseconds())
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *DMStore) AckSpaceCompositeEffect(ctx context.Context, e *SpaceCompositeEffect) error {
	if e == nil || e.Token == uuid.Nil || e.EventID == uuid.Nil {
		return ErrSpaceCompositeLease
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE chat_space_composite_outbox SET delivered_at=clock_timestamp(),lease_token=NULL,lease_expires_at=NULL,last_error_class=NULL
		WHERE operation_id=$1 AND event_id=$2 AND effect_kind=$3 AND eligibility='READY' AND delivered_at IS NULL
		AND lease_token=$4 AND lease_expires_at>clock_timestamp()`, e.OperationID, e.EventID, e.Kind, e.Token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrSpaceCompositeLease
	}
	return nil
}

func (s *DMStore) RetrySpaceCompositeEffect(ctx context.Context, e *SpaceCompositeEffect, delay time.Duration) error {
	if e == nil || e.Token == uuid.Nil || delay < 0 || delay > time.Hour {
		return ErrSpaceCompositeLease
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE chat_space_composite_outbox SET lease_token=NULL,lease_expires_at=NULL,
		next_attempt_at=clock_timestamp()+$5*interval '1 millisecond',last_error_class='DELIVERY_UNACKNOWLEDGED'
		WHERE operation_id=$1 AND event_id=$2 AND effect_kind=$3 AND eligibility='READY' AND delivered_at IS NULL
		AND lease_token=$4 AND lease_expires_at>clock_timestamp()`, e.OperationID, e.EventID, e.Kind, e.Token, delay.Milliseconds())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrSpaceCompositeLease
	}
	return nil
}

// MinimizeSpaceCompositeEvidence retains only structural operation/resource
// fences after this participant's own successful P3 completion plus 30 days.
// Neither unresolved operations nor unacknowledged delivery can be pruned.
func (s *DMStore) MinimizeSpaceCompositeEvidence(ctx context.Context, limit int) (int64, error) {
	if limit < 1 || limit > 1000 {
		return 0, ErrSpaceCompositeEvidence
	}
	rows, err := s.Pool.Query(ctx, `SELECT p.operation_id FROM chat_space_composite_operations p
		WHERE p.state<>'PREPARED' AND p.full_evidence_expires_at<=clock_timestamp() AND p.binding_bytes IS NOT NULL
		AND NOT EXISTS(SELECT 1 FROM chat_space_composite_outbox o WHERE o.operation_id=p.operation_id
		AND (o.eligibility='HELD' OR (o.eligibility='READY' AND o.delivered_at IS NULL)))
		ORDER BY p.operation_id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	var count int64
	for _, id := range ids {
		changed, err := s.minimizeSpaceComposite(ctx, id)
		if err != nil {
			return count, err
		}
		if changed {
			count++
		}
	}
	return count, nil
}

func (s *DMStore) minimizeSpaceComposite(ctx context.Context, id uuid.UUID) (bool, error) {
	// One Space per transaction, in the same Space -> keyed reservation ->
	// operation order as finalization. Never acquire a key after an op lock.
	tx, err := beginSpaceComposite(ctx, s)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var space uuid.UUID
	var actor, key *uuid.UUID
	if err = tx.QueryRow(ctx, "SELECT space_id,creator_profile_id,client_request_id FROM chat_space_composite_operations WHERE operation_id=$1", id).Scan(&space, &actor, &key); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::bigint)", spacemutationlock.Key(space)); err != nil {
		return false, err
	}
	if key != nil {
		if _, err = tx.Exec(ctx, "SELECT 1 FROM chat_create_requests WHERE creator_profile_id=$1 AND request_id=$2 FOR UPDATE", actor, key); err != nil {
			return false, err
		}
	}
	var eligible bool
	if err = tx.QueryRow(ctx, `SELECT state<>'PREPARED' AND full_evidence_expires_at<=clock_timestamp() AND binding_bytes IS NOT NULL
		AND NOT EXISTS(SELECT 1 FROM chat_space_composite_outbox WHERE operation_id=$1
		AND (eligibility='HELD' OR (eligibility='READY' AND delivered_at IS NULL)))
		FROM chat_space_composite_operations WHERE operation_id=$1 FOR UPDATE`, id).Scan(&eligible); err != nil {
		return false, err
	}
	if !eligible {
		return false, tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM chat_create_requests WHERE operation_id=$1 AND execution_kind='space_composite_v1'", id); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, "UPDATE chat_space_composite_outbox SET payload=NULL,last_error_class=NULL,lease_token=NULL,lease_expires_at=NULL WHERE operation_id=$1", id); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE chat_space_composite_operations SET creator_profile_id=NULL,client_request_id=NULL,create_request_sha256=NULL,
			binding_bytes=NULL,response_bytes=NULL,outcome_bytes=NULL,lease_token=NULL,lease_expires_at=NULL,last_error_class=NULL WHERE operation_id=$1`, id); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// Claim only coordinates autonomous workers. An expired lease never changes
// domain state and never authorizes cancellation of a prepared operation.
func (s *DMStore) ClaimSpaceComposite(ctx context.Context, lease time.Duration) (*SpaceCompositeClaim, error) {
	if lease <= 0 || lease > 5*time.Minute {
		return nil, ErrSpaceCompositeLease
	}
	tx, err := beginSpaceComposite(ctx, s)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id uuid.UUID
	err = tx.QueryRow(ctx, `SELECT operation_id FROM chat_space_composite_operations
		WHERE state='PREPARED' AND next_attempt_at<=clock_timestamp()
		AND (lease_token IS NULL OR lease_expires_at<=clock_timestamp())
		ORDER BY next_attempt_at,operation_id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	op, err := loadSpaceComposite(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	token := uuid.New()
	_, err = tx.Exec(ctx, `UPDATE chat_space_composite_operations SET lease_token=$2,
		lease_expires_at=clock_timestamp()+$3*interval '1 millisecond',attempt_count=attempt_count+1 WHERE operation_id=$1`, id, token, lease.Milliseconds())
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &SpaceCompositeClaim{Operation: op, Token: token}, nil
}

func (s *DMStore) RetrySpaceComposite(ctx context.Context, claim *SpaceCompositeClaim, delay time.Duration) error {
	if claim == nil || claim.Operation == nil || claim.Operation.Binding == nil || claim.Token == uuid.Nil || delay < 0 || delay > time.Hour {
		return ErrSpaceCompositeLease
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE chat_space_composite_operations SET lease_token=NULL,lease_expires_at=NULL,
		next_attempt_at=clock_timestamp()+$3*interval '1 millisecond',last_error_class='OUTCOME_UNRESOLVED'
		WHERE operation_id=$1 AND state='PREPARED' AND lease_token=$2 AND lease_expires_at>clock_timestamp()`, claim.Operation.Binding.OperationId, claim.Token, delay.Milliseconds())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrSpaceCompositeLease
	}
	return nil
}

func (s *DMStore) ResolveClaimedSpaceComposite(ctx context.Context, claim *SpaceCompositeClaim, outcome *spacev1.SpaceChatCreationOutcome) (*SpaceCompositeOperation, error) {
	if claim == nil || claim.Operation == nil || !chatcreation.ValidBinding(claim.Operation.Binding) || claim.Token == uuid.Nil {
		return nil, ErrSpaceCompositeLease
	}
	return s.resolveSpaceComposite(ctx, uuid.MustParse(claim.Operation.Binding.OperationId), outcome, &claim.Token)
}

func compositeBytes(m proto.Message) ([]byte, error) {
	return (proto.MarshalOptions{Deterministic: true}).Marshal(m)
}

// Every new composite writer takes the table compatibility locks before its
// Space advisory lock. A guarded DOWN cannot overtake a first writer.
func beginSpaceComposite(ctx context.Context, s *DMStore) (pgx.Tx, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrSpaceLifecycleState
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE chat_space_composite_operations,chat_space_composite_outbox IN ROW EXCLUSIVE MODE"); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func compositeNormalization(req *chatv1.CreateChatRequest) (space uuid.UUID, kind, name string, topic *string, legacyHash string, digest [32]byte, err error) {
	space, err = chatcreation.CanonicalID(req.GetSpaceId())
	if err != nil {
		return
	}
	switch req.GetType() {
	case chatv1.ChatType_CHAT_TYPE_GROUP:
		kind = "group"
	case chatv1.ChatType_CHAT_TYPE_CHANNEL:
		kind = "channel"
	default:
		err = ErrSpaceCompositeEvidence
		return
	}
	name = strings.TrimSpace(req.GetName())
	if name == "" {
		err = ErrSpaceCompositeEvidence
		return
	}
	if req.Topic != nil && strings.TrimSpace(*req.Topic) != "" {
		t := strings.TrimSpace(*req.Topic)
		topic = &t
	}
	canonical := struct {
		Type    string  `json:"type"`
		SpaceID string  `json:"space_id"`
		Name    string  `json:"name"`
		Topic   *string `json:"topic"`
	}{kind, space.String(), name, topic}
	body, marshalErr := json.Marshal(canonical)
	if marshalErr != nil {
		err = marshalErr
		return
	}
	oldDigest := sha256.Sum256(body)
	legacyHash = "sha256:" + hex.EncodeToString(oldDigest[:])
	digest = sha256.Sum256(append([]byte("voice.chat.space_composite.v1\x00"), body...))
	return
}

func (s *DMStore) PrepareSpaceComposite(ctx context.Context, req *chatv1.CreateChatRequest) (*SpaceCompositeOperation, error) {
	if req == nil {
		return nil, chatcreation.ErrInitiator
	}
	req = proto.Clone(req).(*chatv1.CreateChatRequest)
	initiator, err := chatcreation.Creator(ctx, req)
	if err != nil {
		return nil, err
	}
	space, kind, name, topic, legacyHash, digest, err := compositeNormalization(req)
	if err != nil {
		return nil, err
	}
	var key *uuid.UUID
	if req.GetRequestId() != "" {
		id, err := chatcreation.CanonicalID(req.GetRequestId())
		if err != nil {
			return nil, err
		}
		key = &id
	}
	tx, err := beginSpaceComposite(ctx, s)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockSpaceLifecycleMutation(ctx, tx, space); err != nil {
		return nil, err
	}
	// A caller may have waited behind freeze or another mutation longer than
	// its short credential lifetime. Recheck before any reservation/effect.
	if _, err = chatcreation.Creator(ctx, req); err != nil {
		return nil, err
	}
	operation := uuid.New()
	if key != nil {
		var inserted uuid.UUID
		err = tx.QueryRow(ctx, `INSERT INTO chat_create_requests(creator_profile_id,request_id,request_hash,execution_kind,operation_id)
			VALUES($1,$2,$3,'space_composite_v1',$4)
			ON CONFLICT(creator_profile_id,request_id) DO NOTHING RETURNING operation_id`,
			initiator.ProfileID, *key, legacyHash, operation).Scan(&inserted)
		if errors.Is(err, pgx.ErrNoRows) {
			var priorHash, execution string
			var savedOperation *uuid.UUID
			var savedChat *uuid.UUID
			var result []byte
			err = tx.QueryRow(ctx, `SELECT request_hash,execution_kind,operation_id,chat_id,result_bytes FROM chat_create_requests
				WHERE creator_profile_id=$1 AND request_id=$2 FOR UPDATE`, initiator.ProfileID, *key).Scan(&priorHash, &execution, &savedOperation, &savedChat, &result)
			if err != nil {
				return nil, err
			}
			if priorHash != legacyHash {
				return nil, ErrChatCreateRequestConflict
			}
			if execution == "legacy" {
				var row ChatRow
				if savedChat == nil || json.Unmarshal(result, &row) != nil || row.ID != *savedChat || row.CreatorProfileID != initiator.ProfileID {
					return nil, ErrSpaceCompositeEvidence
				}
				if err = tx.Commit(ctx); err != nil {
					return nil, err
				}
				return &SpaceCompositeOperation{State: "COMPLETE", LegacyChat: &row}, nil
			}
			if execution != "space_composite_v1" || savedOperation == nil {
				return nil, ErrSpaceCompositeEvidence
			}
			saved, err := loadSpaceComposite(ctx, tx, *savedOperation, false)
			if err != nil {
				return nil, err
			}
			if saved.Binding.SpaceId != space.String() || !bytes.Equal(saved.Binding.CreateRequestSha256, digest[:]) {
				return nil, ErrChatCreateRequestConflict
			}
			// The original initiating context is immutable. Session refresh does
			// not rewrite the stored fact, but another actor cannot adopt it.
			if saved.Binding.CreatorProfileId != initiator.ProfileID.String() {
				return nil, ErrChatCreateRequestConflict
			}
			if initiator.User != nil {
				if saved.Binding.GetUser() == nil || saved.Binding.GetUser().AccountId != initiator.User.AccountId {
					return nil, ErrChatCreateRequestConflict
				}
			} else if !proto.Equal(saved.Binding.GetBot(), initiator.Bot) {
				return nil, ErrChatCreateRequestConflict
			}
			if err = tx.Commit(ctx); err != nil {
				return nil, err
			}
			return saved, nil
		}
		if err != nil {
			return nil, err
		}
	}
	// A conflicting keyed INSERT can wait independently of the Space lock.
	// Do not retain a reservation after that wait outlives its initiating JWT.
	if _, err = chatcreation.Creator(ctx, req); err != nil {
		return nil, err
	}
	chatID, nodeID, auditID := uuid.New(), uuid.New(), uuid.New()
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return nil, err
	}
	chat := &chatv1.Chat{Id: chatID.String(), Type: req.GetType(), SpaceId: proto.String(space.String()), Name: proto.String(name), Topic: topic,
		CreatorProfileId: initiator.ProfileID.String(), CreatedAt: timestamppb.New(now), UpdatedAt: timestamppb.New(now), ThreadsEnabled: kind == "channel", AllowUserMainFeed: kind == "group"}
	binding := &spacev1.SpaceChatCreationBinding{ProtocolVersion: 1, OperationId: operation.String(), CreatorProfileId: initiator.ProfileID.String(),
		SpaceId: space.String(), ChatId: chatID.String(), ChatType: req.GetType(), NodeId: nodeID.String(), AuditEventId: auditID.String(), CreateRequestSha256: digest[:]}
	if initiator.User != nil {
		binding.Initiator = &spacev1.SpaceChatCreationBinding_User{User: initiator.User}
	} else {
		binding.Initiator = &spacev1.SpaceChatCreationBinding_Bot{Bot: initiator.Bot}
	}
	response := &chatv1.CreateChatResponse{Chat: chat, CompositeOperationId: operation.String(), CompositeNodeId: nodeID.String()}
	bindingBytes, err := compositeBytes(binding)
	if err != nil {
		return nil, err
	}
	responseBytes, err := compositeBytes(response)
	if err != nil {
		return nil, err
	}
	audit := &spacev1.AppendAuditEventRequest{AuditEventId: auditID.String(), SpaceId: space.String(), ActorProfileId: initiator.ProfileID.String(),
		Action: "chat_created", TargetType: "chat", TargetId: chatID.String(), DetailsJson: "{}", OccurredAt: timestamppb.New(now)}
	auditBytes, err := compositeBytes(audit)
	if err != nil {
		return nil, err
	}
	eventID := uuid.NewSHA1(operation, []byte("voice.chat.space_composite.v1.chat.created"))
	event := &eventsv1.ChatStreamEvent{EventId: eventID.String(), OccurredAt: timestamppb.New(now),
		Payload: &eventsv1.ChatStreamEvent_ChatCreated{ChatCreated: &eventsv1.ChatCreated{ChatId: chatID.String(), Type: kind}}}
	eventBytes, err := compositeBytes(event)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat_space_composite_operations(operation_id,space_id,chat_id,node_id,audit_event_id,chat_type,state,
		creator_profile_id,client_request_id,create_request_sha256,binding_bytes,response_bytes,created_at)
		VALUES($1,$2,$3,$4,$5,$6,'PREPARED',$7,$8,$9,$10,$11,$12)`,
		operation, space, chatID, nodeID, auditID, kind, initiator.ProfileID, key, digest[:], bindingBytes, responseBytes, now)
	if err != nil {
		return nil, err
	}
	for _, effect := range []struct {
		kind    string
		id      uuid.UUID
		payload []byte
	}{{"AUDIT", auditID, auditBytes}, {"CHAT_CREATED", eventID, eventBytes}} {
		_, err = tx.Exec(ctx, "INSERT INTO chat_space_composite_outbox(operation_id,effect_kind,event_id,eligibility,payload) VALUES($1,$2,$3,'HELD',$4)",
			operation, effect.kind, effect.id, effect.payload)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &SpaceCompositeOperation{Binding: binding, Response: response, State: "PREPARED"}, nil
}

func loadSpaceComposite(ctx context.Context, tx pgx.Tx, operation uuid.UUID, forUpdate bool) (*SpaceCompositeOperation, error) {
	query := "SELECT state,failure_code,binding_bytes,response_bytes,COALESCE(full_evidence_expires_at<=clock_timestamp(),false),space_id,chat_id,node_id,audit_event_id,creator_profile_id,create_request_sha256,chat_type,created_at FROM chat_space_composite_operations WHERE operation_id=$1"
	if forUpdate {
		query += " FOR UPDATE"
	}
	var state string
	var code int32
	var binding, response []byte
	var expired bool
	var space, chat, node, audit uuid.UUID
	var actor *uuid.UUID
	var digest []byte
	var kind string
	var createdAt time.Time
	if err := tx.QueryRow(ctx, query, operation).Scan(&state, &code, &binding, &response, &expired, &space, &chat, &node, &audit, &actor, &digest, &kind, &createdAt); err != nil {
		return nil, err
	}
	if expired {
		return nil, ErrSpaceCompositeExpired
	}
	b, r := &spacev1.SpaceChatCreationBinding{}, &chatv1.CreateChatResponse{}
	if proto.Unmarshal(binding, b) != nil || proto.Unmarshal(response, r) != nil || chatcreation.HasUnknown(r) || !chatcreation.ValidBinding(b) || b.OperationId != operation.String() ||
		b.SpaceId != space.String() || b.ChatId != chat.String() || b.NodeId != node.String() || b.AuditEventId != audit.String() ||
		actor == nil || b.CreatorProfileId != actor.String() || !bytes.Equal(b.CreateRequestSha256, digest) ||
		(kind == "group") != (b.ChatType == chatv1.ChatType_CHAT_TYPE_GROUP) ||
		r.GetCompositeOperationId() != b.OperationId || r.GetCompositeNodeId() != b.NodeId || r.GetChat().GetId() != b.ChatId ||
		r.GetChat().GetSpaceId() != b.SpaceId || r.GetChat().GetType() != b.ChatType || r.GetChat().GetCreatorProfileId() != b.CreatorProfileId ||
		r.GetChat().GetCreatedAt() == nil || r.GetChat().GetCreatedAt().CheckValid() != nil {
		return nil, ErrSpaceCompositeEvidence
	}
	c := r.Chat
	_, _, _, _, _, normalized, err := compositeNormalization(&chatv1.CreateChatRequest{Type: c.Type, SpaceId: c.SpaceId, Name: c.Name, Topic: c.Topic})
	if err != nil || !bytes.Equal(normalized[:], b.CreateRequestSha256) || !proto.Equal(c.CreatedAt, timestamppb.New(createdAt)) ||
		!proto.Equal(c.UpdatedAt, c.CreatedAt) || c.ThreadsEnabled != (kind == "channel") || c.AvatarUrl != nil || c.LastMessageAt != nil ||
		c.SlowModeSeconds != 0 || c.AllowUserMainFeed != (kind == "group") || c.E2EEnabled || c.AllowGuests {
		return nil, ErrSpaceCompositeEvidence
	}
	return &SpaceCompositeOperation{State: state, FailureCode: codes.Code(code), Binding: b, Response: r}, nil
}

func validateHeldSpaceComposite(ctx context.Context, tx pgx.Tx, op *SpaceCompositeOperation) error {
	b, c := op.Binding, op.Response.Chat
	audit := &spacev1.AppendAuditEventRequest{AuditEventId: b.AuditEventId, SpaceId: b.SpaceId, ActorProfileId: b.CreatorProfileId,
		Action: "chat_created", TargetType: "chat", TargetId: b.ChatId, DetailsJson: "{}", OccurredAt: c.CreatedAt}
	eventID := uuid.NewSHA1(uuid.MustParse(b.OperationId), []byte("voice.chat.space_composite.v1.chat.created"))
	kind := "group"
	if b.ChatType == chatv1.ChatType_CHAT_TYPE_CHANNEL {
		kind = "channel"
	}
	event := &eventsv1.ChatStreamEvent{EventId: eventID.String(), OccurredAt: c.CreatedAt, Payload: &eventsv1.ChatStreamEvent_ChatCreated{ChatCreated: &eventsv1.ChatCreated{ChatId: b.ChatId, Type: kind}}}
	auditBytes, err := compositeBytes(audit)
	if err != nil {
		return err
	}
	eventBytes, err := compositeBytes(event)
	if err != nil {
		return err
	}
	expected := map[string]struct {
		id   string
		body []byte
	}{"AUDIT": {b.AuditEventId, auditBytes}, "CHAT_CREATED": {eventID.String(), eventBytes}}
	rows, err := tx.Query(ctx, "SELECT effect_kind,event_id,eligibility,payload FROM chat_space_composite_outbox WHERE operation_id=$1 ORDER BY effect_kind FOR UPDATE", b.OperationId)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var kind, id, eligibility string
		var payload []byte
		if err = rows.Scan(&kind, &id, &eligibility, &payload); err != nil {
			return err
		}
		want, ok := expected[kind]
		if !ok || id != want.id || eligibility != "HELD" || !bytes.Equal(payload, want.body) {
			return ErrSpaceCompositeEvidence
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if count != 2 {
		return ErrSpaceCompositeEvidence
	}
	return nil
}

func (s *DMStore) LoadSpaceComposite(ctx context.Context, operation uuid.UUID) (*SpaceCompositeOperation, error) {
	tx, err := beginSpaceComposite(ctx, s)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := loadSpaceComposite(ctx, tx, operation, false)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func validCompositeOutcome(b *spacev1.SpaceChatCreationBinding, out *spacev1.SpaceChatCreationOutcome) bool {
	if out == nil || !chatcreation.ValidBinding(b) || chatcreation.HasUnknown(out) || !proto.Equal(b, out.GetBinding()) {
		return false
	}
	if _, err := principal.RequestHash(out); err != nil {
		return false
	}
	switch out.State {
	case spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED:
		n := out.Node
		return out.FailureCode == 0 && n != nil && n.Id == b.NodeId && n.SpaceId == b.SpaceId && n.Kind == "text_chat" &&
			n.GetVoiceRoomId() == "" && n.GetLinkedChat().GetId() == b.ChatId &&
			(n.GetLinkedChat().Type == nil || n.GetLinkedChat().GetType() == b.ChatType)
	case spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED:
		switch codes.Code(out.FailureCode) {
		case codes.PermissionDenied, codes.NotFound, codes.FailedPrecondition, codes.ResourceExhausted:
			return out.Node == nil
		}
	}
	return false
}

// ResolveSpaceComposite is called only with the authenticated Space response.
// No absence/timeout/lease expiry is a terminal outcome.
func (s *DMStore) ResolveSpaceComposite(ctx context.Context, operation uuid.UUID, outcome *spacev1.SpaceChatCreationOutcome) (*SpaceCompositeOperation, error) {
	return s.resolveSpaceComposite(ctx, operation, outcome, nil)
}

func (s *DMStore) resolveSpaceComposite(ctx context.Context, operation uuid.UUID, outcome *spacev1.SpaceChatCreationOutcome, token *uuid.UUID) (*SpaceCompositeOperation, error) {
	tx, err := beginSpaceComposite(ctx, s)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var space uuid.UUID
	var actor, key *uuid.UUID
	if err = tx.QueryRow(ctx, "SELECT space_id,creator_profile_id,client_request_id FROM chat_space_composite_operations WHERE operation_id=$1", operation).Scan(&space, &actor, &key); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::bigint)", spacemutationlock.Key(space)); err != nil {
		return nil, err
	}
	if key != nil {
		if _, err = tx.Exec(ctx, "SELECT 1 FROM chat_create_requests WHERE creator_profile_id=$1 AND request_id=$2 FOR UPDATE", actor, key); err != nil {
			return nil, err
		}
	}
	saved, err := loadSpaceComposite(ctx, tx, operation, true)
	if err != nil {
		return nil, err
	}
	if !validCompositeOutcome(saved.Binding, outcome) {
		return nil, ErrSpaceCompositeEvidence
	}
	terminal := "COMPLETE"
	if outcome.State == spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED {
		terminal = "ABORTED"
	}
	outcomeBytes, err := compositeBytes(outcome)
	if err != nil {
		return nil, err
	}
	if saved.State != "PREPARED" {
		var original []byte
		if err = tx.QueryRow(ctx, "SELECT outcome_bytes FROM chat_space_composite_operations WHERE operation_id=$1", operation).Scan(&original); err != nil {
			return nil, err
		}
		if saved.State != terminal || !bytes.Equal(original, outcomeBytes) {
			return nil, ErrSpaceCompositeEvidence
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return saved, nil
	}
	if err = validateHeldSpaceComposite(ctx, tx, saved); err != nil {
		return nil, err
	}
	if token != nil {
		var valid bool
		if err = tx.QueryRow(ctx, "SELECT lease_token=$2 AND lease_expires_at>clock_timestamp() FROM chat_space_composite_operations WHERE operation_id=$1", operation, *token).Scan(&valid); err != nil {
			return nil, err
		}
		if !valid {
			return nil, ErrSpaceCompositeLease
		}
	}
	var lifecycle string
	err = tx.QueryRow(ctx, "SELECT state FROM chat_space_lifecycle_fences WHERE space_id=$1", space).Scan(&lifecycle)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if lifecycle == "PURGED" || lifecycle == "PURGE_DECIDED" {
		return nil, ErrSpaceLifecycleState
	}
	eligibility := "CANCELLED"
	if terminal == "COMPLETE" {
		c := saved.Response.Chat
		kind := "group"
		if c.Type == chatv1.ChatType_CHAT_TYPE_CHANNEL {
			kind = "channel"
		}
		_, err = tx.Exec(ctx, `INSERT INTO chats(id,type,space_id,name,topic,creator_profile_id,created_at,updated_at,threads_enabled,allow_user_main_feed)
			VALUES($1,$2,$3,$4,$5,$6,$7,$7,$8,$9)`, c.Id, kind, space, c.Name, c.Topic, c.CreatorProfileId, c.CreatedAt.AsTime(), c.ThreadsEnabled, c.AllowUserMainFeed)
		if err != nil {
			return nil, err
		}
		if key != nil {
			row := ChatRow{ID: uuid.MustParse(c.Id), Type: kind, SpaceID: &space, Name: c.Name, Topic: c.Topic,
				CreatorProfileID: uuid.MustParse(c.CreatorProfileId), CreatedAt: c.CreatedAt.AsTime(), UpdatedAt: c.UpdatedAt.AsTime(), ThreadsEnabled: c.ThreadsEnabled, AllowUserMainFeed: c.AllowUserMainFeed}
			result, err := json.Marshal(&row)
			if err != nil {
				return nil, err
			}
			if _, err = tx.Exec(ctx, "UPDATE chat_create_requests SET chat_id=$3,result_bytes=$4 WHERE creator_profile_id=$1 AND request_id=$2 AND operation_id=$5",
				actor, key, c.Id, result, operation); err != nil {
				return nil, err
			}
		}
		eligibility = "READY"
	}
	_, err = tx.Exec(ctx, "UPDATE chat_space_composite_outbox SET eligibility=$2 WHERE operation_id=$1 AND eligibility='HELD'", operation, eligibility)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "UPDATE chat_space_composite_operations SET state=$2,failure_code=$3,outcome_bytes=$4,lease_token=NULL,lease_expires_at=NULL WHERE operation_id=$1 AND state='PREPARED'",
		operation, terminal, outcome.FailureCode, outcomeBytes)
	if err != nil {
		return nil, err
	}
	saved.State, saved.FailureCode = terminal, codes.Code(outcome.FailureCode)
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return saved, nil
}
