// This is a generated file - do not edit.
//
// Generated from voice/common/v1/report_target.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Internal owner decision; never exposed through a public complaint receipt.
class ReportTargetState extends $pb.ProtobufEnum {
  static const ReportTargetState REPORT_TARGET_STATE_UNSPECIFIED =
      ReportTargetState._(
          0, _omitEnumNames ? '' : 'REPORT_TARGET_STATE_UNSPECIFIED');
  static const ReportTargetState REPORT_TARGET_STATE_LIVE =
      ReportTargetState._(1, _omitEnumNames ? '' : 'REPORT_TARGET_STATE_LIVE');
  static const ReportTargetState REPORT_TARGET_STATE_MISSING =
      ReportTargetState._(
          2, _omitEnumNames ? '' : 'REPORT_TARGET_STATE_MISSING');
  static const ReportTargetState REPORT_TARGET_STATE_DELETED =
      ReportTargetState._(
          3, _omitEnumNames ? '' : 'REPORT_TARGET_STATE_DELETED');

  static const $core.List<ReportTargetState> values = <ReportTargetState>[
    REPORT_TARGET_STATE_UNSPECIFIED,
    REPORT_TARGET_STATE_LIVE,
    REPORT_TARGET_STATE_MISSING,
    REPORT_TARGET_STATE_DELETED,
  ];

  static final $core.List<ReportTargetState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ReportTargetState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ReportTargetState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
