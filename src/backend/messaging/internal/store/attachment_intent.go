package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/spacemutationlock"
)

var ErrAttachmentIntentConflict = errors.New("attachment send intent conflicts with saved request")
var ErrAttachmentIntentExpired = errors.New("attachment send intent expired")

// One session lock serializes send/recovery across the owner RPC and durable
// message commit. No transaction or row lock is held across the network call.
func attachmentIntentLock(ctx context.Context, pool *pgxpool.Pool, op uuid.UUID) (*pgxpool.Conn, func(), error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}
	hash := sha256.Sum256(append([]byte("voice.messaging.attachment-intent.v1\x00"), op[:]...))
	key := int64(binary.BigEndian.Uint64(hash[:8]))
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock($1::bigint)`, key); err != nil {
		conn.Release()
		return nil, nil, err
	}
	return conn, func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		err := conn.QueryRow(unlockCtx, `SELECT pg_advisory_unlock($1::bigint)`, key).Scan(&unlocked)
		if err != nil || !unlocked {
			_ = conn.Hijack().Close(unlockCtx)
		} else {
			conn.Release()
		}
	}, nil
}

func attachmentIntentOperation(row MessageRow) uuid.UUID {
	if row.ClientMessageID == nil {
		return row.ID
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("voice.messaging.attachment-send.v1\x00"+row.ChatID.String()+"\x00"+row.SenderProfileID.String()+"\x00"+row.ClientMessageID.String()))
}

func (s *MessagesStore) InsertMessageWithReferences(ctx context.Context, row MessageRow, space *uuid.UUID, request *filev1.AcquireFileReferencesRequest, acquire func(context.Context, *filev1.AcquireFileReferencesRequest) error) (*MessageRow, error) {
	if s == nil || s.Pool == nil || request == nil || len(request.References) == 0 || acquire == nil {
		return nil, errors.New("attachment reference dependencies unavailable")
	}
	op := attachmentIntentOperation(row)
	conn, unlock, err := attachmentIntentLock(ctx, s.Pool, op)
	if err != nil {
		return nil, err
	}
	defer unlock()
	canonical := row
	canonical.ID = uuid.Nil
	identityBytes, err := json.Marshal(struct {
		Row   MessageRow
		Space *uuid.UUID
	}{canonical, space})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(identityBytes)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if space != nil {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(*space)); err != nil {
			return nil, err
		}
	}
	var savedID uuid.UUID
	var savedHash, raw []byte
	var state string
	var expired bool
	err = tx.QueryRow(ctx, `SELECT message_id,request_sha256,references_bytes,state,expires_at<=clock_timestamp() FROM messaging_attachment_send_intents WHERE operation_id=$1`, op).Scan(&savedID, &savedHash, &raw, &state, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		// A pre-migration successful send already owns the client's dedupe key.
		// Preserve its original read-only retry; never acquire for a fresh UUID
		// which INSERT ON CONFLICT would discard in favour of that saved row.
		if row.ClientMessageID != nil {
			existing, loadErr := scanMessageRow(tx.QueryRow(ctx, messageSelectSQL+` FROM messages WHERE chat_id=$1 AND sender_profile_id=$2 AND client_message_id=$3`, row.ChatID, row.SenderProfileID, *row.ClientMessageID))
			if loadErr == nil {
				if err = tx.Commit(ctx); err != nil {
					return nil, err
				}
				return existing, nil
			}
			if !errors.Is(loadErr, pgx.ErrNoRows) {
				return nil, loadErr
			}
		}
		if space != nil {
			var blocked bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messaging_space_lifecycle_fences WHERE space_id=$1 AND state<>'LIVE')`, *space).Scan(&blocked); err != nil {
				return nil, err
			}
			if blocked {
				return nil, ErrSpaceLifecycleOrder
			}
		}
		savedID = row.ID
		bound := proto.Clone(request).(*filev1.AcquireFileReferencesRequest)
		bound.OperationId = uuid.NewSHA1(savedID, []byte("voice.message.file.acquire.v1")).String()
		for _, ref := range bound.References {
			ref.OwnerId = savedID.String()
			if space != nil {
				scope := space.String()
				ref.ScopeSpaceId = &scope
			}
		}
		raw, err = (proto.MarshalOptions{Deterministic: true}).Marshal(bound)
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO messaging_attachment_send_intents(operation_id,message_id,chat_id,scope_space_id,request_sha256,references_bytes) VALUES($1,$2,$3,$4,$5,$6)`, op, savedID, row.ChatID, space, hash[:], raw); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if !bytes.Equal(savedHash, hash[:]) {
			return nil, ErrAttachmentIntentConflict
		}
		if state == "RELEASED" {
			return nil, ErrAttachmentIntentExpired
		}
		if saved, loadErr := s.GetMessageByID(ctx, savedID); loadErr == nil {
			if _, err = tx.Exec(ctx, `UPDATE messaging_attachment_send_intents SET state='COMMITTED',completed_at=COALESCE(completed_at,clock_timestamp()) WHERE operation_id=$1`, op); err != nil {
				return nil, err
			}
			if err = tx.Commit(ctx); err != nil {
				return nil, err
			}
			return saved, nil
		} else if !errors.Is(loadErr, pgx.ErrNoRows) {
			return nil, loadErr
		}
		if expired {
			return nil, ErrAttachmentIntentExpired
		}
		if state == "COMMITTED" {
			return nil, ErrAttachmentIntentExpired
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	var bound filev1.AcquireFileReferencesRequest
	if err = proto.Unmarshal(raw, &bound); err != nil {
		return nil, err
	}
	if err = acquire(ctx, &bound); err != nil {
		return nil, err
	}
	row.ID = savedID
	commit, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = commit.Rollback(context.Background()) }()
	if space != nil {
		if _, err = commit.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(*space)); err != nil {
			return nil, err
		}
		var blocked bool
		if err = commit.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messaging_space_lifecycle_fences WHERE space_id=$1 AND state<>'LIVE')`, *space).Scan(&blocked); err != nil {
			return nil, err
		}
		if blocked {
			return nil, ErrSpaceLifecycleOrder
		}
	}
	saved, err := insertMessageDB(ctx, commit, row)
	if err != nil {
		return nil, err
	}
	if saved.ID != savedID {
		return nil, ErrAttachmentIntentConflict
	}
	if _, err = commit.Exec(ctx, `UPDATE messaging_attachment_send_intents SET state='COMMITTED',completed_at=clock_timestamp() WHERE operation_id=$1`, op); err != nil {
		return nil, err
	}
	if err = commit.Commit(ctx); err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *MessagesStore) ReconcileAttachmentIntents(ctx context.Context, release func(context.Context, *filev1.ReleaseFileReferencesRequest) error) error {
	if s == nil || s.Pool == nil || release == nil {
		return errors.New("attachment recovery dependencies unavailable")
	}
	rows, err := s.Pool.Query(ctx, `SELECT operation_id FROM messaging_attachment_send_intents WHERE state='PENDING' AND expires_at<=clock_timestamp() ORDER BY expires_at LIMIT 100`)
	if err != nil {
		return err
	}
	var ops []uuid.UUID
	for rows.Next() {
		var op uuid.UUID
		if err = rows.Scan(&op); err != nil {
			rows.Close()
			return err
		}
		ops = append(ops, op)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var failures []error
	for _, op := range ops {
		if err = s.reconcileAttachmentIntent(ctx, op, release); err != nil {
			failures = append(failures, err)
			if ctx.Err() != nil {
				break
			}
		}
	}
	return errors.Join(failures...)
}

func (s *MessagesStore) reconcileAttachmentIntent(ctx context.Context, op uuid.UUID, release func(context.Context, *filev1.ReleaseFileReferencesRequest) error) error {
	conn, unlock, err := attachmentIntentLock(ctx, s.Pool, op)
	if err != nil {
		return err
	}
	defer unlock()
	var id uuid.UUID
	var raw []byte
	var exists bool
	err = conn.QueryRow(ctx, `SELECT i.message_id,i.references_bytes,EXISTS(SELECT 1 FROM messages m WHERE m.id=i.message_id) FROM messaging_attachment_send_intents i WHERE operation_id=$1 AND state='PENDING' AND expires_at<=clock_timestamp()`, op).Scan(&id, &raw, &exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	state := "COMMITTED"
	if !exists {
		// A sealed Space producer, not the ordinary LIVE-only release, owns
		// pending refs while frozen. PURGED means that exact producer release
		// and every child have already completed under the irreversible fence.
		var frozen, purged bool
		if err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messaging_attachment_send_intents i JOIN messaging_space_lifecycle_fences f ON f.space_id=i.scope_space_id WHERE i.operation_id=$1 AND f.state<>'LIVE'),EXISTS(SELECT 1 FROM messaging_attachment_send_intents i JOIN messaging_space_lifecycle_fences f ON f.space_id=i.scope_space_id JOIN messaging_space_purge_receipts r ON r.space_id=f.space_id AND r.deletion_operation_id=f.deletion_operation_id WHERE i.operation_id=$1 AND f.state='PURGED')`, op).Scan(&frozen, &purged); err != nil {
			return err
		}
		if frozen && !purged {
			return nil
		}
		var acquired filev1.AcquireFileReferencesRequest
		if err = proto.Unmarshal(raw, &acquired); err != nil {
			return err
		}
		request := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewSHA1(id, []byte("voice.message.file.abandon.v1")).String(), ProducerId: acquired.ProducerId, References: acquired.References}
		if !purged {
			if err = release(ctx, request); err != nil {
				return err
			}
		}
		state = "RELEASED"
	}
	_, err = conn.Exec(ctx, `UPDATE messaging_attachment_send_intents SET state=$2,completed_at=clock_timestamp() WHERE operation_id=$1`, op, state)
	return err
}
