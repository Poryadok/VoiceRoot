package grpcsvc

import (
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

func spaceRowToProto(r *store.SpaceRow) *spacev1.Space {
	return r.ToProto()
}
