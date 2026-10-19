// Package legacycut owns maintenance-only legacy compatibility drain evidence.
// Protected deployment inventory and fresh resource retirement are prerequisites;
// an empty Redis scan, expired TTL or source label is never such evidence.
package legacycut

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

const Protocol = "voice.legacy-compatibility.retirement.v1"
const SourceKind = "LEGACY_COMPATIBILITY"

type Endpoint struct {
	ID               string
	ResourceIdentity retirementproof.Digest
	Database         int
	Prefix           string
}

// Inventory comes exclusively from the protected closed deployment inventory.
// The implementation must include ALL historical endpoint/DB/prefix routes and
// execution cohorts, including endpoints no longer in current serving config.
// CanonicalDeploymentBytes contains non-secret lineage/evidence, never DSNs.
type Inventory struct {
	Cut                      retirementproof.CutBinding
	Endpoints                []Endpoint
	ExecutionCohorts         []ExecutionCohort
	CanonicalDeploymentBytes json.RawMessage
}

type ExecutionCohort struct {
	ID                   uuid.UUID
	Implementation       retirementproof.Digest
	MemoryInventoryBytes json.RawMessage
}

type ProtectedInventoryReader interface {
	ReadClosedLegacyInventory(context.Context, retirementproof.CutBinding) (Inventory, error)
}

type MaintenanceAuthority interface {
	CheckMaintenance(context.Context, retirementproof.CutBinding) error
}

// EndpointOwner resolves the exact historical resource identity with scoped
// cleanup credentials. It must reject an omitted/replaced endpoint. The source
// owner supplies this implementation; IDs/addresses are never writer fences.
type EndpointOwner interface {
	Capture(context.Context, Endpoint) (store.LegacyRedisSnapshot, error)
	CompareCleanup(context.Context, Endpoint, store.LegacyRedisSnapshot) error
}

type Owner struct {
	Pool      *pgxpool.Pool
	Inventory ProtectedInventoryReader
	Authority MaintenanceAuthority
	Endpoints EndpointOwner
	Verifier  retirementproof.Verifier
}

type Receipt struct {
	Work           retirementproof.OwnerWorkRef
	Proof          retirementproof.ProofRef
	CanonicalBytes []byte
	Hash           retirementproof.Digest
}

type target struct {
	Kind      string
	Inventory *Inventory                 `json:",omitempty"`
	Endpoint  *Endpoint                  `json:",omitempty"`
	Snapshot  *store.LegacyRedisSnapshot `json:",omitempty"`
	Table     string                     `json:",omitempty"`
	Rows      []json.RawMessage
}

func ContractHash() retirementproof.Digest {
	return retirementproof.Hash("voice.legacy.owner-contract.v1", []byte("voice024;closed-historical-inventory;raw-redis-r22-memory;all-unknown-issuance;fresh-proof;exact-compare-cleanup;immutable-receipt;no-serving"))
}
