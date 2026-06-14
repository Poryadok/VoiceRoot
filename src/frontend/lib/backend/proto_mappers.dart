import 'dart:convert';

import 'package:fixnum/fixnum.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as pb_ts;

import '../gen/voice/calls/v1/calls.pb.dart' as calls_pb;
import '../gen/voice/calls/v1/calls.pbenum.dart' as calls_enum;
import '../gen/voice/chat/v1/chat.pb.dart' as chat_pb;
import '../gen/voice/chat/v1/chat.pbenum.dart';
import '../gen/voice/file/v1/file.pb.dart' as file_pb;
import '../gen/voice/messaging/v1/messaging.pb.dart' as messaging_pb;
import '../gen/voice/messaging/v1/messaging.pbenum.dart';
import '../gen/voice/social/v1/social.pb.dart' as social_pb;
import '../gen/voice/space/v1/space.pb.dart' as space_pb;
import '../gen/voice/subscription/v1/subscription.pb.dart' as sub_pb;
import '../gen/voice/user/v1/user.pb.dart' as user_pb;
import 'chats_client.dart';
import 'files_client.dart';
import 'friends_client.dart';
import 'messages_client.dart';
import 'spaces_client.dart';
import 'subscription_client.dart';
import 'users_client.dart';
import 'voice_client.dart';

DateTime? protoTimestampToDateTime(pb_ts.Timestamp? ts) {
  if (ts == null) return null;
  if (ts.seconds == Int64.ZERO && ts.nanos == 0) return null;
  return DateTime.fromMillisecondsSinceEpoch(
    ts.seconds.toInt() * 1000 + (ts.nanos ~/ 1000000),
    isUtc: true,
  );
}

String? emptyToNull(String? value) {
  if (value == null || value.isEmpty) return null;
  return value;
}

VoiceMessageKind voiceMessageKindFromProto(MessageKind kind) {
  return switch (kind) {
    MessageKind.MESSAGE_KIND_FORWARD => VoiceMessageKind.forward,
    MessageKind.MESSAGE_KIND_SYSTEM => VoiceMessageKind.system,
    MessageKind.MESSAGE_KIND_REGULAR => VoiceMessageKind.regular,
    MessageKind.MESSAGE_KIND_UNSPECIFIED => VoiceMessageKind.unknown,
    _ => VoiceMessageKind.unknown,
  };
}

VoiceMessage voiceMessageFromProto(messaging_pb.Message msg) {
  return VoiceMessage(
    id: msg.id,
    chatId: msg.hasChat() ? msg.chat.id : '',
    senderProfileId: msg.senderProfileId,
    content: msg.content,
    attachments: MessageAttachment.listFromWire(msg.attachmentsJson),
    reactions: MessageReaction.listFromWire(msg.reactionsJson),
    mentions: MessageMention.listFromWire(msg.mentionsJson),
    messageKind: msg.hasMessageKind()
        ? voiceMessageKindFromProto(msg.messageKind)
        : VoiceMessageKind.regular,
    forwardFromId: emptyToNull(msg.forwardFromId),
    forwardFromSender: emptyToNull(msg.forwardFromSender),
    editedAt: protoTimestampToDateTime(msg.hasEditedAt() ? msg.editedAt : null),
    deletedAt: protoTimestampToDateTime(msg.hasDeletedAt() ? msg.deletedAt : null),
    createdAt: protoTimestampToDateTime(msg.hasCreatedAt() ? msg.createdAt : null),
    isPinned: msg.hasIsPinned() && msg.isPinned,
    threadParentId: msg.hasThreadParentId() ? emptyToNull(msg.threadParentId) : null,
  );
}

messaging_pb.PinMessageRequest pinMessageRequestToProto({
  required String chatId,
  required String messageId,
}) {
  return messaging_pb.PinMessageRequest(
    chat: chatRefToProto(chatId),
    messageId: messageId,
  );
}

MessageListData messageListFromProto(messaging_pb.MessageList list) {
  return MessageListData(
    messages: list.messages.map(voiceMessageFromProto).toList(growable: false),
    nextCursor: emptyToNull(list.nextCursor),
    hasMore: list.hasMore,
  );
}

SharedMediaListData sharedMediaListFromProto(messaging_pb.SharedMediaList list) {
  return SharedMediaListData(
    items: list.items.map(sharedMediaItemFromProto).toList(growable: false),
    nextCursor: emptyToNull(list.nextCursor),
    hasMore: list.hasMore,
  );
}

SharedMediaItemData sharedMediaItemFromProto(messaging_pb.SharedMediaItem item) {
  return SharedMediaItemData(
    messageId: item.messageId,
    senderProfileId: item.senderProfileId,
    createdAt: item.hasCreatedAt() ? item.createdAt.toDateTime() : null,
    fileId: item.hasFileId() ? emptyToNull(item.fileId) : null,
    attachmentType:
        item.hasAttachmentType() ? emptyToNull(item.attachmentType) : null,
    externalUrl: item.hasExternalUrl() ? emptyToNull(item.externalUrl) : null,
    title: item.hasTitle() ? emptyToNull(item.title) : null,
    sortOrder: item.sortOrder,
    originalName:
        item.hasOriginalName() ? emptyToNull(item.originalName) : null,
    sizeBytes: item.hasSizeBytes() ? item.sizeBytes.toInt() : null,
  );
}

ReadStateData readStateFromProto(messaging_pb.ReadState state) {
  return ReadStateData(
    chatId: state.hasChat() ? state.chat.id : '',
    profileId: state.profileId,
    lastReadMessageId: state.lastReadMessageId,
  );
}

chat_pb.ChatRef chatRefToProto(String chatId, {ChatType? type}) {
  if (type == null || type == ChatType.CHAT_TYPE_UNSPECIFIED) {
    return chat_pb.ChatRef(id: chatId);
  }
  return chat_pb.ChatRef(id: chatId, type: type);
}

messaging_pb.SendMessageRequest sendMessageRequestToProto({
  required String chatId,
  required String content,
  List<MessageAttachment> attachments = const [],
  List<MessageMention> mentions = const [],
  String? clientMessageId,
  String? threadParentId,
}) {
  return messaging_pb.SendMessageRequest(
    chat: chatRefToProto(chatId),
    content: content,
    clientMessageId: clientMessageId,
    threadParentId: threadParentId,
    attachmentsJson: attachments.isEmpty
        ? ''
        : jsonEncode(
            attachments.map((a) => a.toJson()).toList(growable: false),
          ),
    mentionsJson: MessageMention.encodeJson(mentions),
  );
}

messaging_pb.MarkReadRequest markReadRequestToProto({
  required String chatId,
  required String lastReadMessageId,
}) {
  return messaging_pb.MarkReadRequest(
    chat: chatRefToProto(chatId),
    lastReadMessageId: lastReadMessageId,
  );
}

messaging_pb.EditMessageRequest editMessageRequestToProto({
  required String messageId,
  required String content,
}) {
  return messaging_pb.EditMessageRequest(
    messageId: messageId,
    content: content,
  );
}

DeleteScope deleteScopeFromWire(String scope) {
  switch (scope) {
    case 'me':
      return DeleteScope.DELETE_SCOPE_FOR_ME;
    case 'everyone':
    default:
      return DeleteScope.DELETE_SCOPE_FOR_EVERYONE;
  }
}

ChatType chatTypeFromWire(String? wire) {
  if (wire == null || wire.isEmpty) return ChatType.CHAT_TYPE_DM;
  for (final value in ChatType.values) {
    if (value.name == wire) return value;
  }
  return ChatType.CHAT_TYPE_DM;
}

VoiceChat voiceChatFromProto(chat_pb.Chat chat) {
  return VoiceChat(
    id: chat.id,
    type: chat.type.name,
    creatorProfileId: chat.creatorProfileId,
    name: chat.hasName() ? emptyToNull(chat.name) : null,
    avatarUrl: chat.hasAvatarUrl() ? emptyToNull(chat.avatarUrl) : null,
    spaceId: chat.hasSpaceId() ? emptyToNull(chat.spaceId) : null,
    slowModeSeconds: chat.slowModeSeconds,
  );
}

ChatListItem chatListItemFromProto(chat_pb.ChatListItem item) {
  return ChatListItem(
    chat: voiceChatFromProto(item.chat),
    lastMessagePreview: item.hasLastMessagePreview()
        ? emptyToNull(item.lastMessagePreview)
        : null,
    unreadCount: item.unreadCount.toInt(),
    inbox: item.hasInbox() ? emptyToNull(item.inbox) : null,
    isStranger: item.hasIsStranger() ? item.isStranger : false,
    dmPeerProfileId: item.hasDmPeerProfileId()
        ? emptyToNull(item.dmPeerProfileId)
        : null,
  );
}

ChatListData chatListFromProto(chat_pb.ChatList list) {
  return ChatListData(
    items: list.items.map(chatListItemFromProto).toList(growable: false),
    nextCursor: emptyToNull(list.nextCursor),
  );
}

ChatMember chatMemberFromProto(chat_pb.ChatMember member) {
  return ChatMember(
    profileId: member.profileId,
    role: member.role,
    joinedAt: protoTimestampToDateTime(
      member.hasJoinedAt() ? member.joinedAt : null,
    ),
    isArchived: member.isArchived,
  );
}

MemberListData memberListFromProto(chat_pb.MemberList list) {
  return MemberListData(
    members: list.members.map(chatMemberFromProto).toList(growable: false),
    nextCursor: emptyToNull(list.nextCursor),
  );
}

chat_pb.CreateDMRequest createDmRequestToProto(String otherProfileId) {
  return chat_pb.CreateDMRequest(otherProfileId: otherProfileId);
}

chat_pb.CreateChatRequest createGroupRequestToProto({required String name}) {
  return chat_pb.CreateChatRequest(
    type: ChatType.CHAT_TYPE_GROUP,
    name: name,
  );
}

chat_pb.AddMembersRequest addMembersRequestToProto({
  required List<String> profileIds,
}) {
  return chat_pb.AddMembersRequest(profileIds: profileIds);
}

chat_pb.UpdateChatRequest updateChatRequestToProto({
  String? name,
  String? avatarUrl,
  int? slowModeSeconds,
}) {
  final req = chat_pb.UpdateChatRequest();
  if (name != null) {
    req.name = name;
  }
  if (avatarUrl != null) {
    req.avatarUrl = avatarUrl;
  }
  if (slowModeSeconds != null) {
    req.slowModeSeconds = slowModeSeconds;
  }
  return req;
}

VoiceProfile voiceProfileFromProto(user_pb.Profile profile) {
  return VoiceProfile(
    id: profile.id,
    accountId: profile.accountId,
    username: profile.username,
    discriminator: profile.discriminator,
    displayName: profile.displayName,
    avatarUrl: profile.hasAvatarUrl() ? emptyToNull(profile.avatarUrl) : null,
    bio: profile.hasBio() ? emptyToNull(profile.bio) : null,
    customStatus: profile.hasCustomStatus()
        ? emptyToNull(profile.customStatus)
        : null,
    isPrimary: profile.isPrimary,
    verificationType: profile.verificationType.isNotEmpty
        ? profile.verificationType
        : 'none',
    verificationBadge: profile.hasVerificationBadge()
        ? emptyToNull(profile.verificationBadge)
        : null,
  );
}

List<VoiceProfile> voiceProfileListFromProto(user_pb.ProfileList list) {
  return list.profiles.map(voiceProfileFromProto).toList(growable: false);
}

VoiceSubscription voiceSubscriptionFromProto(sub_pb.Subscription subscription) {
  return VoiceSubscription(
    id: subscription.id,
    accountId: subscription.accountId,
    plan: subscription.plan.isNotEmpty ? subscription.plan : 'free',
    billingPeriod: subscription.billingPeriod.isNotEmpty
        ? subscription.billingPeriod
        : 'monthly',
    status: subscription.status.isNotEmpty ? subscription.status : 'cancelled',
    provider: emptyToNull(subscription.provider),
    providerSubscriptionId: emptyToNull(subscription.providerSubscriptionId),
    currentPeriodEnd: protoTimestampToDateTime(
      subscription.hasCurrentPeriodEnd() ? subscription.currentPeriodEnd : null,
    ),
  );
}

sub_pb.CreateCheckoutSessionRequest createCheckoutSessionRequestToProto({
  required String plan,
  required String billingPeriod,
  required String successUrl,
  required String cancelUrl,
}) {
  return sub_pb.CreateCheckoutSessionRequest(
    plan: plan,
    billingPeriod: billingPeriod,
    successUrl: successUrl,
    cancelUrl: cancelUrl,
  );
}

VoiceCheckoutSession voiceCheckoutSessionFromProto(sub_pb.CheckoutResponse resp) {
  return VoiceCheckoutSession(
    checkoutUrl: resp.checkoutUrl,
    sessionId: resp.sessionId,
  );
}

VoiceSubscriptionLimits voiceSubscriptionLimitsFromProto(sub_pb.Limits limits) {
  return VoiceSubscriptionLimits(limitsJson: limits.limitsJson);
}

VoicePresence voicePresenceFromProto(user_pb.PresenceStatus status) {
  return VoicePresence(
    profileId: status.profileId,
    status: status.status.isNotEmpty ? status.status : 'invisible',
    lastSeen: protoTimestampToDateTime(
      status.hasLastSeen() ? status.lastSeen : null,
    ),
  );
}

SearchProfilesData searchProfilesFromProto(user_pb.SearchProfilesResponse resp) {
  final list = resp.hasProfileList() ? resp.profileList : user_pb.ProfileList();
  final page = resp.hasPage() ? resp.page : null;
  return SearchProfilesData(
    profiles: list.profiles.map(voiceProfileFromProto).toList(growable: false),
    nextCursor: page != null ? emptyToNull(page.nextCursor) : null,
    hasMore: page?.hasMore ?? false,
  );
}

AvatarPresignedUpload avatarPresignedFromProto(
  user_pb.CreateAvatarPresignedUploadResponse resp,
) {
  return AvatarPresignedUpload(
    httpMethod: resp.httpMethod.isNotEmpty ? resp.httpMethod : 'PUT',
    uploadUrl: resp.uploadUrl,
    requiredHeaders: Map<String, String>.from(resp.requiredHeaders),
    maxBytes: resp.maxBytes.toInt(),
    expiresAt: protoTimestampToDateTime(
      resp.hasExpiresAt() ? resp.expiresAt : null,
    ),
    publicUrl: resp.publicUrl,
    objectKey: resp.objectKey,
  );
}

user_pb.GetBulkPresenceRequest bulkPresenceRequestToProto(
  List<String> profileIds,
) {
  return user_pb.GetBulkPresenceRequest(profileIds: profileIds);
}

user_pb.UpdateProfileRequest updateProfileRequestToProto({
  String? displayName,
  String? bio,
  String? avatarUrl,
}) {
  return user_pb.UpdateProfileRequest(
    displayName: displayName,
    bio: bio,
    avatarUrl: avatarUrl,
  );
}

FriendsListData friendsListFromProto(social_pb.FriendList list) {
  return FriendsListData(
    friends: list.friends.map((e) => e.profileId).toList(growable: false),
    nextCursor: emptyToNull(list.nextCursor),
  );
}

FriendRequestsData friendRequestsFromProto(social_pb.FriendRequestList list) {
  return FriendRequestsData(
    incoming: list.incoming.map((e) => e.profileId).toList(growable: false),
    outgoing: list.outgoing.map((e) => e.profileId).toList(growable: false),
  );
}

FileUploadTicket fileUploadTicketFromProto(file_pb.UploadResponse upload) {
  return FileUploadTicket(
    fileId: upload.fileId,
    presignedPutUrl: Uri.parse(upload.presignedPutUrl),
    r2Key: upload.r2Key,
  );
}

FileMetadataData fileMetadataFromProto(file_pb.FileMetadata meta) {
  return FileMetadataData(
    fileId: meta.id,
    fileType: meta.fileType.isNotEmpty ? meta.fileType : 'other',
    status: meta.status,
    originalName: meta.originalName,
    // HTTP URLs come from GET /files/{id}/url, not R2 keys in metadata.
    sizeBytes: meta.hasSizeBytes() ? meta.sizeBytes.toInt() : null,
  );
}

file_pb.RequestUploadRequest requestUploadToProto({
  required String originalName,
  required String mimeType,
  required int sizeBytes,
  String? chatId,
  ChatType chatType = ChatType.CHAT_TYPE_DM,
}) {
  return file_pb.RequestUploadRequest(
    originalName: originalName,
    mimeType: mimeType,
    sizeBytes: Int64(sizeBytes),
    contextChat: chatId == null || chatId.isEmpty
        ? null
        : chatRefToProto(chatId, type: chatType),
  );
}

file_pb.ConfirmUploadRequest confirmUploadRequestToProto({
  required String fileId,
  required String sha256Hash,
}) {
  return file_pb.ConfirmUploadRequest(
    fileId: fileId,
    sha256Hash: sha256Hash,
  );
}

VoiceCallMediaKind voiceCallMediaKindFromProto(calls_enum.CallMediaKind kind) {
  return switch (kind) {
    calls_enum.CallMediaKind.CALL_MEDIA_KIND_VIDEO =>
      VoiceCallMediaKind.video,
    calls_enum.CallMediaKind.CALL_MEDIA_KIND_AUDIO ||
    calls_enum.CallMediaKind.CALL_MEDIA_KIND_UNSPECIFIED =>
      VoiceCallMediaKind.audio,
    _ => VoiceCallMediaKind.audio,
  };
}

calls_enum.CallMediaKind callMediaKindToProto(VoiceCallMediaKind kind) {
  switch (kind) {
    case VoiceCallMediaKind.video:
      return calls_enum.CallMediaKind.CALL_MEDIA_KIND_VIDEO;
    case VoiceCallMediaKind.audio:
      return calls_enum.CallMediaKind.CALL_MEDIA_KIND_AUDIO;
  }
}

VoiceCallStatus voiceCallStatusFromProto(calls_enum.CallStatus status) {
  return switch (status) {
    calls_enum.CallStatus.CALL_STATUS_RINGING => VoiceCallStatus.ringing,
    calls_enum.CallStatus.CALL_STATUS_ACTIVE => VoiceCallStatus.active,
    calls_enum.CallStatus.CALL_STATUS_DECLINED => VoiceCallStatus.declined,
    calls_enum.CallStatus.CALL_STATUS_MISSED => VoiceCallStatus.missed,
    calls_enum.CallStatus.CALL_STATUS_ENDED => VoiceCallStatus.ended,
    calls_enum.CallStatus.CALL_STATUS_UNSPECIFIED => VoiceCallStatus.unknown,
    _ => VoiceCallStatus.unknown,
  };
}

VoiceSessionKind voiceSessionKindFromProto(
  calls_enum.VoiceSessionKind kind,
  String roomType,
) {
  return switch (kind) {
    calls_enum.VoiceSessionKind.VOICE_SESSION_KIND_GROUP_VOICE =>
      VoiceSessionKind.groupVoice,
    calls_enum.VoiceSessionKind.VOICE_SESSION_KIND_VOICE_ROOM =>
      VoiceSessionKind.voiceRoom,
    calls_enum.VoiceSessionKind.VOICE_SESSION_KIND_CALL => VoiceSessionKind.dm,
    calls_enum.VoiceSessionKind.VOICE_SESSION_KIND_UNSPECIFIED =>
      switch (roomType) {
        'group_voice' => VoiceSessionKind.groupVoice,
        'voice_room' => VoiceSessionKind.voiceRoom,
        _ => VoiceSessionKind.dm,
      },
    _ => VoiceSessionKind.unknown,
  };
}

VoiceCallSession voiceCallSessionFromProto(calls_pb.CallSession session) {
  return VoiceCallSession(
    roomId: session.roomId,
    livekitRoomName: session.livekitRoomName,
    chatId: session.hasLinkedChat() ? session.linkedChat.id : '',
    initiatorProfileId: session.initiatorProfileId,
    calleeProfileId: session.calleeProfileId,
    mediaKind: voiceCallMediaKindFromProto(session.mediaKind),
    status: voiceCallStatusFromProto(session.status),
    sessionKind: voiceSessionKindFromProto(
      session.roomTypeEnum,
      session.roomType,
    ),
    expiresAt: protoTimestampToDateTime(
      session.hasExpiresAt() ? session.expiresAt : null,
    ),
  );
}

VoiceJoinToken voiceJoinTokenFromProto(calls_pb.GetJoinTokenResponse resp) {
  return VoiceJoinToken(
    jwt: resp.jwt,
    expiresAt: protoTimestampToDateTime(
      resp.hasExpiresAt() ? resp.expiresAt : null,
    ),
    livekitUrl: emptyToNull(resp.livekitUrl),
  );
}

calls_pb.StartCallRequest startCallRequestToProto({
  required String chatId,
  required String calleeProfileId,
  required VoiceCallMediaKind mediaKind,
}) {
  return calls_pb.StartCallRequest(
    linkedChat: chatRefToProto(chatId),
    calleeProfileId: calleeProfileId,
    mediaKind: callMediaKindToProto(mediaKind),
  );
}

calls_pb.StartCallRequest startGroupVoiceRequestToProto({
  required String groupChatId,
  required VoiceCallMediaKind mediaKind,
}) {
  return calls_pb.StartCallRequest(
    roomTypeEnum:
        calls_enum.VoiceSessionKind.VOICE_SESSION_KIND_GROUP_VOICE,
    linkedChat: chatRefToProto(
      groupChatId,
      type: ChatType.CHAT_TYPE_GROUP,
    ),
    mediaKind: callMediaKindToProto(mediaKind),
  );
}

messaging_pb.ForwardMessageRequest forwardMessageRequestToProto({
  required String sourceMessageId,
  required String targetChatId,
  String? commentary,
}) {
  return messaging_pb.ForwardMessageRequest(
    sourceMessageId: sourceMessageId,
    targetChat: chatRefToProto(targetChatId),
    commentary: commentary,
  );
}

calls_pb.UpdateVoiceStateRequest updateVoiceStateRequestToProto({
  bool? isMuted,
  bool? isDeafened,
  bool? isVideoOn,
  required String roomId,
}) {
  return calls_pb.UpdateVoiceStateRequest(
    roomId: roomId,
    isMuted: isMuted,
    isDeafened: isDeafened,
    isVideoOn: isVideoOn,
  );
}

VoiceSpace voiceSpaceFromProto(space_pb.Space space) {
  return VoiceSpace(
    id: space.id,
    name: space.name,
    description: emptyToNull(space.description),
    iconUrl: space.hasIconUrl() ? emptyToNull(space.iconUrl) : null,
    visibility: space.visibility,
    ownerProfileId: space.ownerProfileId,
    memberCount: space.memberCount,
    createdAt: protoTimestampToDateTime(
      space.hasCreatedAt() ? space.createdAt : null,
    ),
    updatedAt: protoTimestampToDateTime(
      space.hasUpdatedAt() ? space.updatedAt : null,
    ),
  );
}

SpaceListData spaceListFromProto(space_pb.SpaceList list) {
  return SpaceListData(
    spaces: list.spaces.map(voiceSpaceFromProto).toList(growable: false),
    nextCursor: emptyToNull(list.nextCursor),
  );
}

space_pb.CreateSpaceRequest createSpaceRequestToProto({
  required String name,
  String? description,
  String visibility = 'private',
}) {
  final req = space_pb.CreateSpaceRequest(name: name, visibility: visibility);
  if (description != null && description.isNotEmpty) {
    req.description = description;
  }
  return req;
}

space_pb.UpdateSpaceRequest updateSpaceRequestToProto({
  String? iconUrl,
  String? description,
}) {
  final req = space_pb.UpdateSpaceRequest();
  if (iconUrl != null && iconUrl.isNotEmpty) {
    req.iconUrl = iconUrl;
  }
  if (description != null) {
    req.description = description;
  }
  return req;
}

SpaceTreeData spaceTreeFromProto(space_pb.ListSpaceTreeResponse resp) {
  final voiceById = <String, VoiceRoomData>{
    for (final vr in resp.voiceRooms)
      vr.id: VoiceRoomData(id: vr.id, spaceId: vr.spaceId, name: vr.name),
  };
  return SpaceTreeData(
    categories: resp.categories
        .map(
          (c) => SpaceCategory(
            id: c.id,
            spaceId: c.spaceId,
            name: c.name,
            sortOrder: c.sortOrder,
          ),
        )
        .toList(growable: false),
    nodes: resp.nodes
        .map((n) => spaceTreeNodeFromProto(n, voiceById))
        .toList(growable: false),
    voiceRooms: resp.voiceRooms
        .map((vr) => VoiceRoomData(id: vr.id, spaceId: vr.spaceId, name: vr.name))
        .toList(growable: false),
  );
}

SpaceTreeData spaceTreeFromJson(Map<String, dynamic> data) {
  final voiceRooms = <VoiceRoomData>[];
  final voiceById = <String, VoiceRoomData>{};
  final rawVoiceRooms = data['voice_rooms'];
  if (rawVoiceRooms is List) {
    for (final item in rawVoiceRooms) {
      if (item is! Map<String, dynamic>) continue;
      final vr = VoiceRoomData(
        id: item['id'] as String? ?? '',
        spaceId: item['space_id'] as String? ?? '',
        name: item['name'] as String? ?? '',
      );
      voiceRooms.add(vr);
      voiceById[vr.id] = vr;
    }
  }

  final categories = <SpaceCategory>[];
  final rawCategories = data['categories'];
  if (rawCategories is List) {
    for (final item in rawCategories) {
      if (item is! Map<String, dynamic>) continue;
      categories.add(
        SpaceCategory(
          id: item['id'] as String? ?? '',
          spaceId: item['space_id'] as String? ?? '',
          name: item['name'] as String? ?? '',
          sortOrder: (item['sort_order'] as num?)?.toInt() ?? 0,
        ),
      );
    }
  }

  final nodes = <SpaceTreeNodeData>[];
  final rawNodes = data['nodes'];
  if (rawNodes is List) {
    for (final item in rawNodes) {
      if (item is Map<String, dynamic>) {
        nodes.add(spaceTreeNodeFromJson(item, voiceById));
      }
    }
  }

  return SpaceTreeData(
    categories: categories,
    nodes: nodes,
    voiceRooms: voiceRooms,
  );
}

pb_ts.Timestamp dateTimeToProtoTimestamp(DateTime dt) {
  final utc = dt.toUtc();
  return pb_ts.Timestamp()
    ..seconds = Int64(utc.millisecondsSinceEpoch ~/ 1000)
    ..nanos = (utc.millisecondsSinceEpoch % 1000) * 1000000;
}

SpaceInvite spaceInviteFromProto(space_pb.Invite invite) {
  return SpaceInvite(
    id: invite.id,
    spaceId: invite.spaceId,
    code: invite.code,
    creatorProfileId: invite.creatorProfileId,
    maxUses: invite.hasMaxUses() ? invite.maxUses : null,
    useCount: invite.useCount,
    expiresAt: protoTimestampToDateTime(
      invite.hasExpiresAt() ? invite.expiresAt : null,
    ),
    createdAt:
        protoTimestampToDateTime(invite.hasCreatedAt() ? invite.createdAt : null) ??
        DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
    revokedAt: protoTimestampToDateTime(
      invite.hasRevokedAt() ? invite.revokedAt : null,
    ),
  );
}

SpaceMembershipData spaceMembershipFromProto(space_pb.SpaceMembership membership) {
  return SpaceMembershipData(
    spaceId: membership.spaceId,
    profileId: membership.profileId,
    joinedAt:
        protoTimestampToDateTime(
          membership.hasJoinedAt() ? membership.joinedAt : null,
        ) ??
        DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
    nickname: membership.hasNickname() ? emptyToNull(membership.nickname) : null,
  );
}

space_pb.CreateInviteRequest createInviteRequestToProto({
  required String spaceId,
  int? maxUses,
  DateTime? expiresAt,
}) {
  final req = space_pb.CreateInviteRequest(spaceId: spaceId);
  if (maxUses != null) {
    req.maxUses = maxUses;
  }
  if (expiresAt != null) {
    req.expiresAt = dateTimeToProtoTimestamp(expiresAt);
  }
  return req;
}

SpaceTreeNodeData spaceTreeNodeFromProto(
  space_pb.SpaceTreeNode node,
  Map<String, VoiceRoomData> voiceById,
) {
  final voiceRoomId = node.hasVoiceRoomId() ? node.voiceRoomId : null;
  final linkedChatId =
      node.hasLinkedChat() && node.linkedChat.hasId() ? node.linkedChat.id : null;
  final voiceName = voiceRoomId != null ? voiceById[voiceRoomId]?.name : null;
  final chatType = node.hasLinkedChat() && node.linkedChat.hasType()
      ? node.linkedChat.type.name
      : null;
  return SpaceTreeNodeData(
    id: node.id,
    spaceId: node.spaceId,
    categoryId: node.hasCategoryId() ? node.categoryId : null,
    kind: node.kind,
    linkedChatId: linkedChatId,
    voiceRoomId: voiceRoomId,
    sortOrder: node.sortOrder,
    isSystem: node.isSystem,
    displayName: voiceName ?? linkedChatId ?? node.id,
    chatType: chatType,
  );
}

SpaceTreeNodeData spaceTreeNodeFromJson(
  Map<String, dynamic> node,
  Map<String, VoiceRoomData> voiceById,
) {
  final voiceRoomId = emptyToNull(node['voice_room_id'] as String?);
  final linkedChat = node['linked_chat'];
  String? linkedChatId;
  String? chatType;
  if (linkedChat is Map<String, dynamic>) {
    linkedChatId = emptyToNull(linkedChat['id'] as String?);
    final rawType = linkedChat['type'] as String?;
    if (rawType != null && rawType.isNotEmpty) {
      chatType = rawType;
    }
  }
  final voiceName = voiceRoomId != null ? voiceById[voiceRoomId]?.name : null;
  final enrichedName = emptyToNull(node['display_name'] as String?);
  return SpaceTreeNodeData(
    id: node['id'] as String? ?? '',
    spaceId: node['space_id'] as String? ?? '',
    categoryId: emptyToNull(node['category_id'] as String?),
    kind: node['kind'] as String? ?? '',
    linkedChatId: linkedChatId,
    voiceRoomId: voiceRoomId,
    sortOrder: (node['sort_order'] as num?)?.toInt() ?? 0,
    isSystem: node['is_system'] as bool? ?? false,
    displayName: enrichedName ?? voiceName ?? linkedChatId ?? node['id'] as String? ?? '',
    chatType: chatType,
  );
}

VoiceRoomSession voiceRoomSessionFromJson(Map<String, dynamic> data) {
  final session = data['voice_session'];
  if (session is! Map<String, dynamic>) {
    return const VoiceRoomSession(
      roomId: '',
      livekitRoomName: '',
      voiceRoomId: '',
    );
  }
  return VoiceRoomSession(
    roomId: session['room_id'] as String? ?? '',
    livekitRoomName: session['livekit_room_name'] as String? ?? '',
    voiceRoomId: session['voice_room_id'] as String? ?? '',
  );
}

List<VoiceRoomParticipantState> voiceRoomParticipantStatesFromJson(
  Map<String, dynamic> data,
) {
  final raw = data['participants'];
  if (raw is! List) return const [];
  return [
    for (final item in raw)
      if (item is Map<String, dynamic>)
        VoiceRoomParticipantState(
          profileId: item['profile_id'] as String? ?? '',
          isMuted: item['is_muted'] as bool? ?? false,
          isDeafened: item['is_deafened'] as bool? ?? false,
          isVideoOn: item['is_video_on'] as bool? ?? false,
          isScreenSharing: item['is_screen_sharing'] as bool? ?? false,
        ),
  ];
}
