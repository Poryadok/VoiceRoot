package authoritysource

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func authGolden(t *testing.T) ([]byte, AuthState) {
	t.Helper()
	raw, err := os.ReadFile("testdata/auth-state-v1.json")
	require.NoError(t, err)
	state, err := DecodeAuthState(raw)
	require.NoError(t, err)
	return raw, state
}

func TestAuthStateJavaGoldenKeepsExactRawAuthority(t *testing.T) {
	raw, state := authGolden(t)
	require.Len(t, state.Accounts, 3)
	require.Len(t, state.SDKIdentities, 3)
	require.True(t, state.AccountActive(state.Accounts[0].AccountID, 1))
	require.False(t, state.AccountActive(state.Accounts[0].AccountID, 2))
	require.False(t, state.AccountActive(state.Accounts[1].AccountID, 1), "standalone SDK identity is not an ordinary account")
	require.Equal(t, int64(1798761600000), state.ValidUntilUnixMillis(1767225600001))
	require.Zero(t, state.ValidUntilUnixMillis(1798761600000), "expired raw leases cannot renew a saved authority cut")
	require.Len(t, state.SDKBindings, 1)
	require.Len(t, state.SDKConversions, 1)
	require.Len(t, state.SDKMessageGrants, 1)
	again, err := json.Marshal(state)
	require.NoError(t, err)
	require.Equal(t, raw, again, "raw expiry facts remain byte stable")
}

func TestAuthStateRejectsIncompleteAndConflictingTuples(t *testing.T) {
	raw, _ := authGolden(t)
	for name, mutate := range map[string]func(*AuthState){
		"missing group":     func(s *AuthState) { s.SDKSessions = nil },
		"missing account":   func(s *AuthState) { s.Accounts = s.Accounts[:2] },
		"duplicate account": func(s *AuthState) { s.Accounts[1] = s.Accounts[0] },
		"ordinary SDK collision": func(s *AuthState) {
			s.Accounts[1] = s.Accounts[0]
			s.Accounts[1].AccountID = s.SDKIdentities[1].AccountID
		},
		"foreign device":            func(s *AuthState) { s.SDKDevices[0].AccountID = s.Accounts[0].AccountID },
		"foreign key environment":   func(s *AuthState) { s.SDKKeys[0].EnvironmentID = s.SDKKeys[0].ApplicationID },
		"duplicate session":         func(s *AuthState) { s.SDKSessions = append(s.SDKSessions, s.SDKSessions[0]) },
		"zero lease":                func(s *AuthState) { s.SDKSessions[0].UntilUnixMillis = 0 },
		"partial binding target":    func(s *AuthState) { s.SDKBindings[0].TargetProfileID = "" },
		"binding scope order":       func(s *AuthState) { s.SDKBindings[0].Scopes = []string{"game.chat.send", "game.chat.send"} },
		"foreign grant source":      func(s *AuthState) { s.SDKMessageGrants[0].SourceAccountID = s.Accounts[0].AccountID },
		"foreign grant environment": func(s *AuthState) { s.SDKMessageGrants[0].EnvironmentID = s.SDKMessageGrants[0].ApplicationID },
		"unknown grant request":     func(s *AuthState) { s.SDKMessageGrants[0].AuthorizationRequestID = s.SDKMessageGrants[0].GrantID },
		"negative floor":            func(s *AuthState) { s.Revision = 1 << 63 },
	} {
		t.Run(name, func(t *testing.T) {
			var state AuthState
			require.NoError(t, json.Unmarshal(raw, &state))
			mutate(&state)
			changed, err := json.Marshal(state)
			require.NoError(t, err)
			_, err = DecodeAuthState(changed)
			require.Error(t, err)
		})
	}
	for _, changed := range [][]byte{
		bytes.Replace(raw, []byte(`"sdk_sessions":`), []byte(`"unknown":`), 1),
		bytes.Replace(raw, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		append(append([]byte{}, raw...), '\n'),
	} {
		_, err := DecodeAuthState(changed)
		require.Error(t, err)
	}
}
