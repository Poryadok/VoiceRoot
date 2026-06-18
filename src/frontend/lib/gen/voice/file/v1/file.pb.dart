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

class GetFileURLRequest extends $pb.GeneratedMessage {
  factory GetFileURLRequest({
    $core.String? fileId,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
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
}

class GetFileMetadataRequest extends $pb.GeneratedMessage {
  factory GetFileMetadataRequest({
    $core.String? fileId,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
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
}

class GetBulkMetadataRequest extends $pb.GeneratedMessage {
  factory GetBulkMetadataRequest({
    $core.Iterable<$core.String>? fileIds,
  }) {
    final result = create();
    if (fileIds != null) result.fileIds.addAll(fileIds);
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

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get fileIds => $_getList(0);
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

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
