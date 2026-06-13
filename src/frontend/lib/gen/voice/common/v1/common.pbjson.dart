// This is a generated file - do not edit.
//
// Generated from voice/common/v1/common.proto.

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

@$core.Deprecated('Use cursorPageRequestDescriptor instead')
const CursorPageRequest$json = {
  '1': 'CursorPageRequest',
  '2': [
    {'1': 'cursor', '3': 1, '4': 1, '5': 9, '10': 'cursor'},
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
  ],
};

/// Descriptor for `CursorPageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cursorPageRequestDescriptor = $convert.base64Decode(
    'ChFDdXJzb3JQYWdlUmVxdWVzdBIWCgZjdXJzb3IYASABKAlSBmN1cnNvchIbCglwYWdlX3Npem'
    'UYAiABKAVSCHBhZ2VTaXpl');

@$core.Deprecated('Use cursorPageResponseDescriptor instead')
const CursorPageResponse$json = {
  '1': 'CursorPageResponse',
  '2': [
    {'1': 'next_cursor', '3': 1, '4': 1, '5': 9, '10': 'nextCursor'},
    {'1': 'has_more', '3': 2, '4': 1, '5': 8, '10': 'hasMore'},
  ],
};

/// Descriptor for `CursorPageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cursorPageResponseDescriptor = $convert.base64Decode(
    'ChJDdXJzb3JQYWdlUmVzcG9uc2USHwoLbmV4dF9jdXJzb3IYASABKAlSCm5leHRDdXJzb3ISGQ'
    'oIaGFzX21vcmUYAiABKAhSB2hhc01vcmU=');

@$core.Deprecated('Use pageRequestDescriptor instead')
const PageRequest$json = {
  '1': 'PageRequest',
  '2': [
    {'1': 'offset', '3': 1, '4': 1, '5': 5, '10': 'offset'},
    {'1': 'limit', '3': 2, '4': 1, '5': 5, '10': 'limit'},
  ],
};

/// Descriptor for `PageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pageRequestDescriptor = $convert.base64Decode(
    'CgtQYWdlUmVxdWVzdBIWCgZvZmZzZXQYASABKAVSBm9mZnNldBIUCgVsaW1pdBgCIAEoBVIFbG'
    'ltaXQ=');

@$core.Deprecated('Use pageResponseDescriptor instead')
const PageResponse$json = {
  '1': 'PageResponse',
  '2': [
    {'1': 'total_count', '3': 1, '4': 1, '5': 5, '10': 'totalCount'},
  ],
};

/// Descriptor for `PageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pageResponseDescriptor = $convert.base64Decode(
    'CgxQYWdlUmVzcG9uc2USHwoLdG90YWxfY291bnQYASABKAVSCnRvdGFsQ291bnQ=');
