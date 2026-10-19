package store

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/memberadmissionrole"
)

type memberAdmissionInitiatingCall struct {
	call    *lifecycleprincipal.VerifiedAdmissionCall
	request proto.Message
}

func newMemberAdmissionInitiatingCall(call *lifecycleprincipal.VerifiedAdmissionCall, request proto.Message) *memberAdmissionInitiatingCall {
	var snapshot proto.Message
	if request != nil {
		snapshot = proto.Clone(request)
	}
	return &memberAdmissionInitiatingCall{call: call, request: snapshot}
}

func (v *memberAdmissionInitiatingCall) check(ctx context.Context) error {
	if v == nil || ctx == nil || ctx.Err() != nil {
		return ErrJoinAttemptUnavailable
	}
	if _, err := v.call.ActorFor(v.request); err != nil {
		return ErrJoinAttemptUnavailable
	}
	return nil
}

// CompleteInteractiveMemberAdmissionWithPublicationCall retains the initiating
// interactive request as an additional finite restriction. It supplies no
// applicant/reviewer/default-role authority and never replaces continuation.
// The background API remains separate; public FIRST publication uses this API.
func (s *SpaceStore) CompleteInteractiveMemberAdmissionWithPublicationCall(ctx context.Context, operation uuid.UUID, roles *memberadmissionrole.Client, subjects *admissionauthority.Client, reviewers *admissionauthority.ReviewerClient, completion MemberAdmissionFinalGuard, publicationHook MemberAdmissionPublicationHook, call *lifecycleprincipal.VerifiedAdmissionCall, request proto.Message) (*MembershipRow, error) {
	return s.completeInteractiveMemberAdmission(ctx, operation, roles, subjects, reviewers, completion, publicationHook, newMemberAdmissionInitiatingCall(call, request))
}
