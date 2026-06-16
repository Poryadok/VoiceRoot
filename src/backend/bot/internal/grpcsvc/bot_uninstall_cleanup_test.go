package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
)

// uninstallRoleSpy records DeleteRolesCreatedByProfile when wired via gRPC test server.
type uninstallRoleSpy struct {
	fakeRoleClient
	deleteRolesCalls int
	lastSpaceID      string
	lastProfileID    string
}

func (f *uninstallRoleSpy) invokeDeleteRolesCreatedByProfile(
	ctx context.Context,
	req proto.Message,
) error {
	f.deleteRolesCalls++
	pr := req.ProtoReflect()
	if fd := pr.Descriptor().Fields().ByName("space_id"); fd != nil {
		f.lastSpaceID = pr.Get(fd).String()
	}
	if fd := pr.Descriptor().Fields().ByName("created_by_profile_id"); fd != nil {
		f.lastProfileID = pr.Get(fd).String()
	}
	return nil
}

func (f *uninstallRoleSpy) DeleteRolesCreatedByProfile(ctx context.Context, req *rolev1.DeleteRolesCreatedByProfileRequest) (*rolev1.DeleteRolesCreatedByProfileResponse, error) {
	_ = f.invokeDeleteRolesCreatedByProfile(ctx, req)
	return &rolev1.DeleteRolesCreatedByProfileResponse{}, nil
}

func (f *uninstallRoleSpy) GetMemberRoles(_ context.Context, _ *rolev1.GetMemberRolesRequest) (*rolev1.GetMemberRolesResponse, error) {
	return &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{}}, nil
}

// uninstallMsgSpy records UnpinMessagesBySenderInChats and preserves sent messages.
type uninstallMsgSpy struct {
	*fakeMessagingClient
	unpinCalls      int
	lastSenderID    string
	lastChatIDs     []string
	sentMessageIDs  []string
	preservedBodies []string
}

func (f *uninstallMsgSpy) SendMessage(_ context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	id := uuid.NewString()
	f.preservedBodies = append(f.preservedBodies, req.GetContent())
	f.sentMessageIDs = append(f.sentMessageIDs, id)
	return &messagingv1.SendMessageResponse{
		Message: &messagingv1.Message{Id: id, Content: req.GetContent()},
	}, nil
}

func (f *uninstallMsgSpy) invokeUnpinMessagesBySenderInChats(
	_ context.Context,
	req proto.Message,
) error {
	f.unpinCalls++
	pr := req.ProtoReflect()
	if fd := pr.Descriptor().Fields().ByName("sender_profile_id"); fd != nil {
		f.lastSenderID = pr.Get(fd).String()
	}
	if fd := pr.Descriptor().Fields().ByName("chat_ids"); fd != nil {
		list := pr.Get(fd).List()
		f.lastChatIDs = f.lastChatIDs[:0]
		for i := 0; i < list.Len(); i++ {
			f.lastChatIDs = append(f.lastChatIDs, list.Get(i).String())
		}
	}
	return nil
}

func (f *uninstallMsgSpy) UnpinMessagesBySenderInChats(ctx context.Context, req *messagingv1.UnpinMessagesBySenderInChatsRequest) (*messagingv1.UnpinMessagesBySenderInChatsResponse, error) {
	_ = f.invokeUnpinMessagesBySenderInChats(ctx, req)
	return &messagingv1.UnpinMessagesBySenderInChatsResponse{}, nil
}

func requireBotProtoField(t *testing.T, messageName, fieldName string) {
	t.Helper()
	mt, err := protoregistry.GlobalTypes.FindMessageByName(
		protoreflect.FullName("voice.bot.v1." + messageName),
	)
	require.NoError(t, err, "voice.bot.v1.%s must exist", messageName)
	fd := mt.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	require.NotNil(t, fd, "voice.bot.v1.%s must have %q field (BOT-B audit)", messageName, fieldName)
}

func TestBot_protoHasSlugField(t *testing.T) {
	requireBotProtoField(t, "Bot", "slug")
}

func TestInstalledBot_protoHasOnlineField(t *testing.T) {
	requireBotProtoField(t, "InstalledBot", "online")
}

func TestUninstallBotFromSpace_invokesRoleDeleteAndMessagingUnpin(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	roleSpy := &uninstallRoleSpy{}
	msgSpy := &uninstallMsgSpy{fakeMessagingClient: &fakeMessagingClient{}}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{
		role: roleSpy,
		msg:  msgSpy,
	})
	defer cleanup()

	ctx, botID, _, chatID, spaceID := setupBotCCommandBot(t, client, st, `["TEXT_CHAT_SEND_MESSAGES"]`)
	botRow, err := st.GetBotByID(ctx, uuid.MustParse(botID))
	require.NoError(t, err)
	actorProfileID := botRow.ActorProfileID.String()

	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	chatRef := &chatv1.ChatRef{Id: chatID.String(), Type: &chatType}

	// Bot message remains after uninstall cleanup (only pins removed).
	_, err = msgSpy.SendMessage(ctx, &messagingv1.SendMessageRequest{
		Chat: chatRef, Content: "bot pinned post", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	_, err = client.UninstallBotFromSpace(ctx, &botv1.UninstallBotFromSpaceRequest{
		BotId:   botID,
		SpaceId: spaceID.String(),
	})
	require.NoError(t, err)

	require.Equal(t, 1, roleSpy.deleteRolesCalls,
		"UninstallBotFromSpace must call Role.DeleteRolesCreatedByProfile (BOT-B)")
	require.Equal(t, spaceID.String(), roleSpy.lastSpaceID)
	require.Equal(t, actorProfileID, roleSpy.lastProfileID)

	require.Equal(t, 1, msgSpy.unpinCalls,
		"UninstallBotFromSpace must call Messaging.UnpinMessagesBySenderInChats (BOT-B)")
	require.Equal(t, actorProfileID, msgSpy.lastSenderID)
	require.Contains(t, msgSpy.lastChatIDs, chatID.String())
	require.NotEmpty(t, msgSpy.preservedBodies,
		"uninstall cleanup must not delete message bodies (BOT-B)")
}

// mustProtoMessage is used by future tests once BOT-B proto fields land.
func mustProtoMessage(t *testing.T, fullName protoreflect.FullName) proto.Message {
	t.Helper()
	mt, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
	require.NoError(t, err, "%s not in generated proto yet (BOT-B audit)", fullName)
	return mt.New().Interface().(proto.Message)
}
