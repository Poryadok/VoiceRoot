package grpcsvc

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestBotChatScopeRequestPreservesFourFieldGoldenWire(t *testing.T) {
	const chat = "00000000-0000-0000-0000-000000000247"
	const space = "00000000-0000-0000-0000-000000000170"
	const operation = "00000000-0000-0000-0000-000000000034"
	const golden = "0801122430303030303030302d303030302d303030302d303030302d3030303030303030303234371a2430303030303030302d303030302d303030302d303030302d303030303030303030313730222430303030303030302d303030302d303030302d303030302d303030303030303030303334"
	want, err := hex.DecodeString(golden)
	require.NoError(t, err)
	old := &chatv1.GetSpaceChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: chat, ExpectedSpaceId: space, OperationId: operation}
	next := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: chat, ExpectedSpaceId: space, OperationId: operation}
	for _, request := range []proto.Message{old, next} {
		got, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
		require.NoError(t, err)
		require.Equal(t, want, got)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		require.Equal(t, "sha256:123c207c601200dc4fed96b1daf6cbf041afb9e7ac3f48da227f631d21220425", hash)
	}
	require.NotEqual(t, chatv1.ChatScopeAuthorityService_GetSpaceChatScopeAuthority_FullMethodName, chatv1.ChatScopeAuthorityService_GetBotChatScopeAuthority_FullMethodName, "same body bytes do not alias the signed RPC")
}
