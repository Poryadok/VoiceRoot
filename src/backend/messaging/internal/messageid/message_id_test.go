package messageid

import (
	"testing"
)

func TestNewMessageID_StringMonotonic(t *testing.T) {
	// google/uuid NewV7 documents time-ordering; message IDs must preserve that for cursor/history ordering.
	const n = 10000
	prev, err := NewMessageID()
	if err != nil {
		t.Fatalf("NewMessageID: %v", err)
	}
	prevS := prev.String()
	for i := 0; i < n; i++ {
		cur, err := NewMessageID()
		if err != nil {
			t.Fatalf("NewMessageID: %v", err)
		}
		s := cur.String()
		if s <= prevS {
			t.Fatalf("monotonicity failed at #%d: %q not after %q", i, s, prevS)
		}
		prevS = s
	}
}
