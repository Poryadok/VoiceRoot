package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/spaceaccountcapture"
	"voice/backend/user/internal/identityassociation"
)

// CanonicalSpaceAccountCaptureAuthority is the Space-purpose projection of the
// single User owner core. Transport registration and caller grants stay separate.
type CanonicalSpaceAccountCaptureAuthority struct {
	capture func(context.Context, []uuid.UUID) (identityassociation.CaptureBatchResult, error)
}

func NewCanonicalSpaceAccountCaptureAuthority(store *identityassociation.Store) (*CanonicalSpaceAccountCaptureAuthority, error) {
	if store == nil {
		return nil, identityassociation.ErrIncomplete
	}
	return &CanonicalSpaceAccountCaptureAuthority{capture: store.CaptureProfiles}, nil
}

// NewCustodiedSpaceAccountCaptureAuthority composes the independently guarded
// capture operation. Its caller must supply a qualified source at startup.
func NewCustodiedSpaceAccountCaptureAuthority(capture func(context.Context, []uuid.UUID) (identityassociation.CaptureBatchResult, error)) (*CanonicalSpaceAccountCaptureAuthority, error) {
	if capture == nil {
		return nil, identityassociation.ErrIncomplete
	}
	return &CanonicalSpaceAccountCaptureAuthority{capture: capture}, nil
}

func (a *CanonicalSpaceAccountCaptureAuthority) CaptureSpaceMemberAccountBindings(ctx context.Context, req *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	if a == nil || a.capture == nil {
		return nil, identityassociation.ErrIncomplete
	}
	if err := spaceaccountcapture.ValidateRequest(req); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(req.Cohort.Entries))
	for _, entry := range req.Cohort.Entries {
		ids = append(ids, uuid.MustParse(entry.ProfileId))
	}
	captured, err := a.capture(ctx, ids)
	if err != nil {
		return nil, err
	}
	cut := captured.Cut
	out := &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: req.Protocol, OperationId: req.OperationId, Purpose: req.Purpose, SpaceId: req.SpaceId, CohortDigest: append([]byte(nil), req.CohortDigest...), RequestDigest: spaceaccountcapture.RequestDigest(req), OwnerSourceRevision: cut.UserAuthorityRevision, UserSourceGeneration: cut.UserSourceGeneration.String(), IdentityCut: &userv1.SpaceAccountIdentityCut{Owner: "user", Stream: "identity-association", HistoryGeneration: cut.HistoryGeneration.String(), ThroughRevision: cut.ThroughRevision, KeysetRevision: cut.KeysetRevision, Digest: append([]byte(nil), cut.Digest[:]...), RetainedFloor: cut.RetainedFloor}}
	for _, entry := range req.Cohort.Entries {
		observation, exists := captured.Observations[uuid.MustParse(entry.ProfileId)]
		if !exists {
			return nil, spaceaccountcapture.ErrContract
		}
		result := &userv1.SpaceAccountCaptureResult{Entry: proto.Clone(entry).(*userv1.SpaceAccountCaptureEntry), Outcome: userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE}
		fact := observation.Fact
		expected := entry.ExpectedAssociationId == "" || (entry.ExpectedAssociationId == fact.AssociationID.String() && entry.ExpectedAssociationRevision == fact.Revision)
		// A historical terminal has no reconstructed raw account tuple. An old
		// expected tuple requires fresh reconciliation rather than an invented pair.
		if expected && (entry.ExpectedAssociationId == "" || observation.AccountID != uuid.Nil) {
			switch observation.Status {
			case identityassociation.CaptureAssociated:
				if observation.AccountID == uuid.Nil {
					return nil, spaceaccountcapture.ErrContract
				}
				result.Outcome = userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED
			case identityassociation.CaptureTerminal:
				if fact.Terminal == nil {
					return nil, spaceaccountcapture.ErrContract
				}
				result.Outcome = userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_TERMINAL_PROFILE
				result.TerminalOperationId = fact.Terminal.OperationID.String()
				result.TerminalRevision = fact.Terminal.Revision
				result.TerminalReceiptId = fact.Terminal.ReceiptID.String()
				result.TerminalReceiptDigest = append([]byte(nil), fact.Terminal.ReceiptDigest[:]...)
			case identityassociation.CaptureIncomplete:
			default:
				return nil, spaceaccountcapture.ErrContract
			}
		}
		if result.Outcome != userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE {
			result.OwnerReceiptId = fact.ReceiptID.String()
			result.OwnerReceiptDigest = append([]byte(nil), fact.ReceiptDigest[:]...)
			result.CaptureCut = timestamppb.New(fact.CapturedAt)
			if observation.AccountID != uuid.Nil {
				result.AccountId = observation.AccountID.String()
				result.AssociationId = fact.AssociationID.String()
				result.AssociationRevision = fact.Revision
				result.ProfileRevision = fact.ProfileRevision
				result.Deleted = fact.Deleted
				result.Frozen = fact.Frozen
				result.AccountInactive = fact.AccountInactive
				result.Origin = userv1.SpaceAccountCaptureOrigin_SPACE_ACCOUNT_CAPTURE_ORIGIN_EXISTING_CURRENT_ROW
				switch fact.Origin {
				case identityassociation.OriginBirth:
					result.AssociationOrigin = userv1.SpaceAccountAssociationOrigin_SPACE_ACCOUNT_ASSOCIATION_ORIGIN_BIRTH
				case identityassociation.OriginRetainedNow:
					result.AssociationOrigin = userv1.SpaceAccountAssociationOrigin_SPACE_ACCOUNT_ASSOCIATION_ORIGIN_RETAINED_NOW
				case identityassociation.OriginVerifiedHistory:
					result.AssociationOrigin = userv1.SpaceAccountAssociationOrigin_SPACE_ACCOUNT_ASSOCIATION_ORIGIN_VERIFIED_HISTORY
				default:
					return nil, spaceaccountcapture.ErrContract
				}
			}
		}
		out.Results = append(out.Results, result)
	}
	if err = spaceaccountcapture.ValidateResponse(req, out); err != nil {
		return nil, err
	}
	return out, nil
}
