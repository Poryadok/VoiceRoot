package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/messaging/internal/store"
)

type privacyStoreStub struct{}

func (privacyStoreStub) ReadReceiptChatIDsForProfile(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (privacyStoreStub) ClearPublicReadReceiptsForProfile(context.Context, uuid.UUID, []uuid.UUID) ([]store.PublicReadReceipt, error) {
	return nil, nil
}

type privacyTargetsStub struct{}

func (privacyTargetsStub) DMReceiptVisibilityTargets(context.Context, uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	return nil, nil
}

type privacyPublisherStub struct{}

func (privacyPublisherStub) PublishReadReceiptRevoked(context.Context, string, string, string, string) error {
	return nil
}

func TestReceiptPrivacyRequiresExactPreprovisionedDurable(t *testing.T) {
	s := startMessagingJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: privacySettingsStreamName, Subjects: []string{"user.settings_changed"}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	require.Error(t, validateReceiptPrivacyDurable(js))
	config := receiptPrivacyConsumerConfig()
	config.DeliverSubject = "_INBOX.wrong"
	_, err = js.AddConsumer(privacySettingsStreamName, config)
	require.NoError(t, err)
	require.Error(t, validateReceiptPrivacyDurable(js))
}

func TestReceiptPrivacyBindsPreprovisionedQueue(t *testing.T) {
	s := startMessagingJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: privacySettingsStreamName, Subjects: []string{privacySettingsSubject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	_, err = js.AddConsumer(privacySettingsStreamName, receiptPrivacyConsumerConfig())
	require.NoError(t, err)
	subA, err := subscribeReceiptPrivacy(context.Background(), js, privacyStoreStub{}, privacyTargetsStub{}, privacyPublisherStub{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subA.Unsubscribe() })
	subB, err := subscribeReceiptPrivacy(context.Background(), js, privacyStoreStub{}, privacyTargetsStub{}, privacyPublisherStub{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subB.Unsubscribe() })
}

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

func TestReceiptOptOutProfileIDAcceptsTypedKeysWithLegacyFalseValue(t *testing.T) {
	profileID := uuid.New()
	b, err := proto.Marshal(&eventsv1.UserStreamEvent{Payload: &eventsv1.UserStreamEvent_SettingsChanged{SettingsChanged: &eventsv1.SettingsChanged{
		ProfileId:       profileID.String(),
		ChangedKeys:     []string{"show_read_receipts"},
		ChangedKeysJson: `[{"key":"show_read_receipts","value":false}]`,
	}}})
	require.NoError(t, err)

	got, ok := receiptOptOutProfileID(b)
	require.True(t, ok)
	require.Equal(t, profileID, got)
}
