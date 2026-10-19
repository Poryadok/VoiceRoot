// Package evidence exposes only the private, authenticated committed-message
// protocol. The ordinary Moderation service never registers this service.
package evidence

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
	"voice/backend/moderation/internal/automod"
	"voice/backend/pkg/automodclient"
	"voice/backend/pkg/automodwire"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/socialprincipal"
)

type SubjectResolver interface {
	ResolveSubject(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error)
}
type Service struct {
	automodv1.UnimplementedModerationEvidenceServiceServer
	Store *automod.Store
	Auth  SubjectResolver
	Gate  *automod.BootGate
}

func (s *Service) authorize(ctx context.Context, method string, request proto.Message) (automod.BootCheck, error) {
	if err := socialprincipal.RequireAutoModEvidence(ctx, method, request); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil || s.Auth == nil {
		return nil, status.Error(codes.Unavailable, "moderation authority unavailable")
	}
	var check automod.BootCheck
	var err error
	if method == automodv1.ModerationEvidenceService_GetEffectiveRestrictionSnapshot_FullMethodName {
		check, err = s.Gate.AcquirePrivateRepair(ctx)
	} else {
		check, err = s.Gate.AcquireEffects(ctx)
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "current-boot moderation reconciliation incomplete")
	}
	return check, nil
}
func (s *Service) fresh(ctx context.Context, captured *automodv1.SubjectBinding) (*automodv1.ResolveModerationSubjectResponse, error) {
	if captured == nil {
		return nil, invalid()
	}
	account, err := identifier(captured.GetAccountId())
	if err != nil {
		return nil, err
	}
	subject, err := identifier(captured.GetSubjectId())
	if err != nil {
		return nil, err
	}
	if !automodclient.ValidSubject(captured, account, subject) {
		return nil, invalid()
	}
	fresh, err := s.Auth.ResolveSubject(ctx, account, subject)
	if err != nil {
		return nil, err
	}
	if fresh == nil || !automodclient.ValidSubject(fresh.GetSubject(), account, subject) || fresh.GetSubject().GetLifecycleRevision() < captured.GetLifecycleRevision() {
		return nil, status.Error(codes.Unavailable, "fresh subject authority unavailable")
	}
	return fresh, nil
}
func (s *Service) ProvisionSubject(ctx context.Context, r *automodv1.ProvisionSubjectRequest) (*automodv1.ProvisionSubjectResponse, error) {
	check, err := s.authorize(ctx, automodv1.ModerationEvidenceService_ProvisionSubject_FullMethodName, r)
	if err != nil {
		return nil, err
	}
	if r.GetBaselineSequence() != 0 {
		return nil, invalid()
	}
	epoch, err := identifier(r.GetSourceEpoch())
	if err != nil {
		return nil, err
	}
	fresh, err := s.fresh(ctx, r.GetSubject())
	if err != nil {
		return nil, err
	}
	if err = check(ctx); err != nil {
		return nil, mapError(err)
	}
	if err = s.Store.ProvisionChecked(ctx, subject(fresh.GetSubject(), epoch), check); err != nil {
		return nil, mapError(err)
	}
	through, err := s.Store.ThroughSequence(ctx, uuid.MustParse(fresh.GetSubject().GetSubjectId()))
	if err != nil {
		return nil, mapError(err)
	}
	return &automodv1.ProvisionSubjectResponse{ThroughSequence: through, SourceEpoch: r.GetSourceEpoch()}, nil
}
func (s *Service) IngestCommittedMessageEvidence(ctx context.Context, r *automodv1.IngestCommittedMessageEvidenceRequest) (*automodv1.IngestCommittedMessageEvidenceResponse, error) {
	check, err := s.authorize(ctx, automodv1.ModerationEvidenceService_IngestCommittedMessageEvidence_FullMethodName, r)
	if err != nil {
		return nil, err
	}
	if automodwire.RejectUnknown(r) != nil {
		return nil, invalid()
	}
	var captured *automodv1.SubjectBinding
	var epoch, eventID uuid.UUID
	var sequence uint64
	var recorded *timestamppb.Timestamp
	var eligibility automod.Eligibility
	var digests []automod.Digest
	var linkCount uint32
	var hash [32]byte
	if e := r.GetCore(); e != nil {
		_, hash, err = automodwire.EncodeCore(e)
		if err != nil {
			return nil, invalid()
		}
		captured = e.GetSubject()
		sequence = e.GetSequence()
		recorded = e.GetRecordedAt()
		epoch, err = identifier(e.GetSourceEpoch())
		if err != nil {
			return nil, err
		}
		eventID, err = identifier(e.GetEventId())
		if err != nil {
			return nil, err
		}
		switch e.GetEligibility() {
		case automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_ELIGIBLE:
			eligibility = automod.Eligible
		case automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_NO_TEXT:
			eligibility = automod.NoText
		case automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_E2E:
			eligibility = automod.E2E
		default:
			return nil, invalid()
		}
		for _, d := range e.GetDigests() {
			if len(d.GetHmacSha256()) != 32 {
				return nil, invalid()
			}
			var digest automod.Digest
			digest.KeyID = d.GetKeyId()
			copy(digest.Hash[:], d.GetHmacSha256())
			digests = append(digests, digest)
		}
		linkCount = e.GetLinkCount()
	} else if e := r.GetCoreExpired(); e != nil {
		if automodwire.ValidateExpired(e) != nil {
			return nil, invalid()
		}
		captured = e.GetSubject()
		sequence = e.GetSequence()
		recorded = e.GetRecordedAt()
		eligibility = automod.ExpiredEvidence
		copy(hash[:], e.GetOriginalCoreSha256())
		epoch, err = identifier(e.GetSourceEpoch())
		if err != nil {
			return nil, err
		}
		eventID, err = identifier(e.GetEventId())
		if err != nil {
			return nil, err
		}
	} else {
		return nil, invalid()
	}
	if sequence == 0 || sequence > math.MaxInt64 || recorded == nil || recorded.CheckValid() != nil || recorded.GetNanos()%1000 != 0 {
		return nil, invalid()
	}
	fresh, err := s.fresh(ctx, captured)
	if err != nil {
		return nil, err
	}
	if err = check(ctx); err != nil {
		return nil, mapError(err)
	}
	if err = s.Store.RefreshSubjectChecked(ctx, subject(fresh.GetSubject(), epoch), check); err != nil {
		return nil, mapError(err)
	}
	envelope := automod.Envelope{Subject: subject(captured, epoch), EventID: eventID, Sequence: int64(sequence), RecordedAt: recorded.AsTime(), Eligibility: eligibility, Digests: digests, LinkCount: linkCount, Hash: hash, BeforeCommit: check}
	receipt, err := s.Store.Ingest(ctx, envelope)
	if err != nil {
		return nil, mapError(err)
	}
	outcome := automodv1.EvidenceOutcome_EVIDENCE_OUTCOME_APPLIED
	if receipt.Duplicate {
		outcome = automodv1.EvidenceOutcome_EVIDENCE_OUTCOME_DUPLICATE
	} else if receipt.Expired {
		outcome = automodv1.EvidenceOutcome_EVIDENCE_OUTCOME_EXPIRED_EVIDENCE
	} else if receipt.Through < int64(sequence) {
		outcome = automodv1.EvidenceOutcome_EVIDENCE_OUTCOME_BUFFERED
	}
	return &automodv1.IngestCommittedMessageEvidenceResponse{Outcome: outcome, ThroughSequence: uint64(receipt.Through)}, nil
}
func (s *Service) GetAccountSendRestriction(ctx context.Context, r *automodv1.GetAccountSendRestrictionRequest) (*automodv1.GetAccountSendRestrictionResponse, error) {
	check, err := s.authorize(ctx, automodv1.ModerationEvidenceService_GetAccountSendRestriction_FullMethodName, r)
	if err != nil {
		return nil, err
	}
	account, err := identifier(r.GetAccountId())
	if err != nil {
		return nil, err
	}
	id, err := identifier(r.GetSubjectId())
	if err != nil {
		return nil, err
	}
	fresh, err := s.Auth.ResolveSubject(ctx, account, id)
	if err != nil {
		return nil, err
	}
	if fresh == nil || !automodclient.ValidSubject(fresh.GetSubject(), account, id) {
		return nil, status.Error(codes.Unavailable, "fresh subject authority unavailable")
	}
	if err = check(ctx); err != nil {
		return nil, mapError(err)
	}
	restriction, err := s.Store.ReadRestriction(ctx, account, id)
	if err != nil {
		return nil, mapError(err)
	}
	response := &automodv1.GetAccountSendRestrictionResponse{Banned: restriction.Banned || fresh.GetAuthRestricted() || fresh.GetSubject().GetState() != automodv1.SubjectState_SUBJECT_STATE_LIVE, Revision: restriction.Revision}
	if restriction.MutedUntil != nil {
		response.MutedUntil = timestamppb.New(*restriction.MutedUntil)
	}
	return response, nil
}
func (s *Service) GetEffectiveRestrictionSnapshot(ctx context.Context, r *automodv1.GetEffectiveRestrictionSnapshotRequest) (*automodv1.GetEffectiveRestrictionSnapshotResponse, error) {
	check, err := s.authorize(ctx, automodv1.ModerationEvidenceService_GetEffectiveRestrictionSnapshot_FullMethodName, r)
	if err != nil {
		return nil, err
	}
	account, err := identifier(r.GetAccountId())
	if err != nil {
		return nil, err
	}
	id, err := identifier(r.GetSubjectId())
	if err != nil {
		return nil, err
	}
	fresh, err := s.Auth.ResolveSubject(ctx, account, id)
	if err != nil {
		return nil, err
	}
	if fresh == nil || !automodclient.ValidSubject(fresh.GetSubject(), account, id) {
		return nil, status.Error(codes.Unavailable, "fresh subject authority unavailable")
	}
	if err = check(ctx); err != nil {
		return nil, mapError(err)
	}
	if err = s.Store.BindSubjectChecked(ctx, subject(fresh.GetSubject(), uuid.Nil), check); err != nil {
		return nil, mapError(err)
	}
	snapshot, err := s.Store.RestrictionSnapshotChecked(ctx, account, id, check)
	if err != nil {
		return nil, mapError(err)
	}
	return &automodv1.GetEffectiveRestrictionSnapshotResponse{Banned: snapshot.Banned, Revision: snapshot.Revision}, nil
}
func subject(b *automodv1.SubjectBinding, epoch uuid.UUID) automod.Subject {
	result := automod.Subject{ID: uuid.MustParse(b.GetSubjectId()), AccountID: uuid.MustParse(b.GetAccountId()), Epoch: epoch, Revision: int64(b.GetLifecycleRevision()), State: "LIVE"}
	if b.GetState() == automodv1.SubjectState_SUBJECT_STATE_DELETED {
		result.State = "DELETED"
		at := b.GetPurgeAt().AsTime()
		result.PurgeAt = &at
		result.CycleID = uuid.MustParse(b.GetDeletionCycleId())
	}
	return result
}
func identifier(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, invalid()
	}
	return id, nil
}
func invalid() error {
	return status.Error(codes.InvalidArgument, "invalid private moderation evidence")
}
func mapError(err error) error {
	switch {
	case errors.Is(err, automod.ErrPurged):
		return status.Error(codes.FailedPrecondition, "terminal moderation subject")
	case errors.Is(err, automod.ErrConflict):
		return status.Error(codes.AlreadyExists, "immutable moderation evidence conflict")
	case errors.Is(err, automod.ErrSubject):
		return status.Error(codes.FailedPrecondition, "moderation stream must be provisioned")
	case errors.Is(err, automod.ErrEvidence), errors.Is(err, automod.ErrSequence):
		return invalid()
	default:
		return status.Error(codes.Unavailable, "private moderation store unavailable")
	}
}
