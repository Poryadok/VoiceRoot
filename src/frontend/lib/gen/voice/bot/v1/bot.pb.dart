// This is a generated file - do not edit.
//
// Generated from voice/bot/v1/bot.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../chat/v1/chat.pb.dart' as $2;
import '../../messaging/v1/messaging.pb.dart' as $3;
import 'bot.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'bot.pbenum.dart';

class Bot extends $pb.GeneratedMessage {
  factory Bot({
    $core.String? id,
    $core.String? ownerAccountId,
    $core.String? name,
    $core.String? description,
    $core.String? avatarUrl,
    $core.String? webhookUrl,
    $core.bool? isPollingMode,
    $core.String? scopesJson,
    $core.String? status,
    $1.Timestamp? createdAt,
    BotLifecycleStatus? statusEnum,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (ownerAccountId != null) result.ownerAccountId = ownerAccountId;
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (avatarUrl != null) result.avatarUrl = avatarUrl;
    if (webhookUrl != null) result.webhookUrl = webhookUrl;
    if (isPollingMode != null) result.isPollingMode = isPollingMode;
    if (scopesJson != null) result.scopesJson = scopesJson;
    if (status != null) result.status = status;
    if (createdAt != null) result.createdAt = createdAt;
    if (statusEnum != null) result.statusEnum = statusEnum;
    return result;
  }

  Bot._();

  factory Bot.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Bot.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Bot',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'ownerAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..aOS(5, _omitFieldNames ? '' : 'avatarUrl')
    ..aOS(6, _omitFieldNames ? '' : 'webhookUrl')
    ..aOB(7, _omitFieldNames ? '' : 'isPollingMode')
    ..aOS(8, _omitFieldNames ? '' : 'scopesJson')
    ..aOS(9, _omitFieldNames ? '' : 'status')
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..aE<BotLifecycleStatus>(11, _omitFieldNames ? '' : 'statusEnum',
        enumValues: BotLifecycleStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Bot clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Bot copyWith(void Function(Bot) updates) =>
      super.copyWith((message) => updates(message as Bot)) as Bot;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Bot create() => Bot._();
  @$core.override
  Bot createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Bot getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Bot>(create);
  static Bot? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get ownerAccountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set ownerAccountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOwnerAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwnerAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get avatarUrl => $_getSZ(4);
  @$pb.TagNumber(5)
  set avatarUrl($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAvatarUrl() => $_has(4);
  @$pb.TagNumber(5)
  void clearAvatarUrl() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get webhookUrl => $_getSZ(5);
  @$pb.TagNumber(6)
  set webhookUrl($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasWebhookUrl() => $_has(5);
  @$pb.TagNumber(6)
  void clearWebhookUrl() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get isPollingMode => $_getBF(6);
  @$pb.TagNumber(7)
  set isPollingMode($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasIsPollingMode() => $_has(6);
  @$pb.TagNumber(7)
  void clearIsPollingMode() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get scopesJson => $_getSZ(7);
  @$pb.TagNumber(8)
  set scopesJson($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasScopesJson() => $_has(7);
  @$pb.TagNumber(8)
  void clearScopesJson() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get status => $_getSZ(8);
  @$pb.TagNumber(9)
  set status($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasStatus() => $_has(8);
  @$pb.TagNumber(9)
  void clearStatus() => $_clearField(9);

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
  BotLifecycleStatus get statusEnum => $_getN(10);
  @$pb.TagNumber(11)
  set statusEnum(BotLifecycleStatus value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasStatusEnum() => $_has(10);
  @$pb.TagNumber(11)
  void clearStatusEnum() => $_clearField(11);
}

class RegisterBotRequest extends $pb.GeneratedMessage {
  factory RegisterBotRequest({
    $core.String? name,
    $core.String? description,
    $core.String? scopesJson,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (scopesJson != null) result.scopesJson = scopesJson;
    return result;
  }

  RegisterBotRequest._();

  factory RegisterBotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterBotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterBotRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOS(3, _omitFieldNames ? '' : 'scopesJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterBotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterBotRequest copyWith(void Function(RegisterBotRequest) updates) =>
      super.copyWith((message) => updates(message as RegisterBotRequest))
          as RegisterBotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterBotRequest create() => RegisterBotRequest._();
  @$core.override
  RegisterBotRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterBotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterBotRequest>(create);
  static RegisterBotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get scopesJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set scopesJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasScopesJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearScopesJson() => $_clearField(3);
}

class UpdateBotRequest extends $pb.GeneratedMessage {
  factory UpdateBotRequest({
    $core.String? botId,
    $core.String? name,
    $core.String? description,
    $core.String? avatarUrl,
    $core.String? scopesJson,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (avatarUrl != null) result.avatarUrl = avatarUrl;
    if (scopesJson != null) result.scopesJson = scopesJson;
    return result;
  }

  UpdateBotRequest._();

  factory UpdateBotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateBotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateBotRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOS(4, _omitFieldNames ? '' : 'avatarUrl')
    ..aOS(5, _omitFieldNames ? '' : 'scopesJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateBotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateBotRequest copyWith(void Function(UpdateBotRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateBotRequest))
          as UpdateBotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateBotRequest create() => UpdateBotRequest._();
  @$core.override
  UpdateBotRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateBotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateBotRequest>(create);
  static UpdateBotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get avatarUrl => $_getSZ(3);
  @$pb.TagNumber(4)
  set avatarUrl($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAvatarUrl() => $_has(3);
  @$pb.TagNumber(4)
  void clearAvatarUrl() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get scopesJson => $_getSZ(4);
  @$pb.TagNumber(5)
  set scopesJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasScopesJson() => $_has(4);
  @$pb.TagNumber(5)
  void clearScopesJson() => $_clearField(5);
}

class DeleteBotRequest extends $pb.GeneratedMessage {
  factory DeleteBotRequest({
    $core.String? botId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    return result;
  }

  DeleteBotRequest._();

  factory DeleteBotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteBotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteBotRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteBotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteBotRequest copyWith(void Function(DeleteBotRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteBotRequest))
          as DeleteBotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteBotRequest create() => DeleteBotRequest._();
  @$core.override
  DeleteBotRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteBotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteBotRequest>(create);
  static DeleteBotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);
}

class GetBotRequest extends $pb.GeneratedMessage {
  factory GetBotRequest({
    $core.String? botId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    return result;
  }

  GetBotRequest._();

  factory GetBotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBotRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBotRequest copyWith(void Function(GetBotRequest) updates) =>
      super.copyWith((message) => updates(message as GetBotRequest))
          as GetBotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBotRequest create() => GetBotRequest._();
  @$core.override
  GetBotRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBotRequest>(create);
  static GetBotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);
}

class ListBotsRequest extends $pb.GeneratedMessage {
  factory ListBotsRequest() => create();

  ListBotsRequest._();

  factory ListBotsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListBotsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListBotsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBotsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBotsRequest copyWith(void Function(ListBotsRequest) updates) =>
      super.copyWith((message) => updates(message as ListBotsRequest))
          as ListBotsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListBotsRequest create() => ListBotsRequest._();
  @$core.override
  ListBotsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListBotsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListBotsRequest>(create);
  static ListBotsRequest? _defaultInstance;
}

class BotList extends $pb.GeneratedMessage {
  factory BotList({
    $core.Iterable<Bot>? bots,
  }) {
    final result = create();
    if (bots != null) result.bots.addAll(bots);
    return result;
  }

  BotList._();

  factory BotList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BotList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BotList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..pPM<Bot>(1, _omitFieldNames ? '' : 'bots', subBuilder: Bot.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotList copyWith(void Function(BotList) updates) =>
      super.copyWith((message) => updates(message as BotList)) as BotList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BotList create() => BotList._();
  @$core.override
  BotList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BotList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<BotList>(create);
  static BotList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Bot> get bots => $_getList(0);
}

class RegenerateTokenRequest extends $pb.GeneratedMessage {
  factory RegenerateTokenRequest({
    $core.String? botId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    return result;
  }

  RegenerateTokenRequest._();

  factory RegenerateTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegenerateTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegenerateTokenRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegenerateTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegenerateTokenRequest copyWith(
          void Function(RegenerateTokenRequest) updates) =>
      super.copyWith((message) => updates(message as RegenerateTokenRequest))
          as RegenerateTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegenerateTokenRequest create() => RegenerateTokenRequest._();
  @$core.override
  RegenerateTokenRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegenerateTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegenerateTokenRequest>(create);
  static RegenerateTokenRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);
}

class TokenResponse extends $pb.GeneratedMessage {
  factory TokenResponse({
    $core.String? token,
  }) {
    final result = create();
    if (token != null) result.token = token;
    return result;
  }

  TokenResponse._();

  factory TokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TokenResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'token')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TokenResponse copyWith(void Function(TokenResponse) updates) =>
      super.copyWith((message) => updates(message as TokenResponse))
          as TokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TokenResponse create() => TokenResponse._();
  @$core.override
  TokenResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TokenResponse>(create);
  static TokenResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get token => $_getSZ(0);
  @$pb.TagNumber(1)
  set token($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearToken() => $_clearField(1);
}

class RegisterCommandsRequest extends $pb.GeneratedMessage {
  factory RegisterCommandsRequest({
    $core.String? botId,
    $core.String? commandsJson,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (commandsJson != null) result.commandsJson = commandsJson;
    return result;
  }

  RegisterCommandsRequest._();

  factory RegisterCommandsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterCommandsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterCommandsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'commandsJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterCommandsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterCommandsRequest copyWith(
          void Function(RegisterCommandsRequest) updates) =>
      super.copyWith((message) => updates(message as RegisterCommandsRequest))
          as RegisterCommandsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterCommandsRequest create() => RegisterCommandsRequest._();
  @$core.override
  RegisterCommandsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterCommandsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterCommandsRequest>(create);
  static RegisterCommandsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get commandsJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set commandsJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCommandsJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearCommandsJson() => $_clearField(2);
}

class GetCommandsRequest extends $pb.GeneratedMessage {
  factory GetCommandsRequest({
    $core.String? botId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    return result;
  }

  GetCommandsRequest._();

  factory GetCommandsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetCommandsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCommandsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCommandsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCommandsRequest copyWith(void Function(GetCommandsRequest) updates) =>
      super.copyWith((message) => updates(message as GetCommandsRequest))
          as GetCommandsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetCommandsRequest create() => GetCommandsRequest._();
  @$core.override
  GetCommandsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetCommandsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCommandsRequest>(create);
  static GetCommandsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);
}

class CommandList extends $pb.GeneratedMessage {
  factory CommandList({
    $core.String? commandsJson,
  }) {
    final result = create();
    if (commandsJson != null) result.commandsJson = commandsJson;
    return result;
  }

  CommandList._();

  factory CommandList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CommandList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CommandList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'commandsJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CommandList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CommandList copyWith(void Function(CommandList) updates) =>
      super.copyWith((message) => updates(message as CommandList))
          as CommandList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CommandList create() => CommandList._();
  @$core.override
  CommandList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CommandList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CommandList>(create);
  static CommandList? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get commandsJson => $_getSZ(0);
  @$pb.TagNumber(1)
  set commandsJson($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCommandsJson() => $_has(0);
  @$pb.TagNumber(1)
  void clearCommandsJson() => $_clearField(1);
}

class SetWebhookURLRequest extends $pb.GeneratedMessage {
  factory SetWebhookURLRequest({
    $core.String? botId,
    $core.String? url,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (url != null) result.url = url;
    return result;
  }

  SetWebhookURLRequest._();

  factory SetWebhookURLRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetWebhookURLRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetWebhookURLRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'url')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetWebhookURLRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetWebhookURLRequest copyWith(void Function(SetWebhookURLRequest) updates) =>
      super.copyWith((message) => updates(message as SetWebhookURLRequest))
          as SetWebhookURLRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetWebhookURLRequest create() => SetWebhookURLRequest._();
  @$core.override
  SetWebhookURLRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetWebhookURLRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetWebhookURLRequest>(create);
  static SetWebhookURLRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get url => $_getSZ(1);
  @$pb.TagNumber(2)
  set url($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUrl() => $_has(1);
  @$pb.TagNumber(2)
  void clearUrl() => $_clearField(2);
}

class GetWebhookURLRequest extends $pb.GeneratedMessage {
  factory GetWebhookURLRequest({
    $core.String? botId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    return result;
  }

  GetWebhookURLRequest._();

  factory GetWebhookURLRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetWebhookURLRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetWebhookURLRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetWebhookURLRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetWebhookURLRequest copyWith(void Function(GetWebhookURLRequest) updates) =>
      super.copyWith((message) => updates(message as GetWebhookURLRequest))
          as GetWebhookURLRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetWebhookURLRequest create() => GetWebhookURLRequest._();
  @$core.override
  GetWebhookURLRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetWebhookURLRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetWebhookURLRequest>(create);
  static GetWebhookURLRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);
}

class SetChatWhitelistRequest extends $pb.GeneratedMessage {
  factory SetChatWhitelistRequest({
    $core.String? botId,
    $core.Iterable<$2.ChatRef>? allowedChats,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (allowedChats != null) result.allowedChats.addAll(allowedChats);
    return result;
  }

  SetChatWhitelistRequest._();

  factory SetChatWhitelistRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetChatWhitelistRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetChatWhitelistRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..pPM<$2.ChatRef>(2, _omitFieldNames ? '' : 'allowedChats',
        subBuilder: $2.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetChatWhitelistRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetChatWhitelistRequest copyWith(
          void Function(SetChatWhitelistRequest) updates) =>
      super.copyWith((message) => updates(message as SetChatWhitelistRequest))
          as SetChatWhitelistRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetChatWhitelistRequest create() => SetChatWhitelistRequest._();
  @$core.override
  SetChatWhitelistRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetChatWhitelistRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetChatWhitelistRequest>(create);
  static SetChatWhitelistRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$2.ChatRef> get allowedChats => $_getList(1);
}

class GetChatWhitelistRequest extends $pb.GeneratedMessage {
  factory GetChatWhitelistRequest({
    $core.String? botId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    return result;
  }

  GetChatWhitelistRequest._();

  factory GetChatWhitelistRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatWhitelistRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatWhitelistRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatWhitelistRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatWhitelistRequest copyWith(
          void Function(GetChatWhitelistRequest) updates) =>
      super.copyWith((message) => updates(message as GetChatWhitelistRequest))
          as GetChatWhitelistRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatWhitelistRequest create() => GetChatWhitelistRequest._();
  @$core.override
  GetChatWhitelistRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatWhitelistRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatWhitelistRequest>(create);
  static GetChatWhitelistRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);
}

class SendBotMessageRequest extends $pb.GeneratedMessage {
  factory SendBotMessageRequest({
    $core.String? botId,
    $2.ChatRef? chat,
    $core.String? content,
    $core.String? threadParentId,
    $core.String? interactionToken,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (chat != null) result.chat = chat;
    if (content != null) result.content = content;
    if (threadParentId != null) result.threadParentId = threadParentId;
    if (interactionToken != null) result.interactionToken = interactionToken;
    return result;
  }

  SendBotMessageRequest._();

  factory SendBotMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendBotMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendBotMessageRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOM<$2.ChatRef>(2, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'content')
    ..aOS(4, _omitFieldNames ? '' : 'threadParentId')
    ..aOS(5, _omitFieldNames ? '' : 'interactionToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendBotMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendBotMessageRequest copyWith(
          void Function(SendBotMessageRequest) updates) =>
      super.copyWith((message) => updates(message as SendBotMessageRequest))
          as SendBotMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendBotMessageRequest create() => SendBotMessageRequest._();
  @$core.override
  SendBotMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendBotMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendBotMessageRequest>(create);
  static SendBotMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.ChatRef get chat => $_getN(1);
  @$pb.TagNumber(2)
  set chat($2.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.ChatRef ensureChat() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get content => $_getSZ(2);
  @$pb.TagNumber(3)
  set content($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasContent() => $_has(2);
  @$pb.TagNumber(3)
  void clearContent() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get threadParentId => $_getSZ(3);
  @$pb.TagNumber(4)
  set threadParentId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasThreadParentId() => $_has(3);
  @$pb.TagNumber(4)
  void clearThreadParentId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get interactionToken => $_getSZ(4);
  @$pb.TagNumber(5)
  set interactionToken($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasInteractionToken() => $_has(4);
  @$pb.TagNumber(5)
  void clearInteractionToken() => $_clearField(5);
}

class EditBotMessageRequest extends $pb.GeneratedMessage {
  factory EditBotMessageRequest({
    $core.String? botId,
    $core.String? messageId,
    $core.String? content,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (messageId != null) result.messageId = messageId;
    if (content != null) result.content = content;
    return result;
  }

  EditBotMessageRequest._();

  factory EditBotMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EditBotMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EditBotMessageRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'messageId')
    ..aOS(3, _omitFieldNames ? '' : 'content')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EditBotMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EditBotMessageRequest copyWith(
          void Function(EditBotMessageRequest) updates) =>
      super.copyWith((message) => updates(message as EditBotMessageRequest))
          as EditBotMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EditBotMessageRequest create() => EditBotMessageRequest._();
  @$core.override
  EditBotMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EditBotMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EditBotMessageRequest>(create);
  static EditBotMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get messageId => $_getSZ(1);
  @$pb.TagNumber(2)
  set messageId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessageId() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessageId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get content => $_getSZ(2);
  @$pb.TagNumber(3)
  set content($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasContent() => $_has(2);
  @$pb.TagNumber(3)
  void clearContent() => $_clearField(3);
}

class SendEphemeralRequest extends $pb.GeneratedMessage {
  factory SendEphemeralRequest({
    $core.String? botId,
    $2.ChatRef? chat,
    $core.String? targetProfileId,
    $core.String? content,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (chat != null) result.chat = chat;
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    if (content != null) result.content = content;
    return result;
  }

  SendEphemeralRequest._();

  factory SendEphemeralRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendEphemeralRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendEphemeralRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOM<$2.ChatRef>(2, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'targetProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'content')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendEphemeralRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendEphemeralRequest copyWith(void Function(SendEphemeralRequest) updates) =>
      super.copyWith((message) => updates(message as SendEphemeralRequest))
          as SendEphemeralRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendEphemeralRequest create() => SendEphemeralRequest._();
  @$core.override
  SendEphemeralRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendEphemeralRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendEphemeralRequest>(create);
  static SendEphemeralRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.ChatRef get chat => $_getN(1);
  @$pb.TagNumber(2)
  set chat($2.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.ChatRef ensureChat() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get targetProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set targetProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTargetProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearTargetProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get content => $_getSZ(3);
  @$pb.TagNumber(4)
  set content($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasContent() => $_has(3);
  @$pb.TagNumber(4)
  void clearContent() => $_clearField(4);
}

class DeferResponseRequest extends $pb.GeneratedMessage {
  factory DeferResponseRequest({
    $core.String? botId,
    $core.String? interactionToken,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (interactionToken != null) result.interactionToken = interactionToken;
    return result;
  }

  DeferResponseRequest._();

  factory DeferResponseRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeferResponseRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeferResponseRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'interactionToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeferResponseRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeferResponseRequest copyWith(void Function(DeferResponseRequest) updates) =>
      super.copyWith((message) => updates(message as DeferResponseRequest))
          as DeferResponseRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeferResponseRequest create() => DeferResponseRequest._();
  @$core.override
  DeferResponseRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeferResponseRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeferResponseRequest>(create);
  static DeferResponseRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get interactionToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set interactionToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInteractionToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearInteractionToken() => $_clearField(2);
}

class PollEventsRequest extends $pb.GeneratedMessage {
  factory PollEventsRequest({
    $core.String? botId,
    $core.String? cursor,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (cursor != null) result.cursor = cursor;
    return result;
  }

  PollEventsRequest._();

  factory PollEventsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PollEventsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PollEventsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'cursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PollEventsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PollEventsRequest copyWith(void Function(PollEventsRequest) updates) =>
      super.copyWith((message) => updates(message as PollEventsRequest))
          as PollEventsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PollEventsRequest create() => PollEventsRequest._();
  @$core.override
  PollEventsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PollEventsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PollEventsRequest>(create);
  static PollEventsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get cursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set cursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearCursor() => $_clearField(2);
}

class BotEvent extends $pb.GeneratedMessage {
  factory BotEvent({
    $core.String? eventId,
    $core.String? eventType,
    $core.String? payloadJson,
    $1.Timestamp? createdAt,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (eventType != null) result.eventType = eventType;
    if (payloadJson != null) result.payloadJson = payloadJson;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  BotEvent._();

  factory BotEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BotEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BotEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOS(2, _omitFieldNames ? '' : 'eventType')
    ..aOS(3, _omitFieldNames ? '' : 'payloadJson')
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotEvent copyWith(void Function(BotEvent) updates) =>
      super.copyWith((message) => updates(message as BotEvent)) as BotEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BotEvent create() => BotEvent._();
  @$core.override
  BotEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BotEvent getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<BotEvent>(create);
  static BotEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get eventId => $_getSZ(0);
  @$pb.TagNumber(1)
  set eventId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEventId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEventId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get eventType => $_getSZ(1);
  @$pb.TagNumber(2)
  set eventType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEventType() => $_has(1);
  @$pb.TagNumber(2)
  void clearEventType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get payloadJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set payloadJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPayloadJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearPayloadJson() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Timestamp get createdAt => $_getN(3);
  @$pb.TagNumber(4)
  set createdAt($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCreatedAt() => $_has(3);
  @$pb.TagNumber(4)
  void clearCreatedAt() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureCreatedAt() => $_ensure(3);
}

class RegisterBotResponse extends $pb.GeneratedMessage {
  factory RegisterBotResponse({
    Bot? bot,
  }) {
    final result = create();
    if (bot != null) result.bot = bot;
    return result;
  }

  RegisterBotResponse._();

  factory RegisterBotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterBotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterBotResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<Bot>(1, _omitFieldNames ? '' : 'bot', subBuilder: Bot.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterBotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterBotResponse copyWith(void Function(RegisterBotResponse) updates) =>
      super.copyWith((message) => updates(message as RegisterBotResponse))
          as RegisterBotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterBotResponse create() => RegisterBotResponse._();
  @$core.override
  RegisterBotResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterBotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterBotResponse>(create);
  static RegisterBotResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Bot get bot => $_getN(0);
  @$pb.TagNumber(1)
  set bot(Bot value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBot() => $_has(0);
  @$pb.TagNumber(1)
  void clearBot() => $_clearField(1);
  @$pb.TagNumber(1)
  Bot ensureBot() => $_ensure(0);
}

class UpdateBotResponse extends $pb.GeneratedMessage {
  factory UpdateBotResponse({
    Bot? bot,
  }) {
    final result = create();
    if (bot != null) result.bot = bot;
    return result;
  }

  UpdateBotResponse._();

  factory UpdateBotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateBotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateBotResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<Bot>(1, _omitFieldNames ? '' : 'bot', subBuilder: Bot.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateBotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateBotResponse copyWith(void Function(UpdateBotResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateBotResponse))
          as UpdateBotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateBotResponse create() => UpdateBotResponse._();
  @$core.override
  UpdateBotResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateBotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateBotResponse>(create);
  static UpdateBotResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Bot get bot => $_getN(0);
  @$pb.TagNumber(1)
  set bot(Bot value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBot() => $_has(0);
  @$pb.TagNumber(1)
  void clearBot() => $_clearField(1);
  @$pb.TagNumber(1)
  Bot ensureBot() => $_ensure(0);
}

class DeleteBotResponse extends $pb.GeneratedMessage {
  factory DeleteBotResponse() => create();

  DeleteBotResponse._();

  factory DeleteBotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteBotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteBotResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteBotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteBotResponse copyWith(void Function(DeleteBotResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteBotResponse))
          as DeleteBotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteBotResponse create() => DeleteBotResponse._();
  @$core.override
  DeleteBotResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteBotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteBotResponse>(create);
  static DeleteBotResponse? _defaultInstance;
}

class GetBotResponse extends $pb.GeneratedMessage {
  factory GetBotResponse({
    Bot? bot,
  }) {
    final result = create();
    if (bot != null) result.bot = bot;
    return result;
  }

  GetBotResponse._();

  factory GetBotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBotResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<Bot>(1, _omitFieldNames ? '' : 'bot', subBuilder: Bot.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBotResponse copyWith(void Function(GetBotResponse) updates) =>
      super.copyWith((message) => updates(message as GetBotResponse))
          as GetBotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBotResponse create() => GetBotResponse._();
  @$core.override
  GetBotResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBotResponse>(create);
  static GetBotResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Bot get bot => $_getN(0);
  @$pb.TagNumber(1)
  set bot(Bot value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBot() => $_has(0);
  @$pb.TagNumber(1)
  void clearBot() => $_clearField(1);
  @$pb.TagNumber(1)
  Bot ensureBot() => $_ensure(0);
}

class ListBotsResponse extends $pb.GeneratedMessage {
  factory ListBotsResponse({
    BotList? botList,
  }) {
    final result = create();
    if (botList != null) result.botList = botList;
    return result;
  }

  ListBotsResponse._();

  factory ListBotsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListBotsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListBotsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<BotList>(1, _omitFieldNames ? '' : 'botList',
        subBuilder: BotList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBotsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBotsResponse copyWith(void Function(ListBotsResponse) updates) =>
      super.copyWith((message) => updates(message as ListBotsResponse))
          as ListBotsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListBotsResponse create() => ListBotsResponse._();
  @$core.override
  ListBotsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListBotsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListBotsResponse>(create);
  static ListBotsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  BotList get botList => $_getN(0);
  @$pb.TagNumber(1)
  set botList(BotList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBotList() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotList() => $_clearField(1);
  @$pb.TagNumber(1)
  BotList ensureBotList() => $_ensure(0);
}

class RegenerateTokenResponse extends $pb.GeneratedMessage {
  factory RegenerateTokenResponse({
    TokenResponse? tokenResponse,
  }) {
    final result = create();
    if (tokenResponse != null) result.tokenResponse = tokenResponse;
    return result;
  }

  RegenerateTokenResponse._();

  factory RegenerateTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegenerateTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegenerateTokenResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<TokenResponse>(1, _omitFieldNames ? '' : 'tokenResponse',
        subBuilder: TokenResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegenerateTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegenerateTokenResponse copyWith(
          void Function(RegenerateTokenResponse) updates) =>
      super.copyWith((message) => updates(message as RegenerateTokenResponse))
          as RegenerateTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegenerateTokenResponse create() => RegenerateTokenResponse._();
  @$core.override
  RegenerateTokenResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegenerateTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegenerateTokenResponse>(create);
  static RegenerateTokenResponse? _defaultInstance;

  @$pb.TagNumber(1)
  TokenResponse get tokenResponse => $_getN(0);
  @$pb.TagNumber(1)
  set tokenResponse(TokenResponse value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTokenResponse() => $_has(0);
  @$pb.TagNumber(1)
  void clearTokenResponse() => $_clearField(1);
  @$pb.TagNumber(1)
  TokenResponse ensureTokenResponse() => $_ensure(0);
}

class RegisterCommandsResponse extends $pb.GeneratedMessage {
  factory RegisterCommandsResponse() => create();

  RegisterCommandsResponse._();

  factory RegisterCommandsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterCommandsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterCommandsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterCommandsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterCommandsResponse copyWith(
          void Function(RegisterCommandsResponse) updates) =>
      super.copyWith((message) => updates(message as RegisterCommandsResponse))
          as RegisterCommandsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterCommandsResponse create() => RegisterCommandsResponse._();
  @$core.override
  RegisterCommandsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterCommandsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterCommandsResponse>(create);
  static RegisterCommandsResponse? _defaultInstance;
}

class GetCommandsResponse extends $pb.GeneratedMessage {
  factory GetCommandsResponse({
    CommandList? commandList,
  }) {
    final result = create();
    if (commandList != null) result.commandList = commandList;
    return result;
  }

  GetCommandsResponse._();

  factory GetCommandsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetCommandsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCommandsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<CommandList>(1, _omitFieldNames ? '' : 'commandList',
        subBuilder: CommandList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCommandsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCommandsResponse copyWith(void Function(GetCommandsResponse) updates) =>
      super.copyWith((message) => updates(message as GetCommandsResponse))
          as GetCommandsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetCommandsResponse create() => GetCommandsResponse._();
  @$core.override
  GetCommandsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetCommandsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCommandsResponse>(create);
  static GetCommandsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CommandList get commandList => $_getN(0);
  @$pb.TagNumber(1)
  set commandList(CommandList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCommandList() => $_has(0);
  @$pb.TagNumber(1)
  void clearCommandList() => $_clearField(1);
  @$pb.TagNumber(1)
  CommandList ensureCommandList() => $_ensure(0);
}

class SetWebhookURLResponse extends $pb.GeneratedMessage {
  factory SetWebhookURLResponse() => create();

  SetWebhookURLResponse._();

  factory SetWebhookURLResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetWebhookURLResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetWebhookURLResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetWebhookURLResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetWebhookURLResponse copyWith(
          void Function(SetWebhookURLResponse) updates) =>
      super.copyWith((message) => updates(message as SetWebhookURLResponse))
          as SetWebhookURLResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetWebhookURLResponse create() => SetWebhookURLResponse._();
  @$core.override
  SetWebhookURLResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetWebhookURLResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetWebhookURLResponse>(create);
  static SetWebhookURLResponse? _defaultInstance;
}

class GetWebhookURLResponse extends $pb.GeneratedMessage {
  factory GetWebhookURLResponse({
    $core.String? url,
  }) {
    final result = create();
    if (url != null) result.url = url;
    return result;
  }

  GetWebhookURLResponse._();

  factory GetWebhookURLResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetWebhookURLResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetWebhookURLResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'url')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetWebhookURLResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetWebhookURLResponse copyWith(
          void Function(GetWebhookURLResponse) updates) =>
      super.copyWith((message) => updates(message as GetWebhookURLResponse))
          as GetWebhookURLResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetWebhookURLResponse create() => GetWebhookURLResponse._();
  @$core.override
  GetWebhookURLResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetWebhookURLResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetWebhookURLResponse>(create);
  static GetWebhookURLResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get url => $_getSZ(0);
  @$pb.TagNumber(1)
  set url($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUrl() => $_has(0);
  @$pb.TagNumber(1)
  void clearUrl() => $_clearField(1);
}

class SetChatWhitelistResponse extends $pb.GeneratedMessage {
  factory SetChatWhitelistResponse() => create();

  SetChatWhitelistResponse._();

  factory SetChatWhitelistResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetChatWhitelistResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetChatWhitelistResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetChatWhitelistResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetChatWhitelistResponse copyWith(
          void Function(SetChatWhitelistResponse) updates) =>
      super.copyWith((message) => updates(message as SetChatWhitelistResponse))
          as SetChatWhitelistResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetChatWhitelistResponse create() => SetChatWhitelistResponse._();
  @$core.override
  SetChatWhitelistResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetChatWhitelistResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetChatWhitelistResponse>(create);
  static SetChatWhitelistResponse? _defaultInstance;
}

class GetChatWhitelistResponse extends $pb.GeneratedMessage {
  factory GetChatWhitelistResponse({
    $core.Iterable<$2.ChatRef>? allowedChats,
  }) {
    final result = create();
    if (allowedChats != null) result.allowedChats.addAll(allowedChats);
    return result;
  }

  GetChatWhitelistResponse._();

  factory GetChatWhitelistResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatWhitelistResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatWhitelistResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..pPM<$2.ChatRef>(1, _omitFieldNames ? '' : 'allowedChats',
        subBuilder: $2.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatWhitelistResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatWhitelistResponse copyWith(
          void Function(GetChatWhitelistResponse) updates) =>
      super.copyWith((message) => updates(message as GetChatWhitelistResponse))
          as GetChatWhitelistResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatWhitelistResponse create() => GetChatWhitelistResponse._();
  @$core.override
  GetChatWhitelistResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatWhitelistResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatWhitelistResponse>(create);
  static GetChatWhitelistResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$2.ChatRef> get allowedChats => $_getList(0);
}

class SendBotMessageResponse extends $pb.GeneratedMessage {
  factory SendBotMessageResponse({
    $3.Message? message,
  }) {
    final result = create();
    if (message != null) result.message = message;
    return result;
  }

  SendBotMessageResponse._();

  factory SendBotMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendBotMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendBotMessageResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<$3.Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: $3.Message.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendBotMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendBotMessageResponse copyWith(
          void Function(SendBotMessageResponse) updates) =>
      super.copyWith((message) => updates(message as SendBotMessageResponse))
          as SendBotMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendBotMessageResponse create() => SendBotMessageResponse._();
  @$core.override
  SendBotMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendBotMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendBotMessageResponse>(create);
  static SendBotMessageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $3.Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message($3.Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.Message ensureMessage() => $_ensure(0);
}

class EditBotMessageResponse extends $pb.GeneratedMessage {
  factory EditBotMessageResponse({
    $3.Message? message,
  }) {
    final result = create();
    if (message != null) result.message = message;
    return result;
  }

  EditBotMessageResponse._();

  factory EditBotMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EditBotMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EditBotMessageResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<$3.Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: $3.Message.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EditBotMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EditBotMessageResponse copyWith(
          void Function(EditBotMessageResponse) updates) =>
      super.copyWith((message) => updates(message as EditBotMessageResponse))
          as EditBotMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EditBotMessageResponse create() => EditBotMessageResponse._();
  @$core.override
  EditBotMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EditBotMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EditBotMessageResponse>(create);
  static EditBotMessageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $3.Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message($3.Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.Message ensureMessage() => $_ensure(0);
}

class SendEphemeralResponse extends $pb.GeneratedMessage {
  factory SendEphemeralResponse() => create();

  SendEphemeralResponse._();

  factory SendEphemeralResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendEphemeralResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendEphemeralResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendEphemeralResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendEphemeralResponse copyWith(
          void Function(SendEphemeralResponse) updates) =>
      super.copyWith((message) => updates(message as SendEphemeralResponse))
          as SendEphemeralResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendEphemeralResponse create() => SendEphemeralResponse._();
  @$core.override
  SendEphemeralResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendEphemeralResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendEphemeralResponse>(create);
  static SendEphemeralResponse? _defaultInstance;
}

class DeferResponseResponse extends $pb.GeneratedMessage {
  factory DeferResponseResponse() => create();

  DeferResponseResponse._();

  factory DeferResponseResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeferResponseResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeferResponseResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeferResponseResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeferResponseResponse copyWith(
          void Function(DeferResponseResponse) updates) =>
      super.copyWith((message) => updates(message as DeferResponseResponse))
          as DeferResponseResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeferResponseResponse create() => DeferResponseResponse._();
  @$core.override
  DeferResponseResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeferResponseResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeferResponseResponse>(create);
  static DeferResponseResponse? _defaultInstance;
}

class PollEventsResponse extends $pb.GeneratedMessage {
  factory PollEventsResponse({
    BotEvent? botEvent,
  }) {
    final result = create();
    if (botEvent != null) result.botEvent = botEvent;
    return result;
  }

  PollEventsResponse._();

  factory PollEventsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PollEventsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PollEventsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<BotEvent>(1, _omitFieldNames ? '' : 'botEvent',
        subBuilder: BotEvent.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PollEventsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PollEventsResponse copyWith(void Function(PollEventsResponse) updates) =>
      super.copyWith((message) => updates(message as PollEventsResponse))
          as PollEventsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PollEventsResponse create() => PollEventsResponse._();
  @$core.override
  PollEventsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PollEventsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PollEventsResponse>(create);
  static PollEventsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  BotEvent get botEvent => $_getN(0);
  @$pb.TagNumber(1)
  set botEvent(BotEvent value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBotEvent() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotEvent() => $_clearField(1);
  @$pb.TagNumber(1)
  BotEvent ensureBotEvent() => $_ensure(0);
}

class ValidateManifestRequest extends $pb.GeneratedMessage {
  factory ValidateManifestRequest({
    $core.String? manifestYaml,
  }) {
    final result = create();
    if (manifestYaml != null) result.manifestYaml = manifestYaml;
    return result;
  }

  ValidateManifestRequest._();

  factory ValidateManifestRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateManifestRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateManifestRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'manifestYaml')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateManifestRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateManifestRequest copyWith(
          void Function(ValidateManifestRequest) updates) =>
      super.copyWith((message) => updates(message as ValidateManifestRequest))
          as ValidateManifestRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateManifestRequest create() => ValidateManifestRequest._();
  @$core.override
  ValidateManifestRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateManifestRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateManifestRequest>(create);
  static ValidateManifestRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get manifestYaml => $_getSZ(0);
  @$pb.TagNumber(1)
  set manifestYaml($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasManifestYaml() => $_has(0);
  @$pb.TagNumber(1)
  void clearManifestYaml() => $_clearField(1);
}

class ValidateManifestResponse extends $pb.GeneratedMessage {
  factory ValidateManifestResponse({
    $core.bool? valid,
    $core.Iterable<$core.String>? errors,
    $core.String? normalizedManifestJson,
  }) {
    final result = create();
    if (valid != null) result.valid = valid;
    if (errors != null) result.errors.addAll(errors);
    if (normalizedManifestJson != null)
      result.normalizedManifestJson = normalizedManifestJson;
    return result;
  }

  ValidateManifestResponse._();

  factory ValidateManifestResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateManifestResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateManifestResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'valid')
    ..pPS(2, _omitFieldNames ? '' : 'errors')
    ..aOS(3, _omitFieldNames ? '' : 'normalizedManifestJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateManifestResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateManifestResponse copyWith(
          void Function(ValidateManifestResponse) updates) =>
      super.copyWith((message) => updates(message as ValidateManifestResponse))
          as ValidateManifestResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateManifestResponse create() => ValidateManifestResponse._();
  @$core.override
  ValidateManifestResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateManifestResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateManifestResponse>(create);
  static ValidateManifestResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get valid => $_getBF(0);
  @$pb.TagNumber(1)
  set valid($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasValid() => $_has(0);
  @$pb.TagNumber(1)
  void clearValid() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get errors => $_getList(1);

  @$pb.TagNumber(3)
  $core.String get normalizedManifestJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set normalizedManifestJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNormalizedManifestJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearNormalizedManifestJson() => $_clearField(3);
}

class ApplyManifestRequest extends $pb.GeneratedMessage {
  factory ApplyManifestRequest({
    $core.String? botId,
    $core.String? manifestYaml,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (manifestYaml != null) result.manifestYaml = manifestYaml;
    return result;
  }

  ApplyManifestRequest._();

  factory ApplyManifestRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyManifestRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyManifestRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'manifestYaml')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyManifestRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyManifestRequest copyWith(void Function(ApplyManifestRequest) updates) =>
      super.copyWith((message) => updates(message as ApplyManifestRequest))
          as ApplyManifestRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyManifestRequest create() => ApplyManifestRequest._();
  @$core.override
  ApplyManifestRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyManifestRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyManifestRequest>(create);
  static ApplyManifestRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get manifestYaml => $_getSZ(1);
  @$pb.TagNumber(2)
  set manifestYaml($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasManifestYaml() => $_has(1);
  @$pb.TagNumber(2)
  void clearManifestYaml() => $_clearField(2);
}

class ApplyManifestResponse extends $pb.GeneratedMessage {
  factory ApplyManifestResponse({
    Bot? bot,
  }) {
    final result = create();
    if (bot != null) result.bot = bot;
    return result;
  }

  ApplyManifestResponse._();

  factory ApplyManifestResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyManifestResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyManifestResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<Bot>(1, _omitFieldNames ? '' : 'bot', subBuilder: Bot.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyManifestResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyManifestResponse copyWith(
          void Function(ApplyManifestResponse) updates) =>
      super.copyWith((message) => updates(message as ApplyManifestResponse))
          as ApplyManifestResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyManifestResponse create() => ApplyManifestResponse._();
  @$core.override
  ApplyManifestResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyManifestResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyManifestResponse>(create);
  static ApplyManifestResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Bot get bot => $_getN(0);
  @$pb.TagNumber(1)
  set bot(Bot value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBot() => $_has(0);
  @$pb.TagNumber(1)
  void clearBot() => $_clearField(1);
  @$pb.TagNumber(1)
  Bot ensureBot() => $_ensure(0);
}

class InstallBotInSpaceRequest extends $pb.GeneratedMessage {
  factory InstallBotInSpaceRequest({
    $core.String? botId,
    $core.String? spaceId,
    $core.Iterable<$2.ChatRef>? allowedChats,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (spaceId != null) result.spaceId = spaceId;
    if (allowedChats != null) result.allowedChats.addAll(allowedChats);
    return result;
  }

  InstallBotInSpaceRequest._();

  factory InstallBotInSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory InstallBotInSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InstallBotInSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..pPM<$2.ChatRef>(3, _omitFieldNames ? '' : 'allowedChats',
        subBuilder: $2.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InstallBotInSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InstallBotInSpaceRequest copyWith(
          void Function(InstallBotInSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as InstallBotInSpaceRequest))
          as InstallBotInSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static InstallBotInSpaceRequest create() => InstallBotInSpaceRequest._();
  @$core.override
  InstallBotInSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static InstallBotInSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<InstallBotInSpaceRequest>(create);
  static InstallBotInSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$2.ChatRef> get allowedChats => $_getList(2);
}

class InstallBotInSpaceResponse extends $pb.GeneratedMessage {
  factory InstallBotInSpaceResponse({
    $core.String? installationId,
  }) {
    final result = create();
    if (installationId != null) result.installationId = installationId;
    return result;
  }

  InstallBotInSpaceResponse._();

  factory InstallBotInSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory InstallBotInSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InstallBotInSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'installationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InstallBotInSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InstallBotInSpaceResponse copyWith(
          void Function(InstallBotInSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as InstallBotInSpaceResponse))
          as InstallBotInSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static InstallBotInSpaceResponse create() => InstallBotInSpaceResponse._();
  @$core.override
  InstallBotInSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static InstallBotInSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<InstallBotInSpaceResponse>(create);
  static InstallBotInSpaceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get installationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set installationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInstallationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearInstallationId() => $_clearField(1);
}

class UninstallBotFromSpaceRequest extends $pb.GeneratedMessage {
  factory UninstallBotFromSpaceRequest({
    $core.String? botId,
    $core.String? spaceId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  UninstallBotFromSpaceRequest._();

  factory UninstallBotFromSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UninstallBotFromSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UninstallBotFromSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UninstallBotFromSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UninstallBotFromSpaceRequest copyWith(
          void Function(UninstallBotFromSpaceRequest) updates) =>
      super.copyWith(
              (message) => updates(message as UninstallBotFromSpaceRequest))
          as UninstallBotFromSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UninstallBotFromSpaceRequest create() =>
      UninstallBotFromSpaceRequest._();
  @$core.override
  UninstallBotFromSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UninstallBotFromSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UninstallBotFromSpaceRequest>(create);
  static UninstallBotFromSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);
}

class UninstallBotFromSpaceResponse extends $pb.GeneratedMessage {
  factory UninstallBotFromSpaceResponse() => create();

  UninstallBotFromSpaceResponse._();

  factory UninstallBotFromSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UninstallBotFromSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UninstallBotFromSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UninstallBotFromSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UninstallBotFromSpaceResponse copyWith(
          void Function(UninstallBotFromSpaceResponse) updates) =>
      super.copyWith(
              (message) => updates(message as UninstallBotFromSpaceResponse))
          as UninstallBotFromSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UninstallBotFromSpaceResponse create() =>
      UninstallBotFromSpaceResponse._();
  @$core.override
  UninstallBotFromSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UninstallBotFromSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UninstallBotFromSpaceResponse>(create);
  static UninstallBotFromSpaceResponse? _defaultInstance;
}

class ListInstalledBotsRequest extends $pb.GeneratedMessage {
  factory ListInstalledBotsRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  ListInstalledBotsRequest._();

  factory ListInstalledBotsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListInstalledBotsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListInstalledBotsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInstalledBotsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInstalledBotsRequest copyWith(
          void Function(ListInstalledBotsRequest) updates) =>
      super.copyWith((message) => updates(message as ListInstalledBotsRequest))
          as ListInstalledBotsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListInstalledBotsRequest create() => ListInstalledBotsRequest._();
  @$core.override
  ListInstalledBotsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListInstalledBotsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListInstalledBotsRequest>(create);
  static ListInstalledBotsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class InstalledBot extends $pb.GeneratedMessage {
  factory InstalledBot({
    Bot? bot,
    $core.String? installationId,
    $core.Iterable<$2.ChatRef>? allowedChats,
  }) {
    final result = create();
    if (bot != null) result.bot = bot;
    if (installationId != null) result.installationId = installationId;
    if (allowedChats != null) result.allowedChats.addAll(allowedChats);
    return result;
  }

  InstalledBot._();

  factory InstalledBot.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory InstalledBot.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InstalledBot',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<Bot>(1, _omitFieldNames ? '' : 'bot', subBuilder: Bot.create)
    ..aOS(2, _omitFieldNames ? '' : 'installationId')
    ..pPM<$2.ChatRef>(3, _omitFieldNames ? '' : 'allowedChats',
        subBuilder: $2.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InstalledBot clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InstalledBot copyWith(void Function(InstalledBot) updates) =>
      super.copyWith((message) => updates(message as InstalledBot))
          as InstalledBot;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static InstalledBot create() => InstalledBot._();
  @$core.override
  InstalledBot createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static InstalledBot getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<InstalledBot>(create);
  static InstalledBot? _defaultInstance;

  @$pb.TagNumber(1)
  Bot get bot => $_getN(0);
  @$pb.TagNumber(1)
  set bot(Bot value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBot() => $_has(0);
  @$pb.TagNumber(1)
  void clearBot() => $_clearField(1);
  @$pb.TagNumber(1)
  Bot ensureBot() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get installationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set installationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInstallationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearInstallationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$2.ChatRef> get allowedChats => $_getList(2);
}

class ListInstalledBotsResponse extends $pb.GeneratedMessage {
  factory ListInstalledBotsResponse({
    $core.Iterable<InstalledBot>? installedBots,
  }) {
    final result = create();
    if (installedBots != null) result.installedBots.addAll(installedBots);
    return result;
  }

  ListInstalledBotsResponse._();

  factory ListInstalledBotsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListInstalledBotsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListInstalledBotsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..pPM<InstalledBot>(1, _omitFieldNames ? '' : 'installedBots',
        subBuilder: InstalledBot.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInstalledBotsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInstalledBotsResponse copyWith(
          void Function(ListInstalledBotsResponse) updates) =>
      super.copyWith((message) => updates(message as ListInstalledBotsResponse))
          as ListInstalledBotsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListInstalledBotsResponse create() => ListInstalledBotsResponse._();
  @$core.override
  ListInstalledBotsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListInstalledBotsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListInstalledBotsResponse>(create);
  static ListInstalledBotsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<InstalledBot> get installedBots => $_getList(0);
}

class SlashCommandOption extends $pb.GeneratedMessage {
  factory SlashCommandOption({
    $core.String? name,
    $core.String? type,
    $core.bool? required,
    $core.bool? autocomplete,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (type != null) result.type = type;
    if (required != null) result.required = required;
    if (autocomplete != null) result.autocomplete = autocomplete;
    return result;
  }

  SlashCommandOption._();

  factory SlashCommandOption.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SlashCommandOption.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SlashCommandOption',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'type')
    ..aOB(3, _omitFieldNames ? '' : 'required')
    ..aOB(4, _omitFieldNames ? '' : 'autocomplete')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SlashCommandOption clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SlashCommandOption copyWith(void Function(SlashCommandOption) updates) =>
      super.copyWith((message) => updates(message as SlashCommandOption))
          as SlashCommandOption;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SlashCommandOption create() => SlashCommandOption._();
  @$core.override
  SlashCommandOption createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SlashCommandOption getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SlashCommandOption>(create);
  static SlashCommandOption? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get type => $_getSZ(1);
  @$pb.TagNumber(2)
  set type($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get required => $_getBF(2);
  @$pb.TagNumber(3)
  set required($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRequired() => $_has(2);
  @$pb.TagNumber(3)
  void clearRequired() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get autocomplete => $_getBF(3);
  @$pb.TagNumber(4)
  set autocomplete($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAutocomplete() => $_has(3);
  @$pb.TagNumber(4)
  void clearAutocomplete() => $_clearField(4);
}

class SlashCommand extends $pb.GeneratedMessage {
  factory SlashCommand({
    $core.String? botId,
    $core.String? botName,
    $core.String? name,
    $core.String? description,
    $core.Iterable<SlashCommandOption>? options,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (botName != null) result.botName = botName;
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (options != null) result.options.addAll(options);
    return result;
  }

  SlashCommand._();

  factory SlashCommand.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SlashCommand.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SlashCommand',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'botName')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..pPM<SlashCommandOption>(5, _omitFieldNames ? '' : 'options',
        subBuilder: SlashCommandOption.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SlashCommand clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SlashCommand copyWith(void Function(SlashCommand) updates) =>
      super.copyWith((message) => updates(message as SlashCommand))
          as SlashCommand;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SlashCommand create() => SlashCommand._();
  @$core.override
  SlashCommand createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SlashCommand getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SlashCommand>(create);
  static SlashCommand? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get botName => $_getSZ(1);
  @$pb.TagNumber(2)
  set botName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBotName() => $_has(1);
  @$pb.TagNumber(2)
  void clearBotName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<SlashCommandOption> get options => $_getList(4);
}

class ListSlashCommandsForChatRequest extends $pb.GeneratedMessage {
  factory ListSlashCommandsForChatRequest({
    $2.ChatRef? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  ListSlashCommandsForChatRequest._();

  factory ListSlashCommandsForChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListSlashCommandsForChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSlashCommandsForChatRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<$2.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSlashCommandsForChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSlashCommandsForChatRequest copyWith(
          void Function(ListSlashCommandsForChatRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListSlashCommandsForChatRequest))
          as ListSlashCommandsForChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListSlashCommandsForChatRequest create() =>
      ListSlashCommandsForChatRequest._();
  @$core.override
  ListSlashCommandsForChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListSlashCommandsForChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSlashCommandsForChatRequest>(
          create);
  static ListSlashCommandsForChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $2.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($2.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.ChatRef ensureChat() => $_ensure(0);
}

class ListSlashCommandsForChatResponse extends $pb.GeneratedMessage {
  factory ListSlashCommandsForChatResponse({
    $core.Iterable<SlashCommand>? commands,
  }) {
    final result = create();
    if (commands != null) result.commands.addAll(commands);
    return result;
  }

  ListSlashCommandsForChatResponse._();

  factory ListSlashCommandsForChatResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListSlashCommandsForChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSlashCommandsForChatResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..pPM<SlashCommand>(1, _omitFieldNames ? '' : 'commands',
        subBuilder: SlashCommand.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSlashCommandsForChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSlashCommandsForChatResponse copyWith(
          void Function(ListSlashCommandsForChatResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListSlashCommandsForChatResponse))
          as ListSlashCommandsForChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListSlashCommandsForChatResponse create() =>
      ListSlashCommandsForChatResponse._();
  @$core.override
  ListSlashCommandsForChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListSlashCommandsForChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSlashCommandsForChatResponse>(
          create);
  static ListSlashCommandsForChatResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SlashCommand> get commands => $_getList(0);
}

class ExecuteSlashInteractionRequest extends $pb.GeneratedMessage {
  factory ExecuteSlashInteractionRequest({
    $2.ChatRef? chat,
    $core.String? botId,
    $core.String? commandName,
    $core.String? optionsJson,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (botId != null) result.botId = botId;
    if (commandName != null) result.commandName = commandName;
    if (optionsJson != null) result.optionsJson = optionsJson;
    return result;
  }

  ExecuteSlashInteractionRequest._();

  factory ExecuteSlashInteractionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ExecuteSlashInteractionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExecuteSlashInteractionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<$2.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'botId')
    ..aOS(3, _omitFieldNames ? '' : 'commandName')
    ..aOS(4, _omitFieldNames ? '' : 'optionsJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExecuteSlashInteractionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExecuteSlashInteractionRequest copyWith(
          void Function(ExecuteSlashInteractionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ExecuteSlashInteractionRequest))
          as ExecuteSlashInteractionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ExecuteSlashInteractionRequest create() =>
      ExecuteSlashInteractionRequest._();
  @$core.override
  ExecuteSlashInteractionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ExecuteSlashInteractionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExecuteSlashInteractionRequest>(create);
  static ExecuteSlashInteractionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $2.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($2.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get botId => $_getSZ(1);
  @$pb.TagNumber(2)
  set botId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBotId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBotId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get commandName => $_getSZ(2);
  @$pb.TagNumber(3)
  set commandName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCommandName() => $_has(2);
  @$pb.TagNumber(3)
  void clearCommandName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get optionsJson => $_getSZ(3);
  @$pb.TagNumber(4)
  set optionsJson($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOptionsJson() => $_has(3);
  @$pb.TagNumber(4)
  void clearOptionsJson() => $_clearField(4);
}

class ExecuteSlashInteractionResponse extends $pb.GeneratedMessage {
  factory ExecuteSlashInteractionResponse({
    $core.String? interactionToken,
    $core.String? content,
    $core.bool? isEphemeral,
    $core.bool? deferred,
    $3.Message? message,
    $core.String? errorCode,
    $core.String? errorMessage,
  }) {
    final result = create();
    if (interactionToken != null) result.interactionToken = interactionToken;
    if (content != null) result.content = content;
    if (isEphemeral != null) result.isEphemeral = isEphemeral;
    if (deferred != null) result.deferred = deferred;
    if (message != null) result.message = message;
    if (errorCode != null) result.errorCode = errorCode;
    if (errorMessage != null) result.errorMessage = errorMessage;
    return result;
  }

  ExecuteSlashInteractionResponse._();

  factory ExecuteSlashInteractionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ExecuteSlashInteractionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExecuteSlashInteractionResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'interactionToken')
    ..aOS(2, _omitFieldNames ? '' : 'content')
    ..aOB(3, _omitFieldNames ? '' : 'isEphemeral')
    ..aOB(4, _omitFieldNames ? '' : 'deferred')
    ..aOM<$3.Message>(5, _omitFieldNames ? '' : 'message',
        subBuilder: $3.Message.create)
    ..aOS(6, _omitFieldNames ? '' : 'errorCode')
    ..aOS(7, _omitFieldNames ? '' : 'errorMessage')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExecuteSlashInteractionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExecuteSlashInteractionResponse copyWith(
          void Function(ExecuteSlashInteractionResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ExecuteSlashInteractionResponse))
          as ExecuteSlashInteractionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ExecuteSlashInteractionResponse create() =>
      ExecuteSlashInteractionResponse._();
  @$core.override
  ExecuteSlashInteractionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ExecuteSlashInteractionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExecuteSlashInteractionResponse>(
          create);
  static ExecuteSlashInteractionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get interactionToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set interactionToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInteractionToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearInteractionToken() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get content => $_getSZ(1);
  @$pb.TagNumber(2)
  set content($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContent() => $_has(1);
  @$pb.TagNumber(2)
  void clearContent() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isEphemeral => $_getBF(2);
  @$pb.TagNumber(3)
  set isEphemeral($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsEphemeral() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsEphemeral() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get deferred => $_getBF(3);
  @$pb.TagNumber(4)
  set deferred($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeferred() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeferred() => $_clearField(4);

  @$pb.TagNumber(5)
  $3.Message get message => $_getN(4);
  @$pb.TagNumber(5)
  set message($3.Message value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasMessage() => $_has(4);
  @$pb.TagNumber(5)
  void clearMessage() => $_clearField(5);
  @$pb.TagNumber(5)
  $3.Message ensureMessage() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get errorCode => $_getSZ(5);
  @$pb.TagNumber(6)
  set errorCode($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasErrorCode() => $_has(5);
  @$pb.TagNumber(6)
  void clearErrorCode() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get errorMessage => $_getSZ(6);
  @$pb.TagNumber(7)
  set errorMessage($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasErrorMessage() => $_has(6);
  @$pb.TagNumber(7)
  void clearErrorMessage() => $_clearField(7);
}

class CompleteInteractionRequest extends $pb.GeneratedMessage {
  factory CompleteInteractionRequest({
    $core.String? interactionToken,
    $core.String? content,
    $core.bool? isEphemeral,
    $core.bool? deferred,
  }) {
    final result = create();
    if (interactionToken != null) result.interactionToken = interactionToken;
    if (content != null) result.content = content;
    if (isEphemeral != null) result.isEphemeral = isEphemeral;
    if (deferred != null) result.deferred = deferred;
    return result;
  }

  CompleteInteractionRequest._();

  factory CompleteInteractionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompleteInteractionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteInteractionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'interactionToken')
    ..aOS(2, _omitFieldNames ? '' : 'content')
    ..aOB(3, _omitFieldNames ? '' : 'isEphemeral')
    ..aOB(4, _omitFieldNames ? '' : 'deferred')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteInteractionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteInteractionRequest copyWith(
          void Function(CompleteInteractionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CompleteInteractionRequest))
          as CompleteInteractionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompleteInteractionRequest create() => CompleteInteractionRequest._();
  @$core.override
  CompleteInteractionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompleteInteractionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteInteractionRequest>(create);
  static CompleteInteractionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get interactionToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set interactionToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInteractionToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearInteractionToken() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get content => $_getSZ(1);
  @$pb.TagNumber(2)
  set content($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContent() => $_has(1);
  @$pb.TagNumber(2)
  void clearContent() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isEphemeral => $_getBF(2);
  @$pb.TagNumber(3)
  set isEphemeral($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsEphemeral() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsEphemeral() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get deferred => $_getBF(3);
  @$pb.TagNumber(4)
  set deferred($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeferred() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeferred() => $_clearField(4);
}

class CompleteInteractionResponse extends $pb.GeneratedMessage {
  factory CompleteInteractionResponse({
    $3.Message? message,
  }) {
    final result = create();
    if (message != null) result.message = message;
    return result;
  }

  CompleteInteractionResponse._();

  factory CompleteInteractionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompleteInteractionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteInteractionResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.bot.v1'),
      createEmptyInstance: create)
    ..aOM<$3.Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: $3.Message.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteInteractionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteInteractionResponse copyWith(
          void Function(CompleteInteractionResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CompleteInteractionResponse))
          as CompleteInteractionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompleteInteractionResponse create() =>
      CompleteInteractionResponse._();
  @$core.override
  CompleteInteractionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompleteInteractionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteInteractionResponse>(create);
  static CompleteInteractionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $3.Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message($3.Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.Message ensureMessage() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
