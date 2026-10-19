package legacycut

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

func TestLegacyMoveHistoryExpandsBothMixedSidesWithoutAssigningOwners(t *testing.T) {
	raw := []byte(`{"result":{"Source":{"room_id":"old","members":[{"profile_id":"native","generation":7},{"profile_id":"legacy","generation":null}]},"Destination":{"room_id":"new","members":[{"profile_id":"other","generation":9}]}}}`)
	obligations, err := deriveMoveHistory("move", "historical", "move:key", raw)
	require.NoError(t, err)
	require.Len(t, obligations, 17, "all scalars and structural nodes on both sides are obligations")
	byID := map[string]Obligation{}
	for _, o := range obligations {
		require.NotContains(t, byID, o.ID)
		byID[o.ID] = o
		require.Equal(t, "historical", o.Resource)
		require.Equal(t, "move:key", o.Key)
		require.True(t, json.Valid(o.SourceBytes))
	}
	for _, side := range []string{"Source", "Destination"} {
		root := "move/@history/" + side
		require.True(t, byID[root].Retain, "raw side remains retained")
		require.True(t, byID[root].CustodyOnly)
		require.Equal(t, "move/@retained", byID[root].ParentID)
		require.True(t, byID[root+"/json/@object"].CustodyOnly)
		require.Equal(t, root, byID[root+"/json/@object"].ParentID)
	}
	for id, value := range map[string]string{
		"move/@history/Source/json/members/0/profile_id":      `"native"`,
		"move/@history/Source/json/members/1/profile_id":      `"legacy"`,
		"move/@history/Source/json/members/1/generation":      `null`,
		"move/@history/Destination/json/members/0/profile_id": `"other"`,
	} {
		o, exists := byID[id]
		require.True(t, exists, id)
		require.Equal(t, value, string(o.SourceBytes))
		require.False(t, o.Retain)
		require.False(t, o.CustodyOnly)
	}
	// Reordered JSON object keys must produce the same ordered semantic children.
	var parsed any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	canonical, err := json.Marshal(parsed)
	require.NoError(t, err)
	other, err := deriveMoveHistory("move", "historical", "move:key", canonical)
	require.NoError(t, err)
	sort.Slice(obligations, func(i, j int) bool { return obligations[i].ID < obligations[j].ID })
	sort.Slice(other, func(i, j int) bool { return other[i].ID < other[j].ID })
	require.Equal(t, obligations, other)
	_, err = deriveMoveHistory("move", "historical", "move:key", []byte(`{"result":{"Source":{}}}`))
	require.ErrorIs(t, err, retirementproof.ErrConflict, "missing historical side cannot disappear")
	for _, raw := range []string{
		`{"result":{"Source":{},"source":{"room_id":"hidden"},"Destination":{}}}`,
		`{"result":{"Source":{},"Destination":{}},"Result":{"Source":{"room_id":"hidden"},"Destination":{}}}`,
	} {
		_, err = deriveMoveHistory("move", "historical", "move:key", []byte(raw))
		require.ErrorIs(t, err, retirementproof.ErrConflict, "ambiguous aliases cannot hide a history branch")
	}
}

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
