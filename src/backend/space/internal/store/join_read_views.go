package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/entrypolicy"
)

var ErrJoinReadNotFound = errors.New("join request not found")

// These projections contain no configured answer, config digest, provider token
// or admission proof. They are data; the protected adapter supplies current
// owner authorization and checks its private capability again before returning.
type JoinQuestionRead struct{ ID, Prompt string }
type JoinRequirementsRead struct {
	Phone, Captcha, Manual bool
	Questions              []JoinQuestionRead
}
type JoinAttemptRead struct {
	ID, SpaceID              uuid.UUID
	PolicyVersion            int64
	State                    string
	CollectionDeadline       time.Time
	Requirements             JoinRequirementsRead
	Answers                  []entrypolicy.Answer
	RequestID, PublicationID *uuid.UUID
}

func joinAttemptRead(a *joinAttemptPrivate) (*JoinAttemptRead, error) {
	p, err := entrypolicy.Parse(a.policyRaw)
	if err != nil || !bytes.Equal(joinDigest("policy", p), a.policyDigest) || !bytes.Equal(joinBinding(a.Input, a.PolicyVersion, a.policyDigest), a.binding) {
		return nil, ErrJoinAttemptUnavailable
	}
	r := &JoinAttemptRead{ID: a.ID, SpaceID: a.Input.SpaceID, PolicyVersion: a.PolicyVersion, State: a.State, CollectionDeadline: a.CollectionDeadline, Requirements: JoinRequirementsRead{Phone: p.RequirePhone, Captcha: p.RequireCaptcha, Manual: p.RequireManualApproval}}
	for _, q := range p.Questions {
		r.Requirements.Questions = append(r.Requirements.Questions, JoinQuestionRead{q.ID.String(), q.Prompt})
	}
	if len(a.answers) > 0 && !bytes.Equal(a.answers, []byte("null")) {
		if json.Unmarshal(a.answers, &r.Answers) != nil {
			return nil, ErrJoinAttemptUnavailable
		}
	}
	if a.publication != nil {
		id := *a.publication
		r.PublicationID = &id
	}
	return r, nil
}

// ReadOwnJoinAttempt deliberately uses both saved account AND profile. A foreign
// attempt and a missing attempt have the same result, before any projection.
func (s *SpaceStore) ReadOwnJoinAttempt(ctx context.Context, space, attempt, account, profile uuid.UUID, finalGuard func() error) (*JoinAttemptRead, error) {
	if ctx == nil || s == nil || space == uuid.Nil || attempt == uuid.Nil || account == uuid.Nil || profile == uuid.Nil || finalGuard == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{space}, func(local *SpaceStore) (*JoinAttemptRead, error) {
		a, err := scanJoinAttempt(local.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 AND space_id=$2 AND account_id=$3 AND profile_id=$4`, attempt, space, account, profile))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJoinReadNotFound
		}
		if err != nil {
			return nil, err
		}
		// Visibility is a read gate. Capacity, guest admission, an existing member
		// and collection expiry do not turn a status read into a new admission.
		var visibility string
		if err = local.db().QueryRow(ctx, `SELECT visibility FROM spaces WHERE id=$1`, space).Scan(&visibility); err != nil {
			return nil, err
		}
		if visibility != "public" {
			member, err := local.IsSpaceMember(ctx, space, profile)
			if err != nil {
				return nil, err
			}
			if !member {
				if a.Input.InviteID == nil {
					return nil, ErrJoinReadNotFound
				}
				inv, err := local.GetInviteByID(ctx, *a.Input.InviteID)
				if err != nil {
					return nil, err
				}
				var now time.Time
				if err = local.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
					return nil, err
				}
				if inv == nil || inv.SpaceID != space || local.validateInviteForJoin(inv, now) != nil {
					return nil, ErrJoinReadNotFound
				}
			}
		}
		r, err := joinAttemptRead(a)
		if err != nil {
			return nil, err
		}
		var request uuid.UUID
		err = local.db().QueryRow(ctx, `SELECT request_id FROM space_join_request_attempts WHERE attempt_id=$1 AND space_id=$2`, attempt, space).Scan(&request)
		if err == nil {
			r.RequestID = &request
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if a.State == "COLLECTING" {
			var now time.Time
			if err = local.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
				return nil, err
			}
			if !now.Before(a.CollectionDeadline) {
				r.State = "EXPIRED"
			}
		}
		if err = finalGuard(); err != nil {
			return nil, err
		}
		return r, nil
	})
}
