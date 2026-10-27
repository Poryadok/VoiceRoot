package notificationrouting

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"testing"
)

type requester func(context.Context, string, []byte) (*nats.Msg, error)

func (f requester) RequestWithContext(c context.Context, s string, b []byte) (*nats.Msg, error) {
	return f(c, s, b)
}

func TestRequestBindsExactSourceAndRejectsInvalidDecisions(t *testing.T) {
	source := []byte("exact source event")
	valid := Target{ProfileID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Type: "new_message"}
	for _, tc := range []struct {
		name            string
		event           []byte
		targets         []Target
		failed, wantErr bool
	}{
		{"allowed", source, []Target{valid}, false, false},
		{"empty", source, nil, false, false},
		{"other event", []byte("another event"), []Target{valid}, false, true},
		{"authority failure", source, []Target{valid}, true, true},
		{"duplicate", source, []Target{valid, valid}, false, true},
		{"bad identity", source, []Target{{ProfileID: "invalid", Type: "new_message"}}, false, true},
		{"unknown type", source, []Target{{ProfileID: valid.ProfileID, Type: "system"}}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := Encode(tc.event, tc.targets, tc.failed)
			require.NoError(t, err)
			got, err := Request(context.Background(), requester(func(_ context.Context, s string, b []byte) (*nats.Msg, error) {
				require.Equal(t, Subject, s)
				require.Equal(t, source, b)
				return &nats.Msg{Data: wire}, nil
			}), source)
			if tc.wantErr {
				require.Error(t, err)
				require.Empty(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.targets, got)
			}
		})
	}
}
