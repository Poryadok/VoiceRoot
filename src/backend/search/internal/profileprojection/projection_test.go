package profileprojection

import (
	"testing"

	"github.com/stretchr/testify/require"

	userv1 "voice.app/voice/user/v1"
)

func TestEventFromProtoUsesNestedUpsertFields(t *testing.T) {
	t.Parallel()

	event, err := eventFromProto(&userv1.SearchProfileProjectionEvent{
		ProtocolVersion: 1,
		EventId:         "event-1",
		ProfileId:       "profile-1",
		SourceRevision:  1,
		Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{
			AccountId:            "account-1",
			Username:             "Alice",
			Discriminator:        "0001",
			DisplayName:          "Alice Example",
			UsernameSearchKey:    "alice",
			DisplayNameSearchKey: "aliceexample",
			NormalizationVersion: 1,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "Alice Example", event.DisplayName)
	require.Equal(t, "alice", event.UsernameSearchKey)
	require.Equal(t, "aliceexample", event.DisplayNameSearchKey)
	require.Equal(t, 1, event.NormalizationVersion)
}

func TestValidateEventRejectsMissingUnsupportedOrMismatchedNormalizerKeys(t *testing.T) {
	t.Parallel()

	valid := Event{
		ProtocolVersion:      1,
		EventID:              "event-1",
		ProfileID:            "profile-1",
		SourceRevision:       1,
		Kind:                 Upsert,
		Username:             "Alice",
		DisplayName:          "Alice Example",
		UsernameSearchKey:    "alice",
		DisplayNameSearchKey: "aliceexample",
		NormalizationVersion: 1,
	}

	for name, mutate := range map[string]func(*Event){
		"missing username key": func(event *Event) { event.UsernameSearchKey = "" },
		"unsupported version":  func(event *Event) { event.NormalizationVersion = 2 },
		"raw key mismatch":     func(event *Event) { event.Username = "Alice"; event.UsernameSearchKey = "ALICE" },
	} {
		t.Run(name, func(t *testing.T) {
			event := valid
			mutate(&event)
			require.Error(t, ValidateEvent(event))
		})
	}

	require.NoError(t, ValidateEvent(valid))
}

func TestApplyRevisionGuardsDuplicateConflictAndStaleResurrection(t *testing.T) {
	t.Parallel()

	state := State{}
	first := Event{ProtocolVersion: 1, EventID: "one", ProfileID: "profile-1", SourceRevision: 1, Kind: Upsert, Username: "Alice", DisplayName: "Alice", UsernameSearchKey: "alice", DisplayNameSearchKey: "alice", NormalizationVersion: 1}
	result, err := Apply(&state, first, "hash-1")
	require.NoError(t, err)
	require.Equal(t, Applied, result)

	result, err = Apply(&state, first, "hash-1")
	require.NoError(t, err)
	require.Equal(t, NoopDuplicate, result)

	result, err = Apply(&state, first, "different-hash")
	require.Error(t, err)
	require.Equal(t, Quarantined, result)

	delete := Event{ProtocolVersion: 1, EventID: "two", ProfileID: "profile-1", SourceRevision: 2, Kind: Delete}
	result, err = Apply(&state, delete, "hash-2")
	require.NoError(t, err)
	require.Equal(t, Applied, result)
	require.True(t, state.Tombstoned)

	result, err = Apply(&state, first, "hash-1")
	require.NoError(t, err)
	require.Equal(t, NoopStale, result)
	require.True(t, state.Tombstoned)
}
