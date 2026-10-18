// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat_lifecycle.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'chat_lifecycle.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'chat_lifecycle.pbenum.dart';

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ResolveChatLifecycleRequest extends $pb.GeneratedMessage {
  factory ResolveChatLifecycleRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? chatId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  ResolveChatLifecycleRequest._();

  factory ResolveChatLifecycleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveChatLifecycleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveChatLifecycleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveChatLifecycleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveChatLifecycleRequest copyWith(
          void Function(ResolveChatLifecycleRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveChatLifecycleRequest))
          as ResolveChatLifecycleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveChatLifecycleRequest create() =>
      ResolveChatLifecycleRequest._();
  @$core.override
  ResolveChatLifecycleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveChatLifecycleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveChatLifecycleRequest>(create);
  static ResolveChatLifecycleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get operationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);
}

class ResolveChatLifecycleResponse extends $pb.GeneratedMessage {
  factory ResolveChatLifecycleResponse({
    $core.int? protocolVersion,
    $core.String? chatId,
    $core.String? spaceId,
    $fixnum.Int64? generation,
    ChatLifecycleState? state,
    $core.String? deletionOperationId,
    $core.int? schemaVersion,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (chatId != null) result.chatId = chatId;
    if (spaceId != null) result.spaceId = spaceId;
    if (generation != null) result.generation = generation;
    if (state != null) result.state = state;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (schemaVersion != null) result.schemaVersion = schemaVersion;
    return result;
  }

  ResolveChatLifecycleResponse._();

  factory ResolveChatLifecycleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveChatLifecycleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveChatLifecycleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<ChatLifecycleState>(5, _omitFieldNames ? '' : 'state',
        enumValues: ChatLifecycleState.values)
    ..aOS(6, _omitFieldNames ? '' : 'deletionOperationId')
    ..aI(7, _omitFieldNames ? '' : 'schemaVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveChatLifecycleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveChatLifecycleResponse copyWith(
          void Function(ResolveChatLifecycleResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveChatLifecycleResponse))
          as ResolveChatLifecycleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveChatLifecycleResponse create() =>
      ResolveChatLifecycleResponse._();
  @$core.override
  ResolveChatLifecycleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveChatLifecycleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveChatLifecycleResponse>(create);
  static ResolveChatLifecycleResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  /// Empty only for an authoritative non-Space chat.
  @$pb.TagNumber(3)
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get generation => $_getI64(3);
  @$pb.TagNumber(4)
  set generation($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  ChatLifecycleState get state => $_getN(4);
  @$pb.TagNumber(5)
  set state(ChatLifecycleState value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasState() => $_has(4);
  @$pb.TagNumber(5)
  void clearState() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get deletionOperationId => $_getSZ(5);
  @$pb.TagNumber(6)
  set deletionOperationId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDeletionOperationId() => $_has(5);
  @$pb.TagNumber(6)
  void clearDeletionOperationId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.int get schemaVersion => $_getIZ(6);
  @$pb.TagNumber(7)
  set schemaVersion($core.int value) => $_setUnsignedInt32(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSchemaVersion() => $_has(6);
  @$pb.TagNumber(7)
  void clearSchemaVersion() => $_clearField(7);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
