// This is a generated file - do not edit.
//
// Generated from voice/common/v1/bot_message.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Exact send-only installation incarnation. This is an authenticated selector,
/// never portable authority or a grant to read history/member lists.
/// @voice.unknown_fields=reject
class BotMessageSendBinding extends $pb.GeneratedMessage {
  factory BotMessageSendBinding({
    $core.int? protocolVersion,
    $core.String? botId,
    $core.String? actorProfileId,
    $core.String? spaceId,
    $core.String? installationId,
    $core.String? installOperationId,
    $fixnum.Int64? installationGeneration,
    $core.String? chatId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (botId != null) result.botId = botId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (spaceId != null) result.spaceId = spaceId;
    if (installationId != null) result.installationId = installationId;
    if (installOperationId != null)
      result.installOperationId = installOperationId;
    if (installationGeneration != null)
      result.installationGeneration = installationGeneration;
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  BotMessageSendBinding._();

  factory BotMessageSendBinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BotMessageSendBinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BotMessageSendBinding',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'botId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'spaceId')
    ..aOS(5, _omitFieldNames ? '' : 'installationId')
    ..aOS(6, _omitFieldNames ? '' : 'installOperationId')
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'installationGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(8, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotMessageSendBinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotMessageSendBinding copyWith(
          void Function(BotMessageSendBinding) updates) =>
      super.copyWith((message) => updates(message as BotMessageSendBinding))
          as BotMessageSendBinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BotMessageSendBinding create() => BotMessageSendBinding._();
  @$core.override
  BotMessageSendBinding createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BotMessageSendBinding getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BotMessageSendBinding>(create);
  static BotMessageSendBinding? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get botId => $_getSZ(1);
  @$pb.TagNumber(2)
  set botId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBotId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBotId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get actorProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set actorProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasActorProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearActorProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get spaceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set spaceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSpaceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpaceId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get installationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set installationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasInstallationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearInstallationId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get installOperationId => $_getSZ(5);
  @$pb.TagNumber(6)
  set installOperationId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasInstallOperationId() => $_has(5);
  @$pb.TagNumber(6)
  void clearInstallOperationId() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get installationGeneration => $_getI64(6);
  @$pb.TagNumber(7)
  set installationGeneration($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasInstallationGeneration() => $_has(6);
  @$pb.TagNumber(7)
  void clearInstallationGeneration() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get chatId => $_getSZ(7);
  @$pb.TagNumber(8)
  set chatId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasChatId() => $_has(7);
  @$pb.TagNumber(8)
  void clearChatId() => $_clearField(8);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
