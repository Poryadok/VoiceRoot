// This is a generated file - do not edit.
//
// Generated from voice/user/v1/space_account_capture.proto.

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

import 'space_account_capture.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'space_account_capture.pbenum.dart';

class SpaceOrdinaryPublicationIdentity extends $pb.GeneratedMessage {
  factory SpaceOrdinaryPublicationIdentity({
    $fixnum.Int64? generation,
    $core.String? publicationId,
  }) {
    final result = create();
    if (generation != null) result.generation = generation;
    if (publicationId != null) result.publicationId = publicationId;
    return result;
  }

  SpaceOrdinaryPublicationIdentity._();

  factory SpaceOrdinaryPublicationIdentity.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceOrdinaryPublicationIdentity.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceOrdinaryPublicationIdentity',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(2, _omitFieldNames ? '' : 'publicationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceOrdinaryPublicationIdentity clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceOrdinaryPublicationIdentity copyWith(
          void Function(SpaceOrdinaryPublicationIdentity) updates) =>
      super.copyWith(
              (message) => updates(message as SpaceOrdinaryPublicationIdentity))
          as SpaceOrdinaryPublicationIdentity;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceOrdinaryPublicationIdentity create() =>
      SpaceOrdinaryPublicationIdentity._();
  @$core.override
  SpaceOrdinaryPublicationIdentity createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceOrdinaryPublicationIdentity getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceOrdinaryPublicationIdentity>(
          create);
  static SpaceOrdinaryPublicationIdentity? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get generation => $_getI64(0);
  @$pb.TagNumber(1)
  set generation($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get publicationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set publicationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPublicationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublicationId() => $_clearField(2);
}

class SpaceCommunityPublicationIdentity extends $pb.GeneratedMessage {
  factory SpaceCommunityPublicationIdentity({
    $fixnum.Int64? ownerGeneration,
    $fixnum.Int64? memberGeneration,
    $core.String? episodeId,
  }) {
    final result = create();
    if (ownerGeneration != null) result.ownerGeneration = ownerGeneration;
    if (memberGeneration != null) result.memberGeneration = memberGeneration;
    if (episodeId != null) result.episodeId = episodeId;
    return result;
  }

  SpaceCommunityPublicationIdentity._();

  factory SpaceCommunityPublicationIdentity.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceCommunityPublicationIdentity.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceCommunityPublicationIdentity',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'ownerGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'memberGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(3, _omitFieldNames ? '' : 'episodeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceCommunityPublicationIdentity clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceCommunityPublicationIdentity copyWith(
          void Function(SpaceCommunityPublicationIdentity) updates) =>
      super.copyWith((message) =>
              updates(message as SpaceCommunityPublicationIdentity))
          as SpaceCommunityPublicationIdentity;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceCommunityPublicationIdentity create() =>
      SpaceCommunityPublicationIdentity._();
  @$core.override
  SpaceCommunityPublicationIdentity createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceCommunityPublicationIdentity getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceCommunityPublicationIdentity>(
          create);
  static SpaceCommunityPublicationIdentity? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get ownerGeneration => $_getI64(0);
  @$pb.TagNumber(1)
  set ownerGeneration($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOwnerGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearOwnerGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get memberGeneration => $_getI64(1);
  @$pb.TagNumber(2)
  set memberGeneration($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMemberGeneration() => $_has(1);
  @$pb.TagNumber(2)
  void clearMemberGeneration() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get episodeId => $_getSZ(2);
  @$pb.TagNumber(3)
  set episodeId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEpisodeId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEpisodeId() => $_clearField(3);
}

class SpaceProspectiveCommunityIdentity extends $pb.GeneratedMessage {
  factory SpaceProspectiveCommunityIdentity({
    $core.String? operationId,
    $fixnum.Int64? ownerGeneration,
    $fixnum.Int64? sourceRevision,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (ownerGeneration != null) result.ownerGeneration = ownerGeneration;
    if (sourceRevision != null) result.sourceRevision = sourceRevision;
    return result;
  }

  SpaceProspectiveCommunityIdentity._();

  factory SpaceProspectiveCommunityIdentity.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceProspectiveCommunityIdentity.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceProspectiveCommunityIdentity',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'ownerGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'sourceRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceProspectiveCommunityIdentity clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceProspectiveCommunityIdentity copyWith(
          void Function(SpaceProspectiveCommunityIdentity) updates) =>
      super.copyWith((message) =>
              updates(message as SpaceProspectiveCommunityIdentity))
          as SpaceProspectiveCommunityIdentity;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceProspectiveCommunityIdentity create() =>
      SpaceProspectiveCommunityIdentity._();
  @$core.override
  SpaceProspectiveCommunityIdentity createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceProspectiveCommunityIdentity getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceProspectiveCommunityIdentity>(
          create);
  static SpaceProspectiveCommunityIdentity? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get ownerGeneration => $_getI64(1);
  @$pb.TagNumber(2)
  set ownerGeneration($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOwnerGeneration() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwnerGeneration() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get sourceRevision => $_getI64(2);
  @$pb.TagNumber(3)
  set sourceRevision($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSourceRevision() => $_has(2);
  @$pb.TagNumber(3)
  void clearSourceRevision() => $_clearField(3);
}

enum SpaceAccountCaptureEntry_Source {
  ordinary,
  community,
  prospectiveCommunity,
  notSet
}

class SpaceAccountCaptureEntry extends $pb.GeneratedMessage {
  factory SpaceAccountCaptureEntry({
    $core.String? profileId,
    SpaceOrdinaryPublicationIdentity? ordinary,
    SpaceCommunityPublicationIdentity? community,
    SpaceProspectiveCommunityIdentity? prospectiveCommunity,
    $core.String? expectedAssociationId,
    $fixnum.Int64? expectedAssociationRevision,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (ordinary != null) result.ordinary = ordinary;
    if (community != null) result.community = community;
    if (prospectiveCommunity != null)
      result.prospectiveCommunity = prospectiveCommunity;
    if (expectedAssociationId != null)
      result.expectedAssociationId = expectedAssociationId;
    if (expectedAssociationRevision != null)
      result.expectedAssociationRevision = expectedAssociationRevision;
    return result;
  }

  SpaceAccountCaptureEntry._();

  factory SpaceAccountCaptureEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceAccountCaptureEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, SpaceAccountCaptureEntry_Source>
      _SpaceAccountCaptureEntry_SourceByTag = {
    2: SpaceAccountCaptureEntry_Source.ordinary,
    3: SpaceAccountCaptureEntry_Source.community,
    4: SpaceAccountCaptureEntry_Source.prospectiveCommunity,
    0: SpaceAccountCaptureEntry_Source.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceAccountCaptureEntry',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..oo(0, [2, 3, 4])
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOM<SpaceOrdinaryPublicationIdentity>(
        2, _omitFieldNames ? '' : 'ordinary',
        subBuilder: SpaceOrdinaryPublicationIdentity.create)
    ..aOM<SpaceCommunityPublicationIdentity>(
        3, _omitFieldNames ? '' : 'community',
        subBuilder: SpaceCommunityPublicationIdentity.create)
    ..aOM<SpaceProspectiveCommunityIdentity>(
        4, _omitFieldNames ? '' : 'prospectiveCommunity',
        subBuilder: SpaceProspectiveCommunityIdentity.create)
    ..aOS(5, _omitFieldNames ? '' : 'expectedAssociationId')
    ..a<$fixnum.Int64>(6, _omitFieldNames ? '' : 'expectedAssociationRevision',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceAccountCaptureEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceAccountCaptureEntry copyWith(
          void Function(SpaceAccountCaptureEntry) updates) =>
      super.copyWith((message) => updates(message as SpaceAccountCaptureEntry))
          as SpaceAccountCaptureEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceAccountCaptureEntry create() => SpaceAccountCaptureEntry._();
  @$core.override
  SpaceAccountCaptureEntry createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceAccountCaptureEntry getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceAccountCaptureEntry>(create);
  static SpaceAccountCaptureEntry? _defaultInstance;

  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  SpaceAccountCaptureEntry_Source whichSource() =>
      _SpaceAccountCaptureEntry_SourceByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  void clearSource() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  SpaceOrdinaryPublicationIdentity get ordinary => $_getN(1);
  @$pb.TagNumber(2)
  set ordinary(SpaceOrdinaryPublicationIdentity value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOrdinary() => $_has(1);
  @$pb.TagNumber(2)
  void clearOrdinary() => $_clearField(2);
  @$pb.TagNumber(2)
  SpaceOrdinaryPublicationIdentity ensureOrdinary() => $_ensure(1);

  @$pb.TagNumber(3)
  SpaceCommunityPublicationIdentity get community => $_getN(2);
  @$pb.TagNumber(3)
  set community(SpaceCommunityPublicationIdentity value) =>
      $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCommunity() => $_has(2);
  @$pb.TagNumber(3)
  void clearCommunity() => $_clearField(3);
  @$pb.TagNumber(3)
  SpaceCommunityPublicationIdentity ensureCommunity() => $_ensure(2);

  @$pb.TagNumber(4)
  SpaceProspectiveCommunityIdentity get prospectiveCommunity => $_getN(3);
  @$pb.TagNumber(4)
  set prospectiveCommunity(SpaceProspectiveCommunityIdentity value) =>
      $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasProspectiveCommunity() => $_has(3);
  @$pb.TagNumber(4)
  void clearProspectiveCommunity() => $_clearField(4);
  @$pb.TagNumber(4)
  SpaceProspectiveCommunityIdentity ensureProspectiveCommunity() => $_ensure(3);

  /// Both absent, or both present. They never select a different association.
  @$pb.TagNumber(5)
  $core.String get expectedAssociationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set expectedAssociationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpectedAssociationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpectedAssociationId() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get expectedAssociationRevision => $_getI64(5);
  @$pb.TagNumber(6)
  set expectedAssociationRevision($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasExpectedAssociationRevision() => $_has(5);
  @$pb.TagNumber(6)
  void clearExpectedAssociationRevision() => $_clearField(6);
}

class SpaceAccountCaptureCohort extends $pb.GeneratedMessage {
  factory SpaceAccountCaptureCohort({
    $core.Iterable<SpaceAccountCaptureEntry>? entries,
  }) {
    final result = create();
    if (entries != null) result.entries.addAll(entries);
    return result;
  }

  SpaceAccountCaptureCohort._();

  factory SpaceAccountCaptureCohort.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceAccountCaptureCohort.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceAccountCaptureCohort',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..pPM<SpaceAccountCaptureEntry>(1, _omitFieldNames ? '' : 'entries',
        subBuilder: SpaceAccountCaptureEntry.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceAccountCaptureCohort clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceAccountCaptureCohort copyWith(
          void Function(SpaceAccountCaptureCohort) updates) =>
      super.copyWith((message) => updates(message as SpaceAccountCaptureCohort))
          as SpaceAccountCaptureCohort;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceAccountCaptureCohort create() => SpaceAccountCaptureCohort._();
  @$core.override
  SpaceAccountCaptureCohort createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceAccountCaptureCohort getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceAccountCaptureCohort>(create);
  static SpaceAccountCaptureCohort? _defaultInstance;

  /// Sorted lexicographically by deterministic protobuf bytes; no duplicate
  /// (profile_id, source tuple), even with different expected associations.
  @$pb.TagNumber(1)
  $pb.PbList<SpaceAccountCaptureEntry> get entries => $_getList(0);
}

class CaptureSpaceMemberAccountBindingsRequest extends $pb.GeneratedMessage {
  factory CaptureSpaceMemberAccountBindingsRequest({
    $core.int? protocol,
    $core.String? operationId,
    SpaceAccountCapturePurpose? purpose,
    $core.String? spaceId,
    $core.List<$core.int>? cohortDigest,
    SpaceAccountCaptureCohort? cohort,
  }) {
    final result = create();
    if (protocol != null) result.protocol = protocol;
    if (operationId != null) result.operationId = operationId;
    if (purpose != null) result.purpose = purpose;
    if (spaceId != null) result.spaceId = spaceId;
    if (cohortDigest != null) result.cohortDigest = cohortDigest;
    if (cohort != null) result.cohort = cohort;
    return result;
  }

  CaptureSpaceMemberAccountBindingsRequest._();

  factory CaptureSpaceMemberAccountBindingsRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CaptureSpaceMemberAccountBindingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CaptureSpaceMemberAccountBindingsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocol', fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aE<SpaceAccountCapturePurpose>(3, _omitFieldNames ? '' : 'purpose',
        enumValues: SpaceAccountCapturePurpose.values)
    ..aOS(4, _omitFieldNames ? '' : 'spaceId')
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'cohortDigest', $pb.PbFieldType.OY)
    ..aOM<SpaceAccountCaptureCohort>(6, _omitFieldNames ? '' : 'cohort',
        subBuilder: SpaceAccountCaptureCohort.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CaptureSpaceMemberAccountBindingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CaptureSpaceMemberAccountBindingsRequest copyWith(
          void Function(CaptureSpaceMemberAccountBindingsRequest) updates) =>
      super.copyWith((message) =>
              updates(message as CaptureSpaceMemberAccountBindingsRequest))
          as CaptureSpaceMemberAccountBindingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CaptureSpaceMemberAccountBindingsRequest create() =>
      CaptureSpaceMemberAccountBindingsRequest._();
  @$core.override
  CaptureSpaceMemberAccountBindingsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CaptureSpaceMemberAccountBindingsRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          CaptureSpaceMemberAccountBindingsRequest>(create);
  static CaptureSpaceMemberAccountBindingsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocol => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocol($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocol() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocol() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get operationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  SpaceAccountCapturePurpose get purpose => $_getN(2);
  @$pb.TagNumber(3)
  set purpose(SpaceAccountCapturePurpose value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPurpose() => $_has(2);
  @$pb.TagNumber(3)
  void clearPurpose() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get spaceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set spaceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSpaceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpaceId() => $_clearField(4);

  /// SHA256("voice.space.account-binding.v1" + NUL + deterministic cohort bytes).
  @$pb.TagNumber(5)
  $core.List<$core.int> get cohortDigest => $_getN(4);
  @$pb.TagNumber(5)
  set cohortDigest($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCohortDigest() => $_has(4);
  @$pb.TagNumber(5)
  void clearCohortDigest() => $_clearField(5);

  /// The complete immutable manifest, never an implicit first page. A request
  /// exceeding transport limits fails; no truncated response is a capture.
  @$pb.TagNumber(6)
  SpaceAccountCaptureCohort get cohort => $_getN(5);
  @$pb.TagNumber(6)
  set cohort(SpaceAccountCaptureCohort value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCohort() => $_has(5);
  @$pb.TagNumber(6)
  void clearCohort() => $_clearField(6);
  @$pb.TagNumber(6)
  SpaceAccountCaptureCohort ensureCohort() => $_ensure(5);
}

/// A minimized committed User identity stream, independent of raw-source clock A.
class SpaceAccountIdentityCut extends $pb.GeneratedMessage {
  factory SpaceAccountIdentityCut({
    $core.String? owner,
    $core.String? stream,
    $core.String? historyGeneration,
    $fixnum.Int64? throughRevision,
    $fixnum.Int64? keysetRevision,
    $core.List<$core.int>? digest,
    $fixnum.Int64? retainedFloor,
  }) {
    final result = create();
    if (owner != null) result.owner = owner;
    if (stream != null) result.stream = stream;
    if (historyGeneration != null) result.historyGeneration = historyGeneration;
    if (throughRevision != null) result.throughRevision = throughRevision;
    if (keysetRevision != null) result.keysetRevision = keysetRevision;
    if (digest != null) result.digest = digest;
    if (retainedFloor != null) result.retainedFloor = retainedFloor;
    return result;
  }

  SpaceAccountIdentityCut._();

  factory SpaceAccountIdentityCut.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceAccountIdentityCut.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceAccountIdentityCut',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'owner')
    ..aOS(2, _omitFieldNames ? '' : 'stream')
    ..aOS(3, _omitFieldNames ? '' : 'historyGeneration')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'throughRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'keysetRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'digest', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'retainedFloor', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceAccountIdentityCut clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceAccountIdentityCut copyWith(
          void Function(SpaceAccountIdentityCut) updates) =>
      super.copyWith((message) => updates(message as SpaceAccountIdentityCut))
          as SpaceAccountIdentityCut;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceAccountIdentityCut create() => SpaceAccountIdentityCut._();
  @$core.override
  SpaceAccountIdentityCut createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceAccountIdentityCut getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceAccountIdentityCut>(create);
  static SpaceAccountIdentityCut? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get owner => $_getSZ(0);
  @$pb.TagNumber(1)
  set owner($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOwner() => $_has(0);
  @$pb.TagNumber(1)
  void clearOwner() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get stream => $_getSZ(1);
  @$pb.TagNumber(2)
  set stream($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStream() => $_has(1);
  @$pb.TagNumber(2)
  void clearStream() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get historyGeneration => $_getSZ(2);
  @$pb.TagNumber(3)
  set historyGeneration($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasHistoryGeneration() => $_has(2);
  @$pb.TagNumber(3)
  void clearHistoryGeneration() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get throughRevision => $_getI64(3);
  @$pb.TagNumber(4)
  set throughRevision($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasThroughRevision() => $_has(3);
  @$pb.TagNumber(4)
  void clearThroughRevision() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get keysetRevision => $_getI64(4);
  @$pb.TagNumber(5)
  set keysetRevision($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasKeysetRevision() => $_has(4);
  @$pb.TagNumber(5)
  void clearKeysetRevision() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get digest => $_getN(5);
  @$pb.TagNumber(6)
  set digest($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDigest() => $_has(5);
  @$pb.TagNumber(6)
  void clearDigest() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get retainedFloor => $_getI64(6);
  @$pb.TagNumber(7)
  set retainedFloor($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRetainedFloor() => $_has(6);
  @$pb.TagNumber(7)
  void clearRetainedFloor() => $_clearField(7);
}

class SpaceAccountCaptureResult extends $pb.GeneratedMessage {
  factory SpaceAccountCaptureResult({
    SpaceAccountCaptureEntry? entry,
    SpaceAccountCaptureOutcome? outcome,
    $core.String? accountId,
    $core.String? associationId,
    $fixnum.Int64? associationRevision,
    $fixnum.Int64? profileRevision,
    $core.bool? deleted,
    $core.bool? frozen,
    SpaceAccountCaptureOrigin? origin,
    $core.String? ownerReceiptId,
    $core.List<$core.int>? ownerReceiptDigest,
    $core.String? terminalOperationId,
    $fixnum.Int64? terminalRevision,
    $core.String? terminalReceiptId,
    $core.List<$core.int>? terminalReceiptDigest,
    $0.Timestamp? captureCut,
    SpaceAccountAssociationOrigin? associationOrigin,
    $core.bool? accountInactive,
  }) {
    final result = create();
    if (entry != null) result.entry = entry;
    if (outcome != null) result.outcome = outcome;
    if (accountId != null) result.accountId = accountId;
    if (associationId != null) result.associationId = associationId;
    if (associationRevision != null)
      result.associationRevision = associationRevision;
    if (profileRevision != null) result.profileRevision = profileRevision;
    if (deleted != null) result.deleted = deleted;
    if (frozen != null) result.frozen = frozen;
    if (origin != null) result.origin = origin;
    if (ownerReceiptId != null) result.ownerReceiptId = ownerReceiptId;
    if (ownerReceiptDigest != null)
      result.ownerReceiptDigest = ownerReceiptDigest;
    if (terminalOperationId != null)
      result.terminalOperationId = terminalOperationId;
    if (terminalRevision != null) result.terminalRevision = terminalRevision;
    if (terminalReceiptId != null) result.terminalReceiptId = terminalReceiptId;
    if (terminalReceiptDigest != null)
      result.terminalReceiptDigest = terminalReceiptDigest;
    if (captureCut != null) result.captureCut = captureCut;
    if (associationOrigin != null) result.associationOrigin = associationOrigin;
    if (accountInactive != null) result.accountInactive = accountInactive;
    return result;
  }

  SpaceAccountCaptureResult._();

  factory SpaceAccountCaptureResult.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceAccountCaptureResult.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceAccountCaptureResult',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceAccountCaptureEntry>(1, _omitFieldNames ? '' : 'entry',
        subBuilder: SpaceAccountCaptureEntry.create)
    ..aE<SpaceAccountCaptureOutcome>(2, _omitFieldNames ? '' : 'outcome',
        enumValues: SpaceAccountCaptureOutcome.values)
    ..aOS(3, _omitFieldNames ? '' : 'accountId')
    ..aOS(4, _omitFieldNames ? '' : 'associationId')
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'associationRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'profileRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOB(7, _omitFieldNames ? '' : 'deleted')
    ..aOB(8, _omitFieldNames ? '' : 'frozen')
    ..aE<SpaceAccountCaptureOrigin>(9, _omitFieldNames ? '' : 'origin',
        enumValues: SpaceAccountCaptureOrigin.values)
    ..aOS(10, _omitFieldNames ? '' : 'ownerReceiptId')
    ..a<$core.List<$core.int>>(
        11, _omitFieldNames ? '' : 'ownerReceiptDigest', $pb.PbFieldType.OY)
    ..aOS(12, _omitFieldNames ? '' : 'terminalOperationId')
    ..a<$fixnum.Int64>(
        13, _omitFieldNames ? '' : 'terminalRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(14, _omitFieldNames ? '' : 'terminalReceiptId')
    ..a<$core.List<$core.int>>(
        15, _omitFieldNames ? '' : 'terminalReceiptDigest', $pb.PbFieldType.OY)
    ..aOM<$0.Timestamp>(16, _omitFieldNames ? '' : 'captureCut',
        subBuilder: $0.Timestamp.create)
    ..aE<SpaceAccountAssociationOrigin>(
        17, _omitFieldNames ? '' : 'associationOrigin',
        enumValues: SpaceAccountAssociationOrigin.values)
    ..aOB(18, _omitFieldNames ? '' : 'accountInactive')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceAccountCaptureResult clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceAccountCaptureResult copyWith(
          void Function(SpaceAccountCaptureResult) updates) =>
      super.copyWith((message) => updates(message as SpaceAccountCaptureResult))
          as SpaceAccountCaptureResult;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceAccountCaptureResult create() => SpaceAccountCaptureResult._();
  @$core.override
  SpaceAccountCaptureResult createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceAccountCaptureResult getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceAccountCaptureResult>(create);
  static SpaceAccountCaptureResult? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceAccountCaptureEntry get entry => $_getN(0);
  @$pb.TagNumber(1)
  set entry(SpaceAccountCaptureEntry value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasEntry() => $_has(0);
  @$pb.TagNumber(1)
  void clearEntry() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceAccountCaptureEntry ensureEntry() => $_ensure(0);

  @$pb.TagNumber(2)
  SpaceAccountCaptureOutcome get outcome => $_getN(1);
  @$pb.TagNumber(2)
  set outcome(SpaceAccountCaptureOutcome value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOutcome() => $_has(1);
  @$pb.TagNumber(2)
  void clearOutcome() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get accountId => $_getSZ(2);
  @$pb.TagNumber(3)
  set accountId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAccountId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAccountId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get associationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set associationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAssociationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearAssociationId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get associationRevision => $_getI64(4);
  @$pb.TagNumber(5)
  set associationRevision($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAssociationRevision() => $_has(4);
  @$pb.TagNumber(5)
  void clearAssociationRevision() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get profileRevision => $_getI64(5);
  @$pb.TagNumber(6)
  set profileRevision($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasProfileRevision() => $_has(5);
  @$pb.TagNumber(6)
  void clearProfileRevision() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get deleted => $_getBF(6);
  @$pb.TagNumber(7)
  set deleted($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasDeleted() => $_has(6);
  @$pb.TagNumber(7)
  void clearDeleted() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get frozen => $_getBF(7);
  @$pb.TagNumber(8)
  set frozen($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasFrozen() => $_has(7);
  @$pb.TagNumber(8)
  void clearFrozen() => $_clearField(8);

  @$pb.TagNumber(9)
  SpaceAccountCaptureOrigin get origin => $_getN(8);
  @$pb.TagNumber(9)
  set origin(SpaceAccountCaptureOrigin value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasOrigin() => $_has(8);
  @$pb.TagNumber(9)
  void clearOrigin() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get ownerReceiptId => $_getSZ(9);
  @$pb.TagNumber(10)
  set ownerReceiptId($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasOwnerReceiptId() => $_has(9);
  @$pb.TagNumber(10)
  void clearOwnerReceiptId() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.List<$core.int> get ownerReceiptDigest => $_getN(10);
  @$pb.TagNumber(11)
  set ownerReceiptDigest($core.List<$core.int> value) => $_setBytes(10, value);
  @$pb.TagNumber(11)
  $core.bool hasOwnerReceiptDigest() => $_has(10);
  @$pb.TagNumber(11)
  void clearOwnerReceiptDigest() => $_clearField(11);

  /// Positive terminal evidence is independent of restorable deleted/frozen.
  @$pb.TagNumber(12)
  $core.String get terminalOperationId => $_getSZ(11);
  @$pb.TagNumber(12)
  set terminalOperationId($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasTerminalOperationId() => $_has(11);
  @$pb.TagNumber(12)
  void clearTerminalOperationId() => $_clearField(12);

  @$pb.TagNumber(13)
  $fixnum.Int64 get terminalRevision => $_getI64(12);
  @$pb.TagNumber(13)
  set terminalRevision($fixnum.Int64 value) => $_setInt64(12, value);
  @$pb.TagNumber(13)
  $core.bool hasTerminalRevision() => $_has(12);
  @$pb.TagNumber(13)
  void clearTerminalRevision() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get terminalReceiptId => $_getSZ(13);
  @$pb.TagNumber(14)
  set terminalReceiptId($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasTerminalReceiptId() => $_has(13);
  @$pb.TagNumber(14)
  void clearTerminalReceiptId() => $_clearField(14);

  @$pb.TagNumber(15)
  $core.List<$core.int> get terminalReceiptDigest => $_getN(14);
  @$pb.TagNumber(15)
  set terminalReceiptDigest($core.List<$core.int> value) =>
      $_setBytes(14, value);
  @$pb.TagNumber(15)
  $core.bool hasTerminalReceiptDigest() => $_has(14);
  @$pb.TagNumber(15)
  void clearTerminalReceiptDigest() => $_clearField(15);

  @$pb.TagNumber(16)
  $0.Timestamp get captureCut => $_getN(15);
  @$pb.TagNumber(16)
  set captureCut($0.Timestamp value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasCaptureCut() => $_has(15);
  @$pb.TagNumber(16)
  void clearCaptureCut() => $_clearField(16);
  @$pb.TagNumber(16)
  $0.Timestamp ensureCaptureCut() => $_ensure(15);

  @$pb.TagNumber(17)
  SpaceAccountAssociationOrigin get associationOrigin => $_getN(16);
  @$pb.TagNumber(17)
  set associationOrigin(SpaceAccountAssociationOrigin value) =>
      $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasAssociationOrigin() => $_has(16);
  @$pb.TagNumber(17)
  void clearAssociationOrigin() => $_clearField(17);

  @$pb.TagNumber(18)
  $core.bool get accountInactive => $_getBF(17);
  @$pb.TagNumber(18)
  set accountInactive($core.bool value) => $_setBool(17, value);
  @$pb.TagNumber(18)
  $core.bool hasAccountInactive() => $_has(17);
  @$pb.TagNumber(18)
  void clearAccountInactive() => $_clearField(18);
}

class CaptureSpaceMemberAccountBindingsResponse extends $pb.GeneratedMessage {
  factory CaptureSpaceMemberAccountBindingsResponse({
    $core.int? protocol,
    $core.String? operationId,
    SpaceAccountCapturePurpose? purpose,
    $core.String? spaceId,
    $core.List<$core.int>? cohortDigest,
    $core.List<$core.int>? requestDigest,
    $fixnum.Int64? ownerSourceRevision,
    $core.Iterable<SpaceAccountCaptureResult>? results,
    $core.String? userSourceGeneration,
    SpaceAccountIdentityCut? identityCut,
  }) {
    final result = create();
    if (protocol != null) result.protocol = protocol;
    if (operationId != null) result.operationId = operationId;
    if (purpose != null) result.purpose = purpose;
    if (spaceId != null) result.spaceId = spaceId;
    if (cohortDigest != null) result.cohortDigest = cohortDigest;
    if (requestDigest != null) result.requestDigest = requestDigest;
    if (ownerSourceRevision != null)
      result.ownerSourceRevision = ownerSourceRevision;
    if (results != null) result.results.addAll(results);
    if (userSourceGeneration != null)
      result.userSourceGeneration = userSourceGeneration;
    if (identityCut != null) result.identityCut = identityCut;
    return result;
  }

  CaptureSpaceMemberAccountBindingsResponse._();

  factory CaptureSpaceMemberAccountBindingsResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CaptureSpaceMemberAccountBindingsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CaptureSpaceMemberAccountBindingsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocol', fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aE<SpaceAccountCapturePurpose>(3, _omitFieldNames ? '' : 'purpose',
        enumValues: SpaceAccountCapturePurpose.values)
    ..aOS(4, _omitFieldNames ? '' : 'spaceId')
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'cohortDigest', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'requestDigest', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'ownerSourceRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..pPM<SpaceAccountCaptureResult>(8, _omitFieldNames ? '' : 'results',
        subBuilder: SpaceAccountCaptureResult.create)
    ..aOS(9, _omitFieldNames ? '' : 'userSourceGeneration')
    ..aOM<SpaceAccountIdentityCut>(10, _omitFieldNames ? '' : 'identityCut',
        subBuilder: SpaceAccountIdentityCut.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CaptureSpaceMemberAccountBindingsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CaptureSpaceMemberAccountBindingsResponse copyWith(
          void Function(CaptureSpaceMemberAccountBindingsResponse) updates) =>
      super.copyWith((message) =>
              updates(message as CaptureSpaceMemberAccountBindingsResponse))
          as CaptureSpaceMemberAccountBindingsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CaptureSpaceMemberAccountBindingsResponse create() =>
      CaptureSpaceMemberAccountBindingsResponse._();
  @$core.override
  CaptureSpaceMemberAccountBindingsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CaptureSpaceMemberAccountBindingsResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          CaptureSpaceMemberAccountBindingsResponse>(create);
  static CaptureSpaceMemberAccountBindingsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocol => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocol($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocol() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocol() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get operationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  SpaceAccountCapturePurpose get purpose => $_getN(2);
  @$pb.TagNumber(3)
  set purpose(SpaceAccountCapturePurpose value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPurpose() => $_has(2);
  @$pb.TagNumber(3)
  void clearPurpose() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get spaceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set spaceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSpaceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpaceId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get cohortDigest => $_getN(4);
  @$pb.TagNumber(5)
  set cohortDigest($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCohortDigest() => $_has(4);
  @$pb.TagNumber(5)
  void clearCohortDigest() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get requestDigest => $_getN(5);
  @$pb.TagNumber(6)
  set requestDigest($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRequestDigest() => $_has(5);
  @$pb.TagNumber(6)
  void clearRequestDigest() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get ownerSourceRevision => $_getI64(6);
  @$pb.TagNumber(7)
  set ownerSourceRevision($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasOwnerSourceRevision() => $_has(6);
  @$pb.TagNumber(7)
  void clearOwnerSourceRevision() => $_clearField(7);

  /// Exactly one result for every manifest entry, in the same canonical order.
  @$pb.TagNumber(8)
  $pb.PbList<SpaceAccountCaptureResult> get results => $_getList(7);

  @$pb.TagNumber(9)
  $core.String get userSourceGeneration => $_getSZ(8);
  @$pb.TagNumber(9)
  set userSourceGeneration($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasUserSourceGeneration() => $_has(8);
  @$pb.TagNumber(9)
  void clearUserSourceGeneration() => $_clearField(9);

  @$pb.TagNumber(10)
  SpaceAccountIdentityCut get identityCut => $_getN(9);
  @$pb.TagNumber(10)
  set identityCut(SpaceAccountIdentityCut value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasIdentityCut() => $_has(9);
  @$pb.TagNumber(10)
  void clearIdentityCut() => $_clearField(10);
  @$pb.TagNumber(10)
  SpaceAccountIdentityCut ensureIdentityCut() => $_ensure(9);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
