// Package reportthreshold consumes VALID receipts without changing public intake.
package reportthreshold

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/metadata"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/moderation/internal/store"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
)

var ErrAuthority = errors.New("report threshold authority unavailable")

type AudienceClient struct {
	User   userv1.UserServiceClient
	Issuer *principal.Issuer
}

func id(raw string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(raw)
	if err != nil || parsed == uuid.Nil || parsed.String() != raw {
		return uuid.Nil, ErrAuthority
	}
	return parsed, nil
}

func (c AudienceClient) Resolve(ctx context.Context, claim *store.ReportThresholdClaim) (store.ReportThresholdObservation, string, error) {
	if c.User == nil || c.Issuer == nil || claim == nil {
		return store.ReportThresholdObservation{}, "", ErrAuthority
	}
	started := time.Now().UTC()
	req := &userv1.ResolveReportThresholdContextRequest{EvaluationId: claim.ClaimID.String(), TargetProfileId: claim.ProfileID.String()}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return store.ReportThresholdObservation{}, "", err
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: userv1.UserService_ResolveReportThresholdContext_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return store.ReportThresholdObservation{}, "", err
	}
	out, err := c.User.ResolveReportThresholdContext(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), req)
	if err != nil {
		return store.ReportThresholdObservation{}, "", err
	}
	if out == nil || out.GetEvaluationId() != claim.ClaimID.String() || out.GetTargetProfileId() != claim.ProfileID.String() || out.GetMetricVersion() != store.ReportAudienceMetric ||
		out.GetObservationStartedAt() == nil || out.GetObservationCompletedAt() == nil || out.GetExpiresAt() == nil || out.GetObservationStartedAt().CheckValid() != nil || out.GetObservationCompletedAt().CheckValid() != nil || out.GetExpiresAt().CheckValid() != nil {
		return store.ReportThresholdObservation{}, "", ErrAuthority
	}
	now := time.Now().UTC()
	first, completed, expires := out.GetObservationStartedAt().AsTime(), out.GetObservationCompletedAt().AsTime(), out.GetExpiresAt().AsTime()
	if first.IsZero() || completed.Before(first) || completed.After(now) || !expires.After(now) || expires.After(first.Add(30*time.Second)) {
		return store.ReportThresholdObservation{}, "", ErrAuthority
	}
	if started.Before(first) {
		first = started
	}
	if expires.After(first.Add(30 * time.Second)) {
		expires = first.Add(30 * time.Second)
	}
	if !expires.After(now) {
		return store.ReportThresholdObservation{}, "", ErrAuthority
	}
	snapshot, err := id(out.GetSnapshotId())
	if err != nil {
		return store.ReportThresholdObservation{}, "", err
	}
	switch out.GetState() {
	case userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_MISSING:
		if out.GetAudienceAccounts() != 0 || out.GetTargetAccountId() != "" || out.GetProfileRevision() != 0 {
			return store.ReportThresholdObservation{}, "", ErrAuthority
		}
		return store.ReportThresholdObservation{}, "TARGET_MISSING", nil
	case userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_DELETED:
		if _, err := id(out.GetTargetAccountId()); err != nil || out.GetProfileRevision() == 0 || out.GetAudienceAccounts() != 0 {
			return store.ReportThresholdObservation{}, "", ErrAuthority
		}
		return store.ReportThresholdObservation{}, "TARGET_DELETED", nil
	case userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_EMPTY_AUDIENCE:
		if out.GetAudienceAccounts() != 0 || out.GetTargetAccountId() != claim.AccountID.String() || out.GetProfileRevision() == 0 {
			return store.ReportThresholdObservation{}, "", ErrAuthority
		}
		return store.ReportThresholdObservation{}, "EMPTY_AUDIENCE", nil
	case userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_LIVE:
		if out.GetAudienceAccounts() <= 0 || out.GetTargetAccountId() != claim.AccountID.String() || out.GetProfileRevision() == 0 || out.GetProfileRevision() > math.MaxInt64 {
			return store.ReportThresholdObservation{}, "", ErrAuthority
		}
	default:
		return store.ReportThresholdObservation{}, "", ErrAuthority
	}
	return store.ReportThresholdObservation{EvaluationID: claim.ClaimID, ProfileID: claim.ProfileID, AccountID: claim.AccountID, SnapshotID: snapshot, ProfileRevision: out.GetProfileRevision(), Audience: out.GetAudienceAccounts(), Metric: out.GetMetricVersion(), StartedAt: first, CompletedAt: completed, ExpiresAt: expires}, "", nil
}

type Worker struct {
	Store    *store.ReportThresholdStore
	Audience AudienceClient
	// Inject BE-045's shared protected client; no second subject authority exists.
	ResolveSubject func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error)
	Events         store.ReportThresholdEvents
}

func (w *Worker) EvaluateOne(ctx context.Context) (bool, error) {
	if w == nil || w.Store == nil || w.ResolveSubject == nil || w.Events == nil {
		return false, ErrAuthority
	}
	claim, err := w.Store.Claim(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if expired, err := w.Store.SettleExpired(ctx, claim); expired || err != nil {
		return true, err
	}
	if claim.AccountID == uuid.Nil {
		return true, w.Store.FinishManual(ctx, claim, "INELIGIBLE_EVIDENCE")
	}
	authorityCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	observation, manual, err := w.Audience.Resolve(authorityCtx, claim)
	if err == nil && manual != "" {
		return true, w.Store.FinishManual(ctx, claim, manual)
	}
	if err == nil {
		var subject *automodv1.ResolveModerationSubjectResponse
		subject, err = w.ResolveSubject(authorityCtx, claim.AccountID, uuid.Nil)
		if err == nil {
			binding := subject.GetSubject()
			if binding == nil || binding.GetProtocolVersion() != 1 || binding.GetAccountId() != claim.AccountID.String() || binding.GetState() != automodv1.SubjectState_SUBJECT_STATE_LIVE || binding.GetLifecycleRevision() == 0 || binding.GetLifecycleRevision() > math.MaxInt64 || binding.GetPurgeAt() != nil || binding.GetDeletionCycleId() != "" {
				err = ErrAuthority
			} else {
				observation.SubjectID, err = id(binding.GetSubjectId())
				observation.LifecycleRevision = binding.GetLifecycleRevision()
			}
		}
	}
	if err == nil {
		err = w.Store.Complete(ctx, claim, observation, w.Events)
	}
	if err != nil {
		retryErr := w.Store.Retry(ctx, claim)
		return true, errors.Join(err, retryErr)
	}
	return true, nil
}
