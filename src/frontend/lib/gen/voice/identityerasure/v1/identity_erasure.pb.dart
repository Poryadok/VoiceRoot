// This is a generated file - do not edit.
//
// Generated from voice/identityerasure/v1/identity_erasure.proto.

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

import 'identity_erasure.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'identity_erasure.pbenum.dart';

/// Exact immutable owner journal cut. Digests frame compact selector/fence facts;
/// neither a digest nor complete=true alone establishes independent CURRENT.
class OwnerCut extends $pb.GeneratedMessage {
  factory OwnerCut({
    $core.String? owner,
    $core.String? streamId,
    $core.String? historyGeneration,
    $fixnum.Int64? throughRevision,
    $fixnum.Int64? selectorKeysetRevision,
    $core.Iterable<$core.String>? selectorKeyVersions,
    $fixnum.Int64? factCount,
    $core.List<$core.int>? digest,
    $core.bool? complete,
  }) {
    final result = create();
    if (owner != null) result.owner = owner;
    if (streamId != null) result.streamId = streamId;
    if (historyGeneration != null) result.historyGeneration = historyGeneration;
    if (throughRevision != null) result.throughRevision = throughRevision;
    if (selectorKeysetRevision != null)
      result.selectorKeysetRevision = selectorKeysetRevision;
    if (selectorKeyVersions != null)
      result.selectorKeyVersions.addAll(selectorKeyVersions);
    if (factCount != null) result.factCount = factCount;
    if (digest != null) result.digest = digest;
    if (complete != null) result.complete = complete;
    return result;
  }

  OwnerCut._();

  factory OwnerCut.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory OwnerCut.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OwnerCut',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'owner')
    ..aOS(2, _omitFieldNames ? '' : 'streamId')
    ..aOS(3, _omitFieldNames ? '' : 'historyGeneration')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'throughRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'selectorKeysetRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..pPS(6, _omitFieldNames ? '' : 'selectorKeyVersions')
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'factCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'digest', $pb.PbFieldType.OY)
    ..aOB(9, _omitFieldNames ? '' : 'complete')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OwnerCut clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OwnerCut copyWith(void Function(OwnerCut) updates) =>
      super.copyWith((message) => updates(message as OwnerCut)) as OwnerCut;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static OwnerCut create() => OwnerCut._();
  @$core.override
  OwnerCut createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static OwnerCut getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<OwnerCut>(create);
  static OwnerCut? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get owner => $_getSZ(0);
  @$pb.TagNumber(1)
  set owner($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOwner() => $_has(0);
  @$pb.TagNumber(1)
  void clearOwner() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get streamId => $_getSZ(1);
  @$pb.TagNumber(2)
  set streamId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStreamId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStreamId() => $_clearField(2);

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
  $fixnum.Int64 get selectorKeysetRevision => $_getI64(4);
  @$pb.TagNumber(5)
  set selectorKeysetRevision($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSelectorKeysetRevision() => $_has(4);
  @$pb.TagNumber(5)
  void clearSelectorKeysetRevision() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get selectorKeyVersions => $_getList(5);

  @$pb.TagNumber(7)
  $fixnum.Int64 get factCount => $_getI64(6);
  @$pb.TagNumber(7)
  set factCount($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasFactCount() => $_has(6);
  @$pb.TagNumber(7)
  void clearFactCount() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get digest => $_getN(7);
  @$pb.TagNumber(8)
  set digest($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasDigest() => $_has(7);
  @$pb.TagNumber(8)
  void clearDigest() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get complete => $_getBF(8);
  @$pb.TagNumber(9)
  set complete($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasComplete() => $_has(8);
  @$pb.TagNumber(9)
  void clearComplete() => $_clearField(9);
}

/// Present for recovery. All fields are required together, and must match the
/// independently durable current Role cut; an old signed vector is insufficient.
class RecoveryBinding extends $pb.GeneratedMessage {
  factory RecoveryBinding({
    $core.String? restoreTarget,
    $core.String? restoreGeneration,
    $core.List<$core.int>? challenge,
    $core.String? roleStreamId,
    $fixnum.Int64? roleThroughRevision,
    $core.List<$core.int>? roleCheckpointDigest,
    $core.String? roleKeysetRevision,
    $fixnum.Int64? inventoryGeneration,
    $core.List<$core.int>? inventoryDigest,
  }) {
    final result = create();
    if (restoreTarget != null) result.restoreTarget = restoreTarget;
    if (restoreGeneration != null) result.restoreGeneration = restoreGeneration;
    if (challenge != null) result.challenge = challenge;
    if (roleStreamId != null) result.roleStreamId = roleStreamId;
    if (roleThroughRevision != null)
      result.roleThroughRevision = roleThroughRevision;
    if (roleCheckpointDigest != null)
      result.roleCheckpointDigest = roleCheckpointDigest;
    if (roleKeysetRevision != null)
      result.roleKeysetRevision = roleKeysetRevision;
    if (inventoryGeneration != null)
      result.inventoryGeneration = inventoryGeneration;
    if (inventoryDigest != null) result.inventoryDigest = inventoryDigest;
    return result;
  }

  RecoveryBinding._();

  factory RecoveryBinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RecoveryBinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecoveryBinding',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'restoreTarget')
    ..aOS(2, _omitFieldNames ? '' : 'restoreGeneration')
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'challenge', $pb.PbFieldType.OY)
    ..aOS(4, _omitFieldNames ? '' : 'roleStreamId')
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'roleThroughRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'roleCheckpointDigest', $pb.PbFieldType.OY)
    ..aOS(7, _omitFieldNames ? '' : 'roleKeysetRevision')
    ..a<$fixnum.Int64>(
        8, _omitFieldNames ? '' : 'inventoryGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        9, _omitFieldNames ? '' : 'inventoryDigest', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecoveryBinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecoveryBinding copyWith(void Function(RecoveryBinding) updates) =>
      super.copyWith((message) => updates(message as RecoveryBinding))
          as RecoveryBinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RecoveryBinding create() => RecoveryBinding._();
  @$core.override
  RecoveryBinding createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RecoveryBinding getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RecoveryBinding>(create);
  static RecoveryBinding? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get restoreTarget => $_getSZ(0);
  @$pb.TagNumber(1)
  set restoreTarget($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRestoreTarget() => $_has(0);
  @$pb.TagNumber(1)
  void clearRestoreTarget() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get restoreGeneration => $_getSZ(1);
  @$pb.TagNumber(2)
  set restoreGeneration($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRestoreGeneration() => $_has(1);
  @$pb.TagNumber(2)
  void clearRestoreGeneration() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get challenge => $_getN(2);
  @$pb.TagNumber(3)
  set challenge($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChallenge() => $_has(2);
  @$pb.TagNumber(3)
  void clearChallenge() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get roleStreamId => $_getSZ(3);
  @$pb.TagNumber(4)
  set roleStreamId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRoleStreamId() => $_has(3);
  @$pb.TagNumber(4)
  void clearRoleStreamId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get roleThroughRevision => $_getI64(4);
  @$pb.TagNumber(5)
  set roleThroughRevision($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRoleThroughRevision() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoleThroughRevision() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get roleCheckpointDigest => $_getN(5);
  @$pb.TagNumber(6)
  set roleCheckpointDigest($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRoleCheckpointDigest() => $_has(5);
  @$pb.TagNumber(6)
  void clearRoleCheckpointDigest() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get roleKeysetRevision => $_getSZ(6);
  @$pb.TagNumber(7)
  set roleKeysetRevision($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRoleKeysetRevision() => $_has(6);
  @$pb.TagNumber(7)
  void clearRoleKeysetRevision() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get inventoryGeneration => $_getI64(7);
  @$pb.TagNumber(8)
  set inventoryGeneration($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasInventoryGeneration() => $_has(7);
  @$pb.TagNumber(8)
  void clearInventoryGeneration() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.List<$core.int> get inventoryDigest => $_getN(8);
  @$pb.TagNumber(9)
  set inventoryDigest($core.List<$core.int> value) => $_setBytes(8, value);
  @$pb.TagNumber(9)
  $core.bool hasInventoryDigest() => $_has(8);
  @$pb.TagNumber(9)
  void clearInventoryDigest() => $_clearField(9);
}

class ObservationWindow extends $pb.GeneratedMessage {
  factory ObservationWindow({
    $1.Timestamp? observedAt,
    $1.Timestamp? validUntil,
  }) {
    final result = create();
    if (observedAt != null) result.observedAt = observedAt;
    if (validUntil != null) result.validUntil = validUntil;
    return result;
  }

  ObservationWindow._();

  factory ObservationWindow.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObservationWindow.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObservationWindow',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aOM<$1.Timestamp>(1, _omitFieldNames ? '' : 'observedAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'validUntil',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObservationWindow clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObservationWindow copyWith(void Function(ObservationWindow) updates) =>
      super.copyWith((message) => updates(message as ObservationWindow))
          as ObservationWindow;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObservationWindow create() => ObservationWindow._();
  @$core.override
  ObservationWindow createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObservationWindow getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObservationWindow>(create);
  static ObservationWindow? _defaultInstance;

  @$pb.TagNumber(1)
  $1.Timestamp get observedAt => $_getN(0);
  @$pb.TagNumber(1)
  set observedAt($1.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasObservedAt() => $_has(0);
  @$pb.TagNumber(1)
  void clearObservedAt() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.Timestamp ensureObservedAt() => $_ensure(0);

  /// Equality is expired. A consumer rechecks its transaction's DB time.
  @$pb.TagNumber(2)
  $1.Timestamp get validUntil => $_getN(1);
  @$pb.TagNumber(2)
  set validUntil($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasValidUntil() => $_has(1);
  @$pb.TagNumber(2)
  void clearValidUntil() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureValidUntil() => $_ensure(1);
}

/// A distinct opaque Auth lifetime handle, never a Mod subject ID. A recoverable
/// cycle's fixed erase_at does not itself constitute an irreversible boundary.
class AccountLifecycleFact extends $pb.GeneratedMessage {
  factory AccountLifecycleFact({
    $core.String? ownerHandle,
    $fixnum.Int64? lifecycleRevision,
    AccountLifecycleState? state,
    $core.String? deletionCycle,
    $1.Timestamp? eraseAt,
    Eligibility? eligibility,
    ErasureDisposition? disposition,
    OwnerCut? ownerCut,
  }) {
    final result = create();
    if (ownerHandle != null) result.ownerHandle = ownerHandle;
    if (lifecycleRevision != null) result.lifecycleRevision = lifecycleRevision;
    if (state != null) result.state = state;
    if (deletionCycle != null) result.deletionCycle = deletionCycle;
    if (eraseAt != null) result.eraseAt = eraseAt;
    if (eligibility != null) result.eligibility = eligibility;
    if (disposition != null) result.disposition = disposition;
    if (ownerCut != null) result.ownerCut = ownerCut;
    return result;
  }

  AccountLifecycleFact._();

  factory AccountLifecycleFact.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AccountLifecycleFact.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AccountLifecycleFact',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'ownerHandle')
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'lifecycleRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<AccountLifecycleState>(3, _omitFieldNames ? '' : 'state',
        enumValues: AccountLifecycleState.values)
    ..aOS(4, _omitFieldNames ? '' : 'deletionCycle')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'eraseAt',
        subBuilder: $1.Timestamp.create)
    ..aE<Eligibility>(6, _omitFieldNames ? '' : 'eligibility',
        enumValues: Eligibility.values)
    ..aE<ErasureDisposition>(7, _omitFieldNames ? '' : 'disposition',
        enumValues: ErasureDisposition.values)
    ..aOM<OwnerCut>(8, _omitFieldNames ? '' : 'ownerCut',
        subBuilder: OwnerCut.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AccountLifecycleFact clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AccountLifecycleFact copyWith(void Function(AccountLifecycleFact) updates) =>
      super.copyWith((message) => updates(message as AccountLifecycleFact))
          as AccountLifecycleFact;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AccountLifecycleFact create() => AccountLifecycleFact._();
  @$core.override
  AccountLifecycleFact createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AccountLifecycleFact getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AccountLifecycleFact>(create);
  static AccountLifecycleFact? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get ownerHandle => $_getSZ(0);
  @$pb.TagNumber(1)
  set ownerHandle($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOwnerHandle() => $_has(0);
  @$pb.TagNumber(1)
  void clearOwnerHandle() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get lifecycleRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set lifecycleRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLifecycleRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearLifecycleRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  AccountLifecycleState get state => $_getN(2);
  @$pb.TagNumber(3)
  set state(AccountLifecycleState value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasState() => $_has(2);
  @$pb.TagNumber(3)
  void clearState() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deletionCycle => $_getSZ(3);
  @$pb.TagNumber(4)
  set deletionCycle($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeletionCycle() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeletionCycle() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get eraseAt => $_getN(4);
  @$pb.TagNumber(5)
  set eraseAt($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasEraseAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearEraseAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureEraseAt() => $_ensure(4);

  @$pb.TagNumber(6)
  Eligibility get eligibility => $_getN(5);
  @$pb.TagNumber(6)
  set eligibility(Eligibility value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasEligibility() => $_has(5);
  @$pb.TagNumber(6)
  void clearEligibility() => $_clearField(6);

  @$pb.TagNumber(7)
  ErasureDisposition get disposition => $_getN(6);
  @$pb.TagNumber(7)
  set disposition(ErasureDisposition value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasDisposition() => $_has(6);
  @$pb.TagNumber(7)
  void clearDisposition() => $_clearField(7);

  @$pb.TagNumber(8)
  OwnerCut get ownerCut => $_getN(7);
  @$pb.TagNumber(8)
  set ownerCut(OwnerCut value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasOwnerCut() => $_has(7);
  @$pb.TagNumber(8)
  void clearOwnerCut() => $_clearField(8);
  @$pb.TagNumber(8)
  OwnerCut ensureOwnerCut() => $_ensure(7);
}

class AccountObservation extends $pb.GeneratedMessage {
  factory AccountObservation({
    $core.String? accountId,
    AccountLifecycleFact? lifecycle,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (lifecycle != null) result.lifecycle = lifecycle;
    return result;
  }

  AccountObservation._();

  factory AccountObservation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AccountObservation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AccountObservation',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOM<AccountLifecycleFact>(2, _omitFieldNames ? '' : 'lifecycle',
        subBuilder: AccountLifecycleFact.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AccountObservation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AccountObservation copyWith(void Function(AccountObservation) updates) =>
      super.copyWith((message) => updates(message as AccountObservation))
          as AccountObservation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AccountObservation create() => AccountObservation._();
  @$core.override
  AccountObservation createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AccountObservation getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AccountObservation>(create);
  static AccountObservation? _defaultInstance;

  /// Echoes exactly one requested selector; never an account enumeration.
  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  AccountLifecycleFact get lifecycle => $_getN(1);
  @$pb.TagNumber(2)
  set lifecycle(AccountLifecycleFact value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasLifecycle() => $_has(1);
  @$pb.TagNumber(2)
  void clearLifecycle() => $_clearField(2);
  @$pb.TagNumber(2)
  AccountLifecycleFact ensureLifecycle() => $_ensure(1);
}

class ObserveAccountsRequest extends $pb.GeneratedMessage {
  factory ObserveAccountsRequest({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.Iterable<$core.String>? accountIds,
    RecoveryBinding? recovery,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (accountIds != null) result.accountIds.addAll(accountIds);
    if (recovery != null) result.recovery = recovery;
    return result;
  }

  ObserveAccountsRequest._();

  factory ObserveAccountsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveAccountsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveAccountsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..pPS(3, _omitFieldNames ? '' : 'accountIds')
    ..aOM<RecoveryBinding>(4, _omitFieldNames ? '' : 'recovery',
        subBuilder: RecoveryBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveAccountsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveAccountsRequest copyWith(
          void Function(ObserveAccountsRequest) updates) =>
      super.copyWith((message) => updates(message as ObserveAccountsRequest))
          as ObserveAccountsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveAccountsRequest create() => ObserveAccountsRequest._();
  @$core.override
  ObserveAccountsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveAccountsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveAccountsRequest>(create);
  static ObserveAccountsRequest? _defaultInstance;

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
  $pb.PbList<$core.String> get accountIds => $_getList(2);

  @$pb.TagNumber(4)
  RecoveryBinding get recovery => $_getN(3);
  @$pb.TagNumber(4)
  set recovery(RecoveryBinding value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasRecovery() => $_has(3);
  @$pb.TagNumber(4)
  void clearRecovery() => $_clearField(4);
  @$pb.TagNumber(4)
  RecoveryBinding ensureRecovery() => $_ensure(3);
}

class ObserveAccountsResponse extends $pb.GeneratedMessage {
  factory ObserveAccountsResponse({
    $core.int? protocolVersion,
    $core.String? requestId,
    ObservationWindow? window,
    $core.Iterable<AccountObservation>? accounts,
    OwnerCut? ownerCut,
    RecoveryBinding? recovery,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (window != null) result.window = window;
    if (accounts != null) result.accounts.addAll(accounts);
    if (ownerCut != null) result.ownerCut = ownerCut;
    if (recovery != null) result.recovery = recovery;
    return result;
  }

  ObserveAccountsResponse._();

  factory ObserveAccountsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveAccountsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveAccountsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOM<ObservationWindow>(3, _omitFieldNames ? '' : 'window',
        subBuilder: ObservationWindow.create)
    ..pPM<AccountObservation>(4, _omitFieldNames ? '' : 'accounts',
        subBuilder: AccountObservation.create)
    ..aOM<OwnerCut>(5, _omitFieldNames ? '' : 'ownerCut',
        subBuilder: OwnerCut.create)
    ..aOM<RecoveryBinding>(6, _omitFieldNames ? '' : 'recovery',
        subBuilder: RecoveryBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveAccountsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveAccountsResponse copyWith(
          void Function(ObserveAccountsResponse) updates) =>
      super.copyWith((message) => updates(message as ObserveAccountsResponse))
          as ObserveAccountsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveAccountsResponse create() => ObserveAccountsResponse._();
  @$core.override
  ObserveAccountsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveAccountsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveAccountsResponse>(create);
  static ObserveAccountsResponse? _defaultInstance;

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
  ObservationWindow get window => $_getN(2);
  @$pb.TagNumber(3)
  set window(ObservationWindow value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasWindow() => $_has(2);
  @$pb.TagNumber(3)
  void clearWindow() => $_clearField(3);
  @$pb.TagNumber(3)
  ObservationWindow ensureWindow() => $_ensure(2);

  @$pb.TagNumber(4)
  $pb.PbList<AccountObservation> get accounts => $_getList(3);

  @$pb.TagNumber(5)
  OwnerCut get ownerCut => $_getN(4);
  @$pb.TagNumber(5)
  set ownerCut(OwnerCut value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOwnerCut() => $_has(4);
  @$pb.TagNumber(5)
  void clearOwnerCut() => $_clearField(5);
  @$pb.TagNumber(5)
  OwnerCut ensureOwnerCut() => $_ensure(4);

  @$pb.TagNumber(6)
  RecoveryBinding get recovery => $_getN(5);
  @$pb.TagNumber(6)
  set recovery(RecoveryBinding value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasRecovery() => $_has(5);
  @$pb.TagNumber(6)
  void clearRecovery() => $_clearField(6);
  @$pb.TagNumber(6)
  RecoveryBinding ensureRecovery() => $_ensure(5);
}

class ResolveAccountHandleRequest extends $pb.GeneratedMessage {
  factory ResolveAccountHandleRequest({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.String? accountId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  ResolveAccountHandleRequest._();

  factory ResolveAccountHandleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveAccountHandleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveAccountHandleRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOS(3, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveAccountHandleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveAccountHandleRequest copyWith(
          void Function(ResolveAccountHandleRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveAccountHandleRequest))
          as ResolveAccountHandleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveAccountHandleRequest create() =>
      ResolveAccountHandleRequest._();
  @$core.override
  ResolveAccountHandleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveAccountHandleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveAccountHandleRequest>(create);
  static ResolveAccountHandleRequest? _defaultInstance;

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
  $core.String get accountId => $_getSZ(2);
  @$pb.TagNumber(3)
  set accountId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAccountId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAccountId() => $_clearField(3);
}

class ResolveAccountHandleResponse extends $pb.GeneratedMessage {
  factory ResolveAccountHandleResponse({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.String? accountId,
    ObservationWindow? window,
    AccountLifecycleFact? lifecycle,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (accountId != null) result.accountId = accountId;
    if (window != null) result.window = window;
    if (lifecycle != null) result.lifecycle = lifecycle;
    return result;
  }

  ResolveAccountHandleResponse._();

  factory ResolveAccountHandleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveAccountHandleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveAccountHandleResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOS(3, _omitFieldNames ? '' : 'accountId')
    ..aOM<ObservationWindow>(4, _omitFieldNames ? '' : 'window',
        subBuilder: ObservationWindow.create)
    ..aOM<AccountLifecycleFact>(5, _omitFieldNames ? '' : 'lifecycle',
        subBuilder: AccountLifecycleFact.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveAccountHandleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveAccountHandleResponse copyWith(
          void Function(ResolveAccountHandleResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveAccountHandleResponse))
          as ResolveAccountHandleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveAccountHandleResponse create() =>
      ResolveAccountHandleResponse._();
  @$core.override
  ResolveAccountHandleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveAccountHandleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveAccountHandleResponse>(create);
  static ResolveAccountHandleResponse? _defaultInstance;

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
  $core.String get accountId => $_getSZ(2);
  @$pb.TagNumber(3)
  set accountId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAccountId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAccountId() => $_clearField(3);

  @$pb.TagNumber(4)
  ObservationWindow get window => $_getN(3);
  @$pb.TagNumber(4)
  set window(ObservationWindow value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasWindow() => $_has(3);
  @$pb.TagNumber(4)
  void clearWindow() => $_clearField(4);
  @$pb.TagNumber(4)
  ObservationWindow ensureWindow() => $_ensure(3);

  @$pb.TagNumber(5)
  AccountLifecycleFact get lifecycle => $_getN(4);
  @$pb.TagNumber(5)
  set lifecycle(AccountLifecycleFact value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasLifecycle() => $_has(4);
  @$pb.TagNumber(5)
  void clearLifecycle() => $_clearField(5);
  @$pb.TagNumber(5)
  AccountLifecycleFact ensureLifecycle() => $_ensure(4);
}

class ObserveAccountHandlesRequest extends $pb.GeneratedMessage {
  factory ObserveAccountHandlesRequest({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.Iterable<$core.String>? ownerHandles,
    OwnerCut? requiredAuthCut,
    RecoveryBinding? recovery,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (ownerHandles != null) result.ownerHandles.addAll(ownerHandles);
    if (requiredAuthCut != null) result.requiredAuthCut = requiredAuthCut;
    if (recovery != null) result.recovery = recovery;
    return result;
  }

  ObserveAccountHandlesRequest._();

  factory ObserveAccountHandlesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveAccountHandlesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveAccountHandlesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..pPS(3, _omitFieldNames ? '' : 'ownerHandles')
    ..aOM<OwnerCut>(4, _omitFieldNames ? '' : 'requiredAuthCut',
        subBuilder: OwnerCut.create)
    ..aOM<RecoveryBinding>(5, _omitFieldNames ? '' : 'recovery',
        subBuilder: RecoveryBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveAccountHandlesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveAccountHandlesRequest copyWith(
          void Function(ObserveAccountHandlesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ObserveAccountHandlesRequest))
          as ObserveAccountHandlesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveAccountHandlesRequest create() =>
      ObserveAccountHandlesRequest._();
  @$core.override
  ObserveAccountHandlesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveAccountHandlesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveAccountHandlesRequest>(create);
  static ObserveAccountHandlesRequest? _defaultInstance;

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
  $pb.PbList<$core.String> get ownerHandles => $_getList(2);

  @$pb.TagNumber(4)
  OwnerCut get requiredAuthCut => $_getN(3);
  @$pb.TagNumber(4)
  set requiredAuthCut(OwnerCut value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasRequiredAuthCut() => $_has(3);
  @$pb.TagNumber(4)
  void clearRequiredAuthCut() => $_clearField(4);
  @$pb.TagNumber(4)
  OwnerCut ensureRequiredAuthCut() => $_ensure(3);

  @$pb.TagNumber(5)
  RecoveryBinding get recovery => $_getN(4);
  @$pb.TagNumber(5)
  set recovery(RecoveryBinding value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasRecovery() => $_has(4);
  @$pb.TagNumber(5)
  void clearRecovery() => $_clearField(5);
  @$pb.TagNumber(5)
  RecoveryBinding ensureRecovery() => $_ensure(4);
}

class ObserveAccountHandlesResponse extends $pb.GeneratedMessage {
  factory ObserveAccountHandlesResponse({
    $core.int? protocolVersion,
    $core.String? requestId,
    ObservationWindow? window,
    $core.Iterable<AccountLifecycleFact>? accounts,
    OwnerCut? ownerCut,
    RecoveryBinding? recovery,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (window != null) result.window = window;
    if (accounts != null) result.accounts.addAll(accounts);
    if (ownerCut != null) result.ownerCut = ownerCut;
    if (recovery != null) result.recovery = recovery;
    return result;
  }

  ObserveAccountHandlesResponse._();

  factory ObserveAccountHandlesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveAccountHandlesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveAccountHandlesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOM<ObservationWindow>(3, _omitFieldNames ? '' : 'window',
        subBuilder: ObservationWindow.create)
    ..pPM<AccountLifecycleFact>(4, _omitFieldNames ? '' : 'accounts',
        subBuilder: AccountLifecycleFact.create)
    ..aOM<OwnerCut>(5, _omitFieldNames ? '' : 'ownerCut',
        subBuilder: OwnerCut.create)
    ..aOM<RecoveryBinding>(6, _omitFieldNames ? '' : 'recovery',
        subBuilder: RecoveryBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveAccountHandlesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveAccountHandlesResponse copyWith(
          void Function(ObserveAccountHandlesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ObserveAccountHandlesResponse))
          as ObserveAccountHandlesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveAccountHandlesResponse create() =>
      ObserveAccountHandlesResponse._();
  @$core.override
  ObserveAccountHandlesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveAccountHandlesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveAccountHandlesResponse>(create);
  static ObserveAccountHandlesResponse? _defaultInstance;

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
  ObservationWindow get window => $_getN(2);
  @$pb.TagNumber(3)
  set window(ObservationWindow value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasWindow() => $_has(2);
  @$pb.TagNumber(3)
  void clearWindow() => $_clearField(3);
  @$pb.TagNumber(3)
  ObservationWindow ensureWindow() => $_ensure(2);

  /// One bound fact per input handle, including explicit unknown/incomplete.
  @$pb.TagNumber(4)
  $pb.PbList<AccountLifecycleFact> get accounts => $_getList(3);

  @$pb.TagNumber(5)
  OwnerCut get ownerCut => $_getN(4);
  @$pb.TagNumber(5)
  set ownerCut(OwnerCut value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOwnerCut() => $_has(4);
  @$pb.TagNumber(5)
  void clearOwnerCut() => $_clearField(5);
  @$pb.TagNumber(5)
  OwnerCut ensureOwnerCut() => $_ensure(4);

  @$pb.TagNumber(6)
  RecoveryBinding get recovery => $_getN(5);
  @$pb.TagNumber(6)
  set recovery(RecoveryBinding value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasRecovery() => $_has(5);
  @$pb.TagNumber(6)
  void clearRecovery() => $_clearField(6);
  @$pb.TagNumber(6)
  RecoveryBinding ensureRecovery() => $_ensure(5);
}

class ProfileObservation extends $pb.GeneratedMessage {
  factory ProfileObservation({
    $core.String? profileId,
    $fixnum.Int64? lifecycleRevision,
    Eligibility? eligibility,
    ErasureDisposition? disposition,
    $1.Timestamp? eraseAt,
    $core.String? parentOwnerHandle,
    AccountLifecycleFact? terminalParent,
    OwnerCut? ownerCut,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (lifecycleRevision != null) result.lifecycleRevision = lifecycleRevision;
    if (eligibility != null) result.eligibility = eligibility;
    if (disposition != null) result.disposition = disposition;
    if (eraseAt != null) result.eraseAt = eraseAt;
    if (parentOwnerHandle != null) result.parentOwnerHandle = parentOwnerHandle;
    if (terminalParent != null) result.terminalParent = terminalParent;
    if (ownerCut != null) result.ownerCut = ownerCut;
    return result;
  }

  ProfileObservation._();

  factory ProfileObservation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ProfileObservation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProfileObservation',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'lifecycleRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<Eligibility>(3, _omitFieldNames ? '' : 'eligibility',
        enumValues: Eligibility.values)
    ..aE<ErasureDisposition>(4, _omitFieldNames ? '' : 'disposition',
        enumValues: ErasureDisposition.values)
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'eraseAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(6, _omitFieldNames ? '' : 'parentOwnerHandle')
    ..aOM<AccountLifecycleFact>(7, _omitFieldNames ? '' : 'terminalParent',
        subBuilder: AccountLifecycleFact.create)
    ..aOM<OwnerCut>(8, _omitFieldNames ? '' : 'ownerCut',
        subBuilder: OwnerCut.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileObservation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileObservation copyWith(void Function(ProfileObservation) updates) =>
      super.copyWith((message) => updates(message as ProfileObservation))
          as ProfileObservation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ProfileObservation create() => ProfileObservation._();
  @$core.override
  ProfileObservation createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ProfileObservation getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProfileObservation>(create);
  static ProfileObservation? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get lifecycleRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set lifecycleRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLifecycleRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearLifecycleRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  Eligibility get eligibility => $_getN(2);
  @$pb.TagNumber(3)
  set eligibility(Eligibility value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasEligibility() => $_has(2);
  @$pb.TagNumber(3)
  void clearEligibility() => $_clearField(3);

  @$pb.TagNumber(4)
  ErasureDisposition get disposition => $_getN(3);
  @$pb.TagNumber(4)
  set disposition(ErasureDisposition value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasDisposition() => $_has(3);
  @$pb.TagNumber(4)
  void clearDisposition() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get eraseAt => $_getN(4);
  @$pb.TagNumber(5)
  set eraseAt($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasEraseAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearEraseAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureEraseAt() => $_ensure(4);

  /// Present only when canonical historical ownership is actually proven.
  @$pb.TagNumber(6)
  $core.String get parentOwnerHandle => $_getSZ(5);
  @$pb.TagNumber(6)
  set parentOwnerHandle($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasParentOwnerHandle() => $_has(5);
  @$pb.TagNumber(6)
  void clearParentOwnerHandle() => $_clearField(6);

  /// Exact positive parent fact for derived account-terminal erasure, not an
  /// inferred account/profile UUID match or a deadline derived from an event.
  @$pb.TagNumber(7)
  AccountLifecycleFact get terminalParent => $_getN(6);
  @$pb.TagNumber(7)
  set terminalParent(AccountLifecycleFact value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasTerminalParent() => $_has(6);
  @$pb.TagNumber(7)
  void clearTerminalParent() => $_clearField(7);
  @$pb.TagNumber(7)
  AccountLifecycleFact ensureTerminalParent() => $_ensure(6);

  @$pb.TagNumber(8)
  OwnerCut get ownerCut => $_getN(7);
  @$pb.TagNumber(8)
  set ownerCut(OwnerCut value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasOwnerCut() => $_has(7);
  @$pb.TagNumber(8)
  void clearOwnerCut() => $_clearField(8);
  @$pb.TagNumber(8)
  OwnerCut ensureOwnerCut() => $_ensure(7);
}

class ObserveProfilesRequest extends $pb.GeneratedMessage {
  factory ObserveProfilesRequest({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.Iterable<$core.String>? profileIds,
    RecoveryBinding? recovery,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (recovery != null) result.recovery = recovery;
    return result;
  }

  ObserveProfilesRequest._();

  factory ObserveProfilesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveProfilesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveProfilesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..pPS(3, _omitFieldNames ? '' : 'profileIds')
    ..aOM<RecoveryBinding>(4, _omitFieldNames ? '' : 'recovery',
        subBuilder: RecoveryBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveProfilesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveProfilesRequest copyWith(
          void Function(ObserveProfilesRequest) updates) =>
      super.copyWith((message) => updates(message as ObserveProfilesRequest))
          as ObserveProfilesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveProfilesRequest create() => ObserveProfilesRequest._();
  @$core.override
  ObserveProfilesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveProfilesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveProfilesRequest>(create);
  static ObserveProfilesRequest? _defaultInstance;

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
  $pb.PbList<$core.String> get profileIds => $_getList(2);

  @$pb.TagNumber(4)
  RecoveryBinding get recovery => $_getN(3);
  @$pb.TagNumber(4)
  set recovery(RecoveryBinding value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasRecovery() => $_has(3);
  @$pb.TagNumber(4)
  void clearRecovery() => $_clearField(4);
  @$pb.TagNumber(4)
  RecoveryBinding ensureRecovery() => $_ensure(3);
}

class ObserveProfilesResponse extends $pb.GeneratedMessage {
  factory ObserveProfilesResponse({
    $core.int? protocolVersion,
    $core.String? requestId,
    ObservationWindow? window,
    $core.Iterable<ProfileObservation>? profiles,
    OwnerCut? ownerCut,
    RecoveryBinding? recovery,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (window != null) result.window = window;
    if (profiles != null) result.profiles.addAll(profiles);
    if (ownerCut != null) result.ownerCut = ownerCut;
    if (recovery != null) result.recovery = recovery;
    return result;
  }

  ObserveProfilesResponse._();

  factory ObserveProfilesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveProfilesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveProfilesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOM<ObservationWindow>(3, _omitFieldNames ? '' : 'window',
        subBuilder: ObservationWindow.create)
    ..pPM<ProfileObservation>(4, _omitFieldNames ? '' : 'profiles',
        subBuilder: ProfileObservation.create)
    ..aOM<OwnerCut>(5, _omitFieldNames ? '' : 'ownerCut',
        subBuilder: OwnerCut.create)
    ..aOM<RecoveryBinding>(6, _omitFieldNames ? '' : 'recovery',
        subBuilder: RecoveryBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveProfilesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveProfilesResponse copyWith(
          void Function(ObserveProfilesResponse) updates) =>
      super.copyWith((message) => updates(message as ObserveProfilesResponse))
          as ObserveProfilesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveProfilesResponse create() => ObserveProfilesResponse._();
  @$core.override
  ObserveProfilesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveProfilesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveProfilesResponse>(create);
  static ObserveProfilesResponse? _defaultInstance;

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
  ObservationWindow get window => $_getN(2);
  @$pb.TagNumber(3)
  set window(ObservationWindow value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasWindow() => $_has(2);
  @$pb.TagNumber(3)
  void clearWindow() => $_clearField(3);
  @$pb.TagNumber(3)
  ObservationWindow ensureWindow() => $_ensure(2);

  @$pb.TagNumber(4)
  $pb.PbList<ProfileObservation> get profiles => $_getList(3);

  @$pb.TagNumber(5)
  OwnerCut get ownerCut => $_getN(4);
  @$pb.TagNumber(5)
  set ownerCut(OwnerCut value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOwnerCut() => $_has(4);
  @$pb.TagNumber(5)
  void clearOwnerCut() => $_clearField(5);
  @$pb.TagNumber(5)
  OwnerCut ensureOwnerCut() => $_ensure(4);

  @$pb.TagNumber(6)
  RecoveryBinding get recovery => $_getN(5);
  @$pb.TagNumber(6)
  set recovery(RecoveryBinding value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasRecovery() => $_has(5);
  @$pb.TagNumber(6)
  void clearRecovery() => $_clearField(6);
  @$pb.TagNumber(6)
  RecoveryBinding ensureRecovery() => $_ensure(5);
}

class ResolveHistoricalCoverageRequest extends $pb.GeneratedMessage {
  factory ResolveHistoricalCoverageRequest({
    $core.int? protocolVersion,
    $core.String? requestId,
    $core.Iterable<$core.String>? profileIds,
    $core.Iterable<AccountLifecycleFact>? requiredAccounts,
    OwnerCut? requiredAuthCut,
    RecoveryBinding? recovery,
    OwnerCut? requiredUserCut,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (requiredAccounts != null)
      result.requiredAccounts.addAll(requiredAccounts);
    if (requiredAuthCut != null) result.requiredAuthCut = requiredAuthCut;
    if (recovery != null) result.recovery = recovery;
    if (requiredUserCut != null) result.requiredUserCut = requiredUserCut;
    return result;
  }

  ResolveHistoricalCoverageRequest._();

  factory ResolveHistoricalCoverageRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveHistoricalCoverageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveHistoricalCoverageRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..pPS(3, _omitFieldNames ? '' : 'profileIds')
    ..pPM<AccountLifecycleFact>(4, _omitFieldNames ? '' : 'requiredAccounts',
        subBuilder: AccountLifecycleFact.create)
    ..aOM<OwnerCut>(5, _omitFieldNames ? '' : 'requiredAuthCut',
        subBuilder: OwnerCut.create)
    ..aOM<RecoveryBinding>(6, _omitFieldNames ? '' : 'recovery',
        subBuilder: RecoveryBinding.create)
    ..aOM<OwnerCut>(7, _omitFieldNames ? '' : 'requiredUserCut',
        subBuilder: OwnerCut.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveHistoricalCoverageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveHistoricalCoverageRequest copyWith(
          void Function(ResolveHistoricalCoverageRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveHistoricalCoverageRequest))
          as ResolveHistoricalCoverageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveHistoricalCoverageRequest create() =>
      ResolveHistoricalCoverageRequest._();
  @$core.override
  ResolveHistoricalCoverageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveHistoricalCoverageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveHistoricalCoverageRequest>(
          create);
  static ResolveHistoricalCoverageRequest? _defaultInstance;

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
  $pb.PbList<$core.String> get profileIds => $_getList(2);

  /// Exact positive account facts requested for account cleanup. No live-only
  /// listing, guessed parent binding or substituted Mod-purpose proof is valid.
  @$pb.TagNumber(4)
  $pb.PbList<AccountLifecycleFact> get requiredAccounts => $_getList(3);

  @$pb.TagNumber(5)
  OwnerCut get requiredAuthCut => $_getN(4);
  @$pb.TagNumber(5)
  set requiredAuthCut(OwnerCut value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasRequiredAuthCut() => $_has(4);
  @$pb.TagNumber(5)
  void clearRequiredAuthCut() => $_clearField(5);
  @$pb.TagNumber(5)
  OwnerCut ensureRequiredAuthCut() => $_ensure(4);

  @$pb.TagNumber(6)
  RecoveryBinding get recovery => $_getN(5);
  @$pb.TagNumber(6)
  set recovery(RecoveryBinding value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasRecovery() => $_has(5);
  @$pb.TagNumber(6)
  void clearRecovery() => $_clearField(6);
  @$pb.TagNumber(6)
  RecoveryBinding ensureRecovery() => $_ensure(5);

  @$pb.TagNumber(7)
  OwnerCut get requiredUserCut => $_getN(6);
  @$pb.TagNumber(7)
  set requiredUserCut(OwnerCut value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasRequiredUserCut() => $_has(6);
  @$pb.TagNumber(7)
  void clearRequiredUserCut() => $_clearField(7);
  @$pb.TagNumber(7)
  OwnerCut ensureRequiredUserCut() => $_ensure(6);
}

class ResolveHistoricalCoverageResponse extends $pb.GeneratedMessage {
  factory ResolveHistoricalCoverageResponse({
    $core.int? protocolVersion,
    $core.String? requestId,
    ObservationWindow? window,
    $core.Iterable<ProfileObservation>? profiles,
    OwnerCut? ownerCut,
    OwnerCut? importedAuthCut,
    $fixnum.Int64? matchedAssociationCount,
    $core.List<$core.int>? associationDigest,
    $core.bool? complete,
    RecoveryBinding? recovery,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (requestId != null) result.requestId = requestId;
    if (window != null) result.window = window;
    if (profiles != null) result.profiles.addAll(profiles);
    if (ownerCut != null) result.ownerCut = ownerCut;
    if (importedAuthCut != null) result.importedAuthCut = importedAuthCut;
    if (matchedAssociationCount != null)
      result.matchedAssociationCount = matchedAssociationCount;
    if (associationDigest != null) result.associationDigest = associationDigest;
    if (complete != null) result.complete = complete;
    if (recovery != null) result.recovery = recovery;
    return result;
  }

  ResolveHistoricalCoverageResponse._();

  factory ResolveHistoricalCoverageResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveHistoricalCoverageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveHistoricalCoverageResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.identityerasure.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOM<ObservationWindow>(3, _omitFieldNames ? '' : 'window',
        subBuilder: ObservationWindow.create)
    ..pPM<ProfileObservation>(4, _omitFieldNames ? '' : 'profiles',
        subBuilder: ProfileObservation.create)
    ..aOM<OwnerCut>(5, _omitFieldNames ? '' : 'ownerCut',
        subBuilder: OwnerCut.create)
    ..aOM<OwnerCut>(6, _omitFieldNames ? '' : 'importedAuthCut',
        subBuilder: OwnerCut.create)
    ..a<$fixnum.Int64>(7, _omitFieldNames ? '' : 'matchedAssociationCount',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'associationDigest', $pb.PbFieldType.OY)
    ..aOB(9, _omitFieldNames ? '' : 'complete')
    ..aOM<RecoveryBinding>(10, _omitFieldNames ? '' : 'recovery',
        subBuilder: RecoveryBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveHistoricalCoverageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveHistoricalCoverageResponse copyWith(
          void Function(ResolveHistoricalCoverageResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ResolveHistoricalCoverageResponse))
          as ResolveHistoricalCoverageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveHistoricalCoverageResponse create() =>
      ResolveHistoricalCoverageResponse._();
  @$core.override
  ResolveHistoricalCoverageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveHistoricalCoverageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveHistoricalCoverageResponse>(
          create);
  static ResolveHistoricalCoverageResponse? _defaultInstance;

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
  ObservationWindow get window => $_getN(2);
  @$pb.TagNumber(3)
  set window(ObservationWindow value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasWindow() => $_has(2);
  @$pb.TagNumber(3)
  void clearWindow() => $_clearField(3);
  @$pb.TagNumber(3)
  ObservationWindow ensureWindow() => $_ensure(2);

  @$pb.TagNumber(4)
  $pb.PbList<ProfileObservation> get profiles => $_getList(3);

  @$pb.TagNumber(5)
  OwnerCut get ownerCut => $_getN(4);
  @$pb.TagNumber(5)
  set ownerCut(OwnerCut value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOwnerCut() => $_has(4);
  @$pb.TagNumber(5)
  void clearOwnerCut() => $_clearField(5);
  @$pb.TagNumber(5)
  OwnerCut ensureOwnerCut() => $_ensure(4);

  /// User has causally imported the EXACT required Auth cut. The selected
  /// historical roster's count/digest covers archived/hard-deleted associations.
  @$pb.TagNumber(6)
  OwnerCut get importedAuthCut => $_getN(5);
  @$pb.TagNumber(6)
  set importedAuthCut(OwnerCut value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasImportedAuthCut() => $_has(5);
  @$pb.TagNumber(6)
  void clearImportedAuthCut() => $_clearField(6);
  @$pb.TagNumber(6)
  OwnerCut ensureImportedAuthCut() => $_ensure(5);

  @$pb.TagNumber(7)
  $fixnum.Int64 get matchedAssociationCount => $_getI64(6);
  @$pb.TagNumber(7)
  set matchedAssociationCount($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasMatchedAssociationCount() => $_has(6);
  @$pb.TagNumber(7)
  void clearMatchedAssociationCount() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get associationDigest => $_getN(7);
  @$pb.TagNumber(8)
  set associationDigest($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAssociationDigest() => $_has(7);
  @$pb.TagNumber(8)
  void clearAssociationDigest() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get complete => $_getBF(8);
  @$pb.TagNumber(9)
  set complete($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasComplete() => $_has(8);
  @$pb.TagNumber(9)
  void clearComplete() => $_clearField(9);

  @$pb.TagNumber(10)
  RecoveryBinding get recovery => $_getN(9);
  @$pb.TagNumber(10)
  set recovery(RecoveryBinding value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasRecovery() => $_has(9);
  @$pb.TagNumber(10)
  void clearRecovery() => $_clearField(10);
  @$pb.TagNumber(10)
  RecoveryBinding ensureRecovery() => $_ensure(9);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
