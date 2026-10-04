package matchsquad

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/matchsquadprincipal"
)

func validCreateRequest() *callsv1.CreateMatchSquadRoomRequest {
	operation := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	match := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	first := uuid.MustParse("00000000-0000-4000-8000-000000000010")
	second := uuid.MustParse("00000000-0000-4000-8000-000000000020")
	manifest := sha256.Sum256(append(append([]byte{}, first[:]...), second[:]...))
	return &callsv1.CreateMatchSquadRoomRequest{
		ProtocolVersion: 1, OperationId: operation.String(), MatchId: match.String(),
		Participants:              []*chatv1.MatchSquadParticipant{{ProfileId: first.String()}, {ProfileId: second.String()}},
		ParticipantManifestSha256: manifest[:],
		ChatCreationReceipt: &chatv1.MatchSquadChatReceipt{
			ProtocolVersion: 1, ReceiptId: "00000000-0000-4000-8000-000000000030",
			OperationId: operation.String(), MatchId: match.String(), ChatId: "00000000-0000-4000-8000-000000000040",
			ParticipantManifestSha256: manifest[:], RequestSha256: make([]byte, sha256.Size),
		},
	}
}

func validCompactionRequest() *callsv1.CompactMatchSquadRoomRequest {
	completed := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	return &callsv1.CompactMatchSquadRoomRequest{
		ProtocolVersion: 1, OperationId: "00000000-0000-4000-8000-000000000101",
		TeardownAggregateId: "00000000-0000-4000-8000-000000000102", MatchId: "00000000-0000-4000-8000-000000000103",
		RoomId: "00000000-0000-4000-8000-000000000104", CreationReceiptId: "00000000-0000-4000-8000-000000000105",
		CreationRequestSha256: make([]byte, sha256.Size), TeardownOperationId: "00000000-0000-4000-8000-000000000106",
		TeardownReceiptId: "00000000-0000-4000-8000-000000000107", TeardownReceiptSha256: make([]byte, sha256.Size),
		ParticipantManifestSha256: make([]byte, sha256.Size), AggregateCompletedAt: timestamppb.New(completed),
		CompactionAuthorizedAt: timestamppb.New(completed.Add(matchSquadRetention)),
	}
}

func TestValidateCompactionUsesOnlyAggregateBoundTimes(t *testing.T) {
	request := validCompactionRequest()
	ids, completed, authorized, raw, err := validateCompaction(request)
	if err != nil || len(ids) != 7 || raw == nil || !authorized.Equal(completed.Add(matchSquadRetention)) {
		t.Fatalf("valid aggregate-authorized request rejected: ids=%v completed=%v authorized=%v err=%v", ids, completed, authorized, err)
	}

	t.Run("early authorization is rejected without a provider clock", func(t *testing.T) {
		bad := validCompactionRequest()
		bad.CompactionAuthorizedAt = timestamppb.New(bad.AggregateCompletedAt.AsTime().Add(matchSquadRetention - time.Nanosecond))
		if _, _, _, _, err := validateCompaction(bad); err == nil {
			t.Fatal("authorization before aggregate completion plus 30 days was accepted")
		}
	})
	t.Run("malformed ids and digests are rejected", func(t *testing.T) {
		badID := validCompactionRequest()
		badID.RoomId = "not-a-uuid"
		if _, _, _, _, err := validateCompaction(badID); err == nil {
			t.Fatal("noncanonical room id was accepted")
		}
		badHash := validCompactionRequest()
		badHash.TeardownReceiptSha256 = []byte{1}
		if _, _, _, _, err := validateCompaction(badHash); err == nil {
			t.Fatal("short receipt digest was accepted")
		}
	})
	t.Run("missing timestamps are rejected", func(t *testing.T) {
		bad := validCompactionRequest()
		bad.CompactionAuthorizedAt = nil
		if _, _, _, _, err := validateCompaction(bad); err == nil {
			t.Fatal("missing aggregate-authorized timestamp was accepted")
		}
	})
}

func TestCompactRequiresVerifiedMatchmakingServiceBeforeDatabase(t *testing.T) {
	req := validCompactionRequest()
	var service *Service
	_, err := service.Compact(context.Background(), req)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unverified compact request reached provider: err=%v", err)
	}

	bad := validCompactionRequest()
	bad.CompactionAuthorizedAt = timestamppb.New(bad.AggregateCompletedAt.AsTime().Add(matchSquadRetention - time.Second))
	hash, err := principal.RequestHash(bad)
	if err != nil {
		t.Fatal(err)
	}
	verified := principal.Principal{
		Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "voice",
		RPC: matchsquadprincipal.CompactMethod, RequestID: bad.GetOperationId(), RequestHash: hash,
	}
	_, err = service.Compact(principal.WithVerified(context.Background(), verified), bad)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("early aggregate authorization was not rejected before database: err=%v", err)
	}
}

func TestValidateCreateRequiresCanonicalManifestAndBoundChatReceipt(t *testing.T) {
	request := validCreateRequest()
	_, _, participants, _, _, _, err := validateCreate(request)
	if err != nil || len(participants) != 2 || participants[0] >= participants[1] {
		t.Fatalf("canonical request rejected: participants=%v err=%v", participants, err)
	}

	t.Run("participant order is canonical UUID byte order", func(t *testing.T) {
		bad := validCreateRequest()
		bad.Participants[0], bad.Participants[1] = bad.Participants[1], bad.Participants[0]
		if _, _, _, _, _, _, err := validateCreate(bad); err == nil {
			t.Fatal("out-of-order roster accepted")
		}
	})
	t.Run("duplicate participants are rejected", func(t *testing.T) {
		bad := validCreateRequest()
		bad.Participants[1].ProfileId = bad.Participants[0].ProfileId
		if _, _, _, _, _, _, err := validateCreate(bad); err == nil {
			t.Fatal("duplicate roster member accepted")
		}
	})
	t.Run("manifest mismatch is rejected", func(t *testing.T) {
		bad := validCreateRequest()
		bad.ParticipantManifestSha256[0] ^= 0xff
		if _, _, _, _, _, _, err := validateCreate(bad); err == nil {
			t.Fatal("mismatched roster manifest accepted")
		}
	})
	t.Run("Chat receipt must bind the same match and roster", func(t *testing.T) {
		bad := validCreateRequest()
		bad.ChatCreationReceipt.MatchId = "00000000-0000-4000-8000-000000000099"
		if _, _, _, _, _, _, err := validateCreate(bad); err == nil {
			t.Fatal("unrelated Chat creation receipt accepted")
		}
	})
}
