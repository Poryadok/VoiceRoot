package messageid

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewMessageID_VersionAndVariant(t *testing.T) {
	t.Parallel()
	u, err := NewMessageID()
	if err != nil {
		t.Fatalf("NewMessageID: %v", err)
	}
	if u.Version() != 7 {
		t.Fatalf("expected version 7, got %d", u.Version())
	}
	if u.Variant() != uuid.RFC4122 {
		t.Fatalf("expected RFC4122 variant, got %v", u.Variant())
	}
}

func TestNewMessageID_UnixMillisEmbedded(t *testing.T) {
	t.Parallel()
	before := time.Now().UnixMilli()
	u, err := NewMessageID()
	if err != nil {
		t.Fatalf("NewMessageID: %v", err)
	}
	after := time.Now().UnixMilli()

	embedded := unixMillisFromV7(u)
	if embedded < before-1 || embedded > after+1 {
		t.Fatalf("embedded unix_ts_ms %d not in [%d, %d]", embedded, before-1, after+1)
	}
}

// unixMillisFromV7 returns the 48-bit unix_ts_ms field (RFC 9562) from bytes 0–5.
func unixMillisFromV7(u uuid.UUID) int64 {
	return int64(u[0])<<40 |
		int64(u[1])<<32 |
		int64(u[2])<<24 |
		int64(u[3])<<16 |
		int64(u[4])<<8 |
		int64(u[5])
}

func TestNewMessageID_Uniqueness(t *testing.T) {
	t.Parallel()
	const n = 256
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		u, err := NewMessageID()
		if err != nil {
			t.Fatalf("NewMessageID: %v", err)
		}
		s := u.String()
		if _, ok := seen[s]; ok {
			t.Fatalf("duplicate id at iteration %d: %s", i, s)
		}
		seen[s] = struct{}{}
	}
}

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
