package grpcsvc

import (
	"voice/backend/role/internal/roleevents"
	"voice/backend/role/internal/store"

	rolev1 "voice.app/voice/role/v1"
)

// RoleGRPC implements voice.role.v1.RoleService (red-phase: Unimplemented stubs only).
type RoleGRPC struct {
	rolev1.UnimplementedRoleServiceServer
	Store  *store.RoleStore
	Events roleevents.Publisher
}
