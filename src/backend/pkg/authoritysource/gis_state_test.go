package authoritysource

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGISStateCanonicalScopeAndLease(t *testing.T) {
	env, app, space, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	state := GISState{SchemaVersion: 1, Revision: 1, SpaceID: space, EnvironmentID: env,
		Subjects: []string{profile}, Bindings: []GISBinding{}, Installations: []GISInstallation{},
		Mappings: []GISMapping{}, ManagedChats: []GISManagedChat{}, Sessions: []GISSession{}}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	decoded, err := DecodeGISState(raw)
	require.NoError(t, err)
	require.False(t, decoded.EnvironmentExists)
	require.Zero(t, decoded.ValidUntilUnixMillis(1000))
	state.EnvironmentExists = true
	state.ApplicationID = app
	state.ApplicationStatus = "sandbox"
	state.EnvironmentStatus = "active"
	state.ApplicationRevision = 1
	state.EnvironmentRevision = 1
	state.EnvironmentKind = "sandbox"
	state.PlayerScopes = []string{"game.chat.read"}
	state.Providers = []string{"google"}
	state.RedirectURIs = []string{"https://game.example/callback"}
	state.AllowedOrigins = []string{}
	binding := GISBinding{BindingID: uuid.NewString(), AccountID: uuid.NewString(), ActorID: profile, ProfileID: profile, DeviceID: uuid.NewString(), Status: "active", AuthorityRevision: 1}
	state.Bindings = []GISBinding{binding}
	state.ManagedChats = []GISManagedChat{{BindingID: binding.BindingID, ChatID: uuid.NewString(), Status: "active", RosterRevision: 1, LeaseUntilUnixMillis: 2000}}
	raw, err = json.Marshal(state)
	require.NoError(t, err)
	decoded, err = DecodeGISState(raw)
	require.NoError(t, err)
	require.EqualValues(t, 2000, decoded.ValidUntilUnixMillis(1000))
	require.Zero(t, decoded.ValidUntilUnixMillis(2000), "raw expired state is stable and supplies no live lease")
	for name, mutate := range map[string]func(*GISState){
		"foreign subject": func(s *GISState) {
			s.Bindings[0].ProfileID = uuid.NewString()
			s.Bindings[0].ActorID = uuid.NewString()
		},
		"zero revision":             func(s *GISState) { s.Revision = 0 },
		"unknown binding status":    func(s *GISState) { s.Bindings[0].Status = "new_status" },
		"missing carries authority": func(s *GISState) { s.EnvironmentExists = false },
		"duplicate subjects":        func(s *GISState) { s.Subjects = append(s.Subjects, profile) },
		"unknown managed binding":   func(s *GISState) { s.ManagedChats[0].BindingID = uuid.NewString() },
	} {
		t.Run(name, func(t *testing.T) {
			var changed GISState
			require.NoError(t, json.Unmarshal(raw, &changed))
			mutate(&changed)
			bad, e := json.Marshal(changed)
			require.NoError(t, e)
			_, e = DecodeGISState(bad)
			require.Error(t, e)
		})
	}
	_, err = DecodeGISState(append(raw, byte(' ')))
	require.Error(t, err)
	_, err = DecodeGISState(append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"secret_digest":"hidden"}`)...))
	require.Error(t, err)
}
