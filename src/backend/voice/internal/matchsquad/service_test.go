package matchsquad

import (
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
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
	t.Run("Chat receipt must bind the same match operation and roster", func(t *testing.T) {
		bad := validCreateRequest()
		bad.ChatCreationReceipt.OperationId = "00000000-0000-4000-8000-000000000099"
		if _, _, _, _, _, _, err := validateCreate(bad); err == nil {
			t.Fatal("unrelated Chat creation receipt accepted")
		}
	})
}
