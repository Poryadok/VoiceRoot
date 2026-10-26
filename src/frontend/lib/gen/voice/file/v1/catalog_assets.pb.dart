// This is a generated file - do not edit.
//
// Generated from voice/file/v1/catalog_assets.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import '../../chat/v1/chat.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ImportCatalogAssetRequest extends $pb.GeneratedMessage {
  factory ImportCatalogAssetRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? sourceId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? sourceUrl,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
    $1.ChatRef? destinationChat,
    $core.String? chatAccountGeneration,
    $core.String? chatProfileGeneration,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (sourceId != null) result.sourceId = sourceId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (sourceUrl != null) result.sourceUrl = sourceUrl;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (destinationChat != null) result.destinationChat = destinationChat;
    if (chatAccountGeneration != null)
      result.chatAccountGeneration = chatAccountGeneration;
    if (chatProfileGeneration != null)
      result.chatProfileGeneration = chatProfileGeneration;
    return result;
  }

  ImportCatalogAssetRequest._();

  factory ImportCatalogAssetRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ImportCatalogAssetRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportCatalogAssetRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'sourceId')
    ..aOS(4, _omitFieldNames ? '' : 'provider')
    ..aOS(5, _omitFieldNames ? '' : 'providerId')
    ..aOS(6, _omitFieldNames ? '' : 'sourceUrl')
    ..aOS(7, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(8, _omitFieldNames ? '' : 'actorProfileId')
    ..aOM<$1.ChatRef>(9, _omitFieldNames ? '' : 'destinationChat',
        subBuilder: $1.ChatRef.create)
    ..aOS(10, _omitFieldNames ? '' : 'chatAccountGeneration')
    ..aOS(11, _omitFieldNames ? '' : 'chatProfileGeneration')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportCatalogAssetRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportCatalogAssetRequest copyWith(
          void Function(ImportCatalogAssetRequest) updates) =>
      super.copyWith((message) => updates(message as ImportCatalogAssetRequest))
          as ImportCatalogAssetRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ImportCatalogAssetRequest create() => ImportCatalogAssetRequest._();
  @$core.override
  ImportCatalogAssetRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ImportCatalogAssetRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImportCatalogAssetRequest>(create);
  static ImportCatalogAssetRequest? _defaultInstance;

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
  $core.String get sourceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set sourceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSourceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSourceId() => $_clearField(3);

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

  /// Protected internal data, resolved by Chat. Never a public upload parameter.
  @$pb.TagNumber(6)
  $core.String get sourceUrl => $_getSZ(5);
  @$pb.TagNumber(6)
  set sourceUrl($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSourceUrl() => $_has(5);
  @$pb.TagNumber(6)
  void clearSourceUrl() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get actorAccountId => $_getSZ(6);
  @$pb.TagNumber(7)
  set actorAccountId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasActorAccountId() => $_has(6);
  @$pb.TagNumber(7)
  void clearActorAccountId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get actorProfileId => $_getSZ(7);
  @$pb.TagNumber(8)
  set actorProfileId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasActorProfileId() => $_has(7);
  @$pb.TagNumber(8)
  void clearActorProfileId() => $_clearField(8);

  @$pb.TagNumber(9)
  $1.ChatRef get destinationChat => $_getN(8);
  @$pb.TagNumber(9)
  set destinationChat($1.ChatRef value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasDestinationChat() => $_has(8);
  @$pb.TagNumber(9)
  void clearDestinationChat() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.ChatRef ensureDestinationChat() => $_ensure(8);

  /// Opaque Chat-local captures. File also enforces its own actor generations.
  @$pb.TagNumber(10)
  $core.String get chatAccountGeneration => $_getSZ(9);
  @$pb.TagNumber(10)
  set chatAccountGeneration($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasChatAccountGeneration() => $_has(9);
  @$pb.TagNumber(10)
  void clearChatAccountGeneration() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get chatProfileGeneration => $_getSZ(10);
  @$pb.TagNumber(11)
  set chatProfileGeneration($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasChatProfileGeneration() => $_has(10);
  @$pb.TagNumber(11)
  void clearChatProfileGeneration() => $_clearField(11);
}

/// @voice.unknown_fields=accept_preserve
class ImportCatalogAssetResponse extends $pb.GeneratedMessage {
  factory ImportCatalogAssetResponse({
    $core.String? sourceId,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? fileId,
    $core.String? status,
    $core.String? operationId,
    $1.ChatRef? destinationChat,
    $core.String? chatAccountGeneration,
    $core.String? chatProfileGeneration,
  }) {
    final result = create();
    if (sourceId != null) result.sourceId = sourceId;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (fileId != null) result.fileId = fileId;
    if (status != null) result.status = status;
    if (operationId != null) result.operationId = operationId;
    if (destinationChat != null) result.destinationChat = destinationChat;
    if (chatAccountGeneration != null)
      result.chatAccountGeneration = chatAccountGeneration;
    if (chatProfileGeneration != null)
      result.chatProfileGeneration = chatProfileGeneration;
    return result;
  }

  ImportCatalogAssetResponse._();

  factory ImportCatalogAssetResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ImportCatalogAssetResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportCatalogAssetResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sourceId')
    ..aOS(2, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'provider')
    ..aOS(5, _omitFieldNames ? '' : 'providerId')
    ..aOS(6, _omitFieldNames ? '' : 'fileId')
    ..aOS(7, _omitFieldNames ? '' : 'status')
    ..aOS(8, _omitFieldNames ? '' : 'operationId')
    ..aOM<$1.ChatRef>(9, _omitFieldNames ? '' : 'destinationChat',
        subBuilder: $1.ChatRef.create)
    ..aOS(10, _omitFieldNames ? '' : 'chatAccountGeneration')
    ..aOS(11, _omitFieldNames ? '' : 'chatProfileGeneration')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportCatalogAssetResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportCatalogAssetResponse copyWith(
          void Function(ImportCatalogAssetResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ImportCatalogAssetResponse))
          as ImportCatalogAssetResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ImportCatalogAssetResponse create() => ImportCatalogAssetResponse._();
  @$core.override
  ImportCatalogAssetResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ImportCatalogAssetResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImportCatalogAssetResponse>(create);
  static ImportCatalogAssetResponse? _defaultInstance;

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

  /// Existing File lifecycle vocabulary; a reservation alone returns no response.
  @$pb.TagNumber(7)
  $core.String get status => $_getSZ(6);
  @$pb.TagNumber(7)
  set status($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasStatus() => $_has(6);
  @$pb.TagNumber(7)
  void clearStatus() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get operationId => $_getSZ(7);
  @$pb.TagNumber(8)
  set operationId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasOperationId() => $_has(7);
  @$pb.TagNumber(8)
  void clearOperationId() => $_clearField(8);

  @$pb.TagNumber(9)
  $1.ChatRef get destinationChat => $_getN(8);
  @$pb.TagNumber(9)
  set destinationChat($1.ChatRef value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasDestinationChat() => $_has(8);
  @$pb.TagNumber(9)
  void clearDestinationChat() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.ChatRef ensureDestinationChat() => $_ensure(8);

  @$pb.TagNumber(10)
  $core.String get chatAccountGeneration => $_getSZ(9);
  @$pb.TagNumber(10)
  set chatAccountGeneration($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasChatAccountGeneration() => $_has(9);
  @$pb.TagNumber(10)
  void clearChatAccountGeneration() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get chatProfileGeneration => $_getSZ(10);
  @$pb.TagNumber(11)
  set chatProfileGeneration($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasChatProfileGeneration() => $_has(10);
  @$pb.TagNumber(11)
  void clearChatProfileGeneration() => $_clearField(11);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ValidateCatalogAssetRequest extends $pb.GeneratedMessage {
  factory ValidateCatalogAssetRequest({
    $core.int? protocolVersion,
    $core.String? sourceId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? fileId,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (sourceId != null) result.sourceId = sourceId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (fileId != null) result.fileId = fileId;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    return result;
  }

  ValidateCatalogAssetRequest._();

  factory ValidateCatalogAssetRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateCatalogAssetRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateCatalogAssetRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'sourceId')
    ..aOS(3, _omitFieldNames ? '' : 'provider')
    ..aOS(4, _omitFieldNames ? '' : 'providerId')
    ..aOS(5, _omitFieldNames ? '' : 'fileId')
    ..aOS(6, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(7, _omitFieldNames ? '' : 'actorProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateCatalogAssetRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateCatalogAssetRequest copyWith(
          void Function(ValidateCatalogAssetRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ValidateCatalogAssetRequest))
          as ValidateCatalogAssetRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateCatalogAssetRequest create() =>
      ValidateCatalogAssetRequest._();
  @$core.override
  ValidateCatalogAssetRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateCatalogAssetRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateCatalogAssetRequest>(create);
  static ValidateCatalogAssetRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sourceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sourceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSourceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSourceId() => $_clearField(2);

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
  $core.String get fileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set fileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasFileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearFileId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get actorAccountId => $_getSZ(5);
  @$pb.TagNumber(6)
  set actorAccountId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasActorAccountId() => $_has(5);
  @$pb.TagNumber(6)
  void clearActorAccountId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get actorProfileId => $_getSZ(6);
  @$pb.TagNumber(7)
  set actorProfileId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasActorProfileId() => $_has(6);
  @$pb.TagNumber(7)
  void clearActorProfileId() => $_clearField(7);
}

/// A successful response attests a READY, non-E2E, unexpired catalog GIF only.
/// This is source validation; it grants no destination or viewer permission.
/// @voice.unknown_fields=accept_preserve
class ValidateCatalogAssetResponse extends $pb.GeneratedMessage {
  factory ValidateCatalogAssetResponse({
    $core.String? sourceId,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? fileId,
    $fixnum.Int64? objectRevision,
    $core.String? intent,
    $core.String? scanResult,
    $core.String? fileType,
    $core.List<$core.int>? sourceSha256,
    $core.List<$core.int>? deliveredSha256,
    $fixnum.Int64? deliveredSizeBytes,
    $core.String? deliveredMimeType,
  }) {
    final result = create();
    if (sourceId != null) result.sourceId = sourceId;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (fileId != null) result.fileId = fileId;
    if (objectRevision != null) result.objectRevision = objectRevision;
    if (intent != null) result.intent = intent;
    if (scanResult != null) result.scanResult = scanResult;
    if (fileType != null) result.fileType = fileType;
    if (sourceSha256 != null) result.sourceSha256 = sourceSha256;
    if (deliveredSha256 != null) result.deliveredSha256 = deliveredSha256;
    if (deliveredSizeBytes != null)
      result.deliveredSizeBytes = deliveredSizeBytes;
    if (deliveredMimeType != null) result.deliveredMimeType = deliveredMimeType;
    return result;
  }

  ValidateCatalogAssetResponse._();

  factory ValidateCatalogAssetResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateCatalogAssetResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateCatalogAssetResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sourceId')
    ..aOS(2, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'provider')
    ..aOS(5, _omitFieldNames ? '' : 'providerId')
    ..aOS(6, _omitFieldNames ? '' : 'fileId')
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'objectRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(8, _omitFieldNames ? '' : 'intent')
    ..aOS(9, _omitFieldNames ? '' : 'scanResult')
    ..aOS(10, _omitFieldNames ? '' : 'fileType')
    ..a<$core.List<$core.int>>(
        11, _omitFieldNames ? '' : 'sourceSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        12, _omitFieldNames ? '' : 'deliveredSha256', $pb.PbFieldType.OY)
    ..aInt64(13, _omitFieldNames ? '' : 'deliveredSizeBytes')
    ..aOS(14, _omitFieldNames ? '' : 'deliveredMimeType')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateCatalogAssetResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateCatalogAssetResponse copyWith(
          void Function(ValidateCatalogAssetResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ValidateCatalogAssetResponse))
          as ValidateCatalogAssetResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateCatalogAssetResponse create() =>
      ValidateCatalogAssetResponse._();
  @$core.override
  ValidateCatalogAssetResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateCatalogAssetResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateCatalogAssetResponse>(create);
  static ValidateCatalogAssetResponse? _defaultInstance;

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

  @$pb.TagNumber(7)
  $fixnum.Int64 get objectRevision => $_getI64(6);
  @$pb.TagNumber(7)
  set objectRevision($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasObjectRevision() => $_has(6);
  @$pb.TagNumber(7)
  void clearObjectRevision() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get intent => $_getSZ(7);
  @$pb.TagNumber(8)
  set intent($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasIntent() => $_has(7);
  @$pb.TagNumber(8)
  void clearIntent() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get scanResult => $_getSZ(8);
  @$pb.TagNumber(9)
  set scanResult($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasScanResult() => $_has(8);
  @$pb.TagNumber(9)
  void clearScanResult() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get fileType => $_getSZ(9);
  @$pb.TagNumber(10)
  set fileType($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasFileType() => $_has(9);
  @$pb.TagNumber(10)
  void clearFileType() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.List<$core.int> get sourceSha256 => $_getN(10);
  @$pb.TagNumber(11)
  set sourceSha256($core.List<$core.int> value) => $_setBytes(10, value);
  @$pb.TagNumber(11)
  $core.bool hasSourceSha256() => $_has(10);
  @$pb.TagNumber(11)
  void clearSourceSha256() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.List<$core.int> get deliveredSha256 => $_getN(11);
  @$pb.TagNumber(12)
  set deliveredSha256($core.List<$core.int> value) => $_setBytes(11, value);
  @$pb.TagNumber(12)
  $core.bool hasDeliveredSha256() => $_has(11);
  @$pb.TagNumber(12)
  void clearDeliveredSha256() => $_clearField(12);

  @$pb.TagNumber(13)
  $fixnum.Int64 get deliveredSizeBytes => $_getI64(12);
  @$pb.TagNumber(13)
  set deliveredSizeBytes($fixnum.Int64 value) => $_setInt64(12, value);
  @$pb.TagNumber(13)
  $core.bool hasDeliveredSizeBytes() => $_has(12);
  @$pb.TagNumber(13)
  void clearDeliveredSizeBytes() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get deliveredMimeType => $_getSZ(13);
  @$pb.TagNumber(14)
  set deliveredMimeType($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasDeliveredMimeType() => $_has(13);
  @$pb.TagNumber(14)
  void clearDeliveredMimeType() => $_clearField(14);
}

/// Server-owned classification. Client attribution does not choose the ACL path.
/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class LookupCatalogAssetRequest extends $pb.GeneratedMessage {
  factory LookupCatalogAssetRequest({
    $core.int? protocolVersion,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
    $core.String? fileId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (fileId != null) result.fileId = fileId;
    return result;
  }

  LookupCatalogAssetRequest._();

  factory LookupCatalogAssetRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LookupCatalogAssetRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LookupCatalogAssetRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'fileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupCatalogAssetRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupCatalogAssetRequest copyWith(
          void Function(LookupCatalogAssetRequest) updates) =>
      super.copyWith((message) => updates(message as LookupCatalogAssetRequest))
          as LookupCatalogAssetRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LookupCatalogAssetRequest create() => LookupCatalogAssetRequest._();
  @$core.override
  LookupCatalogAssetRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LookupCatalogAssetRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LookupCatalogAssetRequest>(create);
  static LookupCatalogAssetRequest? _defaultInstance;

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
  $core.String get fileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set fileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearFileId() => $_clearField(4);
}

enum LookupCatalogAssetResponse_Classification { catalog, ordinaryFile, notSet }

/// Unknown/unavailable Files and unauthorized catalog actors return an error.
/// @voice.unknown_fields=accept_preserve
class LookupCatalogAssetResponse extends $pb.GeneratedMessage {
  factory LookupCatalogAssetResponse({
    ValidateCatalogAssetResponse? catalog,
    $core.bool? ordinaryFile,
  }) {
    final result = create();
    if (catalog != null) result.catalog = catalog;
    if (ordinaryFile != null) result.ordinaryFile = ordinaryFile;
    return result;
  }

  LookupCatalogAssetResponse._();

  factory LookupCatalogAssetResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LookupCatalogAssetResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, LookupCatalogAssetResponse_Classification>
      _LookupCatalogAssetResponse_ClassificationByTag = {
    1: LookupCatalogAssetResponse_Classification.catalog,
    2: LookupCatalogAssetResponse_Classification.ordinaryFile,
    0: LookupCatalogAssetResponse_Classification.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LookupCatalogAssetResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..oo(0, [1, 2])
    ..aOM<ValidateCatalogAssetResponse>(1, _omitFieldNames ? '' : 'catalog',
        subBuilder: ValidateCatalogAssetResponse.create)
    ..aOB(2, _omitFieldNames ? '' : 'ordinaryFile')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupCatalogAssetResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LookupCatalogAssetResponse copyWith(
          void Function(LookupCatalogAssetResponse) updates) =>
      super.copyWith(
              (message) => updates(message as LookupCatalogAssetResponse))
          as LookupCatalogAssetResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LookupCatalogAssetResponse create() => LookupCatalogAssetResponse._();
  @$core.override
  LookupCatalogAssetResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LookupCatalogAssetResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LookupCatalogAssetResponse>(create);
  static LookupCatalogAssetResponse? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  LookupCatalogAssetResponse_Classification whichClassification() =>
      _LookupCatalogAssetResponse_ClassificationByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  void clearClassification() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  ValidateCatalogAssetResponse get catalog => $_getN(0);
  @$pb.TagNumber(1)
  set catalog(ValidateCatalogAssetResponse value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCatalog() => $_has(0);
  @$pb.TagNumber(1)
  void clearCatalog() => $_clearField(1);
  @$pb.TagNumber(1)
  ValidateCatalogAssetResponse ensureCatalog() => $_ensure(0);

  /// True only after finding a real File with no catalog provenance.
  @$pb.TagNumber(2)
  $core.bool get ordinaryFile => $_getBF(1);
  @$pb.TagNumber(2)
  set ordinaryFile($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOrdinaryFile() => $_has(1);
  @$pb.TagNumber(2)
  void clearOrdinaryFile() => $_clearField(2);
}

/// The public selection identity, without a remote URL or derived source UUID.
/// @voice.unknown_fields=reject
class CatalogImportOperationSelector extends $pb.GeneratedMessage {
  factory CatalogImportOperationSelector({
    $core.String? operationId,
    $core.String? provider,
    $core.String? providerId,
    $1.ChatRef? destinationChat,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (destinationChat != null) result.destinationChat = destinationChat;
    return result;
  }

  CatalogImportOperationSelector._();

  factory CatalogImportOperationSelector.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CatalogImportOperationSelector.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CatalogImportOperationSelector',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'provider')
    ..aOS(3, _omitFieldNames ? '' : 'providerId')
    ..aOM<$1.ChatRef>(4, _omitFieldNames ? '' : 'destinationChat',
        subBuilder: $1.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CatalogImportOperationSelector clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CatalogImportOperationSelector copyWith(
          void Function(CatalogImportOperationSelector) updates) =>
      super.copyWith(
              (message) => updates(message as CatalogImportOperationSelector))
          as CatalogImportOperationSelector;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CatalogImportOperationSelector create() =>
      CatalogImportOperationSelector._();
  @$core.override
  CatalogImportOperationSelector createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CatalogImportOperationSelector getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CatalogImportOperationSelector>(create);
  static CatalogImportOperationSelector? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

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
  $1.ChatRef get destinationChat => $_getN(3);
  @$pb.TagNumber(4)
  set destinationChat($1.ChatRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasDestinationChat() => $_has(3);
  @$pb.TagNumber(4)
  void clearDestinationChat() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.ChatRef ensureDestinationChat() => $_ensure(3);
}

/// @voice.unknown_fields=reject
class CatalogAssetSelector extends $pb.GeneratedMessage {
  factory CatalogAssetSelector({
    $core.String? sourceId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? fileId,
  }) {
    final result = create();
    if (sourceId != null) result.sourceId = sourceId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (fileId != null) result.fileId = fileId;
    return result;
  }

  CatalogAssetSelector._();

  factory CatalogAssetSelector.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CatalogAssetSelector.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CatalogAssetSelector',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sourceId')
    ..aOS(2, _omitFieldNames ? '' : 'provider')
    ..aOS(3, _omitFieldNames ? '' : 'providerId')
    ..aOS(4, _omitFieldNames ? '' : 'fileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CatalogAssetSelector clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CatalogAssetSelector copyWith(void Function(CatalogAssetSelector) updates) =>
      super.copyWith((message) => updates(message as CatalogAssetSelector))
          as CatalogAssetSelector;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CatalogAssetSelector create() => CatalogAssetSelector._();
  @$core.override
  CatalogAssetSelector createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CatalogAssetSelector getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CatalogAssetSelector>(create);
  static CatalogAssetSelector? _defaultInstance;

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
}

enum GetCatalogAssetStatusRequest_Selector { operation, asset, notSet }

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class GetCatalogAssetStatusRequest extends $pb.GeneratedMessage {
  factory GetCatalogAssetStatusRequest({
    $core.int? protocolVersion,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
    CatalogImportOperationSelector? operation,
    CatalogAssetSelector? asset,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (operation != null) result.operation = operation;
    if (asset != null) result.asset = asset;
    return result;
  }

  GetCatalogAssetStatusRequest._();

  factory GetCatalogAssetStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetCatalogAssetStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, GetCatalogAssetStatusRequest_Selector>
      _GetCatalogAssetStatusRequest_SelectorByTag = {
    4: GetCatalogAssetStatusRequest_Selector.operation,
    5: GetCatalogAssetStatusRequest_Selector.asset,
    0: GetCatalogAssetStatusRequest_Selector.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCatalogAssetStatusRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..oo(0, [4, 5])
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOM<CatalogImportOperationSelector>(4, _omitFieldNames ? '' : 'operation',
        subBuilder: CatalogImportOperationSelector.create)
    ..aOM<CatalogAssetSelector>(5, _omitFieldNames ? '' : 'asset',
        subBuilder: CatalogAssetSelector.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCatalogAssetStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCatalogAssetStatusRequest copyWith(
          void Function(GetCatalogAssetStatusRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetCatalogAssetStatusRequest))
          as GetCatalogAssetStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetCatalogAssetStatusRequest create() =>
      GetCatalogAssetStatusRequest._();
  @$core.override
  GetCatalogAssetStatusRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetCatalogAssetStatusRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCatalogAssetStatusRequest>(create);
  static GetCatalogAssetStatusRequest? _defaultInstance;

  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  GetCatalogAssetStatusRequest_Selector whichSelector() =>
      _GetCatalogAssetStatusRequest_SelectorByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  void clearSelector() => $_clearField($_whichOneof(0));

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
  CatalogImportOperationSelector get operation => $_getN(3);
  @$pb.TagNumber(4)
  set operation(CatalogImportOperationSelector value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasOperation() => $_has(3);
  @$pb.TagNumber(4)
  void clearOperation() => $_clearField(4);
  @$pb.TagNumber(4)
  CatalogImportOperationSelector ensureOperation() => $_ensure(3);

  @$pb.TagNumber(5)
  CatalogAssetSelector get asset => $_getN(4);
  @$pb.TagNumber(5)
  set asset(CatalogAssetSelector value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasAsset() => $_has(4);
  @$pb.TagNumber(5)
  void clearAsset() => $_clearField(5);
  @$pb.TagNumber(5)
  CatalogAssetSelector ensureAsset() => $_ensure(4);
}

/// Operation lookup precedes resolving a new source, including after response
/// loss and CDN URL rotation. Compacted operations return terminal/unavailable.
/// @voice.unknown_fields=accept_preserve
class GetCatalogAssetStatusResponse extends $pb.GeneratedMessage {
  factory GetCatalogAssetStatusResponse({
    $core.String? sourceId,
    $core.String? actorAccountId,
    $core.String? actorProfileId,
    $core.String? provider,
    $core.String? providerId,
    $core.String? fileId,
    $core.String? status,
    $core.String? operationId,
    $1.ChatRef? destinationChat,
    $core.String? chatAccountGeneration,
    $core.String? chatProfileGeneration,
    $core.String? terminalFenceId,
  }) {
    final result = create();
    if (sourceId != null) result.sourceId = sourceId;
    if (actorAccountId != null) result.actorAccountId = actorAccountId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (provider != null) result.provider = provider;
    if (providerId != null) result.providerId = providerId;
    if (fileId != null) result.fileId = fileId;
    if (status != null) result.status = status;
    if (operationId != null) result.operationId = operationId;
    if (destinationChat != null) result.destinationChat = destinationChat;
    if (chatAccountGeneration != null)
      result.chatAccountGeneration = chatAccountGeneration;
    if (chatProfileGeneration != null)
      result.chatProfileGeneration = chatProfileGeneration;
    if (terminalFenceId != null) result.terminalFenceId = terminalFenceId;
    return result;
  }

  GetCatalogAssetStatusResponse._();

  factory GetCatalogAssetStatusResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetCatalogAssetStatusResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCatalogAssetStatusResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sourceId')
    ..aOS(2, _omitFieldNames ? '' : 'actorAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'provider')
    ..aOS(5, _omitFieldNames ? '' : 'providerId')
    ..aOS(6, _omitFieldNames ? '' : 'fileId')
    ..aOS(7, _omitFieldNames ? '' : 'status')
    ..aOS(8, _omitFieldNames ? '' : 'operationId')
    ..aOM<$1.ChatRef>(9, _omitFieldNames ? '' : 'destinationChat',
        subBuilder: $1.ChatRef.create)
    ..aOS(10, _omitFieldNames ? '' : 'chatAccountGeneration')
    ..aOS(11, _omitFieldNames ? '' : 'chatProfileGeneration')
    ..aOS(12, _omitFieldNames ? '' : 'terminalFenceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCatalogAssetStatusResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCatalogAssetStatusResponse copyWith(
          void Function(GetCatalogAssetStatusResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetCatalogAssetStatusResponse))
          as GetCatalogAssetStatusResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetCatalogAssetStatusResponse create() =>
      GetCatalogAssetStatusResponse._();
  @$core.override
  GetCatalogAssetStatusResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetCatalogAssetStatusResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCatalogAssetStatusResponse>(create);
  static GetCatalogAssetStatusResponse? _defaultInstance;

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

  @$pb.TagNumber(7)
  $core.String get status => $_getSZ(6);
  @$pb.TagNumber(7)
  set status($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasStatus() => $_has(6);
  @$pb.TagNumber(7)
  void clearStatus() => $_clearField(7);

  /// Present with the exact destination only for an operation selector.
  @$pb.TagNumber(8)
  $core.String get operationId => $_getSZ(7);
  @$pb.TagNumber(8)
  set operationId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasOperationId() => $_has(7);
  @$pb.TagNumber(8)
  void clearOperationId() => $_clearField(8);

  @$pb.TagNumber(9)
  $1.ChatRef get destinationChat => $_getN(8);
  @$pb.TagNumber(9)
  set destinationChat($1.ChatRef value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasDestinationChat() => $_has(8);
  @$pb.TagNumber(9)
  void clearDestinationChat() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.ChatRef ensureDestinationChat() => $_ensure(8);

  @$pb.TagNumber(10)
  $core.String get chatAccountGeneration => $_getSZ(9);
  @$pb.TagNumber(10)
  set chatAccountGeneration($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasChatAccountGeneration() => $_has(9);
  @$pb.TagNumber(10)
  void clearChatAccountGeneration() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get chatProfileGeneration => $_getSZ(10);
  @$pb.TagNumber(11)
  set chatProfileGeneration($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasChatProfileGeneration() => $_has(10);
  @$pb.TagNumber(11)
  void clearChatProfileGeneration() => $_clearField(11);

  /// Asset selector only: exact-tuple terminal proof, never READY admission.
  /// Both Chat generations may be absent only with this canonical opaque UUID
  /// and status failed/deleted/expired after raw terminal details were erased.
  @$pb.TagNumber(12)
  $core.String get terminalFenceId => $_getSZ(11);
  @$pb.TagNumber(12)
  set terminalFenceId($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasTerminalFenceId() => $_has(11);
  @$pb.TagNumber(12)
  void clearTerminalFenceId() => $_clearField(12);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
