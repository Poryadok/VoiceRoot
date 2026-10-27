package main

import (
	"testing"
	"voice/backend/pkg/notificationrouting"
)

func TestRoutingReplyGrantCannotMoveToAnotherActor(t *testing.T) {
	for _, owner := range []string{"notification", "messaging", "realtime", "bootstrap"} {
		acl := fixtureACL()
		if owner == "bootstrap" {
			acl.Bootstrap.Publish = append(acl.Bootstrap.Publish, notificationrouting.ReplyPrefix+">")
		} else {
			g := acl.Services[owner]
			g.Publish = append(g.Publish, notificationrouting.ReplyPrefix+">")
			acl.Services[owner] = g
		}
		err := validateACL(acl)
		if (err == nil) != (owner == "notification") {
			t.Errorf("owner=%s got %v", owner, err)
		}
	}
}
