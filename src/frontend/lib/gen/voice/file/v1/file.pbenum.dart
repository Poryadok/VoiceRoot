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

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for FileMetadata.status (string). See microservices/file-service.md for lifecycle.
class FileLifecycleStatus extends $pb.ProtobufEnum {
  static const FileLifecycleStatus FILE_LIFECYCLE_STATUS_UNSPECIFIED =
      FileLifecycleStatus._(
          0, _omitEnumNames ? '' : 'FILE_LIFECYCLE_STATUS_UNSPECIFIED');
  static const FileLifecycleStatus FILE_LIFECYCLE_STATUS_PENDING_UPLOAD =
      FileLifecycleStatus._(
          1, _omitEnumNames ? '' : 'FILE_LIFECYCLE_STATUS_PENDING_UPLOAD');
  static const FileLifecycleStatus FILE_LIFECYCLE_STATUS_PROCESSING =
      FileLifecycleStatus._(
          2, _omitEnumNames ? '' : 'FILE_LIFECYCLE_STATUS_PROCESSING');
  static const FileLifecycleStatus FILE_LIFECYCLE_STATUS_READY =
      FileLifecycleStatus._(
          3, _omitEnumNames ? '' : 'FILE_LIFECYCLE_STATUS_READY');
  static const FileLifecycleStatus FILE_LIFECYCLE_STATUS_FAILED =
      FileLifecycleStatus._(
          4, _omitEnumNames ? '' : 'FILE_LIFECYCLE_STATUS_FAILED');
  static const FileLifecycleStatus FILE_LIFECYCLE_STATUS_DELETED =
      FileLifecycleStatus._(
          5, _omitEnumNames ? '' : 'FILE_LIFECYCLE_STATUS_DELETED');

  static const $core.List<FileLifecycleStatus> values = <FileLifecycleStatus>[
    FILE_LIFECYCLE_STATUS_UNSPECIFIED,
    FILE_LIFECYCLE_STATUS_PENDING_UPLOAD,
    FILE_LIFECYCLE_STATUS_PROCESSING,
    FILE_LIFECYCLE_STATUS_READY,
    FILE_LIFECYCLE_STATUS_FAILED,
    FILE_LIFECYCLE_STATUS_DELETED,
  ];

  static final $core.List<FileLifecycleStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static FileLifecycleStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const FileLifecycleStatus._(super.value, super.name);
}

/// Canonical values for FileMetadata.file_type (string).
class FileMediaCategory extends $pb.ProtobufEnum {
  static const FileMediaCategory FILE_MEDIA_CATEGORY_UNSPECIFIED =
      FileMediaCategory._(
          0, _omitEnumNames ? '' : 'FILE_MEDIA_CATEGORY_UNSPECIFIED');
  static const FileMediaCategory FILE_MEDIA_CATEGORY_IMAGE =
      FileMediaCategory._(1, _omitEnumNames ? '' : 'FILE_MEDIA_CATEGORY_IMAGE');
  static const FileMediaCategory FILE_MEDIA_CATEGORY_VIDEO =
      FileMediaCategory._(2, _omitEnumNames ? '' : 'FILE_MEDIA_CATEGORY_VIDEO');
  static const FileMediaCategory FILE_MEDIA_CATEGORY_AUDIO =
      FileMediaCategory._(3, _omitEnumNames ? '' : 'FILE_MEDIA_CATEGORY_AUDIO');
  static const FileMediaCategory FILE_MEDIA_CATEGORY_DOCUMENT =
      FileMediaCategory._(
          4, _omitEnumNames ? '' : 'FILE_MEDIA_CATEGORY_DOCUMENT');
  static const FileMediaCategory FILE_MEDIA_CATEGORY_OTHER =
      FileMediaCategory._(5, _omitEnumNames ? '' : 'FILE_MEDIA_CATEGORY_OTHER');

  static const $core.List<FileMediaCategory> values = <FileMediaCategory>[
    FILE_MEDIA_CATEGORY_UNSPECIFIED,
    FILE_MEDIA_CATEGORY_IMAGE,
    FILE_MEDIA_CATEGORY_VIDEO,
    FILE_MEDIA_CATEGORY_AUDIO,
    FILE_MEDIA_CATEGORY_DOCUMENT,
    FILE_MEDIA_CATEGORY_OTHER,
  ];

  static final $core.List<FileMediaCategory?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static FileMediaCategory? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const FileMediaCategory._(super.value, super.name);
}

/// Canonical values for FileMetadata.scan_result (string).
class FileScanOutcome extends $pb.ProtobufEnum {
  static const FileScanOutcome FILE_SCAN_OUTCOME_UNSPECIFIED =
      FileScanOutcome._(
          0, _omitEnumNames ? '' : 'FILE_SCAN_OUTCOME_UNSPECIFIED');
  static const FileScanOutcome FILE_SCAN_OUTCOME_PENDING =
      FileScanOutcome._(1, _omitEnumNames ? '' : 'FILE_SCAN_OUTCOME_PENDING');
  static const FileScanOutcome FILE_SCAN_OUTCOME_CLEAN =
      FileScanOutcome._(2, _omitEnumNames ? '' : 'FILE_SCAN_OUTCOME_CLEAN');
  static const FileScanOutcome FILE_SCAN_OUTCOME_INFECTED =
      FileScanOutcome._(3, _omitEnumNames ? '' : 'FILE_SCAN_OUTCOME_INFECTED');
  static const FileScanOutcome FILE_SCAN_OUTCOME_ERROR =
      FileScanOutcome._(4, _omitEnumNames ? '' : 'FILE_SCAN_OUTCOME_ERROR');
  static const FileScanOutcome FILE_SCAN_OUTCOME_SKIPPED =
      FileScanOutcome._(5, _omitEnumNames ? '' : 'FILE_SCAN_OUTCOME_SKIPPED');

  static const $core.List<FileScanOutcome> values = <FileScanOutcome>[
    FILE_SCAN_OUTCOME_UNSPECIFIED,
    FILE_SCAN_OUTCOME_PENDING,
    FILE_SCAN_OUTCOME_CLEAN,
    FILE_SCAN_OUTCOME_INFECTED,
    FILE_SCAN_OUTCOME_ERROR,
    FILE_SCAN_OUTCOME_SKIPPED,
  ];

  static final $core.List<FileScanOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static FileScanOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const FileScanOutcome._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
