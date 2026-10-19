package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
)

var ErrChatScopeObservation = errors.New("Chat source observation is invalid")

// ChatScopeObservation contains facts obtained by the authenticated private Chat
// caller. It is not a transferable capability or a reservation over Chat state.
type ChatScopeObservation struct {
	chat, space, operation, incarnation uuid.UUID
	revision, generation                uint64
	request, response                   []byte
	requestSHA, responseSHA             [32]byte
	exists, live                        bool
}

func NewChatScopeObservation(request *chatv1.GetBotChatScopeAuthorityRequest, response *chatv1.GetBotChatScopeAuthorityResponse, admission bool) (ChatScopeObservation, error) {
	var o ChatScopeObservation
	if request == nil || response == nil || response.Scope == nil || request.ProtocolVersion != 1 || len(request.ProtoReflect().GetUnknown()) != 0 || len(response.ProtoReflect().GetUnknown()) != 0 || len(response.Scope.ProtoReflect().GetUnknown()) != 0 {
		return o, ErrChatScopeObservation
	}
	for _, item := range []struct {
		raw    string
		target *uuid.UUID
	}{{request.ChatId, &o.chat}, {request.ExpectedSpaceId, &o.space}, {request.OperationId, &o.operation}, {response.Scope.ChatScopeIncarnation, &o.incarnation}} {
		id, err := uuid.Parse(item.raw)
		if err != nil || id == uuid.Nil || id.String() != item.raw {
			return o, ErrChatScopeObservation
		}
		*item.target = id
	}
	s := response.Scope
	if s.ProtocolVersion != 1 || s.ChatId != request.ChatId || s.ExpectedSpaceId != request.ExpectedSpaceId || s.ActualSpaceId != request.ExpectedSpaceId || s.OperationId != request.OperationId || s.ChatAuthorityRevision == 0 || s.ChatAuthorityRevision > math.MaxInt64 || s.LifecycleGeneration > math.MaxInt64 || s.LifecycleState < chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE || s.LifecycleState > chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_PURGED {
		return o, ErrChatScopeObservation
	}
	if s.Exists {
		if (s.ChatType != chatv1.ChatType_CHAT_TYPE_GROUP && s.ChatType != chatv1.ChatType_CHAT_TYPE_CHANNEL) || s.ManagedKind < chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY || s.ManagedKind > chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD || s.Live != (s.LifecycleState == chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE) {
			return o, ErrChatScopeObservation
		}
	} else if s.Live || s.ChatType != chatv1.ChatType_CHAT_TYPE_UNSPECIFIED || s.ManagedKind != chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED {
		return o, ErrChatScopeObservation
	}
	if admission && (!s.Exists || !s.Live) {
		return o, ErrChatScopeObservation
	}
	var err error
	o.request, err = (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return o, ErrChatScopeObservation
	}
	o.requestSHA = sha256.Sum256(o.request)
	if !bytes.Equal(response.RequestSha256, o.requestSHA[:]) {
		return o, ErrChatScopeObservation
	}
	o.response, err = (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return o, ErrChatScopeObservation
	}
	o.responseSHA = sha256.Sum256(o.response)
	o.revision, o.generation, o.exists, o.live = s.ChatAuthorityRevision, s.LifecycleGeneration, s.Exists, s.Live
	return o, nil
}

type chatScopeObservationsKey struct{}

func WithChatScopeObservations(ctx context.Context, observations []ChatScopeObservation) context.Context {
	copy := append([]ChatScopeObservation(nil), observations...)
	for i := range copy {
		copy[i].request = bytes.Clone(copy[i].request)
		copy[i].response = bytes.Clone(copy[i].response)
	}
	return context.WithValue(ctx, chatScopeObservationsKey{}, copy)
}

// The service's qualified writers supply the complete accepted set. Legacy
// internal persistence callers without an observation context remain explicitly
// unqualified; they do not acquire source provenance through a fabricated row.
func persistChatScopeObservations(ctx context.Context, tx pgx.Tx, bot, space uuid.UUID, chats []uuid.UUID, intent *uuid.UUID, origin string) ([]uuid.UUID, error) {
	observations, qualified := ctx.Value(chatScopeObservationsKey{}).([]ChatScopeObservation)
	if !qualified {
		return nil, nil
	}
	chats = normalizedInstallationChats(chats)
	if len(chats) != len(observations) {
		return nil, ErrChatScopeObservation
	}
	byChat := make(map[uuid.UUID]ChatScopeObservation, len(observations))
	for _, o := range observations {
		var request chatv1.GetBotChatScopeAuthorityRequest
		var response chatv1.GetBotChatScopeAuthorityResponse
		if proto.Unmarshal(o.request, &request) != nil || proto.Unmarshal(o.response, &response) != nil {
			return nil, ErrChatScopeObservation
		}
		checked, err := NewChatScopeObservation(&request, &response, true)
		if err != nil || checked.space != space || checked.requestSHA != o.requestSHA || checked.responseSHA != o.responseSHA {
			return nil, ErrChatScopeObservation
		}
		if _, duplicate := byChat[checked.chat]; duplicate {
			return nil, ErrChatScopeObservation
		}
		byChat[checked.chat] = checked
	}
	ids := make([]uuid.UUID, 0, len(chats))
	for _, chat := range chats {
		o, ok := byChat[chat]
		if !ok {
			return nil, ErrChatScopeObservation
		}
		_, err := tx.Exec(ctx, `INSERT INTO bot_chat_scope_observations(observation_id,bot_id,chat_id,space_id,intent_operation_id,origin_kind,request_bytes,request_sha256,response_bytes,response_sha256,scope_incarnation,authority_revision,lifecycle_generation,source_exists,source_live) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT(observation_id) DO NOTHING`, o.operation, bot, chat, space, intent, origin, o.request, o.requestSHA[:], o.response, o.responseSHA[:], o.incarnation, int64(o.revision), int64(o.generation), o.exists, o.live)
		if err != nil {
			return nil, err
		}
		var storedBot, storedChat, storedSpace uuid.UUID
		var storedRequest, storedResponse []byte
		var storedIntent *uuid.UUID
		var storedOrigin string
		err = tx.QueryRow(ctx, `SELECT bot_id,chat_id,space_id,request_bytes,response_bytes,intent_operation_id,origin_kind FROM bot_chat_scope_observations WHERE observation_id=$1`, o.operation).Scan(&storedBot, &storedChat, &storedSpace, &storedRequest, &storedResponse, &storedIntent, &storedOrigin)
		if err != nil {
			return nil, err
		}
		if storedBot != bot || storedChat != chat || storedSpace != space || !bytes.Equal(storedRequest, o.request) || !bytes.Equal(storedResponse, o.response) || storedOrigin != origin || (storedIntent == nil) != (intent == nil) || (intent != nil && *storedIntent != *intent) {
			return nil, ErrChatScopeObservation
		}
		ids = append(ids, o.operation)
	}
	return ids, nil
}

func persistInitialInstallationScopes(ctx context.Context, tx pgx.Tx, intent *InstallationIntent) error {
	ids, err := persistChatScopeObservations(ctx, tx, intent.BotID, intent.SpaceID, intent.Chats, &intent.OperationID, "install_admission")
	if err != nil || ids == nil {
		return err
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE bot_installation_intents SET chat_scope_observation_ids=$2::jsonb WHERE operation_id=$1`, intent.OperationID, raw)
	return err
}

type retainedChatScopeObservation struct {
	observation ChatScopeObservation
	intent      *uuid.UUID
	origin      string
}

// Cleanup retains the original accepted attachment separately from a newer
// point-in-time admission check. Neither observation is a Chat state lease.
type ChatScopeCleanupObservation struct {
	OperationID uuid.UUID `json:"operation_id"`
	Request     []byte    `json:"request"`
	Response    []byte    `json:"response"`
	RequestSHA  []byte    `json:"request_sha256"`
	ResponseSHA []byte    `json:"response_sha256"`
}

type ChatScopeCleanupSource struct {
	ChatID         uuid.UUID                   `json:"chat_id"`
	InstallationID uuid.UUID                   `json:"installation_id"`
	Original       ChatScopeCleanupObservation `json:"original"`
	Current        ChatScopeCleanupObservation `json:"current"`
}

func snapshotChatScopeSources(ctx context.Context, tx pgx.Tx, bot, space uuid.UUID) ([]ChatScopeCleanupSource, error) {
	rows, err := tx.Query(ctx, `SELECT chat_id,(to_jsonb(w)->>'origin_scope_observation_id')::uuid,(to_jsonb(w)->>'scope_observation_id')::uuid,(to_jsonb(w)->>'source_installation_id')::uuid FROM bot_chat_whitelist w WHERE bot_id=$1 AND space_id=$2 ORDER BY chat_id FOR UPDATE`, bot, space)
	if err != nil {
		return nil, err
	}
	type pointers struct {
		chat                            uuid.UUID
		original, current, installation *uuid.UUID
	}
	var pending []pointers
	for rows.Next() {
		var p pointers
		if err = rows.Scan(&p.chat, &p.original, &p.current, &p.installation); err != nil {
			rows.Close()
			return nil, err
		}
		pending = append(pending, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var result []ChatScopeCleanupSource
	for _, p := range pending {
		if p.original == nil && p.current == nil && p.installation == nil {
			continue
		} // actual unqualified legacy row
		if p.original == nil || p.current == nil || p.installation == nil {
			return nil, ErrChatScopeObservation
		}
		source := ChatScopeCleanupSource{ChatID: p.chat, InstallationID: *p.installation}
		for index, id := range []uuid.UUID{*p.original, *p.current} {
			r, err := readChatScopeObservation(ctx, tx, bot, space, id)
			if err != nil {
				return nil, err
			}
			o := r.observation
			if o.chat != p.chat {
				return nil, ErrChatScopeObservation
			}
			copy := ChatScopeCleanupObservation{OperationID: id, Request: bytes.Clone(o.request), Response: bytes.Clone(o.response), RequestSHA: bytes.Clone(o.requestSHA[:]), ResponseSHA: bytes.Clone(o.responseSHA[:])}
			if index == 0 {
				source.Original = copy
			} else {
				source.Current = copy
			}
		}
		result = append(result, source)
	}
	return result, nil
}

func readChatScopeObservation(ctx context.Context, tx pgx.Tx, bot, space, id uuid.UUID) (retainedChatScopeObservation, error) {
	var result retainedChatScopeObservation
	var storedBot, chat, storedSpace, incarnation uuid.UUID
	var request, response, requestSHA, responseSHA []byte
	var revision, generation int64
	var exists, live bool
	err := tx.QueryRow(ctx, `SELECT bot_id,chat_id,space_id,request_bytes,request_sha256,response_bytes,response_sha256,scope_incarnation,authority_revision,lifecycle_generation,source_exists,source_live,intent_operation_id,origin_kind FROM bot_chat_scope_observations WHERE observation_id=$1`, id).Scan(&storedBot, &chat, &storedSpace, &request, &requestSHA, &response, &responseSHA, &incarnation, &revision, &generation, &exists, &live, &result.intent, &result.origin)
	if err != nil {
		return result, err
	}
	var req chatv1.GetBotChatScopeAuthorityRequest
	var resp chatv1.GetBotChatScopeAuthorityResponse
	if proto.Unmarshal(request, &req) != nil || proto.Unmarshal(response, &resp) != nil {
		return result, ErrChatScopeObservation
	}
	o, err := NewChatScopeObservation(&req, &resp, true)
	if err != nil || storedBot != bot || storedSpace != space || o.chat != chat || o.space != space || o.operation != id || o.incarnation != incarnation || o.revision != uint64(revision) || o.generation != uint64(generation) || o.exists != exists || o.live != live || !bytes.Equal(o.request, request) || !bytes.Equal(o.response, response) || !bytes.Equal(o.requestSHA[:], requestSHA) || !bytes.Equal(o.responseSHA[:], responseSHA) {
		return result, ErrChatScopeObservation
	}
	result.observation = o
	return result, nil
}

func attachChatScopeObservations(ctx context.Context, tx pgx.Tx, bot, space uuid.UUID, ids []uuid.UUID) error {
	seenIDs, seenChats := make(map[uuid.UUID]bool, len(ids)), make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if seenIDs[id] {
			return ErrChatScopeObservation
		}
		seenIDs[id] = true
		retained, err := readChatScopeObservation(ctx, tx, bot, space, id)
		if err != nil {
			return err
		}
		chat := retained.observation.chat
		if seenChats[chat] {
			return ErrChatScopeObservation
		}
		seenChats[chat] = true
		result, err := tx.Exec(ctx, `UPDATE bot_chat_whitelist w SET scope_observation_id=$4,
origin_scope_observation_id=CASE WHEN w.source_installation_id=i.id THEN COALESCE(w.origin_scope_observation_id,$4) ELSE $4 END,
source_installation_id=i.id
FROM bot_space_installations i WHERE w.bot_id=$1 AND w.chat_id=$2 AND w.space_id=$3 AND i.bot_id=w.bot_id AND i.space_id=w.space_id`, bot, chat, space, id)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ErrChatScopeObservation
		}
	}
	return nil
}

// ReusableInstallationIntent recovers only an exact accepted native request.
// New targets/generations still require a fresh authenticated Chat observation.
func (s *BotStore) ReusableInstallationIntent(ctx context.Context, bot, space uuid.UUID, chats []uuid.UUID) (*InstallationIntent, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	row, err := s.lockInstallation(ctx, tx, bot, space, true)
	if err != nil {
		return nil, err
	}
	intent, err := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE bot_id=$1 AND space_id=$2 AND state<>'CANCELED' FOR UPDATE`, bot, space))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	chats = normalizedInstallationChats(chats)
	if intent.ActorProfileID != row.ActorProfileID || intent.State == "CANCELING" {
		return nil, ErrInstallationConflict
	}
	if len(intent.Chats) != len(chats) {
		return nil, nil
	}
	for index, chat := range chats {
		if intent.Chats[index] != chat {
			return nil, nil
		}
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT to_jsonb(i)->'chat_scope_observation_ids' FROM bot_installation_intents i WHERE operation_id=$1`, intent.OperationID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	if len(raw) > 0 && json.Unmarshal(raw, &ids) != nil {
		return nil, ErrChatScopeObservation
	}
	if len(chats) > 0 && len(ids) == 0 {
		return nil, nil
	}
	if len(ids) != len(chats) {
		return nil, ErrChatScopeObservation
	}
	wanted := make(map[uuid.UUID]bool, len(chats))
	for _, chat := range chats {
		wanted[chat] = true
	}
	for _, id := range ids {
		retained, err := readChatScopeObservation(ctx, tx, bot, space, id)
		if err != nil {
			return nil, err
		}
		if retained.intent == nil || *retained.intent != intent.OperationID || retained.origin != "install_admission" || !wanted[retained.observation.chat] {
			return nil, ErrChatScopeObservation
		}
		delete(wanted, retained.observation.chat)
	}
	if len(wanted) > 0 {
		return nil, ErrChatScopeObservation
	}
	return intent, tx.Commit(ctx)
}
