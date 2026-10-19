package store

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
)

func TestJoinLegacySelectionContextPurposeAndExactOrigin(t *testing.T) {
	invite := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	c := JoinLegacySelectionContext{Operation: JoinOperationInput{
		ActorAccountID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		ActorProfileID: uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		OperationID:    uuid.MustParse("33333333-3333-4333-8333-333333333333"),
		SpaceID:        uuid.MustParse("44444444-4444-4444-8444-444444444444"),
		Method:         spacev1.SpaceService_JoinByInvite_FullMethodName, RequestSHA256: bytes.Repeat([]byte{1}, 32),
	}, Origin: JoinOperationServerNoKey, InviteID: &invite, PolicyVersion: 7, PolicySHA256: bytes.Repeat([]byte{2}, 32)}
	got, err := c.SHA256()
	require.NoError(t, err)
	require.Equal(t, "f18da221622c91d2b2b6d9ce58eac22dd50efd2ff58c8d15f69ea19224a143cb", hex.EncodeToString(got[:]))
	for name, change := range map[string]func(*JoinLegacySelectionContext){
		"keyed_origin":      func(v *JoinLegacySelectionContext) { v.Origin = JoinOperationClientKey },
		"private_operation": func(v *JoinLegacySelectionContext) { v.Operation.OperationID = uuid.New() },
		"policy_revision":   func(v *JoinLegacySelectionContext) { v.PolicyVersion++ },
		"exact_invite":      func(v *JoinLegacySelectionContext) { id := uuid.New(); v.InviteID = &id },
		"body":              func(v *JoinLegacySelectionContext) { v.Operation.RequestSHA256 = bytes.Repeat([]byte{3}, 32) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := c
			change(&changed)
			h, err := changed.SHA256()
			require.NoError(t, err)
			require.NotEqual(t, got, h)
		})
	}
	for name, change := range map[string]func(*JoinLegacySelectionContext){
		"fake_submit":    func(v *JoinLegacySelectionContext) { v.Operation.Method = JoinOperationSubmit },
		"missing_invite": func(v *JoinLegacySelectionContext) { v.InviteID = nil },
		"zero_invite":    func(v *JoinLegacySelectionContext) { id := uuid.Nil; v.InviteID = &id },
		"invalid_origin": func(v *JoinLegacySelectionContext) { v.Origin = 0 },
		"missing_policy": func(v *JoinLegacySelectionContext) { v.PolicySHA256 = nil },
		"zero_operation": func(v *JoinLegacySelectionContext) { v.Operation.OperationID = uuid.Nil },
	} {
		t.Run(name, func(t *testing.T) {
			changed := c
			change(&changed)
			_, err := changed.SHA256()
			require.ErrorIs(t, err, ErrJoinDecisionInvalid)
		})
	}
	c.Operation.Method, c.InviteID = spacev1.SpaceService_JoinSpace_FullMethodName, nil
	_, err = c.SHA256()
	require.NoError(t, err)
	c.InviteID = &invite
	_, err = c.SHA256()
	require.ErrorIs(t, err, ErrJoinDecisionInvalid, "direct joins never discover or borrow an invite")
}
