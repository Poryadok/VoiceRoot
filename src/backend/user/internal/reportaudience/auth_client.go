package reportaudience

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"voice/backend/pkg/principal"
	authv1 "voice/backend/user/pb/voice/auth/v1"
)

// AuthClient is used only with the dedicated mTLS Auth audience listener.
// Each batch gets a fresh exact-body credential; inbound user metadata is discarded.
type AuthClient struct {
	Client authv1.AuthServiceClient
	Issuer *principal.Issuer
}

func (c AuthClient) Classify(ctx context.Context, evaluation uuid.UUID, ids []uuid.UUID) (Classification, error) {
	if c.Client == nil || c.Issuer == nil || evaluation == uuid.Nil || len(ids) == 0 || len(ids) > BatchSize {
		return Classification{}, ErrUnavailable
	}
	req := &authv1.ClassifyReportAudienceAccountsRequest{EvaluationId: evaluation.String()}
	for _, id := range ids {
		req.AccountIds = append(req.AccountIds, id.String())
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return Classification{}, err
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: authv1.AuthService_ClassifyReportAudienceAccounts_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return Classification{}, err
	}
	out, err := c.Client.ClassifyReportAudienceAccounts(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), req)
	if err != nil {
		return Classification{}, err
	}
	if out == nil || out.GetObservationStartedAt() == nil || out.GetObservationCompletedAt() == nil || out.GetExpiresAt() == nil ||
		out.GetObservationStartedAt().CheckValid() != nil || out.GetObservationCompletedAt().CheckValid() != nil || out.GetExpiresAt().CheckValid() != nil {
		return Classification{}, ErrUnavailable
	}
	result := Classification{StartedAt: out.GetObservationStartedAt().AsTime(), CompletedAt: out.GetObservationCompletedAt().AsTime(), ExpiresAt: out.GetExpiresAt().AsTime()}
	result.EvaluationID, err = canonicalID(out.GetEvaluationId())
	if err != nil {
		return Classification{}, err
	}
	result.SnapshotID, err = canonicalID(out.GetSnapshotId())
	if err != nil {
		return Classification{}, err
	}
	for _, account := range out.GetAccounts() {
		id, parseErr := canonicalID(account.GetAccountId())
		if parseErr != nil {
			return Classification{}, parseErr
		}
		state := Unknown
		switch account.GetState() {
		case authv1.ReportAudienceAccountState_REPORT_AUDIENCE_ACCOUNT_STATE_LIVE:
			state = Live
		case authv1.ReportAudienceAccountState_REPORT_AUDIENCE_ACCOUNT_STATE_DELETED:
			state = Deleted
		case authv1.ReportAudienceAccountState_REPORT_AUDIENCE_ACCOUNT_STATE_MISSING:
			state = Missing
		}
		result.Accounts = append(result.Accounts, Account{ID: id, State: state})
	}
	return result, nil
}

func canonicalID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, ErrUnavailable
	}
	return id, nil
}
