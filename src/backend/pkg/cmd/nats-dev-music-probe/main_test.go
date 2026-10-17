package main

import "testing"

func TestClassifyMusicInfo(t *testing.T) {
	tests := []struct {
		name string
		body string
		want result
		fail bool
	}{
		{"present exact", `{"stream_name":"file_events","name":"messaging_music_metadata","config":{"durable_name":"messaging_music_metadata","filter_subject":"file.processed","deliver_subject":"_INBOX.voice.messaging.messaging_music_metadata","deliver_group":"messaging_music_metadata","ack_policy":"explicit","deliver_policy":"new","max_deliver":-1,"backoff":[]}}`, result{"voice-dev-music-broker-preflight-v1", true, "authorized_present", true, true, true}, false},
		{"present mismatch", `{"stream_name":"file_events","name":"messaging_music_metadata","config":{"durable_name":"messaging_music_metadata","filter_subject":"file.other","deliver_subject":"_INBOX.voice.messaging.messaging_music_metadata","deliver_group":"messaging_music_metadata","ack_policy":"explicit","deliver_policy":"new","max_deliver":-1,"backoff":[]}}`, result{"voice-dev-music-broker-preflight-v1", true, "authorized_present", true, true, false}, false},
		{"absent", `{"error":{"code":404,"err_code":10014,"description":"consumer not found"}}`, result{"voice-dev-music-broker-preflight-v1", true, "authorized_absent", true, false, false}, false},
		{"wrong consumer", `{"stream_name":"file_events","name":"other"}`, result{}, true},
		{"other api error", `{"error":{"code":403,"err_code":10000}}`, result{}, true},
		{"malformed", `{`, result{}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := classify([]byte(test.body))
			if (err != nil) != test.fail {
				t.Fatalf("classify error = %v, want fail %v", err, test.fail)
			}
			if err == nil && got != test.want {
				t.Fatalf("classify = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestProbeUsesOnlyFixedRoleSpecificReplyPrefixes(t *testing.T) {
	if infoSubject != "$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata" {
		t.Fatalf("unexpected request subject %q", infoSubject)
	}
	for role, want := range map[string]string{
		"messaging": messagingInboxPrefix,
		"bootstrap": bootstrapInboxPrefix,
	} {
		got, err := inboxPrefixForRole(role)
		if err != nil || got != want {
			t.Fatalf("inboxPrefixForRole(%q) = %q, %v; want %q", role, got, err, want)
		}
	}
	if _, err := inboxPrefixForRole("other"); err == nil {
		t.Fatal("inboxPrefixForRole accepted an unknown actor role")
	}
}
