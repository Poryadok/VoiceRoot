// Package fencecustody implements the installer-custodied finite fencer from
// BE011-FENCE-CREDENTIAL-CUSTODY-v1. It is separate from the coverage executor:
// callers supply only protected command references, never credentials or SQL.
package fencecustody

import (
	"github.com/google/uuid"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/retirementproof"
)

const CommandDomain = "voice.party.postgres-fencer.command.v1"
const ReceiptDomain = "voice.party.postgres-fencer.receipt.v1"
const ObservationDomain = "voice.party.postgres-fencer.observation.v1"
const InstallerRootExec = "installer-local-root-exec:v1"

type CommandRef struct {
	CommandID uuid.UUID
	Hash      [32]byte
}

// PreparedPostgresCommand is admitted by the installer, not the executor. Its
// credential reference resolves only inside the fencer's private secret mount.
// A shared role or unknown alternate/promotable endpoint is unsupported by this
// first topology; installation custody must establish exclusive cluster use.
type PreparedPostgresCommand struct {
	Protocol                     uint32
	Kind                         string
	CommandID                    uuid.UUID
	Cut                          retirementproof.CutBinding
	PredecessorExecutionCohortID uuid.UUID
	Target                       partycoverage.PostgresRoleBinding
	Endpoint                     UnixPeerEndpoint
	CredentialCustodyID          uuid.UUID
	FencerLogin                  LoginIdentity
	FencerArtifactSHA256         [32]byte
	TransportBinding             string
}

// UnixPeerEndpoint is installer-protected endpoint lineage. A passwordless
// reachable TCP socket is not this authentication mechanism. PeerUID identifies
// the actual PostgreSQL OS principal observed on the connected Unix socket.
type UnixPeerEndpoint struct {
	SocketDirectory, Database string
	Port                      uint16
	PeerUID                   uint32
}

type LoginIdentity struct {
	RoleOID  uint32
	RoleName string
}

// InstallationWindow is an authorization record in external installer custody.
// It is not evidence that ingress is closed: actual deployment/execution and
// credential custody must already enforce the admitted closed window.
type InstallationWindow struct {
	Protocol                   uint32
	Installation               retirementproof.Installation
	CutoverOperationID         uuid.UUID
	CoverageEpoch              uint64
	PlanHash                   [32]byte
	ClosedAdmissionBarrierID   uuid.UUID
	ClosedAdmissionBarrierHash [32]byte
	AdmittedCommands           []CommandRef
}

type ReceiptRef struct {
	ReceiptID uuid.UUID
	Hash      [32]byte
}

type PostgresFenceReceipt struct {
	Protocol            uint32
	ReceiptID           uuid.UUID
	Command             CommandRef
	Cut                 retirementproof.CutBinding
	FencerArtifact      [32]byte
	Login               LoginIdentity
	CredentialCustodyID uuid.UUID
	Evidence            partycoverage.PostgresFenceEvidence
}

type ObservationRef struct {
	ObservationID uuid.UUID
	Hash          [32]byte
}

// Observations are new bound records, separate from the stable completed
// receipt. Historical receipt readback never substitutes for reobservation.
type PostgresFenceObservation struct {
	Protocol      uint32
	ObservationID uuid.UUID
	Command       CommandRef
	Receipt       ReceiptRef
	ObservedUTC   string
	Evidence      partycoverage.PostgresFenceEvidence
}

type FenceResult struct {
	Receipt     ReceiptRef
	Observation ObservationRef
}
