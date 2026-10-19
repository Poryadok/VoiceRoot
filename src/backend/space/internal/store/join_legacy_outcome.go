package store

import (
	"context"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

// ReplayLegacyJoinOutcome reads only the saved legacy result. The constructor
// has already authenticated the exact caller/body and ordinary current target
// access; no Subject/Role/current-policy read is performed here. A fresh no-key
// request constructs a different private operation and has no replay entitlement.
func (s *SpaceStore) ReplayLegacyJoinOutcome(ctx context.Context, p *JoinLegacyStartProposal) (*spacev1.SpaceMembership, bool, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil || p == nil || p.call == nil || p.selection == nil {
		return nil, false, ErrJoinAttemptUnavailable
	}
	if _, err := p.call.ActorFor(p.request); err != nil {
		return nil, false, err
	}
	in := p.selection.Context.Operation
	saved, err := s.loadLegacyJoinSelection(ctx, in, p.selection.Context.Origin)
	if err != nil {
		return nil, false, err
	}
	if saved.Operation.State != "COMPLETED" {
		return nil, false, nil
	}
	var code int32
	var raw []byte
	if err = s.db().QueryRow(ctx, `SELECT outcome_code,outcome_proto FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 AND state='COMPLETED'`, in.ActorProfileID, in.OperationID).Scan(&code, &raw); err != nil {
		return nil, true, ErrJoinAttemptUnavailable
	}
	if saved.Operation.AttemptID == nil || raw == nil {
		return nil, true, ErrJoinAttemptUnavailable
	}
	var result spacev1.SpaceMembership
	if proto.Unmarshal(raw, &result) != nil || len(result.ProtoReflect().GetUnknown()) != 0 {
		return nil, true, ErrJoinAttemptUnavailable
	}
	if _, err = p.call.ActorFor(p.request); err != nil {
		return nil, true, err
	}
	switch code {
	case 9:
		if result.SpaceId != "" || result.ProfileId != "" || result.JoinedAt != nil {
			return nil, true, ErrJoinAttemptUnavailable
		}
		return nil, true, ErrJoinAttemptPending
	case 0:
		if result.SpaceId != in.SpaceID.String() || result.ProfileId != in.ActorProfileID.String() || result.JoinedAt == nil || !result.JoinedAt.IsValid() {
			return nil, true, ErrJoinAttemptUnavailable
		}
		return &result, true, nil
	default:
		return nil, true, ErrJoinAttemptUnavailable
	}
}
