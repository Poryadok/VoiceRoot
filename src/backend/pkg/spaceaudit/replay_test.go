package spaceaudit

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func replayFixture() ReplayEvent {
	return ReplayEvent{
		AuditEventID: roleID, SpaceID: "11111111-1111-1111-1111-111111111111",
		ActorProfileID: "22222222-2222-2222-2222-222222222222",
		Action:         "role_created", TargetType: "role", TargetID: "33333333-3333-3333-3333-333333333333",
		DetailsJSON: "{}", OccurredAt: &timestamppb.Timestamp{},
	}
}

func TestReplayFingerprintFrozenVectors(t *testing.T) {
	// Independent vectors use Python struct.pack('>I', len(s)), exact UTF-8 bytes,
	// and struct.pack('>qI', seconds, nanos); never protobuf or UnixNano.
	for _, tc := range []struct {
		seconds int64
		nanos   int32
		digest  string
	}{
		{0, 0, "4c12dcf91d7c51e33667c75de520a86e775b8f723e36bc1a583eb53b0a0091ff"},
		{-62135596800, 1, "fbf2411891b09660e8ec8c793c1650a4865e187ffc8f85bc41aaab8682cef1b3"},
		{253402300799, 999999999, "d60a34dad4f7575180b6b099e781d85c42bf3c2e48973dac046a0d4f2ed50284"},
		{-1, 999999999, "a31d6ae2f7d2c066eb549180565e17e3cb88df7d7df5ac5a4032d621f6759bf0"},
	} {
		event := replayFixture()
		event.OccurredAt = &timestamppb.Timestamp{Seconds: tc.seconds, Nanos: tc.nanos}
		require.NoError(t, ValidateEvent("role", event))
		digest, err := ReplayFingerprint(event)
		require.NoError(t, err)
		require.Equal(t, tc.digest, hex.EncodeToString(digest[:]))
	}
}

func TestReplayFingerprintPreservesOriginalValidatedFields(t *testing.T) {
	base := replayFixture()
	want, err := ReplayFingerprint(base)
	require.NoError(t, err)
	for _, change := range []func(*ReplayEvent){
		func(e *ReplayEvent) { e.AuditEventID = strings.ToUpper(e.AuditEventID) },
		func(e *ReplayEvent) { e.SpaceID = "44444444-4444-4444-4444-444444444444" },
		func(e *ReplayEvent) { e.ActorProfileID = "44444444-4444-4444-4444-444444444444" },
		func(e *ReplayEvent) { e.Action = "role_deleted" },
		func(e *ReplayEvent) { e.TargetID = "44444444-4444-4444-4444-444444444444" },
		func(e *ReplayEvent) { e.OccurredAt = &timestamppb.Timestamp{Seconds: 1} },
		func(e *ReplayEvent) { e.OccurredAt = &timestamppb.Timestamp{Nanos: 1} },
	} {
		changed := base
		change(&changed)
		got, err := ReplayFingerprint(changed)
		require.NoError(t, err)
		require.NotEqual(t, want, got)
	}
	first := base
	first.Action, first.TargetType = "chat_override_set", "chat"
	first.DetailsJSON, err = OverrideDetails(first.Action, roleID, 0, 0)
	require.NoError(t, err)
	want, err = ReplayFingerprint(first)
	require.NoError(t, err)
	first.DetailsJSON, err = OverrideDetails(first.Action, roleID, 1, 0)
	require.NoError(t, err)
	got, err := ReplayFingerprint(first)
	require.NoError(t, err)
	require.NotEqual(t, want, got)
}

func TestValidateEventRejectsInvalidBeforeReplay(t *testing.T) {
	base := replayFixture()
	require.ErrorIs(t, ValidateEvent("chat", base), ErrProducerAction)
	require.ErrorIs(t, ValidateEvent("gateway", base), ErrProducerAction)
	for _, change := range []func(*ReplayEvent){
		func(e *ReplayEvent) { e.AuditEventID = "" },
		func(e *ReplayEvent) { e.SpaceID = "not-a-uuid" },
		func(e *ReplayEvent) { e.ActorProfileID = "00000000-0000-0000-0000-000000000000" },
		func(e *ReplayEvent) { e.TargetID = "" },
		func(e *ReplayEvent) { e.TargetType = "space" },
		func(e *ReplayEvent) { e.Action = "space_created" },
		func(e *ReplayEvent) { e.DetailsJSON = "{}\n" },
		func(e *ReplayEvent) { e.OccurredAt = nil },
		func(e *ReplayEvent) { e.OccurredAt = &timestamppb.Timestamp{Seconds: 253402300800} },
		func(e *ReplayEvent) { e.OccurredAt = &timestamppb.Timestamp{Nanos: -1} },
		func(e *ReplayEvent) {
			e.OccurredAt = &timestamppb.Timestamp{}
			e.OccurredAt.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		},
	} {
		invalid := base
		change(&invalid)
		require.Error(t, ValidateEvent("role", invalid))
		_, err := ReplayFingerprint(invalid)
		require.Error(t, err)
	}
	aggregate := base
	aggregate.Action, aggregate.TargetType, aggregate.DetailsJSON = "roles_reordered", "space", `{"count":1}`
	require.Error(t, ValidateEvent("role", aggregate))
	aggregate.TargetID = aggregate.SpaceID
	require.NoError(t, ValidateEvent("role", aggregate))
}
