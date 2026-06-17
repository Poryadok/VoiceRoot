// This is a generated file - do not edit.
//
// Generated from voice/messaging/v1/messaging.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $3;

import '../../chat/v1/chat.pb.dart' as $1;
import '../../common/v1/common.pb.dart' as $2;
import 'messaging.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'messaging.pbenum.dart';

class SendMessageRequest extends $pb.GeneratedMessage {
  factory SendMessageRequest({
    $1.ChatRef? chat,
    $core.String? content,
    $core.String? clientMessageId,
    $core.String? attachmentsJson,
    $core.String? mentionsJson,
    $core.String? threadParentId,
    MessageKind? messageKind,
    $core.bool? postedAsChat,
    $core.bool? isE2e,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (content != null) result.content = content;
    if (clientMessageId != null) result.clientMessageId = clientMessageId;
    if (attachmentsJson != null) result.attachmentsJson = attachmentsJson;
    if (mentionsJson != null) result.mentionsJson = mentionsJson;
    if (threadParentId != null) result.threadParentId = threadParentId;
    if (messageKind != null) result.messageKind = messageKind;
    if (postedAsChat != null) result.postedAsChat = postedAsChat;
    if (isE2e != null) result.isE2e = isE2e;
    return result;
  }

  SendMessageRequest._();

  factory SendMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'content')
    ..aOS(3, _omitFieldNames ? '' : 'clientMessageId')
    ..aOS(4, _omitFieldNames ? '' : 'attachmentsJson')
    ..aOS(5, _omitFieldNames ? '' : 'mentionsJson')
    ..aOS(6, _omitFieldNames ? '' : 'threadParentId')
    ..aE<MessageKind>(7, _omitFieldNames ? '' : 'messageKind',
        enumValues: MessageKind.values)
    ..aOB(8, _omitFieldNames ? '' : 'postedAsChat')
    ..aOB(9, _omitFieldNames ? '' : 'isE2e')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageRequest copyWith(void Function(SendMessageRequest) updates) =>
      super.copyWith((message) => updates(message as SendMessageRequest))
          as SendMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendMessageRequest create() => SendMessageRequest._();
  @$core.override
  SendMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendMessageRequest>(create);
  static SendMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get content => $_getSZ(1);
  @$pb.TagNumber(2)
  set content($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContent() => $_has(1);
  @$pb.TagNumber(2)
  void clearContent() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get clientMessageId => $_getSZ(2);
  @$pb.TagNumber(3)
  set clientMessageId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasClientMessageId() => $_has(2);
  @$pb.TagNumber(3)
  void clearClientMessageId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get attachmentsJson => $_getSZ(3);
  @$pb.TagNumber(4)
  set attachmentsJson($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAttachmentsJson() => $_has(3);
  @$pb.TagNumber(4)
  void clearAttachmentsJson() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get mentionsJson => $_getSZ(4);
  @$pb.TagNumber(5)
  set mentionsJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMentionsJson() => $_has(4);
  @$pb.TagNumber(5)
  void clearMentionsJson() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get threadParentId => $_getSZ(5);
  @$pb.TagNumber(6)
  set threadParentId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasThreadParentId() => $_has(5);
  @$pb.TagNumber(6)
  void clearThreadParentId() => $_clearField(6);

  /// Prefer when set; aligns outgoing Message.type / Message.message_kind for new sends (docs/REPOSITORIES.md).
  @$pb.TagNumber(7)
  MessageKind get messageKind => $_getN(6);
  @$pb.TagNumber(7)
  set messageKind(MessageKind value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasMessageKind() => $_has(6);
  @$pb.TagNumber(7)
  void clearMessageKind() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get postedAsChat => $_getBF(7);
  @$pb.TagNumber(8)
  set postedAsChat($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasPostedAsChat() => $_has(7);
  @$pb.TagNumber(8)
  void clearPostedAsChat() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get isE2e => $_getBF(8);
  @$pb.TagNumber(9)
  set isE2e($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasIsE2e() => $_has(8);
  @$pb.TagNumber(9)
  void clearIsE2e() => $_clearField(9);
}

class EditMessageRequest extends $pb.GeneratedMessage {
  factory EditMessageRequest({
    $core.String? messageId,
    $core.String? content,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (content != null) result.content = content;
    return result;
  }

  EditMessageRequest._();

  factory EditMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EditMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EditMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'content')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EditMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EditMessageRequest copyWith(void Function(EditMessageRequest) updates) =>
      super.copyWith((message) => updates(message as EditMessageRequest))
          as EditMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EditMessageRequest create() => EditMessageRequest._();
  @$core.override
  EditMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EditMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EditMessageRequest>(create);
  static EditMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get content => $_getSZ(1);
  @$pb.TagNumber(2)
  set content($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContent() => $_has(1);
  @$pb.TagNumber(2)
  void clearContent() => $_clearField(2);
}

class DeleteMessageRequest extends $pb.GeneratedMessage {
  factory DeleteMessageRequest({
    $core.String? messageId,
    DeleteScope? scope,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (scope != null) result.scope = scope;
    return result;
  }

  DeleteMessageRequest._();

  factory DeleteMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aE<DeleteScope>(2, _omitFieldNames ? '' : 'scope',
        enumValues: DeleteScope.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteMessageRequest copyWith(void Function(DeleteMessageRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteMessageRequest))
          as DeleteMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteMessageRequest create() => DeleteMessageRequest._();
  @$core.override
  DeleteMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteMessageRequest>(create);
  static DeleteMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  DeleteScope get scope => $_getN(1);
  @$pb.TagNumber(2)
  set scope(DeleteScope value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasScope() => $_has(1);
  @$pb.TagNumber(2)
  void clearScope() => $_clearField(2);
}

class GetMessagesRequest extends $pb.GeneratedMessage {
  factory GetMessagesRequest({
    $1.ChatRef? chat,
    $core.String? afterMessageId,
    $core.String? beforeMessageId,
    $core.String? lastMessageId,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (afterMessageId != null) result.afterMessageId = afterMessageId;
    if (beforeMessageId != null) result.beforeMessageId = beforeMessageId;
    if (lastMessageId != null) result.lastMessageId = lastMessageId;
    if (page != null) result.page = page;
    return result;
  }

  GetMessagesRequest._();

  factory GetMessagesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMessagesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMessagesRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'afterMessageId')
    ..aOS(3, _omitFieldNames ? '' : 'beforeMessageId')
    ..aOS(4, _omitFieldNames ? '' : 'lastMessageId')
    ..aOM<$2.CursorPageRequest>(5, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMessagesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMessagesRequest copyWith(void Function(GetMessagesRequest) updates) =>
      super.copyWith((message) => updates(message as GetMessagesRequest))
          as GetMessagesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMessagesRequest create() => GetMessagesRequest._();
  @$core.override
  GetMessagesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMessagesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMessagesRequest>(create);
  static GetMessagesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get afterMessageId => $_getSZ(1);
  @$pb.TagNumber(2)
  set afterMessageId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAfterMessageId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAfterMessageId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get beforeMessageId => $_getSZ(2);
  @$pb.TagNumber(3)
  set beforeMessageId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBeforeMessageId() => $_has(2);
  @$pb.TagNumber(3)
  void clearBeforeMessageId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get lastMessageId => $_getSZ(3);
  @$pb.TagNumber(4)
  set lastMessageId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasLastMessageId() => $_has(3);
  @$pb.TagNumber(4)
  void clearLastMessageId() => $_clearField(4);

  @$pb.TagNumber(5)
  $2.CursorPageRequest get page => $_getN(4);
  @$pb.TagNumber(5)
  set page($2.CursorPageRequest value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPage() => $_has(4);
  @$pb.TagNumber(5)
  void clearPage() => $_clearField(5);
  @$pb.TagNumber(5)
  $2.CursorPageRequest ensurePage() => $_ensure(4);
}

class GetMessageRequest extends $pb.GeneratedMessage {
  factory GetMessageRequest({
    $core.String? messageId,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    return result;
  }

  GetMessageRequest._();

  factory GetMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMessageRequest copyWith(void Function(GetMessageRequest) updates) =>
      super.copyWith((message) => updates(message as GetMessageRequest))
          as GetMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMessageRequest create() => GetMessageRequest._();
  @$core.override
  GetMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMessageRequest>(create);
  static GetMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);
}

class MessageList extends $pb.GeneratedMessage {
  factory MessageList({
    $core.Iterable<Message>? messages,
    $core.String? nextCursor,
    $core.bool? hasMore,
    $2.CursorPageResponse? page,
  }) {
    final result = create();
    if (messages != null) result.messages.addAll(messages);
    if (nextCursor != null) result.nextCursor = nextCursor;
    if (hasMore != null) result.hasMore = hasMore;
    if (page != null) result.page = page;
    return result;
  }

  MessageList._();

  factory MessageList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MessageList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..pPM<Message>(1, _omitFieldNames ? '' : 'messages',
        subBuilder: Message.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..aOB(3, _omitFieldNames ? '' : 'hasMore')
    ..aOM<$2.CursorPageResponse>(4, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageList copyWith(void Function(MessageList) updates) =>
      super.copyWith((message) => updates(message as MessageList))
          as MessageList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MessageList create() => MessageList._();
  @$core.override
  MessageList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MessageList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MessageList>(create);
  static MessageList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Message> get messages => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get hasMore => $_getBF(2);
  @$pb.TagNumber(3)
  set hasMore($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasHasMore() => $_has(2);
  @$pb.TagNumber(3)
  void clearHasMore() => $_clearField(3);

  /// Optional structured pagination; when set, should mirror next_cursor (as next_cursor) and has_more.
  @$pb.TagNumber(4)
  $2.CursorPageResponse get page => $_getN(3);
  @$pb.TagNumber(4)
  set page($2.CursorPageResponse value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPage() => $_has(3);
  @$pb.TagNumber(4)
  void clearPage() => $_clearField(4);
  @$pb.TagNumber(4)
  $2.CursorPageResponse ensurePage() => $_ensure(3);
}

class Message extends $pb.GeneratedMessage {
  factory Message({
    $core.String? id,
    $1.ChatRef? chat,
    $core.String? senderProfileId,
    $core.bool? postedAsChat,
    $core.String? displayChatId,
    $core.String? content,
    $core.String? type,
    $core.String? threadParentId,
    $core.String? forwardFromId,
    $core.String? forwardFromSender,
    $core.String? attachmentsJson,
    $core.String? mentionsJson,
    $3.Timestamp? editedAt,
    $3.Timestamp? deletedAt,
    $3.Timestamp? createdAt,
    MessageKind? messageKind,
    $core.String? reactionsJson,
    $core.bool? isPinned,
    $core.bool? isE2e,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (chat != null) result.chat = chat;
    if (senderProfileId != null) result.senderProfileId = senderProfileId;
    if (postedAsChat != null) result.postedAsChat = postedAsChat;
    if (displayChatId != null) result.displayChatId = displayChatId;
    if (content != null) result.content = content;
    if (type != null) result.type = type;
    if (threadParentId != null) result.threadParentId = threadParentId;
    if (forwardFromId != null) result.forwardFromId = forwardFromId;
    if (forwardFromSender != null) result.forwardFromSender = forwardFromSender;
    if (attachmentsJson != null) result.attachmentsJson = attachmentsJson;
    if (mentionsJson != null) result.mentionsJson = mentionsJson;
    if (editedAt != null) result.editedAt = editedAt;
    if (deletedAt != null) result.deletedAt = deletedAt;
    if (createdAt != null) result.createdAt = createdAt;
    if (messageKind != null) result.messageKind = messageKind;
    if (reactionsJson != null) result.reactionsJson = reactionsJson;
    if (isPinned != null) result.isPinned = isPinned;
    if (isE2e != null) result.isE2e = isE2e;
    return result;
  }

  Message._();

  factory Message.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Message.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Message',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOM<$1.ChatRef>(2, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(4, _omitFieldNames ? '' : 'senderProfileId')
    ..aOB(5, _omitFieldNames ? '' : 'postedAsChat')
    ..aOS(6, _omitFieldNames ? '' : 'displayChatId')
    ..aOS(7, _omitFieldNames ? '' : 'content')
    ..aOS(8, _omitFieldNames ? '' : 'type')
    ..aOS(9, _omitFieldNames ? '' : 'threadParentId')
    ..aOS(10, _omitFieldNames ? '' : 'forwardFromId')
    ..aOS(11, _omitFieldNames ? '' : 'forwardFromSender')
    ..aOS(12, _omitFieldNames ? '' : 'attachmentsJson')
    ..aOS(13, _omitFieldNames ? '' : 'mentionsJson')
    ..aOM<$3.Timestamp>(14, _omitFieldNames ? '' : 'editedAt',
        subBuilder: $3.Timestamp.create)
    ..aOM<$3.Timestamp>(15, _omitFieldNames ? '' : 'deletedAt',
        subBuilder: $3.Timestamp.create)
    ..aOM<$3.Timestamp>(16, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $3.Timestamp.create)
    ..aE<MessageKind>(17, _omitFieldNames ? '' : 'messageKind',
        enumValues: MessageKind.values)
    ..aOS(18, _omitFieldNames ? '' : 'reactionsJson')
    ..aOB(19, _omitFieldNames ? '' : 'isPinned')
    ..aOB(20, _omitFieldNames ? '' : 'isE2e')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Message clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Message copyWith(void Function(Message) updates) =>
      super.copyWith((message) => updates(message as Message)) as Message;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Message create() => Message._();
  @$core.override
  Message createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Message getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Message>(create);
  static Message? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.ChatRef get chat => $_getN(1);
  @$pb.TagNumber(2)
  set chat($1.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.ChatRef ensureChat() => $_ensure(1);

  @$pb.TagNumber(4)
  $core.String get senderProfileId => $_getSZ(2);
  @$pb.TagNumber(4)
  set senderProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(4)
  $core.bool hasSenderProfileId() => $_has(2);
  @$pb.TagNumber(4)
  void clearSenderProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get postedAsChat => $_getBF(3);
  @$pb.TagNumber(5)
  set postedAsChat($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(5)
  $core.bool hasPostedAsChat() => $_has(3);
  @$pb.TagNumber(5)
  void clearPostedAsChat() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get displayChatId => $_getSZ(4);
  @$pb.TagNumber(6)
  set displayChatId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(6)
  $core.bool hasDisplayChatId() => $_has(4);
  @$pb.TagNumber(6)
  void clearDisplayChatId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get content => $_getSZ(5);
  @$pb.TagNumber(7)
  set content($core.String value) => $_setString(5, value);
  @$pb.TagNumber(7)
  $core.bool hasContent() => $_has(5);
  @$pb.TagNumber(7)
  void clearContent() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get type => $_getSZ(6);
  @$pb.TagNumber(8)
  set type($core.String value) => $_setString(6, value);
  @$pb.TagNumber(8)
  $core.bool hasType() => $_has(6);
  @$pb.TagNumber(8)
  void clearType() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get threadParentId => $_getSZ(7);
  @$pb.TagNumber(9)
  set threadParentId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(9)
  $core.bool hasThreadParentId() => $_has(7);
  @$pb.TagNumber(9)
  void clearThreadParentId() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get forwardFromId => $_getSZ(8);
  @$pb.TagNumber(10)
  set forwardFromId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(10)
  $core.bool hasForwardFromId() => $_has(8);
  @$pb.TagNumber(10)
  void clearForwardFromId() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get forwardFromSender => $_getSZ(9);
  @$pb.TagNumber(11)
  set forwardFromSender($core.String value) => $_setString(9, value);
  @$pb.TagNumber(11)
  $core.bool hasForwardFromSender() => $_has(9);
  @$pb.TagNumber(11)
  void clearForwardFromSender() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get attachmentsJson => $_getSZ(10);
  @$pb.TagNumber(12)
  set attachmentsJson($core.String value) => $_setString(10, value);
  @$pb.TagNumber(12)
  $core.bool hasAttachmentsJson() => $_has(10);
  @$pb.TagNumber(12)
  void clearAttachmentsJson() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.String get mentionsJson => $_getSZ(11);
  @$pb.TagNumber(13)
  set mentionsJson($core.String value) => $_setString(11, value);
  @$pb.TagNumber(13)
  $core.bool hasMentionsJson() => $_has(11);
  @$pb.TagNumber(13)
  void clearMentionsJson() => $_clearField(13);

  @$pb.TagNumber(14)
  $3.Timestamp get editedAt => $_getN(12);
  @$pb.TagNumber(14)
  set editedAt($3.Timestamp value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasEditedAt() => $_has(12);
  @$pb.TagNumber(14)
  void clearEditedAt() => $_clearField(14);
  @$pb.TagNumber(14)
  $3.Timestamp ensureEditedAt() => $_ensure(12);

  @$pb.TagNumber(15)
  $3.Timestamp get deletedAt => $_getN(13);
  @$pb.TagNumber(15)
  set deletedAt($3.Timestamp value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasDeletedAt() => $_has(13);
  @$pb.TagNumber(15)
  void clearDeletedAt() => $_clearField(15);
  @$pb.TagNumber(15)
  $3.Timestamp ensureDeletedAt() => $_ensure(13);

  @$pb.TagNumber(16)
  $3.Timestamp get createdAt => $_getN(14);
  @$pb.TagNumber(16)
  set createdAt($3.Timestamp value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasCreatedAt() => $_has(14);
  @$pb.TagNumber(16)
  void clearCreatedAt() => $_clearField(16);
  @$pb.TagNumber(16)
  $3.Timestamp ensureCreatedAt() => $_ensure(14);

  @$pb.TagNumber(17)
  MessageKind get messageKind => $_getN(15);
  @$pb.TagNumber(17)
  set messageKind(MessageKind value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasMessageKind() => $_has(15);
  @$pb.TagNumber(17)
  void clearMessageKind() => $_clearField(17);

  @$pb.TagNumber(18)
  $core.String get reactionsJson => $_getSZ(16);
  @$pb.TagNumber(18)
  set reactionsJson($core.String value) => $_setString(16, value);
  @$pb.TagNumber(18)
  $core.bool hasReactionsJson() => $_has(16);
  @$pb.TagNumber(18)
  void clearReactionsJson() => $_clearField(18);

  @$pb.TagNumber(19)
  $core.bool get isPinned => $_getBF(17);
  @$pb.TagNumber(19)
  set isPinned($core.bool value) => $_setBool(17, value);
  @$pb.TagNumber(19)
  $core.bool hasIsPinned() => $_has(17);
  @$pb.TagNumber(19)
  void clearIsPinned() => $_clearField(19);

  @$pb.TagNumber(20)
  $core.bool get isE2e => $_getBF(18);
  @$pb.TagNumber(20)
  set isE2e($core.bool value) => $_setBool(18, value);
  @$pb.TagNumber(20)
  $core.bool hasIsE2e() => $_has(18);
  @$pb.TagNumber(20)
  void clearIsE2e() => $_clearField(20);
}

class GetThreadMessagesRequest extends $pb.GeneratedMessage {
  factory GetThreadMessagesRequest({
    $1.ChatRef? chat,
    $core.String? threadParentId,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (threadParentId != null) result.threadParentId = threadParentId;
    if (page != null) result.page = page;
    return result;
  }

  GetThreadMessagesRequest._();

  factory GetThreadMessagesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetThreadMessagesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetThreadMessagesRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'threadParentId')
    ..aOM<$2.CursorPageRequest>(3, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadMessagesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadMessagesRequest copyWith(
          void Function(GetThreadMessagesRequest) updates) =>
      super.copyWith((message) => updates(message as GetThreadMessagesRequest))
          as GetThreadMessagesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetThreadMessagesRequest create() => GetThreadMessagesRequest._();
  @$core.override
  GetThreadMessagesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetThreadMessagesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetThreadMessagesRequest>(create);
  static GetThreadMessagesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get threadParentId => $_getSZ(1);
  @$pb.TagNumber(2)
  set threadParentId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasThreadParentId() => $_has(1);
  @$pb.TagNumber(2)
  void clearThreadParentId() => $_clearField(2);

  @$pb.TagNumber(3)
  $2.CursorPageRequest get page => $_getN(2);
  @$pb.TagNumber(3)
  set page($2.CursorPageRequest value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPage() => $_has(2);
  @$pb.TagNumber(3)
  void clearPage() => $_clearField(3);
  @$pb.TagNumber(3)
  $2.CursorPageRequest ensurePage() => $_ensure(2);
}

class ThreadSummary extends $pb.GeneratedMessage {
  factory ThreadSummary({
    $core.String? threadParentId,
    $core.int? replyCount,
    $3.Timestamp? lastReplyAt,
    $core.String? lastReplyPreview,
  }) {
    final result = create();
    if (threadParentId != null) result.threadParentId = threadParentId;
    if (replyCount != null) result.replyCount = replyCount;
    if (lastReplyAt != null) result.lastReplyAt = lastReplyAt;
    if (lastReplyPreview != null) result.lastReplyPreview = lastReplyPreview;
    return result;
  }

  ThreadSummary._();

  factory ThreadSummary.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ThreadSummary.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ThreadSummary',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'threadParentId')
    ..aI(2, _omitFieldNames ? '' : 'replyCount')
    ..aOM<$3.Timestamp>(3, _omitFieldNames ? '' : 'lastReplyAt',
        subBuilder: $3.Timestamp.create)
    ..aOS(4, _omitFieldNames ? '' : 'lastReplyPreview')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadSummary clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadSummary copyWith(void Function(ThreadSummary) updates) =>
      super.copyWith((message) => updates(message as ThreadSummary))
          as ThreadSummary;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ThreadSummary create() => ThreadSummary._();
  @$core.override
  ThreadSummary createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ThreadSummary getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ThreadSummary>(create);
  static ThreadSummary? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadParentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadParentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadParentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadParentId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get replyCount => $_getIZ(1);
  @$pb.TagNumber(2)
  set replyCount($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReplyCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearReplyCount() => $_clearField(2);

  @$pb.TagNumber(3)
  $3.Timestamp get lastReplyAt => $_getN(2);
  @$pb.TagNumber(3)
  set lastReplyAt($3.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasLastReplyAt() => $_has(2);
  @$pb.TagNumber(3)
  void clearLastReplyAt() => $_clearField(3);
  @$pb.TagNumber(3)
  $3.Timestamp ensureLastReplyAt() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get lastReplyPreview => $_getSZ(3);
  @$pb.TagNumber(4)
  set lastReplyPreview($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasLastReplyPreview() => $_has(3);
  @$pb.TagNumber(4)
  void clearLastReplyPreview() => $_clearField(4);
}

class ListThreadsRequest extends $pb.GeneratedMessage {
  factory ListThreadsRequest({
    $1.ChatRef? chat,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (page != null) result.page = page;
    return result;
  }

  ListThreadsRequest._();

  factory ListThreadsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListThreadsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListThreadsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadsRequest copyWith(void Function(ListThreadsRequest) updates) =>
      super.copyWith((message) => updates(message as ListThreadsRequest))
          as ListThreadsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListThreadsRequest create() => ListThreadsRequest._();
  @$core.override
  ListThreadsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListThreadsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListThreadsRequest>(create);
  static ListThreadsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $2.CursorPageRequest get page => $_getN(1);
  @$pb.TagNumber(2)
  set page($2.CursorPageRequest value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPage() => $_has(1);
  @$pb.TagNumber(2)
  void clearPage() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.CursorPageRequest ensurePage() => $_ensure(1);
}

class ThreadList extends $pb.GeneratedMessage {
  factory ThreadList({
    $core.Iterable<ThreadSummary>? threads,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (threads != null) result.threads.addAll(threads);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  ThreadList._();

  factory ThreadList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ThreadList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ThreadList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..pPM<ThreadSummary>(1, _omitFieldNames ? '' : 'threads',
        subBuilder: ThreadSummary.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadList copyWith(void Function(ThreadList) updates) =>
      super.copyWith((message) => updates(message as ThreadList)) as ThreadList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ThreadList create() => ThreadList._();
  @$core.override
  ThreadList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ThreadList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ThreadList>(create);
  static ThreadList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ThreadSummary> get threads => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class AddReactionRequest extends $pb.GeneratedMessage {
  factory AddReactionRequest({
    $core.String? messageId,
    $core.String? emoji,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (emoji != null) result.emoji = emoji;
    return result;
  }

  AddReactionRequest._();

  factory AddReactionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddReactionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddReactionRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'emoji')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddReactionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddReactionRequest copyWith(void Function(AddReactionRequest) updates) =>
      super.copyWith((message) => updates(message as AddReactionRequest))
          as AddReactionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddReactionRequest create() => AddReactionRequest._();
  @$core.override
  AddReactionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddReactionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddReactionRequest>(create);
  static AddReactionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get emoji => $_getSZ(1);
  @$pb.TagNumber(2)
  set emoji($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEmoji() => $_has(1);
  @$pb.TagNumber(2)
  void clearEmoji() => $_clearField(2);
}

class RemoveReactionRequest extends $pb.GeneratedMessage {
  factory RemoveReactionRequest({
    $core.String? messageId,
    $core.String? emoji,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (emoji != null) result.emoji = emoji;
    return result;
  }

  RemoveReactionRequest._();

  factory RemoveReactionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveReactionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveReactionRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'emoji')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveReactionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveReactionRequest copyWith(
          void Function(RemoveReactionRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveReactionRequest))
          as RemoveReactionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveReactionRequest create() => RemoveReactionRequest._();
  @$core.override
  RemoveReactionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveReactionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveReactionRequest>(create);
  static RemoveReactionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get emoji => $_getSZ(1);
  @$pb.TagNumber(2)
  set emoji($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEmoji() => $_has(1);
  @$pb.TagNumber(2)
  void clearEmoji() => $_clearField(2);
}

class PinMessageRequest extends $pb.GeneratedMessage {
  factory PinMessageRequest({
    $1.ChatRef? chat,
    $core.String? messageId,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (messageId != null) result.messageId = messageId;
    return result;
  }

  PinMessageRequest._();

  factory PinMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PinMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PinMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'messageId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PinMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PinMessageRequest copyWith(void Function(PinMessageRequest) updates) =>
      super.copyWith((message) => updates(message as PinMessageRequest))
          as PinMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PinMessageRequest create() => PinMessageRequest._();
  @$core.override
  PinMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PinMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PinMessageRequest>(create);
  static PinMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get messageId => $_getSZ(1);
  @$pb.TagNumber(2)
  set messageId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessageId() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessageId() => $_clearField(2);
}

class UnpinMessageRequest extends $pb.GeneratedMessage {
  factory UnpinMessageRequest({
    $1.ChatRef? chat,
    $core.String? messageId,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (messageId != null) result.messageId = messageId;
    return result;
  }

  UnpinMessageRequest._();

  factory UnpinMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnpinMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpinMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'messageId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpinMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpinMessageRequest copyWith(void Function(UnpinMessageRequest) updates) =>
      super.copyWith((message) => updates(message as UnpinMessageRequest))
          as UnpinMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnpinMessageRequest create() => UnpinMessageRequest._();
  @$core.override
  UnpinMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnpinMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpinMessageRequest>(create);
  static UnpinMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get messageId => $_getSZ(1);
  @$pb.TagNumber(2)
  set messageId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessageId() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessageId() => $_clearField(2);
}

class GetPinnedMessagesRequest extends $pb.GeneratedMessage {
  factory GetPinnedMessagesRequest({
    $1.ChatRef? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  GetPinnedMessagesRequest._();

  factory GetPinnedMessagesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPinnedMessagesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPinnedMessagesRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPinnedMessagesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPinnedMessagesRequest copyWith(
          void Function(GetPinnedMessagesRequest) updates) =>
      super.copyWith((message) => updates(message as GetPinnedMessagesRequest))
          as GetPinnedMessagesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPinnedMessagesRequest create() => GetPinnedMessagesRequest._();
  @$core.override
  GetPinnedMessagesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPinnedMessagesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPinnedMessagesRequest>(create);
  static GetPinnedMessagesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);
}

class ForwardMessageRequest extends $pb.GeneratedMessage {
  factory ForwardMessageRequest({
    $core.String? sourceMessageId,
    $1.ChatRef? targetChat,
    $core.String? commentary,
  }) {
    final result = create();
    if (sourceMessageId != null) result.sourceMessageId = sourceMessageId;
    if (targetChat != null) result.targetChat = targetChat;
    if (commentary != null) result.commentary = commentary;
    return result;
  }

  ForwardMessageRequest._();

  factory ForwardMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ForwardMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ForwardMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sourceMessageId')
    ..aOM<$1.ChatRef>(2, _omitFieldNames ? '' : 'targetChat',
        subBuilder: $1.ChatRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'commentary')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ForwardMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ForwardMessageRequest copyWith(
          void Function(ForwardMessageRequest) updates) =>
      super.copyWith((message) => updates(message as ForwardMessageRequest))
          as ForwardMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ForwardMessageRequest create() => ForwardMessageRequest._();
  @$core.override
  ForwardMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ForwardMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ForwardMessageRequest>(create);
  static ForwardMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sourceMessageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sourceMessageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSourceMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSourceMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.ChatRef get targetChat => $_getN(1);
  @$pb.TagNumber(2)
  set targetChat($1.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasTargetChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearTargetChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.ChatRef ensureTargetChat() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get commentary => $_getSZ(2);
  @$pb.TagNumber(3)
  set commentary($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCommentary() => $_has(2);
  @$pb.TagNumber(3)
  void clearCommentary() => $_clearField(3);
}

class MarkReadRequest extends $pb.GeneratedMessage {
  factory MarkReadRequest({
    $1.ChatRef? chat,
    $core.String? lastReadMessageId,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (lastReadMessageId != null) result.lastReadMessageId = lastReadMessageId;
    return result;
  }

  MarkReadRequest._();

  factory MarkReadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MarkReadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkReadRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'lastReadMessageId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkReadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkReadRequest copyWith(void Function(MarkReadRequest) updates) =>
      super.copyWith((message) => updates(message as MarkReadRequest))
          as MarkReadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MarkReadRequest create() => MarkReadRequest._();
  @$core.override
  MarkReadRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MarkReadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkReadRequest>(create);
  static MarkReadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get lastReadMessageId => $_getSZ(1);
  @$pb.TagNumber(2)
  set lastReadMessageId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLastReadMessageId() => $_has(1);
  @$pb.TagNumber(2)
  void clearLastReadMessageId() => $_clearField(2);
}

class GetReadStateRequest extends $pb.GeneratedMessage {
  factory GetReadStateRequest({
    $1.ChatRef? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  GetReadStateRequest._();

  factory GetReadStateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetReadStateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetReadStateRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReadStateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReadStateRequest copyWith(void Function(GetReadStateRequest) updates) =>
      super.copyWith((message) => updates(message as GetReadStateRequest))
          as GetReadStateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetReadStateRequest create() => GetReadStateRequest._();
  @$core.override
  GetReadStateRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetReadStateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetReadStateRequest>(create);
  static GetReadStateRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);
}

class ReadState extends $pb.GeneratedMessage {
  factory ReadState({
    $1.ChatRef? chat,
    $core.String? profileId,
    $core.String? lastReadMessageId,
    $3.Timestamp? updatedAt,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (profileId != null) result.profileId = profileId;
    if (lastReadMessageId != null) result.lastReadMessageId = lastReadMessageId;
    if (updatedAt != null) result.updatedAt = updatedAt;
    return result;
  }

  ReadState._();

  factory ReadState.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadState.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadState',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'lastReadMessageId')
    ..aOM<$3.Timestamp>(4, _omitFieldNames ? '' : 'updatedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadState clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadState copyWith(void Function(ReadState) updates) =>
      super.copyWith((message) => updates(message as ReadState)) as ReadState;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadState create() => ReadState._();
  @$core.override
  ReadState createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadState getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReadState>(create);
  static ReadState? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get lastReadMessageId => $_getSZ(2);
  @$pb.TagNumber(3)
  set lastReadMessageId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLastReadMessageId() => $_has(2);
  @$pb.TagNumber(3)
  void clearLastReadMessageId() => $_clearField(3);

  @$pb.TagNumber(4)
  $3.Timestamp get updatedAt => $_getN(3);
  @$pb.TagNumber(4)
  set updatedAt($3.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasUpdatedAt() => $_has(3);
  @$pb.TagNumber(4)
  void clearUpdatedAt() => $_clearField(4);
  @$pb.TagNumber(4)
  $3.Timestamp ensureUpdatedAt() => $_ensure(3);
}

class GetBulkReadStateRequest extends $pb.GeneratedMessage {
  factory GetBulkReadStateRequest({
    $core.Iterable<$1.ChatRef>? chats,
  }) {
    final result = create();
    if (chats != null) result.chats.addAll(chats);
    return result;
  }

  GetBulkReadStateRequest._();

  factory GetBulkReadStateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBulkReadStateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBulkReadStateRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..pPM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chats',
        subBuilder: $1.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkReadStateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkReadStateRequest copyWith(
          void Function(GetBulkReadStateRequest) updates) =>
      super.copyWith((message) => updates(message as GetBulkReadStateRequest))
          as GetBulkReadStateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBulkReadStateRequest create() => GetBulkReadStateRequest._();
  @$core.override
  GetBulkReadStateRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBulkReadStateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBulkReadStateRequest>(create);
  static GetBulkReadStateRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$1.ChatRef> get chats => $_getList(0);
}

class GetChatListMetadataRequest extends $pb.GeneratedMessage {
  factory GetChatListMetadataRequest({
    $core.Iterable<$1.ChatRef>? chats,
  }) {
    final result = create();
    if (chats != null) result.chats.addAll(chats);
    return result;
  }

  GetChatListMetadataRequest._();

  factory GetChatListMetadataRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatListMetadataRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatListMetadataRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..pPM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chats',
        subBuilder: $1.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatListMetadataRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatListMetadataRequest copyWith(
          void Function(GetChatListMetadataRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetChatListMetadataRequest))
          as GetChatListMetadataRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatListMetadataRequest create() => GetChatListMetadataRequest._();
  @$core.override
  GetChatListMetadataRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatListMetadataRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatListMetadataRequest>(create);
  static GetChatListMetadataRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$1.ChatRef> get chats => $_getList(0);
}

class ChatListMetadata extends $pb.GeneratedMessage {
  factory ChatListMetadata({
    $1.ChatRef? chat,
    $core.String? lastMessagePreview,
    $fixnum.Int64? unreadCount,
    $3.Timestamp? lastMessageAt,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (lastMessagePreview != null)
      result.lastMessagePreview = lastMessagePreview;
    if (unreadCount != null) result.unreadCount = unreadCount;
    if (lastMessageAt != null) result.lastMessageAt = lastMessageAt;
    return result;
  }

  ChatListMetadata._();

  factory ChatListMetadata.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChatListMetadata.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatListMetadata',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'lastMessagePreview')
    ..aInt64(3, _omitFieldNames ? '' : 'unreadCount')
    ..aOM<$3.Timestamp>(4, _omitFieldNames ? '' : 'lastMessageAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatListMetadata clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatListMetadata copyWith(void Function(ChatListMetadata) updates) =>
      super.copyWith((message) => updates(message as ChatListMetadata))
          as ChatListMetadata;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChatListMetadata create() => ChatListMetadata._();
  @$core.override
  ChatListMetadata createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChatListMetadata getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ChatListMetadata>(create);
  static ChatListMetadata? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get lastMessagePreview => $_getSZ(1);
  @$pb.TagNumber(2)
  set lastMessagePreview($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLastMessagePreview() => $_has(1);
  @$pb.TagNumber(2)
  void clearLastMessagePreview() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get unreadCount => $_getI64(2);
  @$pb.TagNumber(3)
  set unreadCount($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasUnreadCount() => $_has(2);
  @$pb.TagNumber(3)
  void clearUnreadCount() => $_clearField(3);

  @$pb.TagNumber(4)
  $3.Timestamp get lastMessageAt => $_getN(3);
  @$pb.TagNumber(4)
  set lastMessageAt($3.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasLastMessageAt() => $_has(3);
  @$pb.TagNumber(4)
  void clearLastMessageAt() => $_clearField(4);
  @$pb.TagNumber(4)
  $3.Timestamp ensureLastMessageAt() => $_ensure(3);
}

class SendMessageResponse extends $pb.GeneratedMessage {
  factory SendMessageResponse({
    Message? message,
  }) {
    final result = create();
    if (message != null) result.message = message;
    return result;
  }

  SendMessageResponse._();

  factory SendMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: Message.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageResponse copyWith(void Function(SendMessageResponse) updates) =>
      super.copyWith((message) => updates(message as SendMessageResponse))
          as SendMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendMessageResponse create() => SendMessageResponse._();
  @$core.override
  SendMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendMessageResponse>(create);
  static SendMessageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message(Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  Message ensureMessage() => $_ensure(0);
}

class EditMessageResponse extends $pb.GeneratedMessage {
  factory EditMessageResponse({
    Message? message,
  }) {
    final result = create();
    if (message != null) result.message = message;
    return result;
  }

  EditMessageResponse._();

  factory EditMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EditMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EditMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: Message.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EditMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EditMessageResponse copyWith(void Function(EditMessageResponse) updates) =>
      super.copyWith((message) => updates(message as EditMessageResponse))
          as EditMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EditMessageResponse create() => EditMessageResponse._();
  @$core.override
  EditMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EditMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EditMessageResponse>(create);
  static EditMessageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message(Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  Message ensureMessage() => $_ensure(0);
}

class DeleteMessageResponse extends $pb.GeneratedMessage {
  factory DeleteMessageResponse() => create();

  DeleteMessageResponse._();

  factory DeleteMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteMessageResponse copyWith(
          void Function(DeleteMessageResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteMessageResponse))
          as DeleteMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteMessageResponse create() => DeleteMessageResponse._();
  @$core.override
  DeleteMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteMessageResponse>(create);
  static DeleteMessageResponse? _defaultInstance;
}

class GetMessagesResponse extends $pb.GeneratedMessage {
  factory GetMessagesResponse({
    MessageList? messageList,
  }) {
    final result = create();
    if (messageList != null) result.messageList = messageList;
    return result;
  }

  GetMessagesResponse._();

  factory GetMessagesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMessagesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMessagesResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<MessageList>(1, _omitFieldNames ? '' : 'messageList',
        subBuilder: MessageList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMessagesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMessagesResponse copyWith(void Function(GetMessagesResponse) updates) =>
      super.copyWith((message) => updates(message as GetMessagesResponse))
          as GetMessagesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMessagesResponse create() => GetMessagesResponse._();
  @$core.override
  GetMessagesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMessagesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMessagesResponse>(create);
  static GetMessagesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MessageList get messageList => $_getN(0);
  @$pb.TagNumber(1)
  set messageList(MessageList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageList() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageList() => $_clearField(1);
  @$pb.TagNumber(1)
  MessageList ensureMessageList() => $_ensure(0);
}

class GetMessageResponse extends $pb.GeneratedMessage {
  factory GetMessageResponse({
    Message? message,
  }) {
    final result = create();
    if (message != null) result.message = message;
    return result;
  }

  GetMessageResponse._();

  factory GetMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: Message.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMessageResponse copyWith(void Function(GetMessageResponse) updates) =>
      super.copyWith((message) => updates(message as GetMessageResponse))
          as GetMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMessageResponse create() => GetMessageResponse._();
  @$core.override
  GetMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMessageResponse>(create);
  static GetMessageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message(Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  Message ensureMessage() => $_ensure(0);
}

class GetThreadMessagesResponse extends $pb.GeneratedMessage {
  factory GetThreadMessagesResponse({
    MessageList? messageList,
  }) {
    final result = create();
    if (messageList != null) result.messageList = messageList;
    return result;
  }

  GetThreadMessagesResponse._();

  factory GetThreadMessagesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetThreadMessagesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetThreadMessagesResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<MessageList>(1, _omitFieldNames ? '' : 'messageList',
        subBuilder: MessageList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadMessagesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadMessagesResponse copyWith(
          void Function(GetThreadMessagesResponse) updates) =>
      super.copyWith((message) => updates(message as GetThreadMessagesResponse))
          as GetThreadMessagesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetThreadMessagesResponse create() => GetThreadMessagesResponse._();
  @$core.override
  GetThreadMessagesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetThreadMessagesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetThreadMessagesResponse>(create);
  static GetThreadMessagesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MessageList get messageList => $_getN(0);
  @$pb.TagNumber(1)
  set messageList(MessageList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageList() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageList() => $_clearField(1);
  @$pb.TagNumber(1)
  MessageList ensureMessageList() => $_ensure(0);
}

class ListThreadsResponse extends $pb.GeneratedMessage {
  factory ListThreadsResponse({
    ThreadList? threadList,
  }) {
    final result = create();
    if (threadList != null) result.threadList = threadList;
    return result;
  }

  ListThreadsResponse._();

  factory ListThreadsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListThreadsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListThreadsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<ThreadList>(1, _omitFieldNames ? '' : 'threadList',
        subBuilder: ThreadList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadsResponse copyWith(void Function(ListThreadsResponse) updates) =>
      super.copyWith((message) => updates(message as ListThreadsResponse))
          as ListThreadsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListThreadsResponse create() => ListThreadsResponse._();
  @$core.override
  ListThreadsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListThreadsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListThreadsResponse>(create);
  static ListThreadsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ThreadList get threadList => $_getN(0);
  @$pb.TagNumber(1)
  set threadList(ThreadList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadList() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadList() => $_clearField(1);
  @$pb.TagNumber(1)
  ThreadList ensureThreadList() => $_ensure(0);
}

class AddReactionResponse extends $pb.GeneratedMessage {
  factory AddReactionResponse() => create();

  AddReactionResponse._();

  factory AddReactionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddReactionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddReactionResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddReactionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddReactionResponse copyWith(void Function(AddReactionResponse) updates) =>
      super.copyWith((message) => updates(message as AddReactionResponse))
          as AddReactionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddReactionResponse create() => AddReactionResponse._();
  @$core.override
  AddReactionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddReactionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddReactionResponse>(create);
  static AddReactionResponse? _defaultInstance;
}

class RemoveReactionResponse extends $pb.GeneratedMessage {
  factory RemoveReactionResponse() => create();

  RemoveReactionResponse._();

  factory RemoveReactionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveReactionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveReactionResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveReactionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveReactionResponse copyWith(
          void Function(RemoveReactionResponse) updates) =>
      super.copyWith((message) => updates(message as RemoveReactionResponse))
          as RemoveReactionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveReactionResponse create() => RemoveReactionResponse._();
  @$core.override
  RemoveReactionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveReactionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveReactionResponse>(create);
  static RemoveReactionResponse? _defaultInstance;
}

class PinMessageResponse extends $pb.GeneratedMessage {
  factory PinMessageResponse() => create();

  PinMessageResponse._();

  factory PinMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PinMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PinMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PinMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PinMessageResponse copyWith(void Function(PinMessageResponse) updates) =>
      super.copyWith((message) => updates(message as PinMessageResponse))
          as PinMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PinMessageResponse create() => PinMessageResponse._();
  @$core.override
  PinMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PinMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PinMessageResponse>(create);
  static PinMessageResponse? _defaultInstance;
}

class UnpinMessageResponse extends $pb.GeneratedMessage {
  factory UnpinMessageResponse() => create();

  UnpinMessageResponse._();

  factory UnpinMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnpinMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpinMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpinMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpinMessageResponse copyWith(void Function(UnpinMessageResponse) updates) =>
      super.copyWith((message) => updates(message as UnpinMessageResponse))
          as UnpinMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnpinMessageResponse create() => UnpinMessageResponse._();
  @$core.override
  UnpinMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnpinMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpinMessageResponse>(create);
  static UnpinMessageResponse? _defaultInstance;
}

class GetPinnedMessagesResponse extends $pb.GeneratedMessage {
  factory GetPinnedMessagesResponse({
    MessageList? messageList,
  }) {
    final result = create();
    if (messageList != null) result.messageList = messageList;
    return result;
  }

  GetPinnedMessagesResponse._();

  factory GetPinnedMessagesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPinnedMessagesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPinnedMessagesResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<MessageList>(1, _omitFieldNames ? '' : 'messageList',
        subBuilder: MessageList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPinnedMessagesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPinnedMessagesResponse copyWith(
          void Function(GetPinnedMessagesResponse) updates) =>
      super.copyWith((message) => updates(message as GetPinnedMessagesResponse))
          as GetPinnedMessagesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPinnedMessagesResponse create() => GetPinnedMessagesResponse._();
  @$core.override
  GetPinnedMessagesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPinnedMessagesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPinnedMessagesResponse>(create);
  static GetPinnedMessagesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MessageList get messageList => $_getN(0);
  @$pb.TagNumber(1)
  set messageList(MessageList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageList() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageList() => $_clearField(1);
  @$pb.TagNumber(1)
  MessageList ensureMessageList() => $_ensure(0);
}

class ForwardMessageResponse extends $pb.GeneratedMessage {
  factory ForwardMessageResponse({
    Message? message,
  }) {
    final result = create();
    if (message != null) result.message = message;
    return result;
  }

  ForwardMessageResponse._();

  factory ForwardMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ForwardMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ForwardMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: Message.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ForwardMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ForwardMessageResponse copyWith(
          void Function(ForwardMessageResponse) updates) =>
      super.copyWith((message) => updates(message as ForwardMessageResponse))
          as ForwardMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ForwardMessageResponse create() => ForwardMessageResponse._();
  @$core.override
  ForwardMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ForwardMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ForwardMessageResponse>(create);
  static ForwardMessageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message(Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  Message ensureMessage() => $_ensure(0);
}

class MarkReadResponse extends $pb.GeneratedMessage {
  factory MarkReadResponse() => create();

  MarkReadResponse._();

  factory MarkReadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MarkReadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkReadResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkReadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkReadResponse copyWith(void Function(MarkReadResponse) updates) =>
      super.copyWith((message) => updates(message as MarkReadResponse))
          as MarkReadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MarkReadResponse create() => MarkReadResponse._();
  @$core.override
  MarkReadResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MarkReadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkReadResponse>(create);
  static MarkReadResponse? _defaultInstance;
}

class GetReadStateResponse extends $pb.GeneratedMessage {
  factory GetReadStateResponse({
    ReadState? readState,
  }) {
    final result = create();
    if (readState != null) result.readState = readState;
    return result;
  }

  GetReadStateResponse._();

  factory GetReadStateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetReadStateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetReadStateResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<ReadState>(1, _omitFieldNames ? '' : 'readState',
        subBuilder: ReadState.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReadStateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReadStateResponse copyWith(void Function(GetReadStateResponse) updates) =>
      super.copyWith((message) => updates(message as GetReadStateResponse))
          as GetReadStateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetReadStateResponse create() => GetReadStateResponse._();
  @$core.override
  GetReadStateResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetReadStateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetReadStateResponse>(create);
  static GetReadStateResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ReadState get readState => $_getN(0);
  @$pb.TagNumber(1)
  set readState(ReadState value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReadState() => $_has(0);
  @$pb.TagNumber(1)
  void clearReadState() => $_clearField(1);
  @$pb.TagNumber(1)
  ReadState ensureReadState() => $_ensure(0);
}

class GetBulkReadStateResponse extends $pb.GeneratedMessage {
  factory GetBulkReadStateResponse({
    $core.Iterable<$core.MapEntry<$core.String, ReadState>>? byChatId,
  }) {
    final result = create();
    if (byChatId != null) result.byChatId.addEntries(byChatId);
    return result;
  }

  GetBulkReadStateResponse._();

  factory GetBulkReadStateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBulkReadStateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBulkReadStateResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..m<$core.String, ReadState>(1, _omitFieldNames ? '' : 'byChatId',
        entryClassName: 'GetBulkReadStateResponse.ByChatIdEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: ReadState.create,
        valueDefaultOrMaker: ReadState.getDefault,
        packageName: const $pb.PackageName('voice.messaging.v1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkReadStateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkReadStateResponse copyWith(
          void Function(GetBulkReadStateResponse) updates) =>
      super.copyWith((message) => updates(message as GetBulkReadStateResponse))
          as GetBulkReadStateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBulkReadStateResponse create() => GetBulkReadStateResponse._();
  @$core.override
  GetBulkReadStateResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBulkReadStateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBulkReadStateResponse>(create);
  static GetBulkReadStateResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbMap<$core.String, ReadState> get byChatId => $_getMap(0);
}

class GetChatListMetadataResponse extends $pb.GeneratedMessage {
  factory GetChatListMetadataResponse({
    $core.Iterable<$core.MapEntry<$core.String, ChatListMetadata>>? byChatId,
  }) {
    final result = create();
    if (byChatId != null) result.byChatId.addEntries(byChatId);
    return result;
  }

  GetChatListMetadataResponse._();

  factory GetChatListMetadataResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatListMetadataResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatListMetadataResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..m<$core.String, ChatListMetadata>(1, _omitFieldNames ? '' : 'byChatId',
        entryClassName: 'GetChatListMetadataResponse.ByChatIdEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: ChatListMetadata.create,
        valueDefaultOrMaker: ChatListMetadata.getDefault,
        packageName: const $pb.PackageName('voice.messaging.v1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatListMetadataResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatListMetadataResponse copyWith(
          void Function(GetChatListMetadataResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetChatListMetadataResponse))
          as GetChatListMetadataResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatListMetadataResponse create() =>
      GetChatListMetadataResponse._();
  @$core.override
  GetChatListMetadataResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatListMetadataResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatListMetadataResponse>(create);
  static GetChatListMetadataResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbMap<$core.String, ChatListMetadata> get byChatId => $_getMap(0);
}

class ListSharedMediaRequest extends $pb.GeneratedMessage {
  factory ListSharedMediaRequest({
    $1.ChatRef? chat,
    SharedMediaKind? kind,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (kind != null) result.kind = kind;
    if (page != null) result.page = page;
    return result;
  }

  ListSharedMediaRequest._();

  factory ListSharedMediaRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListSharedMediaRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSharedMediaRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aE<SharedMediaKind>(2, _omitFieldNames ? '' : 'kind',
        enumValues: SharedMediaKind.values)
    ..aOM<$2.CursorPageRequest>(3, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSharedMediaRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSharedMediaRequest copyWith(
          void Function(ListSharedMediaRequest) updates) =>
      super.copyWith((message) => updates(message as ListSharedMediaRequest))
          as ListSharedMediaRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListSharedMediaRequest create() => ListSharedMediaRequest._();
  @$core.override
  ListSharedMediaRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListSharedMediaRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSharedMediaRequest>(create);
  static ListSharedMediaRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  SharedMediaKind get kind => $_getN(1);
  @$pb.TagNumber(2)
  set kind(SharedMediaKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);

  @$pb.TagNumber(3)
  $2.CursorPageRequest get page => $_getN(2);
  @$pb.TagNumber(3)
  set page($2.CursorPageRequest value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPage() => $_has(2);
  @$pb.TagNumber(3)
  void clearPage() => $_clearField(3);
  @$pb.TagNumber(3)
  $2.CursorPageRequest ensurePage() => $_ensure(2);
}

class SharedMediaItem extends $pb.GeneratedMessage {
  factory SharedMediaItem({
    $core.String? messageId,
    $core.String? senderProfileId,
    $3.Timestamp? createdAt,
    $core.String? fileId,
    $core.String? attachmentType,
    $core.String? externalUrl,
    $core.String? title,
    $core.int? sortOrder,
    $core.String? originalName,
    $fixnum.Int64? sizeBytes,
    $core.String? e2eKeyWire,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (senderProfileId != null) result.senderProfileId = senderProfileId;
    if (createdAt != null) result.createdAt = createdAt;
    if (fileId != null) result.fileId = fileId;
    if (attachmentType != null) result.attachmentType = attachmentType;
    if (externalUrl != null) result.externalUrl = externalUrl;
    if (title != null) result.title = title;
    if (sortOrder != null) result.sortOrder = sortOrder;
    if (originalName != null) result.originalName = originalName;
    if (sizeBytes != null) result.sizeBytes = sizeBytes;
    if (e2eKeyWire != null) result.e2eKeyWire = e2eKeyWire;
    return result;
  }

  SharedMediaItem._();

  factory SharedMediaItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SharedMediaItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SharedMediaItem',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'senderProfileId')
    ..aOM<$3.Timestamp>(3, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $3.Timestamp.create)
    ..aOS(4, _omitFieldNames ? '' : 'fileId')
    ..aOS(5, _omitFieldNames ? '' : 'attachmentType')
    ..aOS(6, _omitFieldNames ? '' : 'externalUrl')
    ..aOS(7, _omitFieldNames ? '' : 'title')
    ..aI(8, _omitFieldNames ? '' : 'sortOrder')
    ..aOS(9, _omitFieldNames ? '' : 'originalName')
    ..aInt64(10, _omitFieldNames ? '' : 'sizeBytes')
    ..aOS(11, _omitFieldNames ? '' : 'e2eKeyWire')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SharedMediaItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SharedMediaItem copyWith(void Function(SharedMediaItem) updates) =>
      super.copyWith((message) => updates(message as SharedMediaItem))
          as SharedMediaItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SharedMediaItem create() => SharedMediaItem._();
  @$core.override
  SharedMediaItem createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SharedMediaItem getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SharedMediaItem>(create);
  static SharedMediaItem? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get senderProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set senderProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSenderProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSenderProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $3.Timestamp get createdAt => $_getN(2);
  @$pb.TagNumber(3)
  set createdAt($3.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCreatedAt() => $_has(2);
  @$pb.TagNumber(3)
  void clearCreatedAt() => $_clearField(3);
  @$pb.TagNumber(3)
  $3.Timestamp ensureCreatedAt() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get fileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set fileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearFileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get attachmentType => $_getSZ(4);
  @$pb.TagNumber(5)
  set attachmentType($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAttachmentType() => $_has(4);
  @$pb.TagNumber(5)
  void clearAttachmentType() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get externalUrl => $_getSZ(5);
  @$pb.TagNumber(6)
  set externalUrl($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasExternalUrl() => $_has(5);
  @$pb.TagNumber(6)
  void clearExternalUrl() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get title => $_getSZ(6);
  @$pb.TagNumber(7)
  set title($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasTitle() => $_has(6);
  @$pb.TagNumber(7)
  void clearTitle() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.int get sortOrder => $_getIZ(7);
  @$pb.TagNumber(8)
  set sortOrder($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasSortOrder() => $_has(7);
  @$pb.TagNumber(8)
  void clearSortOrder() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get originalName => $_getSZ(8);
  @$pb.TagNumber(9)
  set originalName($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasOriginalName() => $_has(8);
  @$pb.TagNumber(9)
  void clearOriginalName() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get sizeBytes => $_getI64(9);
  @$pb.TagNumber(10)
  set sizeBytes($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasSizeBytes() => $_has(9);
  @$pb.TagNumber(10)
  void clearSizeBytes() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get e2eKeyWire => $_getSZ(10);
  @$pb.TagNumber(11)
  set e2eKeyWire($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasE2eKeyWire() => $_has(10);
  @$pb.TagNumber(11)
  void clearE2eKeyWire() => $_clearField(11);
}

class SharedMediaList extends $pb.GeneratedMessage {
  factory SharedMediaList({
    $core.Iterable<SharedMediaItem>? items,
    $core.String? nextCursor,
    $core.bool? hasMore,
    $2.CursorPageResponse? page,
  }) {
    final result = create();
    if (items != null) result.items.addAll(items);
    if (nextCursor != null) result.nextCursor = nextCursor;
    if (hasMore != null) result.hasMore = hasMore;
    if (page != null) result.page = page;
    return result;
  }

  SharedMediaList._();

  factory SharedMediaList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SharedMediaList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SharedMediaList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..pPM<SharedMediaItem>(1, _omitFieldNames ? '' : 'items',
        subBuilder: SharedMediaItem.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..aOB(3, _omitFieldNames ? '' : 'hasMore')
    ..aOM<$2.CursorPageResponse>(4, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SharedMediaList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SharedMediaList copyWith(void Function(SharedMediaList) updates) =>
      super.copyWith((message) => updates(message as SharedMediaList))
          as SharedMediaList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SharedMediaList create() => SharedMediaList._();
  @$core.override
  SharedMediaList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SharedMediaList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SharedMediaList>(create);
  static SharedMediaList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SharedMediaItem> get items => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get hasMore => $_getBF(2);
  @$pb.TagNumber(3)
  set hasMore($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasHasMore() => $_has(2);
  @$pb.TagNumber(3)
  void clearHasMore() => $_clearField(3);

  @$pb.TagNumber(4)
  $2.CursorPageResponse get page => $_getN(3);
  @$pb.TagNumber(4)
  set page($2.CursorPageResponse value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPage() => $_has(3);
  @$pb.TagNumber(4)
  void clearPage() => $_clearField(4);
  @$pb.TagNumber(4)
  $2.CursorPageResponse ensurePage() => $_ensure(3);
}

class ListSharedMediaResponse extends $pb.GeneratedMessage {
  factory ListSharedMediaResponse({
    SharedMediaList? sharedMediaList,
  }) {
    final result = create();
    if (sharedMediaList != null) result.sharedMediaList = sharedMediaList;
    return result;
  }

  ListSharedMediaResponse._();

  factory ListSharedMediaResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListSharedMediaResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSharedMediaResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<SharedMediaList>(1, _omitFieldNames ? '' : 'sharedMediaList',
        subBuilder: SharedMediaList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSharedMediaResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSharedMediaResponse copyWith(
          void Function(ListSharedMediaResponse) updates) =>
      super.copyWith((message) => updates(message as ListSharedMediaResponse))
          as ListSharedMediaResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListSharedMediaResponse create() => ListSharedMediaResponse._();
  @$core.override
  ListSharedMediaResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListSharedMediaResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSharedMediaResponse>(create);
  static ListSharedMediaResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SharedMediaList get sharedMediaList => $_getN(0);
  @$pb.TagNumber(1)
  set sharedMediaList(SharedMediaList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSharedMediaList() => $_has(0);
  @$pb.TagNumber(1)
  void clearSharedMediaList() => $_clearField(1);
  @$pb.TagNumber(1)
  SharedMediaList ensureSharedMediaList() => $_ensure(0);
}

class UploadPreKeyBundleRequest extends $pb.GeneratedMessage {
  factory UploadPreKeyBundleRequest({
    $core.String? bundle,
  }) {
    final result = create();
    if (bundle != null) result.bundle = bundle;
    return result;
  }

  UploadPreKeyBundleRequest._();

  factory UploadPreKeyBundleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UploadPreKeyBundleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UploadPreKeyBundleRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'bundle')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UploadPreKeyBundleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UploadPreKeyBundleRequest copyWith(
          void Function(UploadPreKeyBundleRequest) updates) =>
      super.copyWith((message) => updates(message as UploadPreKeyBundleRequest))
          as UploadPreKeyBundleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UploadPreKeyBundleRequest create() => UploadPreKeyBundleRequest._();
  @$core.override
  UploadPreKeyBundleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UploadPreKeyBundleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UploadPreKeyBundleRequest>(create);
  static UploadPreKeyBundleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get bundle => $_getSZ(0);
  @$pb.TagNumber(1)
  set bundle($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBundle() => $_has(0);
  @$pb.TagNumber(1)
  void clearBundle() => $_clearField(1);
}

class UploadPreKeyBundleResponse extends $pb.GeneratedMessage {
  factory UploadPreKeyBundleResponse() => create();

  UploadPreKeyBundleResponse._();

  factory UploadPreKeyBundleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UploadPreKeyBundleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UploadPreKeyBundleResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UploadPreKeyBundleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UploadPreKeyBundleResponse copyWith(
          void Function(UploadPreKeyBundleResponse) updates) =>
      super.copyWith(
              (message) => updates(message as UploadPreKeyBundleResponse))
          as UploadPreKeyBundleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UploadPreKeyBundleResponse create() => UploadPreKeyBundleResponse._();
  @$core.override
  UploadPreKeyBundleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UploadPreKeyBundleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UploadPreKeyBundleResponse>(create);
  static UploadPreKeyBundleResponse? _defaultInstance;
}

class UnpinMessagesBySenderInChatsRequest extends $pb.GeneratedMessage {
  factory UnpinMessagesBySenderInChatsRequest({
    $core.String? senderProfileId,
    $core.Iterable<$core.String>? chatIds,
  }) {
    final result = create();
    if (senderProfileId != null) result.senderProfileId = senderProfileId;
    if (chatIds != null) result.chatIds.addAll(chatIds);
    return result;
  }

  UnpinMessagesBySenderInChatsRequest._();

  factory UnpinMessagesBySenderInChatsRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnpinMessagesBySenderInChatsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpinMessagesBySenderInChatsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'senderProfileId')
    ..pPS(2, _omitFieldNames ? '' : 'chatIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpinMessagesBySenderInChatsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpinMessagesBySenderInChatsRequest copyWith(
          void Function(UnpinMessagesBySenderInChatsRequest) updates) =>
      super.copyWith((message) =>
              updates(message as UnpinMessagesBySenderInChatsRequest))
          as UnpinMessagesBySenderInChatsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnpinMessagesBySenderInChatsRequest create() =>
      UnpinMessagesBySenderInChatsRequest._();
  @$core.override
  UnpinMessagesBySenderInChatsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnpinMessagesBySenderInChatsRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          UnpinMessagesBySenderInChatsRequest>(create);
  static UnpinMessagesBySenderInChatsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get senderProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set senderProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSenderProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSenderProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get chatIds => $_getList(1);
}

class UnpinMessagesBySenderInChatsResponse extends $pb.GeneratedMessage {
  factory UnpinMessagesBySenderInChatsResponse() => create();

  UnpinMessagesBySenderInChatsResponse._();

  factory UnpinMessagesBySenderInChatsResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnpinMessagesBySenderInChatsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpinMessagesBySenderInChatsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpinMessagesBySenderInChatsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpinMessagesBySenderInChatsResponse copyWith(
          void Function(UnpinMessagesBySenderInChatsResponse) updates) =>
      super.copyWith((message) =>
              updates(message as UnpinMessagesBySenderInChatsResponse))
          as UnpinMessagesBySenderInChatsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnpinMessagesBySenderInChatsResponse create() =>
      UnpinMessagesBySenderInChatsResponse._();
  @$core.override
  UnpinMessagesBySenderInChatsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnpinMessagesBySenderInChatsResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          UnpinMessagesBySenderInChatsResponse>(create);
  static UnpinMessagesBySenderInChatsResponse? _defaultInstance;
}

class GetPreKeyBundleRequest extends $pb.GeneratedMessage {
  factory GetPreKeyBundleRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetPreKeyBundleRequest._();

  factory GetPreKeyBundleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPreKeyBundleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPreKeyBundleRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPreKeyBundleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPreKeyBundleRequest copyWith(
          void Function(GetPreKeyBundleRequest) updates) =>
      super.copyWith((message) => updates(message as GetPreKeyBundleRequest))
          as GetPreKeyBundleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPreKeyBundleRequest create() => GetPreKeyBundleRequest._();
  @$core.override
  GetPreKeyBundleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPreKeyBundleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPreKeyBundleRequest>(create);
  static GetPreKeyBundleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class GetPreKeyBundleResponse extends $pb.GeneratedMessage {
  factory GetPreKeyBundleResponse({
    $core.String? bundle,
  }) {
    final result = create();
    if (bundle != null) result.bundle = bundle;
    return result;
  }

  GetPreKeyBundleResponse._();

  factory GetPreKeyBundleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPreKeyBundleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPreKeyBundleResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'bundle')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPreKeyBundleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPreKeyBundleResponse copyWith(
          void Function(GetPreKeyBundleResponse) updates) =>
      super.copyWith((message) => updates(message as GetPreKeyBundleResponse))
          as GetPreKeyBundleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPreKeyBundleResponse create() => GetPreKeyBundleResponse._();
  @$core.override
  GetPreKeyBundleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPreKeyBundleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPreKeyBundleResponse>(create);
  static GetPreKeyBundleResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get bundle => $_getSZ(0);
  @$pb.TagNumber(1)
  set bundle($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBundle() => $_has(0);
  @$pb.TagNumber(1)
  void clearBundle() => $_clearField(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
