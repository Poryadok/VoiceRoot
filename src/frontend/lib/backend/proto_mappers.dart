import 'dart:convert';

import 'package:fixnum/fixnum.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as pb_ts;

import '../gen/voice/calls/v1/calls.pb.dart' as calls_pb;
import '../gen/voice/calls/v1/calls.pbenum.dart';
import '../gen/voice/chat/v1/chat.pb.dart' as chat_pb;
import '../gen/voice/chat/v1/chat.pbenum.dart';
import '../gen/voice/file/v1/file.pb.dart' as file_pb;
import '../gen/voice/messaging/v1/messaging.pb.dart' as messaging_pb;
import '../gen/voice/messaging/v1/messaging.pbenum.dart';
import '../gen/voice/social/v1/social.pb.dart' as social_pb;
import '../gen/voice/user/v1/user.pb.dart' as user_pb;
import 'chats_client.dart';
import 'files_client.dart';
import 'friends_client.dart';
import 'messages_client.dart';
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

VoiceMessage voiceMessageFromProto(messaging_pb.Message msg) {
  return VoiceMessage(
    id: msg.id,
    chatId: msg.hasChat() ? msg.chat.id : '',
    senderProfileId: msg.senderProfileId,
    content: msg.content,
    attachments: MessageAttachment.listFromWire(msg.attachmentsJson),
    editedAt: protoTimestampToDateTime(msg.hasEditedAt() ? msg.editedAt : null),
    deletedAt: protoTimestampToDateTime(msg.hasDeletedAt() ? msg.deletedAt : null),
    createdAt: protoTimestampToDateTime(msg.hasCreatedAt() ? msg.createdAt : null),
  );
}

MessageListData messageListFromProto(messaging_pb.MessageList list) {
  return MessageListData(
    messages: list.messages.map(voiceMessageFromProto).toList(growable: false),
    nextCursor: emptyToNull(list.nextCursor),
    hasMore: list.hasMore,
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
  String? clientMessageId,
}) {
  return messaging_pb.SendMessageRequest(
    chat: chatRefToProto(chatId),
    content: content,
    clientMessageId: clientMessageId,
    attachmentsJson: attachments.isEmpty
        ? ''
        : jsonEncode(
            attachments.map((a) => a.toJson()).toList(growable: false),
          ),
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
}) {
  final req = chat_pb.UpdateChatRequest();
  if (name != null) {
    req.name = name;
  }
  if (avatarUrl != null) {
    req.avatarUrl = avatarUrl;
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
  );
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

VoiceCallMediaKind voiceCallMediaKindFromProto(CallMediaKind kind) {
  return switch (kind) {
    CallMediaKind.CALL_MEDIA_KIND_VIDEO => VoiceCallMediaKind.video,
    CallMediaKind.CALL_MEDIA_KIND_AUDIO ||
    CallMediaKind.CALL_MEDIA_KIND_UNSPECIFIED => VoiceCallMediaKind.audio,
    _ => VoiceCallMediaKind.audio,
  };
}

CallMediaKind callMediaKindToProto(VoiceCallMediaKind kind) {
  switch (kind) {
    case VoiceCallMediaKind.video:
      return CallMediaKind.CALL_MEDIA_KIND_VIDEO;
    case VoiceCallMediaKind.audio:
      return CallMediaKind.CALL_MEDIA_KIND_AUDIO;
  }
}

VoiceCallStatus voiceCallStatusFromProto(CallStatus status) {
  return switch (status) {
    CallStatus.CALL_STATUS_RINGING => VoiceCallStatus.ringing,
    CallStatus.CALL_STATUS_ACTIVE => VoiceCallStatus.active,
    CallStatus.CALL_STATUS_DECLINED => VoiceCallStatus.declined,
    CallStatus.CALL_STATUS_MISSED => VoiceCallStatus.missed,
    CallStatus.CALL_STATUS_ENDED => VoiceCallStatus.ended,
    CallStatus.CALL_STATUS_UNSPECIFIED => VoiceCallStatus.unknown,
    _ => VoiceCallStatus.unknown,
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
