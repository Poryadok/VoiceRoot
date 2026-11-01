// This is a generated file - do not edit.
//
// Generated from voice/user/v1/user_source_custody.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'user_source_custody.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'user_source_custody.pbenum.dart';

/// Private source-incarnation binding. Epoch is 1..MaxInt64; source_id is 1..128
/// ASCII letters, digits, '.', '_', ':' or '-'; generations are canonical
/// distinct lowercase non-nil UUIDs; keyset_digest is exactly 32 bytes and not all zero. Source ID is not
/// physical-custody proof.
class UserSourcePermitBinding extends $pb.GeneratedMessage {
  factory UserSourcePermitBinding({
    $fixnum.Int64? epoch,
    $core.String? sourceId,
    $core.String? historyGeneration,
    $core.String? userSourceGeneration,
    $core.List<$core.int>? keysetDigest,
  }) {
    final result = create();
    if (epoch != null) result.epoch = epoch;
    if (sourceId != null) result.sourceId = sourceId;
    if (historyGeneration != null) result.historyGeneration = historyGeneration;
    if (userSourceGeneration != null)
      result.userSourceGeneration = userSourceGeneration;
    if (keysetDigest != null) result.keysetDigest = keysetDigest;
    return result;
  }

  UserSourcePermitBinding._();

  factory UserSourcePermitBinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserSourcePermitBinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserSourcePermitBinding',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(1, _omitFieldNames ? '' : 'epoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(2, _omitFieldNames ? '' : 'sourceId')
    ..aOS(3, _omitFieldNames ? '' : 'historyGeneration')
    ..aOS(4, _omitFieldNames ? '' : 'userSourceGeneration')
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'keysetDigest', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserSourcePermitBinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserSourcePermitBinding copyWith(
          void Function(UserSourcePermitBinding) updates) =>
      super.copyWith((message) => updates(message as UserSourcePermitBinding))
          as UserSourcePermitBinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserSourcePermitBinding create() => UserSourcePermitBinding._();
  @$core.override
  UserSourcePermitBinding createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserSourcePermitBinding getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserSourcePermitBinding>(create);
  static UserSourcePermitBinding? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get epoch => $_getI64(0);
  @$pb.TagNumber(1)
  set epoch($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEpoch() => $_has(0);
  @$pb.TagNumber(1)
  void clearEpoch() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sourceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sourceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSourceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSourceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get historyGeneration => $_getSZ(2);
  @$pb.TagNumber(3)
  set historyGeneration($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasHistoryGeneration() => $_has(2);
  @$pb.TagNumber(3)
  void clearHistoryGeneration() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get userSourceGeneration => $_getSZ(3);
  @$pb.TagNumber(4)
  set userSourceGeneration($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasUserSourceGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearUserSourceGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get keysetDigest => $_getN(4);
  @$pb.TagNumber(5)
  set keysetDigest($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasKeysetDigest() => $_has(4);
  @$pb.TagNumber(5)
  void clearKeysetDigest() => $_clearField(5);
}

class AcquireUserSourcePermitRequest extends $pb.GeneratedMessage {
  factory AcquireUserSourcePermitRequest({
    $core.int? protocol,
    UserSourcePermitBinding? binding,
  }) {
    final result = create();
    if (protocol != null) result.protocol = protocol;
    if (binding != null) result.binding = binding;
    return result;
  }

  AcquireUserSourcePermitRequest._();

  factory AcquireUserSourcePermitRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcquireUserSourcePermitRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcquireUserSourcePermitRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocol', fieldType: $pb.PbFieldType.OU3)
    ..aOM<UserSourcePermitBinding>(2, _omitFieldNames ? '' : 'binding',
        subBuilder: UserSourcePermitBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireUserSourcePermitRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireUserSourcePermitRequest copyWith(
          void Function(AcquireUserSourcePermitRequest) updates) =>
      super.copyWith(
              (message) => updates(message as AcquireUserSourcePermitRequest))
          as AcquireUserSourcePermitRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcquireUserSourcePermitRequest create() =>
      AcquireUserSourcePermitRequest._();
  @$core.override
  AcquireUserSourcePermitRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcquireUserSourcePermitRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcquireUserSourcePermitRequest>(create);
  static AcquireUserSourcePermitRequest? _defaultInstance;

  /// 1 is a legacy lock-only grant; 2 additionally binds the physical record.
  /// Unknown fields, missing binding and malformed values are denied.
  @$pb.TagNumber(1)
  $core.int get protocol => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocol($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocol() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocol() => $_clearField(1);

  @$pb.TagNumber(2)
  UserSourcePermitBinding get binding => $_getN(1);
  @$pb.TagNumber(2)
  set binding(UserSourcePermitBinding value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasBinding() => $_has(1);
  @$pb.TagNumber(2)
  void clearBinding() => $_clearField(2);
  @$pb.TagNumber(2)
  UserSourcePermitBinding ensureBinding() => $_ensure(1);
}

/// A controller-held physical record. These values can survive a physical
/// backup clone and are not by themselves proof of exclusive custody.
class UserSourcePhysicalBinding extends $pb.GeneratedMessage {
  factory UserSourcePhysicalBinding({
    $core.String? systemIdentifier,
    $core.String? database,
    $core.int? databaseOid,
  }) {
    final result = create();
    if (systemIdentifier != null) result.systemIdentifier = systemIdentifier;
    if (database != null) result.database = database;
    if (databaseOid != null) result.databaseOid = databaseOid;
    return result;
  }

  UserSourcePhysicalBinding._();

  factory UserSourcePhysicalBinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserSourcePhysicalBinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserSourcePhysicalBinding',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'systemIdentifier')
    ..aOS(2, _omitFieldNames ? '' : 'database')
    ..aI(3, _omitFieldNames ? '' : 'databaseOid',
        fieldType: $pb.PbFieldType.OU3)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserSourcePhysicalBinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserSourcePhysicalBinding copyWith(
          void Function(UserSourcePhysicalBinding) updates) =>
      super.copyWith((message) => updates(message as UserSourcePhysicalBinding))
          as UserSourcePhysicalBinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserSourcePhysicalBinding create() => UserSourcePhysicalBinding._();
  @$core.override
  UserSourcePhysicalBinding createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserSourcePhysicalBinding getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserSourcePhysicalBinding>(create);
  static UserSourcePhysicalBinding? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get systemIdentifier => $_getSZ(0);
  @$pb.TagNumber(1)
  set systemIdentifier($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSystemIdentifier() => $_has(0);
  @$pb.TagNumber(1)
  void clearSystemIdentifier() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get database => $_getSZ(1);
  @$pb.TagNumber(2)
  set database($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDatabase() => $_has(1);
  @$pb.TagNumber(2)
  void clearDatabase() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get databaseOid => $_getIZ(2);
  @$pb.TagNumber(3)
  set databaseOid($core.int value) => $_setUnsignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDatabaseOid() => $_has(2);
  @$pb.TagNumber(3)
  void clearDatabaseOid() => $_clearField(3);
}

class AcquireUserSourcePermitResponse extends $pb.GeneratedMessage {
  factory AcquireUserSourcePermitResponse({
    $core.int? protocol,
    UserSourcePermitBinding? binding,
    $fixnum.Int64? sequence,
    UserSourcePermitFrameKind? kind,
    UserSourcePhysicalBinding? physical,
  }) {
    final result = create();
    if (protocol != null) result.protocol = protocol;
    if (binding != null) result.binding = binding;
    if (sequence != null) result.sequence = sequence;
    if (kind != null) result.kind = kind;
    if (physical != null) result.physical = physical;
    return result;
  }

  AcquireUserSourcePermitResponse._();

  factory AcquireUserSourcePermitResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcquireUserSourcePermitResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcquireUserSourcePermitResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocol', fieldType: $pb.PbFieldType.OU3)
    ..aOM<UserSourcePermitBinding>(2, _omitFieldNames ? '' : 'binding',
        subBuilder: UserSourcePermitBinding.create)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'sequence', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<UserSourcePermitFrameKind>(4, _omitFieldNames ? '' : 'kind',
        enumValues: UserSourcePermitFrameKind.values)
    ..aOM<UserSourcePhysicalBinding>(5, _omitFieldNames ? '' : 'physical',
        subBuilder: UserSourcePhysicalBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireUserSourcePermitResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireUserSourcePermitResponse copyWith(
          void Function(AcquireUserSourcePermitResponse) updates) =>
      super.copyWith(
              (message) => updates(message as AcquireUserSourcePermitResponse))
          as AcquireUserSourcePermitResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcquireUserSourcePermitResponse create() =>
      AcquireUserSourcePermitResponse._();
  @$core.override
  AcquireUserSourcePermitResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcquireUserSourcePermitResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcquireUserSourcePermitResponse>(
          create);
  static AcquireUserSourcePermitResponse? _defaultInstance;

  /// Matches the request protocol and complete binding. The first frame is GRANTED
  /// with sequence 1, then HEARTBEAT with strictly increasing sequence; no
  /// unknown fields, unknown enum, overflow or mismatched binding is accepted.
  @$pb.TagNumber(1)
  $core.int get protocol => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocol($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocol() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocol() => $_clearField(1);

  @$pb.TagNumber(2)
  UserSourcePermitBinding get binding => $_getN(1);
  @$pb.TagNumber(2)
  set binding(UserSourcePermitBinding value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasBinding() => $_has(1);
  @$pb.TagNumber(2)
  void clearBinding() => $_clearField(2);
  @$pb.TagNumber(2)
  UserSourcePermitBinding ensureBinding() => $_ensure(1);

  @$pb.TagNumber(3)
  $fixnum.Int64 get sequence => $_getI64(2);
  @$pb.TagNumber(3)
  set sequence($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSequence() => $_has(2);
  @$pb.TagNumber(3)
  void clearSequence() => $_clearField(3);

  @$pb.TagNumber(4)
  UserSourcePermitFrameKind get kind => $_getN(3);
  @$pb.TagNumber(4)
  set kind(UserSourcePermitFrameKind value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasKind() => $_has(3);
  @$pb.TagNumber(4)
  void clearKind() => $_clearField(4);

  /// Required for protocol 2, absent for protocol 1, unchanged on heartbeats.
  @$pb.TagNumber(5)
  UserSourcePhysicalBinding get physical => $_getN(4);
  @$pb.TagNumber(5)
  set physical(UserSourcePhysicalBinding value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPhysical() => $_has(4);
  @$pb.TagNumber(5)
  void clearPhysical() => $_clearField(5);
  @$pb.TagNumber(5)
  UserSourcePhysicalBinding ensurePhysical() => $_ensure(4);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
