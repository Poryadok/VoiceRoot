package main

import (
	"testing"
	"voice/backend/pkg/notificationrouting"
)

func TestRoutingReplyGrantIsExactAndNotificationOwned(t *testing.T) {
	for _, tc := range []struct {
		owner, subject string
		want           bool
	}{
		{"notification", notificationrouting.ReplyPrefix + ">", true},
		{"messaging", notificationrouting.ReplyPrefix + ">", false},
		{"realtime", notificationrouting.ReplyPrefix + ">", false},
		{"notification", "_INBOX.voice.realtime.>", false},
		{"notification", "_INBOX.voice.realtime.notification.*", false},
		{"notification", "_INBOX.voice.realtime.notification.adjacent.>", false},
	} {
		got := validGrant(grant{Publish: []string{tc.subject}, NoResponse: true}, tc.owner, false)
		if got != tc.want {
			t.Errorf("owner=%s subject=%s got %v want %v", tc.owner, tc.subject, got, tc.want)
		}
	}
}
