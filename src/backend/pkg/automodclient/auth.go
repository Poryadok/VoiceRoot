// Package automodclient implements private request-bound authority clients.
// Callers supply the service's one existing signer and an mTLS connection;
// this package creates neither a signer nor a transport downgrade.
package automodclient

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/automodwire"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
)

type AuthClient struct {
	Issuer *principal.Issuer
	Client automodv1.AuthModerationAuthorityServiceClient
}

// RestrictionAheadError is not an acknowledgement or proof of history loss.
// The caller must compare Accepted with CURRENT local authority, outside any
// reciprocal network call's account transaction, before deciding on recovery.
type RestrictionAheadError struct {
	Requested, Accepted uint64
}

func (e *RestrictionAheadError) Error() string {
	return fmt.Sprintf("Auth accepted moderation revision %d exceeds requested revision %d", e.Accepted, e.Requested)
}

func (e *RestrictionAheadError) DivergesFrom(current uint64) bool {
	return e != nil && e.Accepted > current
}

func NewAuthClient(issuer *principal.Issuer, conn grpc.ClientConnInterface) *AuthClient {
	return &AuthClient{Issuer: issuer, Client: automodv1.NewAuthModerationAuthorityServiceClient(conn)}
}
func (c *AuthClient) signed(ctx context.Context, request proto.Message, method string) (context.Context, error) {
	if c == nil || c.Issuer == nil || c.Client == nil {
		return nil, unavailable()
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, unavailable()
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, unavailable()
	}
	// Drop all propagated user credentials and raw identity headers.
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}
func (c *AuthClient) ResolveSubject(ctx context.Context, account, expectedSubject uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
	if account == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "account is required")
	}
	request := &automodv1.ResolveModerationSubjectRequest{AccountId: account.String()}
	if expectedSubject != uuid.Nil {
		raw := expectedSubject.String()
		request.SubjectId = &raw
	}
	signed, err := c.signed(ctx, request, automodv1.AuthModerationAuthorityService_ResolveModerationSubject_FullMethodName)
	if err != nil {
		return nil, err
	}
	response, err := c.Client.ResolveModerationSubject(signed, request)
	if err != nil {
		return nil, err
	}
	if response == nil || !ValidSubject(response.GetSubject(), account, expectedSubject) || response.GetSessionEpoch() == 0 || response.GetSessionEpoch() > math.MaxInt64 {
		return nil, unavailable()
	}
	if response.GetSubject().GetState() != automodv1.SubjectState_SUBJECT_STATE_LIVE && !response.GetAuthRestricted() {
		return nil, unavailable()
	}
	return response, nil
}

// ValidSubject validates the exact binding rather than trusting an echoed ID.
func ValidSubject(binding *automodv1.SubjectBinding, account, expected uuid.UUID) bool {
	if binding == nil || binding.GetProtocolVersion() != 1 || binding.GetAccountId() != account.String() || binding.GetLifecycleRevision() == 0 || binding.GetLifecycleRevision() > math.MaxInt64 {
		return false
	}
	subject, err := uuid.Parse(binding.GetSubjectId())
	if err != nil || subject == uuid.Nil || subject.String() != binding.GetSubjectId() || expected != uuid.Nil && subject != expected {
		return false
	}
	switch binding.GetState() {
	case automodv1.SubjectState_SUBJECT_STATE_LIVE:
		return binding.PurgeAt == nil && binding.DeletionCycleId == nil
	case automodv1.SubjectState_SUBJECT_STATE_DELETED:
		if binding.GetPurgeAt() == nil || binding.GetPurgeAt().CheckValid() != nil {
			return false
		}
		cycle, err := uuid.Parse(binding.GetDeletionCycleId())
		return err == nil && cycle != uuid.Nil && cycle.String() == binding.GetDeletionCycleId()
	default:
		return false
	}
}
func (c *AuthClient) TerminalSubjects(ctx context.Context) ([]uuid.UUID, error) {
	response, err := c.TerminalSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]uuid.UUID, len(response.GetSubjectIds()))
	for index, raw := range response.GetSubjectIds() {
		result[index] = uuid.MustParse(raw)
	}
	return result, nil
}

// TerminalSnapshot preserves the exact seal for the current-boot import receipt.
// Do not replace it with a bare complete bool or a truncated list of subjects.
func (c *AuthClient) TerminalSnapshot(ctx context.Context) (*automodv1.ListTerminalModerationSubjectsResponse, error) {
	request := &automodv1.ListTerminalModerationSubjectsRequest{}
	signed, err := c.signed(ctx, request, automodv1.AuthModerationAuthorityService_ListTerminalModerationSubjects_FullMethodName)
	if err != nil {
		return nil, err
	}
	response, err := c.Client.ListTerminalModerationSubjects(signed, request)
	if err != nil {
		return nil, err
	}
	if automodwire.ValidateTerminalSnapshot(response) != nil {
		return nil, unavailable()
	}
	return response, nil
}
func (c *AuthClient) Reconcile(ctx context.Context, account, subject uuid.UUID, revision uint64, banned bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
	if account == uuid.Nil || subject == uuid.Nil || revision == 0 || revision > math.MaxInt64 {
		return nil, status.Error(codes.InvalidArgument, "invalid restriction binding")
	}
	request := &automodv1.ReconcileModerationRestrictionRequest{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), Revision: revision, ModerationBanned: banned}
	signed, err := c.signed(ctx, request, automodv1.AuthModerationAuthorityService_ReconcileModerationRestriction_FullMethodName)
	if err != nil {
		return nil, err
	}
	response, err := c.Client.ReconcileModerationRestriction(signed, request)
	if err != nil {
		return nil, err
	}
	if response == nil || response.GetProtocolVersion() != 1 || response.GetAccountId() != account.String() || response.GetSubjectId() != subject.String() || response.GetRevision() > math.MaxInt64 || (response.GetState() != automodv1.SubjectState_SUBJECT_STATE_LIVE && response.GetState() != automodv1.SubjectState_SUBJECT_STATE_DELETED) {
		return nil, unavailable()
	}
	if response.GetRevision() > revision {
		return nil, &RestrictionAheadError{Requested: revision, Accepted: response.GetRevision()}
	}
	if response.GetRevision() != revision || response.GetModerationBanned() != banned {
		return nil, unavailable()
	}
	return response, nil
}
func unavailable() error {
	return status.Error(codes.Unavailable, "private moderation authority unavailable")
}
