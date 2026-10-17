package matchsquad

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

func TestManifestEntitlementDoesNotAdmitProfilesOutsideTheExactOwnedManifest(t *testing.T) {
	request := validCreateRequest()
	_, match, _, manifest, _, encoded, err := validateCreate(request)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := uuid.Parse(request.GetParticipants()[0].GetProfileId())
	second, _ := uuid.Parse(request.GetParticipants()[1].GetProfileId())
	outsider := uuid.MustParse("00000000-0000-4000-8000-000000000099")
	if !manifestContains(encoded, match, manifest[:], first) {
		t.Fatal("an exact manifest participant was rejected")
	}
	if manifestContains(encoded, match, manifest[:], outsider) {
		t.Fatal("an unlisted profile received MatchSquad membership entitlement")
	}
	if manifestContains(encoded, uuid.New(), manifest[:], first) {
		t.Fatal("manifest from another match was accepted")
	}
	wrongManifest := append([]byte(nil), manifest[:]...)
	wrongManifest[0] ^= 0xff
	if manifestContains(encoded, match, wrongManifest, second) {
		t.Fatal("manifest digest mismatch was accepted")
	}
}

func TestMemberActorRequiresExactDelegatedMethodRequestAndOperationBinding(t *testing.T) {
	request := &callsv1.JoinMatchSquadRoomRequest{
		ProtocolVersion: 1, OperationId: "00000000-0000-4000-8000-000000000001",
		MatchId: "00000000-0000-4000-8000-000000000002", RoomId: "00000000-0000-4000-8000-000000000003",
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	actor := principal.Principal{
		Kind: "delegated_user", Issuer: "gateway", Subject: "00000000-0000-4000-8000-000000000004",
		AccountID: "00000000-0000-4000-8000-000000000004", ProfileID: "00000000-0000-4000-8000-000000000005",
		Audience: "voice", RPC: callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName,
		RequestID: request.GetOperationId(), RequestHash: hash, SessionEpoch: 7, ExpiresAt: time.Now().Add(time.Minute),
	}
	ctx := principal.WithVerified(context.Background(), actor)
	if _, _, _, err := memberActor(ctx, request, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, request.GetOperationId()); err != nil {
		t.Fatalf("exact verified delegated request rejected: %v", err)
	}
	if _, _, _, err := memberActor(ctx, request, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, request.GetOperationId()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("principal for another method accepted: %v", err)
	}
	if _, _, _, err := memberActor(ctx, request, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, "00000000-0000-4000-8000-000000000099"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("operation id mismatch accepted: %v", err)
	}
	request.MatchId = "00000000-0000-4000-8000-000000000009"
	if _, _, _, err := memberActor(ctx, request, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, request.GetOperationId()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("mutated request accepted with stale request hash: %v", err)
	}
}
