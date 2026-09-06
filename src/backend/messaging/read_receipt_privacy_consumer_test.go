package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func TestReceiptOptOutProfileIDAcceptsOnlyExplicitFalse(t *testing.T) {
	profileID := uuid.New()
	encode := func(changed string) []byte {
		b, err := proto.Marshal(&eventsv1.UserStreamEvent{Payload: &eventsv1.UserStreamEvent_SettingsChanged{SettingsChanged: &eventsv1.SettingsChanged{ProfileId: profileID.String(), ChangedKeysJson: changed}}})
		require.NoError(t, err)
		return b
	}
	got, ok := receiptOptOutProfileID(encode(`[{"key":"show_read_receipts","value":false}]`))
	require.True(t, ok)
	require.Equal(t, profileID, got)
	_, ok = receiptOptOutProfileID(encode(`[{"key":"show_read_receipts","value":true}]`))
	require.False(t, ok)
	_, ok = receiptOptOutProfileID(encode(`[{"key":"allow_dm","value":false}]`))
	require.False(t, ok)
}
