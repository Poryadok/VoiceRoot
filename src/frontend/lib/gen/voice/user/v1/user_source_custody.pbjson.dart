// This is a generated file - do not edit.
//
// Generated from voice/user/v1/user_source_custody.proto.

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

@$core.Deprecated('Use userSourcePermitFrameKindDescriptor instead')
const UserSourcePermitFrameKind$json = {
  '1': 'UserSourcePermitFrameKind',
  '2': [
    {'1': 'USER_SOURCE_PERMIT_FRAME_KIND_UNSPECIFIED', '2': 0},
    {'1': 'USER_SOURCE_PERMIT_FRAME_KIND_GRANTED', '2': 1},
    {'1': 'USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT', '2': 2},
  ],
};

/// Descriptor for `UserSourcePermitFrameKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List userSourcePermitFrameKindDescriptor = $convert.base64Decode(
    'ChlVc2VyU291cmNlUGVybWl0RnJhbWVLaW5kEi0KKVVTRVJfU09VUkNFX1BFUk1JVF9GUkFNRV'
    '9LSU5EX1VOU1BFQ0lGSUVEEAASKQolVVNFUl9TT1VSQ0VfUEVSTUlUX0ZSQU1FX0tJTkRfR1JB'
    'TlRFRBABEisKJ1VTRVJfU09VUkNFX1BFUk1JVF9GUkFNRV9LSU5EX0hFQVJUQkVBVBAC');

@$core.Deprecated('Use userSourcePermitBindingDescriptor instead')
const UserSourcePermitBinding$json = {
  '1': 'UserSourcePermitBinding',
  '2': [
    {'1': 'epoch', '3': 1, '4': 1, '5': 4, '10': 'epoch'},
    {'1': 'source_id', '3': 2, '4': 1, '5': 9, '10': 'sourceId'},
    {
      '1': 'history_generation',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'historyGeneration'
    },
    {
      '1': 'user_source_generation',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'userSourceGeneration'
    },
    {'1': 'keyset_digest', '3': 5, '4': 1, '5': 12, '10': 'keysetDigest'},
  ],
};

/// Descriptor for `UserSourcePermitBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userSourcePermitBindingDescriptor = $convert.base64Decode(
    'ChdVc2VyU291cmNlUGVybWl0QmluZGluZxIUCgVlcG9jaBgBIAEoBFIFZXBvY2gSGwoJc291cm'
    'NlX2lkGAIgASgJUghzb3VyY2VJZBItChJoaXN0b3J5X2dlbmVyYXRpb24YAyABKAlSEWhpc3Rv'
    'cnlHZW5lcmF0aW9uEjQKFnVzZXJfc291cmNlX2dlbmVyYXRpb24YBCABKAlSFHVzZXJTb3VyY2'
    'VHZW5lcmF0aW9uEiMKDWtleXNldF9kaWdlc3QYBSABKAxSDGtleXNldERpZ2VzdA==');

@$core.Deprecated('Use acquireUserSourcePermitRequestDescriptor instead')
const AcquireUserSourcePermitRequest$json = {
  '1': 'AcquireUserSourcePermitRequest',
  '2': [
    {'1': 'protocol', '3': 1, '4': 1, '5': 13, '10': 'protocol'},
    {
      '1': 'binding',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.UserSourcePermitBinding',
      '10': 'binding'
    },
  ],
};

/// Descriptor for `AcquireUserSourcePermitRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acquireUserSourcePermitRequestDescriptor =
    $convert.base64Decode(
        'Ch5BY3F1aXJlVXNlclNvdXJjZVBlcm1pdFJlcXVlc3QSGgoIcHJvdG9jb2wYASABKA1SCHByb3'
        'RvY29sEkAKB2JpbmRpbmcYAiABKAsyJi52b2ljZS51c2VyLnYxLlVzZXJTb3VyY2VQZXJtaXRC'
        'aW5kaW5nUgdiaW5kaW5n');

@$core.Deprecated('Use userSourcePhysicalBindingDescriptor instead')
const UserSourcePhysicalBinding$json = {
  '1': 'UserSourcePhysicalBinding',
  '2': [
    {
      '1': 'system_identifier',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'systemIdentifier'
    },
    {'1': 'database', '3': 2, '4': 1, '5': 9, '10': 'database'},
    {'1': 'database_oid', '3': 3, '4': 1, '5': 13, '10': 'databaseOid'},
  ],
};

/// Descriptor for `UserSourcePhysicalBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userSourcePhysicalBindingDescriptor = $convert.base64Decode(
    'ChlVc2VyU291cmNlUGh5c2ljYWxCaW5kaW5nEisKEXN5c3RlbV9pZGVudGlmaWVyGAEgASgJUh'
    'BzeXN0ZW1JZGVudGlmaWVyEhoKCGRhdGFiYXNlGAIgASgJUghkYXRhYmFzZRIhCgxkYXRhYmFz'
    'ZV9vaWQYAyABKA1SC2RhdGFiYXNlT2lk');

@$core.Deprecated('Use acquireUserSourcePermitResponseDescriptor instead')
const AcquireUserSourcePermitResponse$json = {
  '1': 'AcquireUserSourcePermitResponse',
  '2': [
    {'1': 'protocol', '3': 1, '4': 1, '5': 13, '10': 'protocol'},
    {
      '1': 'binding',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.UserSourcePermitBinding',
      '10': 'binding'
    },
    {'1': 'sequence', '3': 3, '4': 1, '5': 4, '10': 'sequence'},
    {
      '1': 'kind',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.UserSourcePermitFrameKind',
      '10': 'kind'
    },
    {
      '1': 'physical',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.UserSourcePhysicalBinding',
      '10': 'physical'
    },
  ],
};

/// Descriptor for `AcquireUserSourcePermitResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acquireUserSourcePermitResponseDescriptor = $convert.base64Decode(
    'Ch9BY3F1aXJlVXNlclNvdXJjZVBlcm1pdFJlc3BvbnNlEhoKCHByb3RvY29sGAEgASgNUghwcm'
    '90b2NvbBJACgdiaW5kaW5nGAIgASgLMiYudm9pY2UudXNlci52MS5Vc2VyU291cmNlUGVybWl0'
    'QmluZGluZ1IHYmluZGluZxIaCghzZXF1ZW5jZRgDIAEoBFIIc2VxdWVuY2USPAoEa2luZBgEIA'
    'EoDjIoLnZvaWNlLnVzZXIudjEuVXNlclNvdXJjZVBlcm1pdEZyYW1lS2luZFIEa2luZBJECghw'
    'aHlzaWNhbBgFIAEoCzIoLnZvaWNlLnVzZXIudjEuVXNlclNvdXJjZVBoeXNpY2FsQmluZGluZ1'
    'IIcGh5c2ljYWw=');
