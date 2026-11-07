// This is a generated file - do not edit.
//
// Generated from voice/user/v1/auth_profile_authority.proto.

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

import 'auth_profile_authority.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'auth_profile_authority.pbenum.dart';

class AuthProfileSourcePin extends $pb.GeneratedMessage {
  factory AuthProfileSourcePin({
    $core.String? userSourceGeneration,
    $core.String? historyGeneration,
    $fixnum.Int64? keysetRevision,
  }) {
    final result = create();
    if (userSourceGeneration != null)
      result.userSourceGeneration = userSourceGeneration;
    if (historyGeneration != null) result.historyGeneration = historyGeneration;
    if (keysetRevision != null) result.keysetRevision = keysetRevision;
    return result;
  }

  AuthProfileSourcePin._();

  factory AuthProfileSourcePin.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AuthProfileSourcePin.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuthProfileSourcePin',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'userSourceGeneration')
    ..aOS(2, _omitFieldNames ? '' : 'historyGeneration')
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'keysetRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthProfileSourcePin clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthProfileSourcePin copyWith(void Function(AuthProfileSourcePin) updates) =>
      super.copyWith((message) => updates(message as AuthProfileSourcePin))
          as AuthProfileSourcePin;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AuthProfileSourcePin create() => AuthProfileSourcePin._();
  @$core.override
  AuthProfileSourcePin createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AuthProfileSourcePin getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AuthProfileSourcePin>(create);
  static AuthProfileSourcePin? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get userSourceGeneration => $_getSZ(0);
  @$pb.TagNumber(1)
  set userSourceGeneration($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUserSourceGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearUserSourceGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get historyGeneration => $_getSZ(1);
  @$pb.TagNumber(2)
  set historyGeneration($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHistoryGeneration() => $_has(1);
  @$pb.TagNumber(2)
  void clearHistoryGeneration() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get keysetRevision => $_getI64(2);
  @$pb.TagNumber(3)
  set keysetRevision($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasKeysetRevision() => $_has(2);
  @$pb.TagNumber(3)
  void clearKeysetRevision() => $_clearField(3);
}

/// An exact qualified local observation, not an independent CURRENT grant.
class AuthProfileSourceCut extends $pb.GeneratedMessage {
  factory AuthProfileSourceCut({
    AuthProfileSourcePin? source,
    $fixnum.Int64? userAuthorityRevision,
    $fixnum.Int64? throughRevision,
    $core.List<$core.int>? headDigest,
    $fixnum.Int64? retainedFloor,
    $fixnum.Int64? unresolvedErasureIssued,
    $fixnum.Int64? verifiedErasureCovered,
    $1.Timestamp? observedAt,
  }) {
    final result = create();
    if (source != null) result.source = source;
    if (userAuthorityRevision != null)
      result.userAuthorityRevision = userAuthorityRevision;
    if (throughRevision != null) result.throughRevision = throughRevision;
    if (headDigest != null) result.headDigest = headDigest;
    if (retainedFloor != null) result.retainedFloor = retainedFloor;
    if (unresolvedErasureIssued != null)
      result.unresolvedErasureIssued = unresolvedErasureIssued;
    if (verifiedErasureCovered != null)
      result.verifiedErasureCovered = verifiedErasureCovered;
    if (observedAt != null) result.observedAt = observedAt;
    return result;
  }

  AuthProfileSourceCut._();

  factory AuthProfileSourceCut.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AuthProfileSourceCut.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuthProfileSourceCut',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<AuthProfileSourcePin>(1, _omitFieldNames ? '' : 'source',
        subBuilder: AuthProfileSourcePin.create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'userAuthorityRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'throughRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'headDigest', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'retainedFloor', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(6, _omitFieldNames ? '' : 'unresolvedErasureIssued',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'verifiedErasureCovered', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'observedAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthProfileSourceCut clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthProfileSourceCut copyWith(void Function(AuthProfileSourceCut) updates) =>
      super.copyWith((message) => updates(message as AuthProfileSourceCut))
          as AuthProfileSourceCut;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AuthProfileSourceCut create() => AuthProfileSourceCut._();
  @$core.override
  AuthProfileSourceCut createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AuthProfileSourceCut getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AuthProfileSourceCut>(create);
  static AuthProfileSourceCut? _defaultInstance;

  @$pb.TagNumber(1)
  AuthProfileSourcePin get source => $_getN(0);
  @$pb.TagNumber(1)
  set source(AuthProfileSourcePin value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSource() => $_has(0);
  @$pb.TagNumber(1)
  void clearSource() => $_clearField(1);
  @$pb.TagNumber(1)
  AuthProfileSourcePin ensureSource() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get userAuthorityRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set userAuthorityRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUserAuthorityRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearUserAuthorityRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get throughRevision => $_getI64(2);
  @$pb.TagNumber(3)
  set throughRevision($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasThroughRevision() => $_has(2);
  @$pb.TagNumber(3)
  void clearThroughRevision() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get headDigest => $_getN(3);
  @$pb.TagNumber(4)
  set headDigest($core.List<$core.int> value) => $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasHeadDigest() => $_has(3);
  @$pb.TagNumber(4)
  void clearHeadDigest() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get retainedFloor => $_getI64(4);
  @$pb.TagNumber(5)
  set retainedFloor($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRetainedFloor() => $_has(4);
  @$pb.TagNumber(5)
  void clearRetainedFloor() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get unresolvedErasureIssued => $_getI64(5);
  @$pb.TagNumber(6)
  set unresolvedErasureIssued($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasUnresolvedErasureIssued() => $_has(5);
  @$pb.TagNumber(6)
  void clearUnresolvedErasureIssued() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get verifiedErasureCovered => $_getI64(6);
  @$pb.TagNumber(7)
  set verifiedErasureCovered($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasVerifiedErasureCovered() => $_has(6);
  @$pb.TagNumber(7)
  void clearVerifiedErasureCovered() => $_clearField(7);

  /// Response observation window is [observed_at, observed_at + 2 seconds).
  /// It expires independently of the request and never grants admission.
  @$pb.TagNumber(8)
  $1.Timestamp get observedAt => $_getN(7);
  @$pb.TagNumber(8)
  set observedAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasObservedAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearObservedAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureObservedAt() => $_ensure(7);
}

class MatchAuthProfileReferencesRequest extends $pb.GeneratedMessage {
  factory MatchAuthProfileReferencesRequest({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.String? inventoryId,
    $fixnum.Int64? inventoryRevision,
    $core.String? accountId,
    $core.String? ownerHandle,
    $core.String? deletionCycleId,
    $core.Iterable<$core.String>? profileIds,
    AuthProfileSourcePin? expectedSource,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (inventoryId != null) result.inventoryId = inventoryId;
    if (inventoryRevision != null) result.inventoryRevision = inventoryRevision;
    if (accountId != null) result.accountId = accountId;
    if (ownerHandle != null) result.ownerHandle = ownerHandle;
    if (deletionCycleId != null) result.deletionCycleId = deletionCycleId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (expectedSource != null) result.expectedSource = expectedSource;
    return result;
  }

  MatchAuthProfileReferencesRequest._();

  factory MatchAuthProfileReferencesRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchAuthProfileReferencesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchAuthProfileReferencesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOS(3, _omitFieldNames ? '' : 'inventoryId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'inventoryRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(5, _omitFieldNames ? '' : 'accountId')
    ..aOS(6, _omitFieldNames ? '' : 'ownerHandle')
    ..aOS(7, _omitFieldNames ? '' : 'deletionCycleId')
    ..pPS(8, _omitFieldNames ? '' : 'profileIds')
    ..aOM<AuthProfileSourcePin>(9, _omitFieldNames ? '' : 'expectedSource',
        subBuilder: AuthProfileSourcePin.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchAuthProfileReferencesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchAuthProfileReferencesRequest copyWith(
          void Function(MatchAuthProfileReferencesRequest) updates) =>
      super.copyWith((message) =>
              updates(message as MatchAuthProfileReferencesRequest))
          as MatchAuthProfileReferencesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchAuthProfileReferencesRequest create() =>
      MatchAuthProfileReferencesRequest._();
  @$core.override
  MatchAuthProfileReferencesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchAuthProfileReferencesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchAuthProfileReferencesRequest>(
          create);
  static MatchAuthProfileReferencesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get inventoryId => $_getSZ(2);
  @$pb.TagNumber(3)
  set inventoryId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasInventoryId() => $_has(2);
  @$pb.TagNumber(3)
  void clearInventoryId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get inventoryRevision => $_getI64(3);
  @$pb.TagNumber(4)
  set inventoryRevision($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasInventoryRevision() => $_has(3);
  @$pb.TagNumber(4)
  void clearInventoryRevision() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get accountId => $_getSZ(4);
  @$pb.TagNumber(5)
  set accountId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAccountId() => $_has(4);
  @$pb.TagNumber(5)
  void clearAccountId() => $_clearField(5);

  /// Echoed Auth context, never asserted as User-owned account lifecycle facts.
  @$pb.TagNumber(6)
  $core.String get ownerHandle => $_getSZ(5);
  @$pb.TagNumber(6)
  set ownerHandle($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasOwnerHandle() => $_has(5);
  @$pb.TagNumber(6)
  void clearOwnerHandle() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get deletionCycleId => $_getSZ(6);
  @$pb.TagNumber(7)
  set deletionCycleId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasDeletionCycleId() => $_has(6);
  @$pb.TagNumber(7)
  void clearDeletionCycleId() => $_clearField(7);

  /// Canonical sorted unique finite batch, 1..256. Never a raw-ID enumeration.
  @$pb.TagNumber(8)
  $pb.PbList<$core.String> get profileIds => $_getList(7);

  @$pb.TagNumber(9)
  AuthProfileSourcePin get expectedSource => $_getN(8);
  @$pb.TagNumber(9)
  set expectedSource(AuthProfileSourcePin value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasExpectedSource() => $_has(8);
  @$pb.TagNumber(9)
  void clearExpectedSource() => $_clearField(9);
  @$pb.TagNumber(9)
  AuthProfileSourcePin ensureExpectedSource() => $_ensure(8);
}

class AuthProfileReferenceMatch extends $pb.GeneratedMessage {
  factory AuthProfileReferenceMatch({
    $core.String? profileId,
    AuthProfileMatchOutcome? outcome,
    $core.String? associationId,
    $fixnum.Int64? associationRevision,
    $core.String? captureOrigin,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (outcome != null) result.outcome = outcome;
    if (associationId != null) result.associationId = associationId;
    if (associationRevision != null)
      result.associationRevision = associationRevision;
    if (captureOrigin != null) result.captureOrigin = captureOrigin;
    return result;
  }

  AuthProfileReferenceMatch._();

  factory AuthProfileReferenceMatch.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AuthProfileReferenceMatch.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuthProfileReferenceMatch',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aE<AuthProfileMatchOutcome>(2, _omitFieldNames ? '' : 'outcome',
        enumValues: AuthProfileMatchOutcome.values)
    ..aOS(3, _omitFieldNames ? '' : 'associationId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'associationRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(5, _omitFieldNames ? '' : 'captureOrigin')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthProfileReferenceMatch clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthProfileReferenceMatch copyWith(
          void Function(AuthProfileReferenceMatch) updates) =>
      super.copyWith((message) => updates(message as AuthProfileReferenceMatch))
          as AuthProfileReferenceMatch;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AuthProfileReferenceMatch create() => AuthProfileReferenceMatch._();
  @$core.override
  AuthProfileReferenceMatch createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AuthProfileReferenceMatch getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AuthProfileReferenceMatch>(create);
  static AuthProfileReferenceMatch? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  AuthProfileMatchOutcome get outcome => $_getN(1);
  @$pb.TagNumber(2)
  set outcome(AuthProfileMatchOutcome value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOutcome() => $_has(1);
  @$pb.TagNumber(2)
  void clearOutcome() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get associationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set associationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAssociationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAssociationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get associationRevision => $_getI64(3);
  @$pb.TagNumber(4)
  set associationRevision($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAssociationRevision() => $_has(3);
  @$pb.TagNumber(4)
  void clearAssociationRevision() => $_clearField(4);

  /// BIRTH, RETAINED_NOW or VERIFIED_HISTORY; absent for INCOMPLETE.
  @$pb.TagNumber(5)
  $core.String get captureOrigin => $_getSZ(4);
  @$pb.TagNumber(5)
  set captureOrigin($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCaptureOrigin() => $_has(4);
  @$pb.TagNumber(5)
  void clearCaptureOrigin() => $_clearField(5);
}

class MatchAuthProfileReferencesResponse extends $pb.GeneratedMessage {
  factory MatchAuthProfileReferencesResponse({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.String? inventoryId,
    $fixnum.Int64? inventoryRevision,
    $core.String? accountId,
    $core.String? ownerHandle,
    $core.String? deletionCycleId,
    $core.List<$core.int>? requestDigest,
    $core.Iterable<AuthProfileReferenceMatch>? results,
    AuthProfileSourceCut? sourceCut,
    $core.bool? allRequestedResolved,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (inventoryId != null) result.inventoryId = inventoryId;
    if (inventoryRevision != null) result.inventoryRevision = inventoryRevision;
    if (accountId != null) result.accountId = accountId;
    if (ownerHandle != null) result.ownerHandle = ownerHandle;
    if (deletionCycleId != null) result.deletionCycleId = deletionCycleId;
    if (requestDigest != null) result.requestDigest = requestDigest;
    if (results != null) result.results.addAll(results);
    if (sourceCut != null) result.sourceCut = sourceCut;
    if (allRequestedResolved != null)
      result.allRequestedResolved = allRequestedResolved;
    return result;
  }

  MatchAuthProfileReferencesResponse._();

  factory MatchAuthProfileReferencesResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchAuthProfileReferencesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchAuthProfileReferencesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOS(3, _omitFieldNames ? '' : 'inventoryId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'inventoryRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(5, _omitFieldNames ? '' : 'accountId')
    ..aOS(6, _omitFieldNames ? '' : 'ownerHandle')
    ..aOS(7, _omitFieldNames ? '' : 'deletionCycleId')
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'requestDigest', $pb.PbFieldType.OY)
    ..pPM<AuthProfileReferenceMatch>(9, _omitFieldNames ? '' : 'results',
        subBuilder: AuthProfileReferenceMatch.create)
    ..aOM<AuthProfileSourceCut>(10, _omitFieldNames ? '' : 'sourceCut',
        subBuilder: AuthProfileSourceCut.create)
    ..aOB(11, _omitFieldNames ? '' : 'allRequestedResolved')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchAuthProfileReferencesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchAuthProfileReferencesResponse copyWith(
          void Function(MatchAuthProfileReferencesResponse) updates) =>
      super.copyWith((message) =>
              updates(message as MatchAuthProfileReferencesResponse))
          as MatchAuthProfileReferencesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchAuthProfileReferencesResponse create() =>
      MatchAuthProfileReferencesResponse._();
  @$core.override
  MatchAuthProfileReferencesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchAuthProfileReferencesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchAuthProfileReferencesResponse>(
          create);
  static MatchAuthProfileReferencesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get inventoryId => $_getSZ(2);
  @$pb.TagNumber(3)
  set inventoryId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasInventoryId() => $_has(2);
  @$pb.TagNumber(3)
  void clearInventoryId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get inventoryRevision => $_getI64(3);
  @$pb.TagNumber(4)
  set inventoryRevision($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasInventoryRevision() => $_has(3);
  @$pb.TagNumber(4)
  void clearInventoryRevision() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get accountId => $_getSZ(4);
  @$pb.TagNumber(5)
  set accountId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAccountId() => $_has(4);
  @$pb.TagNumber(5)
  void clearAccountId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get ownerHandle => $_getSZ(5);
  @$pb.TagNumber(6)
  set ownerHandle($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasOwnerHandle() => $_has(5);
  @$pb.TagNumber(6)
  void clearOwnerHandle() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get deletionCycleId => $_getSZ(6);
  @$pb.TagNumber(7)
  set deletionCycleId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasDeletionCycleId() => $_has(6);
  @$pb.TagNumber(7)
  void clearDeletionCycleId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get requestDigest => $_getN(7);
  @$pb.TagNumber(8)
  set requestDigest($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasRequestDigest() => $_has(7);
  @$pb.TagNumber(8)
  void clearRequestDigest() => $_clearField(8);

  @$pb.TagNumber(9)
  $pb.PbList<AuthProfileReferenceMatch> get results => $_getList(8);

  @$pb.TagNumber(10)
  AuthProfileSourceCut get sourceCut => $_getN(9);
  @$pb.TagNumber(10)
  set sourceCut(AuthProfileSourceCut value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasSourceCut() => $_has(9);
  @$pb.TagNumber(10)
  void clearSourceCut() => $_clearField(10);
  @$pb.TagNumber(10)
  AuthProfileSourceCut ensureSourceCut() => $_ensure(9);

  /// Only these requested entries have evidence; never a global roster claim.
  @$pb.TagNumber(11)
  $core.bool get allRequestedResolved => $_getBF(10);
  @$pb.TagNumber(11)
  set allRequestedResolved($core.bool value) => $_setBool(10, value);
  @$pb.TagNumber(11)
  $core.bool hasAllRequestedResolved() => $_has(10);
  @$pb.TagNumber(11)
  void clearAllRequestedResolved() => $_clearField(11);
}

class ResolveAuthOwnershipTargetRequest extends $pb.GeneratedMessage {
  factory ResolveAuthOwnershipTargetRequest({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.String? operationId,
    $core.String? profileId,
    AuthProfileSourcePin? expectedSource,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (operationId != null) result.operationId = operationId;
    if (profileId != null) result.profileId = profileId;
    if (expectedSource != null) result.expectedSource = expectedSource;
    return result;
  }

  ResolveAuthOwnershipTargetRequest._();

  factory ResolveAuthOwnershipTargetRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveAuthOwnershipTargetRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveAuthOwnershipTargetRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..aOS(4, _omitFieldNames ? '' : 'profileId')
    ..aOM<AuthProfileSourcePin>(5, _omitFieldNames ? '' : 'expectedSource',
        subBuilder: AuthProfileSourcePin.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveAuthOwnershipTargetRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveAuthOwnershipTargetRequest copyWith(
          void Function(ResolveAuthOwnershipTargetRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ResolveAuthOwnershipTargetRequest))
          as ResolveAuthOwnershipTargetRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveAuthOwnershipTargetRequest create() =>
      ResolveAuthOwnershipTargetRequest._();
  @$core.override
  ResolveAuthOwnershipTargetRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveAuthOwnershipTargetRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveAuthOwnershipTargetRequest>(
          create);
  static ResolveAuthOwnershipTargetRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get profileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set profileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  AuthProfileSourcePin get expectedSource => $_getN(4);
  @$pb.TagNumber(5)
  set expectedSource(AuthProfileSourcePin value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasExpectedSource() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpectedSource() => $_clearField(5);
  @$pb.TagNumber(5)
  AuthProfileSourcePin ensureExpectedSource() => $_ensure(4);
}

class ResolveAuthOwnershipTargetResponse extends $pb.GeneratedMessage {
  factory ResolveAuthOwnershipTargetResponse({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.String? operationId,
    $core.String? profileId,
    $core.List<$core.int>? requestDigest,
    AuthOwnershipTargetOutcome? outcome,
    $core.String? accountId,
    $fixnum.Int64? profileRevision,
    $core.bool? deleted,
    $core.bool? frozen,
    $core.bool? accountInactive,
    $core.String? associationId,
    $fixnum.Int64? associationRevision,
    $core.String? terminalOperationId,
    $fixnum.Int64? terminalRevision,
    $core.String? terminalReceiptId,
    $core.List<$core.int>? terminalReceiptDigest,
    AuthProfileSourceCut? sourceCut,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (operationId != null) result.operationId = operationId;
    if (profileId != null) result.profileId = profileId;
    if (requestDigest != null) result.requestDigest = requestDigest;
    if (outcome != null) result.outcome = outcome;
    if (accountId != null) result.accountId = accountId;
    if (profileRevision != null) result.profileRevision = profileRevision;
    if (deleted != null) result.deleted = deleted;
    if (frozen != null) result.frozen = frozen;
    if (accountInactive != null) result.accountInactive = accountInactive;
    if (associationId != null) result.associationId = associationId;
    if (associationRevision != null)
      result.associationRevision = associationRevision;
    if (terminalOperationId != null)
      result.terminalOperationId = terminalOperationId;
    if (terminalRevision != null) result.terminalRevision = terminalRevision;
    if (terminalReceiptId != null) result.terminalReceiptId = terminalReceiptId;
    if (terminalReceiptDigest != null)
      result.terminalReceiptDigest = terminalReceiptDigest;
    if (sourceCut != null) result.sourceCut = sourceCut;
    return result;
  }

  ResolveAuthOwnershipTargetResponse._();

  factory ResolveAuthOwnershipTargetResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveAuthOwnershipTargetResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveAuthOwnershipTargetResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..aOS(4, _omitFieldNames ? '' : 'profileId')
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'requestDigest', $pb.PbFieldType.OY)
    ..aE<AuthOwnershipTargetOutcome>(6, _omitFieldNames ? '' : 'outcome',
        enumValues: AuthOwnershipTargetOutcome.values)
    ..aOS(7, _omitFieldNames ? '' : 'accountId')
    ..a<$fixnum.Int64>(
        8, _omitFieldNames ? '' : 'profileRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOB(9, _omitFieldNames ? '' : 'deleted')
    ..aOB(10, _omitFieldNames ? '' : 'frozen')
    ..aOB(11, _omitFieldNames ? '' : 'accountInactive')
    ..aOS(12, _omitFieldNames ? '' : 'associationId')
    ..a<$fixnum.Int64>(
        13, _omitFieldNames ? '' : 'associationRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(14, _omitFieldNames ? '' : 'terminalOperationId')
    ..a<$fixnum.Int64>(
        15, _omitFieldNames ? '' : 'terminalRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(16, _omitFieldNames ? '' : 'terminalReceiptId')
    ..a<$core.List<$core.int>>(
        17, _omitFieldNames ? '' : 'terminalReceiptDigest', $pb.PbFieldType.OY)
    ..aOM<AuthProfileSourceCut>(18, _omitFieldNames ? '' : 'sourceCut',
        subBuilder: AuthProfileSourceCut.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveAuthOwnershipTargetResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveAuthOwnershipTargetResponse copyWith(
          void Function(ResolveAuthOwnershipTargetResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ResolveAuthOwnershipTargetResponse))
          as ResolveAuthOwnershipTargetResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveAuthOwnershipTargetResponse create() =>
      ResolveAuthOwnershipTargetResponse._();
  @$core.override
  ResolveAuthOwnershipTargetResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveAuthOwnershipTargetResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveAuthOwnershipTargetResponse>(
          create);
  static ResolveAuthOwnershipTargetResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get profileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set profileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get requestDigest => $_getN(4);
  @$pb.TagNumber(5)
  set requestDigest($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRequestDigest() => $_has(4);
  @$pb.TagNumber(5)
  void clearRequestDigest() => $_clearField(5);

  @$pb.TagNumber(6)
  AuthOwnershipTargetOutcome get outcome => $_getN(5);
  @$pb.TagNumber(6)
  set outcome(AuthOwnershipTargetOutcome value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasOutcome() => $_has(5);
  @$pb.TagNumber(6)
  void clearOutcome() => $_clearField(6);

  /// Private current tuple only. Never reconstructed for terminal/unknown IDs.
  @$pb.TagNumber(7)
  $core.String get accountId => $_getSZ(6);
  @$pb.TagNumber(7)
  set accountId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasAccountId() => $_has(6);
  @$pb.TagNumber(7)
  void clearAccountId() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get profileRevision => $_getI64(7);
  @$pb.TagNumber(8)
  set profileRevision($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasProfileRevision() => $_has(7);
  @$pb.TagNumber(8)
  void clearProfileRevision() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get deleted => $_getBF(8);
  @$pb.TagNumber(9)
  set deleted($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasDeleted() => $_has(8);
  @$pb.TagNumber(9)
  void clearDeleted() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.bool get frozen => $_getBF(9);
  @$pb.TagNumber(10)
  set frozen($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasFrozen() => $_has(9);
  @$pb.TagNumber(10)
  void clearFrozen() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.bool get accountInactive => $_getBF(10);
  @$pb.TagNumber(11)
  set accountInactive($core.bool value) => $_setBool(10, value);
  @$pb.TagNumber(11)
  $core.bool hasAccountInactive() => $_has(10);
  @$pb.TagNumber(11)
  void clearAccountInactive() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get associationId => $_getSZ(11);
  @$pb.TagNumber(12)
  set associationId($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasAssociationId() => $_has(11);
  @$pb.TagNumber(12)
  void clearAssociationId() => $_clearField(12);

  @$pb.TagNumber(13)
  $fixnum.Int64 get associationRevision => $_getI64(12);
  @$pb.TagNumber(13)
  set associationRevision($fixnum.Int64 value) => $_setInt64(12, value);
  @$pb.TagNumber(13)
  $core.bool hasAssociationRevision() => $_has(12);
  @$pb.TagNumber(13)
  void clearAssociationRevision() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get terminalOperationId => $_getSZ(13);
  @$pb.TagNumber(14)
  set terminalOperationId($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasTerminalOperationId() => $_has(13);
  @$pb.TagNumber(14)
  void clearTerminalOperationId() => $_clearField(14);

  @$pb.TagNumber(15)
  $fixnum.Int64 get terminalRevision => $_getI64(14);
  @$pb.TagNumber(15)
  set terminalRevision($fixnum.Int64 value) => $_setInt64(14, value);
  @$pb.TagNumber(15)
  $core.bool hasTerminalRevision() => $_has(14);
  @$pb.TagNumber(15)
  void clearTerminalRevision() => $_clearField(15);

  @$pb.TagNumber(16)
  $core.String get terminalReceiptId => $_getSZ(15);
  @$pb.TagNumber(16)
  set terminalReceiptId($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasTerminalReceiptId() => $_has(15);
  @$pb.TagNumber(16)
  void clearTerminalReceiptId() => $_clearField(16);

  @$pb.TagNumber(17)
  $core.List<$core.int> get terminalReceiptDigest => $_getN(16);
  @$pb.TagNumber(17)
  set terminalReceiptDigest($core.List<$core.int> value) =>
      $_setBytes(16, value);
  @$pb.TagNumber(17)
  $core.bool hasTerminalReceiptDigest() => $_has(16);
  @$pb.TagNumber(17)
  void clearTerminalReceiptDigest() => $_clearField(17);

  @$pb.TagNumber(18)
  AuthProfileSourceCut get sourceCut => $_getN(17);
  @$pb.TagNumber(18)
  set sourceCut(AuthProfileSourceCut value) => $_setField(18, value);
  @$pb.TagNumber(18)
  $core.bool hasSourceCut() => $_has(17);
  @$pb.TagNumber(18)
  void clearSourceCut() => $_clearField(18);
  @$pb.TagNumber(18)
  AuthProfileSourceCut ensureSourceCut() => $_ensure(17);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
