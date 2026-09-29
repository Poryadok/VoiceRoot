package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const liveACLProofTimeout = 3 * time.Second

var (
	liveACLProofGenerationPattern = regexp.MustCompile(`^r[0-9]{8}[a-z0-9]{0,8}$`)
	liveACLProofSHAPattern        = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// liveACLProofFailure deliberately carries only a fixed code: upstream NATS
// errors can include credential material or message payloads.
type liveACLProofFailure struct{ code string }

func (f *liveACLProofFailure) Error() string { return f.code }
func liveACLFail(code string) error          { return &liveACLProofFailure{code: code} }

func validateLiveACLProofURL(raw, wantHost string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "nats" && u.Host == wantHost && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}

func runNATSLiveACLProofFromEnv() error {
	instanceID := strings.TrimSpace(os.Getenv("REALTIME_INSTANCE_ID"))
	if instanceID != "realtime-1" || !validateLiveACLProofURL(os.Getenv("NATS_URL"), "127.0.0.1:4222") ||
		!validateLiveACLProofURL(os.Getenv("REALTIME_NATS_PROOF_URL"), "127.0.0.1:4223") {
		return liveACLFail("invalid_config")
	}
	hubURL := os.Getenv("REALTIME_NATS_HUB_URL")
	if !validateLiveACLProofURL(hubURL, "voice-nats:4222") && !validateLiveACLProofURL(hubURL, "voice-nats.voice-staging.svc.cluster.local:4222") {
		return liveACLFail("invalid_config")
	}
	credsFile := strings.TrimSpace(os.Getenv("REALTIME_NATS_HUB_CREDS_FILE"))
	if credsFile == "" {
		return liveACLFail("invalid_config")
	}
	quietErrors := nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, _ error) {})
	connectOpts := []nats.Option{nats.Timeout(liveACLProofTimeout), nats.MaxReconnects(0), nats.PermissionErrOnSubscribe(true), nats.CustomInboxPrefix("_INBOX.voice.realtime"), quietErrors}
	leafRT, err := nats.Connect(os.Getenv("NATS_URL"), connectOpts...)
	if err != nil {
		return liveACLFail("leaf_connect_failed")
	}
	defer leafRT.Close()
	hubOpts := append([]nats.Option{}, connectOpts...)
	hubOpts = append(hubOpts, nats.UserCredentials(credsFile))
	hubRT, err := nats.Connect(hubURL, hubOpts...)
	if err != nil {
		return liveACLFail("hub_connect_failed")
	}
	defer hubRT.Close()
	proofOpts := []nats.Option{nats.Timeout(liveACLProofTimeout), nats.MaxReconnects(0), nats.CustomInboxPrefix("_INBOX.voice.nats-proof"), quietErrors}
	proofNC, err := nats.Connect(os.Getenv("REALTIME_NATS_PROOF_URL"), proofOpts...)
	if err != nil {
		return liveACLFail("proof_connect_failed")
	}
	defer proofNC.Close()
	return runNATSLiveACLProof(leafRT, hubRT, proofNC, instanceID)
}

func runNATSLiveACLProof(leafRT, hubRT, proofNC *nats.Conn, instanceID string) (result error) {
	if instanceID != "realtime-1" || leafRT == nil || hubRT == nil || proofNC == nil {
		return liveACLFail("invalid_config")
	}
	proofJS, err := proofNC.JetStream(nats.MaxWait(liveACLProofTimeout))
	if err != nil {
		return liveACLFail("proof_jetstream_failed")
	}
	streamInfo, err := proofJS.StreamInfo(jsStreamSocialEvents)
	if err != nil {
		return liveACLFail("stream_info_failed")
	}
	if streamInfo.State.Msgs != 0 || streamInfo.State.LastSeq != 0 {
		return liveACLFail("stream_not_empty")
	}

	durable := friendRequestConsumerDurableName(instanceID)
	deliver := realtimeConsumerDeliverSubject(instanceID, "friend_request")
	for _, nc := range []*nats.Conn{leafRT, hubRT} {
		js, jsErr := nc.JetStream(nats.MaxWait(liveACLProofTimeout))
		if jsErr != nil {
			return liveACLFail("realtime_jetstream_failed")
		}
		if cfgErr := validateRealtimeConsumerConfig(js, jsStreamSocialEvents, durable, "social.friend_request", deliver); cfgErr != nil {
			return liveACLFail("consumer_info_failed")
		}
	}
	// The hub CLIENT connection enforces the signed Realtime user permissions.
	// Leaf delivery is tested separately because leaf ACK routing can bypass a
	// leaf-local publish permission check.
	if err := requireDeniedAdjacentSUB(hubRT, deliver+"_adjacent"); err != nil {
		return err
	}
	if err := requireDeniedConsumerCREATE(hubRT, "$JS.API.CONSUMER.CREATE.social_events."+durable); err != nil {
		return err
	}

	leafSub, err := leafRT.SubscribeSync(deliver)
	if err != nil {
		return liveACLFail("leaf_sub_failed")
	}
	defer leafSub.Unsubscribe()
	hubSub, err := hubRT.SubscribeSync(deliver)
	if err != nil {
		return liveACLFail("hub_sub_failed")
	}
	defer hubSub.Unsubscribe()
	if err := leafRT.FlushTimeout(liveACLProofTimeout); err != nil {
		return liveACLFail("leaf_sub_failed")
	}
	if err := hubRT.FlushTimeout(liveACLProofTimeout); err != nil {
		return liveACLFail("hub_sub_failed")
	}

	marker := make([]byte, 32)
	if _, err := rand.Read(marker); err != nil {
		return liveACLFail("random_failed")
	}
	// An error or timeout here leaves the PubAck sequence unknown. Never guess a
	// sequence or delete a message that may belong to another publisher.
	ack, err := proofJS.Publish("social.friend_request", marker)
	if err != nil {
		return liveACLFail("cleanup_uncertain")
	}
	if ack == nil || ack.Stream != jsStreamSocialEvents || ack.Sequence != 1 {
		return liveACLFail("cleanup_uncertain")
	}
	sequence := ack.Sequence
	defer func() {
		// Verify ownership before deleting exactly the acknowledged sequence.
		stored, getErr := proofJS.GetMsg(jsStreamSocialEvents, sequence)
		if getErr != nil || stored == nil || stored.Sequence != sequence || !bytes.Equal(stored.Data, marker) {
			result = liveACLFail("cleanup_uncertain")
			return
		}
		if err := proofJS.DeleteMsg(jsStreamSocialEvents, sequence); err != nil {
			result = liveACLFail("cleanup_uncertain")
			return
		}
		info, infoErr := proofJS.StreamInfo(jsStreamSocialEvents)
		if infoErr != nil || info.State.Msgs != 0 {
			result = liveACLFail("cleanup_uncertain")
		}
	}()

	leafMsg, err := leafSub.NextMsg(liveACLProofTimeout)
	if err != nil || !bytes.Equal(leafMsg.Data, marker) {
		return liveACLFail("leaf_delivery_failed")
	}
	leafMetadata, err := leafMsg.Metadata()
	if err != nil || leafMetadata.Stream != jsStreamSocialEvents || leafMetadata.Sequence.Stream != sequence || leafMetadata.Consumer != durable {
		return liveACLFail("leaf_delivery_failed")
	}
	hubMsg, err := hubSub.NextMsg(liveACLProofTimeout)
	if err != nil || !bytes.Equal(hubMsg.Data, marker) {
		return liveACLFail("hub_delivery_failed")
	}
	metadata, err := hubMsg.Metadata()
	if err != nil || metadata.Stream != jsStreamSocialEvents || metadata.Sequence.Stream != sequence || metadata.Consumer != durable {
		return liveACLFail("hub_delivery_failed")
	}
	if err := leafMsg.AckSync(nats.AckWait(liveACLProofTimeout)); err != nil {
		return liveACLFail("leaf_ack_failed")
	}
	if err := hubMsg.AckSync(nats.AckWait(liveACLProofTimeout)); err != nil {
		return liveACLFail("hub_ack_failed")
	}
	hubJS, err := hubRT.JetStream(nats.MaxWait(liveACLProofTimeout))
	if err != nil {
		return liveACLFail("ack_state_failed")
	}
	consumerInfo, err := hubJS.ConsumerInfo(jsStreamSocialEvents, durable)
	if err != nil || consumerInfo.AckFloor.Stream != sequence || consumerInfo.NumAckPending != 0 || consumerInfo.NumPending != 0 {
		return liveACLFail("ack_state_failed")
	}
	return nil
}

func requireDeniedAdjacentSUB(nc *nats.Conn, subject string) error {
	sub, err := nc.SubscribeSync(subject)
	if err != nil {
		if errors.Is(err, nats.ErrPermissionViolation) && strings.Contains(err.Error(), subject) {
			return nil
		}
		return liveACLFail("adjacent_sub_unproven")
	}
	defer sub.Unsubscribe()
	if err := nc.FlushTimeout(liveACLProofTimeout); err != nil {
		return liveACLFail("adjacent_sub_unproven")
	}
	_, err = sub.NextMsg(liveACLProofTimeout)
	if errors.Is(err, nats.ErrPermissionViolation) && strings.Contains(err.Error(), subject) {
		return nil
	}
	return liveACLFail("adjacent_sub_unproven")
}

func requireDeniedConsumerCREATE(nc *nats.Conn, subject string) error {
	previous := nc.ErrorHandler()
	denied := make(chan error, 4)
	nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		select {
		case denied <- err:
		default:
		}
	})
	defer nc.SetErrorHandler(previous)
	reply := nc.NewInbox()
	sub, err := nc.SubscribeSync(reply)
	if err != nil {
		return liveACLFail("consumer_create_unproven")
	}
	defer sub.Unsubscribe()
	if err := nc.FlushTimeout(liveACLProofTimeout); err != nil {
		return liveACLFail("consumer_create_unproven")
	}
	// Malformed JSON must yield an API error when publishing is allowed, while
	// a denied PUB produces the server's explicit async permission violation.
	if err := nc.PublishRequest(subject, reply, []byte("{")); err != nil {
		return liveACLFail("consumer_create_unproven")
	}
	if err := nc.FlushTimeout(liveACLProofTimeout); err != nil {
		return liveACLFail("consumer_create_unproven")
	}
	deadline := time.NewTimer(liveACLProofTimeout)
	defer deadline.Stop()
	response := make(chan struct{}, 1)
	go func() {
		if _, err := sub.NextMsg(liveACLProofTimeout); err == nil {
			response <- struct{}{}
		}
	}()
	for {
		select {
		case violation := <-denied:
			if errors.Is(violation, nats.ErrPermissionViolation) && strings.Contains(violation.Error(), subject) {
				return nil
			}
		case <-response:
			return liveACLFail("consumer_create_allowed")
		case <-deadline.C:
			return liveACLFail("consumer_create_unproven")
		}
	}
}

func liveACLProofResultLine(err error, generation, sha string) string {
	if err == nil {
		return fmt.Sprintf("NATS_LIVE_ACL_PROOF=PASS generation=%s acl_sha=%s", generation, sha)
	}
	var failure *liveACLProofFailure
	if errors.As(err, &failure) {
		return "NATS_LIVE_ACL_PROOF=FAIL code=" + failure.code
	}
	return "NATS_LIVE_ACL_PROOF=FAIL code=internal_error"
}

func runNATSLiveACLProofMain() int {
	generation := os.Getenv("REALTIME_NATS_PROOF_GENERATION")
	sha := os.Getenv("REALTIME_NATS_PROOF_ACL_SHA")
	if !liveACLProofGenerationPattern.MatchString(generation) || !liveACLProofSHAPattern.MatchString(sha) {
		fmt.Println(liveACLProofResultLine(liveACLFail("invalid_config"), "", ""))
		return 1
	}
	err := runNATSLiveACLProofFromEnv()
	fmt.Println(liveACLProofResultLine(err, generation, sha))
	if err != nil {
		return 1
	}
	return 0
}
