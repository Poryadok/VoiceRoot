//go:build !voice_compose_fcm_diagnostic

package main

import "testing"

func TestComposeFcmObserverIsAbsentWithoutPrivateBuildTag(t *testing.T) {
	t.Setenv("VOICE_FCM_DIAGNOSTIC_FILE", "ignored")
	if newComposeFcmObserver() != nil {
		t.Fatal("diagnostic observer activated without private build tag")
	}
}
