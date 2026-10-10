package main

import "testing"

func TestOwnedReadonlyClosedSubjectPayload(t *testing.T) {
	for _, subject := range []string{"$JS.API.INFO", "$JS.API.STREAM.INFO.events", "$JS.API.CONSUMER.INFO.events.worker"} {
		if ownedReadPayload(subject, nil) != nil {
			t.Fatal("valid fixed READ refused", subject)
		}
		if ownedReadPayload(subject, []byte(`{}`)) == nil {
			t.Fatal("INFO payload accepted")
		}
	}
	if ownedReadPayload("$JS.API.STREAM.MSG.GET.events", []byte(`{"seq":1}`)) != nil {
		t.Fatal("exact GET refused")
	}
	for _, raw := range []string{`{"seq":0}`, `{"seq":-1}`, `{"seq":1,"seq":2}`, `{"seq":1,"extra":true}`, `{"seq":"1"}`, `{"seq":9223372036854775808}`, `{"seq":1} {}`} {
		if ownedReadPayload("$JS.API.STREAM.MSG.GET.events", []byte(raw)) == nil {
			t.Fatal("invalid GET admitted", raw)
		}
	}
	for _, subject := range []string{"$JS.API.STREAM.UPDATE.events", "$JS.API.CONSUMER.CREATE.events.worker", "$JS.API.STREAM.MSG.DELETE.events", "$JS.API.STREAM.PURGE.events", "$JS.API.STREAM.INFO.*", "$JS.API.STREAM.INFO.events.other", "nats://other:4222"} {
		if ownedReadPayload(subject, nil) == nil {
			t.Fatal("unbound/write endpoint admitted", subject)
		}
	}
}
