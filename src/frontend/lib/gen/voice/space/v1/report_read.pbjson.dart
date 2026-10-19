// This is a generated file - do not edit.
//
// Generated from voice/space/v1/report_read.proto.

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

@$core.Deprecated('Use resolveSpaceReportReadContextRequestDescriptor instead')
const ResolveSpaceReportReadContextRequest$json = {
  '1': 'ResolveSpaceReportReadContextRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'chat',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'viewer_profile_id', '3': 5, '4': 1, '5': 9, '10': 'viewerProfileId'},
  ],
};

/// Descriptor for `ResolveSpaceReportReadContextRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveSpaceReportReadContextRequestDescriptor =
    $convert.base64Decode(
        'CiRSZXNvbHZlU3BhY2VSZXBvcnRSZWFkQ29udGV4dFJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2'
        'lvbhgBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEiEKDG9wZXJhdGlvbl9pZBgCIAEoCVILb3BlcmF0'
        'aW9uSWQSGQoIc3BhY2VfaWQYAyABKAlSB3NwYWNlSWQSKgoEY2hhdBgEIAEoCzIWLnZvaWNlLm'
        'NoYXQudjEuQ2hhdFJlZlIEY2hhdBIqChF2aWV3ZXJfcHJvZmlsZV9pZBgFIAEoCVIPdmlld2Vy'
        'UHJvZmlsZUlk');

@$core.Deprecated('Use resolveSpaceReportReadContextResponseDescriptor instead')
const ResolveSpaceReportReadContextResponse$json = {
  '1': 'ResolveSpaceReportReadContextResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'chat',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'viewer_profile_id', '3': 5, '4': 1, '5': 9, '10': 'viewerProfileId'},
    {'1': 'eligible', '3': 6, '4': 1, '5': 8, '10': 'eligible'},
    {
      '1': 'space_authority_revision',
      '3': 7,
      '4': 1,
      '5': 4,
      '10': 'spaceAuthorityRevision'
    },
    {
      '1': 'lifecycle_generation',
      '3': 8,
      '4': 1,
      '5': 4,
      '10': 'lifecycleGeneration'
    },
    {
      '1': 'tree_attachment_incarnation',
      '3': 9,
      '4': 1,
      '5': 9,
      '10': 'treeAttachmentIncarnation'
    },
    {
      '1': 'membership_binding',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.MembershipBinding',
      '10': 'membershipBinding'
    },
    {'1': 'history_known', '3': 11, '4': 1, '5': 8, '10': 'historyKnown'},
    {
      '1': 'history_visible_since',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'historyVisibleSince'
    },
    {
      '1': 'effective_episode_id',
      '3': 13,
      '4': 1,
      '5': 9,
      '10': 'effectiveEpisodeId'
    },
    {
      '1': 'effective_episode_generation',
      '3': 14,
      '4': 1,
      '5': 4,
      '10': 'effectiveEpisodeGeneration'
    },
    {'1': 'ledger_revision', '3': 15, '4': 1, '5': 4, '10': 'ledgerRevision'},
    {
      '1': 'evaluated_at',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'evaluatedAt'
    },
    {
      '1': 'validity_remaining_ms',
      '3': 17,
      '4': 1,
      '5': 13,
      '10': 'validityRemainingMs'
    },
  ],
};

/// Descriptor for `ResolveSpaceReportReadContextResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveSpaceReportReadContextResponseDescriptor = $convert.base64Decode(
    'CiVSZXNvbHZlU3BhY2VSZXBvcnRSZWFkQ29udGV4dFJlc3BvbnNlEikKEHByb3RvY29sX3Zlcn'
    'Npb24YASABKA1SD3Byb3RvY29sVmVyc2lvbhIhCgxvcGVyYXRpb25faWQYAiABKAlSC29wZXJh'
    'dGlvbklkEhkKCHNwYWNlX2lkGAMgASgJUgdzcGFjZUlkEioKBGNoYXQYBCABKAsyFi52b2ljZS'
    '5jaGF0LnYxLkNoYXRSZWZSBGNoYXQSKgoRdmlld2VyX3Byb2ZpbGVfaWQYBSABKAlSD3ZpZXdl'
    'clByb2ZpbGVJZBIaCghlbGlnaWJsZRgGIAEoCFIIZWxpZ2libGUSOAoYc3BhY2VfYXV0aG9yaX'
    'R5X3JldmlzaW9uGAcgASgEUhZzcGFjZUF1dGhvcml0eVJldmlzaW9uEjEKFGxpZmVjeWNsZV9n'
    'ZW5lcmF0aW9uGAggASgEUhNsaWZlY3ljbGVHZW5lcmF0aW9uEj4KG3RyZWVfYXR0YWNobWVudF'
    '9pbmNhcm5hdGlvbhgJIAEoCVIZdHJlZUF0dGFjaG1lbnRJbmNhcm5hdGlvbhJPChJtZW1iZXJz'
    'aGlwX2JpbmRpbmcYCiABKAsyIC52b2ljZS5yb2xlLnYxLk1lbWJlcnNoaXBCaW5kaW5nUhFtZW'
    '1iZXJzaGlwQmluZGluZxIjCg1oaXN0b3J5X2tub3duGAsgASgIUgxoaXN0b3J5S25vd24STgoV'
    'aGlzdG9yeV92aXNpYmxlX3NpbmNlGAwgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcF'
    'ITaGlzdG9yeVZpc2libGVTaW5jZRIwChRlZmZlY3RpdmVfZXBpc29kZV9pZBgNIAEoCVISZWZm'
    'ZWN0aXZlRXBpc29kZUlkEkAKHGVmZmVjdGl2ZV9lcGlzb2RlX2dlbmVyYXRpb24YDiABKARSGm'
    'VmZmVjdGl2ZUVwaXNvZGVHZW5lcmF0aW9uEicKD2xlZGdlcl9yZXZpc2lvbhgPIAEoBFIObGVk'
    'Z2VyUmV2aXNpb24SPQoMZXZhbHVhdGVkX2F0GBAgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbW'
    'VzdGFtcFILZXZhbHVhdGVkQXQSMgoVdmFsaWRpdHlfcmVtYWluaW5nX21zGBEgASgNUhN2YWxp'
    'ZGl0eVJlbWFpbmluZ01z');
