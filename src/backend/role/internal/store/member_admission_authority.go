package store

import (
	"context"
	"time"

	"voice/backend/pkg/memberadmission"
	"voice/backend/role/internal/memberauthority"
)

// PrepareQualifiedDefaultMemberRole takes an already authenticated observation
// obtained outside every Role transaction. It rechecks scope/expiry after lock
// acquisition and before commit. Only a new receipt needs exact PREPARING;
// historical replay remains inert even after source publication or removal.
func (s *RoleStore) PrepareQualifiedDefaultMemberRole(ctx context.Context, binding memberadmission.EnsureBinding, observation memberauthority.Observation) (MemberAdmissionReceipt, error) {
	return s.prepareDefaultMemberRole(ctx, binding, func(newReceipt bool) error {
		if observation.SpaceID() != binding.SpaceID || observation.ProfileID() != binding.ProfileID || observation.ChatID() != "" || !observation.Available(time.Now()) {
			return ErrScopeUnavailable
		}
		if newReceipt && !observation.CanPrepare(binding.MembershipGeneration, binding.AdmissionOperationID, binding.AdmissionBindingSHA256, time.Now()) {
			return ErrScopeUnavailable
		}
		return nil
	})
}

// RemoveQualifiedOrdinaryMemberRoles uses Space's exact authenticated removal
// request as the floor authority; the fresh observation supplies lifecycle and
// availability only. It never queries remote authority while holding Role locks.
func (s *RoleStore) RemoveQualifiedOrdinaryMemberRoles(ctx context.Context, binding memberadmission.RemoveBinding, observation memberauthority.Observation) (MemberAdmissionReceipt, error) {
	return s.removeOrdinaryMemberRoles(ctx, binding, func() error {
		if observation.SpaceID() != binding.SpaceID || observation.ProfileID() != binding.ProfileID || observation.ChatID() != "" || !observation.Available(time.Now()) {
			return ErrScopeUnavailable
		}
		return nil
	})
}
