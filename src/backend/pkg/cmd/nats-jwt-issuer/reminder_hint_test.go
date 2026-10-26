package main

import "testing"

func TestReminderHintGrantHasOnePublisherAndNoDirectSubscriber(t *testing.T) {
	const subject = "notification.subscription_grace_reminder"
	for _, owner := range serviceNames {
		if got := validGrant(grant{Publish: []string{subject}, NoResponse: true}, owner, false); got != (owner == "notification") {
			t.Errorf("publisher %s=%v", owner, got)
		}
		if validGrant(grant{Subscribe: []string{subject}, NoResponse: true}, owner, false) {
			t.Errorf("direct reminder subscriber %s", owner)
		}
	}
	for _, bad := range []string{subject + ".forged", subject + ".>", "notification.*"} {
		if validGrant(grant{Publish: []string{bad}, NoResponse: true}, "notification", false) {
			t.Errorf("accepted adjacent subject %s", bad)
		}
	}
	acl, err := readPolicy(canonicalACL())
	if err != nil {
		t.Fatal(err)
	}
	for owner, g := range acl.Services {
		for _, p := range g.Publish {
			if p == subject && owner != "notification" {
				t.Fatal("wrong canonical publisher")
			}
		}
	}
}
