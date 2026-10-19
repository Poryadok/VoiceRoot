package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

type JoinReviewReadProposal struct {
	request    proto.Message
	call       *lifecycleprincipal.VerifiedAdmissionCall
	input      JoinOperationInput
	expected   *authv1.ObserveSpaceAdmissionSubjectRequest
	requestID  uuid.UUID
	limit      int
	cursorTime time.Time
	cursorID   uuid.UUID
}
type VerifiedJoinReviewRead struct {
	proposal   *JoinReviewReadProposal
	subject    *admissionauthority.VerifiedSubject
	permission *admissionauthority.VerifiedReviewerPermission
}

// The exact private method and signed body select the read. Neither a cursor,
// Q identifier, nor a caller-provided principal establishes visibility.
func PrepareJoinReviewRead(ctx context.Context, original proto.Message) (*JoinReviewReadProposal, error) {
	var call *lifecycleprincipal.VerifiedAdmissionCall
	var spaceRaw string
	var err error
	var limit int
	var cursor string
	var requestID uuid.UUID
	switch r := original.(type) {
	case *spacev1.ListJoinRequestsRequest:
		if r == nil {
			return nil, entrypolicy.ErrInvalid
		}
		call, err = lifecycleprincipal.RequireAdmissionRequestList(ctx, r)
		spaceRaw = r.SpaceId
		limit = int(r.Limit)
		cursor = r.Cursor
		if limit == 0 {
			limit = 50
		}
		if limit > 100 {
			return nil, entrypolicy.ErrInvalid
		}
	case *spacev1.GetJoinRequestRequest:
		if r == nil {
			return nil, entrypolicy.ErrInvalid
		}
		call, err = lifecycleprincipal.RequireAdmissionRequestRead(ctx, r)
		spaceRaw = r.SpaceId
		var parseErr error
		requestID, parseErr = canonicalJoinID(r.RequestId)
		if parseErr != nil {
			return nil, parseErr
		}
	default:
		return nil, entrypolicy.ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	request := proto.Clone(original)
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	space, err := canonicalJoinID(spaceRaw)
	if err != nil {
		return nil, err
	}
	account, e := canonicalJoinID(actor.AccountID)
	profile, e2 := canonicalJoinID(actor.ProfileID)
	if e != nil || e2 != nil || actor.SessionEpoch <= 0 {
		return nil, ErrJoinAttemptUnavailable
	}
	p := &JoinReviewReadProposal{request: request, call: call, input: JoinOperationInput{SpaceID: space, ActorAccountID: account, ActorProfileID: profile, Method: actor.RPC}, requestID: requestID, limit: limit}
	if cursor != "" {
		p.cursorTime, p.cursorID, err = decodeJoinReviewCursor(cursor, space)
		if err != nil {
			return nil, err
		}
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	sha := sha256.Sum256(raw)
	frame := []byte("voice.space.join.read-context.v1\x00")
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(actor.RPC)))
	frame = append(frame, actor.RPC...)
	frame = append(frame, account[:]...)
	frame = append(frame, profile[:]...)
	frame = append(frame, space[:]...)
	frame = append(frame, sha[:]...)
	binding := sha256.Sum256(frame)
	epoch := uint64(actor.SessionEpoch)
	p.expected = &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER, AccountId: account.String(), ProfileId: profile.String(), SpaceId: space.String(), AdmissionBindingSha256: binding[:], InteractiveSessionEpoch: &epoch}
	return p, nil
}
func (p *JoinReviewReadProposal) ingress() error {
	if p == nil || p.call == nil || p.request == nil {
		return ErrJoinAttemptUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil {
		return err
	}
	if actor.AccountID != p.input.ActorAccountID.String() || actor.ProfileID != p.input.ActorProfileID.String() || actor.RPC != p.input.Method {
		return ErrJoinAttemptUnavailable
	}
	return nil
}
func (p *JoinReviewReadProposal) OwnerRequest() (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if err := p.ingress(); err != nil {
		return nil, err
	}
	return proto.Clone(p.expected).(*authv1.ObserveSpaceAdmissionSubjectRequest), nil
}
func (p *JoinReviewReadProposal) Seal(subject *admissionauthority.VerifiedSubject, permission *admissionauthority.VerifiedReviewerPermission) (*VerifiedJoinReviewRead, error) {
	d := &VerifiedJoinReviewRead{p, subject, permission}
	if err := d.check(); err != nil {
		return nil, err
	}
	return d, nil
}
func (d *VerifiedJoinReviewRead) check() error {
	if d == nil || d.proposal == nil {
		return ErrJoinAttemptUnavailable
	}
	p := d.proposal
	if err := p.ingress(); err != nil {
		return err
	}
	subject, err := d.subject.Transcript(p.expected)
	if err != nil {
		return err
	}
	permission, err := d.permission.Transcript(p.expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(subject.ObservationSHA256[:], permission.Subject.ObservationSHA256[:]) {
		return ErrJoinAttemptUnavailable
	}
	return nil
}

// Both data reads and final membership reread use the same fenced transaction.
// Role/Auth I/O has already completed outside the Space fence.
func (s *SpaceStore) readJoinReviews(ctx context.Context, d *VerifiedJoinReviewRead) (*spacev1.ListJoinRequestsResponse, *spacev1.GetJoinRequestResponse, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if err := d.check(); err != nil {
		return nil, nil, err
	}
	p := d.proposal
	type views struct {
		list   *spacev1.ListJoinRequestsResponse
		detail *spacev1.GetJoinRequestResponse
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{p.input.SpaceID}); err != nil {
		return nil, nil, err
	}
	scoped := *s
	scoped.tx = tx
	result, err := func(local *SpaceStore) (*views, error) {
		before, err := local.settingsMembership(ctx, p.input)
		if err != nil {
			return nil, err
		}
		out := &views{}
		if p.requestID != uuid.Nil {
			group, err := local.readJoinReviewGroup(ctx, p.input.SpaceID, p.requestID)
			if err != nil {
				return nil, err
			}
			out.detail = &spacev1.GetJoinRequestResponse{Request: group.summary, Attempts: group.attempts}
		} else {
			var boundary *time.Time
			var id *uuid.UUID
			if !p.cursorTime.IsZero() {
				boundary = &p.cursorTime
				id = &p.cursorID
			}
			rows, err := local.db().Query(ctx, `SELECT g.id FROM space_join_request_groups g WHERE g.space_id=$1 AND ($2::timestamptz IS NULL OR (g.created_at,g.id)<($2,$3::uuid)) AND EXISTS(SELECT 1 FROM space_join_request_attempts c JOIN space_join_attempts a ON a.id=c.attempt_id WHERE c.request_id=g.id AND a.state IN ('PENDING','READY','CLAIMED')) ORDER BY g.created_at DESC,g.id DESC LIMIT $4`, p.input.SpaceID, boundary, id, p.limit+1)
			if err != nil {
				return nil, err
			}
			var ids []uuid.UUID
			for rows.Next() {
				var q uuid.UUID
				if err = rows.Scan(&q); err != nil {
					rows.Close()
					return nil, err
				}
				ids = append(ids, q)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return nil, err
			}
			more := len(ids) > p.limit
			if more {
				ids = ids[:p.limit]
			}
			out.list = &spacev1.ListJoinRequestsResponse{}
			for _, q := range ids {
				group, e := local.readJoinReviewGroup(ctx, p.input.SpaceID, q)
				if e != nil {
					return nil, e
				}
				out.list.Requests = append(out.list.Requests, group.summary)
			}
			if more {
				last := out.list.Requests[len(out.list.Requests)-1]
				q, err := canonicalJoinID(last.RequestId)
				if err != nil {
					return nil, ErrJoinAttemptUnavailable
				}
				out.list.NextCursor = encodeJoinReviewCursor(p.input.SpaceID, last.CreatedAt.AsTime(), q)
			}
		}
		after, err := local.settingsMembership(ctx, p.input)
		if err != nil {
			return nil, err
		}
		if len(before) != len(after) {
			return nil, ErrJoinAttemptDenied
		}
		for i := range before {
			if !proto.Equal(before[i], after[i]) {
				return nil, ErrJoinAttemptDenied
			}
		}
		if err = d.check(); err != nil {
			return nil, err
		}
		return out, nil
	}(&scoped)
	if err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return result.list, result.detail, nil
}
func (s *SpaceStore) ListJoinRequestsVerified(ctx context.Context, d *VerifiedJoinReviewRead) (*spacev1.ListJoinRequestsResponse, error) {
	if d == nil || d.proposal == nil || d.proposal.limit == 0 {
		return nil, ErrJoinAttemptUnavailable
	}
	list, _, err := s.readJoinReviews(ctx, d)
	return list, err
}
func (s *SpaceStore) GetJoinRequestVerified(ctx context.Context, d *VerifiedJoinReviewRead) (*spacev1.GetJoinRequestResponse, error) {
	if d == nil || d.proposal == nil || d.proposal.requestID == uuid.Nil {
		return nil, ErrJoinAttemptUnavailable
	}
	_, detail, err := s.readJoinReviews(ctx, d)
	return detail, err
}
