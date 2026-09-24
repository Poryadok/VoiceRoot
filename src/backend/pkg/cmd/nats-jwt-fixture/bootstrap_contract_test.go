package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCentralBootstrapPreprovisionsEveryFixedConsumer(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	jobs := []string{"realtime", "notification", "search", "analytics-chat"}
	counts := map[string]int{}
	all := ""
	for _, job := range jobs {
		contents, err := os.ReadFile(filepath.Join(root, "deploy", "templates", "nats-"+job+"-bootstrap.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var config struct {
			Data map[string]string `yaml:"data"`
		}
		if err := yaml.Unmarshal(contents, &config); err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(config.Data["bootstrap.sh"], "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "consumer ") || strings.HasPrefix(line, "consumer_filters ") || strings.HasPrefix(line, "pull_consumer ") {
				fields := strings.Fields(line)
				key := fields[1] + "/" + fields[2]
				counts[key]++
			}
		}
		all += config.Data["bootstrap.sh"] + "\n"
	}
	for key, count := range counts {
		if count != 1 {
			t.Errorf("%s appears %d times, want exactly once", key, count)
		}
	}
	if len(counts) != 40 {
		t.Errorf("central bootstrap defines %d unique durables, want 40", len(counts))
	}
	for _, line := range []string{
		"consumer message_events chat_message_activity message.sent _INBOX.voice.chat.chat_message_activity all chat_message_activity",
		"consumer message_events messaging_delivery_ack message.delivery_ack _INBOX.voice.messaging.messaging_delivery_ack all messaging_delivery_ack",
		"consumer user_events messaging_receipt_privacy user.settings_changed _INBOX.voice.messaging.messaging_receipt_privacy all messaging_receipt_privacy",
		"consumer message_events bot_message_events message.sent _INBOX.voice.bot.bot_message_events all ''",
		"consumer_filters story_events matchmaking_story_lfp_v2 _INBOX.voice.matchmaking.matchmaking_story_lfp_v2 all story.lfp_created story.lfp_response",
		"consumer_filters subscription_events space_subscription_entitlement _INBOX.voice.space.space_subscription_entitlement new subscription.space_pro_started subscription.space_pro_expired",
		"pull_consumer user_events user-account-deletion-v1 user.account_deleted all",
		"consumer subscription_events auth_subscription_tier 'subscription.>' _INBOX.voice.auth.subscription_tier new ''",
	} {
		if !strings.Contains(all, "\n"+line+"\n") {
			t.Errorf("missing exact consumer contract: %s", line)
		}
	}
}

func TestCanonicalBootstrapACLMatchesConsumers(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	acl, err := loadACL(filepath.Join(root, "deploy", "nats", "acl-intent.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range []string{"realtime", "notification", "search", "analytics-chat"} {
		contents, err := os.ReadFile(filepath.Join(root, "deploy", "templates", "nats-"+job+"-bootstrap.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var config struct {
			Data map[string]string `yaml:"data"`
		}
		if err := yaml.Unmarshal(contents, &config); err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(config.Data["bootstrap.sh"], "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) < 4 || (fields[0] != "consumer" && fields[0] != "consumer_filters" && fields[0] != "pull_consumer") {
				continue
			}
			stream, durable := fields[1], fields[2]
			service, delivery := "", ""
			if fields[0] == "pull_consumer" {
				service = "user"
			} else {
				index := 4
				if fields[0] == "consumer_filters" {
					index = 3
				}
				delivery = fields[index]
				parts := strings.Split(delivery, ".")
				if len(parts) < 4 {
					t.Fatalf("bad delivery subject in %s: %s", job, line)
				}
				service = parts[2]
				if service == "realtime1" {
					service = "realtime"
				}
			}
			grant, ok := acl.Services[service]
			if !ok {
				t.Fatalf("%s/%s has no ACL owner %s", stream, durable, service)
			}
			for _, subject := range []string{"$JS.API.CONSUMER.INFO." + stream + "." + durable, "$JS.ACK." + stream + "." + durable + ".>"} {
				if !contains(grant.Publish, subject) {
					t.Errorf("%s missing %s", service, subject)
				}
			}
			for _, subject := range []string{"$JS.API.CONSUMER.INFO." + stream + "." + durable, "$JS.API.CONSUMER.CREATE." + stream + "." + durable} {
				if !contains(acl.Bootstrap.Publish, subject) {
					t.Errorf("bootstrap missing %s", subject)
				}
			}
			if delivery != "" && !contains(grant.Subscribe, delivery) {
				t.Errorf("%s missing exact delivery %s", service, delivery)
			}
		}
	}
	for _, tc := range []struct{ service, stream, durable, delivery string }{
		{"chat", "message_events", "chat_message_activity", "_INBOX.voice.chat.chat_message_activity"},
		{"messaging", "message_events", "messaging_delivery_ack", "_INBOX.voice.messaging.messaging_delivery_ack"},
		{"messaging", "user_events", "messaging_receipt_privacy", "_INBOX.voice.messaging.messaging_receipt_privacy"},
		{"bot", "message_events", "bot_message_events", "_INBOX.voice.bot.bot_message_events"},
		{"matchmaking", "story_events", "matchmaking_story_lfp_v2", "_INBOX.voice.matchmaking.matchmaking_story_lfp_v2"},
		{"space", "subscription_events", "space_subscription_entitlement", "_INBOX.voice.space.space_subscription_entitlement"},
		{"user", "user_events", "user-account-deletion-v1", ""},
		{"auth", "subscription_events", "auth_subscription_tier", "_INBOX.voice.auth.subscription_tier"},
	} {
		grant := acl.Services[tc.service]
		for _, subject := range []string{"$JS.API.CONSUMER.INFO." + tc.stream + "." + tc.durable, "$JS.ACK." + tc.stream + "." + tc.durable + ".>"} {
			if !contains(grant.Publish, subject) {
				t.Errorf("%s missing %s", tc.service, subject)
			}
		}
		if tc.delivery != "" && !contains(grant.Subscribe, tc.delivery) {
			t.Errorf("%s missing delivery %s", tc.service, tc.delivery)
		}
		if !contains(acl.Bootstrap.Publish, "$JS.API.CONSUMER.CREATE."+tc.stream+"."+tc.durable) {
			t.Errorf("bootstrap cannot create %s/%s", tc.stream, tc.durable)
		}
	}
}

func contains(subjects []string, subject string) bool {
	for _, candidate := range subjects {
		if candidate == subject {
			return true
		}
	}
	return false
}
