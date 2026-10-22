package store

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MemberAdmissionRetirementReason selects an authoritative Space DB predicate.
// A caller's assertion or an unavailable remote dependency is never a reason.
type MemberAdmissionRetirementReason uint8

const (
	MemberAdmissionRetireInviteInvalid MemberAdmissionRetirementReason = iota + 1
	MemberAdmissionRetirePolicyChanged
	MemberAdmissionRetireAccountBanned
)

// LookupMemberAdmission returns saved attempt identity/outcome only. It never
// refreshes eligibility, publishes a member or attaches a target redemption.
func (s *SpaceStore) LookupMemberAdmission(ctx context.Context, operation uuid.UUID) (*MemberAdmissionIntent, error) {
	if s == nil || s.Pool == nil || operation == uuid.Nil {
		return nil, ErrMemberAdmissionConflict
	}
	i, _, err := scanMemberAdmission(s.db().QueryRow(ctx, `SELECT `+memberAdmissionColumns+` FROM space_member_admission_intents WHERE operation_id=$1`, operation))
	return i, err
}

// FindCurrent returns the exact PREPARING obstruction in one DB statement.
// Absence does not authorize publication; Prepare still serializes the head.
func (s *SpaceStore) FindCurrent(ctx context.Context, space, profile uuid.UUID) (*MemberAdmissionIntent, error) {
	if s == nil || s.Pool == nil || space == uuid.Nil || profile == uuid.Nil {
		return nil, ErrMemberAdmissionConflict
	}
	i, _, err := scanMemberAdmission(s.db().QueryRow(ctx, `SELECT `+memberAdmissionColumns+` FROM space_member_admission_intents i
	 WHERE i.space_id=$1 AND i.profile_id=$2 AND i.state='PREPARING' AND EXISTS(
	 SELECT 1 FROM space_ordinary_member_heads h WHERE h.space_id=i.space_id AND h.profile_id=i.profile_id
	 AND h.generation=i.generation AND h.publication_id=i.operation_id AND h.state='PREPARING')`, space, profile))
	return i, err
}

// RetireCandidate proves a known terminal invalidation locally, then retires
// only the named PREPARING generation and queues its Role removal atomically.
// It intentionally uses the Space fence without requiring LIVE availability:
// cancelling a candidate must remain possible while other mutations are fenced.
func (s *SpaceStore) RetireCandidate(ctx context.Context, operation uuid.UUID, expectedGeneration uint64, reason MemberAdmissionRetirementReason) (*MemberAdmissionIntent, error) {
	if s == nil || s.Pool == nil || s.tx != nil || operation == uuid.Nil || expectedGeneration == 0 || expectedGeneration >= math.MaxInt64 || reason < MemberAdmissionRetireInviteInvalid || reason > MemberAdmissionRetireAccountBanned {
		return nil, ErrMemberAdmissionConflict
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockMemberAdmissionOperation(ctx, tx, operation); err != nil {
		return nil, err
	}
	i, _, err := scanMemberAdmission(tx.QueryRow(ctx, `SELECT `+memberAdmissionColumns+` FROM space_member_admission_intents WHERE operation_id=$1`, operation))
	if err != nil {
		return nil, err
	}
	if i.Generation != expectedGeneration {
		return nil, ErrMemberAdmissionStale
	}
	if i.State == "RETIRED" {
		return i, tx.Commit(ctx)
	}
	if i.State != "PREPARING" {
		return nil, ErrMemberAdmissionStale
	}
	if err = lockLifecycleSpace(ctx, tx, i.Input.SpaceID); err != nil {
		return nil, err
	}
	scoped := *s
	scoped.tx = tx
	if err = scoped.memberAdmissionRetirementScope(ctx, i.Input.SpaceID); err != nil {
		return nil, err
	}
	proven, err := scoped.proveMemberAdmissionRetirement(ctx, i, reason)
	if err != nil {
		return nil, err
	}
	if !proven {
		return nil, ErrMemberAdmissionConflict
	}
	capture, err := scoped.captureMemberAdmissionRetirement(ctx, i)
	if err != nil {
		return nil, err
	}
	if err = scoped.invalidateMemberAdmission(ctx, i); err != nil {
		return nil, err
	}
	if capture != nil {
		if err = scoped.finishEffectiveMemberMutations(ctx, capture); err != nil {
			return nil, err
		}
	}
	i.State = "RETIRED"
	return i, tx.Commit(ctx)
}

func (s *SpaceStore) proveMemberAdmissionRetirement(ctx context.Context, i *MemberAdmissionIntent, reason MemberAdmissionRetirementReason) (bool, error) {
	switch reason {
	case MemberAdmissionRetireAccountBanned:
		return s.IsAccountBanned(ctx, i.Input.SpaceID, i.Input.AccountID)
	case MemberAdmissionRetirePolicyChanged:
		policy, err := s.GetEntryPolicy(ctx, i.Input.SpaceID)
		return err == nil && policy.Version != i.PolicyVersion, err
	case MemberAdmissionRetireInviteInvalid:
		if i.Input.InviteID == nil {
			return false, nil
		}
		invite, err := s.GetInviteByID(ctx, *i.Input.InviteID)
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if invite == nil {
			return true, nil
		}
		if invite.SpaceID != i.Input.SpaceID {
			return false, ErrMemberAdmissionConflict
		}
		var now time.Time
		if err = s.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return false, err
		}
		err = s.validateInviteForJoin(invite, now)
		return errors.Is(err, ErrInviteRevoked) || errors.Is(err, ErrInviteExpired) || errors.Is(err, ErrInviteMaxUses), nil
	default:
		return false, ErrMemberAdmissionConflict
	}
}

// retirePendingMemberAdmission runs inside an already held Space mutation
// fence. Administrative removal is itself the proven invalidation; there need
// not yet be a space_members row or a successful Role preparation.
func (s *SpaceStore) retirePendingMemberAdmission(ctx context.Context, space, profile uuid.UUID) (bool, error) {
	if s.tx == nil {
		return false, ErrMemberAdmissionConflict
	}
	i, err := s.FindCurrent(ctx, space, profile)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = s.invalidateMemberAdmission(ctx, i); err != nil {
		return false, err
	}
	return true, nil
}
