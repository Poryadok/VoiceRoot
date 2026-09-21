package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "voice.app/voice/user/v1"
)

// ApplyAccountDeleted atomically records Auth's durable event, overlays the
// account as inactive, and appends a higher-revision delete for every profile,
// including rows that were already soft deleted. A duplicate event is a no-op.
func (s *ProfileStore) ApplyAccountDeleted(ctx context.Context, eventID, accountID uuid.UUID, occurredAt time.Time) error {
	if s == nil || s.pool == nil || eventID == uuid.Nil || accountID == uuid.Nil {
		return fmt.Errorf("invalid account deletion event")
	}
	return s.withSearchProjectionTx(ctx, func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `INSERT INTO user_account_lifecycle_inbox(event_id, account_id) VALUES ($1,$2) ON CONFLICT (event_id) DO NOTHING`, eventID, accountID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_account_lifecycle(account_id,state,source_event_id,occurred_at) VALUES ($1,'ACCOUNT_INACTIVE',$2,$3) ON CONFLICT (account_id) DO NOTHING`, accountID, eventID, occurredAt.UTC()); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `UPDATE profiles SET search_projection_revision=search_projection_revision+1, updated_at=now() WHERE account_id=$1 RETURNING `+profileSelectCols, accountID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			profile, err := scanProfile(rows)
			if err != nil {
				return err
			}
			childID := uuid.NewSHA1(eventID, []byte(profile.ID.String()))
			if err := AppendSearchProjection(ctx, tx, &userv1.SearchProfileProjectionEvent{
				ProtocolVersion: 1, EventId: childID.String(), OccurredAt: timestamppb.New(occurredAt.UTC()), ProfileId: profile.ID.String(), SourceRevision: profile.SearchProjectionRevision,
				Payload: &userv1.SearchProfileProjectionEvent_Delete{Delete: &userv1.SearchProfileDelete{}},
			}); err != nil {
				return err
			}
		}
		return rows.Err()
	})
}

func accountInactive(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_account_lifecycle WHERE account_id=$1 AND state='ACCOUNT_INACTIVE')`, accountID).Scan(&exists)
	return exists, err
}

// searchProjectionForProfile preserves the tombstone fence for mutations that
// race account deletion. It must be called in the writer's transaction.
func searchProjectionForProfile(ctx context.Context, tx pgx.Tx, profile *ProfileRow) (*userv1.SearchProfileProjectionEvent, error) {
	inactive, err := accountInactive(ctx, tx, profile.AccountID)
	if err != nil {
		return nil, err
	}
	if !inactive && profile.DeletedAt == nil {
		return searchProjectionUpsert(profile), nil
	}
	return &userv1.SearchProfileProjectionEvent{ProtocolVersion: 1, EventId: uuid.NewString(), OccurredAt: timestamppb.Now(), ProfileId: profile.ID.String(), SourceRevision: profile.SearchProjectionRevision, Payload: &userv1.SearchProfileProjectionEvent_Delete{Delete: &userv1.SearchProfileDelete{}}}, nil
}
