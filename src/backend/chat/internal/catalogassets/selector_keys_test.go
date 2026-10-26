package catalogassets

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSelectorAliasesBindExactActorGenerationAndProviderTuple(t *testing.T) {
	oldKey, newKey := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	keys, err := NewActorKeyring("old", map[string][]byte{"old": oldKey})
	require.NoError(t, err)
	actor := Actor{AccountID: uuid.New(), ProfileID: uuid.New()}
	capture := ActorCapture{AccountGeneration: uuid.New(), ProfileGeneration: uuid.New()}
	selection := Selection{Provider: "fixture", ProviderID: "one"}
	aliases, err := keys.selectorAliases(actor, capture, selection)
	require.NoError(t, err)
	require.Len(t, aliases, 1)
	for _, change := range []func(*Actor, *ActorCapture, *Selection){
		func(a *Actor, _ *ActorCapture, _ *Selection) { a.AccountID = uuid.New() },
		func(a *Actor, _ *ActorCapture, _ *Selection) { a.ProfileID = uuid.New() },
		func(_ *Actor, c *ActorCapture, _ *Selection) { c.AccountGeneration = uuid.New() },
		func(_ *Actor, c *ActorCapture, _ *Selection) { c.ProfileGeneration = uuid.New() },
		func(_ *Actor, _ *ActorCapture, s *Selection) { s.Provider = "other" },
		func(_ *Actor, _ *ActorCapture, s *Selection) { s.ProviderID = "other" },
	} {
		a, c, s := actor, capture, selection
		change(&a, &c, &s)
		got, err := keys.selectorAliases(a, c, s)
		require.NoError(t, err)
		require.NotEqual(t, aliases[0].digest, got[0].digest)
	}
	rotated, err := NewActorKeyring("new", map[string][]byte{"old": oldKey, "new": newKey})
	require.NoError(t, err)
	got, err := rotated.selectorAliases(actor, capture, selection)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "new", got[0].keyID)
	require.Equal(t, aliases[0], got[1])
	_, err = keys.selectorAliases(Actor{}, capture, selection)
	require.Error(t, err)
	_, err = keys.selectorAliases(actor, ActorCapture{}, selection)
	require.Error(t, err)
	_, err = keys.selectorAliases(actor, capture, Selection{})
	require.Error(t, err)
}
