package ordinarycall

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/voice/internal/retirementproof"
)

type CutoverExpectedRoom struct {
	WorkID, RoomID, Incarnation uuid.UUID
	Version                     uint64
}
type CutoverMaintenanceAuthority interface {
	CheckMaintenance(context.Context, retirementproof.CutBinding) error
}
type CutoverDrainOwner struct {
	Store       *PostgresStore
	Authority   CutoverMaintenanceAuthority
	Verifier    retirementproof.Verifier
	LegacyWorks retirementproof.OwnerWorkReader
}
