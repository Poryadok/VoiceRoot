// This is a generated file - do not edit.
//
// Generated from voice/auth/v1/subscription_lifecycle.proto.

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

import 'subscription_lifecycle.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'subscription_lifecycle.pbenum.dart';

class ObserveSubscriptionAccountRequest extends $pb.GeneratedMessage {
  factory ObserveSubscriptionAccountRequest({
    $core.String? accountId,
    $fixnum.Int64? minimumLifecycleRevision,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (minimumLifecycleRevision != null)
      result.minimumLifecycleRevision = minimumLifecycleRevision;
    return result;
  }

  ObserveSubscriptionAccountRequest._();

  factory ObserveSubscriptionAccountRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveSubscriptionAccountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveSubscriptionAccountRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..a<$fixnum.Int64>(2, _omitFieldNames ? '' : 'minimumLifecycleRevision',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveSubscriptionAccountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveSubscriptionAccountRequest copyWith(
          void Function(ObserveSubscriptionAccountRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ObserveSubscriptionAccountRequest))
          as ObserveSubscriptionAccountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveSubscriptionAccountRequest create() =>
      ObserveSubscriptionAccountRequest._();
  @$core.override
  ObserveSubscriptionAccountRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveSubscriptionAccountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveSubscriptionAccountRequest>(
          create);
  static ObserveSubscriptionAccountRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get minimumLifecycleRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set minimumLifecycleRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMinimumLifecycleRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearMinimumLifecycleRevision() => $_clearField(2);
}

class SubscriptionAccountLifecycleFact extends $pb.GeneratedMessage {
  factory SubscriptionAccountLifecycleFact({
    $core.String? accountId,
    $fixnum.Int64? sourceRevision,
    $core.String? eventId,
    $core.String? deletionCycleId,
    SubscriptionAccountLifecycleKind? kind,
    $fixnum.Int64? sessionEpoch,
    $1.Timestamp? scheduledAt,
    $1.Timestamp? purgeAt,
    $1.Timestamp? effectiveAt,
    $1.Timestamp? capturedAt,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (sourceRevision != null) result.sourceRevision = sourceRevision;
    if (eventId != null) result.eventId = eventId;
    if (deletionCycleId != null) result.deletionCycleId = deletionCycleId;
    if (kind != null) result.kind = kind;
    if (sessionEpoch != null) result.sessionEpoch = sessionEpoch;
    if (scheduledAt != null) result.scheduledAt = scheduledAt;
    if (purgeAt != null) result.purgeAt = purgeAt;
    if (effectiveAt != null) result.effectiveAt = effectiveAt;
    if (capturedAt != null) result.capturedAt = capturedAt;
    return result;
  }

  SubscriptionAccountLifecycleFact._();

  factory SubscriptionAccountLifecycleFact.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SubscriptionAccountLifecycleFact.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubscriptionAccountLifecycleFact',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'sourceRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(3, _omitFieldNames ? '' : 'eventId')
    ..aOS(4, _omitFieldNames ? '' : 'deletionCycleId')
    ..aE<SubscriptionAccountLifecycleKind>(5, _omitFieldNames ? '' : 'kind',
        enumValues: SubscriptionAccountLifecycleKind.values)
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'sessionEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'scheduledAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'purgeAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'effectiveAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'capturedAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscriptionAccountLifecycleFact clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscriptionAccountLifecycleFact copyWith(
          void Function(SubscriptionAccountLifecycleFact) updates) =>
      super.copyWith(
              (message) => updates(message as SubscriptionAccountLifecycleFact))
          as SubscriptionAccountLifecycleFact;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SubscriptionAccountLifecycleFact create() =>
      SubscriptionAccountLifecycleFact._();
  @$core.override
  SubscriptionAccountLifecycleFact createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SubscriptionAccountLifecycleFact getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubscriptionAccountLifecycleFact>(
          create);
  static SubscriptionAccountLifecycleFact? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get sourceRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set sourceRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSourceRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearSourceRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get eventId => $_getSZ(2);
  @$pb.TagNumber(3)
  set eventId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEventId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEventId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deletionCycleId => $_getSZ(3);
  @$pb.TagNumber(4)
  set deletionCycleId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeletionCycleId() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeletionCycleId() => $_clearField(4);

  @$pb.TagNumber(5)
  SubscriptionAccountLifecycleKind get kind => $_getN(4);
  @$pb.TagNumber(5)
  set kind(SubscriptionAccountLifecycleKind value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasKind() => $_has(4);
  @$pb.TagNumber(5)
  void clearKind() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get sessionEpoch => $_getI64(5);
  @$pb.TagNumber(6)
  set sessionEpoch($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSessionEpoch() => $_has(5);
  @$pb.TagNumber(6)
  void clearSessionEpoch() => $_clearField(6);

  @$pb.TagNumber(7)
  $1.Timestamp get scheduledAt => $_getN(6);
  @$pb.TagNumber(7)
  set scheduledAt($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasScheduledAt() => $_has(6);
  @$pb.TagNumber(7)
  void clearScheduledAt() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensureScheduledAt() => $_ensure(6);

  @$pb.TagNumber(8)
  $1.Timestamp get purgeAt => $_getN(7);
  @$pb.TagNumber(8)
  set purgeAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasPurgeAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearPurgeAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensurePurgeAt() => $_ensure(7);

  @$pb.TagNumber(9)
  $1.Timestamp get effectiveAt => $_getN(8);
  @$pb.TagNumber(9)
  set effectiveAt($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasEffectiveAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearEffectiveAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureEffectiveAt() => $_ensure(8);

  @$pb.TagNumber(10)
  $1.Timestamp get capturedAt => $_getN(9);
  @$pb.TagNumber(10)
  set capturedAt($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCapturedAt() => $_has(9);
  @$pb.TagNumber(10)
  void clearCapturedAt() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureCapturedAt() => $_ensure(9);
}

class ObserveSubscriptionAccountResponse extends $pb.GeneratedMessage {
  factory ObserveSubscriptionAccountResponse({
    $core.int? protocolVersion,
    $core.String? accountId,
    $core.String? accountType,
    $core.String? accountStatus,
    $core.bool? regularEmailVerificationPending,
    $fixnum.Int64? sessionEpoch,
    $fixnum.Int64? securityRevision,
    $1.Timestamp? deletedAt,
    $1.Timestamp? observedAt,
    SubscriptionAccountCaptureCoverage? captureCoverage,
    SubscriptionAccountLifecycleFact? head,
    $core.Iterable<SubscriptionAccountLifecycleFact>? facts,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (accountId != null) result.accountId = accountId;
    if (accountType != null) result.accountType = accountType;
    if (accountStatus != null) result.accountStatus = accountStatus;
    if (regularEmailVerificationPending != null)
      result.regularEmailVerificationPending = regularEmailVerificationPending;
    if (sessionEpoch != null) result.sessionEpoch = sessionEpoch;
    if (securityRevision != null) result.securityRevision = securityRevision;
    if (deletedAt != null) result.deletedAt = deletedAt;
    if (observedAt != null) result.observedAt = observedAt;
    if (captureCoverage != null) result.captureCoverage = captureCoverage;
    if (head != null) result.head = head;
    if (facts != null) result.facts.addAll(facts);
    return result;
  }

  ObserveSubscriptionAccountResponse._();

  factory ObserveSubscriptionAccountResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveSubscriptionAccountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveSubscriptionAccountResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..aOS(3, _omitFieldNames ? '' : 'accountType')
    ..aOS(4, _omitFieldNames ? '' : 'accountStatus')
    ..aOB(5, _omitFieldNames ? '' : 'regularEmailVerificationPending')
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'sessionEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'securityRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'deletedAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'observedAt',
        subBuilder: $1.Timestamp.create)
    ..aE<SubscriptionAccountCaptureCoverage>(
        10, _omitFieldNames ? '' : 'captureCoverage',
        enumValues: SubscriptionAccountCaptureCoverage.values)
    ..aOM<SubscriptionAccountLifecycleFact>(11, _omitFieldNames ? '' : 'head',
        subBuilder: SubscriptionAccountLifecycleFact.create)
    ..pPM<SubscriptionAccountLifecycleFact>(12, _omitFieldNames ? '' : 'facts',
        subBuilder: SubscriptionAccountLifecycleFact.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveSubscriptionAccountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveSubscriptionAccountResponse copyWith(
          void Function(ObserveSubscriptionAccountResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ObserveSubscriptionAccountResponse))
          as ObserveSubscriptionAccountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveSubscriptionAccountResponse create() =>
      ObserveSubscriptionAccountResponse._();
  @$core.override
  ObserveSubscriptionAccountResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveSubscriptionAccountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveSubscriptionAccountResponse>(
          create);
  static ObserveSubscriptionAccountResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get accountType => $_getSZ(2);
  @$pb.TagNumber(3)
  set accountType($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAccountType() => $_has(2);
  @$pb.TagNumber(3)
  void clearAccountType() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get accountStatus => $_getSZ(3);
  @$pb.TagNumber(4)
  set accountStatus($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAccountStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearAccountStatus() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get regularEmailVerificationPending => $_getBF(4);
  @$pb.TagNumber(5)
  set regularEmailVerificationPending($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRegularEmailVerificationPending() => $_has(4);
  @$pb.TagNumber(5)
  void clearRegularEmailVerificationPending() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get sessionEpoch => $_getI64(5);
  @$pb.TagNumber(6)
  set sessionEpoch($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSessionEpoch() => $_has(5);
  @$pb.TagNumber(6)
  void clearSessionEpoch() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get securityRevision => $_getI64(6);
  @$pb.TagNumber(7)
  set securityRevision($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSecurityRevision() => $_has(6);
  @$pb.TagNumber(7)
  void clearSecurityRevision() => $_clearField(7);

  @$pb.TagNumber(8)
  $1.Timestamp get deletedAt => $_getN(7);
  @$pb.TagNumber(8)
  set deletedAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasDeletedAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearDeletedAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureDeletedAt() => $_ensure(7);

  @$pb.TagNumber(9)
  $1.Timestamp get observedAt => $_getN(8);
  @$pb.TagNumber(9)
  set observedAt($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasObservedAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearObservedAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureObservedAt() => $_ensure(8);

  @$pb.TagNumber(10)
  SubscriptionAccountCaptureCoverage get captureCoverage => $_getN(9);
  @$pb.TagNumber(10)
  set captureCoverage(SubscriptionAccountCaptureCoverage value) =>
      $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCaptureCoverage() => $_has(9);
  @$pb.TagNumber(10)
  void clearCaptureCoverage() => $_clearField(10);

  @$pb.TagNumber(11)
  SubscriptionAccountLifecycleFact get head => $_getN(10);
  @$pb.TagNumber(11)
  set head(SubscriptionAccountLifecycleFact value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasHead() => $_has(10);
  @$pb.TagNumber(11)
  void clearHead() => $_clearField(11);
  @$pb.TagNumber(11)
  SubscriptionAccountLifecycleFact ensureHead() => $_ensure(10);

  @$pb.TagNumber(12)
  $pb.PbList<SubscriptionAccountLifecycleFact> get facts => $_getList(11);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
