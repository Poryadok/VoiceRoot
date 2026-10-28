// This is a generated file - do not edit.
//
// Generated from voice/common/v1/reference_inventory.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'reference_inventory.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'reference_inventory.pbenum.dart';

class ReferenceInventoryIdentity extends $pb.GeneratedMessage {
  factory ReferenceInventoryIdentity({
    $core.int? protocolVersion,
    $core.String? installationId,
    $core.String? restoreCohortId,
    $core.String? migrationId,
    $core.String? fileIncarnationId,
    $core.String? sourceService,
    $core.String? sourceComponent,
    $core.String? ownerIncarnationId,
    $core.int? writerProtocolVersion,
    $core.String? snapshotId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (installationId != null) result.installationId = installationId;
    if (restoreCohortId != null) result.restoreCohortId = restoreCohortId;
    if (migrationId != null) result.migrationId = migrationId;
    if (fileIncarnationId != null) result.fileIncarnationId = fileIncarnationId;
    if (sourceService != null) result.sourceService = sourceService;
    if (sourceComponent != null) result.sourceComponent = sourceComponent;
    if (ownerIncarnationId != null)
      result.ownerIncarnationId = ownerIncarnationId;
    if (writerProtocolVersion != null)
      result.writerProtocolVersion = writerProtocolVersion;
    if (snapshotId != null) result.snapshotId = snapshotId;
    return result;
  }

  ReferenceInventoryIdentity._();

  factory ReferenceInventoryIdentity.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReferenceInventoryIdentity.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReferenceInventoryIdentity',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'installationId')
    ..aOS(3, _omitFieldNames ? '' : 'restoreCohortId')
    ..aOS(4, _omitFieldNames ? '' : 'migrationId')
    ..aOS(5, _omitFieldNames ? '' : 'fileIncarnationId')
    ..aOS(6, _omitFieldNames ? '' : 'sourceService')
    ..aOS(7, _omitFieldNames ? '' : 'sourceComponent')
    ..aOS(8, _omitFieldNames ? '' : 'ownerIncarnationId')
    ..aI(9, _omitFieldNames ? '' : 'writerProtocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(10, _omitFieldNames ? '' : 'snapshotId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryIdentity clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryIdentity copyWith(
          void Function(ReferenceInventoryIdentity) updates) =>
      super.copyWith(
              (message) => updates(message as ReferenceInventoryIdentity))
          as ReferenceInventoryIdentity;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryIdentity create() => ReferenceInventoryIdentity._();
  @$core.override
  ReferenceInventoryIdentity createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryIdentity getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReferenceInventoryIdentity>(create);
  static ReferenceInventoryIdentity? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get installationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set installationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInstallationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearInstallationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get restoreCohortId => $_getSZ(2);
  @$pb.TagNumber(3)
  set restoreCohortId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRestoreCohortId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRestoreCohortId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get migrationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set migrationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMigrationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearMigrationId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get fileIncarnationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set fileIncarnationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasFileIncarnationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearFileIncarnationId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get sourceService => $_getSZ(5);
  @$pb.TagNumber(6)
  set sourceService($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSourceService() => $_has(5);
  @$pb.TagNumber(6)
  void clearSourceService() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get sourceComponent => $_getSZ(6);
  @$pb.TagNumber(7)
  set sourceComponent($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSourceComponent() => $_has(6);
  @$pb.TagNumber(7)
  void clearSourceComponent() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get ownerIncarnationId => $_getSZ(7);
  @$pb.TagNumber(8)
  set ownerIncarnationId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasOwnerIncarnationId() => $_has(7);
  @$pb.TagNumber(8)
  void clearOwnerIncarnationId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.int get writerProtocolVersion => $_getIZ(8);
  @$pb.TagNumber(9)
  set writerProtocolVersion($core.int value) => $_setUnsignedInt32(8, value);
  @$pb.TagNumber(9)
  $core.bool hasWriterProtocolVersion() => $_has(8);
  @$pb.TagNumber(9)
  void clearWriterProtocolVersion() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get snapshotId => $_getSZ(9);
  @$pb.TagNumber(10)
  set snapshotId($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasSnapshotId() => $_has(9);
  @$pb.TagNumber(10)
  void clearSnapshotId() => $_clearField(10);
}

class ReferenceInventoryKey extends $pb.GeneratedMessage {
  factory ReferenceInventoryKey({
    $core.String? fileId,
    ReferenceInventoryOwnerType? ownerType,
    $core.String? ownerId,
    $core.String? subresourceId,
    $core.String? scopeSpaceId,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
    if (ownerType != null) result.ownerType = ownerType;
    if (ownerId != null) result.ownerId = ownerId;
    if (subresourceId != null) result.subresourceId = subresourceId;
    if (scopeSpaceId != null) result.scopeSpaceId = scopeSpaceId;
    return result;
  }

  ReferenceInventoryKey._();

  factory ReferenceInventoryKey.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReferenceInventoryKey.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReferenceInventoryKey',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aE<ReferenceInventoryOwnerType>(2, _omitFieldNames ? '' : 'ownerType',
        enumValues: ReferenceInventoryOwnerType.values)
    ..aOS(3, _omitFieldNames ? '' : 'ownerId')
    ..aOS(4, _omitFieldNames ? '' : 'subresourceId')
    ..aOS(5, _omitFieldNames ? '' : 'scopeSpaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryKey clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryKey copyWith(
          void Function(ReferenceInventoryKey) updates) =>
      super.copyWith((message) => updates(message as ReferenceInventoryKey))
          as ReferenceInventoryKey;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryKey create() => ReferenceInventoryKey._();
  @$core.override
  ReferenceInventoryKey createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryKey getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReferenceInventoryKey>(create);
  static ReferenceInventoryKey? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  ReferenceInventoryOwnerType get ownerType => $_getN(1);
  @$pb.TagNumber(2)
  set ownerType(ReferenceInventoryOwnerType value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOwnerType() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwnerType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get ownerId => $_getSZ(2);
  @$pb.TagNumber(3)
  set ownerId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOwnerId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOwnerId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get subresourceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set subresourceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSubresourceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSubresourceId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get scopeSpaceId => $_getSZ(4);
  @$pb.TagNumber(5)
  set scopeSpaceId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasScopeSpaceId() => $_has(4);
  @$pb.TagNumber(5)
  void clearScopeSpaceId() => $_clearField(5);
}

/// Private protective evidence, never an access capability or a readable ref.
class ReferenceInventoryEntry extends $pb.GeneratedMessage {
  factory ReferenceInventoryEntry({
    ReferenceInventoryKey? key,
    $fixnum.Int64? ownerRevision,
    $core.Iterable<ReferenceInventoryProtectionReason>? reasons,
    $core.Iterable<$core.String>? sourceOperationIds,
    $core.String? authorProfileId,
    $core.String? mediaType,
    $core.String? issueId,
    ReferenceInventoryIssueKind? issueKind,
    $core.bool? retired,
    $core.String? observedFileId,
  }) {
    final result = create();
    if (key != null) result.key = key;
    if (ownerRevision != null) result.ownerRevision = ownerRevision;
    if (reasons != null) result.reasons.addAll(reasons);
    if (sourceOperationIds != null)
      result.sourceOperationIds.addAll(sourceOperationIds);
    if (authorProfileId != null) result.authorProfileId = authorProfileId;
    if (mediaType != null) result.mediaType = mediaType;
    if (issueId != null) result.issueId = issueId;
    if (issueKind != null) result.issueKind = issueKind;
    if (retired != null) result.retired = retired;
    if (observedFileId != null) result.observedFileId = observedFileId;
    return result;
  }

  ReferenceInventoryEntry._();

  factory ReferenceInventoryEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReferenceInventoryEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReferenceInventoryEntry',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryKey>(1, _omitFieldNames ? '' : 'key',
        subBuilder: ReferenceInventoryKey.create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'ownerRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..pc<ReferenceInventoryProtectionReason>(
        3, _omitFieldNames ? '' : 'reasons', $pb.PbFieldType.KE,
        valueOf: ReferenceInventoryProtectionReason.valueOf,
        enumValues: ReferenceInventoryProtectionReason.values,
        defaultEnumValue: ReferenceInventoryProtectionReason
            .REFERENCE_INVENTORY_PROTECTION_REASON_UNSPECIFIED)
    ..pPS(4, _omitFieldNames ? '' : 'sourceOperationIds')
    ..aOS(5, _omitFieldNames ? '' : 'authorProfileId')
    ..aOS(6, _omitFieldNames ? '' : 'mediaType')
    ..aOS(7, _omitFieldNames ? '' : 'issueId')
    ..aE<ReferenceInventoryIssueKind>(8, _omitFieldNames ? '' : 'issueKind',
        enumValues: ReferenceInventoryIssueKind.values)
    ..aOB(9, _omitFieldNames ? '' : 'retired')
    ..aOS(10, _omitFieldNames ? '' : 'observedFileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryEntry copyWith(
          void Function(ReferenceInventoryEntry) updates) =>
      super.copyWith((message) => updates(message as ReferenceInventoryEntry))
          as ReferenceInventoryEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryEntry create() => ReferenceInventoryEntry._();
  @$core.override
  ReferenceInventoryEntry createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryEntry getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReferenceInventoryEntry>(create);
  static ReferenceInventoryEntry? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryKey get key => $_getN(0);
  @$pb.TagNumber(1)
  set key(ReferenceInventoryKey value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearKey() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryKey ensureKey() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get ownerRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set ownerRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOwnerRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwnerRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<ReferenceInventoryProtectionReason> get reasons => $_getList(2);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get sourceOperationIds => $_getList(3);

  @$pb.TagNumber(5)
  $core.String get authorProfileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set authorProfileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAuthorProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearAuthorProfileId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get mediaType => $_getSZ(5);
  @$pb.TagNumber(6)
  set mediaType($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasMediaType() => $_has(5);
  @$pb.TagNumber(6)
  void clearMediaType() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get issueId => $_getSZ(6);
  @$pb.TagNumber(7)
  set issueId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasIssueId() => $_has(6);
  @$pb.TagNumber(7)
  void clearIssueId() => $_clearField(7);

  @$pb.TagNumber(8)
  ReferenceInventoryIssueKind get issueKind => $_getN(7);
  @$pb.TagNumber(8)
  set issueKind(ReferenceInventoryIssueKind value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasIssueKind() => $_has(7);
  @$pb.TagNumber(8)
  void clearIssueKind() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get retired => $_getBF(8);
  @$pb.TagNumber(9)
  set retired($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRetired() => $_has(8);
  @$pb.TagNumber(9)
  void clearRetired() => $_clearField(9);

  /// Known File for an unmappable issue; never invent an owner key for it.
  @$pb.TagNumber(10)
  $core.String get observedFileId => $_getSZ(9);
  @$pb.TagNumber(10)
  set observedFileId($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasObservedFileId() => $_has(9);
  @$pb.TagNumber(10)
  void clearObservedFileId() => $_clearField(10);
}

class ReferenceInventoryPage extends $pb.GeneratedMessage {
  factory ReferenceInventoryPage({
    ReferenceInventoryIdentity? identity,
    $fixnum.Int64? boundaryRevision,
    $core.int? pageIndex,
    $core.Iterable<ReferenceInventoryEntry>? entries,
  }) {
    final result = create();
    if (identity != null) result.identity = identity;
    if (boundaryRevision != null) result.boundaryRevision = boundaryRevision;
    if (pageIndex != null) result.pageIndex = pageIndex;
    if (entries != null) result.entries.addAll(entries);
    return result;
  }

  ReferenceInventoryPage._();

  factory ReferenceInventoryPage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReferenceInventoryPage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReferenceInventoryPage',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryIdentity>(1, _omitFieldNames ? '' : 'identity',
        subBuilder: ReferenceInventoryIdentity.create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'boundaryRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aI(3, _omitFieldNames ? '' : 'pageIndex', fieldType: $pb.PbFieldType.OU3)
    ..pPM<ReferenceInventoryEntry>(4, _omitFieldNames ? '' : 'entries',
        subBuilder: ReferenceInventoryEntry.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryPage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryPage copyWith(
          void Function(ReferenceInventoryPage) updates) =>
      super.copyWith((message) => updates(message as ReferenceInventoryPage))
          as ReferenceInventoryPage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryPage create() => ReferenceInventoryPage._();
  @$core.override
  ReferenceInventoryPage createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryPage getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReferenceInventoryPage>(create);
  static ReferenceInventoryPage? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryIdentity get identity => $_getN(0);
  @$pb.TagNumber(1)
  set identity(ReferenceInventoryIdentity value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIdentity() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdentity() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryIdentity ensureIdentity() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get boundaryRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set boundaryRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBoundaryRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearBoundaryRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get pageIndex => $_getIZ(2);
  @$pb.TagNumber(3)
  set pageIndex($core.int value) => $_setUnsignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageIndex() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageIndex() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<ReferenceInventoryEntry> get entries => $_getList(3);
}

class InventoryPageCommitment extends $pb.GeneratedMessage {
  factory InventoryPageCommitment({
    $core.int? index,
    $core.int? count,
    $core.List<$core.int>? pageSha256,
  }) {
    final result = create();
    if (index != null) result.index = index;
    if (count != null) result.count = count;
    if (pageSha256 != null) result.pageSha256 = pageSha256;
    return result;
  }

  InventoryPageCommitment._();

  factory InventoryPageCommitment.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory InventoryPageCommitment.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InventoryPageCommitment',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'index', fieldType: $pb.PbFieldType.OU3)
    ..aI(2, _omitFieldNames ? '' : 'count', fieldType: $pb.PbFieldType.OU3)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'pageSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InventoryPageCommitment clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InventoryPageCommitment copyWith(
          void Function(InventoryPageCommitment) updates) =>
      super.copyWith((message) => updates(message as InventoryPageCommitment))
          as InventoryPageCommitment;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static InventoryPageCommitment create() => InventoryPageCommitment._();
  @$core.override
  InventoryPageCommitment createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static InventoryPageCommitment getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<InventoryPageCommitment>(create);
  static InventoryPageCommitment? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get index => $_getIZ(0);
  @$pb.TagNumber(1)
  set index($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasIndex() => $_has(0);
  @$pb.TagNumber(1)
  void clearIndex() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get count => $_getIZ(1);
  @$pb.TagNumber(2)
  set count($core.int value) => $_setUnsignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearCount() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get pageSha256 => $_getN(2);
  @$pb.TagNumber(3)
  set pageSha256($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageSha256() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageSha256() => $_clearField(3);
}

class InventoryManifest extends $pb.GeneratedMessage {
  factory InventoryManifest({
    ReferenceInventoryIdentity? identity,
    $fixnum.Int64? boundaryRevision,
    $fixnum.Int64? totalCount,
    $fixnum.Int64? unresolvedCount,
    $core.Iterable<InventoryPageCommitment>? commitments,
  }) {
    final result = create();
    if (identity != null) result.identity = identity;
    if (boundaryRevision != null) result.boundaryRevision = boundaryRevision;
    if (totalCount != null) result.totalCount = totalCount;
    if (unresolvedCount != null) result.unresolvedCount = unresolvedCount;
    if (commitments != null) result.commitments.addAll(commitments);
    return result;
  }

  InventoryManifest._();

  factory InventoryManifest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory InventoryManifest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InventoryManifest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryIdentity>(1, _omitFieldNames ? '' : 'identity',
        subBuilder: ReferenceInventoryIdentity.create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'boundaryRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'totalCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'unresolvedCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..pPM<InventoryPageCommitment>(5, _omitFieldNames ? '' : 'commitments',
        subBuilder: InventoryPageCommitment.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InventoryManifest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InventoryManifest copyWith(void Function(InventoryManifest) updates) =>
      super.copyWith((message) => updates(message as InventoryManifest))
          as InventoryManifest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static InventoryManifest create() => InventoryManifest._();
  @$core.override
  InventoryManifest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static InventoryManifest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<InventoryManifest>(create);
  static InventoryManifest? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryIdentity get identity => $_getN(0);
  @$pb.TagNumber(1)
  set identity(ReferenceInventoryIdentity value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIdentity() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdentity() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryIdentity ensureIdentity() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get boundaryRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set boundaryRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBoundaryRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearBoundaryRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get totalCount => $_getI64(2);
  @$pb.TagNumber(3)
  set totalCount($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalCount() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalCount() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get unresolvedCount => $_getI64(3);
  @$pb.TagNumber(4)
  set unresolvedCount($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasUnresolvedCount() => $_has(3);
  @$pb.TagNumber(4)
  void clearUnresolvedCount() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<InventoryPageCommitment> get commitments => $_getList(4);
}

class ReferenceInventoryDescriptor extends $pb.GeneratedMessage {
  factory ReferenceInventoryDescriptor({
    InventoryManifest? manifest,
    $core.List<$core.int>? manifestSha256,
  }) {
    final result = create();
    if (manifest != null) result.manifest = manifest;
    if (manifestSha256 != null) result.manifestSha256 = manifestSha256;
    return result;
  }

  ReferenceInventoryDescriptor._();

  factory ReferenceInventoryDescriptor.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReferenceInventoryDescriptor.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReferenceInventoryDescriptor',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<InventoryManifest>(1, _omitFieldNames ? '' : 'manifest',
        subBuilder: InventoryManifest.create)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'manifestSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryDescriptor clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryDescriptor copyWith(
          void Function(ReferenceInventoryDescriptor) updates) =>
      super.copyWith(
              (message) => updates(message as ReferenceInventoryDescriptor))
          as ReferenceInventoryDescriptor;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryDescriptor create() =>
      ReferenceInventoryDescriptor._();
  @$core.override
  ReferenceInventoryDescriptor createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryDescriptor getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReferenceInventoryDescriptor>(create);
  static ReferenceInventoryDescriptor? _defaultInstance;

  @$pb.TagNumber(1)
  InventoryManifest get manifest => $_getN(0);
  @$pb.TagNumber(1)
  set manifest(InventoryManifest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasManifest() => $_has(0);
  @$pb.TagNumber(1)
  void clearManifest() => $_clearField(1);
  @$pb.TagNumber(1)
  InventoryManifest ensureManifest() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.List<$core.int> get manifestSha256 => $_getN(1);
  @$pb.TagNumber(2)
  set manifestSha256($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasManifestSha256() => $_has(1);
  @$pb.TagNumber(2)
  void clearManifestSha256() => $_clearField(2);
}

class BeginReferenceInventoryRequest extends $pb.GeneratedMessage {
  factory BeginReferenceInventoryRequest({
    $core.String? operationId,
    ReferenceInventoryIdentity? identity,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (identity != null) result.identity = identity;
    return result;
  }

  BeginReferenceInventoryRequest._();

  factory BeginReferenceInventoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BeginReferenceInventoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BeginReferenceInventoryRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOM<ReferenceInventoryIdentity>(2, _omitFieldNames ? '' : 'identity',
        subBuilder: ReferenceInventoryIdentity.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginReferenceInventoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginReferenceInventoryRequest copyWith(
          void Function(BeginReferenceInventoryRequest) updates) =>
      super.copyWith(
              (message) => updates(message as BeginReferenceInventoryRequest))
          as BeginReferenceInventoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BeginReferenceInventoryRequest create() =>
      BeginReferenceInventoryRequest._();
  @$core.override
  BeginReferenceInventoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BeginReferenceInventoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BeginReferenceInventoryRequest>(create);
  static BeginReferenceInventoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  ReferenceInventoryIdentity get identity => $_getN(1);
  @$pb.TagNumber(2)
  set identity(ReferenceInventoryIdentity value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasIdentity() => $_has(1);
  @$pb.TagNumber(2)
  void clearIdentity() => $_clearField(2);
  @$pb.TagNumber(2)
  ReferenceInventoryIdentity ensureIdentity() => $_ensure(1);
}

class BeginReferenceInventoryResponse extends $pb.GeneratedMessage {
  factory BeginReferenceInventoryResponse({
    ReferenceInventoryDescriptor? descriptor,
  }) {
    final result = create();
    if (descriptor != null) result.descriptor = descriptor;
    return result;
  }

  BeginReferenceInventoryResponse._();

  factory BeginReferenceInventoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BeginReferenceInventoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BeginReferenceInventoryResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryDescriptor>(1, _omitFieldNames ? '' : 'descriptor',
        subBuilder: ReferenceInventoryDescriptor.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginReferenceInventoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginReferenceInventoryResponse copyWith(
          void Function(BeginReferenceInventoryResponse) updates) =>
      super.copyWith(
              (message) => updates(message as BeginReferenceInventoryResponse))
          as BeginReferenceInventoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BeginReferenceInventoryResponse create() =>
      BeginReferenceInventoryResponse._();
  @$core.override
  BeginReferenceInventoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BeginReferenceInventoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BeginReferenceInventoryResponse>(
          create);
  static BeginReferenceInventoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryDescriptor get descriptor => $_getN(0);
  @$pb.TagNumber(1)
  set descriptor(ReferenceInventoryDescriptor value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasDescriptor() => $_has(0);
  @$pb.TagNumber(1)
  void clearDescriptor() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryDescriptor ensureDescriptor() => $_ensure(0);
}

class ReadReferenceInventoryPageRequest extends $pb.GeneratedMessage {
  factory ReadReferenceInventoryPageRequest({
    ReferenceInventoryIdentity? identity,
    $core.int? pageIndex,
  }) {
    final result = create();
    if (identity != null) result.identity = identity;
    if (pageIndex != null) result.pageIndex = pageIndex;
    return result;
  }

  ReadReferenceInventoryPageRequest._();

  factory ReadReferenceInventoryPageRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReferenceInventoryPageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReferenceInventoryPageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryIdentity>(1, _omitFieldNames ? '' : 'identity',
        subBuilder: ReferenceInventoryIdentity.create)
    ..aI(2, _omitFieldNames ? '' : 'pageIndex', fieldType: $pb.PbFieldType.OU3)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryPageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryPageRequest copyWith(
          void Function(ReadReferenceInventoryPageRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ReadReferenceInventoryPageRequest))
          as ReadReferenceInventoryPageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryPageRequest create() =>
      ReadReferenceInventoryPageRequest._();
  @$core.override
  ReadReferenceInventoryPageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryPageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadReferenceInventoryPageRequest>(
          create);
  static ReadReferenceInventoryPageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryIdentity get identity => $_getN(0);
  @$pb.TagNumber(1)
  set identity(ReferenceInventoryIdentity value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIdentity() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdentity() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryIdentity ensureIdentity() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.int get pageIndex => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageIndex($core.int value) => $_setUnsignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageIndex() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageIndex() => $_clearField(2);
}

class ReadReferenceInventoryPageResponse extends $pb.GeneratedMessage {
  factory ReadReferenceInventoryPageResponse({
    ReferenceInventoryPage? page,
  }) {
    final result = create();
    if (page != null) result.page = page;
    return result;
  }

  ReadReferenceInventoryPageResponse._();

  factory ReadReferenceInventoryPageResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReferenceInventoryPageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReferenceInventoryPageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryPage>(1, _omitFieldNames ? '' : 'page',
        subBuilder: ReferenceInventoryPage.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryPageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryPageResponse copyWith(
          void Function(ReadReferenceInventoryPageResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ReadReferenceInventoryPageResponse))
          as ReadReferenceInventoryPageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryPageResponse create() =>
      ReadReferenceInventoryPageResponse._();
  @$core.override
  ReadReferenceInventoryPageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryPageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadReferenceInventoryPageResponse>(
          create);
  static ReadReferenceInventoryPageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryPage get page => $_getN(0);
  @$pb.TagNumber(1)
  set page(ReferenceInventoryPage value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPage() => $_has(0);
  @$pb.TagNumber(1)
  void clearPage() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryPage ensurePage() => $_ensure(0);
}

/// Each revision is complete; its count is checked before advancing a cursor.
class ReferenceInventoryRevision extends $pb.GeneratedMessage {
  factory ReferenceInventoryRevision({
    $fixnum.Int64? revision,
    $core.int? finalCount,
    $core.Iterable<ReferenceInventoryEntry>? entries,
  }) {
    final result = create();
    if (revision != null) result.revision = revision;
    if (finalCount != null) result.finalCount = finalCount;
    if (entries != null) result.entries.addAll(entries);
    return result;
  }

  ReferenceInventoryRevision._();

  factory ReferenceInventoryRevision.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReferenceInventoryRevision.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReferenceInventoryRevision',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'revision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aI(2, _omitFieldNames ? '' : 'finalCount', fieldType: $pb.PbFieldType.OU3)
    ..pPM<ReferenceInventoryEntry>(3, _omitFieldNames ? '' : 'entries',
        subBuilder: ReferenceInventoryEntry.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryRevision clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReferenceInventoryRevision copyWith(
          void Function(ReferenceInventoryRevision) updates) =>
      super.copyWith(
              (message) => updates(message as ReferenceInventoryRevision))
          as ReferenceInventoryRevision;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryRevision create() => ReferenceInventoryRevision._();
  @$core.override
  ReferenceInventoryRevision createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReferenceInventoryRevision getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReferenceInventoryRevision>(create);
  static ReferenceInventoryRevision? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get revision => $_getI64(0);
  @$pb.TagNumber(1)
  set revision($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRevision() => $_has(0);
  @$pb.TagNumber(1)
  void clearRevision() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get finalCount => $_getIZ(1);
  @$pb.TagNumber(2)
  set finalCount($core.int value) => $_setUnsignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFinalCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearFinalCount() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<ReferenceInventoryEntry> get entries => $_getList(2);
}

class ReadReferenceInventoryChangesRequest extends $pb.GeneratedMessage {
  factory ReadReferenceInventoryChangesRequest({
    ReferenceInventoryIdentity? identity,
    $fixnum.Int64? afterRevision,
    $core.int? limit,
  }) {
    final result = create();
    if (identity != null) result.identity = identity;
    if (afterRevision != null) result.afterRevision = afterRevision;
    if (limit != null) result.limit = limit;
    return result;
  }

  ReadReferenceInventoryChangesRequest._();

  factory ReadReferenceInventoryChangesRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReferenceInventoryChangesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReferenceInventoryChangesRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryIdentity>(1, _omitFieldNames ? '' : 'identity',
        subBuilder: ReferenceInventoryIdentity.create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'afterRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aI(3, _omitFieldNames ? '' : 'limit', fieldType: $pb.PbFieldType.OU3)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryChangesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryChangesRequest copyWith(
          void Function(ReadReferenceInventoryChangesRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ReadReferenceInventoryChangesRequest))
          as ReadReferenceInventoryChangesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryChangesRequest create() =>
      ReadReferenceInventoryChangesRequest._();
  @$core.override
  ReadReferenceInventoryChangesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryChangesRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReadReferenceInventoryChangesRequest>(create);
  static ReadReferenceInventoryChangesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryIdentity get identity => $_getN(0);
  @$pb.TagNumber(1)
  set identity(ReferenceInventoryIdentity value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIdentity() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdentity() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryIdentity ensureIdentity() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get afterRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set afterRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAfterRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearAfterRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get limit => $_getIZ(2);
  @$pb.TagNumber(3)
  set limit($core.int value) => $_setUnsignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLimit() => $_has(2);
  @$pb.TagNumber(3)
  void clearLimit() => $_clearField(3);
}

class ReadReferenceInventoryChangesResponse extends $pb.GeneratedMessage {
  factory ReadReferenceInventoryChangesResponse({
    ReferenceInventoryIdentity? identity,
    $fixnum.Int64? retainedFloor,
    $fixnum.Int64? highWatermark,
    $core.Iterable<ReferenceInventoryRevision>? revisions,
  }) {
    final result = create();
    if (identity != null) result.identity = identity;
    if (retainedFloor != null) result.retainedFloor = retainedFloor;
    if (highWatermark != null) result.highWatermark = highWatermark;
    if (revisions != null) result.revisions.addAll(revisions);
    return result;
  }

  ReadReferenceInventoryChangesResponse._();

  factory ReadReferenceInventoryChangesResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReferenceInventoryChangesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReferenceInventoryChangesResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryIdentity>(1, _omitFieldNames ? '' : 'identity',
        subBuilder: ReferenceInventoryIdentity.create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'retainedFloor', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'highWatermark', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..pPM<ReferenceInventoryRevision>(4, _omitFieldNames ? '' : 'revisions',
        subBuilder: ReferenceInventoryRevision.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryChangesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryChangesResponse copyWith(
          void Function(ReadReferenceInventoryChangesResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ReadReferenceInventoryChangesResponse))
          as ReadReferenceInventoryChangesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryChangesResponse create() =>
      ReadReferenceInventoryChangesResponse._();
  @$core.override
  ReadReferenceInventoryChangesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryChangesResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReadReferenceInventoryChangesResponse>(create);
  static ReadReferenceInventoryChangesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryIdentity get identity => $_getN(0);
  @$pb.TagNumber(1)
  set identity(ReferenceInventoryIdentity value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIdentity() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdentity() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryIdentity ensureIdentity() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get retainedFloor => $_getI64(1);
  @$pb.TagNumber(2)
  set retainedFloor($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRetainedFloor() => $_has(1);
  @$pb.TagNumber(2)
  void clearRetainedFloor() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get highWatermark => $_getI64(2);
  @$pb.TagNumber(3)
  set highWatermark($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasHighWatermark() => $_has(2);
  @$pb.TagNumber(3)
  void clearHighWatermark() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<ReferenceInventoryRevision> get revisions => $_getList(3);
}

class SealReferenceInventoryRequest extends $pb.GeneratedMessage {
  factory SealReferenceInventoryRequest({
    $core.String? operationId,
    ReferenceInventoryIdentity? identity,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (identity != null) result.identity = identity;
    return result;
  }

  SealReferenceInventoryRequest._();

  factory SealReferenceInventoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SealReferenceInventoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SealReferenceInventoryRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOM<ReferenceInventoryIdentity>(2, _omitFieldNames ? '' : 'identity',
        subBuilder: ReferenceInventoryIdentity.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SealReferenceInventoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SealReferenceInventoryRequest copyWith(
          void Function(SealReferenceInventoryRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SealReferenceInventoryRequest))
          as SealReferenceInventoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SealReferenceInventoryRequest create() =>
      SealReferenceInventoryRequest._();
  @$core.override
  SealReferenceInventoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SealReferenceInventoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SealReferenceInventoryRequest>(create);
  static SealReferenceInventoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  ReferenceInventoryIdentity get identity => $_getN(1);
  @$pb.TagNumber(2)
  set identity(ReferenceInventoryIdentity value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasIdentity() => $_has(1);
  @$pb.TagNumber(2)
  void clearIdentity() => $_clearField(2);
  @$pb.TagNumber(2)
  ReferenceInventoryIdentity ensureIdentity() => $_ensure(1);
}

class SealReferenceInventoryResponse extends $pb.GeneratedMessage {
  factory SealReferenceInventoryResponse({
    ReferenceInventoryIdentity? identity,
    $fixnum.Int64? cutoverRevision,
    $core.List<$core.int>? writerCoverageSha256,
  }) {
    final result = create();
    if (identity != null) result.identity = identity;
    if (cutoverRevision != null) result.cutoverRevision = cutoverRevision;
    if (writerCoverageSha256 != null)
      result.writerCoverageSha256 = writerCoverageSha256;
    return result;
  }

  SealReferenceInventoryResponse._();

  factory SealReferenceInventoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SealReferenceInventoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SealReferenceInventoryResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOM<ReferenceInventoryIdentity>(1, _omitFieldNames ? '' : 'identity',
        subBuilder: ReferenceInventoryIdentity.create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'cutoverRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'writerCoverageSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SealReferenceInventoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SealReferenceInventoryResponse copyWith(
          void Function(SealReferenceInventoryResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SealReferenceInventoryResponse))
          as SealReferenceInventoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SealReferenceInventoryResponse create() =>
      SealReferenceInventoryResponse._();
  @$core.override
  SealReferenceInventoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SealReferenceInventoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SealReferenceInventoryResponse>(create);
  static SealReferenceInventoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ReferenceInventoryIdentity get identity => $_getN(0);
  @$pb.TagNumber(1)
  set identity(ReferenceInventoryIdentity value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIdentity() => $_has(0);
  @$pb.TagNumber(1)
  void clearIdentity() => $_clearField(1);
  @$pb.TagNumber(1)
  ReferenceInventoryIdentity ensureIdentity() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get cutoverRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set cutoverRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCutoverRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearCutoverRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get writerCoverageSha256 => $_getN(2);
  @$pb.TagNumber(3)
  set writerCoverageSha256($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWriterCoverageSha256() => $_has(2);
  @$pb.TagNumber(3)
  void clearWriterCoverageSha256() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
