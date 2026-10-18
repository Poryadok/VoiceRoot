package main

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// remoteProofSearchMatchID reads matchId from the canonical protobuf response
// shape used by GET /api/v1/matchmaking/search/{id}.
func remoteProofSearchMatchID(body []byte) (string, error) {
	var response matchmakingv1.GetSearchStatusResponse
	if err := protojson.Unmarshal(body, &response); err != nil {
		return "", err
	}
	return response.GetSearchSession().GetMatchId(), nil
}

func TestRemoteProofSearchStatusUsesCanonicalProtoResponse(t *testing.T) {
	fixtures := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "protojson camelCase",
			body: `{"searchSession":{"id":"search-1","profileId":"profile-a","gameId":"game-1","mode":"duo","status":"matched","matchId":"match-camel"}}`,
			want: "match-camel",
		},
		{
			name: "protojson snake_case",
			body: `{"search_session":{"id":"search-2","profile_id":"profile-b","game_id":"game-1","mode":"duo","status":"matched","match_id":"match-snake"}}`,
			want: "match-snake",
		},
		{
			name:    "top-level match ID is outside the protobuf response schema",
			body:    `{"matchId":"not-the-search-session-match"}`,
			wantErr: true,
		},
		{
			name: "unmatched search session",
			body: `{"searchSession":{"id":"search-3","profileId":"profile-c","gameId":"game-1","mode":"duo","status":"searching"}}`,
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			got, err := remoteProofSearchMatchID([]byte(fixture.body))
			if fixture.wantErr {
				if err == nil {
					t.Fatal("response outside the protobuf schema should be rejected")
				}
				return
			}
			if err != nil {
				t.Fatalf("parse source-shaped status response: %v", err)
			}
			if got != fixture.want {
				t.Fatalf("match ID = %q, want %q", got, fixture.want)
			}
		})
	}
	if _, err := remoteProofSearchMatchID([]byte(`{"searchSession":`)); err == nil {
		t.Fatal("malformed status response should be rejected")
	}
}
