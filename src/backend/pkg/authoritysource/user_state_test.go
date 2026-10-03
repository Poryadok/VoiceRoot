package authoritysource

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUserStateKeepsMissingSDKActorsDistinctFromInactiveProfiles(t *testing.T) {
	profile, account, actor := uuid.NewString(), uuid.NewString(), uuid.NewString()
	state := UserState{SchemaVersion: 1, Revision: 2, Profiles: []UserProfile{{ProfileID: profile, Exists: true, AccountID: account, ProfileRevision: 3}}, Tombstones: []UserAuthorTombstone{}}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	decoded, err := DecodeUserState(raw)
	require.NoError(t, err)
	require.True(t, decoded.EligibleProfile(profile, account))
	require.False(t, decoded.EligibleProfile(profile, uuid.NewString()))
	for _, field := range []string{"deleted", "frozen", "inactive"} {
		changed := state
		changed.Profiles = append([]UserProfile{}, state.Profiles...)
		switch field {
		case "deleted":
			changed.Profiles[0].Deleted = true
		case "frozen":
			changed.Profiles[0].Frozen = true
		case "inactive":
			changed.Profiles[0].AccountInactive = true
		}
		require.False(t, changed.EligibleProfile(profile, account), field)
	}
	missing := UserState{SchemaVersion: 1, Revision: 2, Profiles: []UserProfile{{ProfileID: actor}}, Tombstones: []UserAuthorTombstone{}}
	raw, err = json.Marshal(missing)
	require.NoError(t, err)
	decoded, err = DecodeUserState(raw)
	require.NoError(t, err, "an opaque SDK actor can have no User profile")
	require.False(t, decoded.EligibleProfile(actor, account), "absence alone cannot grant User eligibility")
	require.False(t, decoded.ActorRetired(actor, account), "absence alone is not a conversion fence")
	missing.Tombstones = []UserAuthorTombstone{{SourceAccountID: account, SourceActorID: actor, TargetAccountID: uuid.NewString(), TargetProfileID: profile, ProfileRevision: 3, TombstoneRevision: 1}}
	raw, err = json.Marshal(missing)
	require.NoError(t, err)
	decoded, err = DecodeUserState(raw)
	require.NoError(t, err)
	require.True(t, decoded.ActorRetired(actor, account))
	require.False(t, decoded.ActorRetired(actor, uuid.NewString()), "a historical alias is an exact actor/account fence")
	require.False(t, decoded.EligibleProfile(profile, account), "tombstone target never imports or grants profile rights")
}

func TestUserStateRejectsIncompleteAndAmbiguousFacts(t *testing.T) {
	profile, account := uuid.NewString(), uuid.NewString()
	state := UserState{SchemaVersion: 1, Revision: 1, Profiles: []UserProfile{{ProfileID: profile, Exists: true, AccountID: account, ProfileRevision: 1}}, Tombstones: []UserAuthorTombstone{}}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, func() error { _, err := DecodeUserState(raw); return err }())
	var facts map[string]any
	require.NoError(t, json.Unmarshal(raw, &facts))
	delete(facts, "tombstones")
	incomplete, _ := json.Marshal(facts)
	_, err = DecodeUserState(incomplete)
	require.Error(t, err)
	for _, mutate := range []func(*UserState){
		func(s *UserState) { s.Revision = 0 },
		func(s *UserState) { s.Profiles = append(s.Profiles, s.Profiles[0]) },
		func(s *UserState) { s.Profiles[0].Exists = false },
		func(s *UserState) { s.Profiles[0].AccountID = uuid.Nil.String() },
		func(s *UserState) {
			s.Tombstones = []UserAuthorTombstone{{SourceAccountID: account, SourceActorID: uuid.NewString(), TargetAccountID: account, TargetProfileID: profile, ProfileRevision: 1, TombstoneRevision: 1}}
		},
	} {
		changed := state
		changed.Profiles = append([]UserProfile{}, state.Profiles...)
		mutate(&changed)
		raw, _ = json.Marshal(changed)
		_, err = DecodeUserState(raw)
		require.Error(t, err)
	}
}
