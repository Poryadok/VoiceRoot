package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
	"voice/backend/messaging/internal/messageevents"
)

var ErrScheduledState = errors.New("scheduled message is no longer pending")

type ScheduledRecord struct {
	ScheduledMessageRow
	RequestRPC        string
	RequestHash       string
	Revision          int64
	DispatchMessageID uuid.UUID
}
type ScheduledPosition struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

const scheduledColumns = `id,chat_id,sender_profile_id,client_message_id,payload::text,schedule_kind,scheduled_at,status,sent_message_id,dispatch_event_id,created_at,updated_at,request_rpc,request_sha256,revision,dispatch_message_id`

func scanScheduled(row pgx.Row) (*ScheduledRecord, error) {
	var r ScheduledRecord
	var dispatch *uuid.UUID
	err := row.Scan(&r.ID, &r.ChatID, &r.SenderProfileID, &r.ClientMessageID, &r.PayloadJSON, &r.ScheduleKind, &r.ScheduledAt, &r.Status, &r.SentMessageID, &r.DispatchEventID, &r.CreatedAt, &r.UpdatedAt, &r.RequestRPC, &r.RequestHash, &r.Revision, &dispatch)
	if dispatch != nil {
		r.DispatchMessageID = *dispatch
	}
	return &r, err
}
func IsSendNamespaceConflict(err error) bool {
	var pg *pgconn.PgError
	return errors.Is(err, ErrSendReceiptConflict) || (errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "message_send_namespace_kind")
}
func (s *MessagesStore) GetOwnedScheduled(ctx context.Context, id, owner uuid.UUID) (*ScheduledRecord, error) {
	return scanScheduled(s.Pool.QueryRow(ctx, `SELECT `+scheduledColumns+` FROM scheduled_messages WHERE id=$1 AND sender_profile_id=$2`, id, owner))
}
func (s *MessagesStore) GetScheduledReceipt(ctx context.Context, chat, owner, key uuid.UUID, rpc, hash string) (*ScheduledRecord, error) {
	r, err := scanScheduled(s.Pool.QueryRow(ctx, `SELECT `+scheduledColumns+` FROM scheduled_messages WHERE chat_id=$1 AND sender_profile_id=$2 AND client_message_id=$3`, chat, owner, key))
	if err == nil && (r.RequestRPC != rpc || r.RequestHash != hash) {
		return nil, ErrSendReceiptConflict
	}
	return r, err
}
func (s *MessagesStore) CreateScheduled(ctx context.Context, r ScheduledRecord, scope ...*uuid.UUID) (*ScheduledRecord, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Serialize against chat/Space purge just like ordinary message mutations.
	var space *uuid.UUID
	if len(scope) > 0 {
		space = scope[0]
	}
	if err = lockMessageMutation(ctx, tx, r.ChatID, space, nil); err != nil {
		return nil, err
	}
	var dispatch any
	if r.DispatchMessageID != uuid.Nil {
		dispatch = r.DispatchMessageID
	}
	_, err = tx.Exec(ctx, `INSERT INTO scheduled_messages(id,chat_id,sender_profile_id,client_message_id,payload,schedule_kind,scheduled_at,request_rpc,request_sha256,dispatch_message_id) VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10) ON CONFLICT(chat_id,sender_profile_id,client_message_id) WHERE client_message_id IS NOT NULL DO NOTHING`, r.ID, r.ChatID, r.SenderProfileID, r.ClientMessageID, r.PayloadJSON, r.ScheduleKind, r.ScheduledAt, r.RequestRPC, r.RequestHash, dispatch)
	if err != nil {
		if IsSendNamespaceConflict(err) {
			return nil, ErrSendReceiptConflict
		}
		return nil, err
	}
	var saved *ScheduledRecord
	if r.ClientMessageID == nil {
		saved, err = scanScheduled(tx.QueryRow(ctx, `SELECT `+scheduledColumns+` FROM scheduled_messages WHERE id=$1`, r.ID))
	} else {
		saved, err = scanScheduled(tx.QueryRow(ctx, `SELECT `+scheduledColumns+` FROM scheduled_messages WHERE chat_id=$1 AND sender_profile_id=$2 AND client_message_id=$3`, r.ChatID, r.SenderProfileID, *r.ClientMessageID))
	}
	if err != nil {
		return nil, err
	}
	if saved.RequestRPC != r.RequestRPC || saved.RequestHash != r.RequestHash {
		return nil, ErrSendReceiptConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return saved, nil
}
func (s *MessagesStore) ListPendingScheduled(ctx context.Context, chat, owner uuid.UUID, after *ScheduledPosition, limit int) ([]ScheduledRecord, error) {
	if limit < 1 || limit > 101 {
		return nil, errors.New("invalid scheduled page size")
	}
	var at any
	var id any
	if after != nil {
		at = after.CreatedAt
		id = after.ID
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+scheduledColumns+` FROM scheduled_messages WHERE chat_id=$1 AND sender_profile_id=$2 AND status='pending' AND ($3::timestamptz IS NULL OR (created_at,id)>($3,$4::uuid)) ORDER BY created_at,id LIMIT $5`, chat, owner, at, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ScheduledRecord, 0)
	for rows.Next() {
		r, err := scanScheduled(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}
func (s *MessagesStore) UpdateScheduled(ctx context.Context, r ScheduledRecord) (*ScheduledRecord, error) {
	saved, err := scanScheduled(s.Pool.QueryRow(ctx, `UPDATE scheduled_messages SET payload=$1::jsonb,schedule_kind=$2,scheduled_at=$3,dispatch_message_id=NULL,revision=revision+1,updated_at=clock_timestamp() WHERE id=$4 AND sender_profile_id=$5 AND status='pending' AND revision=$6 AND (dispatch_lease_expires_at IS NULL OR dispatch_lease_expires_at<=clock_timestamp()) RETURNING `+scheduledColumns, r.PayloadJSON, r.ScheduleKind, r.ScheduledAt, r.ID, r.SenderProfileID, r.Revision))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrScheduledState
	}
	return saved, err
}
func (s *MessagesStore) CancelScheduled(ctx context.Context, id, owner uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var state ScheduledMessageStatus
	var leased bool
	err = tx.QueryRow(ctx, `SELECT status,COALESCE(dispatch_lease_expires_at>clock_timestamp(),false) FROM scheduled_messages WHERE id=$1 AND sender_profile_id=$2 FOR UPDATE`, id, owner).Scan(&state, &leased)
	if err != nil {
		return err
	}
	if state == ScheduledMessageCancelled {
		return nil
	}
	if state != ScheduledMessagePending || leased {
		return ErrScheduledState
	}
	_, err = tx.Exec(ctx, `UPDATE scheduled_messages SET status='cancelled',revision=revision+1,updated_at=clock_timestamp() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type scheduledDispatchKey struct{}

// ReserveScheduledDispatch allocates UUIDv7 when dispatch begins. Concurrent
// dispatchers obtain the same durable identity.
func (s *MessagesStore) ReserveScheduledDispatch(ctx context.Context, r ScheduledRecord, id uuid.UUID) (*ScheduledRecord, error) {
	saved, err := scanScheduled(s.Pool.QueryRow(ctx, `UPDATE scheduled_messages SET dispatch_message_id=COALESCE(dispatch_message_id,$1) WHERE id=$2 AND sender_profile_id=$3 AND status='pending' AND revision=$4 AND (dispatch_lease_expires_at IS NULL OR dispatch_lease_expires_at<=clock_timestamp()) RETURNING `+scheduledColumns, id, r.ID, r.SenderProfileID, r.Revision))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrScheduledState
	}
	return saved, err
}

type scheduledDispatch struct {
	ID, Owner, MessageID uuid.UUID
	Revision             int64
	ScheduledAt          *time.Time
}

func WithScheduledDispatch(ctx context.Context, r ScheduledRecord) context.Context {
	return context.WithValue(ctx, scheduledDispatchKey{}, scheduledDispatch{r.ID, r.SenderProfileID, r.DispatchMessageID, r.Revision, r.ScheduledAt})
}

func ScheduledDispatchTime(ctx context.Context) (*time.Time, bool) {
	d, ok := ctx.Value(scheduledDispatchKey{}).(scheduledDispatch)
	return d.ScheduledAt, ok
}
func ScheduledDispatchMessageID(ctx context.Context) (uuid.UUID, bool) {
	d, ok := ctx.Value(scheduledDispatchKey{}).(scheduledDispatch)
	return d.MessageID, ok
}

func validateScheduledDispatch(ctx context.Context, tx pgx.Tx) error {
	d, ok := ctx.Value(scheduledDispatchKey{}).(scheduledDispatch)
	if !ok {
		return nil
	}
	var valid bool
	err := tx.QueryRow(ctx, `SELECT status='pending' AND revision=$3 AND dispatch_message_id=$4 AND (dispatch_lease_expires_at IS NULL OR dispatch_lease_expires_at<=clock_timestamp()) FROM scheduled_messages WHERE id=$1 AND sender_profile_id=$2 FOR UPDATE`, d.ID, d.Owner, d.Revision, d.MessageID).Scan(&valid)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !valid {
		return ErrScheduledState
	}
	return err
}

// Called in BOTH ordinary and attachment commit transactions. A losing stale
// dispatch rolls back its message, acquired-reference intent and outbox writes.
func completeScheduledDispatch(ctx context.Context, tx pgx.Tx, row *MessageRow, events []messageevents.OutboxEvent) error {
	d, ok := ctx.Value(scheduledDispatchKey{}).(scheduledDispatch)
	if !ok {
		return nil
	}
	if row.ID != d.MessageID || row.SenderProfileID != d.Owner || row.ClientMessageID != nil {
		return ErrScheduledState
	}
	eventID := uuid.NewSHA1(d.ID, []byte("voice.scheduled.shadow-dispatch.v1"))
	for _, event := range events {
		if event.Subject == "message.sent" {
			eventID = event.EventID
			break
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE scheduled_messages SET status='sent',sent_message_id=$1,dispatch_event_id=$2,revision=revision+1,updated_at=clock_timestamp(),dispatch_lease_owner=NULL,dispatch_lease_expires_at=NULL WHERE id=$3 AND sender_profile_id=$4 AND chat_id=$5 AND status='pending' AND revision=$6 AND dispatch_message_id=$1 AND (dispatch_lease_expires_at IS NULL OR dispatch_lease_expires_at<=clock_timestamp())`, row.ID, eventID, d.ID, d.Owner, row.ChatID, d.Revision)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrScheduledState
	}
	return nil
}
