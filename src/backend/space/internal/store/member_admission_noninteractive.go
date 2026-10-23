package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/memberadmissionrole"
)

// CompleteNonInteractiveMemberAdmission retains the exact public caller across
// Role preparation and the final local transaction. Interactive policies still
// require their separate decision/continuation protocol.
func (s *SpaceStore) CompleteNonInteractiveMemberAdmission(ctx context.Context, operation uuid.UUID, roles *memberadmissionrole.Client, call *lifecycleprincipal.VerifiedAdmissionCall, request proto.Message) (*MembershipRow, error) {
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	i, err := s.LookupMemberAdmission(ctx, operation)
	if err != nil {
		return nil, err
	}
	if i.State != "PREPARING" || i.Input.JoinAttempt != nil || len(i.Input.DecisionBasisSHA256) != 0 ||
		i.Input.AccountID.String() != actor.AccountID || i.Input.ProfileID.String() != actor.ProfileID || i.Input.Guest != (actor.AccountType == "guest") {
		return nil, ErrMemberAdmissionConflict
	}
	switch r := request.(type) {
	case *spacev1.JoinSpaceRequest:
		if actor.RPC != spacev1.SpaceService_JoinSpace_FullMethodName || i.Input.InviteID != nil || r.GetSpaceId() != i.Input.SpaceID.String() {
			return nil, ErrMemberAdmissionConflict
		}
	case *spacev1.JoinByInviteRequest:
		if actor.RPC != spacev1.SpaceService_JoinByInvite_FullMethodName || i.Input.InviteID == nil {
			return nil, ErrMemberAdmissionConflict
		}
		invite, err := s.GetInviteByID(ctx, *i.Input.InviteID)
		if err != nil {
			return nil, err
		}
		if invite == nil || invite.SpaceID != i.Input.SpaceID || invite.Code != strings.TrimSpace(r.GetCode()) {
			return nil, ErrMemberAdmissionConflict
		}
	default:
		return nil, ErrMemberAdmissionConflict
	}
	remaining, err := call.Remaining(request)
	if err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	binding := i.EnsureBinding()
	prepared, err := roles.Ensure(bounded, binding)
	if err != nil {
		return nil, err
	}
	role, receipt, err := prepared.For(binding)
	if err != nil {
		return nil, err
	}
	return s.publishMemberAdmissionWithInitiatingCall(bounded, operation, role, receipt, nil, nil, nil, newMemberAdmissionInitiatingCall(call, request))
}
