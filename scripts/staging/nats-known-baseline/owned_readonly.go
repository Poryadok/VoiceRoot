package main

// A request transport, never an authority or an isolated-store Actor. The root
// owner must hold and bracket an exact Pod-bound loopback forward and enforced
// HUB-only isolation. No caller URL, reconnect, discovery or mutation API.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/nats-io/nats.go"
	"io"
	"os"
	"regexp"
	"runtime"
	"time"
)

var ownedReadSubject = regexp.MustCompile(`^\$JS\.API\.(INFO|STREAM\.INFO\.[A-Za-z0-9_-]{1,255}|CONSUMER\.INFO\.[A-Za-z0-9_-]{1,255}\.[A-Za-z0-9_-]{1,255}|STREAM\.MSG\.GET\.[A-Za-z0-9_-]{1,255})$`)
var ownedReadError = errors.New("owned_readonly_request_refused")

func ownedReadPayload(subject string, raw []byte) error {
	if len(raw) > 128 || !ownedReadSubject.MatchString(subject) {
		return ownedReadError
	}
	if bytes.Contains([]byte(subject), []byte(".MSG.GET.")) {
		var row struct {
			Sequence uint64 `json:"seq"`
		}
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if d.Decode(&row) != nil || d.Decode(new(any)) != io.EOF || row.Sequence == 0 || row.Sequence > 1<<63-1 {
			return ownedReadError
		}
		// Require the exact closed payload, including duplicate-key rejection.
		canonical, _ := json.Marshal(row)
		var generic map[string]json.RawMessage
		if json.Unmarshal(raw, &generic) != nil || len(generic) != 1 || generic["seq"] == nil {
			return ownedReadError
		}
		var compact bytes.Buffer
		if json.Compact(&compact, raw) != nil || !bytes.Equal(compact.Bytes(), canonical) {
			return ownedReadError
		}
	} else if len(raw) != 0 {
		return ownedReadError
	}
	return nil
}

func ownedReadonly(args []string, input io.Reader, output io.Writer) error {
	if runtime.GOOS != "linux" || os.Geteuid() != 65532 || len(args) != 1 {
		return ownedReadError
	}
	raw, e := io.ReadAll(io.LimitReader(input, 129))
	if e != nil || ownedReadPayload(args[0], raw) != nil {
		return ownedReadError
	}
	if _, e = privateInput("bootstrap.creds"); e != nil {
		return ownedReadError
	}
	// Fixed root-controlled port; never use NATS_URL, INFO connect_urls, redirect
	// or a pod/service name supplied by the caller. Root binds this forward.
	nc, e := nats.Connect("nats://127.0.0.1:42221", nats.UserCredentials("/inputs/bootstrap.creds"),
		nats.CustomInboxPrefix("_INBOX.voice.bootstrap.reply"), nats.Timeout(2*time.Second), nats.NoReconnect(),
		nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {}))
	if e != nil {
		return ownedReadError
	}
	defer nc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	reply, e := nc.RequestWithContext(ctx, args[0], raw)
	if e != nil || len(reply.Data) == 0 || len(reply.Data) > 8<<20 || !json.Valid(reply.Data) ||
		nc.ConnectedServerVersion() != "2.12.12" || nc.ConnectedServerId() == "" || nc.ConnectedUrl() != "nats://127.0.0.1:42221" {
		return ownedReadError
	}
	envelope := struct {
		Schema        string          `json:"schema"`
		ServerID      string          `json:"server_id"`
		ServerVersion string          `json:"server_version"`
		API           string          `json:"api"`
		Reply         json.RawMessage `json:"reply"`
	}{"voice-owned-readonly-transport-v1", nc.ConnectedServerId(), nc.ConnectedServerVersion(), args[0], reply.Data}
	if e = json.NewEncoder(output).Encode(envelope); e != nil {
		return ownedReadError
	}
	return nil
}
