package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/authprofileauthority"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/user/internal/identityassociation"
)

type AuthProfileReader interface {
	ReadAuthProfiles(context.Context, uuid.UUID, []uuid.UUID) (identityassociation.AuthProfileBatch, error)
}

// AuthProfileAuthorityGRPC is registered only on the dedicated Auth listener.
// Production supplies the qualified custody reader, never a raw store.
type AuthProfileAuthorityGRPC struct {
	userv1.UnimplementedAuthProfileAuthorityServiceServer
	Reader AuthProfileReader
}

func requireAuthProfile(ctx context.Context, method, id string, request proto.Message) error {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != "auth" || p.Subject != "service:auth" {
		return status.Error(codes.PermissionDenied, "Auth profile authority required")
	}
	hash, err := principal.RequestHash(request)
	if err != nil || !socialprincipal.AllowsMethod("auth-profile-authority", method) || p.Audience != "user" || p.RPC != method || p.RequestID != id || p.RequestHash != hash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "invalid Auth profile binding")
	}
	return nil
}

func authProfileCut(b identityassociation.AuthProfileBatch) *userv1.AuthProfileSourceCut {
	c := b.Cut
	return &userv1.AuthProfileSourceCut{Source: &userv1.AuthProfileSourcePin{UserSourceGeneration: c.UserSourceGeneration.String(), HistoryGeneration: c.HistoryGeneration.String(), KeysetRevision: c.KeysetRevision}, UserAuthorityRevision: c.UserAuthorityRevision, ThroughRevision: c.ThroughRevision, HeadDigest: append([]byte(nil), c.Digest[:]...), RetainedFloor: c.RetainedFloor, UnresolvedErasureIssued: b.UnresolvedErasureIssued, VerifiedErasureCovered: b.VerifiedErasureCovered, ObservedAt: timestamppb.New(b.ObservedAt)}
}

func (s *AuthProfileAuthorityGRPC) MatchAuthProfileReferences(ctx context.Context, r *userv1.MatchAuthProfileReferencesRequest) (*userv1.MatchAuthProfileReferencesResponse, error) {
	if err := authprofileauthority.ValidateMatchRequest(r); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Auth profile inventory")
	}
	if err := requireAuthProfile(ctx, userv1.AuthProfileAuthorityService_MatchAuthProfileReferences_FullMethodName, r.RequestId, r); err != nil {
		return nil, err
	}
	if s == nil || s.Reader == nil {
		return nil, status.Error(codes.Unavailable, "Auth profile authority unavailable")
	}
	ids := make([]uuid.UUID, len(r.ProfileIds))
	for i, id := range r.ProfileIds {
		ids[i] = uuid.MustParse(id)
	}
	b, err := s.Reader.ReadAuthProfiles(ctx, uuid.MustParse(r.AccountId), ids)
	if err != nil || ctx.Err() != nil || len(b.Results) != len(ids) {
		return nil, status.Error(codes.Unavailable, "Auth profile authority unavailable")
	}
	out := &userv1.MatchAuthProfileReferencesResponse{ProtocolVersion: 1, RequestId: r.RequestId, InventoryId: r.InventoryId, InventoryRevision: r.InventoryRevision, AccountId: r.AccountId, OwnerHandle: r.OwnerHandle, DeletionCycleId: r.DeletionCycleId, RequestDigest: authprofileauthority.Digest(r), SourceCut: authProfileCut(b), AllRequestedResolved: true}
	for _, v := range b.Results {
		item := &userv1.AuthProfileReferenceMatch{ProfileId: v.ProfileID.String(), Outcome: userv1.AuthProfileMatchOutcome_AUTH_PROFILE_MATCH_OUTCOME_INCOMPLETE}
		switch v.Match {
		case "MATCH":
			item.Outcome = userv1.AuthProfileMatchOutcome_AUTH_PROFILE_MATCH_OUTCOME_MATCH
		case "DIFFERENT_ACCOUNT":
			item.Outcome = userv1.AuthProfileMatchOutcome_AUTH_PROFILE_MATCH_OUTCOME_DIFFERENT_ACCOUNT
		case "INCOMPLETE":
			out.AllRequestedResolved = false
		default:
			return nil, status.Error(codes.Unavailable, "Auth profile authority unavailable")
		}
		if item.Outcome != userv1.AuthProfileMatchOutcome_AUTH_PROFILE_MATCH_OUTCOME_INCOMPLETE {
			item.AssociationId = v.Fact.AssociationID.String()
			item.AssociationRevision = v.Fact.Revision
			item.CaptureOrigin = string(v.Fact.Origin)
		}
		out.Results = append(out.Results, item)
	}
	if authprofileauthority.ValidateMatchResponse(r, out, time.Now()) != nil {
		return nil, status.Error(codes.Unavailable, "Auth profile authority unavailable")
	}
	return out, nil
}

func (s *AuthProfileAuthorityGRPC) ResolveAuthOwnershipTarget(ctx context.Context, r *userv1.ResolveAuthOwnershipTargetRequest) (*userv1.ResolveAuthOwnershipTargetResponse, error) {
	if authprofileauthority.ValidateTargetRequest(r) != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Auth ownership target")
	}
	if err := requireAuthProfile(ctx, userv1.AuthProfileAuthorityService_ResolveAuthOwnershipTarget_FullMethodName, r.RequestId, r); err != nil {
		return nil, err
	}
	if s == nil || s.Reader == nil {
		return nil, status.Error(codes.Unavailable, "Auth profile authority unavailable")
	}
	b, err := s.Reader.ReadAuthProfiles(ctx, uuid.Nil, []uuid.UUID{uuid.MustParse(r.ProfileId)})
	if err != nil || ctx.Err() != nil || len(b.Results) != 1 || b.Results[0].ProfileID.String() != r.ProfileId {
		return nil, status.Error(codes.Unavailable, "Auth profile authority unavailable")
	}
	v := b.Results[0]
	out := &userv1.ResolveAuthOwnershipTargetResponse{ProtocolVersion: 1, RequestId: r.RequestId, OperationId: r.OperationId, ProfileId: r.ProfileId, RequestDigest: authprofileauthority.Digest(r), SourceCut: authProfileCut(b), Outcome: userv1.AuthOwnershipTargetOutcome_AUTH_OWNERSHIP_TARGET_OUTCOME_INCOMPLETE}
	if v.Fact.Terminal != nil {
		out.Outcome = userv1.AuthOwnershipTargetOutcome_AUTH_OWNERSHIP_TARGET_OUTCOME_TERMINAL_PROFILE
		terminal := v.Fact.Terminal
		out.TerminalOperationId = terminal.OperationID.String()
		out.TerminalRevision = terminal.Revision
		out.TerminalReceiptId = terminal.ReceiptID.String()
		out.TerminalReceiptDigest = append([]byte(nil), terminal.ReceiptDigest[:]...)
	} else if v.CurrentAccount != uuid.Nil && v.Fact.AssociationID != uuid.Nil {
		out.Outcome = userv1.AuthOwnershipTargetOutcome_AUTH_OWNERSHIP_TARGET_OUTCOME_CURRENT_PROFILE
		out.AccountId = v.CurrentAccount.String()
		out.ProfileRevision = v.CurrentRevision
		out.Deleted = v.Deleted
		out.Frozen = v.Frozen
		out.AccountInactive = v.AccountInactive
	}
	if out.Outcome != userv1.AuthOwnershipTargetOutcome_AUTH_OWNERSHIP_TARGET_OUTCOME_INCOMPLETE {
		out.AssociationId = v.Fact.AssociationID.String()
		out.AssociationRevision = v.Fact.Revision
	}
	if authprofileauthority.ValidateTargetResponse(r, out, time.Now()) != nil {
		return nil, status.Error(codes.Unavailable, "Auth profile authority unavailable")
	}
	return out, nil
}
