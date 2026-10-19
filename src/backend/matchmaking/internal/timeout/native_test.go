package timeout

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/store"
)

func TestNativeSweeperRefusesMissingAndInvalidBindingBeforeStores(t *testing.T) {
	var absent *Sweeper
	require.Error(t, absent.RunNativeOnce(context.Background()))
	s := &Sweeper{Matches: &store.MatchStore{}, Sessions: &store.SessionStore{}}
	require.Error(t, s.RunNativeOnce(context.Background()))
	s.Matches.NativeAdmission = &mmadmission.Binding{}
	require.Error(t, s.RunNativeOnce(context.Background()), "a nonnil pointer is not a validated native binding")
}
