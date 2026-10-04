package main

import (
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// Exact selected baseline: INFO-only APIs, never NAMES/LIST permissions.
var streamNames = strings.Fields("analytics_events bot_events chat_events file_events matchmaking_events message_events moderation_events role_events social_events story_events subscription_auth_quarantine subscription_events user_events user_profile_projection voice_events")
var durablePairs = strings.Fields(`analytics_events/analytics_v2_telemetry bot_events/analytics_v2_bot chat_events/analytics_v2_chat chat_events/rt_realtime1_chat chat_events/search-indexer-chat-v1 file_events/analytics_v2_file matchmaking_events/analytics_v2_mm matchmaking_events/notif_mm matchmaking_events/rt_realtime1_matchmaking message_events/analytics_v2_msg message_events/bot_message_events message_events/chat_message_activity message_events/messaging_delivery_ack message_events/notif_msg_v2 message_events/rt_realtime1_msg message_events/search-indexer-message-v1 moderation_events/analytics_v2_moderation moderation_events/notif_mod role_events/analytics_v2_role role_events/rt_realtime1_role social_events/analytics_v2_social social_events/chat_friend_accepted social_events/notif_social social_events/rt_realtime1_friend_request social_events/rt_realtime1_social story_events/analytics_v2_story story_events/matchmaking_story_lfp_v2 story_events/notif_story subscription_events/analytics_v2_subscription subscription_events/auth_subscription_tier subscription_events/notif_subscription subscription_events/space_subscription_entitlement user_events/analytics_v2_user user_events/chat_account_deleted user_events/messaging_receipt_privacy user_events/rt_realtime1_user user_events/search-indexer-user-v1 user_profile_projection/search-user-profile-projection-v1 voice_events/analytics_v2_voice voice_events/notif_voice voice_events/rt_realtime1_voice user_events/user-account-deletion-v1`)

type streamCensus struct {
	Name      string `json:"name"`
	ConfigSHA string `json:"config_sha256"`
	Messages  uint64 `json:"messages"`
	First     uint64 `json:"first_seq"`
	Last      uint64 `json:"last_seq"`
	Consumers int    `json:"consumers"`
}
type consumerCensus struct {
	Stream            string `json:"stream"`
	Name              string `json:"name"`
	ConfigSHA         string `json:"config_sha256"`
	AckConsumer       uint64 `json:"ack_consumer"`
	AckStream         uint64 `json:"ack_stream"`
	DeliveredConsumer uint64 `json:"delivered_consumer"`
	DeliveredStream   uint64 `json:"delivered_stream"`
	AckPending        int    `json:"ack_pending"`
	Pending           uint64 `json:"pending"`
}
type census struct {
	Schema    string           `json:"schema"`
	Streams   []streamCensus   `json:"streams"`
	Consumers []consumerCensus `json:"consumers"`
}

func captureCensus(nc *nats.Conn) (census, error) {
	js, e := nc.JetStream(nats.MaxWait(2 * time.Second))
	if e != nil {
		return census{}, fixedError("census_connection_failed")
	}
	account, e := js.AccountInfo()
	if e != nil || account.Streams != len(streamNames) || account.Consumers != len(durablePairs) {
		return census{}, fixedError("census_account_contract_failed")
	}
	c := census{Schema: "known-nats-census-v1"}
	expected := map[string]int{}
	for _, pair := range durablePairs {
		expected[strings.Split(pair, "/")[0]]++
	}
	for _, name := range streamNames {
		si, e := js.StreamInfo(name)
		if e != nil || si.Config.Name != name || si.State.Consumers != expected[name] {
			return census{}, fixedError("census_stream_contract_failed")
		}
		c.Streams = append(c.Streams, streamCensus{Name: name, ConfigSHA: canonicalDigest(si.Config), Messages: si.State.Msgs, First: si.State.FirstSeq, Last: si.State.LastSeq, Consumers: si.State.Consumers})
	}
	for _, pair := range durablePairs {
		parts := strings.Split(pair, "/")
		ci, e := js.ConsumerInfo(parts[0], parts[1])
		if e != nil || ci.Name != parts[1] || ci.Config.Durable != parts[1] {
			return census{}, fixedError("census_consumer_contract_failed")
		}
		c.Consumers = append(c.Consumers, consumerCensus{Stream: parts[0], Name: parts[1], ConfigSHA: canonicalDigest(ci.Config), AckConsumer: ci.AckFloor.Consumer, AckStream: ci.AckFloor.Stream, DeliveredConsumer: ci.Delivered.Consumer, DeliveredStream: ci.Delivered.Stream, AckPending: ci.NumAckPending, Pending: ci.NumPending})
	}
	return c, nil
}
