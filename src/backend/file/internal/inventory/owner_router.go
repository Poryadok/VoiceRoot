package inventory

import (
	commonv1 "voice.app/voice/common/v1"
	inv "voice/backend/pkg/referenceinventory"
)

// OwnerRouter selects only configured sources for the exact compiled identity.
type OwnerRouter struct{ Story, Messaging Source }

func (r *OwnerRouter) SourceFor(id *commonv1.ReferenceInventoryIdentity) (Source, error) {
	if r == nil || inv.ValidateIdentity(id) != nil {
		return nil, inv.ErrInvalid
	}
	switch {
	case id.SourceService == "story" && id.SourceComponent == "story.media.v1":
		if r.Story != nil {
			return r.Story, nil
		}
	case id.SourceService == "messaging" && id.SourceComponent == "messaging.message.v1":
		if r.Messaging != nil {
			return r.Messaging, nil
		}
	}
	return nil, inv.ErrUnavailable
}
