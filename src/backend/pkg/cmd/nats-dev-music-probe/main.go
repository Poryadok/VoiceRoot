// nats-dev-music-probe performs one read-only Messaging consumer INFO request
// through a root-created loopback kubectl port-forward. It never publishes,
// acknowledges, creates, or updates JetStream state.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	infoSubject = "$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata"
	messagingInboxPrefix = "_INBOX.voice.messaging.music_preflight"
	bootstrapInboxPrefix = "_INBOX.voice.bootstrap.reply.music_preflight"
)

type result struct {
	Schema                  string `json:"schema"`
	Authenticated           bool   `json:"broker_authenticated"`
	Outcome                 string `json:"outcome"`
	ConsumerInfoAuthorized bool   `json:"consumer_info_authorized"`
	ConsumerExists          bool   `json:"consumer_exists"`
	ConsumerMatches          bool   `json:"consumer_matches"`
}

type infoResponse struct {
	Stream string `json:"stream_name"`
	Name   string `json:"name"`
	Config struct {
		DurableName   string  `json:"durable_name"`
		FilterSubject string  `json:"filter_subject"`
		DeliverSubject string `json:"deliver_subject"`
		DeliverGroup  string  `json:"deliver_group"`
		AckPolicy     string  `json:"ack_policy"`
		DeliverPolicy string  `json:"deliver_policy"`
		MaxDeliver    *int64  `json:"max_deliver"`
		Backoff       []int64 `json:"backoff"`
	} `json:"config"`
	Error  *struct {
		Code    int `json:"code"`
		ErrCode int `json:"err_code"`
	} `json:"error"`
}

func main() {
	port := flag.Int("loopback-port", 0, "root-owned local port-forward only")
	creds := flag.String("credentials-file", "", "root-private temporary credentials file")
	role := flag.String("actor-role", "messaging", "fixed messaging or bootstrap actor contract")
	flag.Parse()
	if flag.NArg() != 0 || *port < 1 || *port > 65535 || *creds == "" ||
		(*role != "messaging" && *role != "bootstrap") {
		refuse()
	}
	if err := probe(*port, *creds, *role); err != nil {
		refuse()
	}
}

func probe(port int, credentials, role string) error {
	inbox, err := inboxPrefixForRole(role)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	server := "nats://127.0.0.1:" + strconv.Itoa(port)
	conn, err := nats.Connect(server,
		nats.UserCredentials(credentials),
		nats.CustomInboxPrefix(inbox),
		nats.Timeout(5*time.Second),
		nats.NoReconnect(),
		nats.Name("voice-dev-music-readonly-preflight"),
	)
	if err != nil {
		return errors.New("broker_authentication_failed")
	}
	defer conn.Close()
	request, err := json.Marshal(map[string]string{})
	if err != nil {
		return errors.New("request_encoding_failed")
	}
	message, err := conn.RequestWithContext(ctx, infoSubject, request)
	if err != nil || message == nil || len(message.Data) == 0 || len(message.Data) > 65536 {
		return json.NewEncoder(os.Stdout).Encode(result{Schema: "voice-dev-music-broker-preflight-v1",
			Authenticated: true, Outcome: "no_response"})
	}
	out, err := classify(message.Data)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func inboxPrefixForRole(role string) (string, error) {
	switch role {
	case "messaging":
		return messagingInboxPrefix, nil
	case "bootstrap":
		return bootstrapInboxPrefix, nil
	default:
		return "", errors.New("actor_role_refused")
	}
}

func classify(raw []byte) (result, error) {
	var response infoResponse
	if len(raw) == 0 || len(raw) > 65536 || json.Unmarshal(raw, &response) != nil {
		return result{}, errors.New("consumer_info_response_invalid")
	}
	out := result{Schema: "voice-dev-music-broker-preflight-v1", Authenticated: true}
	if response.Error != nil {
		// JetStream's stable not-found shape proves the request reached the
		// authorized API handler. Other API errors are deliberately opaque.
		if response.Error.Code != 404 || response.Error.ErrCode != 10014 {
			return result{}, errors.New("consumer_info_probe_refused")
		}
		out.ConsumerInfoAuthorized = true
		out.Outcome = "authorized_absent"
		return out, nil
	}
	if response.Stream != "file_events" || response.Name != "messaging_music_metadata" {
		return result{}, errors.New("consumer_info_identity_mismatch")
	}
	out.ConsumerInfoAuthorized = true
	out.ConsumerExists = true
	out.Outcome = "authorized_present"
	maxDeliver := int64(-1)
	if response.Config.MaxDeliver != nil {
		maxDeliver = *response.Config.MaxDeliver
	}
	out.ConsumerMatches = response.Config.DurableName == "messaging_music_metadata" &&
		response.Config.FilterSubject == "file.processed" &&
		response.Config.DeliverSubject == "_INBOX.voice.messaging.messaging_music_metadata" &&
		response.Config.DeliverGroup == "messaging_music_metadata" &&
		response.Config.AckPolicy == "explicit" && response.Config.DeliverPolicy == "new" &&
		maxDeliver == -1 && len(response.Config.Backoff) == 0
	return out, nil
}

func refuse() {
	fmt.Fprintln(os.Stderr, "NATS Music read-only probe refused")
	os.Exit(1)
}

