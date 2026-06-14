// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat.proto.

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
    as $1;

import '../../common/v1/common.pb.dart' as $2;
import 'chat.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'chat.pbenum.dart';

class Chat extends $pb.GeneratedMessage {
  factory Chat({
    $core.String? id,
    ChatType? type,
    $core.String? spaceId,
    $core.String? name,
    $core.String? avatarUrl,
    $core.String? topic,
    $core.String? creatorProfileId,
    $core.int? slowModeSeconds,
    $1.Timestamp? lastMessageAt,
    $1.Timestamp? createdAt,
    $1.Timestamp? updatedAt,
    $core.bool? threadsEnabled,
    $core.bool? allowUserMainFeed,
    $core.bool? e2eEnabled,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (type != null) result.type = type;
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    if (avatarUrl != null) result.avatarUrl = avatarUrl;
    if (topic != null) result.topic = topic;
    if (creatorProfileId != null) result.creatorProfileId = creatorProfileId;
    if (slowModeSeconds != null) result.slowModeSeconds = slowModeSeconds;
    if (lastMessageAt != null) result.lastMessageAt = lastMessageAt;
    if (createdAt != null) result.createdAt = createdAt;
    if (updatedAt != null) result.updatedAt = updatedAt;
    if (threadsEnabled != null) result.threadsEnabled = threadsEnabled;
    if (allowUserMainFeed != null) result.allowUserMainFeed = allowUserMainFeed;
    if (e2eEnabled != null) result.e2eEnabled = e2eEnabled;
    return result;
  }

  Chat._();

  factory Chat.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Chat.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Chat',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<ChatType>(2, _omitFieldNames ? '' : 'type',
        enumValues: ChatType.values)
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..aOS(4, _omitFieldNames ? '' : 'name')
    ..aOS(5, _omitFieldNames ? '' : 'avatarUrl')
    ..aOS(6, _omitFieldNames ? '' : 'topic')
    ..aOS(7, _omitFieldNames ? '' : 'creatorProfileId')
    ..aI(8, _omitFieldNames ? '' : 'slowModeSeconds')
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'lastMessageAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(11, _omitFieldNames ? '' : 'updatedAt',
        subBuilder: $1.Timestamp.create)
    ..aOB(12, _omitFieldNames ? '' : 'threadsEnabled')
    ..aOB(13, _omitFieldNames ? '' : 'allowUserMainFeed')
    ..aOB(14, _omitFieldNames ? '' : 'e2eEnabled')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Chat clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Chat copyWith(void Function(Chat) updates) =>
      super.copyWith((message) => updates(message as Chat)) as Chat;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Chat create() => Chat._();
  @$core.override
  Chat createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Chat getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Chat>(create);
  static Chat? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  ChatType get type => $_getN(1);
  @$pb.TagNumber(2)
  set type(ChatType value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get name => $_getSZ(3);
  @$pb.TagNumber(4)
  set name($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasName() => $_has(3);
  @$pb.TagNumber(4)
  void clearName() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get avatarUrl => $_getSZ(4);
  @$pb.TagNumber(5)
  set avatarUrl($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAvatarUrl() => $_has(4);
  @$pb.TagNumber(5)
  void clearAvatarUrl() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get topic => $_getSZ(5);
  @$pb.TagNumber(6)
  set topic($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasTopic() => $_has(5);
  @$pb.TagNumber(6)
  void clearTopic() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get creatorProfileId => $_getSZ(6);
  @$pb.TagNumber(7)
  set creatorProfileId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasCreatorProfileId() => $_has(6);
  @$pb.TagNumber(7)
  void clearCreatorProfileId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.int get slowModeSeconds => $_getIZ(7);
  @$pb.TagNumber(8)
  set slowModeSeconds($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasSlowModeSeconds() => $_has(7);
  @$pb.TagNumber(8)
  void clearSlowModeSeconds() => $_clearField(8);

  @$pb.TagNumber(9)
  $1.Timestamp get lastMessageAt => $_getN(8);
  @$pb.TagNumber(9)
  set lastMessageAt($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasLastMessageAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearLastMessageAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureLastMessageAt() => $_ensure(8);

  @$pb.TagNumber(10)
  $1.Timestamp get createdAt => $_getN(9);
  @$pb.TagNumber(10)
  set createdAt($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCreatedAt() => $_has(9);
  @$pb.TagNumber(10)
  void clearCreatedAt() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureCreatedAt() => $_ensure(9);

  @$pb.TagNumber(11)
  $1.Timestamp get updatedAt => $_getN(10);
  @$pb.TagNumber(11)
  set updatedAt($1.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasUpdatedAt() => $_has(10);
  @$pb.TagNumber(11)
  void clearUpdatedAt() => $_clearField(11);
  @$pb.TagNumber(11)
  $1.Timestamp ensureUpdatedAt() => $_ensure(10);

  @$pb.TagNumber(12)
  $core.bool get threadsEnabled => $_getBF(11);
  @$pb.TagNumber(12)
  set threadsEnabled($core.bool value) => $_setBool(11, value);
  @$pb.TagNumber(12)
  $core.bool hasThreadsEnabled() => $_has(11);
  @$pb.TagNumber(12)
  void clearThreadsEnabled() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.bool get allowUserMainFeed => $_getBF(12);
  @$pb.TagNumber(13)
  set allowUserMainFeed($core.bool value) => $_setBool(12, value);
  @$pb.TagNumber(13)
  $core.bool hasAllowUserMainFeed() => $_has(12);
  @$pb.TagNumber(13)
  void clearAllowUserMainFeed() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.bool get e2eEnabled => $_getBF(13);
  @$pb.TagNumber(14)
  set e2eEnabled($core.bool value) => $_setBool(13, value);
  @$pb.TagNumber(14)
  $core.bool hasE2eEnabled() => $_has(13);
  @$pb.TagNumber(14)
  void clearE2eEnabled() => $_clearField(14);
}

class CreateDMRequest extends $pb.GeneratedMessage {
  factory CreateDMRequest({
    $core.String? otherProfileId,
  }) {
    final result = create();
    if (otherProfileId != null) result.otherProfileId = otherProfileId;
    return result;
  }

  CreateDMRequest._();

  factory CreateDMRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateDMRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateDMRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'otherProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateDMRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateDMRequest copyWith(void Function(CreateDMRequest) updates) =>
      super.copyWith((message) => updates(message as CreateDMRequest))
          as CreateDMRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateDMRequest create() => CreateDMRequest._();
  @$core.override
  CreateDMRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateDMRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateDMRequest>(create);
  static CreateDMRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get otherProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set otherProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOtherProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOtherProfileId() => $_clearField(1);
}

class GetDMRequest extends $pb.GeneratedMessage {
  factory GetDMRequest({
    $core.String? otherProfileId,
  }) {
    final result = create();
    if (otherProfileId != null) result.otherProfileId = otherProfileId;
    return result;
  }

  GetDMRequest._();

  factory GetDMRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetDMRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetDMRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'otherProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDMRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDMRequest copyWith(void Function(GetDMRequest) updates) =>
      super.copyWith((message) => updates(message as GetDMRequest))
          as GetDMRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetDMRequest create() => GetDMRequest._();
  @$core.override
  GetDMRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetDMRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetDMRequest>(create);
  static GetDMRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get otherProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set otherProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOtherProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOtherProfileId() => $_clearField(1);
}

class CreateChatRequest extends $pb.GeneratedMessage {
  factory CreateChatRequest({
    ChatType? type,
    $core.String? spaceId,
    $core.String? name,
    $core.String? topic,
  }) {
    final result = create();
    if (type != null) result.type = type;
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    if (topic != null) result.topic = topic;
    return result;
  }

  CreateChatRequest._();

  factory CreateChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateChatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aE<ChatType>(1, _omitFieldNames ? '' : 'type',
        enumValues: ChatType.values)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..aOS(4, _omitFieldNames ? '' : 'topic')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateChatRequest copyWith(void Function(CreateChatRequest) updates) =>
      super.copyWith((message) => updates(message as CreateChatRequest))
          as CreateChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateChatRequest create() => CreateChatRequest._();
  @$core.override
  CreateChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateChatRequest>(create);
  static CreateChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  ChatType get type => $_getN(0);
  @$pb.TagNumber(1)
  set type(ChatType value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasType() => $_has(0);
  @$pb.TagNumber(1)
  void clearType() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get topic => $_getSZ(3);
  @$pb.TagNumber(4)
  set topic($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTopic() => $_has(3);
  @$pb.TagNumber(4)
  void clearTopic() => $_clearField(4);
}

class UpdateChatRequest extends $pb.GeneratedMessage {
  factory UpdateChatRequest({
    $core.String? chatId,
    $core.String? name,
    $core.String? topic,
    $core.int? slowModeSeconds,
    $core.String? avatarUrl,
    $core.bool? threadsEnabled,
    $core.bool? allowUserMainFeed,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (name != null) result.name = name;
    if (topic != null) result.topic = topic;
    if (slowModeSeconds != null) result.slowModeSeconds = slowModeSeconds;
    if (avatarUrl != null) result.avatarUrl = avatarUrl;
    if (threadsEnabled != null) result.threadsEnabled = threadsEnabled;
    if (allowUserMainFeed != null) result.allowUserMainFeed = allowUserMainFeed;
    return result;
  }

  UpdateChatRequest._();

  factory UpdateChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateChatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'topic')
    ..aI(4, _omitFieldNames ? '' : 'slowModeSeconds')
    ..aOS(5, _omitFieldNames ? '' : 'avatarUrl')
    ..aOB(6, _omitFieldNames ? '' : 'threadsEnabled')
    ..aOB(7, _omitFieldNames ? '' : 'allowUserMainFeed')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateChatRequest copyWith(void Function(UpdateChatRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateChatRequest))
          as UpdateChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateChatRequest create() => UpdateChatRequest._();
  @$core.override
  UpdateChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateChatRequest>(create);
  static UpdateChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get topic => $_getSZ(2);
  @$pb.TagNumber(3)
  set topic($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTopic() => $_has(2);
  @$pb.TagNumber(3)
  void clearTopic() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get slowModeSeconds => $_getIZ(3);
  @$pb.TagNumber(4)
  set slowModeSeconds($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSlowModeSeconds() => $_has(3);
  @$pb.TagNumber(4)
  void clearSlowModeSeconds() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get avatarUrl => $_getSZ(4);
  @$pb.TagNumber(5)
  set avatarUrl($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAvatarUrl() => $_has(4);
  @$pb.TagNumber(5)
  void clearAvatarUrl() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get threadsEnabled => $_getBF(5);
  @$pb.TagNumber(6)
  set threadsEnabled($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasThreadsEnabled() => $_has(5);
  @$pb.TagNumber(6)
  void clearThreadsEnabled() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get allowUserMainFeed => $_getBF(6);
  @$pb.TagNumber(7)
  set allowUserMainFeed($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasAllowUserMainFeed() => $_has(6);
  @$pb.TagNumber(7)
  void clearAllowUserMainFeed() => $_clearField(7);
}

class DeleteChatRequest extends $pb.GeneratedMessage {
  factory DeleteChatRequest({
    $core.String? chatId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  DeleteChatRequest._();

  factory DeleteChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteChatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteChatRequest copyWith(void Function(DeleteChatRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteChatRequest))
          as DeleteChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteChatRequest create() => DeleteChatRequest._();
  @$core.override
  DeleteChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteChatRequest>(create);
  static DeleteChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);
}

class AddMembersRequest extends $pb.GeneratedMessage {
  factory AddMembersRequest({
    $core.String? chatId,
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  AddMembersRequest._();

  factory AddMembersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddMembersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddMembersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..pPS(2, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddMembersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddMembersRequest copyWith(void Function(AddMembersRequest) updates) =>
      super.copyWith((message) => updates(message as AddMembersRequest))
          as AddMembersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddMembersRequest create() => AddMembersRequest._();
  @$core.override
  AddMembersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddMembersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddMembersRequest>(create);
  static AddMembersRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get profileIds => $_getList(1);
}

class RemoveMemberRequest extends $pb.GeneratedMessage {
  factory RemoveMemberRequest({
    $core.String? chatId,
    $core.String? profileId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  RemoveMemberRequest._();

  factory RemoveMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveMemberRequest copyWith(void Function(RemoveMemberRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveMemberRequest))
          as RemoveMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveMemberRequest create() => RemoveMemberRequest._();
  @$core.override
  RemoveMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveMemberRequest>(create);
  static RemoveMemberRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

class LeaveChatRequest extends $pb.GeneratedMessage {
  factory LeaveChatRequest({
    $core.String? chatId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  LeaveChatRequest._();

  factory LeaveChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveChatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveChatRequest copyWith(void Function(LeaveChatRequest) updates) =>
      super.copyWith((message) => updates(message as LeaveChatRequest))
          as LeaveChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveChatRequest create() => LeaveChatRequest._();
  @$core.override
  LeaveChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveChatRequest>(create);
  static LeaveChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);
}

class TransferGroupOwnershipRequest extends $pb.GeneratedMessage {
  factory TransferGroupOwnershipRequest({
    $core.String? chatId,
    $core.String? newOwnerProfileId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (newOwnerProfileId != null) result.newOwnerProfileId = newOwnerProfileId;
    return result;
  }

  TransferGroupOwnershipRequest._();

  factory TransferGroupOwnershipRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TransferGroupOwnershipRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TransferGroupOwnershipRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOS(2, _omitFieldNames ? '' : 'newOwnerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransferGroupOwnershipRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransferGroupOwnershipRequest copyWith(
          void Function(TransferGroupOwnershipRequest) updates) =>
      super.copyWith(
              (message) => updates(message as TransferGroupOwnershipRequest))
          as TransferGroupOwnershipRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TransferGroupOwnershipRequest create() =>
      TransferGroupOwnershipRequest._();
  @$core.override
  TransferGroupOwnershipRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TransferGroupOwnershipRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TransferGroupOwnershipRequest>(create);
  static TransferGroupOwnershipRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get newOwnerProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set newOwnerProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNewOwnerProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearNewOwnerProfileId() => $_clearField(2);
}

class TransferGroupOwnershipResponse extends $pb.GeneratedMessage {
  factory TransferGroupOwnershipResponse() => create();

  TransferGroupOwnershipResponse._();

  factory TransferGroupOwnershipResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TransferGroupOwnershipResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TransferGroupOwnershipResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransferGroupOwnershipResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransferGroupOwnershipResponse copyWith(
          void Function(TransferGroupOwnershipResponse) updates) =>
      super.copyWith(
              (message) => updates(message as TransferGroupOwnershipResponse))
          as TransferGroupOwnershipResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TransferGroupOwnershipResponse create() =>
      TransferGroupOwnershipResponse._();
  @$core.override
  TransferGroupOwnershipResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TransferGroupOwnershipResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TransferGroupOwnershipResponse>(create);
  static TransferGroupOwnershipResponse? _defaultInstance;
}

class ListMembersRequest extends $pb.GeneratedMessage {
  factory ListMembersRequest({
    $core.String? chatId,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (page != null) result.page = page;
    return result;
  }

  ListMembersRequest._();

  factory ListMembersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMembersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMembersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersRequest copyWith(void Function(ListMembersRequest) updates) =>
      super.copyWith((message) => updates(message as ListMembersRequest))
          as ListMembersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMembersRequest create() => ListMembersRequest._();
  @$core.override
  ListMembersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMembersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMembersRequest>(create);
  static ListMembersRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

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

class MemberList extends $pb.GeneratedMessage {
  factory MemberList({
    $core.Iterable<ChatMember>? members,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (members != null) result.members.addAll(members);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  MemberList._();

  factory MemberList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..pPM<ChatMember>(1, _omitFieldNames ? '' : 'members',
        subBuilder: ChatMember.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberList copyWith(void Function(MemberList) updates) =>
      super.copyWith((message) => updates(message as MemberList)) as MemberList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberList create() => MemberList._();
  @$core.override
  MemberList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberList>(create);
  static MemberList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ChatMember> get members => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class ChatMember extends $pb.GeneratedMessage {
  factory ChatMember({
    $core.String? profileId,
    $core.String? role,
    $1.Timestamp? joinedAt,
    $1.Timestamp? mutedUntil,
    $core.bool? isArchived,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (role != null) result.role = role;
    if (joinedAt != null) result.joinedAt = joinedAt;
    if (mutedUntil != null) result.mutedUntil = mutedUntil;
    if (isArchived != null) result.isArchived = isArchived;
    return result;
  }

  ChatMember._();

  factory ChatMember.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChatMember.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatMember',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'role')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'joinedAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'mutedUntil',
        subBuilder: $1.Timestamp.create)
    ..aOB(5, _omitFieldNames ? '' : 'isArchived')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatMember clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatMember copyWith(void Function(ChatMember) updates) =>
      super.copyWith((message) => updates(message as ChatMember)) as ChatMember;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChatMember create() => ChatMember._();
  @$core.override
  ChatMember createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChatMember getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ChatMember>(create);
  static ChatMember? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get role => $_getSZ(1);
  @$pb.TagNumber(2)
  set role($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRole() => $_has(1);
  @$pb.TagNumber(2)
  void clearRole() => $_clearField(2);

  @$pb.TagNumber(3)
  $1.Timestamp get joinedAt => $_getN(2);
  @$pb.TagNumber(3)
  set joinedAt($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasJoinedAt() => $_has(2);
  @$pb.TagNumber(3)
  void clearJoinedAt() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureJoinedAt() => $_ensure(2);

  @$pb.TagNumber(4)
  $1.Timestamp get mutedUntil => $_getN(3);
  @$pb.TagNumber(4)
  set mutedUntil($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasMutedUntil() => $_has(3);
  @$pb.TagNumber(4)
  void clearMutedUntil() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureMutedUntil() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.bool get isArchived => $_getBF(4);
  @$pb.TagNumber(5)
  set isArchived($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasIsArchived() => $_has(4);
  @$pb.TagNumber(5)
  void clearIsArchived() => $_clearField(5);
}

class ListChatsRequest extends $pb.GeneratedMessage {
  factory ListChatsRequest({
    $2.CursorPageRequest? page,
    $core.String? inbox,
  }) {
    final result = create();
    if (page != null) result.page = page;
    if (inbox != null) result.inbox = inbox;
    return result;
  }

  ListChatsRequest._();

  factory ListChatsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListChatsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListChatsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<$2.CursorPageRequest>(1, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..aOS(2, _omitFieldNames ? '' : 'inbox')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListChatsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListChatsRequest copyWith(void Function(ListChatsRequest) updates) =>
      super.copyWith((message) => updates(message as ListChatsRequest))
          as ListChatsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListChatsRequest create() => ListChatsRequest._();
  @$core.override
  ListChatsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListChatsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListChatsRequest>(create);
  static ListChatsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $2.CursorPageRequest get page => $_getN(0);
  @$pb.TagNumber(1)
  set page($2.CursorPageRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPage() => $_has(0);
  @$pb.TagNumber(1)
  void clearPage() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.CursorPageRequest ensurePage() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get inbox => $_getSZ(1);
  @$pb.TagNumber(2)
  set inbox($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInbox() => $_has(1);
  @$pb.TagNumber(2)
  void clearInbox() => $_clearField(2);
}

/// One row in the inbox: chat metadata plus list fields sourced from Messaging (S2S) when configured.
class ChatListItem extends $pb.GeneratedMessage {
  factory ChatListItem({
    Chat? chat,
    $core.String? lastMessagePreview,
    $fixnum.Int64? unreadCount,
    $core.String? inbox,
    $core.bool? isStranger,
    $core.String? dmPeerProfileId,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (lastMessagePreview != null)
      result.lastMessagePreview = lastMessagePreview;
    if (unreadCount != null) result.unreadCount = unreadCount;
    if (inbox != null) result.inbox = inbox;
    if (isStranger != null) result.isStranger = isStranger;
    if (dmPeerProfileId != null) result.dmPeerProfileId = dmPeerProfileId;
    return result;
  }

  ChatListItem._();

  factory ChatListItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChatListItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatListItem',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<Chat>(1, _omitFieldNames ? '' : 'chat', subBuilder: Chat.create)
    ..aOS(2, _omitFieldNames ? '' : 'lastMessagePreview')
    ..aInt64(3, _omitFieldNames ? '' : 'unreadCount')
    ..aOS(4, _omitFieldNames ? '' : 'inbox')
    ..aOB(5, _omitFieldNames ? '' : 'isStranger')
    ..aOS(6, _omitFieldNames ? '' : 'dmPeerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatListItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatListItem copyWith(void Function(ChatListItem) updates) =>
      super.copyWith((message) => updates(message as ChatListItem))
          as ChatListItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChatListItem create() => ChatListItem._();
  @$core.override
  ChatListItem createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChatListItem getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ChatListItem>(create);
  static ChatListItem? _defaultInstance;

  @$pb.TagNumber(1)
  Chat get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat(Chat value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  Chat ensureChat() => $_ensure(0);

  /// Truncated last visible message text for list UI; empty/absent when unknown or no messages yet.
  @$pb.TagNumber(2)
  $core.String get lastMessagePreview => $_getSZ(1);
  @$pb.TagNumber(2)
  set lastMessagePreview($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLastMessagePreview() => $_has(1);
  @$pb.TagNumber(2)
  void clearLastMessagePreview() => $_clearField(2);

  /// Count of messages unread by the caller in this chat; 0 when caught up or when Messaging is unavailable.
  @$pb.TagNumber(3)
  $fixnum.Int64 get unreadCount => $_getI64(2);
  @$pb.TagNumber(3)
  set unreadCount($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasUnreadCount() => $_has(2);
  @$pb.TagNumber(3)
  void clearUnreadCount() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get inbox => $_getSZ(3);
  @$pb.TagNumber(4)
  set inbox($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasInbox() => $_has(3);
  @$pb.TagNumber(4)
  void clearInbox() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get isStranger => $_getBF(4);
  @$pb.TagNumber(5)
  set isStranger($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasIsStranger() => $_has(4);
  @$pb.TagNumber(5)
  void clearIsStranger() => $_clearField(5);

  /// Other participant in a DM (for list titles / avatars); absent for non-DM or when unknown.
  @$pb.TagNumber(6)
  $core.String get dmPeerProfileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set dmPeerProfileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDmPeerProfileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearDmPeerProfileId() => $_clearField(6);
}

class ChatList extends $pb.GeneratedMessage {
  factory ChatList({
    $core.Iterable<ChatListItem>? items,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (items != null) result.items.addAll(items);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  ChatList._();

  factory ChatList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChatList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..pPM<ChatListItem>(1, _omitFieldNames ? '' : 'items',
        subBuilder: ChatListItem.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatList copyWith(void Function(ChatList) updates) =>
      super.copyWith((message) => updates(message as ChatList)) as ChatList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChatList create() => ChatList._();
  @$core.override
  ChatList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChatList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ChatList>(create);
  static ChatList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ChatListItem> get items => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class GetChatRequest extends $pb.GeneratedMessage {
  factory GetChatRequest({
    $core.String? chatId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  GetChatRequest._();

  factory GetChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatRequest copyWith(void Function(GetChatRequest) updates) =>
      super.copyWith((message) => updates(message as GetChatRequest))
          as GetChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatRequest create() => GetChatRequest._();
  @$core.override
  GetChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatRequest>(create);
  static GetChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);
}

class ListFoldersRequest extends $pb.GeneratedMessage {
  factory ListFoldersRequest() => create();

  ListFoldersRequest._();

  factory ListFoldersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFoldersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFoldersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFoldersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFoldersRequest copyWith(void Function(ListFoldersRequest) updates) =>
      super.copyWith((message) => updates(message as ListFoldersRequest))
          as ListFoldersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFoldersRequest create() => ListFoldersRequest._();
  @$core.override
  ListFoldersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFoldersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFoldersRequest>(create);
  static ListFoldersRequest? _defaultInstance;
}

class FolderList extends $pb.GeneratedMessage {
  factory FolderList({
    $core.Iterable<Folder>? folders,
  }) {
    final result = create();
    if (folders != null) result.folders.addAll(folders);
    return result;
  }

  FolderList._();

  factory FolderList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FolderList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FolderList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..pPM<Folder>(1, _omitFieldNames ? '' : 'folders',
        subBuilder: Folder.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FolderList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FolderList copyWith(void Function(FolderList) updates) =>
      super.copyWith((message) => updates(message as FolderList)) as FolderList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FolderList create() => FolderList._();
  @$core.override
  FolderList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FolderList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FolderList>(create);
  static FolderList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Folder> get folders => $_getList(0);
}

class Folder extends $pb.GeneratedMessage {
  factory Folder({
    $core.String? id,
    $core.String? name,
    $core.String? folderType,
    $core.String? filterConfigJson,
    $core.int? sortOrder,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (folderType != null) result.folderType = folderType;
    if (filterConfigJson != null) result.filterConfigJson = filterConfigJson;
    if (sortOrder != null) result.sortOrder = sortOrder;
    return result;
  }

  Folder._();

  factory Folder.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Folder.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Folder',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'folderType')
    ..aOS(4, _omitFieldNames ? '' : 'filterConfigJson')
    ..aI(5, _omitFieldNames ? '' : 'sortOrder')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Folder clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Folder copyWith(void Function(Folder) updates) =>
      super.copyWith((message) => updates(message as Folder)) as Folder;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Folder create() => Folder._();
  @$core.override
  Folder createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Folder getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Folder>(create);
  static Folder? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get folderType => $_getSZ(2);
  @$pb.TagNumber(3)
  set folderType($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFolderType() => $_has(2);
  @$pb.TagNumber(3)
  void clearFolderType() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get filterConfigJson => $_getSZ(3);
  @$pb.TagNumber(4)
  set filterConfigJson($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFilterConfigJson() => $_has(3);
  @$pb.TagNumber(4)
  void clearFilterConfigJson() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.int get sortOrder => $_getIZ(4);
  @$pb.TagNumber(5)
  set sortOrder($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSortOrder() => $_has(4);
  @$pb.TagNumber(5)
  void clearSortOrder() => $_clearField(5);
}

class CreateFolderRequest extends $pb.GeneratedMessage {
  factory CreateFolderRequest({
    $core.String? name,
    $core.String? filterConfigJson,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (filterConfigJson != null) result.filterConfigJson = filterConfigJson;
    return result;
  }

  CreateFolderRequest._();

  factory CreateFolderRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateFolderRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateFolderRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'filterConfigJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateFolderRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateFolderRequest copyWith(void Function(CreateFolderRequest) updates) =>
      super.copyWith((message) => updates(message as CreateFolderRequest))
          as CreateFolderRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateFolderRequest create() => CreateFolderRequest._();
  @$core.override
  CreateFolderRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateFolderRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateFolderRequest>(create);
  static CreateFolderRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get filterConfigJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set filterConfigJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFilterConfigJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearFilterConfigJson() => $_clearField(2);
}

class UpdateFolderRequest extends $pb.GeneratedMessage {
  factory UpdateFolderRequest({
    $core.String? folderId,
    $core.String? name,
    $core.String? filterConfigJson,
    $core.int? sortOrder,
  }) {
    final result = create();
    if (folderId != null) result.folderId = folderId;
    if (name != null) result.name = name;
    if (filterConfigJson != null) result.filterConfigJson = filterConfigJson;
    if (sortOrder != null) result.sortOrder = sortOrder;
    return result;
  }

  UpdateFolderRequest._();

  factory UpdateFolderRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateFolderRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateFolderRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'folderId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'filterConfigJson')
    ..aI(4, _omitFieldNames ? '' : 'sortOrder')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateFolderRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateFolderRequest copyWith(void Function(UpdateFolderRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateFolderRequest))
          as UpdateFolderRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateFolderRequest create() => UpdateFolderRequest._();
  @$core.override
  UpdateFolderRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateFolderRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateFolderRequest>(create);
  static UpdateFolderRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get folderId => $_getSZ(0);
  @$pb.TagNumber(1)
  set folderId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFolderId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFolderId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get filterConfigJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set filterConfigJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFilterConfigJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearFilterConfigJson() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get sortOrder => $_getIZ(3);
  @$pb.TagNumber(4)
  set sortOrder($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSortOrder() => $_has(3);
  @$pb.TagNumber(4)
  void clearSortOrder() => $_clearField(4);
}

class DeleteFolderRequest extends $pb.GeneratedMessage {
  factory DeleteFolderRequest({
    $core.String? folderId,
  }) {
    final result = create();
    if (folderId != null) result.folderId = folderId;
    return result;
  }

  DeleteFolderRequest._();

  factory DeleteFolderRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteFolderRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteFolderRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'folderId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteFolderRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteFolderRequest copyWith(void Function(DeleteFolderRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteFolderRequest))
          as DeleteFolderRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteFolderRequest create() => DeleteFolderRequest._();
  @$core.override
  DeleteFolderRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteFolderRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteFolderRequest>(create);
  static DeleteFolderRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get folderId => $_getSZ(0);
  @$pb.TagNumber(1)
  set folderId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFolderId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFolderId() => $_clearField(1);
}

class AcceptDMRequestRequest extends $pb.GeneratedMessage {
  factory AcceptDMRequestRequest({
    $core.String? chatId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  AcceptDMRequestRequest._();

  factory AcceptDMRequestRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcceptDMRequestRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptDMRequestRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptDMRequestRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptDMRequestRequest copyWith(
          void Function(AcceptDMRequestRequest) updates) =>
      super.copyWith((message) => updates(message as AcceptDMRequestRequest))
          as AcceptDMRequestRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcceptDMRequestRequest create() => AcceptDMRequestRequest._();
  @$core.override
  AcceptDMRequestRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcceptDMRequestRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptDMRequestRequest>(create);
  static AcceptDMRequestRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);
}

class DeclineDMRequestRequest extends $pb.GeneratedMessage {
  factory DeclineDMRequestRequest({
    $core.String? chatId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  DeclineDMRequestRequest._();

  factory DeclineDMRequestRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeclineDMRequestRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineDMRequestRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineDMRequestRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineDMRequestRequest copyWith(
          void Function(DeclineDMRequestRequest) updates) =>
      super.copyWith((message) => updates(message as DeclineDMRequestRequest))
          as DeclineDMRequestRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeclineDMRequestRequest create() => DeclineDMRequestRequest._();
  @$core.override
  DeclineDMRequestRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeclineDMRequestRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineDMRequestRequest>(create);
  static DeclineDMRequestRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);
}

class MuteChatRequest extends $pb.GeneratedMessage {
  factory MuteChatRequest({
    $core.String? chatId,
    $1.Timestamp? mutedUntil,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (mutedUntil != null) result.mutedUntil = mutedUntil;
    return result;
  }

  MuteChatRequest._();

  factory MuteChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MuteChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MuteChatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'mutedUntil',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MuteChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MuteChatRequest copyWith(void Function(MuteChatRequest) updates) =>
      super.copyWith((message) => updates(message as MuteChatRequest))
          as MuteChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MuteChatRequest create() => MuteChatRequest._();
  @$core.override
  MuteChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MuteChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MuteChatRequest>(create);
  static MuteChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.Timestamp get mutedUntil => $_getN(1);
  @$pb.TagNumber(2)
  set mutedUntil($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasMutedUntil() => $_has(1);
  @$pb.TagNumber(2)
  void clearMutedUntil() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureMutedUntil() => $_ensure(1);
}

class ArchiveChatRequest extends $pb.GeneratedMessage {
  factory ArchiveChatRequest({
    $core.String? chatId,
    $core.bool? archived,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (archived != null) result.archived = archived;
    return result;
  }

  ArchiveChatRequest._();

  factory ArchiveChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ArchiveChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ArchiveChatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOB(2, _omitFieldNames ? '' : 'archived')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ArchiveChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ArchiveChatRequest copyWith(void Function(ArchiveChatRequest) updates) =>
      super.copyWith((message) => updates(message as ArchiveChatRequest))
          as ArchiveChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ArchiveChatRequest create() => ArchiveChatRequest._();
  @$core.override
  ArchiveChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ArchiveChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ArchiveChatRequest>(create);
  static ArchiveChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get archived => $_getBF(1);
  @$pb.TagNumber(2)
  set archived($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasArchived() => $_has(1);
  @$pb.TagNumber(2)
  void clearArchived() => $_clearField(2);
}

class EnableChatE2ERequest extends $pb.GeneratedMessage {
  factory EnableChatE2ERequest({
    $core.String? chatId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  EnableChatE2ERequest._();

  factory EnableChatE2ERequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EnableChatE2ERequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EnableChatE2ERequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnableChatE2ERequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnableChatE2ERequest copyWith(void Function(EnableChatE2ERequest) updates) =>
      super.copyWith((message) => updates(message as EnableChatE2ERequest))
          as EnableChatE2ERequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EnableChatE2ERequest create() => EnableChatE2ERequest._();
  @$core.override
  EnableChatE2ERequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EnableChatE2ERequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EnableChatE2ERequest>(create);
  static EnableChatE2ERequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);
}

class DisableChatE2ERequest extends $pb.GeneratedMessage {
  factory DisableChatE2ERequest({
    $core.String? chatId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  DisableChatE2ERequest._();

  factory DisableChatE2ERequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DisableChatE2ERequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DisableChatE2ERequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DisableChatE2ERequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DisableChatE2ERequest copyWith(
          void Function(DisableChatE2ERequest) updates) =>
      super.copyWith((message) => updates(message as DisableChatE2ERequest))
          as DisableChatE2ERequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DisableChatE2ERequest create() => DisableChatE2ERequest._();
  @$core.override
  DisableChatE2ERequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DisableChatE2ERequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DisableChatE2ERequest>(create);
  static DisableChatE2ERequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);
}

/// Cross-service pointer to chat_db.chats (docs/DATA_MODEL.md). Use in other packages instead of parallel chat_id + chat_type strings.
class ChatRef extends $pb.GeneratedMessage {
  factory ChatRef({
    $core.String? id,
    ChatType? type,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (type != null) result.type = type;
    return result;
  }

  ChatRef._();

  factory ChatRef.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChatRef.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatRef',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<ChatType>(2, _omitFieldNames ? '' : 'type',
        enumValues: ChatType.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatRef clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatRef copyWith(void Function(ChatRef) updates) =>
      super.copyWith((message) => updates(message as ChatRef)) as ChatRef;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChatRef create() => ChatRef._();
  @$core.override
  ChatRef createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChatRef getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ChatRef>(create);
  static ChatRef? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  ChatType get type => $_getN(1);
  @$pb.TagNumber(2)
  set type(ChatType value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);
}

class CreateDMResponse extends $pb.GeneratedMessage {
  factory CreateDMResponse({
    Chat? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  CreateDMResponse._();

  factory CreateDMResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateDMResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateDMResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<Chat>(1, _omitFieldNames ? '' : 'chat', subBuilder: Chat.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateDMResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateDMResponse copyWith(void Function(CreateDMResponse) updates) =>
      super.copyWith((message) => updates(message as CreateDMResponse))
          as CreateDMResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateDMResponse create() => CreateDMResponse._();
  @$core.override
  CreateDMResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateDMResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateDMResponse>(create);
  static CreateDMResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Chat get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat(Chat value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  Chat ensureChat() => $_ensure(0);
}

class GetDMResponse extends $pb.GeneratedMessage {
  factory GetDMResponse({
    Chat? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  GetDMResponse._();

  factory GetDMResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetDMResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetDMResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<Chat>(1, _omitFieldNames ? '' : 'chat', subBuilder: Chat.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDMResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDMResponse copyWith(void Function(GetDMResponse) updates) =>
      super.copyWith((message) => updates(message as GetDMResponse))
          as GetDMResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetDMResponse create() => GetDMResponse._();
  @$core.override
  GetDMResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetDMResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetDMResponse>(create);
  static GetDMResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Chat get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat(Chat value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  Chat ensureChat() => $_ensure(0);
}

class CreateChatResponse extends $pb.GeneratedMessage {
  factory CreateChatResponse({
    Chat? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  CreateChatResponse._();

  factory CreateChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateChatResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<Chat>(1, _omitFieldNames ? '' : 'chat', subBuilder: Chat.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateChatResponse copyWith(void Function(CreateChatResponse) updates) =>
      super.copyWith((message) => updates(message as CreateChatResponse))
          as CreateChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateChatResponse create() => CreateChatResponse._();
  @$core.override
  CreateChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateChatResponse>(create);
  static CreateChatResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Chat get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat(Chat value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  Chat ensureChat() => $_ensure(0);
}

class UpdateChatResponse extends $pb.GeneratedMessage {
  factory UpdateChatResponse({
    Chat? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  UpdateChatResponse._();

  factory UpdateChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateChatResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<Chat>(1, _omitFieldNames ? '' : 'chat', subBuilder: Chat.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateChatResponse copyWith(void Function(UpdateChatResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateChatResponse))
          as UpdateChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateChatResponse create() => UpdateChatResponse._();
  @$core.override
  UpdateChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateChatResponse>(create);
  static UpdateChatResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Chat get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat(Chat value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  Chat ensureChat() => $_ensure(0);
}

class DeleteChatResponse extends $pb.GeneratedMessage {
  factory DeleteChatResponse() => create();

  DeleteChatResponse._();

  factory DeleteChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteChatResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteChatResponse copyWith(void Function(DeleteChatResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteChatResponse))
          as DeleteChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteChatResponse create() => DeleteChatResponse._();
  @$core.override
  DeleteChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteChatResponse>(create);
  static DeleteChatResponse? _defaultInstance;
}

class AddMembersResponse extends $pb.GeneratedMessage {
  factory AddMembersResponse() => create();

  AddMembersResponse._();

  factory AddMembersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddMembersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddMembersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddMembersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddMembersResponse copyWith(void Function(AddMembersResponse) updates) =>
      super.copyWith((message) => updates(message as AddMembersResponse))
          as AddMembersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddMembersResponse create() => AddMembersResponse._();
  @$core.override
  AddMembersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddMembersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddMembersResponse>(create);
  static AddMembersResponse? _defaultInstance;
}

class RemoveMemberResponse extends $pb.GeneratedMessage {
  factory RemoveMemberResponse() => create();

  RemoveMemberResponse._();

  factory RemoveMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveMemberResponse copyWith(void Function(RemoveMemberResponse) updates) =>
      super.copyWith((message) => updates(message as RemoveMemberResponse))
          as RemoveMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveMemberResponse create() => RemoveMemberResponse._();
  @$core.override
  RemoveMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveMemberResponse>(create);
  static RemoveMemberResponse? _defaultInstance;
}

class LeaveChatResponse extends $pb.GeneratedMessage {
  factory LeaveChatResponse() => create();

  LeaveChatResponse._();

  factory LeaveChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveChatResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveChatResponse copyWith(void Function(LeaveChatResponse) updates) =>
      super.copyWith((message) => updates(message as LeaveChatResponse))
          as LeaveChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveChatResponse create() => LeaveChatResponse._();
  @$core.override
  LeaveChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveChatResponse>(create);
  static LeaveChatResponse? _defaultInstance;
}

class ListMembersResponse extends $pb.GeneratedMessage {
  factory ListMembersResponse({
    MemberList? memberList,
  }) {
    final result = create();
    if (memberList != null) result.memberList = memberList;
    return result;
  }

  ListMembersResponse._();

  factory ListMembersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMembersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMembersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<MemberList>(1, _omitFieldNames ? '' : 'memberList',
        subBuilder: MemberList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersResponse copyWith(void Function(ListMembersResponse) updates) =>
      super.copyWith((message) => updates(message as ListMembersResponse))
          as ListMembersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMembersResponse create() => ListMembersResponse._();
  @$core.override
  ListMembersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMembersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMembersResponse>(create);
  static ListMembersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MemberList get memberList => $_getN(0);
  @$pb.TagNumber(1)
  set memberList(MemberList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMemberList() => $_has(0);
  @$pb.TagNumber(1)
  void clearMemberList() => $_clearField(1);
  @$pb.TagNumber(1)
  MemberList ensureMemberList() => $_ensure(0);
}

class ListChatsResponse extends $pb.GeneratedMessage {
  factory ListChatsResponse({
    ChatList? chatList,
  }) {
    final result = create();
    if (chatList != null) result.chatList = chatList;
    return result;
  }

  ListChatsResponse._();

  factory ListChatsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListChatsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListChatsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<ChatList>(1, _omitFieldNames ? '' : 'chatList',
        subBuilder: ChatList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListChatsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListChatsResponse copyWith(void Function(ListChatsResponse) updates) =>
      super.copyWith((message) => updates(message as ListChatsResponse))
          as ListChatsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListChatsResponse create() => ListChatsResponse._();
  @$core.override
  ListChatsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListChatsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListChatsResponse>(create);
  static ListChatsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ChatList get chatList => $_getN(0);
  @$pb.TagNumber(1)
  set chatList(ChatList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChatList() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatList() => $_clearField(1);
  @$pb.TagNumber(1)
  ChatList ensureChatList() => $_ensure(0);
}

class GetChatResponse extends $pb.GeneratedMessage {
  factory GetChatResponse({
    Chat? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  GetChatResponse._();

  factory GetChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<Chat>(1, _omitFieldNames ? '' : 'chat', subBuilder: Chat.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatResponse copyWith(void Function(GetChatResponse) updates) =>
      super.copyWith((message) => updates(message as GetChatResponse))
          as GetChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatResponse create() => GetChatResponse._();
  @$core.override
  GetChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatResponse>(create);
  static GetChatResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Chat get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat(Chat value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  Chat ensureChat() => $_ensure(0);
}

class ListFoldersResponse extends $pb.GeneratedMessage {
  factory ListFoldersResponse({
    FolderList? folderList,
  }) {
    final result = create();
    if (folderList != null) result.folderList = folderList;
    return result;
  }

  ListFoldersResponse._();

  factory ListFoldersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFoldersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFoldersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<FolderList>(1, _omitFieldNames ? '' : 'folderList',
        subBuilder: FolderList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFoldersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFoldersResponse copyWith(void Function(ListFoldersResponse) updates) =>
      super.copyWith((message) => updates(message as ListFoldersResponse))
          as ListFoldersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFoldersResponse create() => ListFoldersResponse._();
  @$core.override
  ListFoldersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFoldersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFoldersResponse>(create);
  static ListFoldersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FolderList get folderList => $_getN(0);
  @$pb.TagNumber(1)
  set folderList(FolderList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFolderList() => $_has(0);
  @$pb.TagNumber(1)
  void clearFolderList() => $_clearField(1);
  @$pb.TagNumber(1)
  FolderList ensureFolderList() => $_ensure(0);
}

class CreateFolderResponse extends $pb.GeneratedMessage {
  factory CreateFolderResponse({
    Folder? folder,
  }) {
    final result = create();
    if (folder != null) result.folder = folder;
    return result;
  }

  CreateFolderResponse._();

  factory CreateFolderResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateFolderResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateFolderResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<Folder>(1, _omitFieldNames ? '' : 'folder', subBuilder: Folder.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateFolderResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateFolderResponse copyWith(void Function(CreateFolderResponse) updates) =>
      super.copyWith((message) => updates(message as CreateFolderResponse))
          as CreateFolderResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateFolderResponse create() => CreateFolderResponse._();
  @$core.override
  CreateFolderResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateFolderResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateFolderResponse>(create);
  static CreateFolderResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Folder get folder => $_getN(0);
  @$pb.TagNumber(1)
  set folder(Folder value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFolder() => $_has(0);
  @$pb.TagNumber(1)
  void clearFolder() => $_clearField(1);
  @$pb.TagNumber(1)
  Folder ensureFolder() => $_ensure(0);
}

class UpdateFolderResponse extends $pb.GeneratedMessage {
  factory UpdateFolderResponse({
    Folder? folder,
  }) {
    final result = create();
    if (folder != null) result.folder = folder;
    return result;
  }

  UpdateFolderResponse._();

  factory UpdateFolderResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateFolderResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateFolderResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOM<Folder>(1, _omitFieldNames ? '' : 'folder', subBuilder: Folder.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateFolderResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateFolderResponse copyWith(void Function(UpdateFolderResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateFolderResponse))
          as UpdateFolderResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateFolderResponse create() => UpdateFolderResponse._();
  @$core.override
  UpdateFolderResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateFolderResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateFolderResponse>(create);
  static UpdateFolderResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Folder get folder => $_getN(0);
  @$pb.TagNumber(1)
  set folder(Folder value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFolder() => $_has(0);
  @$pb.TagNumber(1)
  void clearFolder() => $_clearField(1);
  @$pb.TagNumber(1)
  Folder ensureFolder() => $_ensure(0);
}

class DeleteFolderResponse extends $pb.GeneratedMessage {
  factory DeleteFolderResponse() => create();

  DeleteFolderResponse._();

  factory DeleteFolderResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteFolderResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteFolderResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteFolderResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteFolderResponse copyWith(void Function(DeleteFolderResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteFolderResponse))
          as DeleteFolderResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteFolderResponse create() => DeleteFolderResponse._();
  @$core.override
  DeleteFolderResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteFolderResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteFolderResponse>(create);
  static DeleteFolderResponse? _defaultInstance;
}

class AcceptDMRequestResponse extends $pb.GeneratedMessage {
  factory AcceptDMRequestResponse() => create();

  AcceptDMRequestResponse._();

  factory AcceptDMRequestResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcceptDMRequestResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptDMRequestResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptDMRequestResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptDMRequestResponse copyWith(
          void Function(AcceptDMRequestResponse) updates) =>
      super.copyWith((message) => updates(message as AcceptDMRequestResponse))
          as AcceptDMRequestResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcceptDMRequestResponse create() => AcceptDMRequestResponse._();
  @$core.override
  AcceptDMRequestResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcceptDMRequestResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptDMRequestResponse>(create);
  static AcceptDMRequestResponse? _defaultInstance;
}

class DeclineDMRequestResponse extends $pb.GeneratedMessage {
  factory DeclineDMRequestResponse() => create();

  DeclineDMRequestResponse._();

  factory DeclineDMRequestResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeclineDMRequestResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineDMRequestResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineDMRequestResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineDMRequestResponse copyWith(
          void Function(DeclineDMRequestResponse) updates) =>
      super.copyWith((message) => updates(message as DeclineDMRequestResponse))
          as DeclineDMRequestResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeclineDMRequestResponse create() => DeclineDMRequestResponse._();
  @$core.override
  DeclineDMRequestResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeclineDMRequestResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineDMRequestResponse>(create);
  static DeclineDMRequestResponse? _defaultInstance;
}

class MuteChatResponse extends $pb.GeneratedMessage {
  factory MuteChatResponse() => create();

  MuteChatResponse._();

  factory MuteChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MuteChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MuteChatResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MuteChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MuteChatResponse copyWith(void Function(MuteChatResponse) updates) =>
      super.copyWith((message) => updates(message as MuteChatResponse))
          as MuteChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MuteChatResponse create() => MuteChatResponse._();
  @$core.override
  MuteChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MuteChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MuteChatResponse>(create);
  static MuteChatResponse? _defaultInstance;
}

class ArchiveChatResponse extends $pb.GeneratedMessage {
  factory ArchiveChatResponse() => create();

  ArchiveChatResponse._();

  factory ArchiveChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ArchiveChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ArchiveChatResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ArchiveChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ArchiveChatResponse copyWith(void Function(ArchiveChatResponse) updates) =>
      super.copyWith((message) => updates(message as ArchiveChatResponse))
          as ArchiveChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ArchiveChatResponse create() => ArchiveChatResponse._();
  @$core.override
  ArchiveChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ArchiveChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ArchiveChatResponse>(create);
  static ArchiveChatResponse? _defaultInstance;
}

class EnableChatE2EResponse extends $pb.GeneratedMessage {
  factory EnableChatE2EResponse() => create();

  EnableChatE2EResponse._();

  factory EnableChatE2EResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EnableChatE2EResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EnableChatE2EResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnableChatE2EResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnableChatE2EResponse copyWith(
          void Function(EnableChatE2EResponse) updates) =>
      super.copyWith((message) => updates(message as EnableChatE2EResponse))
          as EnableChatE2EResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EnableChatE2EResponse create() => EnableChatE2EResponse._();
  @$core.override
  EnableChatE2EResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EnableChatE2EResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EnableChatE2EResponse>(create);
  static EnableChatE2EResponse? _defaultInstance;
}

class DisableChatE2EResponse extends $pb.GeneratedMessage {
  factory DisableChatE2EResponse() => create();

  DisableChatE2EResponse._();

  factory DisableChatE2EResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DisableChatE2EResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DisableChatE2EResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DisableChatE2EResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DisableChatE2EResponse copyWith(
          void Function(DisableChatE2EResponse) updates) =>
      super.copyWith((message) => updates(message as DisableChatE2EResponse))
          as DisableChatE2EResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DisableChatE2EResponse create() => DisableChatE2EResponse._();
  @$core.override
  DisableChatE2EResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DisableChatE2EResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DisableChatE2EResponse>(create);
  static DisableChatE2EResponse? _defaultInstance;
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
