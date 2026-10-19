package store

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/entrypolicy"
)

type joinReviewGroupRead struct {
	summary  *spacev1.JoinRequestSummary
	attempts []*spacev1.JoinRequestAttempt
}

// A cursor is a position, never an authorization. Every page is authorized
// afresh. Its exact Space and tuple survive deletion of the boundary row.
func encodeJoinReviewCursor(space uuid.UUID, created time.Time, id uuid.UUID) string {
	raw := []byte{1}
	raw = append(raw, space[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(created.UnixMicro()))
	raw = append(raw, id[:]...)
	return base64.RawURLEncoding.EncodeToString(raw)
}
func decodeJoinReviewCursor(token string, space uuid.UUID) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 41 || raw[0] != 1 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return time.Time{}, uuid.Nil, entrypolicy.ErrInvalid
	}
	scope, e := uuid.FromBytes(raw[1:17])
	id, e2 := uuid.FromBytes(raw[25:41])
	micros := int64(binary.BigEndian.Uint64(raw[17:25]))
	if e != nil || e2 != nil || scope != space || id == uuid.Nil || micros <= 0 {
		return time.Time{}, uuid.Nil, entrypolicy.ErrInvalid
	}
	return time.UnixMicro(micros).UTC(), id, nil
}

// Internal data projection only. Public readers must use the sealed capability
// adapter, which checks current local membership before and after these reads.
func (s *SpaceStore) readJoinReviewGroup(ctx context.Context, space, request uuid.UUID) (*joinReviewGroupRead, error) {
	if s == nil || s.tx == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	group, err := s.checkedJoinRequestGroup(ctx, space, request)
	if errors.Is(err, ErrJoinAttemptDenied) {
		return nil, ErrJoinReadNotFound
	}
	if err != nil {
		return nil, err
	}
	var created time.Time
	if err = s.db().QueryRow(ctx, `SELECT created_at FROM space_join_request_groups WHERE id=$1 AND space_id=$2`, request, space).Scan(&created); err != nil {
		return nil, err
	}
	rows, err := s.db().Query(ctx, `SELECT a.id FROM space_join_request_attempts c JOIN space_join_attempts a ON a.id=c.attempt_id WHERE c.request_id=$1 AND c.space_id=$2 ORDER BY a.id`, request, space)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, ErrJoinAttemptUnavailable
	}
	result := &joinReviewGroupRead{summary: &spacev1.JoinRequestSummary{RequestId: request.String(), SpaceId: space.String(), ApplicantProfileId: group.ProfileID.String(), CreatedAt: timestamppb.New(created), AttemptCount: uint32(len(ids))}}
	if result.summary.CreatedAt.CheckValid() != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	for _, id := range ids {
		a, e := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, id))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, ErrJoinAttemptUnavailable
		}
		if e != nil {
			return nil, e
		}
		if !group.contains(a) {
			return nil, ErrJoinAttemptUnavailable
		}
		view, e := joinAttemptRead(a)
		if e != nil {
			return nil, e
		}
		requirements := &spacev1.JoinRequirements{RequirePhone: view.Requirements.Phone, RequireCaptcha: view.Requirements.Captcha, RequireManualApproval: view.Requirements.Manual}
		for _, q := range view.Requirements.Questions {
			requirements.Questions = append(requirements.Questions, &spacev1.JoinQuestion{QuestionId: q.ID, Prompt: q.Prompt})
		}
		projected := &spacev1.JoinRequestAttempt{AttemptId: id.String(), PolicyVersion: view.PolicyVersion, Requirements: requirements}
		for _, answer := range view.Answers {
			projected.SubmittedAnswers = append(projected.SubmittedAnswers, &spacev1.JoinAnswer{QuestionId: answer.QuestionID.String(), Value: answer.Value})
		}
		result.attempts = append(result.attempts, projected)
	}
	return result, nil
}
