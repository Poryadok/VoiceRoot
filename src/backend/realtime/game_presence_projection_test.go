package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGamePresenceRealAdapterPreservesFilteredViewerProjection(t *testing.T) {
	conn, fixture := boundaryPresenceClient(t)
	fixture.gameTitle = "Filtered game"
	for _, tc := range []struct {
		name, account, source string
		visible               bool
	}{
		{"visible", boundaryAccount, "online", true},
		{"blocked", boundaryBlockedAccount, "online", false},
		{"invisible", boundaryAccount, "invisible", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := &connReg{accountID: tc.account, profileID: boundaryProfile, accountType: "regular"}
			payload, err := presenceFanoutPayload(boundaryAmbientContext(), newGRPCPresenceViewer(conn), boundaryTarget, tc.source, reg, "")
			require.NoError(t, err)
			requireBoundaryMetadata(t, <-fixture.metadata, tc.account, "regular")
			var body map[string]any
			require.NoError(t, json.Unmarshal(payload, &body))
			if tc.visible {
				require.Equal(t, "Filtered game", body["game_title"])
			} else {
				require.NotContains(t, body, "game_title")
			}
		})
	}
}

func TestGamePresenceFanoutOmitsRevokedExpiredAndFailedLookups(t *testing.T) {
	for _, tc := range []struct {
		name     string
		snapshot viewerPresence
		err      error
	}{
		{"game audience denied", viewerPresence{Status: "online"}, nil},
		{"expired", viewerPresence{}, nil},
		{"lookup failed", viewerPresence{}, errors.New("lookup unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viewer := &stubPresenceViewer{byViewer: map[string]viewerPresence{"viewer": tc.snapshot}, errs: map[string]error{"viewer": tc.err}}
			payload, err := presenceFanoutPayload(context.Background(), viewer, "target", "online", &connReg{profileID: "viewer"}, "")
			if tc.err != nil {
				require.Error(t, err)
				require.Nil(t, payload)
				return
			}
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(payload, &body))
			require.NotContains(t, body, "game_title")
		})
	}
}
