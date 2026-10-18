package spaceaudit

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const roleID = "01234567-89ab-cdef-0123-456789abcdef"

func TestDetailsGoldenVectors(t *testing.T) {
	for _, action := range []string{"role_created", "role_deleted", "default_join_role_set", "chat_created", "chat_deleted"} {
		got, err := EmptyDetails(action)
		require.NoError(t, err)
		require.Equal(t, `{}`, got)
		require.NoError(t, ValidateDetails(action, got))
	}
	for _, action := range []string{"role_assigned", "role_revoked"} {
		got, err := RoleAssignmentDetails(action, roleID)
		require.NoError(t, err)
		require.Equal(t, `{"role_id":"`+roleID+`"}`, got)
		require.NoError(t, ValidateDetails(action, got))
	}
	for _, action := range []string{"chat_override_set", "chat_override_removed", "voice_room_override_set", "voice_room_override_removed"} {
		for _, mask := range []uint64{0, 1 << 63, math.MaxUint64} {
			got, err := OverrideDetails(action, roleID, mask, mask)
			require.NoError(t, err, "overlapping and future bits are facts, not ingress policy")
			require.NoError(t, ValidateDetails(action, got))
		}
	}
	got, err := OverrideDetails("chat_override_set", roleID, math.MaxUint64, 0)
	require.NoError(t, err)
	require.Equal(t, `{"allow_mask":18446744073709551615,"deny_mask":0,"role_id":"`+roleID+`"}`, got)
	for _, count := range []uint64{1, math.MaxUint64} {
		got, err := ReorderDetails(count)
		require.NoError(t, err)
		require.NoError(t, ValidateDetails("roles_reordered", got))
	}
	fields := []string{"position", "name", "permissions"}
	got, err = UpdateDetails("role_updated", fields)
	require.NoError(t, err)
	require.Equal(t, `{"changed_fields":["name","permissions","position"]}`, got)
	require.Equal(t, []string{"position", "name", "permissions"}, fields, "builder must not mutate caller data")
	require.NoError(t, ValidateDetails("role_updated", got))
	got, err = UpdateDetails("chat_updated", []string{"topic", "threads_enabled", "slow_mode_seconds", "name", "avatar_url", "allow_user_main_feed", "allow_guests"})
	require.NoError(t, err)
	require.Equal(t, `{"changed_fields":["allow_guests","allow_user_main_feed","avatar_url","name","slow_mode_seconds","threads_enabled","topic"]}`, got)
	require.NoError(t, ValidateDetails("chat_updated", got))
}

func TestDetailsRejectNoncanonicalOrWrongDomain(t *testing.T) {
	for _, tc := range []struct{ action, body string }{
		{"role_created", ""}, {"role_created", "{}\n"}, {"role_created", " {}"},
		{"role_created", "\xef\xbb\xbf{}"}, {"role_created", "{}{}"},
		{"role_created", "null"}, {"role_created", "[]"}, {"role_created", `{"extra":0}`},
		{"space_created", `{}`}, {"unknown_action", `{}`},
		{"role_updated", `{}`}, {"role_updated", `{"changed_fields":[]}`},
		{"role_updated", `{"changed_fields":null}`}, {"role_updated", `{"changed_fields":["name","name"]}`},
		{"role_updated", `{"changed_fields":["position","name"]}`},
		{"role_updated", `{"changed_fields":["permissions_mask"]}`},
		{"role_updated", `{"changed_fields":["topic"]}`},
		{"role_updated", `{"changed_fields":["\u006eame"]}`},
		{"role_updated", `{"changed_fields":["name"],"changed_fields":["name"]}`},
		{"role_updated", `{"changed_fields":["name"],"name":"private value"}`},
		{"role_updated", `{"changed_fields": ["name"]}`},
		{"chat_updated", `{"changed_fields":["permissions"]}`},
		{"role_assigned", `{"role_id":"` + strings.ToUpper(roleID) + `"}`},
		{"role_assigned", `{"role_id":"invalid"}`},
		{"role_assigned", `{"role_id":"` + roleID + `","role_id":"` + roleID + `"}`},
		{"role_assigned", `{"\u0072ole_id":"` + roleID + `"}`},
		{"role_assigned", "{\"role_id\":\"\xff\"}"},
		{"roles_reordered", `{"count":0}`}, {"roles_reordered", `{"count":-1}`},
		{"roles_reordered", `{"count":"1"}`}, {"roles_reordered", `{"count":1.0}`},
		{"roles_reordered", `{"count":1e0}`}, {"roles_reordered", `{"count":01}`},
		{"roles_reordered", `{"count":18446744073709551616}`},
		{"chat_override_set", `{"allow_mask":0,"deny_mask":0}`},
		{"chat_override_set", `{"allow_mask":"0","deny_mask":0,"role_id":"` + roleID + `"}`},
		{"chat_override_set", `{"allow_mask":-0,"deny_mask":0,"role_id":"` + roleID + `"}`},
		{"chat_override_set", `{"allow_mask":0,"deny_mask":0,"role_id":null}`},
		{"chat_override_set", `{"deny_mask":0,"allow_mask":0,"role_id":"` + roleID + `"}`},
	} {
		t.Run(tc.action+"/"+tc.body, func(t *testing.T) { require.ErrorIs(t, ValidateDetails(tc.action, tc.body), ErrInvalidDetails) })
	}
}

func TestBuildersRejectWrongActionAndEmptyChanges(t *testing.T) {
	_, err := EmptyDetails("role_updated")
	require.ErrorIs(t, err, ErrInvalidDetails)
	for _, fields := range [][]string{nil, {"name", "name"}, {"permissions_mask"}, {"color"}} {
		_, err = UpdateDetails("role_updated", fields)
		require.ErrorIs(t, err, ErrInvalidDetails)
	}
	_, err = UpdateDetails("role_created", []string{"name"})
	require.ErrorIs(t, err, ErrInvalidDetails)
	_, err = RoleAssignmentDetails("role_created", roleID)
	require.ErrorIs(t, err, ErrInvalidDetails)
	_, err = OverrideDetails("chat_created", roleID, 0, 0)
	require.ErrorIs(t, err, ErrInvalidDetails)
	_, err = ReorderDetails(0)
	require.ErrorIs(t, err, ErrInvalidDetails)
}
