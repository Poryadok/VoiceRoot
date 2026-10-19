package chatcreation

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

func bindingFixture() *spacev1.SpaceChatCreationBinding {
	return &spacev1.SpaceChatCreationBinding{
		ProtocolVersion: 1, OperationId: "10000000-0000-4000-8000-000000000001",
		CreatorProfileId: "10000000-0000-4000-8000-000000000002", SpaceId: "10000000-0000-4000-8000-000000000003",
		ChatId: "10000000-0000-4000-8000-000000000004", NodeId: "10000000-0000-4000-8000-000000000005",
		AuditEventId: "10000000-0000-4000-8000-000000000006", ChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL,
		CreateRequestSha256: make([]byte, 32),
		Initiator:           &spacev1.SpaceChatCreationBinding_User{User: &spacev1.SpaceChatCreationUser{AccountId: "10000000-0000-4000-8000-000000000007", SessionEpoch: 3, AccountType: "regular"}},
	}
}

func TestBindingRejectsUntrustedOrAmbiguousIntent(t *testing.T) {
	require.NoError(t, ValidateBinding(bindingFixture()))
	cases := map[string]func(*spacev1.SpaceChatCreationBinding){
		"zero version":          func(b *spacev1.SpaceChatCreationBinding) { b.ProtocolVersion = 0 },
		"noncanonical UUID":     func(b *spacev1.SpaceChatCreationBinding) { b.OperationId = "{10000000-0000-4000-8000-000000000001}" },
		"DM":                    func(b *spacev1.SpaceChatCreationBinding) { b.ChatType = chatv1.ChatType_CHAT_TYPE_DM },
		"missing digest":        func(b *spacev1.SpaceChatCreationBinding) { b.CreateRequestSha256 = nil },
		"missing initiator":     func(b *spacev1.SpaceChatCreationBinding) { b.Initiator = nil },
		"missing epoch":         func(b *spacev1.SpaceChatCreationBinding) { b.GetUser().SessionEpoch = 0 },
		"no default regular":    func(b *spacev1.SpaceChatCreationBinding) { b.GetUser().AccountType = "" },
		"unknown nested fields": func(b *spacev1.SpaceChatCreationBinding) { b.GetUser().ProtoReflect().SetUnknown([]byte{0x78, 1}) },
		"wrong bot scope": func(b *spacev1.SpaceChatCreationBinding) {
			b.Initiator = &spacev1.SpaceChatCreationBinding_Bot{Bot: &spacev1.SpaceChatCreationBot{BotId: b.ChatId, InstallationId: b.NodeId, BotScope: "SPACE_MANAGE_ROLES"}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) { b := bindingFixture(); mutate(b); require.Error(t, ValidateBinding(b)) })
	}
	require.Error(t, ValidateBinding(nil))
}

func TestOutcomeNeverAcceptsChangedBindingOrPartialSuccess(t *testing.T) {
	b := bindingFixture()
	out := &spacev1.SpaceChatCreationOutcome{Binding: proto.Clone(b).(*spacev1.SpaceChatCreationBinding), State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED, Node: &spacev1.SpaceTreeNode{Id: b.NodeId, SpaceId: b.SpaceId, Kind: "text_chat", LinkedChat: &chatv1.ChatRef{Id: b.ChatId}}}
	require.NoError(t, ValidateOutcome(b, out))
	changed := proto.Clone(out).(*spacev1.SpaceChatCreationOutcome)
	changed.Binding.AuditEventId = b.OperationId
	require.Error(t, ValidateOutcome(b, changed))
	changed = proto.Clone(out).(*spacev1.SpaceChatCreationOutcome)
	changed.Node.Id = b.ChatId
	require.Error(t, ValidateOutcome(b, changed))
	changed = proto.Clone(out).(*spacev1.SpaceChatCreationOutcome)
	changed.FailureCode = 7
	require.Error(t, ValidateOutcome(b, changed))
	require.Error(t, ValidateOutcome(b, nil), "missing result is UNKNOWN, never cleanup authority")
	aborted := &spacev1.SpaceChatCreationOutcome{Binding: proto.Clone(b).(*spacev1.SpaceChatCreationBinding), State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED, FailureCode: 7}
	require.NoError(t, ValidateOutcome(b, aborted))
	aborted.Node = out.Node
	require.Error(t, ValidateOutcome(b, aborted))
}
