package reminderhint

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func fixture() Hint {
	return Hint{Version: 1, NotificationID: LogicalID("PERSONAL", "11111111-1111-4111-8111-111111111111", "7", 3), TargetProfileID: "22222222-2222-4222-8222-222222222222", AggregateKind: "PERSONAL", AggregateID: "11111111-1111-4111-8111-111111111111", GraceRevision: "7", Day: 3, Plan: "premium", WindowEnd: "2026-10-28T00:00:00Z"}
}

func TestHintRejectsCaseVariantAliases(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	h := fixture()
	raw, err := json.Marshal(h)
	require.NoError(t, err)
	for _, field := range []struct{ key, alias, value string }{
		{"target_profile_id", "TARGET_PROFILE_ID", `"33333333-3333-4333-8333-333333333333"`},
		{"day", "DAY", `3`},
		{"schema_version", "SCHEMA_VERSION", `1`},
	} {
		t.Run(field.key, func(t *testing.T) {
			duplicate := strings.TrimSuffix(string(raw), "}") + `,"` + field.alias + `":` + field.value + `}`
			_, err := Decode([]byte(duplicate), now)
			require.ErrorIs(t, err, ErrInvalid, "case alias must not overwrite an exact field")
			aliasOnly := strings.Replace(string(raw), `"`+field.key+`":`, `"`+field.alias+`":`, 1)
			_, err = Decode([]byte(aliasOnly), now)
			require.ErrorIs(t, err, ErrInvalid, "only canonical field spelling is accepted")
		})
	}
}

func TestHintStrictShapeAndClientPrivacy(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	h := fixture()
	raw, err := json.Marshal(h)
	require.NoError(t, err)
	got, err := Decode(raw, now)
	require.NoError(t, err)
	require.Equal(t, h, got)
	client, err := got.ClientData()
	require.NoError(t, err)
	require.NotContains(t, string(client), h.AggregateID)
	for _, field := range []string{"aggregate_id", "aggregate_kind", "grace_revision", "payer", "provider"} {
		require.NotContains(t, string(client), field)
	}
	require.Contains(t, string(client), `"type":"subscription_grace_reminder"`)
	for _, mutate := range []func(*Hint){
		func(h *Hint) { h.Version = 2 }, func(h *Hint) { h.GraceRevision = "07" },
		func(h *Hint) { h.TargetProfileID = "invalid" }, func(h *Hint) { h.Day = 2 },
		func(h *Hint) { h.Plan = "space_pro" }, func(h *Hint) { h.SpaceID = h.AggregateID },
		func(h *Hint) { h.WindowEnd = "2026-10-28T00:00:00+00:00" },
		func(h *Hint) { h.NotificationID = h.TargetProfileID },
	} {
		bad := h
		mutate(&bad)
		b, _ := json.Marshal(bad)
		_, err = Decode(b, now)
		require.Error(t, err, string(b))
	}
	_, err = Decode(raw, time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC))
	require.ErrorIs(t, err, ErrExpired)
	_, err = Decode([]byte(strings.TrimSuffix(string(raw), "}")+`,"target_profile_id":"33333333-3333-4333-8333-333333333333"}`), now)
	require.Error(t, err)
	_, err = Decode(append(raw, []byte(` {}`)...), now)
	require.Error(t, err)
	_, err = Decode([]byte(strings.TrimSuffix(string(raw), "}")+`,"provider":"secret"}`), now)
	require.Error(t, err)
	h.AggregateKind = "SPACE"
	h.Plan = "space_pro"
	h.SpaceID = h.AggregateID
	h.NotificationID = LogicalID(h.AggregateKind, h.AggregateID, h.GraceRevision, h.Day)
	raw, _ = json.Marshal(h)
	_, err = Decode(raw, now)
	require.NoError(t, err)
	client, err = h.ClientData()
	require.NoError(t, err)
	require.Contains(t, string(client), `"space_id":"`+h.SpaceID+`"`)
}
