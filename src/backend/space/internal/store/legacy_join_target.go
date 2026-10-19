package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CheckScopedLegacyJoinTarget is an additional PRE restriction for the native
// publisher, not a membership/Role grant. The protected adapter performs Social
// RPCs outside locks; this checks their exact owner/inviter target under the
// publisher's current fence and bounded observation deadline.
func (s *SpaceStore) CheckScopedLegacyJoinTarget(ctx context.Context, intent *MemberAdmissionIntent, target uuid.UUID, observed time.Time) error {
	if s == nil || s.tx == nil || intent == nil || target == uuid.Nil || observed.IsZero() || time.Now().Before(observed) || !time.Now().Before(observed.Add(5*time.Second)) {
		return ErrJoinAttemptUnavailable
	}
	if err := s.effectiveMutationScope(ctx, intent.Input.SpaceID); err != nil {
		return err
	}
	var current uuid.UUID
	var err error
	if intent.Input.InviteID == nil {
		err = s.tx.QueryRow(ctx, `SELECT owner_profile_id FROM public.spaces WHERE id=$1`, intent.Input.SpaceID).Scan(&current)
	} else {
		err = s.tx.QueryRow(ctx, `SELECT creator_profile_id FROM public.invites WHERE id=$1 AND space_id=$2`, *intent.Input.InviteID, intent.Input.SpaceID).Scan(&current)
	}
	if err != nil || current != target || !time.Now().Before(observed.Add(5*time.Second)) {
		return ErrJoinAttemptUnavailable
	}
	return nil
}
