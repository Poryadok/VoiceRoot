// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_actor.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'lfp_actor.pbenum.dart';
import 'lfp_consent.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'lfp_actor.pbenum.dart';

/// PRIVATE immutable original actor sidecar. Deterministic canonical fields1-12
/// exactly once, no unknown fields. A valid encoding/hash is data, not live auth.
class LfpActorProvenanceMaterial extends $pb.GeneratedMessage {
  factory LfpActorProvenanceMaterial({
    $fixnum.Int64? protocolVersion,
    LfpActorActionKind? actionKind,
    LfpActorKind? actorKind,
    LfpActorAccountType? accountType,
    $core.List<$core.int>? accountId,
    $core.List<$core.int>? profileId,
    $fixnum.Int64? sessionEpoch,
    $core.List<$core.int>? actionOperationId,
    $core.String? issuer,
    $core.String? audience,
    $core.String? fullRpc,
    $core.List<$core.int>? principalRequestSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (actionKind != null) result.actionKind = actionKind;
    if (actorKind != null) result.actorKind = actorKind;
    if (accountType != null) result.accountType = accountType;
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    if (sessionEpoch != null) result.sessionEpoch = sessionEpoch;
    if (actionOperationId != null) result.actionOperationId = actionOperationId;
    if (issuer != null) result.issuer = issuer;
    if (audience != null) result.audience = audience;
    if (fullRpc != null) result.fullRpc = fullRpc;
    if (principalRequestSha256 != null)
      result.principalRequestSha256 = principalRequestSha256;
    return result;
  }

  LfpActorProvenanceMaterial._();

  factory LfpActorProvenanceMaterial.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LfpActorProvenanceMaterial.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LfpActorProvenanceMaterial',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<LfpActorActionKind>(2, _omitFieldNames ? '' : 'actionKind',
        enumValues: LfpActorActionKind.values)
    ..aE<LfpActorKind>(3, _omitFieldNames ? '' : 'actorKind',
        enumValues: LfpActorKind.values)
    ..aE<LfpActorAccountType>(4, _omitFieldNames ? '' : 'accountType',
        enumValues: LfpActorAccountType.values)
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'accountId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'profileId', $pb.PbFieldType.OY)
    ..aInt64(7, _omitFieldNames ? '' : 'sessionEpoch')
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'actionOperationId', $pb.PbFieldType.OY)
    ..aOS(9, _omitFieldNames ? '' : 'issuer')
    ..aOS(10, _omitFieldNames ? '' : 'audience')
    ..aOS(11, _omitFieldNames ? '' : 'fullRpc')
    ..a<$core.List<$core.int>>(
        12, _omitFieldNames ? '' : 'principalRequestSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpActorProvenanceMaterial clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpActorProvenanceMaterial copyWith(
          void Function(LfpActorProvenanceMaterial) updates) =>
      super.copyWith(
              (message) => updates(message as LfpActorProvenanceMaterial))
          as LfpActorProvenanceMaterial;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LfpActorProvenanceMaterial create() => LfpActorProvenanceMaterial._();
  @$core.override
  LfpActorProvenanceMaterial createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LfpActorProvenanceMaterial getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LfpActorProvenanceMaterial>(create);
  static LfpActorProvenanceMaterial? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  LfpActorActionKind get actionKind => $_getN(1);
  @$pb.TagNumber(2)
  set actionKind(LfpActorActionKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasActionKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearActionKind() => $_clearField(2);

  @$pb.TagNumber(3)
  LfpActorKind get actorKind => $_getN(2);
  @$pb.TagNumber(3)
  set actorKind(LfpActorKind value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasActorKind() => $_has(2);
  @$pb.TagNumber(3)
  void clearActorKind() => $_clearField(3);

  @$pb.TagNumber(4)
  LfpActorAccountType get accountType => $_getN(3);
  @$pb.TagNumber(4)
  set accountType(LfpActorAccountType value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasAccountType() => $_has(3);
  @$pb.TagNumber(4)
  void clearAccountType() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get accountId => $_getN(4);
  @$pb.TagNumber(5)
  set accountId($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAccountId() => $_has(4);
  @$pb.TagNumber(5)
  void clearAccountId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get profileId => $_getN(5);
  @$pb.TagNumber(6)
  set profileId($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasProfileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearProfileId() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get sessionEpoch => $_getI64(6);
  @$pb.TagNumber(7)
  set sessionEpoch($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSessionEpoch() => $_has(6);
  @$pb.TagNumber(7)
  void clearSessionEpoch() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get actionOperationId => $_getN(7);
  @$pb.TagNumber(8)
  set actionOperationId($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasActionOperationId() => $_has(7);
  @$pb.TagNumber(8)
  void clearActionOperationId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get issuer => $_getSZ(8);
  @$pb.TagNumber(9)
  set issuer($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasIssuer() => $_has(8);
  @$pb.TagNumber(9)
  void clearIssuer() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get audience => $_getSZ(9);
  @$pb.TagNumber(10)
  set audience($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasAudience() => $_has(9);
  @$pb.TagNumber(10)
  void clearAudience() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get fullRpc => $_getSZ(10);
  @$pb.TagNumber(11)
  set fullRpc($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasFullRpc() => $_has(10);
  @$pb.TagNumber(11)
  void clearFullRpc() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.List<$core.int> get principalRequestSha256 => $_getN(11);
  @$pb.TagNumber(12)
  set principalRequestSha256($core.List<$core.int> value) =>
      $_setBytes(11, value);
  @$pb.TagNumber(12)
  $core.bool hasPrincipalRequestSha256() => $_has(11);
  @$pb.TagNumber(12)
  void clearPrincipalRequestSha256() => $_clearField(12);
}

/// Persisted original306/314 provenance. Only actual controller-admitted
/// credential/peer/DB composition establishes authority for these private bytes.
class LfpConsentOrigin extends $pb.GeneratedMessage {
  factory LfpConsentOrigin({
    $core.List<$core.int>? environmentId,
    $core.List<$core.int>? installationId,
    $core.List<$core.int>? authorityIncarnation,
    $core.List<$core.int>? databaseBindingSha256,
  }) {
    final result = create();
    if (environmentId != null) result.environmentId = environmentId;
    if (installationId != null) result.installationId = installationId;
    if (authorityIncarnation != null)
      result.authorityIncarnation = authorityIncarnation;
    if (databaseBindingSha256 != null)
      result.databaseBindingSha256 = databaseBindingSha256;
    return result;
  }

  LfpConsentOrigin._();

  factory LfpConsentOrigin.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LfpConsentOrigin.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LfpConsentOrigin',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$core.List<$core.int>>(
        1, _omitFieldNames ? '' : 'environmentId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'installationId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'authorityIncarnation', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'databaseBindingSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpConsentOrigin clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpConsentOrigin copyWith(void Function(LfpConsentOrigin) updates) =>
      super.copyWith((message) => updates(message as LfpConsentOrigin))
          as LfpConsentOrigin;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LfpConsentOrigin create() => LfpConsentOrigin._();
  @$core.override
  LfpConsentOrigin createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LfpConsentOrigin getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LfpConsentOrigin>(create);
  static LfpConsentOrigin? _defaultInstance;

  @$pb.TagNumber(1)
  $core.List<$core.int> get environmentId => $_getN(0);
  @$pb.TagNumber(1)
  set environmentId($core.List<$core.int> value) => $_setBytes(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEnvironmentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEnvironmentId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get installationId => $_getN(1);
  @$pb.TagNumber(2)
  set installationId($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInstallationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearInstallationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get authorityIncarnation => $_getN(2);
  @$pb.TagNumber(3)
  set authorityIncarnation($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAuthorityIncarnation() => $_has(2);
  @$pb.TagNumber(3)
  void clearAuthorityIncarnation() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get databaseBindingSha256 => $_getN(3);
  @$pb.TagNumber(4)
  set databaseBindingSha256($core.List<$core.int> value) =>
      $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDatabaseBindingSha256() => $_has(3);
  @$pb.TagNumber(4)
  void clearDatabaseBindingSha256() => $_clearField(4);
}

class LfpActorBoundConsentEnvelope extends $pb.GeneratedMessage {
  factory LfpActorBoundConsentEnvelope({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? canonicalV1Consent,
    $core.List<$core.int>? canonicalActor,
    LfpConsentOrigin? origin,
    $core.List<$core.int>? consentSha256,
    $core.List<$core.int>? actorSha256,
    $core.List<$core.int>? qualifiedConsentSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (canonicalV1Consent != null)
      result.canonicalV1Consent = canonicalV1Consent;
    if (canonicalActor != null) result.canonicalActor = canonicalActor;
    if (origin != null) result.origin = origin;
    if (consentSha256 != null) result.consentSha256 = consentSha256;
    if (actorSha256 != null) result.actorSha256 = actorSha256;
    if (qualifiedConsentSha256 != null)
      result.qualifiedConsentSha256 = qualifiedConsentSha256;
    return result;
  }

  LfpActorBoundConsentEnvelope._();

  factory LfpActorBoundConsentEnvelope.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LfpActorBoundConsentEnvelope.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LfpActorBoundConsentEnvelope',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'canonicalV1Consent', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'canonicalActor', $pb.PbFieldType.OY)
    ..aOM<LfpConsentOrigin>(4, _omitFieldNames ? '' : 'origin',
        subBuilder: LfpConsentOrigin.create)
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'consentSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'actorSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'qualifiedConsentSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpActorBoundConsentEnvelope clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpActorBoundConsentEnvelope copyWith(
          void Function(LfpActorBoundConsentEnvelope) updates) =>
      super.copyWith(
              (message) => updates(message as LfpActorBoundConsentEnvelope))
          as LfpActorBoundConsentEnvelope;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LfpActorBoundConsentEnvelope create() =>
      LfpActorBoundConsentEnvelope._();
  @$core.override
  LfpActorBoundConsentEnvelope createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LfpActorBoundConsentEnvelope getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LfpActorBoundConsentEnvelope>(create);
  static LfpActorBoundConsentEnvelope? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get canonicalV1Consent => $_getN(1);
  @$pb.TagNumber(2)
  set canonicalV1Consent($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCanonicalV1Consent() => $_has(1);
  @$pb.TagNumber(2)
  void clearCanonicalV1Consent() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get canonicalActor => $_getN(2);
  @$pb.TagNumber(3)
  set canonicalActor($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCanonicalActor() => $_has(2);
  @$pb.TagNumber(3)
  void clearCanonicalActor() => $_clearField(3);

  @$pb.TagNumber(4)
  LfpConsentOrigin get origin => $_getN(3);
  @$pb.TagNumber(4)
  set origin(LfpConsentOrigin value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasOrigin() => $_has(3);
  @$pb.TagNumber(4)
  void clearOrigin() => $_clearField(4);
  @$pb.TagNumber(4)
  LfpConsentOrigin ensureOrigin() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.List<$core.int> get consentSha256 => $_getN(4);
  @$pb.TagNumber(5)
  set consentSha256($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasConsentSha256() => $_has(4);
  @$pb.TagNumber(5)
  void clearConsentSha256() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get actorSha256 => $_getN(5);
  @$pb.TagNumber(6)
  set actorSha256($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasActorSha256() => $_has(5);
  @$pb.TagNumber(6)
  void clearActorSha256() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get qualifiedConsentSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set qualifiedConsentSha256($core.List<$core.int> value) =>
      $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasQualifiedConsentSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearQualifiedConsentSha256() => $_clearField(7);
}

/// All fields required. Base registration retains its unchangedv1 hash.
class LfpActorBoundRegistrationAck extends $pb.GeneratedMessage {
  factory LfpActorBoundRegistrationAck({
    $fixnum.Int64? protocolVersion,
    $1.LfpConsentRegistration? registration,
    LfpConsentOrigin? origin,
    $core.List<$core.int>? qualifiedConsentSha256,
    $core.List<$core.int>? actorBoundRegistrationSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (registration != null) result.registration = registration;
    if (origin != null) result.origin = origin;
    if (qualifiedConsentSha256 != null)
      result.qualifiedConsentSha256 = qualifiedConsentSha256;
    if (actorBoundRegistrationSha256 != null)
      result.actorBoundRegistrationSha256 = actorBoundRegistrationSha256;
    return result;
  }

  LfpActorBoundRegistrationAck._();

  factory LfpActorBoundRegistrationAck.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LfpActorBoundRegistrationAck.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LfpActorBoundRegistrationAck',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$1.LfpConsentRegistration>(2, _omitFieldNames ? '' : 'registration',
        subBuilder: $1.LfpConsentRegistration.create)
    ..aOM<LfpConsentOrigin>(3, _omitFieldNames ? '' : 'origin',
        subBuilder: LfpConsentOrigin.create)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'qualifiedConsentSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        5,
        _omitFieldNames ? '' : 'actorBoundRegistrationSha256',
        $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpActorBoundRegistrationAck clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpActorBoundRegistrationAck copyWith(
          void Function(LfpActorBoundRegistrationAck) updates) =>
      super.copyWith(
              (message) => updates(message as LfpActorBoundRegistrationAck))
          as LfpActorBoundRegistrationAck;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LfpActorBoundRegistrationAck create() =>
      LfpActorBoundRegistrationAck._();
  @$core.override
  LfpActorBoundRegistrationAck createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LfpActorBoundRegistrationAck getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LfpActorBoundRegistrationAck>(create);
  static LfpActorBoundRegistrationAck? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.LfpConsentRegistration get registration => $_getN(1);
  @$pb.TagNumber(2)
  set registration($1.LfpConsentRegistration value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasRegistration() => $_has(1);
  @$pb.TagNumber(2)
  void clearRegistration() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.LfpConsentRegistration ensureRegistration() => $_ensure(1);

  @$pb.TagNumber(3)
  LfpConsentOrigin get origin => $_getN(2);
  @$pb.TagNumber(3)
  set origin(LfpConsentOrigin value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasOrigin() => $_has(2);
  @$pb.TagNumber(3)
  void clearOrigin() => $_clearField(3);
  @$pb.TagNumber(3)
  LfpConsentOrigin ensureOrigin() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.List<$core.int> get qualifiedConsentSha256 => $_getN(3);
  @$pb.TagNumber(4)
  set qualifiedConsentSha256($core.List<$core.int> value) =>
      $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasQualifiedConsentSha256() => $_has(3);
  @$pb.TagNumber(4)
  void clearQualifiedConsentSha256() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get actorBoundRegistrationSha256 => $_getN(4);
  @$pb.TagNumber(5)
  set actorBoundRegistrationSha256($core.List<$core.int> value) =>
      $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasActorBoundRegistrationSha256() => $_has(4);
  @$pb.TagNumber(5)
  void clearActorBoundRegistrationSha256() => $_clearField(5);
}

class RegisterActorBoundLfpConsentRequest extends $pb.GeneratedMessage {
  factory RegisterActorBoundLfpConsentRequest({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? registrationOperationId,
    LfpActorBoundConsentEnvelope? envelope,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (registrationOperationId != null)
      result.registrationOperationId = registrationOperationId;
    if (envelope != null) result.envelope = envelope;
    return result;
  }

  RegisterActorBoundLfpConsentRequest._();

  factory RegisterActorBoundLfpConsentRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterActorBoundLfpConsentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterActorBoundLfpConsentRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'registrationOperationId', $pb.PbFieldType.OY)
    ..aOM<LfpActorBoundConsentEnvelope>(3, _omitFieldNames ? '' : 'envelope',
        subBuilder: LfpActorBoundConsentEnvelope.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterActorBoundLfpConsentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterActorBoundLfpConsentRequest copyWith(
          void Function(RegisterActorBoundLfpConsentRequest) updates) =>
      super.copyWith((message) =>
              updates(message as RegisterActorBoundLfpConsentRequest))
          as RegisterActorBoundLfpConsentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterActorBoundLfpConsentRequest create() =>
      RegisterActorBoundLfpConsentRequest._();
  @$core.override
  RegisterActorBoundLfpConsentRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterActorBoundLfpConsentRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          RegisterActorBoundLfpConsentRequest>(create);
  static RegisterActorBoundLfpConsentRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get registrationOperationId => $_getN(1);
  @$pb.TagNumber(2)
  set registrationOperationId($core.List<$core.int> value) =>
      $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRegistrationOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRegistrationOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  LfpActorBoundConsentEnvelope get envelope => $_getN(2);
  @$pb.TagNumber(3)
  set envelope(LfpActorBoundConsentEnvelope value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasEnvelope() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnvelope() => $_clearField(3);
  @$pb.TagNumber(3)
  LfpActorBoundConsentEnvelope ensureEnvelope() => $_ensure(2);
}

class RegisterActorBoundLfpConsentResponse extends $pb.GeneratedMessage {
  factory RegisterActorBoundLfpConsentResponse({
    LfpActorBoundRegistrationAck? acknowledgement,
  }) {
    final result = create();
    if (acknowledgement != null) result.acknowledgement = acknowledgement;
    return result;
  }

  RegisterActorBoundLfpConsentResponse._();

  factory RegisterActorBoundLfpConsentResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterActorBoundLfpConsentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterActorBoundLfpConsentResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<LfpActorBoundRegistrationAck>(
        1, _omitFieldNames ? '' : 'acknowledgement',
        subBuilder: LfpActorBoundRegistrationAck.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterActorBoundLfpConsentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterActorBoundLfpConsentResponse copyWith(
          void Function(RegisterActorBoundLfpConsentResponse) updates) =>
      super.copyWith((message) =>
              updates(message as RegisterActorBoundLfpConsentResponse))
          as RegisterActorBoundLfpConsentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterActorBoundLfpConsentResponse create() =>
      RegisterActorBoundLfpConsentResponse._();
  @$core.override
  RegisterActorBoundLfpConsentResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterActorBoundLfpConsentResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          RegisterActorBoundLfpConsentResponse>(create);
  static RegisterActorBoundLfpConsentResponse? _defaultInstance;

  @$pb.TagNumber(1)
  LfpActorBoundRegistrationAck get acknowledgement => $_getN(0);
  @$pb.TagNumber(1)
  set acknowledgement(LfpActorBoundRegistrationAck value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAcknowledgement() => $_has(0);
  @$pb.TagNumber(1)
  void clearAcknowledgement() => $_clearField(1);
  @$pb.TagNumber(1)
  LfpActorBoundRegistrationAck ensureAcknowledgement() => $_ensure(0);
}

class ReadActorBoundLfpConsentRegistrationRequest extends $pb.GeneratedMessage {
  factory ReadActorBoundLfpConsentRegistrationRequest({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? registrationOperationId,
    $core.List<$core.int>? consentId,
    $core.List<$core.int>? consentSha256,
    $core.List<$core.int>? actorSha256,
    $core.List<$core.int>? qualifiedConsentSha256,
    LfpConsentOrigin? origin,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (registrationOperationId != null)
      result.registrationOperationId = registrationOperationId;
    if (consentId != null) result.consentId = consentId;
    if (consentSha256 != null) result.consentSha256 = consentSha256;
    if (actorSha256 != null) result.actorSha256 = actorSha256;
    if (qualifiedConsentSha256 != null)
      result.qualifiedConsentSha256 = qualifiedConsentSha256;
    if (origin != null) result.origin = origin;
    return result;
  }

  ReadActorBoundLfpConsentRegistrationRequest._();

  factory ReadActorBoundLfpConsentRegistrationRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadActorBoundLfpConsentRegistrationRequest.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadActorBoundLfpConsentRegistrationRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'registrationOperationId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'consentId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'consentSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'actorSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'qualifiedConsentSha256', $pb.PbFieldType.OY)
    ..aOM<LfpConsentOrigin>(7, _omitFieldNames ? '' : 'origin',
        subBuilder: LfpConsentOrigin.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadActorBoundLfpConsentRegistrationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadActorBoundLfpConsentRegistrationRequest copyWith(
          void Function(ReadActorBoundLfpConsentRegistrationRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ReadActorBoundLfpConsentRegistrationRequest))
          as ReadActorBoundLfpConsentRegistrationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadActorBoundLfpConsentRegistrationRequest create() =>
      ReadActorBoundLfpConsentRegistrationRequest._();
  @$core.override
  ReadActorBoundLfpConsentRegistrationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadActorBoundLfpConsentRegistrationRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReadActorBoundLfpConsentRegistrationRequest>(create);
  static ReadActorBoundLfpConsentRegistrationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get registrationOperationId => $_getN(1);
  @$pb.TagNumber(2)
  set registrationOperationId($core.List<$core.int> value) =>
      $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRegistrationOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRegistrationOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get consentId => $_getN(2);
  @$pb.TagNumber(3)
  set consentId($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasConsentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearConsentId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get consentSha256 => $_getN(3);
  @$pb.TagNumber(4)
  set consentSha256($core.List<$core.int> value) => $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasConsentSha256() => $_has(3);
  @$pb.TagNumber(4)
  void clearConsentSha256() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get actorSha256 => $_getN(4);
  @$pb.TagNumber(5)
  set actorSha256($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasActorSha256() => $_has(4);
  @$pb.TagNumber(5)
  void clearActorSha256() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get qualifiedConsentSha256 => $_getN(5);
  @$pb.TagNumber(6)
  set qualifiedConsentSha256($core.List<$core.int> value) =>
      $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasQualifiedConsentSha256() => $_has(5);
  @$pb.TagNumber(6)
  void clearQualifiedConsentSha256() => $_clearField(6);

  @$pb.TagNumber(7)
  LfpConsentOrigin get origin => $_getN(6);
  @$pb.TagNumber(7)
  set origin(LfpConsentOrigin value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasOrigin() => $_has(6);
  @$pb.TagNumber(7)
  void clearOrigin() => $_clearField(7);
  @$pb.TagNumber(7)
  LfpConsentOrigin ensureOrigin() => $_ensure(6);
}

class ReadActorBoundLfpConsentRegistrationResponse
    extends $pb.GeneratedMessage {
  factory ReadActorBoundLfpConsentRegistrationResponse({
    LfpActorBoundRegistrationAck? acknowledgement,
  }) {
    final result = create();
    if (acknowledgement != null) result.acknowledgement = acknowledgement;
    return result;
  }

  ReadActorBoundLfpConsentRegistrationResponse._();

  factory ReadActorBoundLfpConsentRegistrationResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadActorBoundLfpConsentRegistrationResponse.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadActorBoundLfpConsentRegistrationResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<LfpActorBoundRegistrationAck>(
        1, _omitFieldNames ? '' : 'acknowledgement',
        subBuilder: LfpActorBoundRegistrationAck.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadActorBoundLfpConsentRegistrationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadActorBoundLfpConsentRegistrationResponse copyWith(
          void Function(ReadActorBoundLfpConsentRegistrationResponse)
              updates) =>
      super.copyWith((message) =>
              updates(message as ReadActorBoundLfpConsentRegistrationResponse))
          as ReadActorBoundLfpConsentRegistrationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadActorBoundLfpConsentRegistrationResponse create() =>
      ReadActorBoundLfpConsentRegistrationResponse._();
  @$core.override
  ReadActorBoundLfpConsentRegistrationResponse createEmptyInstance() =>
      create();
  @$core.pragma('dart2js:noInline')
  static ReadActorBoundLfpConsentRegistrationResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReadActorBoundLfpConsentRegistrationResponse>(create);
  static ReadActorBoundLfpConsentRegistrationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  LfpActorBoundRegistrationAck get acknowledgement => $_getN(0);
  @$pb.TagNumber(1)
  set acknowledgement(LfpActorBoundRegistrationAck value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAcknowledgement() => $_has(0);
  @$pb.TagNumber(1)
  void clearAcknowledgement() => $_clearField(1);
  @$pb.TagNumber(1)
  LfpActorBoundRegistrationAck ensureAcknowledgement() => $_ensure(0);
}

class ValidateActorBoundLfpConsentOfferRequest extends $pb.GeneratedMessage {
  factory ValidateActorBoundLfpConsentOfferRequest({
    $fixnum.Int64? protocolVersion,
    $1.ValidateLfpConsentOfferRequest? offer,
    $core.List<$core.int>? qualifiedConsentSha256,
    $core.List<$core.int>? actorSha256,
    LfpConsentOrigin? origin,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (offer != null) result.offer = offer;
    if (qualifiedConsentSha256 != null)
      result.qualifiedConsentSha256 = qualifiedConsentSha256;
    if (actorSha256 != null) result.actorSha256 = actorSha256;
    if (origin != null) result.origin = origin;
    return result;
  }

  ValidateActorBoundLfpConsentOfferRequest._();

  factory ValidateActorBoundLfpConsentOfferRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateActorBoundLfpConsentOfferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateActorBoundLfpConsentOfferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$1.ValidateLfpConsentOfferRequest>(2, _omitFieldNames ? '' : 'offer',
        subBuilder: $1.ValidateLfpConsentOfferRequest.create)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'qualifiedConsentSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'actorSha256', $pb.PbFieldType.OY)
    ..aOM<LfpConsentOrigin>(5, _omitFieldNames ? '' : 'origin',
        subBuilder: LfpConsentOrigin.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateActorBoundLfpConsentOfferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateActorBoundLfpConsentOfferRequest copyWith(
          void Function(ValidateActorBoundLfpConsentOfferRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ValidateActorBoundLfpConsentOfferRequest))
          as ValidateActorBoundLfpConsentOfferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateActorBoundLfpConsentOfferRequest create() =>
      ValidateActorBoundLfpConsentOfferRequest._();
  @$core.override
  ValidateActorBoundLfpConsentOfferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateActorBoundLfpConsentOfferRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ValidateActorBoundLfpConsentOfferRequest>(create);
  static ValidateActorBoundLfpConsentOfferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.ValidateLfpConsentOfferRequest get offer => $_getN(1);
  @$pb.TagNumber(2)
  set offer($1.ValidateLfpConsentOfferRequest value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOffer() => $_has(1);
  @$pb.TagNumber(2)
  void clearOffer() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.ValidateLfpConsentOfferRequest ensureOffer() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.List<$core.int> get qualifiedConsentSha256 => $_getN(2);
  @$pb.TagNumber(3)
  set qualifiedConsentSha256($core.List<$core.int> value) =>
      $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasQualifiedConsentSha256() => $_has(2);
  @$pb.TagNumber(3)
  void clearQualifiedConsentSha256() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get actorSha256 => $_getN(3);
  @$pb.TagNumber(4)
  set actorSha256($core.List<$core.int> value) => $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasActorSha256() => $_has(3);
  @$pb.TagNumber(4)
  void clearActorSha256() => $_clearField(4);

  @$pb.TagNumber(5)
  LfpConsentOrigin get origin => $_getN(4);
  @$pb.TagNumber(5)
  set origin(LfpConsentOrigin value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOrigin() => $_has(4);
  @$pb.TagNumber(5)
  void clearOrigin() => $_clearField(5);
  @$pb.TagNumber(5)
  LfpConsentOrigin ensureOrigin() => $_ensure(4);
}

class ValidateActorBoundLfpConsentOfferResponse extends $pb.GeneratedMessage {
  factory ValidateActorBoundLfpConsentOfferResponse({
    $fixnum.Int64? protocolVersion,
    $1.ValidateLfpConsentOfferResponse? offer,
    $core.List<$core.int>? qualifiedConsentSha256,
    $core.List<$core.int>? actorSha256,
    LfpConsentOrigin? origin,
    $core.List<$core.int>? canonicalCriteriaJson,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (offer != null) result.offer = offer;
    if (qualifiedConsentSha256 != null)
      result.qualifiedConsentSha256 = qualifiedConsentSha256;
    if (actorSha256 != null) result.actorSha256 = actorSha256;
    if (origin != null) result.origin = origin;
    if (canonicalCriteriaJson != null)
      result.canonicalCriteriaJson = canonicalCriteriaJson;
    return result;
  }

  ValidateActorBoundLfpConsentOfferResponse._();

  factory ValidateActorBoundLfpConsentOfferResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateActorBoundLfpConsentOfferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateActorBoundLfpConsentOfferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$1.ValidateLfpConsentOfferResponse>(2, _omitFieldNames ? '' : 'offer',
        subBuilder: $1.ValidateLfpConsentOfferResponse.create)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'qualifiedConsentSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'actorSha256', $pb.PbFieldType.OY)
    ..aOM<LfpConsentOrigin>(5, _omitFieldNames ? '' : 'origin',
        subBuilder: LfpConsentOrigin.create)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'canonicalCriteriaJson', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateActorBoundLfpConsentOfferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateActorBoundLfpConsentOfferResponse copyWith(
          void Function(ValidateActorBoundLfpConsentOfferResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ValidateActorBoundLfpConsentOfferResponse))
          as ValidateActorBoundLfpConsentOfferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateActorBoundLfpConsentOfferResponse create() =>
      ValidateActorBoundLfpConsentOfferResponse._();
  @$core.override
  ValidateActorBoundLfpConsentOfferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateActorBoundLfpConsentOfferResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ValidateActorBoundLfpConsentOfferResponse>(create);
  static ValidateActorBoundLfpConsentOfferResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.ValidateLfpConsentOfferResponse get offer => $_getN(1);
  @$pb.TagNumber(2)
  set offer($1.ValidateLfpConsentOfferResponse value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOffer() => $_has(1);
  @$pb.TagNumber(2)
  void clearOffer() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.ValidateLfpConsentOfferResponse ensureOffer() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.List<$core.int> get qualifiedConsentSha256 => $_getN(2);
  @$pb.TagNumber(3)
  set qualifiedConsentSha256($core.List<$core.int> value) =>
      $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasQualifiedConsentSha256() => $_has(2);
  @$pb.TagNumber(3)
  void clearQualifiedConsentSha256() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get actorSha256 => $_getN(3);
  @$pb.TagNumber(4)
  set actorSha256($core.List<$core.int> value) => $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasActorSha256() => $_has(3);
  @$pb.TagNumber(4)
  void clearActorSha256() => $_clearField(4);

  @$pb.TagNumber(5)
  LfpConsentOrigin get origin => $_getN(4);
  @$pb.TagNumber(5)
  set origin(LfpConsentOrigin value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOrigin() => $_has(4);
  @$pb.TagNumber(5)
  void clearOrigin() => $_clearField(5);
  @$pb.TagNumber(5)
  LfpConsentOrigin ensureOrigin() => $_ensure(4);

  /// REQUIRED exact immutable PostgreSQL lfp_criteria::text bytes. SHA256 must
  /// equal the v1 receipt criteria_sha256; parse/validate its shape before MM
  /// capture. A public projection or generic JSON re-encoding is not equivalent.
  @$pb.TagNumber(6)
  $core.List<$core.int> get canonicalCriteriaJson => $_getN(5);
  @$pb.TagNumber(6)
  set canonicalCriteriaJson($core.List<$core.int> value) =>
      $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCanonicalCriteriaJson() => $_has(5);
  @$pb.TagNumber(6)
  void clearCanonicalCriteriaJson() => $_clearField(6);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
