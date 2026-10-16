package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
)

func TestFixturePrivacyRequiresExactStoryCallerAndProfile(t *testing.T) {
	author := uuid.New()
	fixture, closeFixture, err := startPrivacyFixture(author)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFixture()
	conn, err := grpc.NewClient("127.0.0.1:9091", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := userv1.NewUserServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, row := range []struct{ caller, profile string }{
		{"", author.String()}, {"gateway", author.String()}, {"story", uuid.New().String()},
	} {
		call := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", row.caller))
		_, err := client.GetPrivacySettings(call, &userv1.GetPrivacySettingsRequest{ProfileId: row.profile})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("unexpected denial: %v", err)
		}
	}
	if fixture.calls.Load() != 0 {
		t.Fatal("rejected call counted as proof")
	}
	call := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "story"))
	response, err := client.GetPrivacySettings(call, &userv1.GetPrivacySettingsRequest{ProfileId: author.String()})
	if err != nil {
		t.Fatal(err)
	}
	settings := response.GetPrivacySettings()
	audience := settings.GetShowStories()
	if settings.GetProfileId() != author.String() || !audience.GetFriends() || audience.GetFriendsOfFriends() || audience.GetSpaceMembers() || audience.GetIncludeGuests() || len(audience.GetSpaceIds()) != 0 {
		t.Fatal("fixture returned a broader privacy floor")
	}
	if fixture.calls.Load() != 1 {
		t.Fatal("real RPC not observed")
	}
}
