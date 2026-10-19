package automod

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
	"voice/backend/pkg/automodclient"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

var ErrRestrictionAck = errors.New("invalid exact Auth restriction acknowledgement")
var ErrRestoreDivergence = errors.New("Auth restriction authority is ahead of current Moderation history")

type RestrictionReconciler interface {
	Reconcile(context.Context, uuid.UUID, uuid.UUID, uint64, bool) (*automodv1.ReconcileModerationRestrictionResponse, error)
}

// SyncRestrictions processes one finite queue view. Remote calls never run
// under an account transaction. The caller retries durable intent on failure.
func (s *Store) SyncRestrictions(ctx context.Context, gate *BootGate, auth RestrictionReconciler) error {
	check, err := gate.AcquirePrivateRepair(ctx)
	if err != nil {
		return err
	}
	if auth == nil {
		return ErrRestrictionAck
	}
	pending, err := s.PendingRestrictions(ctx)
	if err != nil {
		return err
	}
	for _, r := range pending {
		if err = check(ctx); err != nil {
			return err
		}
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		ack, callErr := auth.Reconcile(callCtx, r.AccountID, r.SubjectID, r.Revision, r.Banned)
		cancel()
		if err = check(ctx); err != nil {
			return err
		}
		if callErr != nil {
			var ahead *automodclient.RestrictionAheadError
			if !errors.As(callErr, &ahead) {
				return callErr
			}
			if ahead.Requested != r.Revision || ahead.Accepted <= r.Revision {
				return ErrRestrictionAck
			}
			if err = s.checkRestrictionAhead(ctx, r, ahead.Accepted, gate, check); errors.Is(err, ErrPurged) {
				continue
			}
			if err != nil {
				return err
			}
			continue // Keep the newer local intent; no ACK for this stale request.
		}
		if ack == nil || len(ack.ProtoReflect().GetUnknown()) != 0 || ack.GetProtocolVersion() != 1 || ack.GetAccountId() != r.AccountID.String() || ack.GetSubjectId() != r.SubjectID.String() || ack.GetRevision() != r.Revision || ack.GetModerationBanned() != r.Banned || (ack.GetState() != automodv1.SubjectState_SUBJECT_STATE_LIVE && ack.GetState() != automodv1.SubjectState_SUBJECT_STATE_DELETED) {
			return ErrRestrictionAck
		}
		if err = s.CompleteRestrictionChecked(ctx, r, check); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) checkRestrictionAhead(ctx context.Context, r Restriction, accepted uint64, gate *BootGate, check BootCheck) error {
	return s.accountTxChecked(ctx, r.AccountID, check, func(tx pgx.Tx) error {
		var revision uint64
		var subject uuid.UUID
		err := tx.QueryRow(ctx, `SELECT restriction_revision,subject_id FROM automod_subjects WHERE account_id=$1`, r.AccountID).Scan(&revision, &subject)
		if errors.Is(err, pgx.ErrNoRows) {
			var terminal bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, r.SubjectID).Scan(&terminal); err != nil {
				return err
			}
			if terminal {
				return ErrPurged
			}
			return ErrSubject
		}
		if err != nil {
			return err
		}
		if subject != r.SubjectID {
			return ErrConflict
		}
		if accepted > revision {
			// The lock makes this a comparison with CURRENT durable authority.
			// Never advance a local revision to overwrite a restored old payload.
			gate.Close()
			return ErrRestoreDivergence
		}
		return nil
	})
}
