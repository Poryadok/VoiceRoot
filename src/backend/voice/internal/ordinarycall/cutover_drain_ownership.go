package ordinarycall

import (
	"bytes"
	"encoding/json"

	"github.com/google/uuid"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

// ordinaryCapturedFootprint identifies only exact native participant tuples in
// an immutable raw call. Foreign participants and their shares remain separate
// obligations. It is not a resource qualification or cleanup authorization.
type ordinaryCapturedFootprint struct {
	Call    store.Call
	Members map[string]Member
}

func inspectOrdinaryCapturedCall(native *Room, raw []byte, capturedVersion uint64) (ordinaryCapturedFootprint, error) {
	var zero ordinaryCapturedFootprint
	if native == nil || native.Intent.RoomID == uuid.Nil || native.Intent.OperationID == uuid.Nil || native.Version == 0 || capturedVersion < native.Version {
		return zero, retirementproof.ErrConflict
	}
	var call store.Call
	if json.Unmarshal(raw, &call) != nil {
		return zero, retirementproof.ErrUnsupported
	}
	// Exact current source schema only: a smaller typed decoder must never make
	// unknown history disappear or turn another owner's field into our claim.
	canonical, err := cutoverCanonical(raw)
	if err != nil {
		return zero, retirementproof.ErrUnsupported
	}
	encoded, err := json.Marshal(call)
	if err != nil {
		return zero, err
	}
	encoded, err = cutoverCanonical(encoded)
	if err != nil || !bytes.Equal(canonical, encoded) {
		return zero, retirementproof.ErrUnsupported
	}
	if call.OrdinaryOperationID == "" {
		return zero, retirementproof.ErrUnsupported
	}
	if call.RoomID != native.Intent.RoomID.String() || call.OrdinaryOperationID != native.Intent.OperationID.String() || call.ChatID != native.Intent.ChatID.String() || call.LivekitRoomName != native.LiveKitRoomName || call.OrdinaryVersion == 0 || call.OrdinaryVersion > capturedVersion {
		return zero, retirementproof.ErrConflict
	}
	kind := callsv1.VoiceSessionKind_VOICE_SESSION_KIND_CALL
	if native.Intent.Group {
		kind = callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE
	}
	if call.SessionKind != kind || call.ManagedGameSession || call.MatchSquadMatchID != "" || len(call.MatchSquadMemberEpochs) != 0 || len(call.MatchSquadTerminalEpochs) != 0 || call.ApplicationID != "" || call.EnvironmentID != "" || call.SessionID != "" || call.SpaceID != "" || call.VoiceRoomID != "" || call.SpaceLifecycleGeneration != 0 || len(call.SpaceMedia) != 0 {
		return zero, retirementproof.ErrConflict
	}
	for profile, state := range call.States {
		if profile != state.ProfileID {
			return zero, retirementproof.ErrConflict
		}
	}
	members := map[string]Member{}
	for profile, identity := range call.OrdinaryMediaIdentities {
		if identity == "" {
			return zero, retirementproof.ErrUnsupported
		}
		matches := []Member{}
		if current, exists := native.Members[profile]; exists && current.MediaIdentity == identity {
			matches = append(matches, current)
		}
		for _, retired := range native.RetiredMembers {
			if retired.ProfileID.String() == profile && retired.MediaIdentity == identity {
				matches = append(matches, retired)
			}
		}
		if len(matches) == 0 {
			// A room marker does not invent an omitted historical member target.
			return zero, retirementproof.ErrUnsupported
		}
		if len(matches) != 1 || matches[0].ProfileID.String() != profile || matches[0].Generation == uuid.Nil {
			return zero, retirementproof.ErrConflict
		}
		members[profile] = matches[0]
	}
	// Return all original mixed state/share bytes as a typed view. This helper
	// claims only the exact Members above, not every child or structural atom.
	return ordinaryCapturedFootprint{Call: call, Members: members}, nil
}
