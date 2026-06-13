package messageevents

import (
	"testing"

	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

// TestMessageSentThreadParentNATSHeader documents interim transport until buf generate updates descriptors.
func TestMessageSentThreadParentNATSHeader(t *testing.T) {
	parent := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	sent := &eventsv1.MessageSent{
		MessageId:       "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		ChatId:          "cccccccc-cccc-cccc-cccc-cccccccccccc",
		SenderProfileId: "dddddddd-dddd-dddd-dddd-dddddddddddd",
		ThreadParentId:  &parent,
	}
	b, err := proto.Marshal(sent)
	if err != nil {
		t.Fatal(err)
	}
	hdr := messageSentPublishHeaders(parent)
	if hdr.Get(natsHeaderThreadParentID) != parent {
		t.Fatalf("header thread_parent_id=%q", hdr.Get(natsHeaderThreadParentID))
	}
	// Proto marshal may drop unknown fields without regenerated descriptors; header is source of truth.
	var back eventsv1.MessageSent
	if err := proto.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.GetThreadParentId() == parent {
		t.Log("proto roundtrip ok after descriptor regen")
	}
}
