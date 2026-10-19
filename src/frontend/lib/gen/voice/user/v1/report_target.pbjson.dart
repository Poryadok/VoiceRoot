// This is a generated file - do not edit.
//
// Generated from voice/user/v1/report_target.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

@$core.Deprecated('Use resolveReportTargetRequestDescriptor instead')
const ResolveReportTargetRequest$json = {
  '1': 'ResolveReportTargetRequest',
  '2': [
    {'1': 'report_id', '3': 1, '4': 1, '5': 9, '10': 'reportId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `ResolveReportTargetRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveReportTargetRequestDescriptor =
    $convert.base64Decode(
        'ChpSZXNvbHZlUmVwb3J0VGFyZ2V0UmVxdWVzdBIbCglyZXBvcnRfaWQYASABKAlSCHJlcG9ydE'
        'lkEh0KCnByb2ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZA==');

@$core.Deprecated('Use resolveReportTargetResponseDescriptor instead')
const ResolveReportTargetResponse$json = {
  '1': 'ResolveReportTargetResponse',
  '2': [
    {
      '1': 'state',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.voice.common.v1.ReportTargetState',
      '10': 'state'
    },
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `ResolveReportTargetResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveReportTargetResponseDescriptor =
    $convert.base64Decode(
        'ChtSZXNvbHZlUmVwb3J0VGFyZ2V0UmVzcG9uc2USOAoFc3RhdGUYASABKA4yIi52b2ljZS5jb2'
        '1tb24udjEuUmVwb3J0VGFyZ2V0U3RhdGVSBXN0YXRlEh0KCmFjY291bnRfaWQYAiABKAlSCWFj'
        'Y291bnRJZA==');
