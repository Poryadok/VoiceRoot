package legacycut

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

func TestLegacyPartitionExactCoverageRejectsSubsetExtraAndDuplicateOwner(t *testing.T) {
	d := retirementproof.Hash("fixture", []byte("value"))
	owner := retirementproof.OwnerWorkRef{Protocol: 1, OwnerProtocol: "unit-owner", WorkID: uuid.New(), RequestHash: d}
	obligations := []Obligation{{ID: "call/native", ValueHash: d}, {ID: "call/unknown", ValueHash: d}}
	p := Partition{ID: uuid.New(), LegacyWork: owner, CaptureHash: d, Proof: retirementproof.ProofRef{Protocol: 1, ProofID: uuid.New(), ReceiptHash: d}, Assignments: []OwnershipAssignment{{ObligationID: "call/native", ValueHash: d, Owner: owner, OwnerWorkHash: d, TargetHash: d, Disposition: "DRAIN"}}}
	require.ErrorIs(t, ValidatePartitionShape(p, obligations), retirementproof.ErrConflict)
	p.Assignments = append(p.Assignments, p.Assignments[0])
	require.ErrorIs(t, ValidatePartitionShape(p, obligations), retirementproof.ErrConflict)
	p.Assignments[1].ObligationID = "unrelated"
	require.ErrorIs(t, ValidatePartitionShape(p, obligations), retirementproof.ErrConflict)
	p.Assignments[1].ObligationID = "call/unknown"
	require.NoError(t, ValidatePartitionShape(p, obligations))
	p.Assignments[1].ValueHash[0] ^= 1
	require.ErrorIs(t, ValidatePartitionShape(p, obligations), retirementproof.ErrConflict)
}
