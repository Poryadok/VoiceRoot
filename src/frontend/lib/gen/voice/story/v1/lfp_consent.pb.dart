// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_consent.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'lfp_consent.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'lfp_consent.pbenum.dart';

/// PRIVATE authority material. UUIDs are nonzero 16 raw bytes, hashes 32 raw
/// bytes. No account/epoch/private bytes may enter public events or responses.
/// All fields are required by protocol 1. Deterministic protobuf bytes only;
/// unknown/duplicate/noncanonical fields are rejected before owner mutation.
/// Receipt SHA256 domain: "voice.story.lfp-consent-receipt.v1\x00".
class LfpConsentMaterial extends $pb.GeneratedMessage {
  factory LfpConsentMaterial({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? consentId,
    $core.List<$core.int>? responseOperationId,
    $core.List<$core.int>? storyId,
    $core.List<$core.int>? storyIncarnation,
    $fixnum.Int64? offerRevision,
    $core.List<$core.int>? authorProfileId,
    $core.List<$core.int>? responderAccountId,
    $core.List<$core.int>? responderProfileId,
    $fixnum.Int64? responderSessionEpoch,
    LfpConsentResponseType? responseType,
    $core.List<$core.int>? criteriaSha256,
    $core.List<$core.int>? offerPayloadSha256,
    $fixnum.Int64? createdAtUnixMillis,
    $fixnum.Int64? offerExpiresAtUnixMillis,
    $core.List<$core.int>? principalRequestSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (consentId != null) result.consentId = consentId;
    if (responseOperationId != null)
      result.responseOperationId = responseOperationId;
    if (storyId != null) result.storyId = storyId;
    if (storyIncarnation != null) result.storyIncarnation = storyIncarnation;
    if (offerRevision != null) result.offerRevision = offerRevision;
    if (authorProfileId != null) result.authorProfileId = authorProfileId;
    if (responderAccountId != null)
      result.responderAccountId = responderAccountId;
    if (responderProfileId != null)
      result.responderProfileId = responderProfileId;
    if (responderSessionEpoch != null)
      result.responderSessionEpoch = responderSessionEpoch;
    if (responseType != null) result.responseType = responseType;
    if (criteriaSha256 != null) result.criteriaSha256 = criteriaSha256;
    if (offerPayloadSha256 != null)
      result.offerPayloadSha256 = offerPayloadSha256;
    if (createdAtUnixMillis != null)
      result.createdAtUnixMillis = createdAtUnixMillis;
    if (offerExpiresAtUnixMillis != null)
      result.offerExpiresAtUnixMillis = offerExpiresAtUnixMillis;
    if (principalRequestSha256 != null)
      result.principalRequestSha256 = principalRequestSha256;
    return result;
  }

  LfpConsentMaterial._();

  factory LfpConsentMaterial.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LfpConsentMaterial.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LfpConsentMaterial',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'consentId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'responseOperationId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'storyId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'storyIncarnation', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'offerRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'authorProfileId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'responderAccountId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        9, _omitFieldNames ? '' : 'responderProfileId', $pb.PbFieldType.OY)
    ..aInt64(10, _omitFieldNames ? '' : 'responderSessionEpoch')
    ..aE<LfpConsentResponseType>(11, _omitFieldNames ? '' : 'responseType',
        enumValues: LfpConsentResponseType.values)
    ..a<$core.List<$core.int>>(
        12, _omitFieldNames ? '' : 'criteriaSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        13, _omitFieldNames ? '' : 'offerPayloadSha256', $pb.PbFieldType.OY)
    ..aInt64(14, _omitFieldNames ? '' : 'createdAtUnixMillis')
    ..aInt64(15, _omitFieldNames ? '' : 'offerExpiresAtUnixMillis')
    ..a<$core.List<$core.int>>(
        16, _omitFieldNames ? '' : 'principalRequestSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpConsentMaterial clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpConsentMaterial copyWith(void Function(LfpConsentMaterial) updates) =>
      super.copyWith((message) => updates(message as LfpConsentMaterial))
          as LfpConsentMaterial;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LfpConsentMaterial create() => LfpConsentMaterial._();
  @$core.override
  LfpConsentMaterial createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LfpConsentMaterial getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LfpConsentMaterial>(create);
  static LfpConsentMaterial? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get consentId => $_getN(1);
  @$pb.TagNumber(2)
  set consentId($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConsentId() => $_has(1);
  @$pb.TagNumber(2)
  void clearConsentId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get responseOperationId => $_getN(2);
  @$pb.TagNumber(3)
  set responseOperationId($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasResponseOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearResponseOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get storyId => $_getN(3);
  @$pb.TagNumber(4)
  set storyId($core.List<$core.int> value) => $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasStoryId() => $_has(3);
  @$pb.TagNumber(4)
  void clearStoryId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get storyIncarnation => $_getN(4);
  @$pb.TagNumber(5)
  set storyIncarnation($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStoryIncarnation() => $_has(4);
  @$pb.TagNumber(5)
  void clearStoryIncarnation() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get offerRevision => $_getI64(5);
  @$pb.TagNumber(6)
  set offerRevision($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasOfferRevision() => $_has(5);
  @$pb.TagNumber(6)
  void clearOfferRevision() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get authorProfileId => $_getN(6);
  @$pb.TagNumber(7)
  set authorProfileId($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasAuthorProfileId() => $_has(6);
  @$pb.TagNumber(7)
  void clearAuthorProfileId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get responderAccountId => $_getN(7);
  @$pb.TagNumber(8)
  set responderAccountId($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasResponderAccountId() => $_has(7);
  @$pb.TagNumber(8)
  void clearResponderAccountId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.List<$core.int> get responderProfileId => $_getN(8);
  @$pb.TagNumber(9)
  set responderProfileId($core.List<$core.int> value) => $_setBytes(8, value);
  @$pb.TagNumber(9)
  $core.bool hasResponderProfileId() => $_has(8);
  @$pb.TagNumber(9)
  void clearResponderProfileId() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get responderSessionEpoch => $_getI64(9);
  @$pb.TagNumber(10)
  set responderSessionEpoch($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasResponderSessionEpoch() => $_has(9);
  @$pb.TagNumber(10)
  void clearResponderSessionEpoch() => $_clearField(10);

  @$pb.TagNumber(11)
  LfpConsentResponseType get responseType => $_getN(10);
  @$pb.TagNumber(11)
  set responseType(LfpConsentResponseType value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasResponseType() => $_has(10);
  @$pb.TagNumber(11)
  void clearResponseType() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.List<$core.int> get criteriaSha256 => $_getN(11);
  @$pb.TagNumber(12)
  set criteriaSha256($core.List<$core.int> value) => $_setBytes(11, value);
  @$pb.TagNumber(12)
  $core.bool hasCriteriaSha256() => $_has(11);
  @$pb.TagNumber(12)
  void clearCriteriaSha256() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.List<$core.int> get offerPayloadSha256 => $_getN(12);
  @$pb.TagNumber(13)
  set offerPayloadSha256($core.List<$core.int> value) => $_setBytes(12, value);
  @$pb.TagNumber(13)
  $core.bool hasOfferPayloadSha256() => $_has(12);
  @$pb.TagNumber(13)
  void clearOfferPayloadSha256() => $_clearField(13);

  @$pb.TagNumber(14)
  $fixnum.Int64 get createdAtUnixMillis => $_getI64(13);
  @$pb.TagNumber(14)
  set createdAtUnixMillis($fixnum.Int64 value) => $_setInt64(13, value);
  @$pb.TagNumber(14)
  $core.bool hasCreatedAtUnixMillis() => $_has(13);
  @$pb.TagNumber(14)
  void clearCreatedAtUnixMillis() => $_clearField(14);

  @$pb.TagNumber(15)
  $fixnum.Int64 get offerExpiresAtUnixMillis => $_getI64(14);
  @$pb.TagNumber(15)
  set offerExpiresAtUnixMillis($fixnum.Int64 value) => $_setInt64(14, value);
  @$pb.TagNumber(15)
  $core.bool hasOfferExpiresAtUnixMillis() => $_has(14);
  @$pb.TagNumber(15)
  void clearOfferExpiresAtUnixMillis() => $_clearField(15);

  @$pb.TagNumber(16)
  $core.List<$core.int> get principalRequestSha256 => $_getN(15);
  @$pb.TagNumber(16)
  set principalRequestSha256($core.List<$core.int> value) =>
      $_setBytes(15, value);
  @$pb.TagNumber(16)
  $core.bool hasPrincipalRequestSha256() => $_has(15);
  @$pb.TagNumber(16)
  void clearPrincipalRequestSha256() => $_clearField(16);
}

class RegisterLfpConsentResponse extends $pb.GeneratedMessage {
  factory RegisterLfpConsentResponse({
    LfpConsentRegistration? registration,
  }) {
    final result = create();
    if (registration != null) result.registration = registration;
    return result;
  }

  RegisterLfpConsentResponse._();

  factory RegisterLfpConsentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterLfpConsentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterLfpConsentResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<LfpConsentRegistration>(1, _omitFieldNames ? '' : 'registration',
        subBuilder: LfpConsentRegistration.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterLfpConsentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterLfpConsentResponse copyWith(
          void Function(RegisterLfpConsentResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RegisterLfpConsentResponse))
          as RegisterLfpConsentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterLfpConsentResponse create() => RegisterLfpConsentResponse._();
  @$core.override
  RegisterLfpConsentResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterLfpConsentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterLfpConsentResponse>(create);
  static RegisterLfpConsentResponse? _defaultInstance;

  @$pb.TagNumber(1)
  LfpConsentRegistration get registration => $_getN(0);
  @$pb.TagNumber(1)
  set registration(LfpConsentRegistration value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRegistration() => $_has(0);
  @$pb.TagNumber(1)
  void clearRegistration() => $_clearField(1);
  @$pb.TagNumber(1)
  LfpConsentRegistration ensureRegistration() => $_ensure(0);
}

class ReadLfpConsentRegistrationResponse extends $pb.GeneratedMessage {
  factory ReadLfpConsentRegistrationResponse({
    LfpConsentRegistration? registration,
  }) {
    final result = create();
    if (registration != null) result.registration = registration;
    return result;
  }

  ReadLfpConsentRegistrationResponse._();

  factory ReadLfpConsentRegistrationResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadLfpConsentRegistrationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadLfpConsentRegistrationResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<LfpConsentRegistration>(1, _omitFieldNames ? '' : 'registration',
        subBuilder: LfpConsentRegistration.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadLfpConsentRegistrationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadLfpConsentRegistrationResponse copyWith(
          void Function(ReadLfpConsentRegistrationResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ReadLfpConsentRegistrationResponse))
          as ReadLfpConsentRegistrationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadLfpConsentRegistrationResponse create() =>
      ReadLfpConsentRegistrationResponse._();
  @$core.override
  ReadLfpConsentRegistrationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadLfpConsentRegistrationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadLfpConsentRegistrationResponse>(
          create);
  static ReadLfpConsentRegistrationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  LfpConsentRegistration get registration => $_getN(0);
  @$pb.TagNumber(1)
  set registration(LfpConsentRegistration value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRegistration() => $_has(0);
  @$pb.TagNumber(1)
  void clearRegistration() => $_clearField(1);
  @$pb.TagNumber(1)
  LfpConsentRegistration ensureRegistration() => $_ensure(0);
}

class RegisterLfpConsentRequest extends $pb.GeneratedMessage {
  factory RegisterLfpConsentRequest({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? registrationOperationId,
    $core.List<$core.int>? canonicalConsent,
    $core.List<$core.int>? consentSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (registrationOperationId != null)
      result.registrationOperationId = registrationOperationId;
    if (canonicalConsent != null) result.canonicalConsent = canonicalConsent;
    if (consentSha256 != null) result.consentSha256 = consentSha256;
    return result;
  }

  RegisterLfpConsentRequest._();

  factory RegisterLfpConsentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterLfpConsentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterLfpConsentRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'registrationOperationId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'canonicalConsent', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'consentSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterLfpConsentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterLfpConsentRequest copyWith(
          void Function(RegisterLfpConsentRequest) updates) =>
      super.copyWith((message) => updates(message as RegisterLfpConsentRequest))
          as RegisterLfpConsentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterLfpConsentRequest create() => RegisterLfpConsentRequest._();
  @$core.override
  RegisterLfpConsentRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterLfpConsentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterLfpConsentRequest>(create);
  static RegisterLfpConsentRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  /// Same immutable operation on every attempt, including recovery/lost ACK.
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
  $core.List<$core.int> get canonicalConsent => $_getN(2);
  @$pb.TagNumber(3)
  set canonicalConsent($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCanonicalConsent() => $_has(2);
  @$pb.TagNumber(3)
  void clearCanonicalConsent() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get consentSha256 => $_getN(3);
  @$pb.TagNumber(4)
  set consentSha256($core.List<$core.int> value) => $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasConsentSha256() => $_has(3);
  @$pb.TagNumber(4)
  void clearConsentSha256() => $_clearField(4);
}

class ReadLfpConsentRegistrationRequest extends $pb.GeneratedMessage {
  factory ReadLfpConsentRegistrationRequest({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? registrationOperationId,
    $core.List<$core.int>? consentId,
    $core.List<$core.int>? consentSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (registrationOperationId != null)
      result.registrationOperationId = registrationOperationId;
    if (consentId != null) result.consentId = consentId;
    if (consentSha256 != null) result.consentSha256 = consentSha256;
    return result;
  }

  ReadLfpConsentRegistrationRequest._();

  factory ReadLfpConsentRegistrationRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadLfpConsentRegistrationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadLfpConsentRegistrationRequest',
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
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadLfpConsentRegistrationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadLfpConsentRegistrationRequest copyWith(
          void Function(ReadLfpConsentRegistrationRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ReadLfpConsentRegistrationRequest))
          as ReadLfpConsentRegistrationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadLfpConsentRegistrationRequest create() =>
      ReadLfpConsentRegistrationRequest._();
  @$core.override
  ReadLfpConsentRegistrationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadLfpConsentRegistrationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadLfpConsentRegistrationRequest>(
          create);
  static ReadLfpConsentRegistrationRequest? _defaultInstance;

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
}

/// Immutable original registration; consumed/declined status is read separately
/// by MM and cannot rewrite this receipt or recapture a newer search intent.
class LfpConsentRegistration extends $pb.GeneratedMessage {
  factory LfpConsentRegistration({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? registrationOperationId,
    $core.List<$core.int>? consentId,
    $core.List<$core.int>? consentSha256,
    $core.List<$core.int>? registrationId,
    $fixnum.Int64? registrationGeneration,
    $core.List<$core.int>? capturedSearchIntentSha256,
    $core.List<$core.int>? registrationSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (registrationOperationId != null)
      result.registrationOperationId = registrationOperationId;
    if (consentId != null) result.consentId = consentId;
    if (consentSha256 != null) result.consentSha256 = consentSha256;
    if (registrationId != null) result.registrationId = registrationId;
    if (registrationGeneration != null)
      result.registrationGeneration = registrationGeneration;
    if (capturedSearchIntentSha256 != null)
      result.capturedSearchIntentSha256 = capturedSearchIntentSha256;
    if (registrationSha256 != null)
      result.registrationSha256 = registrationSha256;
    return result;
  }

  LfpConsentRegistration._();

  factory LfpConsentRegistration.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LfpConsentRegistration.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LfpConsentRegistration',
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
        5, _omitFieldNames ? '' : 'registrationId', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'registrationGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(7,
        _omitFieldNames ? '' : 'capturedSearchIntentSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'registrationSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpConsentRegistration clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpConsentRegistration copyWith(
          void Function(LfpConsentRegistration) updates) =>
      super.copyWith((message) => updates(message as LfpConsentRegistration))
          as LfpConsentRegistration;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LfpConsentRegistration create() => LfpConsentRegistration._();
  @$core.override
  LfpConsentRegistration createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LfpConsentRegistration getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LfpConsentRegistration>(create);
  static LfpConsentRegistration? _defaultInstance;

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
  $core.List<$core.int> get registrationId => $_getN(4);
  @$pb.TagNumber(5)
  set registrationId($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRegistrationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRegistrationId() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get registrationGeneration => $_getI64(5);
  @$pb.TagNumber(6)
  set registrationGeneration($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRegistrationGeneration() => $_has(5);
  @$pb.TagNumber(6)
  void clearRegistrationGeneration() => $_clearField(6);

  /// Hash of MM's original exact replacement/search-intent witness including
  /// explicit NONE; its private bytes remain owned by MM, not sent to Story.
  @$pb.TagNumber(7)
  $core.List<$core.int> get capturedSearchIntentSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set capturedSearchIntentSha256($core.List<$core.int> value) =>
      $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasCapturedSearchIntentSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearCapturedSearchIntentSha256() => $_clearField(7);

  /// SHA256("voice.mm.lfp-consent-registration.v1\x00" || deterministic fields1-7).
  /// Field8 is omitted from the hash input. Unknown fields deny.
  @$pb.TagNumber(8)
  $core.List<$core.int> get registrationSha256 => $_getN(7);
  @$pb.TagNumber(8)
  set registrationSha256($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasRegistrationSha256() => $_has(7);
  @$pb.TagNumber(8)
  void clearRegistrationSha256() => $_clearField(8);
}

class ValidateLfpConsentOfferRequest extends $pb.GeneratedMessage {
  factory ValidateLfpConsentOfferRequest({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? consentId,
    $core.List<$core.int>? consentSha256,
    $core.List<$core.int>? storyId,
    $core.List<$core.int>? storyIncarnation,
    $fixnum.Int64? offerRevision,
    $core.List<$core.int>? offerPayloadSha256,
    $core.List<$core.int>? criteriaSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (consentId != null) result.consentId = consentId;
    if (consentSha256 != null) result.consentSha256 = consentSha256;
    if (storyId != null) result.storyId = storyId;
    if (storyIncarnation != null) result.storyIncarnation = storyIncarnation;
    if (offerRevision != null) result.offerRevision = offerRevision;
    if (offerPayloadSha256 != null)
      result.offerPayloadSha256 = offerPayloadSha256;
    if (criteriaSha256 != null) result.criteriaSha256 = criteriaSha256;
    return result;
  }

  ValidateLfpConsentOfferRequest._();

  factory ValidateLfpConsentOfferRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateLfpConsentOfferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateLfpConsentOfferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'consentId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'consentSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'storyId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'storyIncarnation', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'offerRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'offerPayloadSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'criteriaSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateLfpConsentOfferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateLfpConsentOfferRequest copyWith(
          void Function(ValidateLfpConsentOfferRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ValidateLfpConsentOfferRequest))
          as ValidateLfpConsentOfferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateLfpConsentOfferRequest create() =>
      ValidateLfpConsentOfferRequest._();
  @$core.override
  ValidateLfpConsentOfferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateLfpConsentOfferRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateLfpConsentOfferRequest>(create);
  static ValidateLfpConsentOfferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get consentId => $_getN(1);
  @$pb.TagNumber(2)
  set consentId($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConsentId() => $_has(1);
  @$pb.TagNumber(2)
  void clearConsentId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get consentSha256 => $_getN(2);
  @$pb.TagNumber(3)
  set consentSha256($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasConsentSha256() => $_has(2);
  @$pb.TagNumber(3)
  void clearConsentSha256() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get storyId => $_getN(3);
  @$pb.TagNumber(4)
  set storyId($core.List<$core.int> value) => $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasStoryId() => $_has(3);
  @$pb.TagNumber(4)
  void clearStoryId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get storyIncarnation => $_getN(4);
  @$pb.TagNumber(5)
  set storyIncarnation($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStoryIncarnation() => $_has(4);
  @$pb.TagNumber(5)
  void clearStoryIncarnation() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get offerRevision => $_getI64(5);
  @$pb.TagNumber(6)
  set offerRevision($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasOfferRevision() => $_has(5);
  @$pb.TagNumber(6)
  void clearOfferRevision() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get offerPayloadSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set offerPayloadSha256($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasOfferPayloadSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearOfferPayloadSha256() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get criteriaSha256 => $_getN(7);
  @$pb.TagNumber(8)
  set criteriaSha256($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasCriteriaSha256() => $_has(7);
  @$pb.TagNumber(8)
  void clearCriteriaSha256() => $_clearField(8);
}

/// Success is a fresh exact Story-owned observation, not a permanent permit or
/// distributed transaction. Stale/expired/retracted/hidden denies; outage is
/// unavailable. MM must independently apply fresh Auth floors/all-member bans.
class ValidateLfpConsentOfferResponse extends $pb.GeneratedMessage {
  factory ValidateLfpConsentOfferResponse({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? consentId,
    $core.List<$core.int>? consentSha256,
    $core.List<$core.int>? storyId,
    $core.List<$core.int>? storyIncarnation,
    $fixnum.Int64? offerRevision,
    $core.List<$core.int>? offerPayloadSha256,
    $core.List<$core.int>? criteriaSha256,
    $fixnum.Int64? offerExpiresAtUnixMillis,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (consentId != null) result.consentId = consentId;
    if (consentSha256 != null) result.consentSha256 = consentSha256;
    if (storyId != null) result.storyId = storyId;
    if (storyIncarnation != null) result.storyIncarnation = storyIncarnation;
    if (offerRevision != null) result.offerRevision = offerRevision;
    if (offerPayloadSha256 != null)
      result.offerPayloadSha256 = offerPayloadSha256;
    if (criteriaSha256 != null) result.criteriaSha256 = criteriaSha256;
    if (offerExpiresAtUnixMillis != null)
      result.offerExpiresAtUnixMillis = offerExpiresAtUnixMillis;
    return result;
  }

  ValidateLfpConsentOfferResponse._();

  factory ValidateLfpConsentOfferResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateLfpConsentOfferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateLfpConsentOfferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'consentId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'consentSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        4, _omitFieldNames ? '' : 'storyId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'storyIncarnation', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'offerRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'offerPayloadSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        8, _omitFieldNames ? '' : 'criteriaSha256', $pb.PbFieldType.OY)
    ..aInt64(9, _omitFieldNames ? '' : 'offerExpiresAtUnixMillis')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateLfpConsentOfferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateLfpConsentOfferResponse copyWith(
          void Function(ValidateLfpConsentOfferResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ValidateLfpConsentOfferResponse))
          as ValidateLfpConsentOfferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateLfpConsentOfferResponse create() =>
      ValidateLfpConsentOfferResponse._();
  @$core.override
  ValidateLfpConsentOfferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateLfpConsentOfferResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateLfpConsentOfferResponse>(
          create);
  static ValidateLfpConsentOfferResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get consentId => $_getN(1);
  @$pb.TagNumber(2)
  set consentId($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConsentId() => $_has(1);
  @$pb.TagNumber(2)
  void clearConsentId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get consentSha256 => $_getN(2);
  @$pb.TagNumber(3)
  set consentSha256($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasConsentSha256() => $_has(2);
  @$pb.TagNumber(3)
  void clearConsentSha256() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get storyId => $_getN(3);
  @$pb.TagNumber(4)
  set storyId($core.List<$core.int> value) => $_setBytes(3, value);
  @$pb.TagNumber(4)
  $core.bool hasStoryId() => $_has(3);
  @$pb.TagNumber(4)
  void clearStoryId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get storyIncarnation => $_getN(4);
  @$pb.TagNumber(5)
  set storyIncarnation($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStoryIncarnation() => $_has(4);
  @$pb.TagNumber(5)
  void clearStoryIncarnation() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get offerRevision => $_getI64(5);
  @$pb.TagNumber(6)
  set offerRevision($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasOfferRevision() => $_has(5);
  @$pb.TagNumber(6)
  void clearOfferRevision() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get offerPayloadSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set offerPayloadSha256($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasOfferPayloadSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearOfferPayloadSha256() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.List<$core.int> get criteriaSha256 => $_getN(7);
  @$pb.TagNumber(8)
  set criteriaSha256($core.List<$core.int> value) => $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasCriteriaSha256() => $_has(7);
  @$pb.TagNumber(8)
  void clearCriteriaSha256() => $_clearField(8);

  @$pb.TagNumber(9)
  $fixnum.Int64 get offerExpiresAtUnixMillis => $_getI64(8);
  @$pb.TagNumber(9)
  set offerExpiresAtUnixMillis($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasOfferExpiresAtUnixMillis() => $_has(8);
  @$pb.TagNumber(9)
  void clearOfferExpiresAtUnixMillis() => $_clearField(9);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
