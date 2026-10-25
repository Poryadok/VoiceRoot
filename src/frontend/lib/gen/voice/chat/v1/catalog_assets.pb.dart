// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/catalog_assets.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'chat.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// @voice.unknown_fields=reject
class ResolveGifAssetRequest extends $pb.GeneratedMessage {
  factory ResolveGifAssetRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? provider,
    $core.String? providerId,
    $1.ChatRef? destinationChat,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (destinationChat != null) result.destinationChat = destinationChat;
    return result;
  }

  ResolveGifAssetRequest._();

  factory ResolveGifAssetRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveGifAssetRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveGifAssetRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'provider')
    ..aOS(4, _omitFieldNames ? '' : 'providerId')
    ..aOM<$1.ChatRef>(5, _omitFieldNames ? '' : 'destinationChat',
        subBuilder: $1.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveGifAssetRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveGifAssetRequest copyWith(
          void Function(ResolveGifAssetRequest) updates) =>
      super.copyWith((message) => updates(message as ResolveGifAssetRequest))
          as ResolveGifAssetRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveGifAssetRequest create() => ResolveGifAssetRequest._();
  @$core.override
  ResolveGifAssetRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveGifAssetRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveGifAssetRequest>(create);
  static ResolveGifAssetRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get operationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get provider => $_getSZ(2);
  @$pb.TagNumber(3)
  set provider($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProvider() => $_has(2);
  @$pb.TagNumber(3)
  void clearProvider() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get providerId => $_getSZ(3);
  @$pb.TagNumber(4)
  set providerId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProviderId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProviderId() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.ChatRef get destinationChat => $_getN(4);
  @$pb.TagNumber(5)
  set destinationChat($1.ChatRef value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasDestinationChat() => $_has(4);
  @$pb.TagNumber(5)
  void clearDestinationChat() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.ChatRef ensureDestinationChat() => $_ensure(4);
}

/// Contains no source URL, storage key or authorization capability.
class ResolveGifAssetResponse extends $pb.GeneratedMessage {
  factory ResolveGifAssetResponse({
    $core.String? sourceId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? fileId,
    $core.String? status,
  }) {
    final result = create();
    if (sourceId != null) result.sourceId = sourceId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (fileId != null) result.fileId = fileId;
    if (status != null) result.status = status;
    return result;
  }

  ResolveGifAssetResponse._();

  factory ResolveGifAssetResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveGifAssetResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveGifAssetResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sourceId')
    ..aOS(2, _omitFieldNames ? '' : 'provider')
    ..aOS(3, _omitFieldNames ? '' : 'providerId')
    ..aOS(4, _omitFieldNames ? '' : 'fileId')
    ..aOS(5, _omitFieldNames ? '' : 'status')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveGifAssetResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveGifAssetResponse copyWith(
          void Function(ResolveGifAssetResponse) updates) =>
      super.copyWith((message) => updates(message as ResolveGifAssetResponse))
          as ResolveGifAssetResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveGifAssetResponse create() => ResolveGifAssetResponse._();
  @$core.override
  ResolveGifAssetResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveGifAssetResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveGifAssetResponse>(create);
  static ResolveGifAssetResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sourceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sourceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSourceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSourceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get provider => $_getSZ(1);
  @$pb.TagNumber(2)
  set provider($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProvider() => $_has(1);
  @$pb.TagNumber(2)
  void clearProvider() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get providerId => $_getSZ(2);
  @$pb.TagNumber(3)
  set providerId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProviderId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProviderId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get fileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set fileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearFileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get status => $_getSZ(4);
  @$pb.TagNumber(5)
  set status($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStatus() => $_has(4);
  @$pb.TagNumber(5)
  void clearStatus() => $_clearField(5);
}

/// @voice.unknown_fields=reject
class ValidateGifAssetRequest extends $pb.GeneratedMessage {
  factory ValidateGifAssetRequest({
    $core.int? protocolVersion,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? fileId,
    $1.ChatRef? destinationChat,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (fileId != null) result.fileId = fileId;
    if (destinationChat != null) result.destinationChat = destinationChat;
    return result;
  }

  ValidateGifAssetRequest._();

  factory ValidateGifAssetRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateGifAssetRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateGifAssetRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'provider')
    ..aOS(5, _omitFieldNames ? '' : 'providerId')
    ..aOS(6, _omitFieldNames ? '' : 'fileId')
    ..aOM<$1.ChatRef>(7, _omitFieldNames ? '' : 'destinationChat',
        subBuilder: $1.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateGifAssetRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateGifAssetRequest copyWith(
          void Function(ValidateGifAssetRequest) updates) =>
      super.copyWith((message) => updates(message as ValidateGifAssetRequest))
          as ValidateGifAssetRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateGifAssetRequest create() => ValidateGifAssetRequest._();
  @$core.override
  ValidateGifAssetRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateGifAssetRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateGifAssetRequest>(create);
  static ValidateGifAssetRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get actorAccountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set actorAccountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasActorAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearActorAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get actorProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set actorProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasActorProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearActorProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get provider => $_getSZ(3);
  @$pb.TagNumber(4)
  set provider($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProvider() => $_has(3);
  @$pb.TagNumber(4)
  void clearProvider() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get providerId => $_getSZ(4);
  @$pb.TagNumber(5)
  set providerId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasProviderId() => $_has(4);
  @$pb.TagNumber(5)
  void clearProviderId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get fileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set fileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasFileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearFileId() => $_clearField(6);

  @$pb.TagNumber(7)
  $1.ChatRef get destinationChat => $_getN(6);
  @$pb.TagNumber(7)
  set destinationChat($1.ChatRef value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasDestinationChat() => $_has(6);
  @$pb.TagNumber(7)
  void clearDestinationChat() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.ChatRef ensureDestinationChat() => $_ensure(6);
}

/// Exact source tuple only. File owns authoritative representation/readiness.
class ValidateGifAssetResponse extends $pb.GeneratedMessage {
  factory ValidateGifAssetResponse({
    $core.String? sourceId,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? fileId,
  }) {
    final result = create();
    if (sourceId != null) result.sourceId = sourceId;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (fileId != null) result.fileId = fileId;
    return result;
  }

  ValidateGifAssetResponse._();

  factory ValidateGifAssetResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateGifAssetResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateGifAssetResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sourceId')
    ..aOS(2, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'provider')
    ..aOS(5, _omitFieldNames ? '' : 'providerId')
    ..aOS(6, _omitFieldNames ? '' : 'fileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateGifAssetResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateGifAssetResponse copyWith(
          void Function(ValidateGifAssetResponse) updates) =>
      super.copyWith((message) => updates(message as ValidateGifAssetResponse))
          as ValidateGifAssetResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateGifAssetResponse create() => ValidateGifAssetResponse._();
  @$core.override
  ValidateGifAssetResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateGifAssetResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateGifAssetResponse>(create);
  static ValidateGifAssetResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sourceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sourceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSourceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSourceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get actorAccountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set actorAccountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasActorAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearActorAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get actorProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set actorProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasActorProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearActorProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get provider => $_getSZ(3);
  @$pb.TagNumber(4)
  set provider($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProvider() => $_has(3);
  @$pb.TagNumber(4)
  void clearProvider() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get providerId => $_getSZ(4);
  @$pb.TagNumber(5)
  set providerId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasProviderId() => $_has(4);
  @$pb.TagNumber(5)
  void clearProviderId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get fileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set fileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasFileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearFileId() => $_clearField(6);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
