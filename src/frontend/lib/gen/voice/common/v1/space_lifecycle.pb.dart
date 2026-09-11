// This is a generated file - do not edit.
//
// Generated from voice/common/v1/space_lifecycle.proto.

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
    as $0;

import 'space_lifecycle.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'space_lifecycle.pbenum.dart';

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ManifestBinding extends $pb.GeneratedMessage {
  factory ManifestBinding({
    $core.String? manifestId,
    $core.List<$core.int>? manifestSha256,
    $fixnum.Int64? itemCount,
  }) {
    final result = create();
    if (manifestId != null) result.manifestId = manifestId;
    if (manifestSha256 != null) result.manifestSha256 = manifestSha256;
    if (itemCount != null) result.itemCount = itemCount;
    return result;
  }

  ManifestBinding._();

  factory ManifestBinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ManifestBinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ManifestBinding',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'manifestId')
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'manifestSha256', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'itemCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ManifestBinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ManifestBinding copyWith(void Function(ManifestBinding) updates) =>
      super.copyWith((message) => updates(message as ManifestBinding))
          as ManifestBinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ManifestBinding create() => ManifestBinding._();
  @$core.override
  ManifestBinding createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ManifestBinding getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ManifestBinding>(create);
  static ManifestBinding? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get manifestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set manifestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasManifestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearManifestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get manifestSha256 => $_getN(1);
  @$pb.TagNumber(2)
  set manifestSha256($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasManifestSha256() => $_has(1);
  @$pb.TagNumber(2)
  void clearManifestSha256() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get itemCount => $_getI64(2);
  @$pb.TagNumber(3)
  set itemCount($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasItemCount() => $_has(2);
  @$pb.TagNumber(3)
  void clearItemCount() => $_clearField(3);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class SpaceLifecycleFenceRequest extends $pb.GeneratedMessage {
  factory SpaceLifecycleFenceRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? generation,
    LifecycleFenceState? desiredState,
    ManifestBinding? manifest,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (generation != null) result.generation = generation;
    if (desiredState != null) result.desiredState = desiredState;
    if (manifest != null) result.manifest = manifest;
    return result;
  }

  SpaceLifecycleFenceRequest._();

  factory SpaceLifecycleFenceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceLifecycleFenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceLifecycleFenceRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<LifecycleFenceState>(5, _omitFieldNames ? '' : 'desiredState',
        enumValues: LifecycleFenceState.values)
    ..aOM<ManifestBinding>(6, _omitFieldNames ? '' : 'manifest',
        subBuilder: ManifestBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceLifecycleFenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceLifecycleFenceRequest copyWith(
          void Function(SpaceLifecycleFenceRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SpaceLifecycleFenceRequest))
          as SpaceLifecycleFenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceLifecycleFenceRequest create() => SpaceLifecycleFenceRequest._();
  @$core.override
  SpaceLifecycleFenceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceLifecycleFenceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceLifecycleFenceRequest>(create);
  static SpaceLifecycleFenceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get deletionOperationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set deletionOperationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeletionOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeletionOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get generation => $_getI64(3);
  @$pb.TagNumber(4)
  set generation($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  LifecycleFenceState get desiredState => $_getN(4);
  @$pb.TagNumber(5)
  set desiredState(LifecycleFenceState value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasDesiredState() => $_has(4);
  @$pb.TagNumber(5)
  void clearDesiredState() => $_clearField(5);

  @$pb.TagNumber(6)
  ManifestBinding get manifest => $_getN(5);
  @$pb.TagNumber(6)
  set manifest(ManifestBinding value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasManifest() => $_has(5);
  @$pb.TagNumber(6)
  void clearManifest() => $_clearField(6);
  @$pb.TagNumber(6)
  ManifestBinding ensureManifest() => $_ensure(5);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class SpaceLifecycleFenceReceipt extends $pb.GeneratedMessage {
  factory SpaceLifecycleFenceReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? generation,
    ParticipantId? participantId,
    LifecycleFenceState? appliedState,
    $core.List<$core.int>? requestSha256,
    $core.List<$core.int>? manifestSha256,
    $0.Timestamp? appliedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (generation != null) result.generation = generation;
    if (participantId != null) result.participantId = participantId;
    if (appliedState != null) result.appliedState = appliedState;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (manifestSha256 != null) result.manifestSha256 = manifestSha256;
    if (appliedAt != null) result.appliedAt = appliedAt;
    return result;
  }

  SpaceLifecycleFenceReceipt._();

  factory SpaceLifecycleFenceReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceLifecycleFenceReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceLifecycleFenceReceipt',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..aOS(4, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<ParticipantId>(6, _omitFieldNames ? '' : 'participantId',
        enumValues: ParticipantId.values)
    ..aE<LifecycleFenceState>(7, _omitFieldNames ? '' : 'appliedState',
        enumValues: LifecycleFenceState.values)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        9, _omitFieldNames ? '' : 'manifestSha256', $pb.PbFieldType.OY)
    ..aOM<$0.Timestamp>(10, _omitFieldNames ? '' : 'appliedAt',
        subBuilder: $0.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceLifecycleFenceReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceLifecycleFenceReceipt copyWith(
          void Function(SpaceLifecycleFenceReceipt) updates) =>
      super.copyWith(
              (message) => updates(message as SpaceLifecycleFenceReceipt))
          as SpaceLifecycleFenceReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceLifecycleFenceReceipt create() => SpaceLifecycleFenceReceipt._();
  @$core.override
  SpaceLifecycleFenceReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceLifecycleFenceReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceLifecycleFenceReceipt>(create);
  static SpaceLifecycleFenceReceipt? _defaultInstance;

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
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deletionOperationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set deletionOperationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeletionOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeletionOperationId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get generation => $_getI64(4);
  @$pb.TagNumber(5)
  set generation($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasGeneration() => $_has(4);
  @$pb.TagNumber(5)
  void clearGeneration() => $_clearField(5);

  @$pb.TagNumber(6)
  ParticipantId get participantId => $_getN(5);
  @$pb.TagNumber(6)
  set participantId(ParticipantId value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasParticipantId() => $_has(5);
  @$pb.TagNumber(6)
  void clearParticipantId() => $_clearField(6);

  @$pb.TagNumber(7)
  LifecycleFenceState get appliedState => $_getN(6);
  @$pb.TagNumber(7)
  set appliedState(LifecycleFenceState value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasAppliedState() => $_has(6);
  @$pb.TagNumber(7)
  void clearAppliedState() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get requestSha256 => $_getN(7);
  @$pb.TagNumber(8)
  set requestSha256($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasRequestSha256() => $_has(7);
  @$pb.TagNumber(8)
  void clearRequestSha256() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.List<$core.int> get manifestSha256 => $_getN(8);
  @$pb.TagNumber(9)
  set manifestSha256($core.List<$core.int> value) => $_setBytes(8, value);
  @$pb.TagNumber(9)
  $core.bool hasManifestSha256() => $_has(8);
  @$pb.TagNumber(9)
  void clearManifestSha256() => $_clearField(9);

  @$pb.TagNumber(10)
  $0.Timestamp get appliedAt => $_getN(9);
  @$pb.TagNumber(10)
  set appliedAt($0.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasAppliedAt() => $_has(9);
  @$pb.TagNumber(10)
  void clearAppliedAt() => $_clearField(10);
  @$pb.TagNumber(10)
  $0.Timestamp ensureAppliedAt() => $_ensure(9);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class SpacePurgeRequest extends $pb.GeneratedMessage {
  factory SpacePurgeRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? generation,
    $0.Timestamp? purgeDecidedAt,
    ParticipantId? participantId,
    ManifestBinding? manifest,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (generation != null) result.generation = generation;
    if (purgeDecidedAt != null) result.purgeDecidedAt = purgeDecidedAt;
    if (participantId != null) result.participantId = participantId;
    if (manifest != null) result.manifest = manifest;
    return result;
  }

  SpacePurgeRequest._();

  factory SpacePurgeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpacePurgeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpacePurgeRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$0.Timestamp>(5, _omitFieldNames ? '' : 'purgeDecidedAt',
        subBuilder: $0.Timestamp.create)
    ..aE<ParticipantId>(6, _omitFieldNames ? '' : 'participantId',
        enumValues: ParticipantId.values)
    ..aOM<ManifestBinding>(7, _omitFieldNames ? '' : 'manifest',
        subBuilder: ManifestBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpacePurgeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpacePurgeRequest copyWith(void Function(SpacePurgeRequest) updates) =>
      super.copyWith((message) => updates(message as SpacePurgeRequest))
          as SpacePurgeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpacePurgeRequest create() => SpacePurgeRequest._();
  @$core.override
  SpacePurgeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpacePurgeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpacePurgeRequest>(create);
  static SpacePurgeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get deletionOperationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set deletionOperationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeletionOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeletionOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get generation => $_getI64(3);
  @$pb.TagNumber(4)
  set generation($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  $0.Timestamp get purgeDecidedAt => $_getN(4);
  @$pb.TagNumber(5)
  set purgeDecidedAt($0.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPurgeDecidedAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearPurgeDecidedAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.Timestamp ensurePurgeDecidedAt() => $_ensure(4);

  @$pb.TagNumber(6)
  ParticipantId get participantId => $_getN(5);
  @$pb.TagNumber(6)
  set participantId(ParticipantId value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasParticipantId() => $_has(5);
  @$pb.TagNumber(6)
  void clearParticipantId() => $_clearField(6);

  @$pb.TagNumber(7)
  ManifestBinding get manifest => $_getN(6);
  @$pb.TagNumber(7)
  set manifest(ManifestBinding value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasManifest() => $_has(6);
  @$pb.TagNumber(7)
  void clearManifest() => $_clearField(7);
  @$pb.TagNumber(7)
  ManifestBinding ensureManifest() => $_ensure(6);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class SpacePurgeReceipt extends $pb.GeneratedMessage {
  factory SpacePurgeReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? generation,
    ParticipantId? participantId,
    PurgeReceiptState? state,
    $core.List<$core.int>? requestSha256,
    $0.Timestamp? completedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (generation != null) result.generation = generation;
    if (participantId != null) result.participantId = participantId;
    if (state != null) result.state = state;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  SpacePurgeReceipt._();

  factory SpacePurgeReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpacePurgeReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpacePurgeReceipt',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..aOS(4, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<ParticipantId>(6, _omitFieldNames ? '' : 'participantId',
        enumValues: ParticipantId.values)
    ..aE<PurgeReceiptState>(7, _omitFieldNames ? '' : 'state',
        enumValues: PurgeReceiptState.values)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$0.Timestamp>(9, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $0.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpacePurgeReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpacePurgeReceipt copyWith(void Function(SpacePurgeReceipt) updates) =>
      super.copyWith((message) => updates(message as SpacePurgeReceipt))
          as SpacePurgeReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpacePurgeReceipt create() => SpacePurgeReceipt._();
  @$core.override
  SpacePurgeReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpacePurgeReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpacePurgeReceipt>(create);
  static SpacePurgeReceipt? _defaultInstance;

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
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deletionOperationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set deletionOperationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeletionOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeletionOperationId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get generation => $_getI64(4);
  @$pb.TagNumber(5)
  set generation($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasGeneration() => $_has(4);
  @$pb.TagNumber(5)
  void clearGeneration() => $_clearField(5);

  @$pb.TagNumber(6)
  ParticipantId get participantId => $_getN(5);
  @$pb.TagNumber(6)
  set participantId(ParticipantId value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasParticipantId() => $_has(5);
  @$pb.TagNumber(6)
  void clearParticipantId() => $_clearField(6);

  @$pb.TagNumber(7)
  PurgeReceiptState get state => $_getN(6);
  @$pb.TagNumber(7)
  set state(PurgeReceiptState value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasState() => $_has(6);
  @$pb.TagNumber(7)
  void clearState() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get requestSha256 => $_getN(7);
  @$pb.TagNumber(8)
  set requestSha256($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasRequestSha256() => $_has(7);
  @$pb.TagNumber(8)
  void clearRequestSha256() => $_clearField(8);

  @$pb.TagNumber(9)
  $0.Timestamp get completedAt => $_getN(8);
  @$pb.TagNumber(9)
  set completedAt($0.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCompletedAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearCompletedAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $0.Timestamp ensureCompletedAt() => $_ensure(8);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
