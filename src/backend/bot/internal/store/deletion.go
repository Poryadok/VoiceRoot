package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrDeletionOwner = errors.New("Bot deletion owner does not match")

// DeletionAdmission contains only verified request-bound identity, never a
// bearer credential. Its bytes become part of the immutable owner decision.
type DeletionAdmission struct {
	AccountID    uuid.UUID
	ProfileID    uuid.UUID
	SessionEpoch int64
	AccountType  string
	Method       string
	RequestID    string
	RequestHash  string
	RequestBytes []byte
	ExpiresAt    time.Time
}

type BotDeletion struct {
	OperationID     uuid.UUID
	BotID           uuid.UUID
	OwnerAccountID  uuid.UUID
	ActorProfileID  uuid.UUID
	DecisionBytes   []byte
	DecisionSHA256  []byte
	State           string
	ClaimGeneration int64
}

// BotDeletionTarget is an immutable recovery snapshot. LegacyEvidence is
// explicit: absent historical installation credentials are never invented.
type BotDeletionTarget struct {
	TargetID           uuid.UUID   `json:"target_id"`
	OperationID        uuid.UUID   `json:"operation_id"`
	BotID              uuid.UUID   `json:"bot_id"`
	SpaceID            uuid.UUID   `json:"space_id"`
	ActorProfileID     uuid.UUID   `json:"actor_profile_id"`
	InstallationID     uuid.UUID   `json:"installation_id"`
	InstallOperationID uuid.UUID   `json:"install_operation_id"`
	BotGeneration      int64       `json:"bot_generation"`
	CancelOperationID  uuid.UUID   `json:"cancel_operation_id"`
	Chats              []uuid.UUID `json:"chat_ids"`
	LegacyEvidence     bool        `json:"legacy_evidence"`
	ScopeBytes         []byte      `json:"-"`
	ScopeSHA256        []byte      `json:"-"`
}

const deletionColumns = `operation_id,bot_id,owner_account_id,actor_profile_id,decision_bytes,decision_sha256,state,claim_generation`

func scanBotDeletion(row pgx.Row) (*BotDeletion, error) {
	var d BotDeletion
	err := row.Scan(&d.OperationID, &d.BotID, &d.OwnerAccountID, &d.ActorProfileID, &d.DecisionBytes, &d.DecisionSHA256, &d.State, &d.ClaimGeneration)
	return &d, err
}

type deletionQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func deletionSpaces(ctx context.Context, q deletionQuerier, bot uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT space_id FROM bot_space_installations WHERE bot_id=$1 UNION SELECT space_id FROM bot_chat_whitelist WHERE bot_id=$1 UNION SELECT space_id FROM bot_installation_intents WHERE bot_id=$1 AND state<>'CANCELED' ORDER BY space_id`, bot)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	spaces := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		spaces = append(spaces, id)
	}
	return spaces, rows.Err()
}

func (s *BotStore) BeginBotDeletion(ctx context.Context, botID, owner uuid.UUID) (*BotDeletion, error) {
	return s.beginBotDeletion(ctx, botID, owner, nil)
}

func (s *BotStore) BeginBotDeletionWithAdmission(ctx context.Context, botID, owner uuid.UUID, admission DeletionAdmission) (*BotDeletion, error) {
	if admission.AccountID != owner || admission.ProfileID == uuid.Nil || admission.SessionEpoch <= 0 || admission.AccountType == "" || admission.Method != "/voice.bot.v1.BotService/DeleteBot" || admission.RequestID == "" || len(admission.RequestBytes) == 0 || admission.RequestHash == "" {
		return nil, ErrDeletionOwner
	}
	return s.beginBotDeletion(ctx, botID, owner, &admission)
}

func (s *BotStore) beginBotDeletion(ctx context.Context, botID, owner uuid.UUID, admission *DeletionAdmission) (*BotDeletion, error) {
	if botID == uuid.Nil || owner == uuid.Nil {
		return nil, ErrDeletionOwner
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		spaces, err := deletionSpaces(ctx, s.Pool, botID)
		if err != nil {
			return nil, err
		}
		result, restart, err := s.beginBotDeletionAttempt(ctx, botID, owner, spaces, admission)
		if err != nil || !restart {
			return result, err
		}
	}
}

func (s *BotStore) beginBotDeletionAttempt(ctx context.Context, botID, owner uuid.UUID, spaces []uuid.UUID, admission *DeletionAdmission) (*BotDeletion, bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Acquire the complete ordered Space set before Bot. A newly discovered set
	// is retried from scratch; no additional Space lock is taken below Bot.
	for _, space := range spaces {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,71431))`, space.String()); err != nil {
			return nil, false, err
		}
	}
	bot, err := s.GetBotByIDForUpdate(ctx, tx, botID)
	if err != nil {
		return nil, false, err
	}
	if bot.OwnerAccountID != owner {
		return nil, false, ErrDeletionOwner
	}
	prior, err := scanBotDeletion(tx.QueryRow(ctx, `SELECT `+deletionColumns+` FROM bot_deletion_operations WHERE bot_id=$1 FOR UPDATE`, botID))
	if err == nil {
		return prior, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	actual, err := deletionSpaces(ctx, tx, botID)
	if err != nil {
		return nil, false, err
	}
	if !slices.Equal(spaces, actual) {
		return nil, true, nil
	}
	if admission != nil && (!time.Now().Before(admission.ExpiresAt) || ctx.Err() != nil) {
		return nil, false, ErrDeletionOwner
	}
	op := &BotDeletion{OperationID: uuid.New(), BotID: botID, OwnerAccountID: owner, ActorProfileID: bot.ActorProfileID, State: "PENDING"}
	// The service signs this persisted owner decision later. It contains no user
	// bearer token, and is never reconstructed from mutable/deleted source rows.
	decision := struct {
		OperationID, BotID, OwnerAccountID, ActorProfileID uuid.UUID
		Purpose                                            string
		Admission                                          *DeletionAdmission `json:",omitempty"`
	}{op.OperationID, botID, owner, bot.ActorProfileID, "global_delete", admission}
	op.DecisionBytes, err = json.Marshal(decision)
	if err != nil {
		return nil, false, err
	}
	digest := sha256.Sum256(op.DecisionBytes)
	op.DecisionSHA256 = digest[:]
	if len(spaces) == 0 {
		op.State = "COMPLETE"
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_deletion_operations(operation_id,bot_id,owner_account_id,actor_profile_id,decision_bytes,decision_sha256,state,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,CASE WHEN $7='COMPLETE' THEN now() ELSE NULL END)`, op.OperationID, op.BotID, owner, op.ActorProfileID, op.DecisionBytes, op.DecisionSHA256, op.State)
	if err != nil {
		return nil, false, err
	}
	for _, space := range spaces {
		target, err := s.snapshotDeletionTarget(ctx, tx, op, space)
		if err != nil {
			return nil, false, err
		}
		if err = saveDeletionTarget(ctx, tx, target); err != nil {
			return nil, false, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE bots SET status='disabled',updated_at=now() WHERE id=$1`, botID); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_message_deliveries SET status='canceled',claimed_until=NULL,payload='{}'::jsonb WHERE bot_id=$1 AND status='pending'`, botID); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_event_log SET delivery_status='canceled',claimed_until=NULL,payload='{}'::jsonb WHERE bot_id=$1 AND delivery_status IN ('pending','deferred')`, botID); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM bot_chat_whitelist WHERE bot_id=$1`, botID); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM bot_space_installations WHERE bot_id=$1`, botID); err != nil {
		return nil, false, err
	}
	return op, false, tx.Commit(ctx)
}

func (s *BotStore) snapshotDeletionTarget(ctx context.Context, tx pgx.Tx, op *BotDeletion, space uuid.UUID) (*BotDeletionTarget, error) {
	t := &BotDeletionTarget{TargetID: uuid.New(), OperationID: op.OperationID, BotID: op.BotID, SpaceID: space, ActorProfileID: op.ActorProfileID, CancelOperationID: uuid.New(), Chats: []uuid.UUID{}, LegacyEvidence: true}
	i, err := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE bot_id=$1 AND space_id=$2 AND state<>'CANCELED' FOR UPDATE`, op.BotID, space))
	if err == nil {
		t.InstallationID = i.InstallationID
		t.InstallOperationID = i.OperationID
		t.BotGeneration = i.Generation
		t.ActorProfileID = i.ActorProfileID
		t.Chats = append(t.Chats, i.Chats...)
		t.LegacyEvidence = i.LegacyEvidence
		if i.CancelOperationID != uuid.Nil {
			t.CancelOperationID = i.CancelOperationID
		}
		if _, err = tx.Exec(ctx, `UPDATE bot_installation_intents SET state='CANCELING',cancel_operation_id=$2,updated_at=now() WHERE installation_id=$1`, i.InstallationID, t.CancelOperationID); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var actual uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM bot_space_installations WHERE bot_id=$1 AND space_id=$2 FOR UPDATE`, op.BotID, space).Scan(&actual)
	if err == nil {
		if t.InstallationID != uuid.Nil && actual != t.InstallationID {
			return nil, ErrInstallationConflict
		}
		t.InstallationID = actual
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT chat_id FROM bot_chat_whitelist WHERE bot_id=$1 AND space_id=$2 ORDER BY chat_id FOR UPDATE`, op.BotID, space)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var chat uuid.UUID
		if err = rows.Scan(&chat); err != nil {
			rows.Close()
			return nil, err
		}
		t.Chats = append(t.Chats, chat)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	t.Chats = normalizedInstallationChats(t.Chats)
	t.ScopeBytes, err = json.Marshal(t)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(t.ScopeBytes)
	t.ScopeSHA256 = digest[:]
	return t, nil
}

func saveDeletionTarget(ctx context.Context, tx pgx.Tx, t *BotDeletionTarget) error {
	chats, err := json.Marshal(t.Chats)
	if err != nil {
		return err
	}
	var installation, installOperation *uuid.UUID
	if t.InstallationID != uuid.Nil {
		installation = &t.InstallationID
	}
	if t.InstallOperationID != uuid.Nil {
		installOperation = &t.InstallOperationID
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_deletion_installations(target_id,operation_id,bot_id,space_id,actor_profile_id,installation_id,install_operation_id,bot_generation,cancel_operation_id,chat_ids,legacy_evidence,scope_bytes,scope_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12,$13)`, t.TargetID, t.OperationID, t.BotID, t.SpaceID, t.ActorProfileID, installation, installOperation, t.BotGeneration, t.CancelOperationID, chats, t.LegacyEvidence, t.ScopeBytes, t.ScopeSHA256)
	if err != nil {
		return err
	}
	for _, kind := range []string{"SPACE_CLEANUP", "ROLE_REMOVE"} {
		if _, err = tx.Exec(ctx, `INSERT INTO bot_deletion_effects(effect_id,target_id,kind) VALUES($1,$2,$3)`, uuid.New(), t.TargetID, kind); err != nil {
			return err
		}
	}
	for _, chat := range t.Chats {
		if _, err = tx.Exec(ctx, `INSERT INTO bot_deletion_effects(effect_id,target_id,kind,chat_id) VALUES($1,$2,'MESSAGING_UNPIN',$3)`, uuid.New(), t.TargetID, chat); err != nil {
			return err
		}
	}
	return nil
}
