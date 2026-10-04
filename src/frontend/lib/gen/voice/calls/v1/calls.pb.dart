// This is a generated file - do not edit.
//
// Generated from voice/calls/v1/calls.proto.

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
    as $2;

import '../../chat/v1/chat.pb.dart' as $1;
import '../../common/v1/space_lifecycle.pb.dart' as $4;
import '../../space/v1/space.pb.dart' as $3;
import 'calls.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'calls.pbenum.dart';

/// Opaque GIS resource identity. Voice never interprets external keys as user or owner IDs.
class GameSessionResourceRef extends $pb.GeneratedMessage {
  factory GameSessionResourceRef({
    GameSessionResourceKind? kind,
    $core.String? externalResourceKey,
  }) {
    final result = create();
    if (kind != null) result.kind = kind;
    if (externalResourceKey != null)
      result.externalResourceKey = externalResourceKey;
    return result;
  }

  GameSessionResourceRef._();

  factory GameSessionResourceRef.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GameSessionResourceRef.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GameSessionResourceRef',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aE<GameSessionResourceKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: GameSessionResourceKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'externalResourceKey')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameSessionResourceRef clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameSessionResourceRef copyWith(
          void Function(GameSessionResourceRef) updates) =>
      super.copyWith((message) => updates(message as GameSessionResourceRef))
          as GameSessionResourceRef;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GameSessionResourceRef create() => GameSessionResourceRef._();
  @$core.override
  GameSessionResourceRef createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GameSessionResourceRef getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GameSessionResourceRef>(create);
  static GameSessionResourceRef? _defaultInstance;

  @$pb.TagNumber(1)
  GameSessionResourceKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(GameSessionResourceKind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get externalResourceKey => $_getSZ(1);
  @$pb.TagNumber(2)
  set externalResourceKey($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExternalResourceKey() => $_has(1);
  @$pb.TagNumber(2)
  void clearExternalResourceKey() => $_clearField(2);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class ProvisionGameSessionRoomRequest extends $pb.GeneratedMessage {
  factory ProvisionGameSessionRoomRequest({
    $core.String? operationId,
    $core.String? applicationId,
    $core.String? environmentId,
    GameSessionResourceRef? resource,
    $core.String? chatId,
    $core.String? chatCreationOperationId,
    $core.String? sessionId,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (applicationId != null) result.applicationId = applicationId;
    if (environmentId != null) result.environmentId = environmentId;
    if (resource != null) result.resource = resource;
    if (chatId != null) result.chatId = chatId;
    if (chatCreationOperationId != null)
      result.chatCreationOperationId = chatCreationOperationId;
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  ProvisionGameSessionRoomRequest._();

  factory ProvisionGameSessionRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ProvisionGameSessionRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProvisionGameSessionRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'applicationId')
    ..aOS(3, _omitFieldNames ? '' : 'environmentId')
    ..aOM<GameSessionResourceRef>(4, _omitFieldNames ? '' : 'resource',
        subBuilder: GameSessionResourceRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'chatId')
    ..aOS(6, _omitFieldNames ? '' : 'chatCreationOperationId')
    ..aOS(7, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProvisionGameSessionRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProvisionGameSessionRoomRequest copyWith(
          void Function(ProvisionGameSessionRoomRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ProvisionGameSessionRoomRequest))
          as ProvisionGameSessionRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ProvisionGameSessionRoomRequest create() =>
      ProvisionGameSessionRoomRequest._();
  @$core.override
  ProvisionGameSessionRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ProvisionGameSessionRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProvisionGameSessionRoomRequest>(
          create);
  static ProvisionGameSessionRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get applicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set applicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApplicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearApplicationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get environmentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set environmentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEnvironmentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnvironmentId() => $_clearField(3);

  @$pb.TagNumber(4)
  GameSessionResourceRef get resource => $_getN(3);
  @$pb.TagNumber(4)
  set resource(GameSessionResourceRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasResource() => $_has(3);
  @$pb.TagNumber(4)
  void clearResource() => $_clearField(4);
  @$pb.TagNumber(4)
  GameSessionResourceRef ensureResource() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get chatId => $_getSZ(4);
  @$pb.TagNumber(5)
  set chatId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasChatId() => $_has(4);
  @$pb.TagNumber(5)
  void clearChatId() => $_clearField(5);

  /// Chat create receipt identity is its original operation_id.
  @$pb.TagNumber(6)
  $core.String get chatCreationOperationId => $_getSZ(5);
  @$pb.TagNumber(6)
  set chatCreationOperationId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasChatCreationOperationId() => $_has(5);
  @$pb.TagNumber(6)
  void clearChatCreationOperationId() => $_clearField(6);

  /// GIS session identity used by Role grants and Voice admission checks.
  @$pb.TagNumber(7)
  $core.String get sessionId => $_getSZ(6);
  @$pb.TagNumber(7)
  set sessionId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSessionId() => $_has(6);
  @$pb.TagNumber(7)
  void clearSessionId() => $_clearField(7);
}

/// Matchmaking owns the participant manifest. UUIDs are canonical lower-case,
/// unique, and ordered by UUID bytes; its digest covers concatenated fixed-width
/// 16-byte UUID values. Digest fields contain raw SHA-256 bytes.
/// operation_id is also the protected principal request-id binding. Hashes
/// cover deterministic serialization of the complete typed request.
/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class CreateMatchSquadRoomRequest extends $pb.GeneratedMessage {
  factory CreateMatchSquadRoomRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? matchId,
    $core.Iterable<$1.MatchSquadParticipant>? participants,
    $core.List<$core.int>? participantManifestSha256,
    $1.MatchSquadChatReceipt? chatCreationReceipt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (matchId != null) result.matchId = matchId;
    if (participants != null) result.participants.addAll(participants);
    if (participantManifestSha256 != null)
      result.participantManifestSha256 = participantManifestSha256;
    if (chatCreationReceipt != null)
      result.chatCreationReceipt = chatCreationReceipt;
    return result;
  }

  CreateMatchSquadRoomRequest._();

  factory CreateMatchSquadRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateMatchSquadRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateMatchSquadRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'matchId')
    ..pPM<$1.MatchSquadParticipant>(4, _omitFieldNames ? '' : 'participants',
        subBuilder: $1.MatchSquadParticipant.create)
    ..a<$core.List<$core.int>>(5,
        _omitFieldNames ? '' : 'participantManifestSha256', $pb.PbFieldType.OY)
    ..aOM<$1.MatchSquadChatReceipt>(
        6, _omitFieldNames ? '' : 'chatCreationReceipt',
        subBuilder: $1.MatchSquadChatReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateMatchSquadRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateMatchSquadRoomRequest copyWith(
          void Function(CreateMatchSquadRoomRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CreateMatchSquadRoomRequest))
          as CreateMatchSquadRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateMatchSquadRoomRequest create() =>
      CreateMatchSquadRoomRequest._();
  @$core.override
  CreateMatchSquadRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateMatchSquadRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateMatchSquadRoomRequest>(create);
  static CreateMatchSquadRoomRequest? _defaultInstance;

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
  $core.String get matchId => $_getSZ(2);
  @$pb.TagNumber(3)
  set matchId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMatchId() => $_has(2);
  @$pb.TagNumber(3)
  void clearMatchId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$1.MatchSquadParticipant> get participants => $_getList(3);

  @$pb.TagNumber(5)
  $core.List<$core.int> get participantManifestSha256 => $_getN(4);
  @$pb.TagNumber(5)
  set participantManifestSha256($core.List<$core.int> value) =>
      $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasParticipantManifestSha256() => $_has(4);
  @$pb.TagNumber(5)
  void clearParticipantManifestSha256() => $_clearField(5);

  @$pb.TagNumber(6)
  $1.MatchSquadChatReceipt get chatCreationReceipt => $_getN(5);
  @$pb.TagNumber(6)
  set chatCreationReceipt($1.MatchSquadChatReceipt value) =>
      $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasChatCreationReceipt() => $_has(5);
  @$pb.TagNumber(6)
  void clearChatCreationReceipt() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.MatchSquadChatReceipt ensureChatCreationReceipt() => $_ensure(5);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class MatchSquadRoomReceipt extends $pb.GeneratedMessage {
  factory MatchSquadRoomReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? operationId,
    $core.String? matchId,
    $core.String? roomId,
    $core.String? chatId,
    $core.String? chatCreationReceiptId,
    $core.List<$core.int>? chatCreationReceiptSha256,
    $core.List<$core.int>? participantManifestSha256,
    $core.List<$core.int>? requestSha256,
    $2.Timestamp? createdAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (operationId != null) result.operationId = operationId;
    if (matchId != null) result.matchId = matchId;
    if (roomId != null) result.roomId = roomId;
    if (chatId != null) result.chatId = chatId;
    if (chatCreationReceiptId != null)
      result.chatCreationReceiptId = chatCreationReceiptId;
    if (chatCreationReceiptSha256 != null)
      result.chatCreationReceiptSha256 = chatCreationReceiptSha256;
    if (participantManifestSha256 != null)
      result.participantManifestSha256 = participantManifestSha256;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  MatchSquadRoomReceipt._();

  factory MatchSquadRoomReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchSquadRoomReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchSquadRoomReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..aOS(4, _omitFieldNames ? '' : 'matchId')
    ..aOS(5, _omitFieldNames ? '' : 'roomId')
    ..aOS(6, _omitFieldNames ? '' : 'chatId')
    ..aOS(7, _omitFieldNames ? '' : 'chatCreationReceiptId')
    ..a<$core.List<$core.int>>(8,
        _omitFieldNames ? '' : 'chatCreationReceiptSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(9,
        _omitFieldNames ? '' : 'participantManifestSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        10, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$2.Timestamp>(11, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchSquadRoomReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchSquadRoomReceipt copyWith(
          void Function(MatchSquadRoomReceipt) updates) =>
      super.copyWith((message) => updates(message as MatchSquadRoomReceipt))
          as MatchSquadRoomReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchSquadRoomReceipt create() => MatchSquadRoomReceipt._();
  @$core.override
  MatchSquadRoomReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchSquadRoomReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchSquadRoomReceipt>(create);
  static MatchSquadRoomReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get matchId => $_getSZ(3);
  @$pb.TagNumber(4)
  set matchId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMatchId() => $_has(3);
  @$pb.TagNumber(4)
  void clearMatchId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get roomId => $_getSZ(4);
  @$pb.TagNumber(5)
  set roomId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRoomId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoomId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get chatId => $_getSZ(5);
  @$pb.TagNumber(6)
  set chatId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasChatId() => $_has(5);
  @$pb.TagNumber(6)
  void clearChatId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get chatCreationReceiptId => $_getSZ(6);
  @$pb.TagNumber(7)
  set chatCreationReceiptId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasChatCreationReceiptId() => $_has(6);
  @$pb.TagNumber(7)
  void clearChatCreationReceiptId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get chatCreationReceiptSha256 => $_getN(7);
  @$pb.TagNumber(8)
  set chatCreationReceiptSha256($core.List<$core.int> value) =>
      $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasChatCreationReceiptSha256() => $_has(7);
  @$pb.TagNumber(8)
  void clearChatCreationReceiptSha256() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.List<$core.int> get participantManifestSha256 => $_getN(8);
  @$pb.TagNumber(9)
  set participantManifestSha256($core.List<$core.int> value) =>
      $_setBytes(8, value);
  @$pb.TagNumber(9)
  $core.bool hasParticipantManifestSha256() => $_has(8);
  @$pb.TagNumber(9)
  void clearParticipantManifestSha256() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.List<$core.int> get requestSha256 => $_getN(9);
  @$pb.TagNumber(10)
  set requestSha256($core.List<$core.int> value) => $_setBytes(9, value);
  @$pb.TagNumber(10)
  $core.bool hasRequestSha256() => $_has(9);
  @$pb.TagNumber(10)
  void clearRequestSha256() => $_clearField(10);

  @$pb.TagNumber(11)
  $2.Timestamp get createdAt => $_getN(10);
  @$pb.TagNumber(11)
  set createdAt($2.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasCreatedAt() => $_has(10);
  @$pb.TagNumber(11)
  void clearCreatedAt() => $_clearField(11);
  @$pb.TagNumber(11)
  $2.Timestamp ensureCreatedAt() => $_ensure(10);
}

class CreateMatchSquadRoomResponse extends $pb.GeneratedMessage {
  factory CreateMatchSquadRoomResponse({
    MatchSquadRoomReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  CreateMatchSquadRoomResponse._();

  factory CreateMatchSquadRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateMatchSquadRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateMatchSquadRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<MatchSquadRoomReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: MatchSquadRoomReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateMatchSquadRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateMatchSquadRoomResponse copyWith(
          void Function(CreateMatchSquadRoomResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CreateMatchSquadRoomResponse))
          as CreateMatchSquadRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateMatchSquadRoomResponse create() =>
      CreateMatchSquadRoomResponse._();
  @$core.override
  CreateMatchSquadRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateMatchSquadRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateMatchSquadRoomResponse>(create);
  static CreateMatchSquadRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MatchSquadRoomReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(MatchSquadRoomReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  MatchSquadRoomReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class TeardownMatchSquadRoomRequest extends $pb.GeneratedMessage {
  factory TeardownMatchSquadRoomRequest({
    $core.int? protocolVersion,
    $core.String? teardownOperationId,
    $core.String? matchId,
    $core.String? roomId,
    $core.String? creationReceiptId,
    $core.List<$core.int>? participantManifestSha256,
    $core.List<$core.int>? creationRequestSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (teardownOperationId != null)
      result.teardownOperationId = teardownOperationId;
    if (matchId != null) result.matchId = matchId;
    if (roomId != null) result.roomId = roomId;
    if (creationReceiptId != null) result.creationReceiptId = creationReceiptId;
    if (participantManifestSha256 != null)
      result.participantManifestSha256 = participantManifestSha256;
    if (creationRequestSha256 != null)
      result.creationRequestSha256 = creationRequestSha256;
    return result;
  }

  TeardownMatchSquadRoomRequest._();

  factory TeardownMatchSquadRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TeardownMatchSquadRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TeardownMatchSquadRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'teardownOperationId')
    ..aOS(3, _omitFieldNames ? '' : 'matchId')
    ..aOS(4, _omitFieldNames ? '' : 'roomId')
    ..aOS(5, _omitFieldNames ? '' : 'creationReceiptId')
    ..a<$core.List<$core.int>>(6,
        _omitFieldNames ? '' : 'participantManifestSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'creationRequestSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TeardownMatchSquadRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TeardownMatchSquadRoomRequest copyWith(
          void Function(TeardownMatchSquadRoomRequest) updates) =>
      super.copyWith(
              (message) => updates(message as TeardownMatchSquadRoomRequest))
          as TeardownMatchSquadRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TeardownMatchSquadRoomRequest create() =>
      TeardownMatchSquadRoomRequest._();
  @$core.override
  TeardownMatchSquadRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TeardownMatchSquadRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TeardownMatchSquadRoomRequest>(create);
  static TeardownMatchSquadRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get teardownOperationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set teardownOperationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTeardownOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearTeardownOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get matchId => $_getSZ(2);
  @$pb.TagNumber(3)
  set matchId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMatchId() => $_has(2);
  @$pb.TagNumber(3)
  void clearMatchId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get roomId => $_getSZ(3);
  @$pb.TagNumber(4)
  set roomId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRoomId() => $_has(3);
  @$pb.TagNumber(4)
  void clearRoomId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get creationReceiptId => $_getSZ(4);
  @$pb.TagNumber(5)
  set creationReceiptId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCreationReceiptId() => $_has(4);
  @$pb.TagNumber(5)
  void clearCreationReceiptId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get participantManifestSha256 => $_getN(5);
  @$pb.TagNumber(6)
  set participantManifestSha256($core.List<$core.int> value) =>
      $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasParticipantManifestSha256() => $_has(5);
  @$pb.TagNumber(6)
  void clearParticipantManifestSha256() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get creationRequestSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set creationRequestSha256($core.List<$core.int> value) =>
      $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasCreationRequestSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearCreationRequestSha256() => $_clearField(7);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class MatchSquadRoomTeardownReceipt extends $pb.GeneratedMessage {
  factory MatchSquadRoomTeardownReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? teardownOperationId,
    $core.String? matchId,
    $core.String? roomId,
    $core.String? creationReceiptId,
    $core.List<$core.int>? participantManifestSha256,
    $core.List<$core.int>? requestSha256,
    MatchSquadTeardownStatus? status,
    $2.Timestamp? completedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (teardownOperationId != null)
      result.teardownOperationId = teardownOperationId;
    if (matchId != null) result.matchId = matchId;
    if (roomId != null) result.roomId = roomId;
    if (creationReceiptId != null) result.creationReceiptId = creationReceiptId;
    if (participantManifestSha256 != null)
      result.participantManifestSha256 = participantManifestSha256;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (status != null) result.status = status;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  MatchSquadRoomTeardownReceipt._();

  factory MatchSquadRoomTeardownReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchSquadRoomTeardownReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchSquadRoomTeardownReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'teardownOperationId')
    ..aOS(4, _omitFieldNames ? '' : 'matchId')
    ..aOS(5, _omitFieldNames ? '' : 'roomId')
    ..aOS(6, _omitFieldNames ? '' : 'creationReceiptId')
    ..a<$core.List<$core.int>>(7,
        _omitFieldNames ? '' : 'participantManifestSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aE<MatchSquadTeardownStatus>(9, _omitFieldNames ? '' : 'status',
        enumValues: MatchSquadTeardownStatus.values)
    ..aOM<$2.Timestamp>(10, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchSquadRoomTeardownReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchSquadRoomTeardownReceipt copyWith(
          void Function(MatchSquadRoomTeardownReceipt) updates) =>
      super.copyWith(
              (message) => updates(message as MatchSquadRoomTeardownReceipt))
          as MatchSquadRoomTeardownReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchSquadRoomTeardownReceipt create() =>
      MatchSquadRoomTeardownReceipt._();
  @$core.override
  MatchSquadRoomTeardownReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchSquadRoomTeardownReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchSquadRoomTeardownReceipt>(create);
  static MatchSquadRoomTeardownReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get teardownOperationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set teardownOperationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTeardownOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearTeardownOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get matchId => $_getSZ(3);
  @$pb.TagNumber(4)
  set matchId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMatchId() => $_has(3);
  @$pb.TagNumber(4)
  void clearMatchId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get roomId => $_getSZ(4);
  @$pb.TagNumber(5)
  set roomId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRoomId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoomId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get creationReceiptId => $_getSZ(5);
  @$pb.TagNumber(6)
  set creationReceiptId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCreationReceiptId() => $_has(5);
  @$pb.TagNumber(6)
  void clearCreationReceiptId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get participantManifestSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set participantManifestSha256($core.List<$core.int> value) =>
      $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasParticipantManifestSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearParticipantManifestSha256() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get requestSha256 => $_getN(7);
  @$pb.TagNumber(8)
  set requestSha256($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasRequestSha256() => $_has(7);
  @$pb.TagNumber(8)
  void clearRequestSha256() => $_clearField(8);

  @$pb.TagNumber(9)
  MatchSquadTeardownStatus get status => $_getN(8);
  @$pb.TagNumber(9)
  set status(MatchSquadTeardownStatus value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasStatus() => $_has(8);
  @$pb.TagNumber(9)
  void clearStatus() => $_clearField(9);

  @$pb.TagNumber(10)
  $2.Timestamp get completedAt => $_getN(9);
  @$pb.TagNumber(10)
  set completedAt($2.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCompletedAt() => $_has(9);
  @$pb.TagNumber(10)
  void clearCompletedAt() => $_clearField(10);
  @$pb.TagNumber(10)
  $2.Timestamp ensureCompletedAt() => $_ensure(9);
}

class TeardownMatchSquadRoomResponse extends $pb.GeneratedMessage {
  factory TeardownMatchSquadRoomResponse({
    MatchSquadRoomTeardownReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  TeardownMatchSquadRoomResponse._();

  factory TeardownMatchSquadRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TeardownMatchSquadRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TeardownMatchSquadRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<MatchSquadRoomTeardownReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: MatchSquadRoomTeardownReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TeardownMatchSquadRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TeardownMatchSquadRoomResponse copyWith(
          void Function(TeardownMatchSquadRoomResponse) updates) =>
      super.copyWith(
              (message) => updates(message as TeardownMatchSquadRoomResponse))
          as TeardownMatchSquadRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TeardownMatchSquadRoomResponse create() =>
      TeardownMatchSquadRoomResponse._();
  @$core.override
  TeardownMatchSquadRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TeardownMatchSquadRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TeardownMatchSquadRoomResponse>(create);
  static TeardownMatchSquadRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MatchSquadRoomTeardownReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(MatchSquadRoomTeardownReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  MatchSquadRoomTeardownReceipt ensureReceipt() => $_ensure(0);
}

/// Matchmaking sends this only after its durable teardown aggregate has been
/// complete for at least 30 days. The stored teardown receipt hash binds the
/// exact provider evidence whose large request and receipt bytes may be cleared.
/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class CompactMatchSquadRoomRequest extends $pb.GeneratedMessage {
  factory CompactMatchSquadRoomRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? teardownAggregateId,
    $core.String? matchId,
    $core.String? roomId,
    $core.String? creationReceiptId,
    $core.List<$core.int>? creationRequestSha256,
    $core.String? teardownOperationId,
    $core.String? teardownReceiptId,
    $core.List<$core.int>? teardownReceiptSha256,
    $core.List<$core.int>? participantManifestSha256,
    $2.Timestamp? aggregateCompletedAt,
    $2.Timestamp? compactionAuthorizedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (teardownAggregateId != null)
      result.teardownAggregateId = teardownAggregateId;
    if (matchId != null) result.matchId = matchId;
    if (roomId != null) result.roomId = roomId;
    if (creationReceiptId != null) result.creationReceiptId = creationReceiptId;
    if (creationRequestSha256 != null)
      result.creationRequestSha256 = creationRequestSha256;
    if (teardownOperationId != null)
      result.teardownOperationId = teardownOperationId;
    if (teardownReceiptId != null) result.teardownReceiptId = teardownReceiptId;
    if (teardownReceiptSha256 != null)
      result.teardownReceiptSha256 = teardownReceiptSha256;
    if (participantManifestSha256 != null)
      result.participantManifestSha256 = participantManifestSha256;
    if (aggregateCompletedAt != null)
      result.aggregateCompletedAt = aggregateCompletedAt;
    if (compactionAuthorizedAt != null)
      result.compactionAuthorizedAt = compactionAuthorizedAt;
    return result;
  }

  CompactMatchSquadRoomRequest._();

  factory CompactMatchSquadRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompactMatchSquadRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompactMatchSquadRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'teardownAggregateId')
    ..aOS(4, _omitFieldNames ? '' : 'matchId')
    ..aOS(5, _omitFieldNames ? '' : 'roomId')
    ..aOS(6, _omitFieldNames ? '' : 'creationReceiptId')
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'creationRequestSha256', $pb.PbFieldType.OY)
    ..aOS(8, _omitFieldNames ? '' : 'teardownOperationId')
    ..aOS(9, _omitFieldNames ? '' : 'teardownReceiptId')
    ..a<$core.List<$core.int>>(
        10, _omitFieldNames ? '' : 'teardownReceiptSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(11,
        _omitFieldNames ? '' : 'participantManifestSha256', $pb.PbFieldType.OY)
    ..aOM<$2.Timestamp>(12, _omitFieldNames ? '' : 'aggregateCompletedAt',
        subBuilder: $2.Timestamp.create)
    ..aOM<$2.Timestamp>(13, _omitFieldNames ? '' : 'compactionAuthorizedAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompactMatchSquadRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompactMatchSquadRoomRequest copyWith(
          void Function(CompactMatchSquadRoomRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CompactMatchSquadRoomRequest))
          as CompactMatchSquadRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompactMatchSquadRoomRequest create() =>
      CompactMatchSquadRoomRequest._();
  @$core.override
  CompactMatchSquadRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompactMatchSquadRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompactMatchSquadRoomRequest>(create);
  static CompactMatchSquadRoomRequest? _defaultInstance;

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
  $core.String get teardownAggregateId => $_getSZ(2);
  @$pb.TagNumber(3)
  set teardownAggregateId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTeardownAggregateId() => $_has(2);
  @$pb.TagNumber(3)
  void clearTeardownAggregateId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get matchId => $_getSZ(3);
  @$pb.TagNumber(4)
  set matchId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMatchId() => $_has(3);
  @$pb.TagNumber(4)
  void clearMatchId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get roomId => $_getSZ(4);
  @$pb.TagNumber(5)
  set roomId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRoomId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoomId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get creationReceiptId => $_getSZ(5);
  @$pb.TagNumber(6)
  set creationReceiptId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCreationReceiptId() => $_has(5);
  @$pb.TagNumber(6)
  void clearCreationReceiptId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get creationRequestSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set creationRequestSha256($core.List<$core.int> value) =>
      $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasCreationRequestSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearCreationRequestSha256() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get teardownOperationId => $_getSZ(7);
  @$pb.TagNumber(8)
  set teardownOperationId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasTeardownOperationId() => $_has(7);
  @$pb.TagNumber(8)
  void clearTeardownOperationId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get teardownReceiptId => $_getSZ(8);
  @$pb.TagNumber(9)
  set teardownReceiptId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasTeardownReceiptId() => $_has(8);
  @$pb.TagNumber(9)
  void clearTeardownReceiptId() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.List<$core.int> get teardownReceiptSha256 => $_getN(9);
  @$pb.TagNumber(10)
  set teardownReceiptSha256($core.List<$core.int> value) =>
      $_setBytes(9, value);
  @$pb.TagNumber(10)
  $core.bool hasTeardownReceiptSha256() => $_has(9);
  @$pb.TagNumber(10)
  void clearTeardownReceiptSha256() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.List<$core.int> get participantManifestSha256 => $_getN(10);
  @$pb.TagNumber(11)
  set participantManifestSha256($core.List<$core.int> value) =>
      $_setBytes(10, value);
  @$pb.TagNumber(11)
  $core.bool hasParticipantManifestSha256() => $_has(10);
  @$pb.TagNumber(11)
  void clearParticipantManifestSha256() => $_clearField(11);

  @$pb.TagNumber(12)
  $2.Timestamp get aggregateCompletedAt => $_getN(11);
  @$pb.TagNumber(12)
  set aggregateCompletedAt($2.Timestamp value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasAggregateCompletedAt() => $_has(11);
  @$pb.TagNumber(12)
  void clearAggregateCompletedAt() => $_clearField(12);
  @$pb.TagNumber(12)
  $2.Timestamp ensureAggregateCompletedAt() => $_ensure(11);

  @$pb.TagNumber(13)
  $2.Timestamp get compactionAuthorizedAt => $_getN(12);
  @$pb.TagNumber(13)
  set compactionAuthorizedAt($2.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasCompactionAuthorizedAt() => $_has(12);
  @$pb.TagNumber(13)
  void clearCompactionAuthorizedAt() => $_clearField(13);
  @$pb.TagNumber(13)
  $2.Timestamp ensureCompactionAuthorizedAt() => $_ensure(12);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class MatchSquadRoomCompactionReceipt extends $pb.GeneratedMessage {
  factory MatchSquadRoomCompactionReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? compactionOperationId,
    $core.String? teardownAggregateId,
    $core.String? matchId,
    $core.String? roomId,
    $core.List<$core.int>? requestSha256,
    $2.Timestamp? aggregateCompletedAt,
    $2.Timestamp? compactionAuthorizedAt,
    MatchSquadRoomCompactionStatus? status,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (compactionOperationId != null)
      result.compactionOperationId = compactionOperationId;
    if (teardownAggregateId != null)
      result.teardownAggregateId = teardownAggregateId;
    if (matchId != null) result.matchId = matchId;
    if (roomId != null) result.roomId = roomId;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (aggregateCompletedAt != null)
      result.aggregateCompletedAt = aggregateCompletedAt;
    if (compactionAuthorizedAt != null)
      result.compactionAuthorizedAt = compactionAuthorizedAt;
    if (status != null) result.status = status;
    return result;
  }

  MatchSquadRoomCompactionReceipt._();

  factory MatchSquadRoomCompactionReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchSquadRoomCompactionReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchSquadRoomCompactionReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'compactionOperationId')
    ..aOS(4, _omitFieldNames ? '' : 'teardownAggregateId')
    ..aOS(5, _omitFieldNames ? '' : 'matchId')
    ..aOS(6, _omitFieldNames ? '' : 'roomId')
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$2.Timestamp>(8, _omitFieldNames ? '' : 'aggregateCompletedAt',
        subBuilder: $2.Timestamp.create)
    ..aOM<$2.Timestamp>(9, _omitFieldNames ? '' : 'compactionAuthorizedAt',
        subBuilder: $2.Timestamp.create)
    ..aE<MatchSquadRoomCompactionStatus>(10, _omitFieldNames ? '' : 'status',
        enumValues: MatchSquadRoomCompactionStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchSquadRoomCompactionReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchSquadRoomCompactionReceipt copyWith(
          void Function(MatchSquadRoomCompactionReceipt) updates) =>
      super.copyWith(
              (message) => updates(message as MatchSquadRoomCompactionReceipt))
          as MatchSquadRoomCompactionReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchSquadRoomCompactionReceipt create() =>
      MatchSquadRoomCompactionReceipt._();
  @$core.override
  MatchSquadRoomCompactionReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchSquadRoomCompactionReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchSquadRoomCompactionReceipt>(
          create);
  static MatchSquadRoomCompactionReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get compactionOperationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set compactionOperationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCompactionOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCompactionOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get teardownAggregateId => $_getSZ(3);
  @$pb.TagNumber(4)
  set teardownAggregateId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTeardownAggregateId() => $_has(3);
  @$pb.TagNumber(4)
  void clearTeardownAggregateId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get matchId => $_getSZ(4);
  @$pb.TagNumber(5)
  set matchId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMatchId() => $_has(4);
  @$pb.TagNumber(5)
  void clearMatchId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get roomId => $_getSZ(5);
  @$pb.TagNumber(6)
  set roomId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRoomId() => $_has(5);
  @$pb.TagNumber(6)
  void clearRoomId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get requestSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set requestSha256($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRequestSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearRequestSha256() => $_clearField(7);

  @$pb.TagNumber(8)
  $2.Timestamp get aggregateCompletedAt => $_getN(7);
  @$pb.TagNumber(8)
  set aggregateCompletedAt($2.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasAggregateCompletedAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearAggregateCompletedAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $2.Timestamp ensureAggregateCompletedAt() => $_ensure(7);

  @$pb.TagNumber(9)
  $2.Timestamp get compactionAuthorizedAt => $_getN(8);
  @$pb.TagNumber(9)
  set compactionAuthorizedAt($2.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCompactionAuthorizedAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearCompactionAuthorizedAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $2.Timestamp ensureCompactionAuthorizedAt() => $_ensure(8);

  @$pb.TagNumber(10)
  MatchSquadRoomCompactionStatus get status => $_getN(9);
  @$pb.TagNumber(10)
  set status(MatchSquadRoomCompactionStatus value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasStatus() => $_has(9);
  @$pb.TagNumber(10)
  void clearStatus() => $_clearField(10);
}

class CompactMatchSquadRoomResponse extends $pb.GeneratedMessage {
  factory CompactMatchSquadRoomResponse({
    MatchSquadRoomCompactionReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  CompactMatchSquadRoomResponse._();

  factory CompactMatchSquadRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompactMatchSquadRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompactMatchSquadRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<MatchSquadRoomCompactionReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: MatchSquadRoomCompactionReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompactMatchSquadRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompactMatchSquadRoomResponse copyWith(
          void Function(CompactMatchSquadRoomResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CompactMatchSquadRoomResponse))
          as CompactMatchSquadRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompactMatchSquadRoomResponse create() =>
      CompactMatchSquadRoomResponse._();
  @$core.override
  CompactMatchSquadRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompactMatchSquadRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompactMatchSquadRoomResponse>(create);
  static CompactMatchSquadRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MatchSquadRoomCompactionReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(MatchSquadRoomCompactionReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  MatchSquadRoomCompactionReceipt ensureReceipt() => $_ensure(0);
}

/// Match ID is caller-supplied request data and must match the immutable
/// Voice-owned room binding. Actor identity is derived from the verified
/// delegated-user principal, never from request fields or forwarded headers.
/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class JoinMatchSquadRoomRequest extends $pb.GeneratedMessage {
  factory JoinMatchSquadRoomRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? matchId,
    $core.String? roomId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (matchId != null) result.matchId = matchId;
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  JoinMatchSquadRoomRequest._();

  factory JoinMatchSquadRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinMatchSquadRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinMatchSquadRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'matchId')
    ..aOS(4, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinMatchSquadRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinMatchSquadRoomRequest copyWith(
          void Function(JoinMatchSquadRoomRequest) updates) =>
      super.copyWith((message) => updates(message as JoinMatchSquadRoomRequest))
          as JoinMatchSquadRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinMatchSquadRoomRequest create() => JoinMatchSquadRoomRequest._();
  @$core.override
  JoinMatchSquadRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinMatchSquadRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinMatchSquadRoomRequest>(create);
  static JoinMatchSquadRoomRequest? _defaultInstance;

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
  $core.String get matchId => $_getSZ(2);
  @$pb.TagNumber(3)
  set matchId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMatchId() => $_has(2);
  @$pb.TagNumber(3)
  void clearMatchId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get roomId => $_getSZ(3);
  @$pb.TagNumber(4)
  set roomId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRoomId() => $_has(3);
  @$pb.TagNumber(4)
  void clearRoomId() => $_clearField(4);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class JoinMatchSquadRoomResponse extends $pb.GeneratedMessage {
  factory JoinMatchSquadRoomResponse({
    CallSession? callSession,
    $core.String? mediaEpoch,
    MatchSquadMembershipState? membershipState,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    if (mediaEpoch != null) result.mediaEpoch = mediaEpoch;
    if (membershipState != null) result.membershipState = membershipState;
    return result;
  }

  JoinMatchSquadRoomResponse._();

  factory JoinMatchSquadRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinMatchSquadRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinMatchSquadRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..aOS(2, _omitFieldNames ? '' : 'mediaEpoch')
    ..aE<MatchSquadMembershipState>(3, _omitFieldNames ? '' : 'membershipState',
        enumValues: MatchSquadMembershipState.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinMatchSquadRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinMatchSquadRoomResponse copyWith(
          void Function(JoinMatchSquadRoomResponse) updates) =>
      super.copyWith(
              (message) => updates(message as JoinMatchSquadRoomResponse))
          as JoinMatchSquadRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinMatchSquadRoomResponse create() => JoinMatchSquadRoomResponse._();
  @$core.override
  JoinMatchSquadRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinMatchSquadRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinMatchSquadRoomResponse>(create);
  static JoinMatchSquadRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get mediaEpoch => $_getSZ(1);
  @$pb.TagNumber(2)
  set mediaEpoch($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMediaEpoch() => $_has(1);
  @$pb.TagNumber(2)
  void clearMediaEpoch() => $_clearField(2);

  @$pb.TagNumber(3)
  MatchSquadMembershipState get membershipState => $_getN(2);
  @$pb.TagNumber(3)
  set membershipState(MatchSquadMembershipState value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasMembershipState() => $_has(2);
  @$pb.TagNumber(3)
  void clearMembershipState() => $_clearField(3);
}

/// Token issuance is an authenticated read of an already-current membership;
/// it never joins, resumes, or recreates a room.
/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class GetMatchSquadJoinTokenRequest extends $pb.GeneratedMessage {
  factory GetMatchSquadJoinTokenRequest({
    $core.int? protocolVersion,
    $core.String? matchId,
    $core.String? roomId,
    $core.String? mediaEpoch,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (matchId != null) result.matchId = matchId;
    if (roomId != null) result.roomId = roomId;
    if (mediaEpoch != null) result.mediaEpoch = mediaEpoch;
    return result;
  }

  GetMatchSquadJoinTokenRequest._();

  factory GetMatchSquadJoinTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMatchSquadJoinTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMatchSquadJoinTokenRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'matchId')
    ..aOS(3, _omitFieldNames ? '' : 'roomId')
    ..aOS(4, _omitFieldNames ? '' : 'mediaEpoch')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchSquadJoinTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchSquadJoinTokenRequest copyWith(
          void Function(GetMatchSquadJoinTokenRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetMatchSquadJoinTokenRequest))
          as GetMatchSquadJoinTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMatchSquadJoinTokenRequest create() =>
      GetMatchSquadJoinTokenRequest._();
  @$core.override
  GetMatchSquadJoinTokenRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMatchSquadJoinTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMatchSquadJoinTokenRequest>(create);
  static GetMatchSquadJoinTokenRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get matchId => $_getSZ(1);
  @$pb.TagNumber(2)
  set matchId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMatchId() => $_has(1);
  @$pb.TagNumber(2)
  void clearMatchId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get roomId => $_getSZ(2);
  @$pb.TagNumber(3)
  set roomId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRoomId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRoomId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get mediaEpoch => $_getSZ(3);
  @$pb.TagNumber(4)
  set mediaEpoch($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMediaEpoch() => $_has(3);
  @$pb.TagNumber(4)
  void clearMediaEpoch() => $_clearField(4);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class GetMatchSquadJoinTokenResponse extends $pb.GeneratedMessage {
  factory GetMatchSquadJoinTokenResponse({
    GetJoinTokenResponse? token,
    $core.String? mediaEpoch,
  }) {
    final result = create();
    if (token != null) result.token = token;
    if (mediaEpoch != null) result.mediaEpoch = mediaEpoch;
    return result;
  }

  GetMatchSquadJoinTokenResponse._();

  factory GetMatchSquadJoinTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMatchSquadJoinTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMatchSquadJoinTokenResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<GetJoinTokenResponse>(1, _omitFieldNames ? '' : 'token',
        subBuilder: GetJoinTokenResponse.create)
    ..aOS(2, _omitFieldNames ? '' : 'mediaEpoch')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchSquadJoinTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchSquadJoinTokenResponse copyWith(
          void Function(GetMatchSquadJoinTokenResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetMatchSquadJoinTokenResponse))
          as GetMatchSquadJoinTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMatchSquadJoinTokenResponse create() =>
      GetMatchSquadJoinTokenResponse._();
  @$core.override
  GetMatchSquadJoinTokenResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMatchSquadJoinTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMatchSquadJoinTokenResponse>(create);
  static GetMatchSquadJoinTokenResponse? _defaultInstance;

  @$pb.TagNumber(1)
  GetJoinTokenResponse get token => $_getN(0);
  @$pb.TagNumber(1)
  set token(GetJoinTokenResponse value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearToken() => $_clearField(1);
  @$pb.TagNumber(1)
  GetJoinTokenResponse ensureToken() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get mediaEpoch => $_getSZ(1);
  @$pb.TagNumber(2)
  set mediaEpoch($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMediaEpoch() => $_has(1);
  @$pb.TagNumber(2)
  void clearMediaEpoch() => $_clearField(2);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class LeaveMatchSquadRoomRequest extends $pb.GeneratedMessage {
  factory LeaveMatchSquadRoomRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? matchId,
    $core.String? roomId,
    $core.String? expectedMediaEpoch,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (matchId != null) result.matchId = matchId;
    if (roomId != null) result.roomId = roomId;
    if (expectedMediaEpoch != null)
      result.expectedMediaEpoch = expectedMediaEpoch;
    return result;
  }

  LeaveMatchSquadRoomRequest._();

  factory LeaveMatchSquadRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveMatchSquadRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveMatchSquadRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'matchId')
    ..aOS(4, _omitFieldNames ? '' : 'roomId')
    ..aOS(5, _omitFieldNames ? '' : 'expectedMediaEpoch')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveMatchSquadRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveMatchSquadRoomRequest copyWith(
          void Function(LeaveMatchSquadRoomRequest) updates) =>
      super.copyWith(
              (message) => updates(message as LeaveMatchSquadRoomRequest))
          as LeaveMatchSquadRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveMatchSquadRoomRequest create() => LeaveMatchSquadRoomRequest._();
  @$core.override
  LeaveMatchSquadRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveMatchSquadRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveMatchSquadRoomRequest>(create);
  static LeaveMatchSquadRoomRequest? _defaultInstance;

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
  $core.String get matchId => $_getSZ(2);
  @$pb.TagNumber(3)
  set matchId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMatchId() => $_has(2);
  @$pb.TagNumber(3)
  void clearMatchId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get roomId => $_getSZ(3);
  @$pb.TagNumber(4)
  set roomId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRoomId() => $_has(3);
  @$pb.TagNumber(4)
  void clearRoomId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get expectedMediaEpoch => $_getSZ(4);
  @$pb.TagNumber(5)
  set expectedMediaEpoch($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpectedMediaEpoch() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpectedMediaEpoch() => $_clearField(5);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class LeaveMatchSquadRoomResponse extends $pb.GeneratedMessage {
  factory LeaveMatchSquadRoomResponse({
    CallSession? callSession,
    $core.String? mediaEpoch,
    MatchSquadMembershipState? membershipState,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    if (mediaEpoch != null) result.mediaEpoch = mediaEpoch;
    if (membershipState != null) result.membershipState = membershipState;
    return result;
  }

  LeaveMatchSquadRoomResponse._();

  factory LeaveMatchSquadRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveMatchSquadRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveMatchSquadRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..aOS(2, _omitFieldNames ? '' : 'mediaEpoch')
    ..aE<MatchSquadMembershipState>(3, _omitFieldNames ? '' : 'membershipState',
        enumValues: MatchSquadMembershipState.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveMatchSquadRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveMatchSquadRoomResponse copyWith(
          void Function(LeaveMatchSquadRoomResponse) updates) =>
      super.copyWith(
              (message) => updates(message as LeaveMatchSquadRoomResponse))
          as LeaveMatchSquadRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveMatchSquadRoomResponse create() =>
      LeaveMatchSquadRoomResponse._();
  @$core.override
  LeaveMatchSquadRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveMatchSquadRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveMatchSquadRoomResponse>(create);
  static LeaveMatchSquadRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get mediaEpoch => $_getSZ(1);
  @$pb.TagNumber(2)
  set mediaEpoch($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMediaEpoch() => $_has(1);
  @$pb.TagNumber(2)
  void clearMediaEpoch() => $_clearField(2);

  @$pb.TagNumber(3)
  MatchSquadMembershipState get membershipState => $_getN(2);
  @$pb.TagNumber(3)
  set membershipState(MatchSquadMembershipState value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasMembershipState() => $_has(2);
  @$pb.TagNumber(3)
  void clearMembershipState() => $_clearField(3);
}

class ProvisionGameSessionRoomResponse extends $pb.GeneratedMessage {
  factory ProvisionGameSessionRoomResponse({
    $core.String? operationId,
    $core.String? applicationId,
    $core.String? environmentId,
    GameSessionResourceRef? resource,
    $core.String? chatId,
    $core.String? chatCreationOperationId,
    $core.String? roomId,
    $core.String? livekitRoomName,
    $core.String? voiceCreationReceiptId,
    $core.List<$core.int>? requestHash,
    $2.Timestamp? createdAt,
    $core.String? sessionId,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (applicationId != null) result.applicationId = applicationId;
    if (environmentId != null) result.environmentId = environmentId;
    if (resource != null) result.resource = resource;
    if (chatId != null) result.chatId = chatId;
    if (chatCreationOperationId != null)
      result.chatCreationOperationId = chatCreationOperationId;
    if (roomId != null) result.roomId = roomId;
    if (livekitRoomName != null) result.livekitRoomName = livekitRoomName;
    if (voiceCreationReceiptId != null)
      result.voiceCreationReceiptId = voiceCreationReceiptId;
    if (requestHash != null) result.requestHash = requestHash;
    if (createdAt != null) result.createdAt = createdAt;
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  ProvisionGameSessionRoomResponse._();

  factory ProvisionGameSessionRoomResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ProvisionGameSessionRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProvisionGameSessionRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'applicationId')
    ..aOS(3, _omitFieldNames ? '' : 'environmentId')
    ..aOM<GameSessionResourceRef>(4, _omitFieldNames ? '' : 'resource',
        subBuilder: GameSessionResourceRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'chatId')
    ..aOS(6, _omitFieldNames ? '' : 'chatCreationOperationId')
    ..aOS(7, _omitFieldNames ? '' : 'roomId')
    ..aOS(8, _omitFieldNames ? '' : 'livekitRoomName')
    ..aOS(9, _omitFieldNames ? '' : 'voiceCreationReceiptId')
    ..a<$core.List<$core.int>>(
        10, _omitFieldNames ? '' : 'requestHash', $pb.PbFieldType.OY)
    ..aOM<$2.Timestamp>(11, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $2.Timestamp.create)
    ..aOS(12, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProvisionGameSessionRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProvisionGameSessionRoomResponse copyWith(
          void Function(ProvisionGameSessionRoomResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ProvisionGameSessionRoomResponse))
          as ProvisionGameSessionRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ProvisionGameSessionRoomResponse create() =>
      ProvisionGameSessionRoomResponse._();
  @$core.override
  ProvisionGameSessionRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ProvisionGameSessionRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProvisionGameSessionRoomResponse>(
          create);
  static ProvisionGameSessionRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get applicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set applicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApplicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearApplicationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get environmentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set environmentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEnvironmentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnvironmentId() => $_clearField(3);

  @$pb.TagNumber(4)
  GameSessionResourceRef get resource => $_getN(3);
  @$pb.TagNumber(4)
  set resource(GameSessionResourceRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasResource() => $_has(3);
  @$pb.TagNumber(4)
  void clearResource() => $_clearField(4);
  @$pb.TagNumber(4)
  GameSessionResourceRef ensureResource() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get chatId => $_getSZ(4);
  @$pb.TagNumber(5)
  set chatId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasChatId() => $_has(4);
  @$pb.TagNumber(5)
  void clearChatId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get chatCreationOperationId => $_getSZ(5);
  @$pb.TagNumber(6)
  set chatCreationOperationId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasChatCreationOperationId() => $_has(5);
  @$pb.TagNumber(6)
  void clearChatCreationOperationId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get roomId => $_getSZ(6);
  @$pb.TagNumber(7)
  set roomId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRoomId() => $_has(6);
  @$pb.TagNumber(7)
  void clearRoomId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get livekitRoomName => $_getSZ(7);
  @$pb.TagNumber(8)
  set livekitRoomName($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasLivekitRoomName() => $_has(7);
  @$pb.TagNumber(8)
  void clearLivekitRoomName() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get voiceCreationReceiptId => $_getSZ(8);
  @$pb.TagNumber(9)
  set voiceCreationReceiptId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasVoiceCreationReceiptId() => $_has(8);
  @$pb.TagNumber(9)
  void clearVoiceCreationReceiptId() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.List<$core.int> get requestHash => $_getN(9);
  @$pb.TagNumber(10)
  set requestHash($core.List<$core.int> value) => $_setBytes(9, value);
  @$pb.TagNumber(10)
  $core.bool hasRequestHash() => $_has(9);
  @$pb.TagNumber(10)
  void clearRequestHash() => $_clearField(10);

  @$pb.TagNumber(11)
  $2.Timestamp get createdAt => $_getN(10);
  @$pb.TagNumber(11)
  set createdAt($2.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasCreatedAt() => $_has(10);
  @$pb.TagNumber(11)
  void clearCreatedAt() => $_clearField(11);
  @$pb.TagNumber(11)
  $2.Timestamp ensureCreatedAt() => $_ensure(10);

  @$pb.TagNumber(12)
  $core.String get sessionId => $_getSZ(11);
  @$pb.TagNumber(12)
  set sessionId($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasSessionId() => $_has(11);
  @$pb.TagNumber(12)
  void clearSessionId() => $_clearField(12);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class CloseGameSessionRoomRequest extends $pb.GeneratedMessage {
  factory CloseGameSessionRoomRequest({
    $core.String? operationId,
    $core.String? applicationId,
    $core.String? environmentId,
    GameSessionResourceRef? resource,
    $core.String? chatId,
    $core.String? chatCreationOperationId,
    $core.String? sessionId,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (applicationId != null) result.applicationId = applicationId;
    if (environmentId != null) result.environmentId = environmentId;
    if (resource != null) result.resource = resource;
    if (chatId != null) result.chatId = chatId;
    if (chatCreationOperationId != null)
      result.chatCreationOperationId = chatCreationOperationId;
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  CloseGameSessionRoomRequest._();

  factory CloseGameSessionRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CloseGameSessionRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CloseGameSessionRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'applicationId')
    ..aOS(3, _omitFieldNames ? '' : 'environmentId')
    ..aOM<GameSessionResourceRef>(4, _omitFieldNames ? '' : 'resource',
        subBuilder: GameSessionResourceRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'chatId')
    ..aOS(6, _omitFieldNames ? '' : 'chatCreationOperationId')
    ..aOS(7, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseGameSessionRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseGameSessionRoomRequest copyWith(
          void Function(CloseGameSessionRoomRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CloseGameSessionRoomRequest))
          as CloseGameSessionRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CloseGameSessionRoomRequest create() =>
      CloseGameSessionRoomRequest._();
  @$core.override
  CloseGameSessionRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CloseGameSessionRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CloseGameSessionRoomRequest>(create);
  static CloseGameSessionRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get applicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set applicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApplicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearApplicationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get environmentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set environmentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEnvironmentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnvironmentId() => $_clearField(3);

  @$pb.TagNumber(4)
  GameSessionResourceRef get resource => $_getN(3);
  @$pb.TagNumber(4)
  set resource(GameSessionResourceRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasResource() => $_has(3);
  @$pb.TagNumber(4)
  void clearResource() => $_clearField(4);
  @$pb.TagNumber(4)
  GameSessionResourceRef ensureResource() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get chatId => $_getSZ(4);
  @$pb.TagNumber(5)
  set chatId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasChatId() => $_has(4);
  @$pb.TagNumber(5)
  void clearChatId() => $_clearField(5);

  /// Chat create receipt identity is its original operation_id.
  @$pb.TagNumber(6)
  $core.String get chatCreationOperationId => $_getSZ(5);
  @$pb.TagNumber(6)
  set chatCreationOperationId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasChatCreationOperationId() => $_has(5);
  @$pb.TagNumber(6)
  void clearChatCreationOperationId() => $_clearField(6);

  /// Must equal the immutable GIS session identity stored at provisioning.
  @$pb.TagNumber(7)
  $core.String get sessionId => $_getSZ(6);
  @$pb.TagNumber(7)
  set sessionId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSessionId() => $_has(6);
  @$pb.TagNumber(7)
  void clearSessionId() => $_clearField(7);
}

class CloseGameSessionRoomResponse extends $pb.GeneratedMessage {
  factory CloseGameSessionRoomResponse({
    $core.String? operationId,
    $core.String? applicationId,
    $core.String? environmentId,
    GameSessionResourceRef? resource,
    $core.String? chatId,
    $core.String? chatCreationOperationId,
    $core.String? sessionId,
    $core.String? roomId,
    $core.String? status,
    $core.String? closeReceiptId,
    $core.List<$core.int>? requestHash,
    $2.Timestamp? closingAt,
    $2.Timestamp? mediaFencedAt,
    $2.Timestamp? closedAt,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (applicationId != null) result.applicationId = applicationId;
    if (environmentId != null) result.environmentId = environmentId;
    if (resource != null) result.resource = resource;
    if (chatId != null) result.chatId = chatId;
    if (chatCreationOperationId != null)
      result.chatCreationOperationId = chatCreationOperationId;
    if (sessionId != null) result.sessionId = sessionId;
    if (roomId != null) result.roomId = roomId;
    if (status != null) result.status = status;
    if (closeReceiptId != null) result.closeReceiptId = closeReceiptId;
    if (requestHash != null) result.requestHash = requestHash;
    if (closingAt != null) result.closingAt = closingAt;
    if (mediaFencedAt != null) result.mediaFencedAt = mediaFencedAt;
    if (closedAt != null) result.closedAt = closedAt;
    return result;
  }

  CloseGameSessionRoomResponse._();

  factory CloseGameSessionRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CloseGameSessionRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CloseGameSessionRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'applicationId')
    ..aOS(3, _omitFieldNames ? '' : 'environmentId')
    ..aOM<GameSessionResourceRef>(4, _omitFieldNames ? '' : 'resource',
        subBuilder: GameSessionResourceRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'chatId')
    ..aOS(6, _omitFieldNames ? '' : 'chatCreationOperationId')
    ..aOS(7, _omitFieldNames ? '' : 'sessionId')
    ..aOS(8, _omitFieldNames ? '' : 'roomId')
    ..aOS(9, _omitFieldNames ? '' : 'status')
    ..aOS(10, _omitFieldNames ? '' : 'closeReceiptId')
    ..a<$core.List<$core.int>>(
        11, _omitFieldNames ? '' : 'requestHash', $pb.PbFieldType.OY)
    ..aOM<$2.Timestamp>(12, _omitFieldNames ? '' : 'closingAt',
        subBuilder: $2.Timestamp.create)
    ..aOM<$2.Timestamp>(13, _omitFieldNames ? '' : 'mediaFencedAt',
        subBuilder: $2.Timestamp.create)
    ..aOM<$2.Timestamp>(14, _omitFieldNames ? '' : 'closedAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseGameSessionRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseGameSessionRoomResponse copyWith(
          void Function(CloseGameSessionRoomResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CloseGameSessionRoomResponse))
          as CloseGameSessionRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CloseGameSessionRoomResponse create() =>
      CloseGameSessionRoomResponse._();
  @$core.override
  CloseGameSessionRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CloseGameSessionRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CloseGameSessionRoomResponse>(create);
  static CloseGameSessionRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get applicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set applicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApplicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearApplicationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get environmentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set environmentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEnvironmentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnvironmentId() => $_clearField(3);

  @$pb.TagNumber(4)
  GameSessionResourceRef get resource => $_getN(3);
  @$pb.TagNumber(4)
  set resource(GameSessionResourceRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasResource() => $_has(3);
  @$pb.TagNumber(4)
  void clearResource() => $_clearField(4);
  @$pb.TagNumber(4)
  GameSessionResourceRef ensureResource() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get chatId => $_getSZ(4);
  @$pb.TagNumber(5)
  set chatId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasChatId() => $_has(4);
  @$pb.TagNumber(5)
  void clearChatId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get chatCreationOperationId => $_getSZ(5);
  @$pb.TagNumber(6)
  set chatCreationOperationId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasChatCreationOperationId() => $_has(5);
  @$pb.TagNumber(6)
  void clearChatCreationOperationId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get sessionId => $_getSZ(6);
  @$pb.TagNumber(7)
  set sessionId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSessionId() => $_has(6);
  @$pb.TagNumber(7)
  void clearSessionId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get roomId => $_getSZ(7);
  @$pb.TagNumber(8)
  set roomId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasRoomId() => $_has(7);
  @$pb.TagNumber(8)
  void clearRoomId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get status => $_getSZ(8);
  @$pb.TagNumber(9)
  set status($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasStatus() => $_has(8);
  @$pb.TagNumber(9)
  void clearStatus() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get closeReceiptId => $_getSZ(9);
  @$pb.TagNumber(10)
  set closeReceiptId($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasCloseReceiptId() => $_has(9);
  @$pb.TagNumber(10)
  void clearCloseReceiptId() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.List<$core.int> get requestHash => $_getN(10);
  @$pb.TagNumber(11)
  set requestHash($core.List<$core.int> value) => $_setBytes(10, value);
  @$pb.TagNumber(11)
  $core.bool hasRequestHash() => $_has(10);
  @$pb.TagNumber(11)
  void clearRequestHash() => $_clearField(11);

  @$pb.TagNumber(12)
  $2.Timestamp get closingAt => $_getN(11);
  @$pb.TagNumber(12)
  set closingAt($2.Timestamp value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasClosingAt() => $_has(11);
  @$pb.TagNumber(12)
  void clearClosingAt() => $_clearField(12);
  @$pb.TagNumber(12)
  $2.Timestamp ensureClosingAt() => $_ensure(11);

  @$pb.TagNumber(13)
  $2.Timestamp get mediaFencedAt => $_getN(12);
  @$pb.TagNumber(13)
  set mediaFencedAt($2.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasMediaFencedAt() => $_has(12);
  @$pb.TagNumber(13)
  void clearMediaFencedAt() => $_clearField(13);
  @$pb.TagNumber(13)
  $2.Timestamp ensureMediaFencedAt() => $_ensure(12);

  @$pb.TagNumber(14)
  $2.Timestamp get closedAt => $_getN(13);
  @$pb.TagNumber(14)
  set closedAt($2.Timestamp value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasClosedAt() => $_has(13);
  @$pb.TagNumber(14)
  void clearClosedAt() => $_clearField(14);
  @$pb.TagNumber(14)
  $2.Timestamp ensureClosedAt() => $_ensure(13);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class ApplyGameSessionRosterRequest extends $pb.GeneratedMessage {
  factory ApplyGameSessionRosterRequest({
    $core.String? operationId,
    $core.String? applicationId,
    $core.String? environmentId,
    $core.String? sessionId,
    $core.String? voiceRoomId,
    $fixnum.Int64? rosterRevision,
    $core.Iterable<$core.String>? profileIds,
    $2.Timestamp? leaseExpiresAt,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (applicationId != null) result.applicationId = applicationId;
    if (environmentId != null) result.environmentId = environmentId;
    if (sessionId != null) result.sessionId = sessionId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (rosterRevision != null) result.rosterRevision = rosterRevision;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (leaseExpiresAt != null) result.leaseExpiresAt = leaseExpiresAt;
    return result;
  }

  ApplyGameSessionRosterRequest._();

  factory ApplyGameSessionRosterRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyGameSessionRosterRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyGameSessionRosterRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'applicationId')
    ..aOS(3, _omitFieldNames ? '' : 'environmentId')
    ..aOS(4, _omitFieldNames ? '' : 'sessionId')
    ..aOS(5, _omitFieldNames ? '' : 'voiceRoomId')
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'rosterRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..pPS(7, _omitFieldNames ? '' : 'profileIds')
    ..aOM<$2.Timestamp>(8, _omitFieldNames ? '' : 'leaseExpiresAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyGameSessionRosterRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyGameSessionRosterRequest copyWith(
          void Function(ApplyGameSessionRosterRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ApplyGameSessionRosterRequest))
          as ApplyGameSessionRosterRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyGameSessionRosterRequest create() =>
      ApplyGameSessionRosterRequest._();
  @$core.override
  ApplyGameSessionRosterRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyGameSessionRosterRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyGameSessionRosterRequest>(create);
  static ApplyGameSessionRosterRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get applicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set applicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApplicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearApplicationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get environmentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set environmentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEnvironmentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnvironmentId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get sessionId => $_getSZ(3);
  @$pb.TagNumber(4)
  set sessionId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSessionId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSessionId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get voiceRoomId => $_getSZ(4);
  @$pb.TagNumber(5)
  set voiceRoomId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasVoiceRoomId() => $_has(4);
  @$pb.TagNumber(5)
  void clearVoiceRoomId() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get rosterRevision => $_getI64(5);
  @$pb.TagNumber(6)
  set rosterRevision($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRosterRevision() => $_has(5);
  @$pb.TagNumber(6)
  void clearRosterRevision() => $_clearField(6);

  /// Complete, sorted profile UUID set. Empty is a valid complete roster.
  @$pb.TagNumber(7)
  $pb.PbList<$core.String> get profileIds => $_getList(6);

  /// GIS database commit time plus the fixed lease duration.
  @$pb.TagNumber(8)
  $2.Timestamp get leaseExpiresAt => $_getN(7);
  @$pb.TagNumber(8)
  set leaseExpiresAt($2.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasLeaseExpiresAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearLeaseExpiresAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $2.Timestamp ensureLeaseExpiresAt() => $_ensure(7);
}

class ApplyGameSessionRosterResponse extends $pb.GeneratedMessage {
  factory ApplyGameSessionRosterResponse({
    $core.String? operationId,
    $core.String? applicationId,
    $core.String? environmentId,
    $core.String? sessionId,
    $core.String? voiceRoomId,
    $core.String? receiptId,
    $core.List<$core.int>? requestHash,
    $fixnum.Int64? acceptedRevision,
    $2.Timestamp? leaseExpiresAt,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (applicationId != null) result.applicationId = applicationId;
    if (environmentId != null) result.environmentId = environmentId;
    if (sessionId != null) result.sessionId = sessionId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (receiptId != null) result.receiptId = receiptId;
    if (requestHash != null) result.requestHash = requestHash;
    if (acceptedRevision != null) result.acceptedRevision = acceptedRevision;
    if (leaseExpiresAt != null) result.leaseExpiresAt = leaseExpiresAt;
    return result;
  }

  ApplyGameSessionRosterResponse._();

  factory ApplyGameSessionRosterResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyGameSessionRosterResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyGameSessionRosterResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'applicationId')
    ..aOS(3, _omitFieldNames ? '' : 'environmentId')
    ..aOS(4, _omitFieldNames ? '' : 'sessionId')
    ..aOS(5, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOS(6, _omitFieldNames ? '' : 'receiptId')
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'requestHash', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        8, _omitFieldNames ? '' : 'acceptedRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$2.Timestamp>(9, _omitFieldNames ? '' : 'leaseExpiresAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyGameSessionRosterResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyGameSessionRosterResponse copyWith(
          void Function(ApplyGameSessionRosterResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ApplyGameSessionRosterResponse))
          as ApplyGameSessionRosterResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyGameSessionRosterResponse create() =>
      ApplyGameSessionRosterResponse._();
  @$core.override
  ApplyGameSessionRosterResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyGameSessionRosterResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyGameSessionRosterResponse>(create);
  static ApplyGameSessionRosterResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get applicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set applicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApplicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearApplicationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get environmentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set environmentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEnvironmentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnvironmentId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get sessionId => $_getSZ(3);
  @$pb.TagNumber(4)
  set sessionId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSessionId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSessionId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get voiceRoomId => $_getSZ(4);
  @$pb.TagNumber(5)
  set voiceRoomId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasVoiceRoomId() => $_has(4);
  @$pb.TagNumber(5)
  void clearVoiceRoomId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get receiptId => $_getSZ(5);
  @$pb.TagNumber(6)
  set receiptId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasReceiptId() => $_has(5);
  @$pb.TagNumber(6)
  void clearReceiptId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get requestHash => $_getN(6);
  @$pb.TagNumber(7)
  set requestHash($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRequestHash() => $_has(6);
  @$pb.TagNumber(7)
  void clearRequestHash() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get acceptedRevision => $_getI64(7);
  @$pb.TagNumber(8)
  set acceptedRevision($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAcceptedRevision() => $_has(7);
  @$pb.TagNumber(8)
  void clearAcceptedRevision() => $_clearField(8);

  @$pb.TagNumber(9)
  $2.Timestamp get leaseExpiresAt => $_getN(8);
  @$pb.TagNumber(9)
  set leaseExpiresAt($2.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasLeaseExpiresAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearLeaseExpiresAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $2.Timestamp ensureLeaseExpiresAt() => $_ensure(8);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class FenceSdkConversionRequest extends $pb.GeneratedMessage {
  factory FenceSdkConversionRequest({
    $core.String? operationId,
    $core.String? bindingId,
    $core.String? sourceAccountId,
    $core.String? sourceActorId,
    $core.String? sourceProfileId,
    $core.String? targetAccountId,
    $core.String? targetProfileId,
    $fixnum.Int64? frozenAuthorityEpoch,
    $core.String? freezeReceiptId,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (bindingId != null) result.bindingId = bindingId;
    if (sourceAccountId != null) result.sourceAccountId = sourceAccountId;
    if (sourceActorId != null) result.sourceActorId = sourceActorId;
    if (sourceProfileId != null) result.sourceProfileId = sourceProfileId;
    if (targetAccountId != null) result.targetAccountId = targetAccountId;
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    if (frozenAuthorityEpoch != null)
      result.frozenAuthorityEpoch = frozenAuthorityEpoch;
    if (freezeReceiptId != null) result.freezeReceiptId = freezeReceiptId;
    return result;
  }

  FenceSdkConversionRequest._();

  factory FenceSdkConversionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FenceSdkConversionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FenceSdkConversionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'bindingId')
    ..aOS(3, _omitFieldNames ? '' : 'sourceAccountId')
    ..aOS(4, _omitFieldNames ? '' : 'sourceActorId')
    ..aOS(5, _omitFieldNames ? '' : 'sourceProfileId')
    ..aOS(6, _omitFieldNames ? '' : 'targetAccountId')
    ..aOS(7, _omitFieldNames ? '' : 'targetProfileId')
    ..a<$fixnum.Int64>(
        8, _omitFieldNames ? '' : 'frozenAuthorityEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(9, _omitFieldNames ? '' : 'freezeReceiptId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FenceSdkConversionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FenceSdkConversionRequest copyWith(
          void Function(FenceSdkConversionRequest) updates) =>
      super.copyWith((message) => updates(message as FenceSdkConversionRequest))
          as FenceSdkConversionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FenceSdkConversionRequest create() => FenceSdkConversionRequest._();
  @$core.override
  FenceSdkConversionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FenceSdkConversionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FenceSdkConversionRequest>(create);
  static FenceSdkConversionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get bindingId => $_getSZ(1);
  @$pb.TagNumber(2)
  set bindingId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBindingId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBindingId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sourceAccountId => $_getSZ(2);
  @$pb.TagNumber(3)
  set sourceAccountId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSourceAccountId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSourceAccountId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get sourceActorId => $_getSZ(3);
  @$pb.TagNumber(4)
  set sourceActorId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSourceActorId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSourceActorId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get sourceProfileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set sourceProfileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSourceProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearSourceProfileId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get targetAccountId => $_getSZ(5);
  @$pb.TagNumber(6)
  set targetAccountId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasTargetAccountId() => $_has(5);
  @$pb.TagNumber(6)
  void clearTargetAccountId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get targetProfileId => $_getSZ(6);
  @$pb.TagNumber(7)
  set targetProfileId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasTargetProfileId() => $_has(6);
  @$pb.TagNumber(7)
  void clearTargetProfileId() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get frozenAuthorityEpoch => $_getI64(7);
  @$pb.TagNumber(8)
  set frozenAuthorityEpoch($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasFrozenAuthorityEpoch() => $_has(7);
  @$pb.TagNumber(8)
  void clearFrozenAuthorityEpoch() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get freezeReceiptId => $_getSZ(8);
  @$pb.TagNumber(9)
  set freezeReceiptId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasFreezeReceiptId() => $_has(8);
  @$pb.TagNumber(9)
  void clearFreezeReceiptId() => $_clearField(9);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class FenceSdkConversionResponse extends $pb.GeneratedMessage {
  factory FenceSdkConversionResponse({
    $core.String? operationId,
    $core.String? bindingId,
    $core.String? sourceAccountId,
    $core.String? sourceActorId,
    $core.String? targetAccountId,
    $core.String? targetProfileId,
    $fixnum.Int64? frozenAuthorityEpoch,
    $core.String? freezeReceiptId,
    $core.String? receiptId,
    $core.List<$core.int>? requestHash,
    $core.bool? targetSessionConflict,
    $core.String? sourceRoomId,
    $fixnum.Int64? mediaGeneration,
    $2.Timestamp? observedEjectionAt,
    $2.Timestamp? committedAt,
    $core.String? sourceProfileId,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (bindingId != null) result.bindingId = bindingId;
    if (sourceAccountId != null) result.sourceAccountId = sourceAccountId;
    if (sourceActorId != null) result.sourceActorId = sourceActorId;
    if (targetAccountId != null) result.targetAccountId = targetAccountId;
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    if (frozenAuthorityEpoch != null)
      result.frozenAuthorityEpoch = frozenAuthorityEpoch;
    if (freezeReceiptId != null) result.freezeReceiptId = freezeReceiptId;
    if (receiptId != null) result.receiptId = receiptId;
    if (requestHash != null) result.requestHash = requestHash;
    if (targetSessionConflict != null)
      result.targetSessionConflict = targetSessionConflict;
    if (sourceRoomId != null) result.sourceRoomId = sourceRoomId;
    if (mediaGeneration != null) result.mediaGeneration = mediaGeneration;
    if (observedEjectionAt != null)
      result.observedEjectionAt = observedEjectionAt;
    if (committedAt != null) result.committedAt = committedAt;
    if (sourceProfileId != null) result.sourceProfileId = sourceProfileId;
    return result;
  }

  FenceSdkConversionResponse._();

  factory FenceSdkConversionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FenceSdkConversionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FenceSdkConversionResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'bindingId')
    ..aOS(3, _omitFieldNames ? '' : 'sourceAccountId')
    ..aOS(4, _omitFieldNames ? '' : 'sourceActorId')
    ..aOS(5, _omitFieldNames ? '' : 'targetAccountId')
    ..aOS(6, _omitFieldNames ? '' : 'targetProfileId')
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'frozenAuthorityEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(8, _omitFieldNames ? '' : 'freezeReceiptId')
    ..aOS(9, _omitFieldNames ? '' : 'receiptId')
    ..a<$core.List<$core.int>>(
        10, _omitFieldNames ? '' : 'requestHash', $pb.PbFieldType.OY)
    ..aOB(11, _omitFieldNames ? '' : 'targetSessionConflict')
    ..aOS(12, _omitFieldNames ? '' : 'sourceRoomId')
    ..a<$fixnum.Int64>(
        13, _omitFieldNames ? '' : 'mediaGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$2.Timestamp>(14, _omitFieldNames ? '' : 'observedEjectionAt',
        subBuilder: $2.Timestamp.create)
    ..aOM<$2.Timestamp>(15, _omitFieldNames ? '' : 'committedAt',
        subBuilder: $2.Timestamp.create)
    ..aOS(16, _omitFieldNames ? '' : 'sourceProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FenceSdkConversionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FenceSdkConversionResponse copyWith(
          void Function(FenceSdkConversionResponse) updates) =>
      super.copyWith(
              (message) => updates(message as FenceSdkConversionResponse))
          as FenceSdkConversionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FenceSdkConversionResponse create() => FenceSdkConversionResponse._();
  @$core.override
  FenceSdkConversionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FenceSdkConversionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FenceSdkConversionResponse>(create);
  static FenceSdkConversionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get bindingId => $_getSZ(1);
  @$pb.TagNumber(2)
  set bindingId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBindingId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBindingId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sourceAccountId => $_getSZ(2);
  @$pb.TagNumber(3)
  set sourceAccountId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSourceAccountId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSourceAccountId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get sourceActorId => $_getSZ(3);
  @$pb.TagNumber(4)
  set sourceActorId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSourceActorId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSourceActorId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get targetAccountId => $_getSZ(4);
  @$pb.TagNumber(5)
  set targetAccountId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTargetAccountId() => $_has(4);
  @$pb.TagNumber(5)
  void clearTargetAccountId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get targetProfileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set targetProfileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasTargetProfileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearTargetProfileId() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get frozenAuthorityEpoch => $_getI64(6);
  @$pb.TagNumber(7)
  set frozenAuthorityEpoch($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasFrozenAuthorityEpoch() => $_has(6);
  @$pb.TagNumber(7)
  void clearFrozenAuthorityEpoch() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get freezeReceiptId => $_getSZ(7);
  @$pb.TagNumber(8)
  set freezeReceiptId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasFreezeReceiptId() => $_has(7);
  @$pb.TagNumber(8)
  void clearFreezeReceiptId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get receiptId => $_getSZ(8);
  @$pb.TagNumber(9)
  set receiptId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasReceiptId() => $_has(8);
  @$pb.TagNumber(9)
  void clearReceiptId() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.List<$core.int> get requestHash => $_getN(9);
  @$pb.TagNumber(10)
  set requestHash($core.List<$core.int> value) => $_setBytes(9, value);
  @$pb.TagNumber(10)
  $core.bool hasRequestHash() => $_has(9);
  @$pb.TagNumber(10)
  void clearRequestHash() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.bool get targetSessionConflict => $_getBF(10);
  @$pb.TagNumber(11)
  set targetSessionConflict($core.bool value) => $_setBool(10, value);
  @$pb.TagNumber(11)
  $core.bool hasTargetSessionConflict() => $_has(10);
  @$pb.TagNumber(11)
  void clearTargetSessionConflict() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get sourceRoomId => $_getSZ(11);
  @$pb.TagNumber(12)
  set sourceRoomId($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasSourceRoomId() => $_has(11);
  @$pb.TagNumber(12)
  void clearSourceRoomId() => $_clearField(12);

  @$pb.TagNumber(13)
  $fixnum.Int64 get mediaGeneration => $_getI64(12);
  @$pb.TagNumber(13)
  set mediaGeneration($fixnum.Int64 value) => $_setInt64(12, value);
  @$pb.TagNumber(13)
  $core.bool hasMediaGeneration() => $_has(12);
  @$pb.TagNumber(13)
  void clearMediaGeneration() => $_clearField(13);

  @$pb.TagNumber(14)
  $2.Timestamp get observedEjectionAt => $_getN(13);
  @$pb.TagNumber(14)
  set observedEjectionAt($2.Timestamp value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasObservedEjectionAt() => $_has(13);
  @$pb.TagNumber(14)
  void clearObservedEjectionAt() => $_clearField(14);
  @$pb.TagNumber(14)
  $2.Timestamp ensureObservedEjectionAt() => $_ensure(13);

  @$pb.TagNumber(15)
  $2.Timestamp get committedAt => $_getN(14);
  @$pb.TagNumber(15)
  set committedAt($2.Timestamp value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasCommittedAt() => $_has(14);
  @$pb.TagNumber(15)
  void clearCommittedAt() => $_clearField(15);
  @$pb.TagNumber(15)
  $2.Timestamp ensureCommittedAt() => $_ensure(14);

  @$pb.TagNumber(16)
  $core.String get sourceProfileId => $_getSZ(15);
  @$pb.TagNumber(16)
  set sourceProfileId($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasSourceProfileId() => $_has(15);
  @$pb.TagNumber(16)
  void clearSourceProfileId() => $_clearField(16);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class CompleteSdkConversionActivationRequest extends $pb.GeneratedMessage {
  factory CompleteSdkConversionActivationRequest({
    $core.String? operationId,
    $core.String? bindingId,
    $fixnum.Int64? frozenAuthorityEpoch,
    $core.String? freezeReceiptId,
    $core.String? voiceReceiptId,
    $core.String? activationReceiptId,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (bindingId != null) result.bindingId = bindingId;
    if (frozenAuthorityEpoch != null)
      result.frozenAuthorityEpoch = frozenAuthorityEpoch;
    if (freezeReceiptId != null) result.freezeReceiptId = freezeReceiptId;
    if (voiceReceiptId != null) result.voiceReceiptId = voiceReceiptId;
    if (activationReceiptId != null)
      result.activationReceiptId = activationReceiptId;
    return result;
  }

  CompleteSdkConversionActivationRequest._();

  factory CompleteSdkConversionActivationRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompleteSdkConversionActivationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteSdkConversionActivationRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'bindingId')
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'frozenAuthorityEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(4, _omitFieldNames ? '' : 'freezeReceiptId')
    ..aOS(5, _omitFieldNames ? '' : 'voiceReceiptId')
    ..aOS(6, _omitFieldNames ? '' : 'activationReceiptId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteSdkConversionActivationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteSdkConversionActivationRequest copyWith(
          void Function(CompleteSdkConversionActivationRequest) updates) =>
      super.copyWith((message) =>
              updates(message as CompleteSdkConversionActivationRequest))
          as CompleteSdkConversionActivationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompleteSdkConversionActivationRequest create() =>
      CompleteSdkConversionActivationRequest._();
  @$core.override
  CompleteSdkConversionActivationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompleteSdkConversionActivationRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          CompleteSdkConversionActivationRequest>(create);
  static CompleteSdkConversionActivationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get bindingId => $_getSZ(1);
  @$pb.TagNumber(2)
  set bindingId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBindingId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBindingId() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get frozenAuthorityEpoch => $_getI64(2);
  @$pb.TagNumber(3)
  set frozenAuthorityEpoch($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFrozenAuthorityEpoch() => $_has(2);
  @$pb.TagNumber(3)
  void clearFrozenAuthorityEpoch() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get freezeReceiptId => $_getSZ(3);
  @$pb.TagNumber(4)
  set freezeReceiptId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFreezeReceiptId() => $_has(3);
  @$pb.TagNumber(4)
  void clearFreezeReceiptId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get voiceReceiptId => $_getSZ(4);
  @$pb.TagNumber(5)
  set voiceReceiptId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasVoiceReceiptId() => $_has(4);
  @$pb.TagNumber(5)
  void clearVoiceReceiptId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get activationReceiptId => $_getSZ(5);
  @$pb.TagNumber(6)
  set activationReceiptId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasActivationReceiptId() => $_has(5);
  @$pb.TagNumber(6)
  void clearActivationReceiptId() => $_clearField(6);
}

/// @voice.unknown_fields=reject
/// @voice.hash=deterministic_protobuf_sha256
class CompleteSdkConversionActivationResponse extends $pb.GeneratedMessage {
  factory CompleteSdkConversionActivationResponse({
    $core.String? operationId,
    $core.String? bindingId,
    $fixnum.Int64? frozenAuthorityEpoch,
    $core.String? voiceReceiptId,
    $core.String? activationReceiptId,
    $core.String? receiptId,
    $core.List<$core.int>? requestHash,
    $core.String? state,
    $2.Timestamp? committedAt,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (bindingId != null) result.bindingId = bindingId;
    if (frozenAuthorityEpoch != null)
      result.frozenAuthorityEpoch = frozenAuthorityEpoch;
    if (voiceReceiptId != null) result.voiceReceiptId = voiceReceiptId;
    if (activationReceiptId != null)
      result.activationReceiptId = activationReceiptId;
    if (receiptId != null) result.receiptId = receiptId;
    if (requestHash != null) result.requestHash = requestHash;
    if (state != null) result.state = state;
    if (committedAt != null) result.committedAt = committedAt;
    return result;
  }

  CompleteSdkConversionActivationResponse._();

  factory CompleteSdkConversionActivationResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompleteSdkConversionActivationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteSdkConversionActivationResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'bindingId')
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'frozenAuthorityEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(4, _omitFieldNames ? '' : 'voiceReceiptId')
    ..aOS(5, _omitFieldNames ? '' : 'activationReceiptId')
    ..aOS(6, _omitFieldNames ? '' : 'receiptId')
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'requestHash', $pb.PbFieldType.OY)
    ..aOS(8, _omitFieldNames ? '' : 'state')
    ..aOM<$2.Timestamp>(9, _omitFieldNames ? '' : 'committedAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteSdkConversionActivationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteSdkConversionActivationResponse copyWith(
          void Function(CompleteSdkConversionActivationResponse) updates) =>
      super.copyWith((message) =>
              updates(message as CompleteSdkConversionActivationResponse))
          as CompleteSdkConversionActivationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompleteSdkConversionActivationResponse create() =>
      CompleteSdkConversionActivationResponse._();
  @$core.override
  CompleteSdkConversionActivationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompleteSdkConversionActivationResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          CompleteSdkConversionActivationResponse>(create);
  static CompleteSdkConversionActivationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get bindingId => $_getSZ(1);
  @$pb.TagNumber(2)
  set bindingId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBindingId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBindingId() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get frozenAuthorityEpoch => $_getI64(2);
  @$pb.TagNumber(3)
  set frozenAuthorityEpoch($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFrozenAuthorityEpoch() => $_has(2);
  @$pb.TagNumber(3)
  void clearFrozenAuthorityEpoch() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get voiceReceiptId => $_getSZ(3);
  @$pb.TagNumber(4)
  set voiceReceiptId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasVoiceReceiptId() => $_has(3);
  @$pb.TagNumber(4)
  void clearVoiceReceiptId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get activationReceiptId => $_getSZ(4);
  @$pb.TagNumber(5)
  set activationReceiptId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasActivationReceiptId() => $_has(4);
  @$pb.TagNumber(5)
  void clearActivationReceiptId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get receiptId => $_getSZ(5);
  @$pb.TagNumber(6)
  set receiptId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasReceiptId() => $_has(5);
  @$pb.TagNumber(6)
  void clearReceiptId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get requestHash => $_getN(6);
  @$pb.TagNumber(7)
  set requestHash($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRequestHash() => $_has(6);
  @$pb.TagNumber(7)
  void clearRequestHash() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get state => $_getSZ(7);
  @$pb.TagNumber(8)
  set state($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasState() => $_has(7);
  @$pb.TagNumber(8)
  void clearState() => $_clearField(8);

  @$pb.TagNumber(9)
  $2.Timestamp get committedAt => $_getN(8);
  @$pb.TagNumber(9)
  set committedAt($2.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCommittedAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearCommittedAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $2.Timestamp ensureCommittedAt() => $_ensure(8);
}

class StartCallRequest extends $pb.GeneratedMessage {
  factory StartCallRequest({
    $core.String? roomType,
    $1.ChatRef? linkedChat,
    $core.String? voiceRoomId,
    $3.SpaceRef? space,
    VoiceSessionKind? roomTypeEnum,
    $core.String? calleeProfileId,
    CallMediaKind? mediaKind,
  }) {
    final result = create();
    if (roomType != null) result.roomType = roomType;
    if (linkedChat != null) result.linkedChat = linkedChat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (space != null) result.space = space;
    if (roomTypeEnum != null) result.roomTypeEnum = roomTypeEnum;
    if (calleeProfileId != null) result.calleeProfileId = calleeProfileId;
    if (mediaKind != null) result.mediaKind = mediaKind;
    return result;
  }

  StartCallRequest._();

  factory StartCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomType')
    ..aOM<$1.ChatRef>(2, _omitFieldNames ? '' : 'linkedChat',
        subBuilder: $1.ChatRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOM<$3.SpaceRef>(4, _omitFieldNames ? '' : 'space',
        subBuilder: $3.SpaceRef.create)
    ..aE<VoiceSessionKind>(5, _omitFieldNames ? '' : 'roomTypeEnum',
        enumValues: VoiceSessionKind.values)
    ..aOS(6, _omitFieldNames ? '' : 'calleeProfileId')
    ..aE<CallMediaKind>(7, _omitFieldNames ? '' : 'mediaKind',
        enumValues: CallMediaKind.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCallRequest copyWith(void Function(StartCallRequest) updates) =>
      super.copyWith((message) => updates(message as StartCallRequest))
          as StartCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartCallRequest create() => StartCallRequest._();
  @$core.override
  StartCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartCallRequest>(create);
  static StartCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomType => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomType($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomType() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomType() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.ChatRef get linkedChat => $_getN(1);
  @$pb.TagNumber(2)
  set linkedChat($1.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasLinkedChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearLinkedChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.ChatRef ensureLinkedChat() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get voiceRoomId => $_getSZ(2);
  @$pb.TagNumber(3)
  set voiceRoomId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasVoiceRoomId() => $_has(2);
  @$pb.TagNumber(3)
  void clearVoiceRoomId() => $_clearField(3);

  @$pb.TagNumber(4)
  $3.SpaceRef get space => $_getN(3);
  @$pb.TagNumber(4)
  set space($3.SpaceRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSpace() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpace() => $_clearField(4);
  @$pb.TagNumber(4)
  $3.SpaceRef ensureSpace() => $_ensure(3);

  @$pb.TagNumber(5)
  VoiceSessionKind get roomTypeEnum => $_getN(4);
  @$pb.TagNumber(5)
  set roomTypeEnum(VoiceSessionKind value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasRoomTypeEnum() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoomTypeEnum() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get calleeProfileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set calleeProfileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCalleeProfileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearCalleeProfileId() => $_clearField(6);

  @$pb.TagNumber(7)
  CallMediaKind get mediaKind => $_getN(6);
  @$pb.TagNumber(7)
  set mediaKind(CallMediaKind value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasMediaKind() => $_has(6);
  @$pb.TagNumber(7)
  void clearMediaKind() => $_clearField(7);
}

class AcceptCallRequest extends $pb.GeneratedMessage {
  factory AcceptCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  AcceptCallRequest._();

  factory AcceptCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcceptCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptCallRequest copyWith(void Function(AcceptCallRequest) updates) =>
      super.copyWith((message) => updates(message as AcceptCallRequest))
          as AcceptCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcceptCallRequest create() => AcceptCallRequest._();
  @$core.override
  AcceptCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcceptCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptCallRequest>(create);
  static AcceptCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class DeclineCallRequest extends $pb.GeneratedMessage {
  factory DeclineCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  DeclineCallRequest._();

  factory DeclineCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeclineCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineCallRequest copyWith(void Function(DeclineCallRequest) updates) =>
      super.copyWith((message) => updates(message as DeclineCallRequest))
          as DeclineCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeclineCallRequest create() => DeclineCallRequest._();
  @$core.override
  DeclineCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeclineCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineCallRequest>(create);
  static DeclineCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class JoinCallRequest extends $pb.GeneratedMessage {
  factory JoinCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  JoinCallRequest._();

  factory JoinCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinCallRequest copyWith(void Function(JoinCallRequest) updates) =>
      super.copyWith((message) => updates(message as JoinCallRequest))
          as JoinCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinCallRequest create() => JoinCallRequest._();
  @$core.override
  JoinCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinCallRequest>(create);
  static JoinCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class LeaveCallRequest extends $pb.GeneratedMessage {
  factory LeaveCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  LeaveCallRequest._();

  factory LeaveCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveCallRequest copyWith(void Function(LeaveCallRequest) updates) =>
      super.copyWith((message) => updates(message as LeaveCallRequest))
          as LeaveCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveCallRequest create() => LeaveCallRequest._();
  @$core.override
  LeaveCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveCallRequest>(create);
  static LeaveCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class EndCallRequest extends $pb.GeneratedMessage {
  factory EndCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  EndCallRequest._();

  factory EndCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EndCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EndCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCallRequest copyWith(void Function(EndCallRequest) updates) =>
      super.copyWith((message) => updates(message as EndCallRequest))
          as EndCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EndCallRequest create() => EndCallRequest._();
  @$core.override
  EndCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EndCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EndCallRequest>(create);
  static EndCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class CallSession extends $pb.GeneratedMessage {
  factory CallSession({
    $core.String? roomId,
    $core.String? livekitRoomName,
    $core.String? roomType,
    $1.ChatRef? linkedChat,
    $core.String? voiceRoomId,
    $2.Timestamp? startedAt,
    VoiceSessionKind? roomTypeEnum,
    $core.String? initiatorProfileId,
    $core.String? calleeProfileId,
    CallMediaKind? mediaKind,
    CallStatus? status,
    $2.Timestamp? expiresAt,
    $2.Timestamp? endedAt,
    $core.String? spaceId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (livekitRoomName != null) result.livekitRoomName = livekitRoomName;
    if (roomType != null) result.roomType = roomType;
    if (linkedChat != null) result.linkedChat = linkedChat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (startedAt != null) result.startedAt = startedAt;
    if (roomTypeEnum != null) result.roomTypeEnum = roomTypeEnum;
    if (initiatorProfileId != null)
      result.initiatorProfileId = initiatorProfileId;
    if (calleeProfileId != null) result.calleeProfileId = calleeProfileId;
    if (mediaKind != null) result.mediaKind = mediaKind;
    if (status != null) result.status = status;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (endedAt != null) result.endedAt = endedAt;
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  CallSession._();

  factory CallSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CallSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CallSession',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'livekitRoomName')
    ..aOS(3, _omitFieldNames ? '' : 'roomType')
    ..aOM<$1.ChatRef>(4, _omitFieldNames ? '' : 'linkedChat',
        subBuilder: $1.ChatRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOM<$2.Timestamp>(6, _omitFieldNames ? '' : 'startedAt',
        subBuilder: $2.Timestamp.create)
    ..aE<VoiceSessionKind>(7, _omitFieldNames ? '' : 'roomTypeEnum',
        enumValues: VoiceSessionKind.values)
    ..aOS(8, _omitFieldNames ? '' : 'initiatorProfileId')
    ..aOS(9, _omitFieldNames ? '' : 'calleeProfileId')
    ..aE<CallMediaKind>(10, _omitFieldNames ? '' : 'mediaKind',
        enumValues: CallMediaKind.values)
    ..aE<CallStatus>(11, _omitFieldNames ? '' : 'status',
        enumValues: CallStatus.values)
    ..aOM<$2.Timestamp>(12, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $2.Timestamp.create)
    ..aOM<$2.Timestamp>(13, _omitFieldNames ? '' : 'endedAt',
        subBuilder: $2.Timestamp.create)
    ..aOS(14, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallSession copyWith(void Function(CallSession) updates) =>
      super.copyWith((message) => updates(message as CallSession))
          as CallSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CallSession create() => CallSession._();
  @$core.override
  CallSession createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CallSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CallSession>(create);
  static CallSession? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get livekitRoomName => $_getSZ(1);
  @$pb.TagNumber(2)
  set livekitRoomName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLivekitRoomName() => $_has(1);
  @$pb.TagNumber(2)
  void clearLivekitRoomName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get roomType => $_getSZ(2);
  @$pb.TagNumber(3)
  set roomType($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRoomType() => $_has(2);
  @$pb.TagNumber(3)
  void clearRoomType() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.ChatRef get linkedChat => $_getN(3);
  @$pb.TagNumber(4)
  set linkedChat($1.ChatRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasLinkedChat() => $_has(3);
  @$pb.TagNumber(4)
  void clearLinkedChat() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.ChatRef ensureLinkedChat() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get voiceRoomId => $_getSZ(4);
  @$pb.TagNumber(5)
  set voiceRoomId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasVoiceRoomId() => $_has(4);
  @$pb.TagNumber(5)
  void clearVoiceRoomId() => $_clearField(5);

  @$pb.TagNumber(6)
  $2.Timestamp get startedAt => $_getN(5);
  @$pb.TagNumber(6)
  set startedAt($2.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasStartedAt() => $_has(5);
  @$pb.TagNumber(6)
  void clearStartedAt() => $_clearField(6);
  @$pb.TagNumber(6)
  $2.Timestamp ensureStartedAt() => $_ensure(5);

  @$pb.TagNumber(7)
  VoiceSessionKind get roomTypeEnum => $_getN(6);
  @$pb.TagNumber(7)
  set roomTypeEnum(VoiceSessionKind value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasRoomTypeEnum() => $_has(6);
  @$pb.TagNumber(7)
  void clearRoomTypeEnum() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get initiatorProfileId => $_getSZ(7);
  @$pb.TagNumber(8)
  set initiatorProfileId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasInitiatorProfileId() => $_has(7);
  @$pb.TagNumber(8)
  void clearInitiatorProfileId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get calleeProfileId => $_getSZ(8);
  @$pb.TagNumber(9)
  set calleeProfileId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasCalleeProfileId() => $_has(8);
  @$pb.TagNumber(9)
  void clearCalleeProfileId() => $_clearField(9);

  @$pb.TagNumber(10)
  CallMediaKind get mediaKind => $_getN(9);
  @$pb.TagNumber(10)
  set mediaKind(CallMediaKind value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasMediaKind() => $_has(9);
  @$pb.TagNumber(10)
  void clearMediaKind() => $_clearField(10);

  @$pb.TagNumber(11)
  CallStatus get status => $_getN(10);
  @$pb.TagNumber(11)
  set status(CallStatus value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasStatus() => $_has(10);
  @$pb.TagNumber(11)
  void clearStatus() => $_clearField(11);

  @$pb.TagNumber(12)
  $2.Timestamp get expiresAt => $_getN(11);
  @$pb.TagNumber(12)
  set expiresAt($2.Timestamp value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasExpiresAt() => $_has(11);
  @$pb.TagNumber(12)
  void clearExpiresAt() => $_clearField(12);
  @$pb.TagNumber(12)
  $2.Timestamp ensureExpiresAt() => $_ensure(11);

  @$pb.TagNumber(13)
  $2.Timestamp get endedAt => $_getN(12);
  @$pb.TagNumber(13)
  set endedAt($2.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasEndedAt() => $_has(12);
  @$pb.TagNumber(13)
  void clearEndedAt() => $_clearField(13);
  @$pb.TagNumber(13)
  $2.Timestamp ensureEndedAt() => $_ensure(12);

  /// Persisted room locator, not a current authorization grant. Absent for legacy/incomplete bindings.
  @$pb.TagNumber(14)
  $core.String get spaceId => $_getSZ(13);
  @$pb.TagNumber(14)
  set spaceId($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasSpaceId() => $_has(13);
  @$pb.TagNumber(14)
  void clearSpaceId() => $_clearField(14);
}

class JoinVoiceRoomRequest extends $pb.GeneratedMessage {
  factory JoinVoiceRoomRequest({
    $core.String? voiceRoomId,
    $3.SpaceRef? space,
    $core.String? operationId,
  }) {
    final result = create();
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (space != null) result.space = space;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  JoinVoiceRoomRequest._();

  factory JoinVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOM<$3.SpaceRef>(2, _omitFieldNames ? '' : 'space',
        subBuilder: $3.SpaceRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinVoiceRoomRequest copyWith(void Function(JoinVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as JoinVoiceRoomRequest))
          as JoinVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinVoiceRoomRequest create() => JoinVoiceRoomRequest._();
  @$core.override
  JoinVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinVoiceRoomRequest>(create);
  static JoinVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get voiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set voiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $3.SpaceRef get space => $_getN(1);
  @$pb.TagNumber(2)
  set space($3.SpaceRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSpace() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpace() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.SpaceRef ensureSpace() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);
}

class LeaveVoiceRoomRequest extends $pb.GeneratedMessage {
  factory LeaveVoiceRoomRequest({
    $core.String? voiceRoomId,
    $3.SpaceRef? space,
    $core.String? operationId,
  }) {
    final result = create();
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (space != null) result.space = space;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  LeaveVoiceRoomRequest._();

  factory LeaveVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOM<$3.SpaceRef>(2, _omitFieldNames ? '' : 'space',
        subBuilder: $3.SpaceRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveVoiceRoomRequest copyWith(
          void Function(LeaveVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as LeaveVoiceRoomRequest))
          as LeaveVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveVoiceRoomRequest create() => LeaveVoiceRoomRequest._();
  @$core.override
  LeaveVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveVoiceRoomRequest>(create);
  static LeaveVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get voiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set voiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $3.SpaceRef get space => $_getN(1);
  @$pb.TagNumber(2)
  set space($3.SpaceRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSpace() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpace() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.SpaceRef ensureSpace() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);
}

class MoveToVoiceRoomRequest extends $pb.GeneratedMessage {
  factory MoveToVoiceRoomRequest({
    $core.String? fromVoiceRoomId,
    $core.String? toVoiceRoomId,
    $3.SpaceRef? space,
    $core.String? operationId,
  }) {
    final result = create();
    if (fromVoiceRoomId != null) result.fromVoiceRoomId = fromVoiceRoomId;
    if (toVoiceRoomId != null) result.toVoiceRoomId = toVoiceRoomId;
    if (space != null) result.space = space;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  MoveToVoiceRoomRequest._();

  factory MoveToVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MoveToVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MoveToVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fromVoiceRoomId')
    ..aOS(2, _omitFieldNames ? '' : 'toVoiceRoomId')
    ..aOM<$3.SpaceRef>(3, _omitFieldNames ? '' : 'space',
        subBuilder: $3.SpaceRef.create)
    ..aOS(4, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveToVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveToVoiceRoomRequest copyWith(
          void Function(MoveToVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as MoveToVoiceRoomRequest))
          as MoveToVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MoveToVoiceRoomRequest create() => MoveToVoiceRoomRequest._();
  @$core.override
  MoveToVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MoveToVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MoveToVoiceRoomRequest>(create);
  static MoveToVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fromVoiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fromVoiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFromVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFromVoiceRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get toVoiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set toVoiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasToVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearToVoiceRoomId() => $_clearField(2);

  @$pb.TagNumber(3)
  $3.SpaceRef get space => $_getN(2);
  @$pb.TagNumber(3)
  set space($3.SpaceRef value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSpace() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpace() => $_clearField(3);
  @$pb.TagNumber(3)
  $3.SpaceRef ensureSpace() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get operationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set operationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearOperationId() => $_clearField(4);
}

class MoveVoiceRoomParticipantRequest extends $pb.GeneratedMessage {
  factory MoveVoiceRoomParticipantRequest({
    $core.String? fromVoiceRoomId,
    $core.String? toVoiceRoomId,
    $3.SpaceRef? space,
    $core.String? participantProfileId,
    $core.String? operationId,
  }) {
    final result = create();
    if (fromVoiceRoomId != null) result.fromVoiceRoomId = fromVoiceRoomId;
    if (toVoiceRoomId != null) result.toVoiceRoomId = toVoiceRoomId;
    if (space != null) result.space = space;
    if (participantProfileId != null)
      result.participantProfileId = participantProfileId;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  MoveVoiceRoomParticipantRequest._();

  factory MoveVoiceRoomParticipantRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MoveVoiceRoomParticipantRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MoveVoiceRoomParticipantRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fromVoiceRoomId')
    ..aOS(2, _omitFieldNames ? '' : 'toVoiceRoomId')
    ..aOM<$3.SpaceRef>(3, _omitFieldNames ? '' : 'space',
        subBuilder: $3.SpaceRef.create)
    ..aOS(4, _omitFieldNames ? '' : 'participantProfileId')
    ..aOS(5, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveVoiceRoomParticipantRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveVoiceRoomParticipantRequest copyWith(
          void Function(MoveVoiceRoomParticipantRequest) updates) =>
      super.copyWith(
              (message) => updates(message as MoveVoiceRoomParticipantRequest))
          as MoveVoiceRoomParticipantRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MoveVoiceRoomParticipantRequest create() =>
      MoveVoiceRoomParticipantRequest._();
  @$core.override
  MoveVoiceRoomParticipantRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MoveVoiceRoomParticipantRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MoveVoiceRoomParticipantRequest>(
          create);
  static MoveVoiceRoomParticipantRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fromVoiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fromVoiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFromVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFromVoiceRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get toVoiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set toVoiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasToVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearToVoiceRoomId() => $_clearField(2);

  @$pb.TagNumber(3)
  $3.SpaceRef get space => $_getN(2);
  @$pb.TagNumber(3)
  set space($3.SpaceRef value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSpace() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpace() => $_clearField(3);
  @$pb.TagNumber(3)
  $3.SpaceRef ensureSpace() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get participantProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set participantProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasParticipantProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearParticipantProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get operationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set operationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasOperationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearOperationId() => $_clearField(5);
}

class VoiceSession extends $pb.GeneratedMessage {
  factory VoiceSession({
    $core.String? roomId,
    $core.String? livekitRoomName,
    $core.String? voiceRoomId,
    $core.String? spaceId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (livekitRoomName != null) result.livekitRoomName = livekitRoomName;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  VoiceSession._();

  factory VoiceSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceSession',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'livekitRoomName')
    ..aOS(3, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOS(4, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceSession copyWith(void Function(VoiceSession) updates) =>
      super.copyWith((message) => updates(message as VoiceSession))
          as VoiceSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceSession create() => VoiceSession._();
  @$core.override
  VoiceSession createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VoiceSession>(create);
  static VoiceSession? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get livekitRoomName => $_getSZ(1);
  @$pb.TagNumber(2)
  set livekitRoomName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLivekitRoomName() => $_has(1);
  @$pb.TagNumber(2)
  void clearLivekitRoomName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get voiceRoomId => $_getSZ(2);
  @$pb.TagNumber(3)
  set voiceRoomId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasVoiceRoomId() => $_has(2);
  @$pb.TagNumber(3)
  void clearVoiceRoomId() => $_clearField(3);

  /// Server-resolved Space persisted when joining this voice room.
  @$pb.TagNumber(4)
  $core.String get spaceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set spaceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSpaceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpaceId() => $_clearField(4);
}

/// Immutable terminal history for one actor-bound lifecycle operation. It never
/// contains a bearer, credential, reusable permission or grant.
class VoiceRoomLifecycleReceipt extends $pb.GeneratedMessage {
  factory VoiceRoomLifecycleReceipt({
    $core.String? operationId,
    $core.String? actorProfileId,
    $core.String? subjectProfileId,
    $3.SpaceRef? space,
    VoiceRoomLifecycleMethod? method,
    VoiceRoomLifecycleOutcome? outcome,
    $core.String? sourceVoiceRoomId,
    $core.String? destinationVoiceRoomId,
    $core.String? roomId,
    $fixnum.Int64? sourceRosterVersion,
    $fixnum.Int64? destinationRosterVersion,
    $core.String? mediaEpoch,
    $fixnum.Int64? spaceAccessEpoch,
    $fixnum.Int64? rolePolicyEpoch,
    $core.List<$core.int>? authorizationDigest,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (subjectProfileId != null) result.subjectProfileId = subjectProfileId;
    if (space != null) result.space = space;
    if (method != null) result.method = method;
    if (outcome != null) result.outcome = outcome;
    if (sourceVoiceRoomId != null) result.sourceVoiceRoomId = sourceVoiceRoomId;
    if (destinationVoiceRoomId != null)
      result.destinationVoiceRoomId = destinationVoiceRoomId;
    if (roomId != null) result.roomId = roomId;
    if (sourceRosterVersion != null)
      result.sourceRosterVersion = sourceRosterVersion;
    if (destinationRosterVersion != null)
      result.destinationRosterVersion = destinationRosterVersion;
    if (mediaEpoch != null) result.mediaEpoch = mediaEpoch;
    if (spaceAccessEpoch != null) result.spaceAccessEpoch = spaceAccessEpoch;
    if (rolePolicyEpoch != null) result.rolePolicyEpoch = rolePolicyEpoch;
    if (authorizationDigest != null)
      result.authorizationDigest = authorizationDigest;
    return result;
  }

  VoiceRoomLifecycleReceipt._();

  factory VoiceRoomLifecycleReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceRoomLifecycleReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceRoomLifecycleReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'subjectProfileId')
    ..aOM<$3.SpaceRef>(4, _omitFieldNames ? '' : 'space',
        subBuilder: $3.SpaceRef.create)
    ..aE<VoiceRoomLifecycleMethod>(5, _omitFieldNames ? '' : 'method',
        enumValues: VoiceRoomLifecycleMethod.values)
    ..aE<VoiceRoomLifecycleOutcome>(6, _omitFieldNames ? '' : 'outcome',
        enumValues: VoiceRoomLifecycleOutcome.values)
    ..aOS(7, _omitFieldNames ? '' : 'sourceVoiceRoomId')
    ..aOS(8, _omitFieldNames ? '' : 'destinationVoiceRoomId')
    ..aOS(9, _omitFieldNames ? '' : 'roomId')
    ..a<$fixnum.Int64>(
        10, _omitFieldNames ? '' : 'sourceRosterVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(11, _omitFieldNames ? '' : 'destinationRosterVersion',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(12, _omitFieldNames ? '' : 'mediaEpoch')
    ..a<$fixnum.Int64>(
        13, _omitFieldNames ? '' : 'spaceAccessEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        14, _omitFieldNames ? '' : 'rolePolicyEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        15, _omitFieldNames ? '' : 'authorizationDigest', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceRoomLifecycleReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceRoomLifecycleReceipt copyWith(
          void Function(VoiceRoomLifecycleReceipt) updates) =>
      super.copyWith((message) => updates(message as VoiceRoomLifecycleReceipt))
          as VoiceRoomLifecycleReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceRoomLifecycleReceipt create() => VoiceRoomLifecycleReceipt._();
  @$core.override
  VoiceRoomLifecycleReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceRoomLifecycleReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VoiceRoomLifecycleReceipt>(create);
  static VoiceRoomLifecycleReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get actorProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set actorProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasActorProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearActorProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get subjectProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set subjectProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSubjectProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSubjectProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $3.SpaceRef get space => $_getN(3);
  @$pb.TagNumber(4)
  set space($3.SpaceRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSpace() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpace() => $_clearField(4);
  @$pb.TagNumber(4)
  $3.SpaceRef ensureSpace() => $_ensure(3);

  @$pb.TagNumber(5)
  VoiceRoomLifecycleMethod get method => $_getN(4);
  @$pb.TagNumber(5)
  set method(VoiceRoomLifecycleMethod value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasMethod() => $_has(4);
  @$pb.TagNumber(5)
  void clearMethod() => $_clearField(5);

  @$pb.TagNumber(6)
  VoiceRoomLifecycleOutcome get outcome => $_getN(5);
  @$pb.TagNumber(6)
  set outcome(VoiceRoomLifecycleOutcome value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasOutcome() => $_has(5);
  @$pb.TagNumber(6)
  void clearOutcome() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get sourceVoiceRoomId => $_getSZ(6);
  @$pb.TagNumber(7)
  set sourceVoiceRoomId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSourceVoiceRoomId() => $_has(6);
  @$pb.TagNumber(7)
  void clearSourceVoiceRoomId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get destinationVoiceRoomId => $_getSZ(7);
  @$pb.TagNumber(8)
  set destinationVoiceRoomId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasDestinationVoiceRoomId() => $_has(7);
  @$pb.TagNumber(8)
  void clearDestinationVoiceRoomId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get roomId => $_getSZ(8);
  @$pb.TagNumber(9)
  set roomId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRoomId() => $_has(8);
  @$pb.TagNumber(9)
  void clearRoomId() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get sourceRosterVersion => $_getI64(9);
  @$pb.TagNumber(10)
  set sourceRosterVersion($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasSourceRosterVersion() => $_has(9);
  @$pb.TagNumber(10)
  void clearSourceRosterVersion() => $_clearField(10);

  @$pb.TagNumber(11)
  $fixnum.Int64 get destinationRosterVersion => $_getI64(10);
  @$pb.TagNumber(11)
  set destinationRosterVersion($fixnum.Int64 value) => $_setInt64(10, value);
  @$pb.TagNumber(11)
  $core.bool hasDestinationRosterVersion() => $_has(10);
  @$pb.TagNumber(11)
  void clearDestinationRosterVersion() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get mediaEpoch => $_getSZ(11);
  @$pb.TagNumber(12)
  set mediaEpoch($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasMediaEpoch() => $_has(11);
  @$pb.TagNumber(12)
  void clearMediaEpoch() => $_clearField(12);

  @$pb.TagNumber(13)
  $fixnum.Int64 get spaceAccessEpoch => $_getI64(12);
  @$pb.TagNumber(13)
  set spaceAccessEpoch($fixnum.Int64 value) => $_setInt64(12, value);
  @$pb.TagNumber(13)
  $core.bool hasSpaceAccessEpoch() => $_has(12);
  @$pb.TagNumber(13)
  void clearSpaceAccessEpoch() => $_clearField(13);

  @$pb.TagNumber(14)
  $fixnum.Int64 get rolePolicyEpoch => $_getI64(13);
  @$pb.TagNumber(14)
  set rolePolicyEpoch($fixnum.Int64 value) => $_setInt64(13, value);
  @$pb.TagNumber(14)
  $core.bool hasRolePolicyEpoch() => $_has(13);
  @$pb.TagNumber(14)
  void clearRolePolicyEpoch() => $_clearField(14);

  @$pb.TagNumber(15)
  $core.List<$core.int> get authorizationDigest => $_getN(14);
  @$pb.TagNumber(15)
  set authorizationDigest($core.List<$core.int> value) => $_setBytes(14, value);
  @$pb.TagNumber(15)
  $core.bool hasAuthorizationDigest() => $_has(14);
  @$pb.TagNumber(15)
  void clearAuthorizationDigest() => $_clearField(15);
}

class GetJoinTokenRequest extends $pb.GeneratedMessage {
  factory GetJoinTokenRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  GetJoinTokenRequest._();

  factory GetJoinTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetJoinTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetJoinTokenRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJoinTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJoinTokenRequest copyWith(void Function(GetJoinTokenRequest) updates) =>
      super.copyWith((message) => updates(message as GetJoinTokenRequest))
          as GetJoinTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetJoinTokenRequest create() => GetJoinTokenRequest._();
  @$core.override
  GetJoinTokenRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetJoinTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetJoinTokenRequest>(create);
  static GetJoinTokenRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class UpdateVoiceStateRequest extends $pb.GeneratedMessage {
  factory UpdateVoiceStateRequest({
    $core.String? roomId,
    $core.bool? isMuted,
    $core.bool? isDeafened,
    $core.bool? isVideoOn,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (isMuted != null) result.isMuted = isMuted;
    if (isDeafened != null) result.isDeafened = isDeafened;
    if (isVideoOn != null) result.isVideoOn = isVideoOn;
    return result;
  }

  UpdateVoiceStateRequest._();

  factory UpdateVoiceStateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateVoiceStateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateVoiceStateRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOB(2, _omitFieldNames ? '' : 'isMuted')
    ..aOB(3, _omitFieldNames ? '' : 'isDeafened')
    ..aOB(4, _omitFieldNames ? '' : 'isVideoOn')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceStateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceStateRequest copyWith(
          void Function(UpdateVoiceStateRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateVoiceStateRequest))
          as UpdateVoiceStateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateVoiceStateRequest create() => UpdateVoiceStateRequest._();
  @$core.override
  UpdateVoiceStateRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateVoiceStateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateVoiceStateRequest>(create);
  static UpdateVoiceStateRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get isMuted => $_getBF(1);
  @$pb.TagNumber(2)
  set isMuted($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIsMuted() => $_has(1);
  @$pb.TagNumber(2)
  void clearIsMuted() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isDeafened => $_getBF(2);
  @$pb.TagNumber(3)
  set isDeafened($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsDeafened() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsDeafened() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get isVideoOn => $_getBF(3);
  @$pb.TagNumber(4)
  set isVideoOn($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIsVideoOn() => $_has(3);
  @$pb.TagNumber(4)
  void clearIsVideoOn() => $_clearField(4);
}

class GetVoiceStatesRequest extends $pb.GeneratedMessage {
  factory GetVoiceStatesRequest({
    $core.String? roomId,
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  GetVoiceStatesRequest._();

  factory GetVoiceStatesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetVoiceStatesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetVoiceStatesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceStatesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceStatesRequest copyWith(
          void Function(GetVoiceStatesRequest) updates) =>
      super.copyWith((message) => updates(message as GetVoiceStatesRequest))
          as GetVoiceStatesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetVoiceStatesRequest create() => GetVoiceStatesRequest._();
  @$core.override
  GetVoiceStatesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetVoiceStatesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetVoiceStatesRequest>(create);
  static GetVoiceStatesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get voiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set voiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearVoiceRoomId() => $_clearField(2);
}

class VoiceParticipantState extends $pb.GeneratedMessage {
  factory VoiceParticipantState({
    $core.String? profileId,
    $core.bool? isMuted,
    $core.bool? isDeafened,
    $core.bool? isVideoOn,
    $core.bool? isScreenSharing,
    $core.bool? isCommander,
    $core.bool? handRaised,
    $core.bool? hasFloor,
    $core.bool? isBroadcasting,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (isMuted != null) result.isMuted = isMuted;
    if (isDeafened != null) result.isDeafened = isDeafened;
    if (isVideoOn != null) result.isVideoOn = isVideoOn;
    if (isScreenSharing != null) result.isScreenSharing = isScreenSharing;
    if (isCommander != null) result.isCommander = isCommander;
    if (handRaised != null) result.handRaised = handRaised;
    if (hasFloor != null) result.hasFloor = hasFloor;
    if (isBroadcasting != null) result.isBroadcasting = isBroadcasting;
    return result;
  }

  VoiceParticipantState._();

  factory VoiceParticipantState.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceParticipantState.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceParticipantState',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOB(2, _omitFieldNames ? '' : 'isMuted')
    ..aOB(3, _omitFieldNames ? '' : 'isDeafened')
    ..aOB(4, _omitFieldNames ? '' : 'isVideoOn')
    ..aOB(5, _omitFieldNames ? '' : 'isScreenSharing')
    ..aOB(6, _omitFieldNames ? '' : 'isCommander')
    ..aOB(7, _omitFieldNames ? '' : 'handRaised')
    ..aOB(8, _omitFieldNames ? '' : 'hasFloor')
    ..aOB(9, _omitFieldNames ? '' : 'isBroadcasting')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceParticipantState clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceParticipantState copyWith(
          void Function(VoiceParticipantState) updates) =>
      super.copyWith((message) => updates(message as VoiceParticipantState))
          as VoiceParticipantState;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceParticipantState create() => VoiceParticipantState._();
  @$core.override
  VoiceParticipantState createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceParticipantState getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VoiceParticipantState>(create);
  static VoiceParticipantState? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get isMuted => $_getBF(1);
  @$pb.TagNumber(2)
  set isMuted($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIsMuted() => $_has(1);
  @$pb.TagNumber(2)
  void clearIsMuted() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isDeafened => $_getBF(2);
  @$pb.TagNumber(3)
  set isDeafened($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsDeafened() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsDeafened() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get isVideoOn => $_getBF(3);
  @$pb.TagNumber(4)
  set isVideoOn($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIsVideoOn() => $_has(3);
  @$pb.TagNumber(4)
  void clearIsVideoOn() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get isScreenSharing => $_getBF(4);
  @$pb.TagNumber(5)
  set isScreenSharing($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasIsScreenSharing() => $_has(4);
  @$pb.TagNumber(5)
  void clearIsScreenSharing() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get isCommander => $_getBF(5);
  @$pb.TagNumber(6)
  set isCommander($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasIsCommander() => $_has(5);
  @$pb.TagNumber(6)
  void clearIsCommander() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get handRaised => $_getBF(6);
  @$pb.TagNumber(7)
  set handRaised($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasHandRaised() => $_has(6);
  @$pb.TagNumber(7)
  void clearHandRaised() => $_clearField(7);

  /// Organizer granted speak floor (raise-hand / raid-school mode).
  @$pb.TagNumber(8)
  $core.bool get hasFloor => $_getBF(7);
  @$pb.TagNumber(8)
  set hasFloor($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasHasFloor() => $_has(7);
  @$pb.TagNumber(8)
  void clearHasFloor() => $_clearField(8);

  /// Commander is holding "start broadcast" — clients duck other tracks.
  @$pb.TagNumber(9)
  $core.bool get isBroadcasting => $_getBF(8);
  @$pb.TagNumber(9)
  set isBroadcasting($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasIsBroadcasting() => $_has(8);
  @$pb.TagNumber(9)
  void clearIsBroadcasting() => $_clearField(9);
}

class GetActiveCallRequest extends $pb.GeneratedMessage {
  factory GetActiveCallRequest() => create();

  GetActiveCallRequest._();

  factory GetActiveCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetActiveCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetActiveCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveCallRequest copyWith(void Function(GetActiveCallRequest) updates) =>
      super.copyWith((message) => updates(message as GetActiveCallRequest))
          as GetActiveCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetActiveCallRequest create() => GetActiveCallRequest._();
  @$core.override
  GetActiveCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetActiveCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetActiveCallRequest>(create);
  static GetActiveCallRequest? _defaultInstance;
}

class StartScreenShareRequest extends $pb.GeneratedMessage {
  factory StartScreenShareRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  StartScreenShareRequest._();

  factory StartScreenShareRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartScreenShareRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartScreenShareRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartScreenShareRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartScreenShareRequest copyWith(
          void Function(StartScreenShareRequest) updates) =>
      super.copyWith((message) => updates(message as StartScreenShareRequest))
          as StartScreenShareRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartScreenShareRequest create() => StartScreenShareRequest._();
  @$core.override
  StartScreenShareRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartScreenShareRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartScreenShareRequest>(create);
  static StartScreenShareRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class ScreenShareSession extends $pb.GeneratedMessage {
  factory ScreenShareSession({
    $core.String? streamId,
  }) {
    final result = create();
    if (streamId != null) result.streamId = streamId;
    return result;
  }

  ScreenShareSession._();

  factory ScreenShareSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ScreenShareSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ScreenShareSession',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'streamId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScreenShareSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScreenShareSession copyWith(void Function(ScreenShareSession) updates) =>
      super.copyWith((message) => updates(message as ScreenShareSession))
          as ScreenShareSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ScreenShareSession create() => ScreenShareSession._();
  @$core.override
  ScreenShareSession createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ScreenShareSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ScreenShareSession>(create);
  static ScreenShareSession? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get streamId => $_getSZ(0);
  @$pb.TagNumber(1)
  set streamId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStreamId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStreamId() => $_clearField(1);
}

class StopScreenShareRequest extends $pb.GeneratedMessage {
  factory StopScreenShareRequest({
    $core.String? roomId,
    $core.String? streamId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (streamId != null) result.streamId = streamId;
    return result;
  }

  StopScreenShareRequest._();

  factory StopScreenShareRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StopScreenShareRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StopScreenShareRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'streamId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StopScreenShareRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StopScreenShareRequest copyWith(
          void Function(StopScreenShareRequest) updates) =>
      super.copyWith((message) => updates(message as StopScreenShareRequest))
          as StopScreenShareRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StopScreenShareRequest create() => StopScreenShareRequest._();
  @$core.override
  StopScreenShareRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StopScreenShareRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StopScreenShareRequest>(create);
  static StopScreenShareRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get streamId => $_getSZ(1);
  @$pb.TagNumber(2)
  set streamId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStreamId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStreamId() => $_clearField(2);
}

class SetCommanderModeRequest extends $pb.GeneratedMessage {
  factory SetCommanderModeRequest({
    $core.String? roomId,
    $core.bool? enabled,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (enabled != null) result.enabled = enabled;
    return result;
  }

  SetCommanderModeRequest._();

  factory SetCommanderModeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetCommanderModeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetCommanderModeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOB(2, _omitFieldNames ? '' : 'enabled')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCommanderModeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCommanderModeRequest copyWith(
          void Function(SetCommanderModeRequest) updates) =>
      super.copyWith((message) => updates(message as SetCommanderModeRequest))
          as SetCommanderModeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetCommanderModeRequest create() => SetCommanderModeRequest._();
  @$core.override
  SetCommanderModeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetCommanderModeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetCommanderModeRequest>(create);
  static SetCommanderModeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get enabled => $_getBF(1);
  @$pb.TagNumber(2)
  set enabled($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEnabled() => $_has(1);
  @$pb.TagNumber(2)
  void clearEnabled() => $_clearField(2);
}

class SetBroadcastingRequest extends $pb.GeneratedMessage {
  factory SetBroadcastingRequest({
    $core.String? roomId,
    $core.bool? enabled,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (enabled != null) result.enabled = enabled;
    return result;
  }

  SetBroadcastingRequest._();

  factory SetBroadcastingRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetBroadcastingRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetBroadcastingRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOB(2, _omitFieldNames ? '' : 'enabled')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetBroadcastingRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetBroadcastingRequest copyWith(
          void Function(SetBroadcastingRequest) updates) =>
      super.copyWith((message) => updates(message as SetBroadcastingRequest))
          as SetBroadcastingRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetBroadcastingRequest create() => SetBroadcastingRequest._();
  @$core.override
  SetBroadcastingRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetBroadcastingRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetBroadcastingRequest>(create);
  static SetBroadcastingRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get enabled => $_getBF(1);
  @$pb.TagNumber(2)
  set enabled($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEnabled() => $_has(1);
  @$pb.TagNumber(2)
  void clearEnabled() => $_clearField(2);
}

class RaiseHandRequest extends $pb.GeneratedMessage {
  factory RaiseHandRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  RaiseHandRequest._();

  factory RaiseHandRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RaiseHandRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RaiseHandRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RaiseHandRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RaiseHandRequest copyWith(void Function(RaiseHandRequest) updates) =>
      super.copyWith((message) => updates(message as RaiseHandRequest))
          as RaiseHandRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RaiseHandRequest create() => RaiseHandRequest._();
  @$core.override
  RaiseHandRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RaiseHandRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RaiseHandRequest>(create);
  static RaiseHandRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class LowerHandRequest extends $pb.GeneratedMessage {
  factory LowerHandRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  LowerHandRequest._();

  factory LowerHandRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LowerHandRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LowerHandRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LowerHandRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LowerHandRequest copyWith(void Function(LowerHandRequest) updates) =>
      super.copyWith((message) => updates(message as LowerHandRequest))
          as LowerHandRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LowerHandRequest create() => LowerHandRequest._();
  @$core.override
  LowerHandRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LowerHandRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LowerHandRequest>(create);
  static LowerHandRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class GrantFloorRequest extends $pb.GeneratedMessage {
  factory GrantFloorRequest({
    $core.String? roomId,
    $core.String? profileId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GrantFloorRequest._();

  factory GrantFloorRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GrantFloorRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GrantFloorRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GrantFloorRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GrantFloorRequest copyWith(void Function(GrantFloorRequest) updates) =>
      super.copyWith((message) => updates(message as GrantFloorRequest))
          as GrantFloorRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GrantFloorRequest create() => GrantFloorRequest._();
  @$core.override
  GrantFloorRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GrantFloorRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GrantFloorRequest>(create);
  static GrantFloorRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

class RevokeFloorRequest extends $pb.GeneratedMessage {
  factory RevokeFloorRequest({
    $core.String? roomId,
    $core.String? profileId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  RevokeFloorRequest._();

  factory RevokeFloorRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RevokeFloorRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeFloorRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeFloorRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeFloorRequest copyWith(void Function(RevokeFloorRequest) updates) =>
      super.copyWith((message) => updates(message as RevokeFloorRequest))
          as RevokeFloorRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RevokeFloorRequest create() => RevokeFloorRequest._();
  @$core.override
  RevokeFloorRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RevokeFloorRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeFloorRequest>(create);
  static RevokeFloorRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

class StartCallResponse extends $pb.GeneratedMessage {
  factory StartCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  StartCallResponse._();

  factory StartCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCallResponse copyWith(void Function(StartCallResponse) updates) =>
      super.copyWith((message) => updates(message as StartCallResponse))
          as StartCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartCallResponse create() => StartCallResponse._();
  @$core.override
  StartCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartCallResponse>(create);
  static StartCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class AcceptCallResponse extends $pb.GeneratedMessage {
  factory AcceptCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  AcceptCallResponse._();

  factory AcceptCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcceptCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptCallResponse copyWith(void Function(AcceptCallResponse) updates) =>
      super.copyWith((message) => updates(message as AcceptCallResponse))
          as AcceptCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcceptCallResponse create() => AcceptCallResponse._();
  @$core.override
  AcceptCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcceptCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptCallResponse>(create);
  static AcceptCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class DeclineCallResponse extends $pb.GeneratedMessage {
  factory DeclineCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  DeclineCallResponse._();

  factory DeclineCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeclineCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineCallResponse copyWith(void Function(DeclineCallResponse) updates) =>
      super.copyWith((message) => updates(message as DeclineCallResponse))
          as DeclineCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeclineCallResponse create() => DeclineCallResponse._();
  @$core.override
  DeclineCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeclineCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineCallResponse>(create);
  static DeclineCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class JoinCallResponse extends $pb.GeneratedMessage {
  factory JoinCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  JoinCallResponse._();

  factory JoinCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinCallResponse copyWith(void Function(JoinCallResponse) updates) =>
      super.copyWith((message) => updates(message as JoinCallResponse))
          as JoinCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinCallResponse create() => JoinCallResponse._();
  @$core.override
  JoinCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinCallResponse>(create);
  static JoinCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class LeaveCallResponse extends $pb.GeneratedMessage {
  factory LeaveCallResponse() => create();

  LeaveCallResponse._();

  factory LeaveCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveCallResponse copyWith(void Function(LeaveCallResponse) updates) =>
      super.copyWith((message) => updates(message as LeaveCallResponse))
          as LeaveCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveCallResponse create() => LeaveCallResponse._();
  @$core.override
  LeaveCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveCallResponse>(create);
  static LeaveCallResponse? _defaultInstance;
}

class EndCallResponse extends $pb.GeneratedMessage {
  factory EndCallResponse() => create();

  EndCallResponse._();

  factory EndCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EndCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EndCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCallResponse copyWith(void Function(EndCallResponse) updates) =>
      super.copyWith((message) => updates(message as EndCallResponse))
          as EndCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EndCallResponse create() => EndCallResponse._();
  @$core.override
  EndCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EndCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EndCallResponse>(create);
  static EndCallResponse? _defaultInstance;
}

class JoinVoiceRoomResponse extends $pb.GeneratedMessage {
  factory JoinVoiceRoomResponse({
    VoiceSession? voiceSession,
    VoiceRoomLifecycleReceipt? receipt,
  }) {
    final result = create();
    if (voiceSession != null) result.voiceSession = voiceSession;
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  JoinVoiceRoomResponse._();

  factory JoinVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceSession>(1, _omitFieldNames ? '' : 'voiceSession',
        subBuilder: VoiceSession.create)
    ..aOM<VoiceRoomLifecycleReceipt>(2, _omitFieldNames ? '' : 'receipt',
        subBuilder: VoiceRoomLifecycleReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinVoiceRoomResponse copyWith(
          void Function(JoinVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as JoinVoiceRoomResponse))
          as JoinVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinVoiceRoomResponse create() => JoinVoiceRoomResponse._();
  @$core.override
  JoinVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinVoiceRoomResponse>(create);
  static JoinVoiceRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceSession get voiceSession => $_getN(0);
  @$pb.TagNumber(1)
  set voiceSession(VoiceSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceSession() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceSession ensureVoiceSession() => $_ensure(0);

  @$pb.TagNumber(2)
  VoiceRoomLifecycleReceipt get receipt => $_getN(1);
  @$pb.TagNumber(2)
  set receipt(VoiceRoomLifecycleReceipt value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasReceipt() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceipt() => $_clearField(2);
  @$pb.TagNumber(2)
  VoiceRoomLifecycleReceipt ensureReceipt() => $_ensure(1);
}

class LeaveVoiceRoomResponse extends $pb.GeneratedMessage {
  factory LeaveVoiceRoomResponse({
    VoiceRoomLifecycleReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  LeaveVoiceRoomResponse._();

  factory LeaveVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceRoomLifecycleReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: VoiceRoomLifecycleReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveVoiceRoomResponse copyWith(
          void Function(LeaveVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as LeaveVoiceRoomResponse))
          as LeaveVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveVoiceRoomResponse create() => LeaveVoiceRoomResponse._();
  @$core.override
  LeaveVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveVoiceRoomResponse>(create);
  static LeaveVoiceRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceRoomLifecycleReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(VoiceRoomLifecycleReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceRoomLifecycleReceipt ensureReceipt() => $_ensure(0);
}

class MoveToVoiceRoomResponse extends $pb.GeneratedMessage {
  factory MoveToVoiceRoomResponse({
    VoiceSession? voiceSession,
    VoiceRoomLifecycleReceipt? receipt,
  }) {
    final result = create();
    if (voiceSession != null) result.voiceSession = voiceSession;
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  MoveToVoiceRoomResponse._();

  factory MoveToVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MoveToVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MoveToVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceSession>(1, _omitFieldNames ? '' : 'voiceSession',
        subBuilder: VoiceSession.create)
    ..aOM<VoiceRoomLifecycleReceipt>(2, _omitFieldNames ? '' : 'receipt',
        subBuilder: VoiceRoomLifecycleReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveToVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveToVoiceRoomResponse copyWith(
          void Function(MoveToVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as MoveToVoiceRoomResponse))
          as MoveToVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MoveToVoiceRoomResponse create() => MoveToVoiceRoomResponse._();
  @$core.override
  MoveToVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MoveToVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MoveToVoiceRoomResponse>(create);
  static MoveToVoiceRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceSession get voiceSession => $_getN(0);
  @$pb.TagNumber(1)
  set voiceSession(VoiceSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceSession() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceSession ensureVoiceSession() => $_ensure(0);

  @$pb.TagNumber(2)
  VoiceRoomLifecycleReceipt get receipt => $_getN(1);
  @$pb.TagNumber(2)
  set receipt(VoiceRoomLifecycleReceipt value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasReceipt() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceipt() => $_clearField(2);
  @$pb.TagNumber(2)
  VoiceRoomLifecycleReceipt ensureReceipt() => $_ensure(1);
}

class MoveVoiceRoomParticipantResponse extends $pb.GeneratedMessage {
  factory MoveVoiceRoomParticipantResponse({
    VoiceRoomLifecycleReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  MoveVoiceRoomParticipantResponse._();

  factory MoveVoiceRoomParticipantResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MoveVoiceRoomParticipantResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MoveVoiceRoomParticipantResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceRoomLifecycleReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: VoiceRoomLifecycleReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveVoiceRoomParticipantResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveVoiceRoomParticipantResponse copyWith(
          void Function(MoveVoiceRoomParticipantResponse) updates) =>
      super.copyWith(
              (message) => updates(message as MoveVoiceRoomParticipantResponse))
          as MoveVoiceRoomParticipantResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MoveVoiceRoomParticipantResponse create() =>
      MoveVoiceRoomParticipantResponse._();
  @$core.override
  MoveVoiceRoomParticipantResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MoveVoiceRoomParticipantResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MoveVoiceRoomParticipantResponse>(
          create);
  static MoveVoiceRoomParticipantResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceRoomLifecycleReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(VoiceRoomLifecycleReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceRoomLifecycleReceipt ensureReceipt() => $_ensure(0);
}

/// JWT and wall-clock expiry (UTC). Public API uses google.protobuf.Timestamp per docs/REPOSITORIES.md.
class GetJoinTokenResponse extends $pb.GeneratedMessage {
  factory GetJoinTokenResponse({
    $core.String? jwt,
    $2.Timestamp? expiresAt,
    $core.String? livekitUrl,
    $core.String? mediaEpoch,
    $fixnum.Int64? spaceAccessEpoch,
    $fixnum.Int64? rolePolicyEpoch,
    $core.List<$core.int>? authorizationDigest,
  }) {
    final result = create();
    if (jwt != null) result.jwt = jwt;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (livekitUrl != null) result.livekitUrl = livekitUrl;
    if (mediaEpoch != null) result.mediaEpoch = mediaEpoch;
    if (spaceAccessEpoch != null) result.spaceAccessEpoch = spaceAccessEpoch;
    if (rolePolicyEpoch != null) result.rolePolicyEpoch = rolePolicyEpoch;
    if (authorizationDigest != null)
      result.authorizationDigest = authorizationDigest;
    return result;
  }

  GetJoinTokenResponse._();

  factory GetJoinTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetJoinTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetJoinTokenResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'jwt')
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $2.Timestamp.create)
    ..aOS(3, _omitFieldNames ? '' : 'livekitUrl')
    ..aOS(4, _omitFieldNames ? '' : 'mediaEpoch')
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'spaceAccessEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'rolePolicyEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'authorizationDigest', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJoinTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJoinTokenResponse copyWith(void Function(GetJoinTokenResponse) updates) =>
      super.copyWith((message) => updates(message as GetJoinTokenResponse))
          as GetJoinTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetJoinTokenResponse create() => GetJoinTokenResponse._();
  @$core.override
  GetJoinTokenResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetJoinTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetJoinTokenResponse>(create);
  static GetJoinTokenResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get jwt => $_getSZ(0);
  @$pb.TagNumber(1)
  set jwt($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasJwt() => $_has(0);
  @$pb.TagNumber(1)
  void clearJwt() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.Timestamp get expiresAt => $_getN(1);
  @$pb.TagNumber(2)
  set expiresAt($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasExpiresAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpiresAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensureExpiresAt() => $_ensure(1);

  /// WebSocket URL for LiveKit SDK connect (public ingress; not the internal service URL).
  @$pb.TagNumber(3)
  $core.String get livekitUrl => $_getSZ(2);
  @$pb.TagNumber(3)
  set livekitUrl($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLivekitUrl() => $_has(2);
  @$pb.TagNumber(3)
  void clearLivekitUrl() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get mediaEpoch => $_getSZ(3);
  @$pb.TagNumber(4)
  set mediaEpoch($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMediaEpoch() => $_has(3);
  @$pb.TagNumber(4)
  void clearMediaEpoch() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get spaceAccessEpoch => $_getI64(4);
  @$pb.TagNumber(5)
  set spaceAccessEpoch($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSpaceAccessEpoch() => $_has(4);
  @$pb.TagNumber(5)
  void clearSpaceAccessEpoch() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get rolePolicyEpoch => $_getI64(5);
  @$pb.TagNumber(6)
  set rolePolicyEpoch($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRolePolicyEpoch() => $_has(5);
  @$pb.TagNumber(6)
  void clearRolePolicyEpoch() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get authorizationDigest => $_getN(6);
  @$pb.TagNumber(7)
  set authorizationDigest($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasAuthorizationDigest() => $_has(6);
  @$pb.TagNumber(7)
  void clearAuthorizationDigest() => $_clearField(7);
}

class UpdateVoiceStateResponse extends $pb.GeneratedMessage {
  factory UpdateVoiceStateResponse() => create();

  UpdateVoiceStateResponse._();

  factory UpdateVoiceStateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateVoiceStateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateVoiceStateResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceStateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceStateResponse copyWith(
          void Function(UpdateVoiceStateResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateVoiceStateResponse))
          as UpdateVoiceStateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateVoiceStateResponse create() => UpdateVoiceStateResponse._();
  @$core.override
  UpdateVoiceStateResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateVoiceStateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateVoiceStateResponse>(create);
  static UpdateVoiceStateResponse? _defaultInstance;
}

class GetVoiceStatesResponse extends $pb.GeneratedMessage {
  factory GetVoiceStatesResponse({
    $core.Iterable<VoiceParticipantState>? participants,
  }) {
    final result = create();
    if (participants != null) result.participants.addAll(participants);
    return result;
  }

  GetVoiceStatesResponse._();

  factory GetVoiceStatesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetVoiceStatesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetVoiceStatesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..pPM<VoiceParticipantState>(1, _omitFieldNames ? '' : 'participants',
        subBuilder: VoiceParticipantState.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceStatesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceStatesResponse copyWith(
          void Function(GetVoiceStatesResponse) updates) =>
      super.copyWith((message) => updates(message as GetVoiceStatesResponse))
          as GetVoiceStatesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetVoiceStatesResponse create() => GetVoiceStatesResponse._();
  @$core.override
  GetVoiceStatesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetVoiceStatesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetVoiceStatesResponse>(create);
  static GetVoiceStatesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<VoiceParticipantState> get participants => $_getList(0);
}

class GetActiveCallResponse extends $pb.GeneratedMessage {
  factory GetActiveCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  GetActiveCallResponse._();

  factory GetActiveCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetActiveCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetActiveCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveCallResponse copyWith(
          void Function(GetActiveCallResponse) updates) =>
      super.copyWith((message) => updates(message as GetActiveCallResponse))
          as GetActiveCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetActiveCallResponse create() => GetActiveCallResponse._();
  @$core.override
  GetActiveCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetActiveCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetActiveCallResponse>(create);
  static GetActiveCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class StartScreenShareResponse extends $pb.GeneratedMessage {
  factory StartScreenShareResponse({
    ScreenShareSession? screenShareSession,
  }) {
    final result = create();
    if (screenShareSession != null)
      result.screenShareSession = screenShareSession;
    return result;
  }

  StartScreenShareResponse._();

  factory StartScreenShareResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartScreenShareResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartScreenShareResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<ScreenShareSession>(1, _omitFieldNames ? '' : 'screenShareSession',
        subBuilder: ScreenShareSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartScreenShareResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartScreenShareResponse copyWith(
          void Function(StartScreenShareResponse) updates) =>
      super.copyWith((message) => updates(message as StartScreenShareResponse))
          as StartScreenShareResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartScreenShareResponse create() => StartScreenShareResponse._();
  @$core.override
  StartScreenShareResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartScreenShareResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartScreenShareResponse>(create);
  static StartScreenShareResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ScreenShareSession get screenShareSession => $_getN(0);
  @$pb.TagNumber(1)
  set screenShareSession(ScreenShareSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasScreenShareSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearScreenShareSession() => $_clearField(1);
  @$pb.TagNumber(1)
  ScreenShareSession ensureScreenShareSession() => $_ensure(0);
}

class StopScreenShareResponse extends $pb.GeneratedMessage {
  factory StopScreenShareResponse() => create();

  StopScreenShareResponse._();

  factory StopScreenShareResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StopScreenShareResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StopScreenShareResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StopScreenShareResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StopScreenShareResponse copyWith(
          void Function(StopScreenShareResponse) updates) =>
      super.copyWith((message) => updates(message as StopScreenShareResponse))
          as StopScreenShareResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StopScreenShareResponse create() => StopScreenShareResponse._();
  @$core.override
  StopScreenShareResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StopScreenShareResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StopScreenShareResponse>(create);
  static StopScreenShareResponse? _defaultInstance;
}

class SetCommanderModeResponse extends $pb.GeneratedMessage {
  factory SetCommanderModeResponse() => create();

  SetCommanderModeResponse._();

  factory SetCommanderModeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetCommanderModeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetCommanderModeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCommanderModeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCommanderModeResponse copyWith(
          void Function(SetCommanderModeResponse) updates) =>
      super.copyWith((message) => updates(message as SetCommanderModeResponse))
          as SetCommanderModeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetCommanderModeResponse create() => SetCommanderModeResponse._();
  @$core.override
  SetCommanderModeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetCommanderModeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetCommanderModeResponse>(create);
  static SetCommanderModeResponse? _defaultInstance;
}

class SetBroadcastingResponse extends $pb.GeneratedMessage {
  factory SetBroadcastingResponse() => create();

  SetBroadcastingResponse._();

  factory SetBroadcastingResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetBroadcastingResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetBroadcastingResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetBroadcastingResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetBroadcastingResponse copyWith(
          void Function(SetBroadcastingResponse) updates) =>
      super.copyWith((message) => updates(message as SetBroadcastingResponse))
          as SetBroadcastingResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetBroadcastingResponse create() => SetBroadcastingResponse._();
  @$core.override
  SetBroadcastingResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetBroadcastingResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetBroadcastingResponse>(create);
  static SetBroadcastingResponse? _defaultInstance;
}

class RaiseHandResponse extends $pb.GeneratedMessage {
  factory RaiseHandResponse() => create();

  RaiseHandResponse._();

  factory RaiseHandResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RaiseHandResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RaiseHandResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RaiseHandResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RaiseHandResponse copyWith(void Function(RaiseHandResponse) updates) =>
      super.copyWith((message) => updates(message as RaiseHandResponse))
          as RaiseHandResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RaiseHandResponse create() => RaiseHandResponse._();
  @$core.override
  RaiseHandResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RaiseHandResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RaiseHandResponse>(create);
  static RaiseHandResponse? _defaultInstance;
}

class LowerHandResponse extends $pb.GeneratedMessage {
  factory LowerHandResponse() => create();

  LowerHandResponse._();

  factory LowerHandResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LowerHandResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LowerHandResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LowerHandResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LowerHandResponse copyWith(void Function(LowerHandResponse) updates) =>
      super.copyWith((message) => updates(message as LowerHandResponse))
          as LowerHandResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LowerHandResponse create() => LowerHandResponse._();
  @$core.override
  LowerHandResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LowerHandResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LowerHandResponse>(create);
  static LowerHandResponse? _defaultInstance;
}

class GrantFloorResponse extends $pb.GeneratedMessage {
  factory GrantFloorResponse() => create();

  GrantFloorResponse._();

  factory GrantFloorResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GrantFloorResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GrantFloorResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GrantFloorResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GrantFloorResponse copyWith(void Function(GrantFloorResponse) updates) =>
      super.copyWith((message) => updates(message as GrantFloorResponse))
          as GrantFloorResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GrantFloorResponse create() => GrantFloorResponse._();
  @$core.override
  GrantFloorResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GrantFloorResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GrantFloorResponse>(create);
  static GrantFloorResponse? _defaultInstance;
}

class RevokeFloorResponse extends $pb.GeneratedMessage {
  factory RevokeFloorResponse() => create();

  RevokeFloorResponse._();

  factory RevokeFloorResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RevokeFloorResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeFloorResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeFloorResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeFloorResponse copyWith(void Function(RevokeFloorResponse) updates) =>
      super.copyWith((message) => updates(message as RevokeFloorResponse))
          as RevokeFloorResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RevokeFloorResponse create() => RevokeFloorResponse._();
  @$core.override
  RevokeFloorResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RevokeFloorResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeFloorResponse>(create);
  static RevokeFloorResponse? _defaultInstance;
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ApplySpaceLifecycleFenceRequest extends $pb.GeneratedMessage {
  factory ApplySpaceLifecycleFenceRequest({
    $4.SpaceLifecycleFenceRequest? fence,
  }) {
    final result = create();
    if (fence != null) result.fence = fence;
    return result;
  }

  ApplySpaceLifecycleFenceRequest._();

  factory ApplySpaceLifecycleFenceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplySpaceLifecycleFenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplySpaceLifecycleFenceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<$4.SpaceLifecycleFenceRequest>(1, _omitFieldNames ? '' : 'fence',
        subBuilder: $4.SpaceLifecycleFenceRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySpaceLifecycleFenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySpaceLifecycleFenceRequest copyWith(
          void Function(ApplySpaceLifecycleFenceRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ApplySpaceLifecycleFenceRequest))
          as ApplySpaceLifecycleFenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplySpaceLifecycleFenceRequest create() =>
      ApplySpaceLifecycleFenceRequest._();
  @$core.override
  ApplySpaceLifecycleFenceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplySpaceLifecycleFenceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplySpaceLifecycleFenceRequest>(
          create);
  static ApplySpaceLifecycleFenceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $4.SpaceLifecycleFenceRequest get fence => $_getN(0);
  @$pb.TagNumber(1)
  set fence($4.SpaceLifecycleFenceRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFence() => $_has(0);
  @$pb.TagNumber(1)
  void clearFence() => $_clearField(1);
  @$pb.TagNumber(1)
  $4.SpaceLifecycleFenceRequest ensureFence() => $_ensure(0);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class ApplySpaceLifecycleFenceResponse extends $pb.GeneratedMessage {
  factory ApplySpaceLifecycleFenceResponse({
    $4.SpaceLifecycleFenceReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  ApplySpaceLifecycleFenceResponse._();

  factory ApplySpaceLifecycleFenceResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplySpaceLifecycleFenceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplySpaceLifecycleFenceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<$4.SpaceLifecycleFenceReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: $4.SpaceLifecycleFenceReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySpaceLifecycleFenceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySpaceLifecycleFenceResponse copyWith(
          void Function(ApplySpaceLifecycleFenceResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ApplySpaceLifecycleFenceResponse))
          as ApplySpaceLifecycleFenceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplySpaceLifecycleFenceResponse create() =>
      ApplySpaceLifecycleFenceResponse._();
  @$core.override
  ApplySpaceLifecycleFenceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplySpaceLifecycleFenceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplySpaceLifecycleFenceResponse>(
          create);
  static ApplySpaceLifecycleFenceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $4.SpaceLifecycleFenceReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt($4.SpaceLifecycleFenceReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  $4.SpaceLifecycleFenceReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class PurgeSpaceRequest extends $pb.GeneratedMessage {
  factory PurgeSpaceRequest({
    $4.SpacePurgeRequest? purge,
  }) {
    final result = create();
    if (purge != null) result.purge = purge;
    return result;
  }

  PurgeSpaceRequest._();

  factory PurgeSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PurgeSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PurgeSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<$4.SpacePurgeRequest>(1, _omitFieldNames ? '' : 'purge',
        subBuilder: $4.SpacePurgeRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PurgeSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PurgeSpaceRequest copyWith(void Function(PurgeSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as PurgeSpaceRequest))
          as PurgeSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PurgeSpaceRequest create() => PurgeSpaceRequest._();
  @$core.override
  PurgeSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PurgeSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PurgeSpaceRequest>(create);
  static PurgeSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $4.SpacePurgeRequest get purge => $_getN(0);
  @$pb.TagNumber(1)
  set purge($4.SpacePurgeRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPurge() => $_has(0);
  @$pb.TagNumber(1)
  void clearPurge() => $_clearField(1);
  @$pb.TagNumber(1)
  $4.SpacePurgeRequest ensurePurge() => $_ensure(0);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class PurgeSpaceResponse extends $pb.GeneratedMessage {
  factory PurgeSpaceResponse({
    $4.SpacePurgeReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  PurgeSpaceResponse._();

  factory PurgeSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PurgeSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PurgeSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<$4.SpacePurgeReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: $4.SpacePurgeReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PurgeSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PurgeSpaceResponse copyWith(void Function(PurgeSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as PurgeSpaceResponse))
          as PurgeSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PurgeSpaceResponse create() => PurgeSpaceResponse._();
  @$core.override
  PurgeSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PurgeSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PurgeSpaceResponse>(create);
  static PurgeSpaceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $4.SpacePurgeReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt($4.SpacePurgeReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  $4.SpacePurgeReceipt ensureReceipt() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
