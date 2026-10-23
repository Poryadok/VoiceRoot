package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"time"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/installprincipal"
)

func membershipJournalExists(ctx context.Context, tx pgx.Tx) (bool, error) {
	var present bool
	err := tx.QueryRow(ctx, `SELECT to_regclass('bot_space_membership_source') IS NOT NULL`).Scan(&present)
	return present, err
}

type MembershipChatSource struct {
	ChatID, AttachmentID uuid.UUID
	Request              *chatv1.GetBotChatScopeAuthorityRequest
	Response             *chatv1.GetBotChatScopeAuthorityResponse
}

func (s *BotStore) PrepareMembershipChat(ctx context.Context, source uuid.UUID, claim MembershipRecord, chat uuid.UUID) (MembershipChatSource, error) {
	result := MembershipChatSource{ChatID: chat}
	tx, err := s.membershipTx(ctx, source, claim.SpaceID, claim.BotID)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, claim.SpaceID, claim.BotID)
	if err != nil {
		return result, err
	}
	if !sameAttempt(r, claim) || r.Revision != claim.Revision || r.Desired != "INSTALL" {
		return result, ErrMembershipConflict
	}
	if err = membershipLive(ctx, tx, r.SpaceID); err != nil {
		return result, err
	}
	var raw, response []byte
	err = tx.QueryRow(ctx, `SELECT attachment_id,request_bytes,response_bytes FROM bot_space_membership_chat_sources WHERE add_operation_id=$1 AND completion_revision=$2 AND chat_id=$3`, r.AddOperationID, r.CompletionRevision, chat).Scan(&result.AttachmentID, &raw, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		var inputs botv1.InstallBotInSpaceRequest
		if proto.Unmarshal(r.CompletionBytes, &inputs) != nil {
			return result, ErrMembershipSource
		}
		found := false
		for _, c := range inputs.AllowedChats {
			found = found || c.GetId() == chat.String()
		}
		if !found {
			return result, ErrMembershipConflict
		}
		result.AttachmentID = uuid.New()
		result.Request = &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ChatId: chat.String(), ExpectedSpaceId: r.SpaceID.String()}
		raw, err = proto.MarshalOptions{Deterministic: true}.Marshal(result.Request)
		if err != nil {
			return result, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_chat_sources(add_operation_id,completion_revision,chat_id,attachment_id,request_bytes) VALUES($1,$2,$3,$4,$5)`, r.AddOperationID, r.CompletionRevision, chat, result.AttachmentID, raw)
	}
	if err != nil {
		return result, err
	}
	result.Request = new(chatv1.GetBotChatScopeAuthorityRequest)
	if proto.Unmarshal(raw, result.Request) != nil {
		return result, ErrMembershipSource
	}
	if response != nil {
		result.Response = new(chatv1.GetBotChatScopeAuthorityResponse)
		if proto.Unmarshal(response, result.Response) != nil {
			return result, ErrMembershipSource
		}
		if _, err = NewChatScopeObservation(result.Request, result.Response, true); err != nil {
			return result, err
		}
	}
	return result, tx.Commit(ctx)
}
func (s *BotStore) SaveMembershipChat(ctx context.Context, source uuid.UUID, claim MembershipRecord, item MembershipChatSource) error {
	if _, err := NewChatScopeObservation(item.Request, item.Response, true); err != nil {
		return err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(item.Response)
	if err != nil {
		return err
	}
	request, err := proto.MarshalOptions{Deterministic: true}.Marshal(item.Request)
	if err != nil {
		return err
	}
	tx, err := s.membershipTx(ctx, source, claim.SpaceID, claim.BotID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, claim.SpaceID, claim.BotID)
	if err != nil {
		return err
	}
	if !sameAttempt(r, claim) || r.Revision != claim.Revision || r.Desired != "INSTALL" {
		return ErrMembershipConflict
	}
	if err = membershipLive(ctx, tx, r.SpaceID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE bot_space_membership_chat_sources SET response_bytes=$6 WHERE add_operation_id=$1 AND completion_revision=$2 AND chat_id=$3 AND attachment_id=$4 AND request_bytes=$5 AND (response_bytes IS NULL OR response_bytes=$6)`, r.AddOperationID, r.CompletionRevision, item.ChatID, item.AttachmentID, request, raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMembershipConflict
	}
	return tx.Commit(ctx)
}

type MembershipCleanupDecision struct {
	ID                        uuid.UUID
	DecisionBytes, ScopeBytes []byte
	Sources                   []MembershipChatSource
}

type membershipScope struct {
	SpaceID, BotID, InstallationID, ActorProfileID, AddOperationID, CancelOperationID uuid.UUID
	Generation                                                                        int64
	Sources                                                                           []membershipScopeSource
}
type membershipScopeSource struct {
	ChatID, AttachmentID uuid.UUID
	Request, Response    []byte
}

func frozenMembershipScope(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, r MembershipRecord) ([]byte, error) {
	scope := membershipScope{SpaceID: r.SpaceID, BotID: r.BotID, InstallationID: r.InstallationID, ActorProfileID: r.ActorProfileID, AddOperationID: r.AddOperationID, CancelOperationID: r.CancelOperationID, Generation: r.Generation}
	rows, err := q.Query(ctx, `SELECT chat_id,attachment_id,request_bytes,response_bytes FROM bot_space_membership_chat_sources WHERE add_operation_id=$1 AND response_bytes IS NOT NULL ORDER BY chat_id,attachment_id`, r.AddOperationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item membershipScopeSource
		if err = rows.Scan(&item.ChatID, &item.AttachmentID, &item.Request, &item.Response); err != nil {
			return nil, err
		}
		scope.Sources = append(scope.Sources, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(scope)
}

func (s *BotStore) MembershipCleanupDecision(ctx context.Context, claim MembershipRecord) (MembershipCleanupDecision, error) {
	var d MembershipCleanupDecision
	err := s.Pool.QueryRow(ctx, `SELECT decision_id,decision_bytes,scope_bytes FROM bot_space_membership_cleanup_decisions WHERE add_operation_id=$1`, claim.AddOperationID).Scan(&d.ID, &d.DecisionBytes, &d.ScopeBytes)
	if err != nil {
		return d, err
	}
	expected, err := frozenMembershipScope(ctx, s.Pool, claim)
	if err != nil {
		return d, err
	}
	if !bytes.Equal(expected, d.ScopeBytes) {
		return d, ErrMembershipSource
	}
	rows, err := s.Pool.Query(ctx, `SELECT chat_id,attachment_id,request_bytes,response_bytes FROM bot_space_membership_chat_sources WHERE add_operation_id=$1 ORDER BY chat_id,attachment_id`, claim.AddOperationID)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var item MembershipChatSource
		var request, response []byte
		if err = rows.Scan(&item.ChatID, &item.AttachmentID, &request, &response); err != nil {
			return d, err
		}
		if response == nil {
			continue
		}
		item.Request = new(chatv1.GetBotChatScopeAuthorityRequest)
		item.Response = new(chatv1.GetBotChatScopeAuthorityResponse)
		if proto.Unmarshal(request, item.Request) != nil || proto.Unmarshal(response, item.Response) != nil {
			return d, ErrMembershipSource
		}
		if _, err = NewChatScopeObservation(item.Request, item.Response, true); err != nil {
			return d, err
		}
		d.Sources = append(d.Sources, item)
	}
	return d, rows.Err()
}
func persistMembershipCleanupDecision(ctx context.Context, tx pgx.Tx, r MembershipRecord, a installprincipal.Administrator) error {
	var existing uuid.UUID
	err := tx.QueryRow(ctx, `SELECT decision_id FROM bot_space_membership_cleanup_decisions WHERE add_operation_id=$1`, r.AddOperationID).Scan(&existing)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if !time.Now().Before(a.ExpiresAt()) {
		return ErrMembershipConflict
	}
	id := uuid.New()
	request := &botv1.UninstallBotFromSpaceRequest{BotId: r.BotID.String(), SpaceId: r.SpaceID.String()}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return err
	}
	decision := struct {
		ID, BotID, ActorProfileID, AdministratorAccount, AdministratorProfile uuid.UUID
		Purpose, RequestID, RequestHash                                       string
		SessionEpoch                                                          int64
		ExpiresAt                                                             time.Time
		Request                                                               []byte
	}{id, r.BotID, r.ActorProfileID, a.AccountID(), a.ProfileID(), "space_uninstall", a.RequestID(), a.RequestHash(), a.SessionEpoch(), a.ExpiresAt(), raw}
	decisionBytes, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	scopeBytes, err := frozenMembershipScope(ctx, tx, r)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_cleanup_decisions(add_operation_id,decision_id,decision_bytes,scope_bytes) VALUES($1,$2,$3,$4)`, r.AddOperationID, id, decisionBytes, scopeBytes)
	return err
}
func (d MembershipCleanupDecision) Digests() ([]byte, []byte) {
	a, b := sha256.Sum256(d.DecisionBytes), sha256.Sum256(d.ScopeBytes)
	return bytes.Clone(a[:]), bytes.Clone(b[:])
}
