// This is a generated file - do not edit.
//
// Generated from voice/realtime/v1/realtime.proto.

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

import '../../chat/v1/chat.pb.dart' as $2;
import '../../common/v1/space_lifecycle.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ApplySpaceLifecycleFenceRequest extends $pb.GeneratedMessage {
  factory ApplySpaceLifecycleFenceRequest({
    $1.SpaceLifecycleFenceRequest? fence,
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
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.realtime.v1'),
      createEmptyInstance: create)
    ..aOM<$1.SpaceLifecycleFenceRequest>(1, _omitFieldNames ? '' : 'fence',
        subBuilder: $1.SpaceLifecycleFenceRequest.create)
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
  $1.SpaceLifecycleFenceRequest get fence => $_getN(0);
  @$pb.TagNumber(1)
  set fence($1.SpaceLifecycleFenceRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFence() => $_has(0);
  @$pb.TagNumber(1)
  void clearFence() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.SpaceLifecycleFenceRequest ensureFence() => $_ensure(0);
}

/// @voice.unknown_fields=accept_preserve
class ApplySpaceLifecycleFenceResponse extends $pb.GeneratedMessage {
  factory ApplySpaceLifecycleFenceResponse({
    $1.SpaceLifecycleFenceReceipt? receipt,
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
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.realtime.v1'),
      createEmptyInstance: create)
    ..aOM<$1.SpaceLifecycleFenceReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: $1.SpaceLifecycleFenceReceipt.create)
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
  $1.SpaceLifecycleFenceReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt($1.SpaceLifecycleFenceReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.SpaceLifecycleFenceReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class PurgeSpaceRequest extends $pb.GeneratedMessage {
  factory PurgeSpaceRequest({
    $1.SpacePurgeRequest? purge,
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
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.realtime.v1'),
      createEmptyInstance: create)
    ..aOM<$1.SpacePurgeRequest>(1, _omitFieldNames ? '' : 'purge',
        subBuilder: $1.SpacePurgeRequest.create)
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
  $1.SpacePurgeRequest get purge => $_getN(0);
  @$pb.TagNumber(1)
  set purge($1.SpacePurgeRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPurge() => $_has(0);
  @$pb.TagNumber(1)
  void clearPurge() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.SpacePurgeRequest ensurePurge() => $_ensure(0);
}

/// @voice.unknown_fields=accept_preserve
class PurgeSpaceResponse extends $pb.GeneratedMessage {
  factory PurgeSpaceResponse({
    $1.SpacePurgeReceipt? receipt,
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
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.realtime.v1'),
      createEmptyInstance: create)
    ..aOM<$1.SpacePurgeReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: $1.SpacePurgeReceipt.create)
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
  $1.SpacePurgeReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt($1.SpacePurgeReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.SpacePurgeReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ImportSpacePurgeManifestPageRequest extends $pb.GeneratedMessage {
  factory ImportSpacePurgeManifestPageRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? scheduleGeneration,
    $2.SpacePurgeManifestPage? page,
    $core.bool? sealsManifest,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (scheduleGeneration != null)
      result.scheduleGeneration = scheduleGeneration;
    if (page != null) result.page = page;
    if (sealsManifest != null) result.sealsManifest = sealsManifest;
    return result;
  }

  ImportSpacePurgeManifestPageRequest._();

  factory ImportSpacePurgeManifestPageRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ImportSpacePurgeManifestPageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportSpacePurgeManifestPageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.realtime.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'scheduleGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$2.SpacePurgeManifestPage>(5, _omitFieldNames ? '' : 'page',
        subBuilder: $2.SpacePurgeManifestPage.create)
    ..aOB(6, _omitFieldNames ? '' : 'sealsManifest')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportSpacePurgeManifestPageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportSpacePurgeManifestPageRequest copyWith(
          void Function(ImportSpacePurgeManifestPageRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ImportSpacePurgeManifestPageRequest))
          as ImportSpacePurgeManifestPageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ImportSpacePurgeManifestPageRequest create() =>
      ImportSpacePurgeManifestPageRequest._();
  @$core.override
  ImportSpacePurgeManifestPageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ImportSpacePurgeManifestPageRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ImportSpacePurgeManifestPageRequest>(create);
  static ImportSpacePurgeManifestPageRequest? _defaultInstance;

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
  $fixnum.Int64 get scheduleGeneration => $_getI64(3);
  @$pb.TagNumber(4)
  set scheduleGeneration($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasScheduleGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearScheduleGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  $2.SpacePurgeManifestPage get page => $_getN(4);
  @$pb.TagNumber(5)
  set page($2.SpacePurgeManifestPage value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPage() => $_has(4);
  @$pb.TagNumber(5)
  void clearPage() => $_clearField(5);
  @$pb.TagNumber(5)
  $2.SpacePurgeManifestPage ensurePage() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.bool get sealsManifest => $_getBF(5);
  @$pb.TagNumber(6)
  set sealsManifest($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSealsManifest() => $_has(5);
  @$pb.TagNumber(6)
  void clearSealsManifest() => $_clearField(6);
}

/// @voice.unknown_fields=accept_preserve
class ImportSpacePurgeManifestPageReceipt extends $pb.GeneratedMessage {
  factory ImportSpacePurgeManifestPageReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? generation,
    $1.ManifestBinding? manifest,
    $fixnum.Int64? pageIndex,
    $fixnum.Int64? acceptedCount,
    $core.List<$core.int>? pageSha256,
    $core.bool? manifestSealed,
    $core.List<$core.int>? requestSha256,
    $3.Timestamp? completedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (generation != null) result.generation = generation;
    if (manifest != null) result.manifest = manifest;
    if (pageIndex != null) result.pageIndex = pageIndex;
    if (acceptedCount != null) result.acceptedCount = acceptedCount;
    if (pageSha256 != null) result.pageSha256 = pageSha256;
    if (manifestSealed != null) result.manifestSealed = manifestSealed;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  ImportSpacePurgeManifestPageReceipt._();

  factory ImportSpacePurgeManifestPageReceipt.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ImportSpacePurgeManifestPageReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportSpacePurgeManifestPageReceipt',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.realtime.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..aOS(4, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$1.ManifestBinding>(6, _omitFieldNames ? '' : 'manifest',
        subBuilder: $1.ManifestBinding.create)
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'pageIndex', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        8, _omitFieldNames ? '' : 'acceptedCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        9, _omitFieldNames ? '' : 'pageSha256', $pb.PbFieldType.OY)
    ..aOB(10, _omitFieldNames ? '' : 'manifestSealed')
    ..a<$core.List<$core.int>>(
        11, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$3.Timestamp>(12, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportSpacePurgeManifestPageReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportSpacePurgeManifestPageReceipt copyWith(
          void Function(ImportSpacePurgeManifestPageReceipt) updates) =>
      super.copyWith((message) =>
              updates(message as ImportSpacePurgeManifestPageReceipt))
          as ImportSpacePurgeManifestPageReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ImportSpacePurgeManifestPageReceipt create() =>
      ImportSpacePurgeManifestPageReceipt._();
  @$core.override
  ImportSpacePurgeManifestPageReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ImportSpacePurgeManifestPageReceipt getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ImportSpacePurgeManifestPageReceipt>(create);
  static ImportSpacePurgeManifestPageReceipt? _defaultInstance;

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
  $1.ManifestBinding get manifest => $_getN(5);
  @$pb.TagNumber(6)
  set manifest($1.ManifestBinding value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasManifest() => $_has(5);
  @$pb.TagNumber(6)
  void clearManifest() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.ManifestBinding ensureManifest() => $_ensure(5);

  @$pb.TagNumber(7)
  $fixnum.Int64 get pageIndex => $_getI64(6);
  @$pb.TagNumber(7)
  set pageIndex($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasPageIndex() => $_has(6);
  @$pb.TagNumber(7)
  void clearPageIndex() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get acceptedCount => $_getI64(7);
  @$pb.TagNumber(8)
  set acceptedCount($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAcceptedCount() => $_has(7);
  @$pb.TagNumber(8)
  void clearAcceptedCount() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.List<$core.int> get pageSha256 => $_getN(8);
  @$pb.TagNumber(9)
  set pageSha256($core.List<$core.int> value) => $_setBytes(8, value);
  @$pb.TagNumber(9)
  $core.bool hasPageSha256() => $_has(8);
  @$pb.TagNumber(9)
  void clearPageSha256() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.bool get manifestSealed => $_getBF(9);
  @$pb.TagNumber(10)
  set manifestSealed($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasManifestSealed() => $_has(9);
  @$pb.TagNumber(10)
  void clearManifestSealed() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.List<$core.int> get requestSha256 => $_getN(10);
  @$pb.TagNumber(11)
  set requestSha256($core.List<$core.int> value) => $_setBytes(10, value);
  @$pb.TagNumber(11)
  $core.bool hasRequestSha256() => $_has(10);
  @$pb.TagNumber(11)
  void clearRequestSha256() => $_clearField(11);

  @$pb.TagNumber(12)
  $3.Timestamp get completedAt => $_getN(11);
  @$pb.TagNumber(12)
  set completedAt($3.Timestamp value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasCompletedAt() => $_has(11);
  @$pb.TagNumber(12)
  void clearCompletedAt() => $_clearField(12);
  @$pb.TagNumber(12)
  $3.Timestamp ensureCompletedAt() => $_ensure(11);
}

class ImportSpacePurgeManifestPageResponse extends $pb.GeneratedMessage {
  factory ImportSpacePurgeManifestPageResponse({
    ImportSpacePurgeManifestPageReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  ImportSpacePurgeManifestPageResponse._();

  factory ImportSpacePurgeManifestPageResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ImportSpacePurgeManifestPageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportSpacePurgeManifestPageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.realtime.v1'),
      createEmptyInstance: create)
    ..aOM<ImportSpacePurgeManifestPageReceipt>(
        1, _omitFieldNames ? '' : 'receipt',
        subBuilder: ImportSpacePurgeManifestPageReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportSpacePurgeManifestPageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportSpacePurgeManifestPageResponse copyWith(
          void Function(ImportSpacePurgeManifestPageResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ImportSpacePurgeManifestPageResponse))
          as ImportSpacePurgeManifestPageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ImportSpacePurgeManifestPageResponse create() =>
      ImportSpacePurgeManifestPageResponse._();
  @$core.override
  ImportSpacePurgeManifestPageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ImportSpacePurgeManifestPageResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ImportSpacePurgeManifestPageResponse>(create);
  static ImportSpacePurgeManifestPageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ImportSpacePurgeManifestPageReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(ImportSpacePurgeManifestPageReceipt value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  ImportSpacePurgeManifestPageReceipt ensureReceipt() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
