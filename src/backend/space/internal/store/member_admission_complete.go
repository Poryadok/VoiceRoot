package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/memberadmissionrole"
)

// CompleteInteractiveMemberAdmission coordinates inert Role preparation and
// fresh owner checks outside locks, then enters the sole local publisher.
func (s *SpaceStore) CompleteInteractiveMemberAdmission(ctx context.Context, operation uuid.UUID, roles *memberadmissionrole.Client, subjects *admissionauthority.Client, reviewers *admissionauthority.ReviewerClient, completion MemberAdmissionFinalGuard) (*MembershipRow, error) {
	return s.CompleteInteractiveMemberAdmissionWithPublicationHook(ctx, operation, roles, subjects, reviewers, completion, nil)
}

// CompleteInteractiveMemberAdmissionWithPublicationHook preserves the pre-write
// final guard and adds a distinct local FIRST-publication journal phase. Saved
// completed outcomes return before Role, owner calls or either callback.
func (s *SpaceStore) CompleteInteractiveMemberAdmissionWithPublicationHook(ctx context.Context, operation uuid.UUID, roles *memberadmissionrole.Client, subjects *admissionauthority.Client, reviewers *admissionauthority.ReviewerClient, completion MemberAdmissionFinalGuard, publicationHook MemberAdmissionPublicationHook) (*MembershipRow, error) {
	return s.completeInteractiveMemberAdmission(ctx, operation, roles, subjects, reviewers, completion, publicationHook, nil)
}

func (s *SpaceStore) completeInteractiveMemberAdmission(ctx context.Context, operation uuid.UUID, roles *memberadmissionrole.Client, subjects *admissionauthority.Client, reviewers *admissionauthority.ReviewerClient, completion MemberAdmissionFinalGuard, publicationHook MemberAdmissionPublicationHook, initiating *memberAdmissionInitiatingCall) (*MembershipRow, error) {
	if s == nil || s.Pool == nil || s.tx != nil || operation == uuid.Nil {
		return nil, ErrJoinAttemptUnavailable
	}
	i, err := s.LookupMemberAdmission(ctx, operation)
	if err != nil {
		return nil, err
	}
	if i.State == "PUBLISHED" {
		member, completed, err := s.replayCompletedMemberAdmission(ctx, operation)
		if err == nil && !completed {
			return nil, ErrJoinAttemptUnavailable // Concurrent terminal erasure is not success.
		}
		return member, err
	}
	if i.State != "PREPARING" {
		return nil, ErrMemberAdmissionStale
	}
	if initiating != nil {
		if err = initiating.check(ctx); err != nil {
			return nil, err
		}
	}
	if i.Input.JoinAttempt == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	binding := i.EnsureBinding()
	prepared, err := roles.Ensure(ctx, binding)
	if err != nil {
		return s.memberAdmissionCompletionUncertain(ctx, operation)
	}
	role, receipt, err := prepared.For(binding)
	if err != nil {
		return s.memberAdmissionCompletionUncertain(ctx, operation)
	}
	continuation, err := s.ObserveMemberAdmissionContinuation(ctx, operation, subjects, reviewers)
	if err != nil {
		return s.memberAdmissionCompletionUncertain(ctx, operation)
	}
	return s.publishMemberAdmissionWithInitiatingCall(ctx, operation, role, receipt, completion, continuation, publicationHook, initiating)
}

func (s *SpaceStore) memberAdmissionCompletionUncertain(ctx context.Context, operation uuid.UUID) (*MembershipRow, error) {
	if member, completed, err := s.replayCompletedMemberAdmission(ctx, operation); completed || err != nil {
		return member, err
	}
	return nil, ErrJoinAttemptUnavailable
}

// ReadMemberAdmissionOutcome reads only a saved publication outcome. It never
// starts admission, calls an owner or runs a completion hook. A true result is
// historical operation data, including after leave; it is not current access.
// False means no retained published outcome, including erased/unknown history.
// The handler must independently authorize and bind its exact A/P/actor tuple.
func (s *SpaceStore) ReadMemberAdmissionOutcome(ctx context.Context, operation uuid.UUID) (*MembershipRow, bool, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || operation == uuid.Nil {
		return nil, false, ErrJoinAttemptUnavailable
	}
	return s.replayCompletedMemberAdmission(ctx, operation)
}

func (s *SpaceStore) replayCompletedMemberAdmission(ctx context.Context, operation uuid.UUID) (*MembershipRow, bool, error) {
	var role uuid.UUID
	var receipt []byte
	err := s.db().QueryRow(ctx, `SELECT role_id,role_receipt_sha256 FROM space_member_admission_intents WHERE operation_id=$1 AND state='PUBLISHED'`, operation).Scan(&role, &receipt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	member, err := s.PublishMemberAdmissionWithContinuation(ctx, operation, role, receipt, nil, nil)
	return member, true, err
}
