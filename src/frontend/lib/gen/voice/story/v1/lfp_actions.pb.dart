// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_actions.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'lfp_consent.pbenum.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Profile-scoped view of the exact immutable offer. IDs/digests are selectors,
/// never identity or restore-cohort authority. No account, epoch or origin here.
class LfpConsentOfferView extends $pb.GeneratedMessage {
  factory LfpConsentOfferView({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? storyId,
    $core.List<$core.int>? storyIncarnation,
    $fixnum.Int64? offerRevision,
    $core.List<$core.int>? authorProfileId,
    $core.List<$core.int>? criteriaSha256,
    $core.List<$core.int>? offerPayloadSha256,
    $fixnum.Int64? offerExpiresAtUnixMillis,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (storyId != null) result.storyId = storyId;
    if (storyIncarnation != null) result.storyIncarnation = storyIncarnation;
    if (offerRevision != null) result.offerRevision = offerRevision;
    if (authorProfileId != null) result.authorProfileId = authorProfileId;
    if (criteriaSha256 != null) result.criteriaSha256 = criteriaSha256;
    if (offerPayloadSha256 != null)
      result.offerPayloadSha256 = offerPayloadSha256;
    if (offerExpiresAtUnixMillis != null)
      result.offerExpiresAtUnixMillis = offerExpiresAtUnixMillis;
    return result;
  }

  LfpConsentOfferView._();

  factory LfpConsentOfferView.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LfpConsentOfferView.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LfpConsentOfferView',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'storyId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'storyIncarnation', $pb.PbFieldType.OY)
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'offerRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'authorProfileId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'criteriaSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'offerPayloadSha256', $pb.PbFieldType.OY)
    ..aInt64(8, _omitFieldNames ? '' : 'offerExpiresAtUnixMillis')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpConsentOfferView clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LfpConsentOfferView copyWith(void Function(LfpConsentOfferView) updates) =>
      super.copyWith((message) => updates(message as LfpConsentOfferView))
          as LfpConsentOfferView;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LfpConsentOfferView create() => LfpConsentOfferView._();
  @$core.override
  LfpConsentOfferView createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LfpConsentOfferView getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LfpConsentOfferView>(create);
  static LfpConsentOfferView? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get storyId => $_getN(1);
  @$pb.TagNumber(2)
  set storyId($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoryId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get storyIncarnation => $_getN(2);
  @$pb.TagNumber(3)
  set storyIncarnation($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoryIncarnation() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoryIncarnation() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get offerRevision => $_getI64(3);
  @$pb.TagNumber(4)
  set offerRevision($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOfferRevision() => $_has(3);
  @$pb.TagNumber(4)
  void clearOfferRevision() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get authorProfileId => $_getN(4);
  @$pb.TagNumber(5)
  set authorProfileId($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAuthorProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearAuthorProfileId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get criteriaSha256 => $_getN(5);
  @$pb.TagNumber(6)
  set criteriaSha256($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCriteriaSha256() => $_has(5);
  @$pb.TagNumber(6)
  void clearCriteriaSha256() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get offerPayloadSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set offerPayloadSha256($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasOfferPayloadSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearOfferPayloadSha256() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get offerExpiresAtUnixMillis => $_getI64(7);
  @$pb.TagNumber(8)
  set offerExpiresAtUnixMillis($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasOfferExpiresAtUnixMillis() => $_has(7);
  @$pb.TagNumber(8)
  void clearOfferExpiresAtUnixMillis() => $_clearField(8);
}

class PublishLfpConsentOfferRequest extends $pb.GeneratedMessage {
  factory PublishLfpConsentOfferRequest({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? publicationOperationId,
    $core.List<$core.int>? storyId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (publicationOperationId != null)
      result.publicationOperationId = publicationOperationId;
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  PublishLfpConsentOfferRequest._();

  factory PublishLfpConsentOfferRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PublishLfpConsentOfferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishLfpConsentOfferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'publicationOperationId', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        3, _omitFieldNames ? '' : 'storyId', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishLfpConsentOfferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishLfpConsentOfferRequest copyWith(
          void Function(PublishLfpConsentOfferRequest) updates) =>
      super.copyWith(
              (message) => updates(message as PublishLfpConsentOfferRequest))
          as PublishLfpConsentOfferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PublishLfpConsentOfferRequest create() =>
      PublishLfpConsentOfferRequest._();
  @$core.override
  PublishLfpConsentOfferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PublishLfpConsentOfferRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishLfpConsentOfferRequest>(create);
  static PublishLfpConsentOfferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get publicationOperationId => $_getN(1);
  @$pb.TagNumber(2)
  set publicationOperationId($core.List<$core.int> value) =>
      $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPublicationOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublicationOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get storyId => $_getN(2);
  @$pb.TagNumber(3)
  set storyId($core.List<$core.int> value) => $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoryId() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoryId() => $_clearField(3);
}

class PublishLfpConsentOfferResponse extends $pb.GeneratedMessage {
  factory PublishLfpConsentOfferResponse({
    LfpConsentOfferView? offer,
  }) {
    final result = create();
    if (offer != null) result.offer = offer;
    return result;
  }

  PublishLfpConsentOfferResponse._();

  factory PublishLfpConsentOfferResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PublishLfpConsentOfferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishLfpConsentOfferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<LfpConsentOfferView>(1, _omitFieldNames ? '' : 'offer',
        subBuilder: LfpConsentOfferView.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishLfpConsentOfferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishLfpConsentOfferResponse copyWith(
          void Function(PublishLfpConsentOfferResponse) updates) =>
      super.copyWith(
              (message) => updates(message as PublishLfpConsentOfferResponse))
          as PublishLfpConsentOfferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PublishLfpConsentOfferResponse create() =>
      PublishLfpConsentOfferResponse._();
  @$core.override
  PublishLfpConsentOfferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PublishLfpConsentOfferResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishLfpConsentOfferResponse>(create);
  static PublishLfpConsentOfferResponse? _defaultInstance;

  @$pb.TagNumber(1)
  LfpConsentOfferView get offer => $_getN(0);
  @$pb.TagNumber(1)
  set offer(LfpConsentOfferView value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOffer() => $_has(0);
  @$pb.TagNumber(1)
  void clearOffer() => $_clearField(1);
  @$pb.TagNumber(1)
  LfpConsentOfferView ensureOffer() => $_ensure(0);
}

class ObserveLfpConsentOfferRequest extends $pb.GeneratedMessage {
  factory ObserveLfpConsentOfferRequest({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? storyId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  ObserveLfpConsentOfferRequest._();

  factory ObserveLfpConsentOfferRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveLfpConsentOfferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveLfpConsentOfferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'storyId', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveLfpConsentOfferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveLfpConsentOfferRequest copyWith(
          void Function(ObserveLfpConsentOfferRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ObserveLfpConsentOfferRequest))
          as ObserveLfpConsentOfferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveLfpConsentOfferRequest create() =>
      ObserveLfpConsentOfferRequest._();
  @$core.override
  ObserveLfpConsentOfferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveLfpConsentOfferRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveLfpConsentOfferRequest>(create);
  static ObserveLfpConsentOfferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get storyId => $_getN(1);
  @$pb.TagNumber(2)
  set storyId($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoryId() => $_clearField(2);
}

class ObserveLfpConsentOfferResponse extends $pb.GeneratedMessage {
  factory ObserveLfpConsentOfferResponse({
    LfpConsentOfferView? offer,
  }) {
    final result = create();
    if (offer != null) result.offer = offer;
    return result;
  }

  ObserveLfpConsentOfferResponse._();

  factory ObserveLfpConsentOfferResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ObserveLfpConsentOfferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ObserveLfpConsentOfferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<LfpConsentOfferView>(1, _omitFieldNames ? '' : 'offer',
        subBuilder: LfpConsentOfferView.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveLfpConsentOfferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ObserveLfpConsentOfferResponse copyWith(
          void Function(ObserveLfpConsentOfferResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ObserveLfpConsentOfferResponse))
          as ObserveLfpConsentOfferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ObserveLfpConsentOfferResponse create() =>
      ObserveLfpConsentOfferResponse._();
  @$core.override
  ObserveLfpConsentOfferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ObserveLfpConsentOfferResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ObserveLfpConsentOfferResponse>(create);
  static ObserveLfpConsentOfferResponse? _defaultInstance;

  @$pb.TagNumber(1)
  LfpConsentOfferView get offer => $_getN(0);
  @$pb.TagNumber(1)
  set offer(LfpConsentOfferView value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOffer() => $_has(0);
  @$pb.TagNumber(1)
  void clearOffer() => $_clearField(1);
  @$pb.TagNumber(1)
  LfpConsentOfferView ensureOffer() => $_ensure(0);
}

class RespondWithLfpConsentRequest extends $pb.GeneratedMessage {
  factory RespondWithLfpConsentRequest({
    $fixnum.Int64? protocolVersion,
    $core.List<$core.int>? responseOperationId,
    LfpConsentOfferView? offer,
    $1.LfpConsentResponseType? responseType,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (responseOperationId != null)
      result.responseOperationId = responseOperationId;
    if (offer != null) result.offer = offer;
    if (responseType != null) result.responseType = responseType;
    return result;
  }

  RespondWithLfpConsentRequest._();

  factory RespondWithLfpConsentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RespondWithLfpConsentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RespondWithLfpConsentRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'protocolVersion', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'responseOperationId', $pb.PbFieldType.OY)
    ..aOM<LfpConsentOfferView>(3, _omitFieldNames ? '' : 'offer',
        subBuilder: LfpConsentOfferView.create)
    ..aE<$1.LfpConsentResponseType>(4, _omitFieldNames ? '' : 'responseType',
        enumValues: $1.LfpConsentResponseType.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RespondWithLfpConsentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RespondWithLfpConsentRequest copyWith(
          void Function(RespondWithLfpConsentRequest) updates) =>
      super.copyWith(
              (message) => updates(message as RespondWithLfpConsentRequest))
          as RespondWithLfpConsentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RespondWithLfpConsentRequest create() =>
      RespondWithLfpConsentRequest._();
  @$core.override
  RespondWithLfpConsentRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RespondWithLfpConsentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RespondWithLfpConsentRequest>(create);
  static RespondWithLfpConsentRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get protocolVersion => $_getI64(0);
  @$pb.TagNumber(1)
  set protocolVersion($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  /// Stable domain operation; every network attempt has a fresh signed JTI.
  @$pb.TagNumber(2)
  $core.List<$core.int> get responseOperationId => $_getN(1);
  @$pb.TagNumber(2)
  set responseOperationId($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasResponseOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearResponseOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  LfpConsentOfferView get offer => $_getN(2);
  @$pb.TagNumber(3)
  set offer(LfpConsentOfferView value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasOffer() => $_has(2);
  @$pb.TagNumber(3)
  void clearOffer() => $_clearField(3);
  @$pb.TagNumber(3)
  LfpConsentOfferView ensureOffer() => $_ensure(2);

  @$pb.TagNumber(4)
  $1.LfpConsentResponseType get responseType => $_getN(3);
  @$pb.TagNumber(4)
  set responseType($1.LfpConsentResponseType value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasResponseType() => $_has(3);
  @$pb.TagNumber(4)
  void clearResponseType() => $_clearField(4);
}

/// Success only after Story's immutable consent and original MM registration
/// acknowledgement are durable. Private receipt/account/epoch is not returned.
class RespondWithLfpConsentResponse extends $pb.GeneratedMessage {
  factory RespondWithLfpConsentResponse() => create();

  RespondWithLfpConsentResponse._();

  factory RespondWithLfpConsentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RespondWithLfpConsentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RespondWithLfpConsentResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RespondWithLfpConsentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RespondWithLfpConsentResponse copyWith(
          void Function(RespondWithLfpConsentResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RespondWithLfpConsentResponse))
          as RespondWithLfpConsentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RespondWithLfpConsentResponse create() =>
      RespondWithLfpConsentResponse._();
  @$core.override
  RespondWithLfpConsentResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RespondWithLfpConsentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RespondWithLfpConsentResponse>(create);
  static RespondWithLfpConsentResponse? _defaultInstance;
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
