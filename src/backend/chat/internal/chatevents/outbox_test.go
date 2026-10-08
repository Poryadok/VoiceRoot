package chatevents

import (
	"testing"
	"time"
)

func TestChatDeletedOutboxBackoffIsBoundedAndKeepsRetrying(t *testing.T) {
	cases := []struct {
		attempt int64
		want    time.Duration
	}{
		{attempt: 0, want: time.Second},
		{attempt: 1, want: time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 7, want: time.Minute},
		{attempt: 100, want: time.Minute},
	}
	for _, tc := range cases {
		if got := chatDeletedOutboxBackoff(tc.attempt); got != tc.want {
			t.Errorf("backoff at attempt %d = %s, want %s", tc.attempt, got, tc.want)
		}
	}
}
