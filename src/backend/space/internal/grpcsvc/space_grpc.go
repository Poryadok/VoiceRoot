package grpcsvc

import (
	"voice/backend/space/internal/spaceevents"
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

// SpaceGRPC implements SpaceService RPCs backed by space_db.
type SpaceGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
	Store       *store.SpaceStore
	SpaceEvents spaceevents.Publisher // optional; CreateSpace publishes space.created
}
