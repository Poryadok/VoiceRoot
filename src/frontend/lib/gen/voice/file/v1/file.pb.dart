// This is a generated file - do not edit.
//
// Generated from voice/file/v1/file.proto.

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
    as $3;

import '../../chat/v1/chat.pb.dart' as $1;
import '../../common/v1/common.pb.dart' as $4;
import '../../common/v1/space_lifecycle.pb.dart' as $5;
import '../../story/v1/story.pb.dart' as $2;
import 'file.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'file.pbenum.dart';

class RequestUploadRequest extends $pb.GeneratedMessage {
  factory RequestUploadRequest({
    $core.String? originalName,
    $core.String? mimeType,
    $fixnum.Int64? sizeBytes,
    $1.ChatRef? contextChat,
    $core.bool? isE2e,
    $2.StoryRef? contextStory,
  }) {
    final result = create();
    if (originalName != null) result.originalName = originalName;
    if (mimeType != null) result.mimeType = mimeType;
    if (sizeBytes != null) result.sizeBytes = sizeBytes;
    if (contextChat != null) result.contextChat = contextChat;
    if (isE2e != null) result.isE2e = isE2e;
    if (contextStory != null) result.contextStory = contextStory;
    return result;
  }

  RequestUploadRequest._();

  factory RequestUploadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RequestUploadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RequestUploadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'originalName')
    ..aOS(2, _omitFieldNames ? '' : 'mimeType')
    ..aInt64(3, _omitFieldNames ? '' : 'sizeBytes')
    ..aOM<$1.ChatRef>(4, _omitFieldNames ? '' : 'contextChat',
        subBuilder: $1.ChatRef.create)
    ..aOB(6, _omitFieldNames ? '' : 'isE2e')
    ..aOM<$2.StoryRef>(7, _omitFieldNames ? '' : 'contextStory',
        subBuilder: $2.StoryRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RequestUploadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RequestUploadRequest copyWith(void Function(RequestUploadRequest) updates) =>
      super.copyWith((message) => updates(message as RequestUploadRequest))
          as RequestUploadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RequestUploadRequest create() => RequestUploadRequest._();
  @$core.override
  RequestUploadRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RequestUploadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RequestUploadRequest>(create);
  static RequestUploadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get originalName => $_getSZ(0);
  @$pb.TagNumber(1)
  set originalName($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOriginalName() => $_has(0);
  @$pb.TagNumber(1)
  void clearOriginalName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get mimeType => $_getSZ(1);
  @$pb.TagNumber(2)
  set mimeType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMimeType() => $_has(1);
  @$pb.TagNumber(2)
  void clearMimeType() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get sizeBytes => $_getI64(2);
  @$pb.TagNumber(3)
  set sizeBytes($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSizeBytes() => $_has(2);
  @$pb.TagNumber(3)
  void clearSizeBytes() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.ChatRef get contextChat => $_getN(3);
  @$pb.TagNumber(4)
  set contextChat($1.ChatRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasContextChat() => $_has(3);
  @$pb.TagNumber(4)
  void clearContextChat() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.ChatRef ensureContextChat() => $_ensure(3);

  @$pb.TagNumber(6)
  $core.bool get isE2e => $_getBF(4);
  @$pb.TagNumber(6)
  set isE2e($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(6)
  $core.bool hasIsE2e() => $_has(4);
  @$pb.TagNumber(6)
  void clearIsE2e() => $_clearField(6);

  @$pb.TagNumber(7)
  $2.StoryRef get contextStory => $_getN(5);
  @$pb.TagNumber(7)
  set contextStory($2.StoryRef value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasContextStory() => $_has(5);
  @$pb.TagNumber(7)
  void clearContextStory() => $_clearField(7);
  @$pb.TagNumber(7)
  $2.StoryRef ensureContextStory() => $_ensure(5);
}

class UploadResponse extends $pb.GeneratedMessage {
  factory UploadResponse({
    $core.String? fileId,
    $core.String? presignedPutUrl,
    $core.String? r2Key,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
    if (presignedPutUrl != null) result.presignedPutUrl = presignedPutUrl;
    if (r2Key != null) result.r2Key = r2Key;
    return result;
  }

  UploadResponse._();

  factory UploadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UploadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UploadResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aOS(2, _omitFieldNames ? '' : 'presignedPutUrl')
    ..aOS(3, _omitFieldNames ? '' : 'r2Key')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UploadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UploadResponse copyWith(void Function(UploadResponse) updates) =>
      super.copyWith((message) => updates(message as UploadResponse))
          as UploadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UploadResponse create() => UploadResponse._();
  @$core.override
  UploadResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UploadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UploadResponse>(create);
  static UploadResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get presignedPutUrl => $_getSZ(1);
  @$pb.TagNumber(2)
  set presignedPutUrl($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPresignedPutUrl() => $_has(1);
  @$pb.TagNumber(2)
  void clearPresignedPutUrl() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get r2Key => $_getSZ(2);
  @$pb.TagNumber(3)
  set r2Key($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasR2Key() => $_has(2);
  @$pb.TagNumber(3)
  void clearR2Key() => $_clearField(3);
}

class ConfirmUploadRequest extends $pb.GeneratedMessage {
  factory ConfirmUploadRequest({
    $core.String? fileId,
    $core.String? sha256Hash,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
    if (sha256Hash != null) result.sha256Hash = sha256Hash;
    return result;
  }

  ConfirmUploadRequest._();

  factory ConfirmUploadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ConfirmUploadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConfirmUploadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aOS(2, _omitFieldNames ? '' : 'sha256Hash')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConfirmUploadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConfirmUploadRequest copyWith(void Function(ConfirmUploadRequest) updates) =>
      super.copyWith((message) => updates(message as ConfirmUploadRequest))
          as ConfirmUploadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ConfirmUploadRequest create() => ConfirmUploadRequest._();
  @$core.override
  ConfirmUploadRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ConfirmUploadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConfirmUploadRequest>(create);
  static ConfirmUploadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sha256Hash => $_getSZ(1);
  @$pb.TagNumber(2)
  set sha256Hash($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSha256Hash() => $_has(1);
  @$pb.TagNumber(2)
  void clearSha256Hash() => $_clearField(2);
}

class FileMetadata extends $pb.GeneratedMessage {
  factory FileMetadata({
    $core.String? id,
    $core.String? uploaderProfileId,
    $core.String? originalName,
    $core.String? mimeType,
    $fixnum.Int64? sizeBytes,
    $core.String? sha256Hash,
    $core.String? r2Key,
    $core.String? status,
    $core.String? fileType,
    $core.int? width,
    $core.int? height,
    $core.int? durationSeconds,
    $core.String? thumbnailR2Key,
    $core.String? convertedR2Key,
    $1.ChatRef? chat,
    $core.bool? isE2e,
    $3.Timestamp? expiresAt,
    $core.String? scanResult,
    $3.Timestamp? createdAt,
    FileLifecycleStatus? statusEnum,
    FileMediaCategory? fileTypeEnum,
    FileScanOutcome? scanResultEnum,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (uploaderProfileId != null) result.uploaderProfileId = uploaderProfileId;
    if (originalName != null) result.originalName = originalName;
    if (mimeType != null) result.mimeType = mimeType;
    if (sizeBytes != null) result.sizeBytes = sizeBytes;
    if (sha256Hash != null) result.sha256Hash = sha256Hash;
    if (r2Key != null) result.r2Key = r2Key;
    if (status != null) result.status = status;
    if (fileType != null) result.fileType = fileType;
    if (width != null) result.width = width;
    if (height != null) result.height = height;
    if (durationSeconds != null) result.durationSeconds = durationSeconds;
    if (thumbnailR2Key != null) result.thumbnailR2Key = thumbnailR2Key;
    if (convertedR2Key != null) result.convertedR2Key = convertedR2Key;
    if (chat != null) result.chat = chat;
    if (isE2e != null) result.isE2e = isE2e;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (scanResult != null) result.scanResult = scanResult;
    if (createdAt != null) result.createdAt = createdAt;
    if (statusEnum != null) result.statusEnum = statusEnum;
    if (fileTypeEnum != null) result.fileTypeEnum = fileTypeEnum;
    if (scanResultEnum != null) result.scanResultEnum = scanResultEnum;
    return result;
  }

  FileMetadata._();

  factory FileMetadata.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileMetadata.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileMetadata',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'uploaderProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'originalName')
    ..aOS(4, _omitFieldNames ? '' : 'mimeType')
    ..aInt64(5, _omitFieldNames ? '' : 'sizeBytes')
    ..aOS(6, _omitFieldNames ? '' : 'sha256Hash')
    ..aOS(7, _omitFieldNames ? '' : 'r2Key')
    ..aOS(8, _omitFieldNames ? '' : 'status')
    ..aOS(9, _omitFieldNames ? '' : 'fileType')
    ..aI(10, _omitFieldNames ? '' : 'width')
    ..aI(11, _omitFieldNames ? '' : 'height')
    ..aI(12, _omitFieldNames ? '' : 'durationSeconds')
    ..aOS(13, _omitFieldNames ? '' : 'thumbnailR2Key')
    ..aOS(14, _omitFieldNames ? '' : 'convertedR2Key')
    ..aOM<$1.ChatRef>(15, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOB(17, _omitFieldNames ? '' : 'isE2e')
    ..aOM<$3.Timestamp>(18, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $3.Timestamp.create)
    ..aOS(19, _omitFieldNames ? '' : 'scanResult')
    ..aOM<$3.Timestamp>(20, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $3.Timestamp.create)
    ..aE<FileLifecycleStatus>(21, _omitFieldNames ? '' : 'statusEnum',
        enumValues: FileLifecycleStatus.values)
    ..aE<FileMediaCategory>(22, _omitFieldNames ? '' : 'fileTypeEnum',
        enumValues: FileMediaCategory.values)
    ..aE<FileScanOutcome>(23, _omitFieldNames ? '' : 'scanResultEnum',
        enumValues: FileScanOutcome.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileMetadata clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileMetadata copyWith(void Function(FileMetadata) updates) =>
      super.copyWith((message) => updates(message as FileMetadata))
          as FileMetadata;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileMetadata create() => FileMetadata._();
  @$core.override
  FileMetadata createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileMetadata getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileMetadata>(create);
  static FileMetadata? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get uploaderProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set uploaderProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUploaderProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearUploaderProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get originalName => $_getSZ(2);
  @$pb.TagNumber(3)
  set originalName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOriginalName() => $_has(2);
  @$pb.TagNumber(3)
  void clearOriginalName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get mimeType => $_getSZ(3);
  @$pb.TagNumber(4)
  set mimeType($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMimeType() => $_has(3);
  @$pb.TagNumber(4)
  void clearMimeType() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get sizeBytes => $_getI64(4);
  @$pb.TagNumber(5)
  set sizeBytes($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSizeBytes() => $_has(4);
  @$pb.TagNumber(5)
  void clearSizeBytes() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get sha256Hash => $_getSZ(5);
  @$pb.TagNumber(6)
  set sha256Hash($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSha256Hash() => $_has(5);
  @$pb.TagNumber(6)
  void clearSha256Hash() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get r2Key => $_getSZ(6);
  @$pb.TagNumber(7)
  set r2Key($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasR2Key() => $_has(6);
  @$pb.TagNumber(7)
  void clearR2Key() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get status => $_getSZ(7);
  @$pb.TagNumber(8)
  set status($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasStatus() => $_has(7);
  @$pb.TagNumber(8)
  void clearStatus() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get fileType => $_getSZ(8);
  @$pb.TagNumber(9)
  set fileType($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasFileType() => $_has(8);
  @$pb.TagNumber(9)
  void clearFileType() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.int get width => $_getIZ(9);
  @$pb.TagNumber(10)
  set width($core.int value) => $_setSignedInt32(9, value);
  @$pb.TagNumber(10)
  $core.bool hasWidth() => $_has(9);
  @$pb.TagNumber(10)
  void clearWidth() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.int get height => $_getIZ(10);
  @$pb.TagNumber(11)
  set height($core.int value) => $_setSignedInt32(10, value);
  @$pb.TagNumber(11)
  $core.bool hasHeight() => $_has(10);
  @$pb.TagNumber(11)
  void clearHeight() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.int get durationSeconds => $_getIZ(11);
  @$pb.TagNumber(12)
  set durationSeconds($core.int value) => $_setSignedInt32(11, value);
  @$pb.TagNumber(12)
  $core.bool hasDurationSeconds() => $_has(11);
  @$pb.TagNumber(12)
  void clearDurationSeconds() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.String get thumbnailR2Key => $_getSZ(12);
  @$pb.TagNumber(13)
  set thumbnailR2Key($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasThumbnailR2Key() => $_has(12);
  @$pb.TagNumber(13)
  void clearThumbnailR2Key() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get convertedR2Key => $_getSZ(13);
  @$pb.TagNumber(14)
  set convertedR2Key($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasConvertedR2Key() => $_has(13);
  @$pb.TagNumber(14)
  void clearConvertedR2Key() => $_clearField(14);

  @$pb.TagNumber(15)
  $1.ChatRef get chat => $_getN(14);
  @$pb.TagNumber(15)
  set chat($1.ChatRef value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasChat() => $_has(14);
  @$pb.TagNumber(15)
  void clearChat() => $_clearField(15);
  @$pb.TagNumber(15)
  $1.ChatRef ensureChat() => $_ensure(14);

  @$pb.TagNumber(17)
  $core.bool get isE2e => $_getBF(15);
  @$pb.TagNumber(17)
  set isE2e($core.bool value) => $_setBool(15, value);
  @$pb.TagNumber(17)
  $core.bool hasIsE2e() => $_has(15);
  @$pb.TagNumber(17)
  void clearIsE2e() => $_clearField(17);

  @$pb.TagNumber(18)
  $3.Timestamp get expiresAt => $_getN(16);
  @$pb.TagNumber(18)
  set expiresAt($3.Timestamp value) => $_setField(18, value);
  @$pb.TagNumber(18)
  $core.bool hasExpiresAt() => $_has(16);
  @$pb.TagNumber(18)
  void clearExpiresAt() => $_clearField(18);
  @$pb.TagNumber(18)
  $3.Timestamp ensureExpiresAt() => $_ensure(16);

  @$pb.TagNumber(19)
  $core.String get scanResult => $_getSZ(17);
  @$pb.TagNumber(19)
  set scanResult($core.String value) => $_setString(17, value);
  @$pb.TagNumber(19)
  $core.bool hasScanResult() => $_has(17);
  @$pb.TagNumber(19)
  void clearScanResult() => $_clearField(19);

  @$pb.TagNumber(20)
  $3.Timestamp get createdAt => $_getN(18);
  @$pb.TagNumber(20)
  set createdAt($3.Timestamp value) => $_setField(20, value);
  @$pb.TagNumber(20)
  $core.bool hasCreatedAt() => $_has(18);
  @$pb.TagNumber(20)
  void clearCreatedAt() => $_clearField(20);
  @$pb.TagNumber(20)
  $3.Timestamp ensureCreatedAt() => $_ensure(18);

  @$pb.TagNumber(21)
  FileLifecycleStatus get statusEnum => $_getN(19);
  @$pb.TagNumber(21)
  set statusEnum(FileLifecycleStatus value) => $_setField(21, value);
  @$pb.TagNumber(21)
  $core.bool hasStatusEnum() => $_has(19);
  @$pb.TagNumber(21)
  void clearStatusEnum() => $_clearField(21);

  @$pb.TagNumber(22)
  FileMediaCategory get fileTypeEnum => $_getN(20);
  @$pb.TagNumber(22)
  set fileTypeEnum(FileMediaCategory value) => $_setField(22, value);
  @$pb.TagNumber(22)
  $core.bool hasFileTypeEnum() => $_has(20);
  @$pb.TagNumber(22)
  void clearFileTypeEnum() => $_clearField(22);

  @$pb.TagNumber(23)
  FileScanOutcome get scanResultEnum => $_getN(21);
  @$pb.TagNumber(23)
  set scanResultEnum(FileScanOutcome value) => $_setField(23, value);
  @$pb.TagNumber(23)
  $core.bool hasScanResultEnum() => $_has(21);
  @$pb.TagNumber(23)
  void clearScanResultEnum() => $_clearField(23);
}

/// @voice.unknown_fields=reject
class GetFileURLRequest extends $pb.GeneratedMessage {
  factory GetFileURLRequest({
    $core.String? fileId,
    FileAccessSelector? access,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
    if (access != null) result.access = access;
    return result;
  }

  GetFileURLRequest._();

  factory GetFileURLRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetFileURLRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetFileURLRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aOM<FileAccessSelector>(2, _omitFieldNames ? '' : 'access',
        subBuilder: FileAccessSelector.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFileURLRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFileURLRequest copyWith(void Function(GetFileURLRequest) updates) =>
      super.copyWith((message) => updates(message as GetFileURLRequest))
          as GetFileURLRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetFileURLRequest create() => GetFileURLRequest._();
  @$core.override
  GetFileURLRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetFileURLRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetFileURLRequest>(create);
  static GetFileURLRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  FileAccessSelector get access => $_getN(1);
  @$pb.TagNumber(2)
  set access(FileAccessSelector value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAccess() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccess() => $_clearField(2);
  @$pb.TagNumber(2)
  FileAccessSelector ensureAccess() => $_ensure(1);
}

/// @voice.unknown_fields=reject
class GetFileMetadataRequest extends $pb.GeneratedMessage {
  factory GetFileMetadataRequest({
    $core.String? fileId,
    FileAccessSelector? access,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
    if (access != null) result.access = access;
    return result;
  }

  GetFileMetadataRequest._();

  factory GetFileMetadataRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetFileMetadataRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetFileMetadataRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aOM<FileAccessSelector>(2, _omitFieldNames ? '' : 'access',
        subBuilder: FileAccessSelector.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFileMetadataRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFileMetadataRequest copyWith(
          void Function(GetFileMetadataRequest) updates) =>
      super.copyWith((message) => updates(message as GetFileMetadataRequest))
          as GetFileMetadataRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetFileMetadataRequest create() => GetFileMetadataRequest._();
  @$core.override
  GetFileMetadataRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetFileMetadataRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetFileMetadataRequest>(create);
  static GetFileMetadataRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  FileAccessSelector get access => $_getN(1);
  @$pb.TagNumber(2)
  set access(FileAccessSelector value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAccess() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccess() => $_clearField(2);
  @$pb.TagNumber(2)
  FileAccessSelector ensureAccess() => $_ensure(1);
}

/// @voice.unknown_fields=reject
class GetBulkMetadataRequest extends $pb.GeneratedMessage {
  factory GetBulkMetadataRequest({
    @$core.Deprecated('This field is deprecated.')
    $core.Iterable<$core.String>? fileIds,
    $core.Iterable<FileAccessItem>? items,
  }) {
    final result = create();
    if (fileIds != null) result.fileIds.addAll(fileIds);
    if (items != null) result.items.addAll(items);
    return result;
  }

  GetBulkMetadataRequest._();

  factory GetBulkMetadataRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBulkMetadataRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBulkMetadataRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'fileIds')
    ..pPM<FileAccessItem>(2, _omitFieldNames ? '' : 'items',
        subBuilder: FileAccessItem.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkMetadataRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkMetadataRequest copyWith(
          void Function(GetBulkMetadataRequest) updates) =>
      super.copyWith((message) => updates(message as GetBulkMetadataRequest))
          as GetBulkMetadataRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBulkMetadataRequest create() => GetBulkMetadataRequest._();
  @$core.override
  GetBulkMetadataRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBulkMetadataRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBulkMetadataRequest>(create);
  static GetBulkMetadataRequest? _defaultInstance;

  @$core.Deprecated('This field is deprecated.')
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get fileIds => $_getList(0);

  @$pb.TagNumber(2)
  $pb.PbList<FileAccessItem> get items => $_getList(1);
}

class BulkFileMetadata extends $pb.GeneratedMessage {
  factory BulkFileMetadata({
    $core.Iterable<$core.MapEntry<$core.String, FileMetadata>>? byFileId,
  }) {
    final result = create();
    if (byFileId != null) result.byFileId.addEntries(byFileId);
    return result;
  }

  BulkFileMetadata._();

  factory BulkFileMetadata.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BulkFileMetadata.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BulkFileMetadata',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..m<$core.String, FileMetadata>(1, _omitFieldNames ? '' : 'byFileId',
        entryClassName: 'BulkFileMetadata.ByFileIdEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: FileMetadata.create,
        valueDefaultOrMaker: FileMetadata.getDefault,
        packageName: const $pb.PackageName('voice.file.v1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BulkFileMetadata clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BulkFileMetadata copyWith(void Function(BulkFileMetadata) updates) =>
      super.copyWith((message) => updates(message as BulkFileMetadata))
          as BulkFileMetadata;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BulkFileMetadata create() => BulkFileMetadata._();
  @$core.override
  BulkFileMetadata createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BulkFileMetadata getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BulkFileMetadata>(create);
  static BulkFileMetadata? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbMap<$core.String, FileMetadata> get byFileId => $_getMap(0);
}

class DeleteFileRequest extends $pb.GeneratedMessage {
  factory DeleteFileRequest({
    $core.String? fileId,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
    return result;
  }

  DeleteFileRequest._();

  factory DeleteFileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteFileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteFileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteFileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteFileRequest copyWith(void Function(DeleteFileRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteFileRequest))
          as DeleteFileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteFileRequest create() => DeleteFileRequest._();
  @$core.override
  DeleteFileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteFileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteFileRequest>(create);
  static DeleteFileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);
}

class ListFilesRequest extends $pb.GeneratedMessage {
  factory ListFilesRequest({
    $1.ChatRef? filterChat,
    $4.CursorPageRequest? page,
  }) {
    final result = create();
    if (filterChat != null) result.filterChat = filterChat;
    if (page != null) result.page = page;
    return result;
  }

  ListFilesRequest._();

  factory ListFilesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFilesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFilesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'filterChat',
        subBuilder: $1.ChatRef.create)
    ..aOM<$4.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $4.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFilesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFilesRequest copyWith(void Function(ListFilesRequest) updates) =>
      super.copyWith((message) => updates(message as ListFilesRequest))
          as ListFilesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFilesRequest create() => ListFilesRequest._();
  @$core.override
  ListFilesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFilesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFilesRequest>(create);
  static ListFilesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get filterChat => $_getN(0);
  @$pb.TagNumber(1)
  set filterChat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFilterChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearFilterChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureFilterChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $4.CursorPageRequest get page => $_getN(1);
  @$pb.TagNumber(2)
  set page($4.CursorPageRequest value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPage() => $_has(1);
  @$pb.TagNumber(2)
  void clearPage() => $_clearField(2);
  @$pb.TagNumber(2)
  $4.CursorPageRequest ensurePage() => $_ensure(1);
}

class FileList extends $pb.GeneratedMessage {
  factory FileList({
    $core.Iterable<FileMetadata>? files,
    $core.String? nextCursor,
    $4.CursorPageResponse? page,
  }) {
    final result = create();
    if (files != null) result.files.addAll(files);
    if (nextCursor != null) result.nextCursor = nextCursor;
    if (page != null) result.page = page;
    return result;
  }

  FileList._();

  factory FileList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..pPM<FileMetadata>(1, _omitFieldNames ? '' : 'files',
        subBuilder: FileMetadata.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..aOM<$4.CursorPageResponse>(3, _omitFieldNames ? '' : 'page',
        subBuilder: $4.CursorPageResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileList copyWith(void Function(FileList) updates) =>
      super.copyWith((message) => updates(message as FileList)) as FileList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileList create() => FileList._();
  @$core.override
  FileList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<FileList>(create);
  static FileList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<FileMetadata> get files => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);

  @$pb.TagNumber(3)
  $4.CursorPageResponse get page => $_getN(2);
  @$pb.TagNumber(3)
  set page($4.CursorPageResponse value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPage() => $_has(2);
  @$pb.TagNumber(3)
  void clearPage() => $_clearField(3);
  @$pb.TagNumber(3)
  $4.CursorPageResponse ensurePage() => $_ensure(2);
}

class CheckQuotaRequest extends $pb.GeneratedMessage {
  factory CheckQuotaRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  CheckQuotaRequest._();

  factory CheckQuotaRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckQuotaRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckQuotaRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckQuotaRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckQuotaRequest copyWith(void Function(CheckQuotaRequest) updates) =>
      super.copyWith((message) => updates(message as CheckQuotaRequest))
          as CheckQuotaRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckQuotaRequest create() => CheckQuotaRequest._();
  @$core.override
  CheckQuotaRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckQuotaRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckQuotaRequest>(create);
  static CheckQuotaRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class QuotaResponse extends $pb.GeneratedMessage {
  factory QuotaResponse({
    $fixnum.Int64? bytesUsed,
    $fixnum.Int64? bytesLimit,
  }) {
    final result = create();
    if (bytesUsed != null) result.bytesUsed = bytesUsed;
    if (bytesLimit != null) result.bytesLimit = bytesLimit;
    return result;
  }

  QuotaResponse._();

  factory QuotaResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory QuotaResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'QuotaResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aInt64(1, _omitFieldNames ? '' : 'bytesUsed')
    ..aInt64(2, _omitFieldNames ? '' : 'bytesLimit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QuotaResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QuotaResponse copyWith(void Function(QuotaResponse) updates) =>
      super.copyWith((message) => updates(message as QuotaResponse))
          as QuotaResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static QuotaResponse create() => QuotaResponse._();
  @$core.override
  QuotaResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static QuotaResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<QuotaResponse>(create);
  static QuotaResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get bytesUsed => $_getI64(0);
  @$pb.TagNumber(1)
  set bytesUsed($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBytesUsed() => $_has(0);
  @$pb.TagNumber(1)
  void clearBytesUsed() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get bytesLimit => $_getI64(1);
  @$pb.TagNumber(2)
  set bytesLimit($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBytesLimit() => $_has(1);
  @$pb.TagNumber(2)
  void clearBytesLimit() => $_clearField(2);
}

class RequestUploadResponse extends $pb.GeneratedMessage {
  factory RequestUploadResponse({
    UploadResponse? uploadResponse,
  }) {
    final result = create();
    if (uploadResponse != null) result.uploadResponse = uploadResponse;
    return result;
  }

  RequestUploadResponse._();

  factory RequestUploadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RequestUploadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RequestUploadResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<UploadResponse>(1, _omitFieldNames ? '' : 'uploadResponse',
        subBuilder: UploadResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RequestUploadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RequestUploadResponse copyWith(
          void Function(RequestUploadResponse) updates) =>
      super.copyWith((message) => updates(message as RequestUploadResponse))
          as RequestUploadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RequestUploadResponse create() => RequestUploadResponse._();
  @$core.override
  RequestUploadResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RequestUploadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RequestUploadResponse>(create);
  static RequestUploadResponse? _defaultInstance;

  @$pb.TagNumber(1)
  UploadResponse get uploadResponse => $_getN(0);
  @$pb.TagNumber(1)
  set uploadResponse(UploadResponse value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasUploadResponse() => $_has(0);
  @$pb.TagNumber(1)
  void clearUploadResponse() => $_clearField(1);
  @$pb.TagNumber(1)
  UploadResponse ensureUploadResponse() => $_ensure(0);
}

class ConfirmUploadResponse extends $pb.GeneratedMessage {
  factory ConfirmUploadResponse({
    FileMetadata? fileMetadata,
  }) {
    final result = create();
    if (fileMetadata != null) result.fileMetadata = fileMetadata;
    return result;
  }

  ConfirmUploadResponse._();

  factory ConfirmUploadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ConfirmUploadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConfirmUploadResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<FileMetadata>(1, _omitFieldNames ? '' : 'fileMetadata',
        subBuilder: FileMetadata.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConfirmUploadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConfirmUploadResponse copyWith(
          void Function(ConfirmUploadResponse) updates) =>
      super.copyWith((message) => updates(message as ConfirmUploadResponse))
          as ConfirmUploadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ConfirmUploadResponse create() => ConfirmUploadResponse._();
  @$core.override
  ConfirmUploadResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ConfirmUploadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConfirmUploadResponse>(create);
  static ConfirmUploadResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FileMetadata get fileMetadata => $_getN(0);
  @$pb.TagNumber(1)
  set fileMetadata(FileMetadata value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFileMetadata() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileMetadata() => $_clearField(1);
  @$pb.TagNumber(1)
  FileMetadata ensureFileMetadata() => $_ensure(0);
}

class GetFileURLResponse extends $pb.GeneratedMessage {
  factory GetFileURLResponse({
    $core.String? presignedGetUrl,
    $3.Timestamp? expiresAt,
  }) {
    final result = create();
    if (presignedGetUrl != null) result.presignedGetUrl = presignedGetUrl;
    if (expiresAt != null) result.expiresAt = expiresAt;
    return result;
  }

  GetFileURLResponse._();

  factory GetFileURLResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetFileURLResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetFileURLResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'presignedGetUrl')
    ..aOM<$3.Timestamp>(2, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFileURLResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFileURLResponse copyWith(void Function(GetFileURLResponse) updates) =>
      super.copyWith((message) => updates(message as GetFileURLResponse))
          as GetFileURLResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetFileURLResponse create() => GetFileURLResponse._();
  @$core.override
  GetFileURLResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetFileURLResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetFileURLResponse>(create);
  static GetFileURLResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get presignedGetUrl => $_getSZ(0);
  @$pb.TagNumber(1)
  set presignedGetUrl($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPresignedGetUrl() => $_has(0);
  @$pb.TagNumber(1)
  void clearPresignedGetUrl() => $_clearField(1);

  @$pb.TagNumber(2)
  $3.Timestamp get expiresAt => $_getN(1);
  @$pb.TagNumber(2)
  set expiresAt($3.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasExpiresAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpiresAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.Timestamp ensureExpiresAt() => $_ensure(1);
}

class GetFileMetadataResponse extends $pb.GeneratedMessage {
  factory GetFileMetadataResponse({
    FileMetadata? fileMetadata,
  }) {
    final result = create();
    if (fileMetadata != null) result.fileMetadata = fileMetadata;
    return result;
  }

  GetFileMetadataResponse._();

  factory GetFileMetadataResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetFileMetadataResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetFileMetadataResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<FileMetadata>(1, _omitFieldNames ? '' : 'fileMetadata',
        subBuilder: FileMetadata.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFileMetadataResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFileMetadataResponse copyWith(
          void Function(GetFileMetadataResponse) updates) =>
      super.copyWith((message) => updates(message as GetFileMetadataResponse))
          as GetFileMetadataResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetFileMetadataResponse create() => GetFileMetadataResponse._();
  @$core.override
  GetFileMetadataResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetFileMetadataResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetFileMetadataResponse>(create);
  static GetFileMetadataResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FileMetadata get fileMetadata => $_getN(0);
  @$pb.TagNumber(1)
  set fileMetadata(FileMetadata value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFileMetadata() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileMetadata() => $_clearField(1);
  @$pb.TagNumber(1)
  FileMetadata ensureFileMetadata() => $_ensure(0);
}

class GetBulkMetadataResponse extends $pb.GeneratedMessage {
  factory GetBulkMetadataResponse({
    BulkFileMetadata? bulkFileMetadata,
  }) {
    final result = create();
    if (bulkFileMetadata != null) result.bulkFileMetadata = bulkFileMetadata;
    return result;
  }

  GetBulkMetadataResponse._();

  factory GetBulkMetadataResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBulkMetadataResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBulkMetadataResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<BulkFileMetadata>(1, _omitFieldNames ? '' : 'bulkFileMetadata',
        subBuilder: BulkFileMetadata.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkMetadataResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkMetadataResponse copyWith(
          void Function(GetBulkMetadataResponse) updates) =>
      super.copyWith((message) => updates(message as GetBulkMetadataResponse))
          as GetBulkMetadataResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBulkMetadataResponse create() => GetBulkMetadataResponse._();
  @$core.override
  GetBulkMetadataResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBulkMetadataResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBulkMetadataResponse>(create);
  static GetBulkMetadataResponse? _defaultInstance;

  @$pb.TagNumber(1)
  BulkFileMetadata get bulkFileMetadata => $_getN(0);
  @$pb.TagNumber(1)
  set bulkFileMetadata(BulkFileMetadata value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBulkFileMetadata() => $_has(0);
  @$pb.TagNumber(1)
  void clearBulkFileMetadata() => $_clearField(1);
  @$pb.TagNumber(1)
  BulkFileMetadata ensureBulkFileMetadata() => $_ensure(0);
}

class DeleteFileResponse extends $pb.GeneratedMessage {
  factory DeleteFileResponse() => create();

  DeleteFileResponse._();

  factory DeleteFileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteFileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteFileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteFileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteFileResponse copyWith(void Function(DeleteFileResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteFileResponse))
          as DeleteFileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteFileResponse create() => DeleteFileResponse._();
  @$core.override
  DeleteFileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteFileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteFileResponse>(create);
  static DeleteFileResponse? _defaultInstance;
}

class ListFilesResponse extends $pb.GeneratedMessage {
  factory ListFilesResponse({
    FileList? fileList,
  }) {
    final result = create();
    if (fileList != null) result.fileList = fileList;
    return result;
  }

  ListFilesResponse._();

  factory ListFilesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFilesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFilesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<FileList>(1, _omitFieldNames ? '' : 'fileList',
        subBuilder: FileList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFilesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFilesResponse copyWith(void Function(ListFilesResponse) updates) =>
      super.copyWith((message) => updates(message as ListFilesResponse))
          as ListFilesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFilesResponse create() => ListFilesResponse._();
  @$core.override
  ListFilesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFilesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFilesResponse>(create);
  static ListFilesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FileList get fileList => $_getN(0);
  @$pb.TagNumber(1)
  set fileList(FileList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFileList() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileList() => $_clearField(1);
  @$pb.TagNumber(1)
  FileList ensureFileList() => $_ensure(0);
}

class CheckQuotaResponse extends $pb.GeneratedMessage {
  factory CheckQuotaResponse({
    QuotaResponse? quotaResponse,
  }) {
    final result = create();
    if (quotaResponse != null) result.quotaResponse = quotaResponse;
    return result;
  }

  CheckQuotaResponse._();

  factory CheckQuotaResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckQuotaResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckQuotaResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<QuotaResponse>(1, _omitFieldNames ? '' : 'quotaResponse',
        subBuilder: QuotaResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckQuotaResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckQuotaResponse copyWith(void Function(CheckQuotaResponse) updates) =>
      super.copyWith((message) => updates(message as CheckQuotaResponse))
          as CheckQuotaResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckQuotaResponse create() => CheckQuotaResponse._();
  @$core.override
  CheckQuotaResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckQuotaResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckQuotaResponse>(create);
  static CheckQuotaResponse? _defaultInstance;

  @$pb.TagNumber(1)
  QuotaResponse get quotaResponse => $_getN(0);
  @$pb.TagNumber(1)
  set quotaResponse(QuotaResponse value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasQuotaResponse() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuotaResponse() => $_clearField(1);
  @$pb.TagNumber(1)
  QuotaResponse ensureQuotaResponse() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class FileReferenceProducerDeclaration extends $pb.GeneratedMessage {
  factory FileReferenceProducerDeclaration({
    FileReferenceProducerId? producerId,
    $fixnum.Int64? expectedTotalCount,
    $core.List<$core.int>? expectedReferencesSha256,
  }) {
    final result = create();
    if (producerId != null) result.producerId = producerId;
    if (expectedTotalCount != null)
      result.expectedTotalCount = expectedTotalCount;
    if (expectedReferencesSha256 != null)
      result.expectedReferencesSha256 = expectedReferencesSha256;
    return result;
  }

  FileReferenceProducerDeclaration._();

  factory FileReferenceProducerDeclaration.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileReferenceProducerDeclaration.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileReferenceProducerDeclaration',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aE<FileReferenceProducerId>(1, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'expectedTotalCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(3,
        _omitFieldNames ? '' : 'expectedReferencesSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileReferenceProducerDeclaration clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileReferenceProducerDeclaration copyWith(
          void Function(FileReferenceProducerDeclaration) updates) =>
      super.copyWith(
              (message) => updates(message as FileReferenceProducerDeclaration))
          as FileReferenceProducerDeclaration;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileReferenceProducerDeclaration create() =>
      FileReferenceProducerDeclaration._();
  @$core.override
  FileReferenceProducerDeclaration createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileReferenceProducerDeclaration getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileReferenceProducerDeclaration>(
          create);
  static FileReferenceProducerDeclaration? _defaultInstance;

  @$pb.TagNumber(1)
  FileReferenceProducerId get producerId => $_getN(0);
  @$pb.TagNumber(1)
  set producerId(FileReferenceProducerId value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProducerId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProducerId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get expectedTotalCount => $_getI64(1);
  @$pb.TagNumber(2)
  set expectedTotalCount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExpectedTotalCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpectedTotalCount() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get expectedReferencesSha256 => $_getN(2);
  @$pb.TagNumber(3)
  set expectedReferencesSha256($core.List<$core.int> value) =>
      $_setBytes(2, value);
  @$pb.TagNumber(3)
  $core.bool hasExpectedReferencesSha256() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpectedReferencesSha256() => $_clearField(3);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class FileReferenceKey extends $pb.GeneratedMessage {
  factory FileReferenceKey({
    $core.String? fileId,
    FileReferenceOwnerType? ownerType,
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

  FileReferenceKey._();

  factory FileReferenceKey.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileReferenceKey.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileReferenceKey',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aE<FileReferenceOwnerType>(2, _omitFieldNames ? '' : 'ownerType',
        enumValues: FileReferenceOwnerType.values)
    ..aOS(3, _omitFieldNames ? '' : 'ownerId')
    ..aOS(4, _omitFieldNames ? '' : 'subresourceId')
    ..aOS(5, _omitFieldNames ? '' : 'scopeSpaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileReferenceKey clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileReferenceKey copyWith(void Function(FileReferenceKey) updates) =>
      super.copyWith((message) => updates(message as FileReferenceKey))
          as FileReferenceKey;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileReferenceKey create() => FileReferenceKey._();
  @$core.override
  FileReferenceKey createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileReferenceKey getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileReferenceKey>(create);
  static FileReferenceKey? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  FileReferenceOwnerType get ownerType => $_getN(1);
  @$pb.TagNumber(2)
  set ownerType(FileReferenceOwnerType value) => $_setField(2, value);
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

enum FileAccessSelector_Selector { reference, capabilityId, notSet }

/// @voice.unknown_fields=reject
class FileAccessSelector extends $pb.GeneratedMessage {
  factory FileAccessSelector({
    FileReferenceKey? reference,
    $core.String? capabilityId,
  }) {
    final result = create();
    if (reference != null) result.reference = reference;
    if (capabilityId != null) result.capabilityId = capabilityId;
    return result;
  }

  FileAccessSelector._();

  factory FileAccessSelector.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileAccessSelector.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, FileAccessSelector_Selector>
      _FileAccessSelector_SelectorByTag = {
    1: FileAccessSelector_Selector.reference,
    2: FileAccessSelector_Selector.capabilityId,
    0: FileAccessSelector_Selector.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileAccessSelector',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..oo(0, [1, 2])
    ..aOM<FileReferenceKey>(1, _omitFieldNames ? '' : 'reference',
        subBuilder: FileReferenceKey.create)
    ..aOS(2, _omitFieldNames ? '' : 'capabilityId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileAccessSelector clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileAccessSelector copyWith(void Function(FileAccessSelector) updates) =>
      super.copyWith((message) => updates(message as FileAccessSelector))
          as FileAccessSelector;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileAccessSelector create() => FileAccessSelector._();
  @$core.override
  FileAccessSelector createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileAccessSelector getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileAccessSelector>(create);
  static FileAccessSelector? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  FileAccessSelector_Selector whichSelector() =>
      _FileAccessSelector_SelectorByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  void clearSelector() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  FileReferenceKey get reference => $_getN(0);
  @$pb.TagNumber(1)
  set reference(FileReferenceKey value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReference() => $_has(0);
  @$pb.TagNumber(1)
  void clearReference() => $_clearField(1);
  @$pb.TagNumber(1)
  FileReferenceKey ensureReference() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get capabilityId => $_getSZ(1);
  @$pb.TagNumber(2)
  set capabilityId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCapabilityId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCapabilityId() => $_clearField(2);
}

/// @voice.unknown_fields=reject
class FileAccessItem extends $pb.GeneratedMessage {
  factory FileAccessItem({
    $core.String? fileId,
    FileAccessSelector? access,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
    if (access != null) result.access = access;
    return result;
  }

  FileAccessItem._();

  factory FileAccessItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileAccessItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileAccessItem',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aOM<FileAccessSelector>(2, _omitFieldNames ? '' : 'access',
        subBuilder: FileAccessSelector.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileAccessItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileAccessItem copyWith(void Function(FileAccessItem) updates) =>
      super.copyWith((message) => updates(message as FileAccessItem))
          as FileAccessItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileAccessItem create() => FileAccessItem._();
  @$core.override
  FileAccessItem createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileAccessItem getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileAccessItem>(create);
  static FileAccessItem? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  FileAccessSelector get access => $_getN(1);
  @$pb.TagNumber(2)
  set access(FileAccessSelector value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAccess() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccess() => $_clearField(2);
  @$pb.TagNumber(2)
  FileAccessSelector ensureAccess() => $_ensure(1);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class IssueFileAccessCapabilityRequest extends $pb.GeneratedMessage {
  factory IssueFileAccessCapabilityRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    FileReferenceKey? reference,
    $core.String? subjectProfileId,
    $core.Iterable<FileReadSurface>? allowedSurfaces,
    $3.Timestamp? expiresAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (reference != null) result.reference = reference;
    if (subjectProfileId != null) result.subjectProfileId = subjectProfileId;
    if (allowedSurfaces != null) result.allowedSurfaces.addAll(allowedSurfaces);
    if (expiresAt != null) result.expiresAt = expiresAt;
    return result;
  }

  IssueFileAccessCapabilityRequest._();

  factory IssueFileAccessCapabilityRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IssueFileAccessCapabilityRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IssueFileAccessCapabilityRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOM<FileReferenceKey>(3, _omitFieldNames ? '' : 'reference',
        subBuilder: FileReferenceKey.create)
    ..aOS(4, _omitFieldNames ? '' : 'subjectProfileId')
    ..pc<FileReadSurface>(
        5, _omitFieldNames ? '' : 'allowedSurfaces', $pb.PbFieldType.KE,
        valueOf: FileReadSurface.valueOf,
        enumValues: FileReadSurface.values,
        defaultEnumValue: FileReadSurface.FILE_READ_SURFACE_UNSPECIFIED)
    ..aOM<$3.Timestamp>(6, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueFileAccessCapabilityRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueFileAccessCapabilityRequest copyWith(
          void Function(IssueFileAccessCapabilityRequest) updates) =>
      super.copyWith(
              (message) => updates(message as IssueFileAccessCapabilityRequest))
          as IssueFileAccessCapabilityRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IssueFileAccessCapabilityRequest create() =>
      IssueFileAccessCapabilityRequest._();
  @$core.override
  IssueFileAccessCapabilityRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IssueFileAccessCapabilityRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IssueFileAccessCapabilityRequest>(
          create);
  static IssueFileAccessCapabilityRequest? _defaultInstance;

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
  FileReferenceKey get reference => $_getN(2);
  @$pb.TagNumber(3)
  set reference(FileReferenceKey value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasReference() => $_has(2);
  @$pb.TagNumber(3)
  void clearReference() => $_clearField(3);
  @$pb.TagNumber(3)
  FileReferenceKey ensureReference() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get subjectProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set subjectProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSubjectProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSubjectProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<FileReadSurface> get allowedSurfaces => $_getList(4);

  @$pb.TagNumber(6)
  $3.Timestamp get expiresAt => $_getN(5);
  @$pb.TagNumber(6)
  set expiresAt($3.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasExpiresAt() => $_has(5);
  @$pb.TagNumber(6)
  void clearExpiresAt() => $_clearField(6);
  @$pb.TagNumber(6)
  $3.Timestamp ensureExpiresAt() => $_ensure(5);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class IssueFileAccessCapabilityResponse extends $pb.GeneratedMessage {
  factory IssueFileAccessCapabilityResponse({
    IssueFileAccessCapabilityReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  IssueFileAccessCapabilityResponse._();

  factory IssueFileAccessCapabilityResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IssueFileAccessCapabilityResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IssueFileAccessCapabilityResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<IssueFileAccessCapabilityReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: IssueFileAccessCapabilityReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueFileAccessCapabilityResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueFileAccessCapabilityResponse copyWith(
          void Function(IssueFileAccessCapabilityResponse) updates) =>
      super.copyWith((message) =>
              updates(message as IssueFileAccessCapabilityResponse))
          as IssueFileAccessCapabilityResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IssueFileAccessCapabilityResponse create() =>
      IssueFileAccessCapabilityResponse._();
  @$core.override
  IssueFileAccessCapabilityResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IssueFileAccessCapabilityResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IssueFileAccessCapabilityResponse>(
          create);
  static IssueFileAccessCapabilityResponse? _defaultInstance;

  @$pb.TagNumber(1)
  IssueFileAccessCapabilityReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(IssueFileAccessCapabilityReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  IssueFileAccessCapabilityReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class IssueFileAccessCapabilityReceipt extends $pb.GeneratedMessage {
  factory IssueFileAccessCapabilityReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? operationId,
    $core.String? capabilityId,
    FileReferenceKey? reference,
    $core.String? subjectProfileId,
    $core.Iterable<FileReadSurface>? allowedSurfaces,
    $3.Timestamp? expiresAt,
    $core.List<$core.int>? requestSha256,
    $3.Timestamp? completedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (operationId != null) result.operationId = operationId;
    if (capabilityId != null) result.capabilityId = capabilityId;
    if (reference != null) result.reference = reference;
    if (subjectProfileId != null) result.subjectProfileId = subjectProfileId;
    if (allowedSurfaces != null) result.allowedSurfaces.addAll(allowedSurfaces);
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  IssueFileAccessCapabilityReceipt._();

  factory IssueFileAccessCapabilityReceipt.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IssueFileAccessCapabilityReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IssueFileAccessCapabilityReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..aOS(4, _omitFieldNames ? '' : 'capabilityId')
    ..aOM<FileReferenceKey>(5, _omitFieldNames ? '' : 'reference',
        subBuilder: FileReferenceKey.create)
    ..aOS(6, _omitFieldNames ? '' : 'subjectProfileId')
    ..pc<FileReadSurface>(
        7, _omitFieldNames ? '' : 'allowedSurfaces', $pb.PbFieldType.KE,
        valueOf: FileReadSurface.valueOf,
        enumValues: FileReadSurface.values,
        defaultEnumValue: FileReadSurface.FILE_READ_SURFACE_UNSPECIFIED)
    ..aOM<$3.Timestamp>(8, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $3.Timestamp.create)
    ..a<$core.List<$core.int>>(
        9, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$3.Timestamp>(10, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueFileAccessCapabilityReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IssueFileAccessCapabilityReceipt copyWith(
          void Function(IssueFileAccessCapabilityReceipt) updates) =>
      super.copyWith(
              (message) => updates(message as IssueFileAccessCapabilityReceipt))
          as IssueFileAccessCapabilityReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IssueFileAccessCapabilityReceipt create() =>
      IssueFileAccessCapabilityReceipt._();
  @$core.override
  IssueFileAccessCapabilityReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IssueFileAccessCapabilityReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IssueFileAccessCapabilityReceipt>(
          create);
  static IssueFileAccessCapabilityReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get capabilityId => $_getSZ(3);
  @$pb.TagNumber(4)
  set capabilityId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCapabilityId() => $_has(3);
  @$pb.TagNumber(4)
  void clearCapabilityId() => $_clearField(4);

  @$pb.TagNumber(5)
  FileReferenceKey get reference => $_getN(4);
  @$pb.TagNumber(5)
  set reference(FileReferenceKey value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasReference() => $_has(4);
  @$pb.TagNumber(5)
  void clearReference() => $_clearField(5);
  @$pb.TagNumber(5)
  FileReferenceKey ensureReference() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get subjectProfileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set subjectProfileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSubjectProfileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearSubjectProfileId() => $_clearField(6);

  @$pb.TagNumber(7)
  $pb.PbList<FileReadSurface> get allowedSurfaces => $_getList(6);

  @$pb.TagNumber(8)
  $3.Timestamp get expiresAt => $_getN(7);
  @$pb.TagNumber(8)
  set expiresAt($3.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasExpiresAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearExpiresAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $3.Timestamp ensureExpiresAt() => $_ensure(7);

  @$pb.TagNumber(9)
  $core.List<$core.int> get requestSha256 => $_getN(8);
  @$pb.TagNumber(9)
  set requestSha256($core.List<$core.int> value) => $_setBytes(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRequestSha256() => $_has(8);
  @$pb.TagNumber(9)
  void clearRequestSha256() => $_clearField(9);

  @$pb.TagNumber(10)
  $3.Timestamp get completedAt => $_getN(9);
  @$pb.TagNumber(10)
  set completedAt($3.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCompletedAt() => $_has(9);
  @$pb.TagNumber(10)
  void clearCompletedAt() => $_clearField(10);
  @$pb.TagNumber(10)
  $3.Timestamp ensureCompletedAt() => $_ensure(9);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class PrepareSpaceDeletionReferenceManifestRequest
    extends $pb.GeneratedMessage {
  factory PrepareSpaceDeletionReferenceManifestRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? scheduleGeneration,
    $5.ManifestBinding? chatManifest,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (scheduleGeneration != null)
      result.scheduleGeneration = scheduleGeneration;
    if (chatManifest != null) result.chatManifest = chatManifest;
    return result;
  }

  PrepareSpaceDeletionReferenceManifestRequest._();

  factory PrepareSpaceDeletionReferenceManifestRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PrepareSpaceDeletionReferenceManifestRequest.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PrepareSpaceDeletionReferenceManifestRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'scheduleGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$5.ManifestBinding>(5, _omitFieldNames ? '' : 'chatManifest',
        subBuilder: $5.ManifestBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareSpaceDeletionReferenceManifestRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareSpaceDeletionReferenceManifestRequest copyWith(
          void Function(PrepareSpaceDeletionReferenceManifestRequest)
              updates) =>
      super.copyWith((message) =>
              updates(message as PrepareSpaceDeletionReferenceManifestRequest))
          as PrepareSpaceDeletionReferenceManifestRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PrepareSpaceDeletionReferenceManifestRequest create() =>
      PrepareSpaceDeletionReferenceManifestRequest._();
  @$core.override
  PrepareSpaceDeletionReferenceManifestRequest createEmptyInstance() =>
      create();
  @$core.pragma('dart2js:noInline')
  static PrepareSpaceDeletionReferenceManifestRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          PrepareSpaceDeletionReferenceManifestRequest>(create);
  static PrepareSpaceDeletionReferenceManifestRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get deletionOperationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set deletionOperationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeletionOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeletionOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get scheduleGeneration => $_getI64(3);
  @$pb.TagNumber(4)
  set scheduleGeneration($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasScheduleGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearScheduleGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  $5.ManifestBinding get chatManifest => $_getN(4);
  @$pb.TagNumber(5)
  set chatManifest($5.ManifestBinding value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasChatManifest() => $_has(4);
  @$pb.TagNumber(5)
  void clearChatManifest() => $_clearField(5);
  @$pb.TagNumber(5)
  $5.ManifestBinding ensureChatManifest() => $_ensure(4);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class PrepareSpaceDeletionReferenceManifestReceipt
    extends $pb.GeneratedMessage {
  factory PrepareSpaceDeletionReferenceManifestReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? scheduleGeneration,
    $5.LifecycleFenceState? appliedState,
    $core.List<$core.int>? requestSha256,
    $3.Timestamp? appliedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (scheduleGeneration != null)
      result.scheduleGeneration = scheduleGeneration;
    if (appliedState != null) result.appliedState = appliedState;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (appliedAt != null) result.appliedAt = appliedAt;
    return result;
  }

  PrepareSpaceDeletionReferenceManifestReceipt._();

  factory PrepareSpaceDeletionReferenceManifestReceipt.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PrepareSpaceDeletionReferenceManifestReceipt.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PrepareSpaceDeletionReferenceManifestReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..aOS(4, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'scheduleGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<$5.LifecycleFenceState>(6, _omitFieldNames ? '' : 'appliedState',
        enumValues: $5.LifecycleFenceState.values)
    ..a<$core.List<$core.int>>(
        7, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$3.Timestamp>(8, _omitFieldNames ? '' : 'appliedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareSpaceDeletionReferenceManifestReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareSpaceDeletionReferenceManifestReceipt copyWith(
          void Function(PrepareSpaceDeletionReferenceManifestReceipt)
              updates) =>
      super.copyWith((message) =>
              updates(message as PrepareSpaceDeletionReferenceManifestReceipt))
          as PrepareSpaceDeletionReferenceManifestReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PrepareSpaceDeletionReferenceManifestReceipt create() =>
      PrepareSpaceDeletionReferenceManifestReceipt._();
  @$core.override
  PrepareSpaceDeletionReferenceManifestReceipt createEmptyInstance() =>
      create();
  @$core.pragma('dart2js:noInline')
  static PrepareSpaceDeletionReferenceManifestReceipt getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          PrepareSpaceDeletionReferenceManifestReceipt>(create);
  static PrepareSpaceDeletionReferenceManifestReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deletionOperationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set deletionOperationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeletionOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeletionOperationId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get scheduleGeneration => $_getI64(4);
  @$pb.TagNumber(5)
  set scheduleGeneration($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasScheduleGeneration() => $_has(4);
  @$pb.TagNumber(5)
  void clearScheduleGeneration() => $_clearField(5);

  @$pb.TagNumber(6)
  $5.LifecycleFenceState get appliedState => $_getN(5);
  @$pb.TagNumber(6)
  set appliedState($5.LifecycleFenceState value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasAppliedState() => $_has(5);
  @$pb.TagNumber(6)
  void clearAppliedState() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get requestSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set requestSha256($core.List<$core.int> value) => $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRequestSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearRequestSha256() => $_clearField(7);

  @$pb.TagNumber(8)
  $3.Timestamp get appliedAt => $_getN(7);
  @$pb.TagNumber(8)
  set appliedAt($3.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasAppliedAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearAppliedAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $3.Timestamp ensureAppliedAt() => $_ensure(7);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class PrepareSpaceDeletionReferenceManifestResponse
    extends $pb.GeneratedMessage {
  factory PrepareSpaceDeletionReferenceManifestResponse({
    PrepareSpaceDeletionReferenceManifestReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  PrepareSpaceDeletionReferenceManifestResponse._();

  factory PrepareSpaceDeletionReferenceManifestResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PrepareSpaceDeletionReferenceManifestResponse.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PrepareSpaceDeletionReferenceManifestResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<PrepareSpaceDeletionReferenceManifestReceipt>(
        1, _omitFieldNames ? '' : 'receipt',
        subBuilder: PrepareSpaceDeletionReferenceManifestReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareSpaceDeletionReferenceManifestResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareSpaceDeletionReferenceManifestResponse copyWith(
          void Function(PrepareSpaceDeletionReferenceManifestResponse)
              updates) =>
      super.copyWith((message) =>
              updates(message as PrepareSpaceDeletionReferenceManifestResponse))
          as PrepareSpaceDeletionReferenceManifestResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PrepareSpaceDeletionReferenceManifestResponse create() =>
      PrepareSpaceDeletionReferenceManifestResponse._();
  @$core.override
  PrepareSpaceDeletionReferenceManifestResponse createEmptyInstance() =>
      create();
  @$core.pragma('dart2js:noInline')
  static PrepareSpaceDeletionReferenceManifestResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          PrepareSpaceDeletionReferenceManifestResponse>(create);
  static PrepareSpaceDeletionReferenceManifestResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PrepareSpaceDeletionReferenceManifestReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(PrepareSpaceDeletionReferenceManifestReceipt value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  PrepareSpaceDeletionReferenceManifestReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class RegisterSpaceDeletionReferenceChunkRequest extends $pb.GeneratedMessage {
  factory RegisterSpaceDeletionReferenceChunkRequest({
    $core.int? protocolVersion,
    FileReferenceProducerId? producerId,
    $core.String? operationId,
    $core.String? deletionOperationId,
    $core.String? spaceId,
    $fixnum.Int64? scheduleGeneration,
    $fixnum.Int64? chunkIndex,
    $core.Iterable<FileReferenceKey>? references,
    $fixnum.Int64? expectedTotalCount,
    $core.List<$core.int>? expectedReferencesSha256,
    $core.bool? sealsProducer,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (producerId != null) result.producerId = producerId;
    if (operationId != null) result.operationId = operationId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (spaceId != null) result.spaceId = spaceId;
    if (scheduleGeneration != null)
      result.scheduleGeneration = scheduleGeneration;
    if (chunkIndex != null) result.chunkIndex = chunkIndex;
    if (references != null) result.references.addAll(references);
    if (expectedTotalCount != null)
      result.expectedTotalCount = expectedTotalCount;
    if (expectedReferencesSha256 != null)
      result.expectedReferencesSha256 = expectedReferencesSha256;
    if (sealsProducer != null) result.sealsProducer = sealsProducer;
    return result;
  }

  RegisterSpaceDeletionReferenceChunkRequest._();

  factory RegisterSpaceDeletionReferenceChunkRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterSpaceDeletionReferenceChunkRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterSpaceDeletionReferenceChunkRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aE<FileReferenceProducerId>(2, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..aOS(4, _omitFieldNames ? '' : 'deletionOperationId')
    ..aOS(5, _omitFieldNames ? '' : 'spaceId')
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'scheduleGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'chunkIndex', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..pPM<FileReferenceKey>(8, _omitFieldNames ? '' : 'references',
        subBuilder: FileReferenceKey.create)
    ..a<$fixnum.Int64>(
        9, _omitFieldNames ? '' : 'expectedTotalCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(10,
        _omitFieldNames ? '' : 'expectedReferencesSha256', $pb.PbFieldType.OY)
    ..aOB(11, _omitFieldNames ? '' : 'sealsProducer')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterSpaceDeletionReferenceChunkRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterSpaceDeletionReferenceChunkRequest copyWith(
          void Function(RegisterSpaceDeletionReferenceChunkRequest) updates) =>
      super.copyWith((message) =>
              updates(message as RegisterSpaceDeletionReferenceChunkRequest))
          as RegisterSpaceDeletionReferenceChunkRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterSpaceDeletionReferenceChunkRequest create() =>
      RegisterSpaceDeletionReferenceChunkRequest._();
  @$core.override
  RegisterSpaceDeletionReferenceChunkRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterSpaceDeletionReferenceChunkRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          RegisterSpaceDeletionReferenceChunkRequest>(create);
  static RegisterSpaceDeletionReferenceChunkRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  FileReferenceProducerId get producerId => $_getN(1);
  @$pb.TagNumber(2)
  set producerId(FileReferenceProducerId value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasProducerId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProducerId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deletionOperationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set deletionOperationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeletionOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeletionOperationId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get spaceId => $_getSZ(4);
  @$pb.TagNumber(5)
  set spaceId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSpaceId() => $_has(4);
  @$pb.TagNumber(5)
  void clearSpaceId() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get scheduleGeneration => $_getI64(5);
  @$pb.TagNumber(6)
  set scheduleGeneration($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasScheduleGeneration() => $_has(5);
  @$pb.TagNumber(6)
  void clearScheduleGeneration() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get chunkIndex => $_getI64(6);
  @$pb.TagNumber(7)
  set chunkIndex($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasChunkIndex() => $_has(6);
  @$pb.TagNumber(7)
  void clearChunkIndex() => $_clearField(7);

  @$pb.TagNumber(8)
  $pb.PbList<FileReferenceKey> get references => $_getList(7);

  @$pb.TagNumber(9)
  $fixnum.Int64 get expectedTotalCount => $_getI64(8);
  @$pb.TagNumber(9)
  set expectedTotalCount($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasExpectedTotalCount() => $_has(8);
  @$pb.TagNumber(9)
  void clearExpectedTotalCount() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.List<$core.int> get expectedReferencesSha256 => $_getN(9);
  @$pb.TagNumber(10)
  set expectedReferencesSha256($core.List<$core.int> value) =>
      $_setBytes(9, value);
  @$pb.TagNumber(10)
  $core.bool hasExpectedReferencesSha256() => $_has(9);
  @$pb.TagNumber(10)
  void clearExpectedReferencesSha256() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.bool get sealsProducer => $_getBF(10);
  @$pb.TagNumber(11)
  set sealsProducer($core.bool value) => $_setBool(10, value);
  @$pb.TagNumber(11)
  $core.bool hasSealsProducer() => $_has(10);
  @$pb.TagNumber(11)
  void clearSealsProducer() => $_clearField(11);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class RegisterSpaceDeletionReferenceChunkReceipt extends $pb.GeneratedMessage {
  factory RegisterSpaceDeletionReferenceChunkReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? operationId,
    $core.String? deletionOperationId,
    $core.String? spaceId,
    $fixnum.Int64? scheduleGeneration,
    $fixnum.Int64? chunkIndex,
    $fixnum.Int64? acceptedCount,
    FileReferenceProducerId? producerId,
    $fixnum.Int64? expectedTotalCount,
    $core.List<$core.int>? expectedReferencesSha256,
    $core.bool? producerSealed,
    $core.List<$core.int>? requestSha256,
    $3.Timestamp? completedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (operationId != null) result.operationId = operationId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (spaceId != null) result.spaceId = spaceId;
    if (scheduleGeneration != null)
      result.scheduleGeneration = scheduleGeneration;
    if (chunkIndex != null) result.chunkIndex = chunkIndex;
    if (acceptedCount != null) result.acceptedCount = acceptedCount;
    if (producerId != null) result.producerId = producerId;
    if (expectedTotalCount != null)
      result.expectedTotalCount = expectedTotalCount;
    if (expectedReferencesSha256 != null)
      result.expectedReferencesSha256 = expectedReferencesSha256;
    if (producerSealed != null) result.producerSealed = producerSealed;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  RegisterSpaceDeletionReferenceChunkReceipt._();

  factory RegisterSpaceDeletionReferenceChunkReceipt.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterSpaceDeletionReferenceChunkReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterSpaceDeletionReferenceChunkReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..aOS(4, _omitFieldNames ? '' : 'deletionOperationId')
    ..aOS(5, _omitFieldNames ? '' : 'spaceId')
    ..a<$fixnum.Int64>(
        6, _omitFieldNames ? '' : 'scheduleGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'chunkIndex', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        8, _omitFieldNames ? '' : 'acceptedCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<FileReferenceProducerId>(9, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..a<$fixnum.Int64>(
        10, _omitFieldNames ? '' : 'expectedTotalCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(11,
        _omitFieldNames ? '' : 'expectedReferencesSha256', $pb.PbFieldType.OY)
    ..aOB(12, _omitFieldNames ? '' : 'producerSealed')
    ..a<$core.List<$core.int>>(
        13, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$3.Timestamp>(14, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterSpaceDeletionReferenceChunkReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterSpaceDeletionReferenceChunkReceipt copyWith(
          void Function(RegisterSpaceDeletionReferenceChunkReceipt) updates) =>
      super.copyWith((message) =>
              updates(message as RegisterSpaceDeletionReferenceChunkReceipt))
          as RegisterSpaceDeletionReferenceChunkReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterSpaceDeletionReferenceChunkReceipt create() =>
      RegisterSpaceDeletionReferenceChunkReceipt._();
  @$core.override
  RegisterSpaceDeletionReferenceChunkReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterSpaceDeletionReferenceChunkReceipt getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          RegisterSpaceDeletionReferenceChunkReceipt>(create);
  static RegisterSpaceDeletionReferenceChunkReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deletionOperationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set deletionOperationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeletionOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeletionOperationId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get spaceId => $_getSZ(4);
  @$pb.TagNumber(5)
  set spaceId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSpaceId() => $_has(4);
  @$pb.TagNumber(5)
  void clearSpaceId() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get scheduleGeneration => $_getI64(5);
  @$pb.TagNumber(6)
  set scheduleGeneration($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasScheduleGeneration() => $_has(5);
  @$pb.TagNumber(6)
  void clearScheduleGeneration() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get chunkIndex => $_getI64(6);
  @$pb.TagNumber(7)
  set chunkIndex($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasChunkIndex() => $_has(6);
  @$pb.TagNumber(7)
  void clearChunkIndex() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get acceptedCount => $_getI64(7);
  @$pb.TagNumber(8)
  set acceptedCount($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAcceptedCount() => $_has(7);
  @$pb.TagNumber(8)
  void clearAcceptedCount() => $_clearField(8);

  @$pb.TagNumber(9)
  FileReferenceProducerId get producerId => $_getN(8);
  @$pb.TagNumber(9)
  set producerId(FileReferenceProducerId value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasProducerId() => $_has(8);
  @$pb.TagNumber(9)
  void clearProducerId() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get expectedTotalCount => $_getI64(9);
  @$pb.TagNumber(10)
  set expectedTotalCount($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasExpectedTotalCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearExpectedTotalCount() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.List<$core.int> get expectedReferencesSha256 => $_getN(10);
  @$pb.TagNumber(11)
  set expectedReferencesSha256($core.List<$core.int> value) =>
      $_setBytes(10, value);
  @$pb.TagNumber(11)
  $core.bool hasExpectedReferencesSha256() => $_has(10);
  @$pb.TagNumber(11)
  void clearExpectedReferencesSha256() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.bool get producerSealed => $_getBF(11);
  @$pb.TagNumber(12)
  set producerSealed($core.bool value) => $_setBool(11, value);
  @$pb.TagNumber(12)
  $core.bool hasProducerSealed() => $_has(11);
  @$pb.TagNumber(12)
  void clearProducerSealed() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.List<$core.int> get requestSha256 => $_getN(12);
  @$pb.TagNumber(13)
  set requestSha256($core.List<$core.int> value) => $_setBytes(12, value);
  @$pb.TagNumber(13)
  $core.bool hasRequestSha256() => $_has(12);
  @$pb.TagNumber(13)
  void clearRequestSha256() => $_clearField(13);

  @$pb.TagNumber(14)
  $3.Timestamp get completedAt => $_getN(13);
  @$pb.TagNumber(14)
  set completedAt($3.Timestamp value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasCompletedAt() => $_has(13);
  @$pb.TagNumber(14)
  void clearCompletedAt() => $_clearField(14);
  @$pb.TagNumber(14)
  $3.Timestamp ensureCompletedAt() => $_ensure(13);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class RegisterSpaceDeletionReferenceChunkResponse extends $pb.GeneratedMessage {
  factory RegisterSpaceDeletionReferenceChunkResponse({
    RegisterSpaceDeletionReferenceChunkReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  RegisterSpaceDeletionReferenceChunkResponse._();

  factory RegisterSpaceDeletionReferenceChunkResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterSpaceDeletionReferenceChunkResponse.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterSpaceDeletionReferenceChunkResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<RegisterSpaceDeletionReferenceChunkReceipt>(
        1, _omitFieldNames ? '' : 'receipt',
        subBuilder: RegisterSpaceDeletionReferenceChunkReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterSpaceDeletionReferenceChunkResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterSpaceDeletionReferenceChunkResponse copyWith(
          void Function(RegisterSpaceDeletionReferenceChunkResponse) updates) =>
      super.copyWith((message) =>
              updates(message as RegisterSpaceDeletionReferenceChunkResponse))
          as RegisterSpaceDeletionReferenceChunkResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterSpaceDeletionReferenceChunkResponse create() =>
      RegisterSpaceDeletionReferenceChunkResponse._();
  @$core.override
  RegisterSpaceDeletionReferenceChunkResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterSpaceDeletionReferenceChunkResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          RegisterSpaceDeletionReferenceChunkResponse>(create);
  static RegisterSpaceDeletionReferenceChunkResponse? _defaultInstance;

  @$pb.TagNumber(1)
  RegisterSpaceDeletionReferenceChunkReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(RegisterSpaceDeletionReferenceChunkReceipt value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  RegisterSpaceDeletionReferenceChunkReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ReleaseSpaceDeletionProducerReferencesRequest
    extends $pb.GeneratedMessage {
  factory ReleaseSpaceDeletionProducerReferencesRequest({
    $core.int? protocolVersion,
    $core.String? deletionOperationId,
    $core.String? spaceId,
    $fixnum.Int64? purgeGeneration,
    $fixnum.Int64? sourceScheduleGeneration,
    FileReferenceProducerId? producerId,
    $core.List<$core.int>? expectedReferencesSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (spaceId != null) result.spaceId = spaceId;
    if (purgeGeneration != null) result.purgeGeneration = purgeGeneration;
    if (sourceScheduleGeneration != null)
      result.sourceScheduleGeneration = sourceScheduleGeneration;
    if (producerId != null) result.producerId = producerId;
    if (expectedReferencesSha256 != null)
      result.expectedReferencesSha256 = expectedReferencesSha256;
    return result;
  }

  ReleaseSpaceDeletionProducerReferencesRequest._();

  factory ReleaseSpaceDeletionProducerReferencesRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReleaseSpaceDeletionProducerReferencesRequest.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReleaseSpaceDeletionProducerReferencesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'deletionOperationId')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'purgeGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(5, _omitFieldNames ? '' : 'sourceScheduleGeneration',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<FileReferenceProducerId>(6, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..a<$core.List<$core.int>>(7,
        _omitFieldNames ? '' : 'expectedReferencesSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseSpaceDeletionProducerReferencesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseSpaceDeletionProducerReferencesRequest copyWith(
          void Function(ReleaseSpaceDeletionProducerReferencesRequest)
              updates) =>
      super.copyWith((message) =>
              updates(message as ReleaseSpaceDeletionProducerReferencesRequest))
          as ReleaseSpaceDeletionProducerReferencesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReleaseSpaceDeletionProducerReferencesRequest create() =>
      ReleaseSpaceDeletionProducerReferencesRequest._();
  @$core.override
  ReleaseSpaceDeletionProducerReferencesRequest createEmptyInstance() =>
      create();
  @$core.pragma('dart2js:noInline')
  static ReleaseSpaceDeletionProducerReferencesRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReleaseSpaceDeletionProducerReferencesRequest>(create);
  static ReleaseSpaceDeletionProducerReferencesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get deletionOperationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set deletionOperationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDeletionOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearDeletionOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get purgeGeneration => $_getI64(3);
  @$pb.TagNumber(4)
  set purgeGeneration($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPurgeGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearPurgeGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get sourceScheduleGeneration => $_getI64(4);
  @$pb.TagNumber(5)
  set sourceScheduleGeneration($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSourceScheduleGeneration() => $_has(4);
  @$pb.TagNumber(5)
  void clearSourceScheduleGeneration() => $_clearField(5);

  @$pb.TagNumber(6)
  FileReferenceProducerId get producerId => $_getN(5);
  @$pb.TagNumber(6)
  set producerId(FileReferenceProducerId value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasProducerId() => $_has(5);
  @$pb.TagNumber(6)
  void clearProducerId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.List<$core.int> get expectedReferencesSha256 => $_getN(6);
  @$pb.TagNumber(7)
  set expectedReferencesSha256($core.List<$core.int> value) =>
      $_setBytes(6, value);
  @$pb.TagNumber(7)
  $core.bool hasExpectedReferencesSha256() => $_has(6);
  @$pb.TagNumber(7)
  void clearExpectedReferencesSha256() => $_clearField(7);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class ReleaseSpaceDeletionProducerReferencesReceipt
    extends $pb.GeneratedMessage {
  factory ReleaseSpaceDeletionProducerReferencesReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? deletionOperationId,
    $core.String? spaceId,
    $fixnum.Int64? purgeGeneration,
    $fixnum.Int64? sourceScheduleGeneration,
    FileReferenceProducerId? producerId,
    $fixnum.Int64? releasedCount,
    $core.List<$core.int>? expectedReferencesSha256,
    $core.List<$core.int>? requestSha256,
    $3.Timestamp? completedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (spaceId != null) result.spaceId = spaceId;
    if (purgeGeneration != null) result.purgeGeneration = purgeGeneration;
    if (sourceScheduleGeneration != null)
      result.sourceScheduleGeneration = sourceScheduleGeneration;
    if (producerId != null) result.producerId = producerId;
    if (releasedCount != null) result.releasedCount = releasedCount;
    if (expectedReferencesSha256 != null)
      result.expectedReferencesSha256 = expectedReferencesSha256;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  ReleaseSpaceDeletionProducerReferencesReceipt._();

  factory ReleaseSpaceDeletionProducerReferencesReceipt.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReleaseSpaceDeletionProducerReferencesReceipt.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReleaseSpaceDeletionProducerReferencesReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'deletionOperationId')
    ..aOS(4, _omitFieldNames ? '' : 'spaceId')
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'purgeGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(6, _omitFieldNames ? '' : 'sourceScheduleGeneration',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<FileReferenceProducerId>(7, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..a<$fixnum.Int64>(
        8, _omitFieldNames ? '' : 'releasedCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(9,
        _omitFieldNames ? '' : 'expectedReferencesSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        10, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$3.Timestamp>(11, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseSpaceDeletionProducerReferencesReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseSpaceDeletionProducerReferencesReceipt copyWith(
          void Function(ReleaseSpaceDeletionProducerReferencesReceipt)
              updates) =>
      super.copyWith((message) =>
              updates(message as ReleaseSpaceDeletionProducerReferencesReceipt))
          as ReleaseSpaceDeletionProducerReferencesReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReleaseSpaceDeletionProducerReferencesReceipt create() =>
      ReleaseSpaceDeletionProducerReferencesReceipt._();
  @$core.override
  ReleaseSpaceDeletionProducerReferencesReceipt createEmptyInstance() =>
      create();
  @$core.pragma('dart2js:noInline')
  static ReleaseSpaceDeletionProducerReferencesReceipt getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReleaseSpaceDeletionProducerReferencesReceipt>(create);
  static ReleaseSpaceDeletionProducerReferencesReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get deletionOperationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set deletionOperationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeletionOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeletionOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get spaceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set spaceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSpaceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpaceId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get purgeGeneration => $_getI64(4);
  @$pb.TagNumber(5)
  set purgeGeneration($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPurgeGeneration() => $_has(4);
  @$pb.TagNumber(5)
  void clearPurgeGeneration() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get sourceScheduleGeneration => $_getI64(5);
  @$pb.TagNumber(6)
  set sourceScheduleGeneration($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSourceScheduleGeneration() => $_has(5);
  @$pb.TagNumber(6)
  void clearSourceScheduleGeneration() => $_clearField(6);

  @$pb.TagNumber(7)
  FileReferenceProducerId get producerId => $_getN(6);
  @$pb.TagNumber(7)
  set producerId(FileReferenceProducerId value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasProducerId() => $_has(6);
  @$pb.TagNumber(7)
  void clearProducerId() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get releasedCount => $_getI64(7);
  @$pb.TagNumber(8)
  set releasedCount($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasReleasedCount() => $_has(7);
  @$pb.TagNumber(8)
  void clearReleasedCount() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.List<$core.int> get expectedReferencesSha256 => $_getN(8);
  @$pb.TagNumber(9)
  set expectedReferencesSha256($core.List<$core.int> value) =>
      $_setBytes(8, value);
  @$pb.TagNumber(9)
  $core.bool hasExpectedReferencesSha256() => $_has(8);
  @$pb.TagNumber(9)
  void clearExpectedReferencesSha256() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.List<$core.int> get requestSha256 => $_getN(9);
  @$pb.TagNumber(10)
  set requestSha256($core.List<$core.int> value) => $_setBytes(9, value);
  @$pb.TagNumber(10)
  $core.bool hasRequestSha256() => $_has(9);
  @$pb.TagNumber(10)
  void clearRequestSha256() => $_clearField(10);

  @$pb.TagNumber(11)
  $3.Timestamp get completedAt => $_getN(10);
  @$pb.TagNumber(11)
  set completedAt($3.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasCompletedAt() => $_has(10);
  @$pb.TagNumber(11)
  void clearCompletedAt() => $_clearField(11);
  @$pb.TagNumber(11)
  $3.Timestamp ensureCompletedAt() => $_ensure(10);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class ReleaseSpaceDeletionProducerReferencesResponse
    extends $pb.GeneratedMessage {
  factory ReleaseSpaceDeletionProducerReferencesResponse({
    ReleaseSpaceDeletionProducerReferencesReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  ReleaseSpaceDeletionProducerReferencesResponse._();

  factory ReleaseSpaceDeletionProducerReferencesResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReleaseSpaceDeletionProducerReferencesResponse.fromJson(
          $core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReleaseSpaceDeletionProducerReferencesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<ReleaseSpaceDeletionProducerReferencesReceipt>(
        1, _omitFieldNames ? '' : 'receipt',
        subBuilder: ReleaseSpaceDeletionProducerReferencesReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseSpaceDeletionProducerReferencesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseSpaceDeletionProducerReferencesResponse copyWith(
          void Function(ReleaseSpaceDeletionProducerReferencesResponse)
              updates) =>
      super.copyWith((message) => updates(
              message as ReleaseSpaceDeletionProducerReferencesResponse))
          as ReleaseSpaceDeletionProducerReferencesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReleaseSpaceDeletionProducerReferencesResponse create() =>
      ReleaseSpaceDeletionProducerReferencesResponse._();
  @$core.override
  ReleaseSpaceDeletionProducerReferencesResponse createEmptyInstance() =>
      create();
  @$core.pragma('dart2js:noInline')
  static ReleaseSpaceDeletionProducerReferencesResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReleaseSpaceDeletionProducerReferencesResponse>(create);
  static ReleaseSpaceDeletionProducerReferencesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ReleaseSpaceDeletionProducerReferencesReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(ReleaseSpaceDeletionProducerReferencesReceipt value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  ReleaseSpaceDeletionProducerReferencesReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ApplySpaceLifecycleFenceRequest extends $pb.GeneratedMessage {
  factory ApplySpaceLifecycleFenceRequest({
    $5.SpaceLifecycleFenceRequest? fence,
  }) {
    final result = create();
    if (fence != null) result.fence = fence;
    return result;
  }

  ApplySpaceLifecycleFenceRequest._();

  factory ApplySpaceLifecycleFenceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplySpaceLifecycleFenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplySpaceLifecycleFenceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<$5.SpaceLifecycleFenceRequest>(1, _omitFieldNames ? '' : 'fence',
        subBuilder: $5.SpaceLifecycleFenceRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySpaceLifecycleFenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySpaceLifecycleFenceRequest copyWith(
          void Function(ApplySpaceLifecycleFenceRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ApplySpaceLifecycleFenceRequest))
          as ApplySpaceLifecycleFenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplySpaceLifecycleFenceRequest create() =>
      ApplySpaceLifecycleFenceRequest._();
  @$core.override
  ApplySpaceLifecycleFenceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplySpaceLifecycleFenceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplySpaceLifecycleFenceRequest>(
          create);
  static ApplySpaceLifecycleFenceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $5.SpaceLifecycleFenceRequest get fence => $_getN(0);
  @$pb.TagNumber(1)
  set fence($5.SpaceLifecycleFenceRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFence() => $_has(0);
  @$pb.TagNumber(1)
  void clearFence() => $_clearField(1);
  @$pb.TagNumber(1)
  $5.SpaceLifecycleFenceRequest ensureFence() => $_ensure(0);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class ApplySpaceLifecycleFenceResponse extends $pb.GeneratedMessage {
  factory ApplySpaceLifecycleFenceResponse({
    $5.SpaceLifecycleFenceReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  ApplySpaceLifecycleFenceResponse._();

  factory ApplySpaceLifecycleFenceResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplySpaceLifecycleFenceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplySpaceLifecycleFenceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<$5.SpaceLifecycleFenceReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: $5.SpaceLifecycleFenceReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySpaceLifecycleFenceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySpaceLifecycleFenceResponse copyWith(
          void Function(ApplySpaceLifecycleFenceResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ApplySpaceLifecycleFenceResponse))
          as ApplySpaceLifecycleFenceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplySpaceLifecycleFenceResponse create() =>
      ApplySpaceLifecycleFenceResponse._();
  @$core.override
  ApplySpaceLifecycleFenceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplySpaceLifecycleFenceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplySpaceLifecycleFenceResponse>(
          create);
  static ApplySpaceLifecycleFenceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $5.SpaceLifecycleFenceReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt($5.SpaceLifecycleFenceReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  $5.SpaceLifecycleFenceReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class PurgeSpaceRequest extends $pb.GeneratedMessage {
  factory PurgeSpaceRequest({
    $5.SpacePurgeRequest? purge,
  }) {
    final result = create();
    if (purge != null) result.purge = purge;
    return result;
  }

  PurgeSpaceRequest._();

  factory PurgeSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PurgeSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PurgeSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<$5.SpacePurgeRequest>(1, _omitFieldNames ? '' : 'purge',
        subBuilder: $5.SpacePurgeRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PurgeSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PurgeSpaceRequest copyWith(void Function(PurgeSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as PurgeSpaceRequest))
          as PurgeSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PurgeSpaceRequest create() => PurgeSpaceRequest._();
  @$core.override
  PurgeSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PurgeSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PurgeSpaceRequest>(create);
  static PurgeSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $5.SpacePurgeRequest get purge => $_getN(0);
  @$pb.TagNumber(1)
  set purge($5.SpacePurgeRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPurge() => $_has(0);
  @$pb.TagNumber(1)
  void clearPurge() => $_clearField(1);
  @$pb.TagNumber(1)
  $5.SpacePurgeRequest ensurePurge() => $_ensure(0);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class PurgeSpaceResponse extends $pb.GeneratedMessage {
  factory PurgeSpaceResponse({
    $5.SpacePurgeReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  PurgeSpaceResponse._();

  factory PurgeSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PurgeSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PurgeSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<$5.SpacePurgeReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: $5.SpacePurgeReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PurgeSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PurgeSpaceResponse copyWith(void Function(PurgeSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as PurgeSpaceResponse))
          as PurgeSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PurgeSpaceResponse create() => PurgeSpaceResponse._();
  @$core.override
  PurgeSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PurgeSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PurgeSpaceResponse>(create);
  static PurgeSpaceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $5.SpacePurgeReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt($5.SpacePurgeReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  $5.SpacePurgeReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class AcquireFileReferencesRequest extends $pb.GeneratedMessage {
  factory AcquireFileReferencesRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    FileReferenceProducerId? producerId,
    $core.Iterable<FileReferenceKey>? references,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (producerId != null) result.producerId = producerId;
    if (references != null) result.references.addAll(references);
    return result;
  }

  AcquireFileReferencesRequest._();

  factory AcquireFileReferencesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcquireFileReferencesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcquireFileReferencesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aE<FileReferenceProducerId>(3, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..pPM<FileReferenceKey>(4, _omitFieldNames ? '' : 'references',
        subBuilder: FileReferenceKey.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireFileReferencesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireFileReferencesRequest copyWith(
          void Function(AcquireFileReferencesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as AcquireFileReferencesRequest))
          as AcquireFileReferencesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcquireFileReferencesRequest create() =>
      AcquireFileReferencesRequest._();
  @$core.override
  AcquireFileReferencesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcquireFileReferencesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcquireFileReferencesRequest>(create);
  static AcquireFileReferencesRequest? _defaultInstance;

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
  FileReferenceProducerId get producerId => $_getN(2);
  @$pb.TagNumber(3)
  set producerId(FileReferenceProducerId value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasProducerId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProducerId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<FileReferenceKey> get references => $_getList(3);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class AcquireFileReferencesReceipt extends $pb.GeneratedMessage {
  factory AcquireFileReferencesReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? operationId,
    FileReferenceProducerId? producerId,
    $fixnum.Int64? referenceCount,
    $core.List<$core.int>? requestSha256,
    $3.Timestamp? completedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (operationId != null) result.operationId = operationId;
    if (producerId != null) result.producerId = producerId;
    if (referenceCount != null) result.referenceCount = referenceCount;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  AcquireFileReferencesReceipt._();

  factory AcquireFileReferencesReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcquireFileReferencesReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcquireFileReferencesReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..aE<FileReferenceProducerId>(4, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'referenceCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$3.Timestamp>(7, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireFileReferencesReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireFileReferencesReceipt copyWith(
          void Function(AcquireFileReferencesReceipt) updates) =>
      super.copyWith(
              (message) => updates(message as AcquireFileReferencesReceipt))
          as AcquireFileReferencesReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcquireFileReferencesReceipt create() =>
      AcquireFileReferencesReceipt._();
  @$core.override
  AcquireFileReferencesReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcquireFileReferencesReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcquireFileReferencesReceipt>(create);
  static AcquireFileReferencesReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  FileReferenceProducerId get producerId => $_getN(3);
  @$pb.TagNumber(4)
  set producerId(FileReferenceProducerId value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasProducerId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProducerId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get referenceCount => $_getI64(4);
  @$pb.TagNumber(5)
  set referenceCount($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasReferenceCount() => $_has(4);
  @$pb.TagNumber(5)
  void clearReferenceCount() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get requestSha256 => $_getN(5);
  @$pb.TagNumber(6)
  set requestSha256($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRequestSha256() => $_has(5);
  @$pb.TagNumber(6)
  void clearRequestSha256() => $_clearField(6);

  @$pb.TagNumber(7)
  $3.Timestamp get completedAt => $_getN(6);
  @$pb.TagNumber(7)
  set completedAt($3.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasCompletedAt() => $_has(6);
  @$pb.TagNumber(7)
  void clearCompletedAt() => $_clearField(7);
  @$pb.TagNumber(7)
  $3.Timestamp ensureCompletedAt() => $_ensure(6);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class AcquireFileReferencesResponse extends $pb.GeneratedMessage {
  factory AcquireFileReferencesResponse({
    AcquireFileReferencesReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  AcquireFileReferencesResponse._();

  factory AcquireFileReferencesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcquireFileReferencesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcquireFileReferencesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<AcquireFileReferencesReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: AcquireFileReferencesReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireFileReferencesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcquireFileReferencesResponse copyWith(
          void Function(AcquireFileReferencesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as AcquireFileReferencesResponse))
          as AcquireFileReferencesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcquireFileReferencesResponse create() =>
      AcquireFileReferencesResponse._();
  @$core.override
  AcquireFileReferencesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcquireFileReferencesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcquireFileReferencesResponse>(create);
  static AcquireFileReferencesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AcquireFileReferencesReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(AcquireFileReferencesReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  AcquireFileReferencesReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class ReleaseFileReferencesRequest extends $pb.GeneratedMessage {
  factory ReleaseFileReferencesRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    FileReferenceProducerId? producerId,
    $core.Iterable<FileReferenceKey>? references,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (producerId != null) result.producerId = producerId;
    if (references != null) result.references.addAll(references);
    return result;
  }

  ReleaseFileReferencesRequest._();

  factory ReleaseFileReferencesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReleaseFileReferencesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReleaseFileReferencesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aE<FileReferenceProducerId>(3, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..pPM<FileReferenceKey>(4, _omitFieldNames ? '' : 'references',
        subBuilder: FileReferenceKey.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseFileReferencesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseFileReferencesRequest copyWith(
          void Function(ReleaseFileReferencesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ReleaseFileReferencesRequest))
          as ReleaseFileReferencesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReleaseFileReferencesRequest create() =>
      ReleaseFileReferencesRequest._();
  @$core.override
  ReleaseFileReferencesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReleaseFileReferencesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReleaseFileReferencesRequest>(create);
  static ReleaseFileReferencesRequest? _defaultInstance;

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
  FileReferenceProducerId get producerId => $_getN(2);
  @$pb.TagNumber(3)
  set producerId(FileReferenceProducerId value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasProducerId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProducerId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<FileReferenceKey> get references => $_getList(3);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class ReleaseFileReferencesReceipt extends $pb.GeneratedMessage {
  factory ReleaseFileReferencesReceipt({
    $core.int? protocolVersion,
    $core.String? receiptId,
    $core.String? operationId,
    FileReferenceProducerId? producerId,
    $fixnum.Int64? releasedCount,
    $core.List<$core.int>? requestSha256,
    $3.Timestamp? completedAt,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (receiptId != null) result.receiptId = receiptId;
    if (operationId != null) result.operationId = operationId;
    if (producerId != null) result.producerId = producerId;
    if (releasedCount != null) result.releasedCount = releasedCount;
    if (requestSha256 != null) result.requestSha256 = requestSha256;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  ReleaseFileReferencesReceipt._();

  factory ReleaseFileReferencesReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReleaseFileReferencesReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReleaseFileReferencesReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'receiptId')
    ..aOS(3, _omitFieldNames ? '' : 'operationId')
    ..aE<FileReferenceProducerId>(4, _omitFieldNames ? '' : 'producerId',
        enumValues: FileReferenceProducerId.values)
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'releasedCount', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'requestSha256', $pb.PbFieldType.OY)
    ..aOM<$3.Timestamp>(7, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseFileReferencesReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseFileReferencesReceipt copyWith(
          void Function(ReleaseFileReferencesReceipt) updates) =>
      super.copyWith(
              (message) => updates(message as ReleaseFileReferencesReceipt))
          as ReleaseFileReferencesReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReleaseFileReferencesReceipt create() =>
      ReleaseFileReferencesReceipt._();
  @$core.override
  ReleaseFileReferencesReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReleaseFileReferencesReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReleaseFileReferencesReceipt>(create);
  static ReleaseFileReferencesReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get receiptId => $_getSZ(1);
  @$pb.TagNumber(2)
  set receiptId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReceiptId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReceiptId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get operationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set operationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  FileReferenceProducerId get producerId => $_getN(3);
  @$pb.TagNumber(4)
  set producerId(FileReferenceProducerId value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasProducerId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProducerId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get releasedCount => $_getI64(4);
  @$pb.TagNumber(5)
  set releasedCount($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasReleasedCount() => $_has(4);
  @$pb.TagNumber(5)
  void clearReleasedCount() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get requestSha256 => $_getN(5);
  @$pb.TagNumber(6)
  set requestSha256($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRequestSha256() => $_has(5);
  @$pb.TagNumber(6)
  void clearRequestSha256() => $_clearField(6);

  @$pb.TagNumber(7)
  $3.Timestamp get completedAt => $_getN(6);
  @$pb.TagNumber(7)
  set completedAt($3.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasCompletedAt() => $_has(6);
  @$pb.TagNumber(7)
  void clearCompletedAt() => $_clearField(7);
  @$pb.TagNumber(7)
  $3.Timestamp ensureCompletedAt() => $_ensure(6);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class ReleaseFileReferencesResponse extends $pb.GeneratedMessage {
  factory ReleaseFileReferencesResponse({
    ReleaseFileReferencesReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  ReleaseFileReferencesResponse._();

  factory ReleaseFileReferencesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReleaseFileReferencesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReleaseFileReferencesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<ReleaseFileReferencesReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: ReleaseFileReferencesReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseFileReferencesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReleaseFileReferencesResponse copyWith(
          void Function(ReleaseFileReferencesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ReleaseFileReferencesResponse))
          as ReleaseFileReferencesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReleaseFileReferencesResponse create() =>
      ReleaseFileReferencesResponse._();
  @$core.override
  ReleaseFileReferencesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReleaseFileReferencesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReleaseFileReferencesResponse>(create);
  static ReleaseFileReferencesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ReleaseFileReferencesReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(ReleaseFileReferencesReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  ReleaseFileReferencesReceipt ensureReceipt() => $_ensure(0);
}

/// @voice.unknown_fields=reject
/// @voice.hash=domain_separated_sha256
class GetSpacePurgeReceiptRequest extends $pb.GeneratedMessage {
  factory GetSpacePurgeReceiptRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? deletionOperationId,
    $fixnum.Int64? generation,
    $core.List<$core.int>? purgeRequestSha256,
    $core.List<$core.int>? manifestSha256,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (deletionOperationId != null)
      result.deletionOperationId = deletionOperationId;
    if (generation != null) result.generation = generation;
    if (purgeRequestSha256 != null)
      result.purgeRequestSha256 = purgeRequestSha256;
    if (manifestSha256 != null) result.manifestSha256 = manifestSha256;
    return result;
  }

  GetSpacePurgeReceiptRequest._();

  factory GetSpacePurgeReceiptRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSpacePurgeReceiptRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSpacePurgeReceiptRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'deletionOperationId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'purgeRequestSha256', $pb.PbFieldType.OY)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'manifestSha256', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpacePurgeReceiptRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpacePurgeReceiptRequest copyWith(
          void Function(GetSpacePurgeReceiptRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetSpacePurgeReceiptRequest))
          as GetSpacePurgeReceiptRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSpacePurgeReceiptRequest create() =>
      GetSpacePurgeReceiptRequest._();
  @$core.override
  GetSpacePurgeReceiptRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSpacePurgeReceiptRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSpacePurgeReceiptRequest>(create);
  static GetSpacePurgeReceiptRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get deletionOperationId => $_getSZ(2);
  @$pb.TagNumber(3)
  set deletionOperationId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeletionOperationId() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeletionOperationId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get generation => $_getI64(3);
  @$pb.TagNumber(4)
  set generation($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get purgeRequestSha256 => $_getN(4);
  @$pb.TagNumber(5)
  set purgeRequestSha256($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPurgeRequestSha256() => $_has(4);
  @$pb.TagNumber(5)
  void clearPurgeRequestSha256() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.List<$core.int> get manifestSha256 => $_getN(5);
  @$pb.TagNumber(6)
  set manifestSha256($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasManifestSha256() => $_has(5);
  @$pb.TagNumber(6)
  void clearManifestSha256() => $_clearField(6);
}

/// @voice.unknown_fields=accept_preserve
/// @voice.hash=domain_separated_sha256
class GetSpacePurgeReceiptResponse extends $pb.GeneratedMessage {
  factory GetSpacePurgeReceiptResponse({
    $5.SpacePurgeReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  GetSpacePurgeReceiptResponse._();

  factory GetSpacePurgeReceiptResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSpacePurgeReceiptResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSpacePurgeReceiptResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.file.v1'),
      createEmptyInstance: create)
    ..aOM<$5.SpacePurgeReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: $5.SpacePurgeReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpacePurgeReceiptResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpacePurgeReceiptResponse copyWith(
          void Function(GetSpacePurgeReceiptResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetSpacePurgeReceiptResponse))
          as GetSpacePurgeReceiptResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSpacePurgeReceiptResponse create() =>
      GetSpacePurgeReceiptResponse._();
  @$core.override
  GetSpacePurgeReceiptResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSpacePurgeReceiptResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSpacePurgeReceiptResponse>(create);
  static GetSpacePurgeReceiptResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $5.SpacePurgeReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt($5.SpacePurgeReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  $5.SpacePurgeReceipt ensureReceipt() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
