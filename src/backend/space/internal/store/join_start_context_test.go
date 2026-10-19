package store

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func fixedStartContext() JoinStartSelectionContext {
	id := func(b byte) uuid.UUID { var out uuid.UUID; copy(out[:], bytes.Repeat([]byte{b}, 16)); return out }
	return JoinStartSelectionContext{Operation: JoinOperationInput{SpaceID: id(6), ActorAccountID: id(3), ActorProfileID: id(4), OperationID: id(5), Method: JoinOperationStart, RequestSHA256: bytes.Repeat([]byte{0x11}, 32)}, Kind: 1, PolicyVersion: 7, PolicySHA256: bytes.Repeat([]byte{0x22}, 32)}
}

func TestJoinStartContextFrozenFrameAndSelectionIsolation(t *testing.T) {
	c := fixedStartContext()
	digest, err := c.SHA256()
	require.NoError(t, err)
	require.Equal(t, "c96aabdab8740e311f93926f5128b6d4521ed14f7cc096eb6194f049aa748cf2", hex.EncodeToString(digest[:]), "independent fixed binary vector, absent A and invite")
	invite := uuid.MustParse("07070707-0707-0707-0707-070707070707")
	c.Kind, c.InviteID = 2, &invite
	invited, err := c.SHA256()
	require.NoError(t, err)
	require.NotEqual(t, digest, invited)
	a := uuid.MustParse("08080808-0808-0808-0808-080808080808")
	c.SelectedAttemptID, c.SelectedBindingSHA256 = &a, bytes.Repeat([]byte{0x33}, 32)
	pinned, err := c.SHA256()
	require.NoError(t, err)
	require.NotEqual(t, invited, pinned)
	for _, mutate := range []func(*JoinStartSelectionContext){
		func(v *JoinStartSelectionContext) { v.Operation.Method = JoinOperationSubmit },
		func(v *JoinStartSelectionContext) { v.Operation.ActorAccountID = uuid.Nil },
		func(v *JoinStartSelectionContext) { v.PolicyVersion = 0 },
		func(v *JoinStartSelectionContext) { v.PolicySHA256 = make([]byte, 32) },
		func(v *JoinStartSelectionContext) { v.InviteID = &invite },
		func(v *JoinStartSelectionContext) { v.Kind = 2 },
		func(v *JoinStartSelectionContext) { v.SelectedBindingSHA256 = bytes.Repeat([]byte{3}, 32) },
		func(v *JoinStartSelectionContext) { v.SelectedAttemptID = &a },
	} {
		bad := fixedStartContext()
		mutate(&bad)
		_, err := bad.SHA256()
		require.ErrorIs(t, err, ErrJoinDecisionInvalid)
	}
}
