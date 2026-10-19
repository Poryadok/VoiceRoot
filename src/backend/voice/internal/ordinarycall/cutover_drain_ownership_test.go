package ordinarycall

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

func TestOrdinaryCapturedFootprintPreservesMixedTemporalOwners(t *testing.T) {
	native, first, _ := pendingGroup(t)
	op := native.Intent.OperationID
	require.NoError(t, native.RecordEnsureResult(op, true, testNow))
	require.NoError(t, native.CommitAdmission(op, testNow))
	member := native.Members[first.ProfileID.String()]
	foreign := uuid.NewString()
	call := store.Call{RoomID: native.Intent.RoomID.String(), ChatID: native.Intent.ChatID.String(), LivekitRoomName: native.LiveKitRoomName,
		OrdinaryOperationID: op.String(), OrdinaryVersion: native.Version,
		SessionKind:             callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		States:                  map[string]store.ParticipantState{first.ProfileID.String(): {ProfileID: first.ProfileID.String()}, foreign: {ProfileID: foreign}},
		OrdinaryMediaIdentities: map[string]string{first.ProfileID.String(): member.MediaIdentity},
		ScreenShares:            []store.ScreenShareEntry{{ProfileID: first.ProfileID.String(), StreamID: "native"}, {ProfileID: foreign, StreamID: "legacy"}}}
	encode := func(c store.Call) []byte { b, e := json.Marshal(c); require.NoError(t, e); return b }
	raw := encode(call)
	footprint, err := inspectOrdinaryCapturedCall(native, raw, native.Version)
	require.NoError(t, err)
	require.Equal(t, map[string]Member{first.ProfileID.String(): member}, footprint.Members)
	require.Equal(t, call, footprint.Call, "native classification never rewrites foreign participant/share evidence")
	retired := member
	retired.Generation = uuid.New()
	retired.MediaIdentity = "ov:" + retired.ProfileID.String() + ":" + retired.Generation.String()
	retired.State = MemberLeft
	native.RetiredMembers = append(native.RetiredMembers, retired)
	old := call
	old.OrdinaryMediaIdentities = map[string]string{first.ProfileID.String(): retired.MediaIdentity}
	oldFootprint, err := inspectOrdinaryCapturedCall(native, encode(old), native.Version)
	require.NoError(t, err)
	require.Equal(t, retired, oldFootprint.Members[first.ProfileID.String()], "retained media identity resolves the old generation, never the latest profile tuple")
	bad := call
	bad.OrdinaryOperationID = uuid.NewString()
	_, err = inspectOrdinaryCapturedCall(native, encode(bad), native.Version)
	require.ErrorIs(t, err, retirementproof.ErrConflict, "same room name is not the same native incarnation")
	bad = call
	bad.OrdinaryVersion++
	_, err = inspectOrdinaryCapturedCall(native, encode(bad), native.Version)
	require.ErrorIs(t, err, retirementproof.ErrConflict, "future projection is not covered by an earlier seal")
	bad = call
	bad.OrdinaryOperationID = ""
	_, err = inspectOrdinaryCapturedCall(native, encode(bad), native.Version)
	require.ErrorIs(t, err, retirementproof.ErrUnsupported, "markerless temporal ownership awaits explicit evidence, not current-head inference")
	bad = call
	bad.SpaceID = uuid.NewString()
	_, err = inspectOrdinaryCapturedCall(native, encode(bad), native.Version)
	require.ErrorIs(t, err, retirementproof.ErrConflict, "contradictory native owner markers cannot be prioritized away")
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	fields["unknown_issuance_lineage"] = json.RawMessage(`{"retained":true}`)
	unknown, err := json.Marshal(fields)
	require.NoError(t, err)
	_, err = inspectOrdinaryCapturedCall(native, unknown, native.Version)
	require.ErrorIs(t, err, retirementproof.ErrUnsupported, "unknown historical schema must remain retained and unavailable")
}
