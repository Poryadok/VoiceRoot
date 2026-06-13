// This is a generated file - do not edit.
//
// Generated from voice/s2s/v1/federation_management.proto.

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

@$core.Deprecated('Use federationNodeRegistrationStatusDescriptor instead')
const FederationNodeRegistrationStatus$json = {
  '1': 'FederationNodeRegistrationStatus',
  '2': [
    {'1': 'FEDERATION_NODE_REGISTRATION_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'FEDERATION_NODE_REGISTRATION_STATUS_PENDING', '2': 1},
    {'1': 'FEDERATION_NODE_REGISTRATION_STATUS_ACTIVE', '2': 2},
    {'1': 'FEDERATION_NODE_REGISTRATION_STATUS_SUSPENDED', '2': 3},
    {'1': 'FEDERATION_NODE_REGISTRATION_STATUS_DEFEDERATED', '2': 4},
  ],
};

/// Descriptor for `FederationNodeRegistrationStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List federationNodeRegistrationStatusDescriptor = $convert.base64Decode(
    'CiBGZWRlcmF0aW9uTm9kZVJlZ2lzdHJhdGlvblN0YXR1cxIzCi9GRURFUkFUSU9OX05PREVfUk'
    'VHSVNUUkFUSU9OX1NUQVRVU19VTlNQRUNJRklFRBAAEi8KK0ZFREVSQVRJT05fTk9ERV9SRUdJ'
    'U1RSQVRJT05fU1RBVFVTX1BFTkRJTkcQARIuCipGRURFUkFUSU9OX05PREVfUkVHSVNUUkFUSU'
    '9OX1NUQVRVU19BQ1RJVkUQAhIxCi1GRURFUkFUSU9OX05PREVfUkVHSVNUUkFUSU9OX1NUQVRV'
    'U19TVVNQRU5ERUQQAxIzCi9GRURFUkFUSU9OX05PREVfUkVHSVNUUkFUSU9OX1NUQVRVU19ERU'
    'ZFREVSQVRFRBAE');

@$core.Deprecated('Use federationNodeDescriptor instead')
const FederationNode$json = {
  '1': 'FederationNode',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'host', '3': 3, '4': 1, '5': 9, '10': 'host'},
    {'1': 'port', '3': 4, '4': 1, '5': 5, '10': 'port'},
    {'1': 'description', '3': 5, '4': 1, '5': 9, '10': 'description'},
    {'1': 'status', '3': 6, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'tls_cert_fingerprint',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'tlsCertFingerprint',
      '17': true
    },
    {
      '1': 'last_heartbeat_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'lastHeartbeatAt',
      '17': true
    },
    {
      '1': 'last_sync_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 2,
      '10': 'lastSyncAt',
      '17': true
    },
    {
      '1': 'registered_at',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'registeredAt'
    },
    {
      '1': 'approved_at',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 3,
      '10': 'approvedAt',
      '17': true
    },
    {
      '1': 'approved_by_profile_id',
      '3': 12,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'approvedByProfileId',
      '17': true
    },
    {
      '1': 'defederated_at',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 5,
      '10': 'defederatedAt',
      '17': true
    },
    {
      '1': 'status_enum',
      '3': 14,
      '4': 1,
      '5': 14,
      '6': '.voice.s2s.v1.FederationNodeRegistrationStatus',
      '9': 6,
      '10': 'statusEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_tls_cert_fingerprint'},
    {'1': '_last_heartbeat_at'},
    {'1': '_last_sync_at'},
    {'1': '_approved_at'},
    {'1': '_approved_by_profile_id'},
    {'1': '_defederated_at'},
    {'1': '_status_enum'},
  ],
};

/// Descriptor for `FederationNode`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List federationNodeDescriptor = $convert.base64Decode(
    'Cg5GZWRlcmF0aW9uTm9kZRIOCgJpZBgBIAEoCVICaWQSEgoEbmFtZRgCIAEoCVIEbmFtZRISCg'
    'Rob3N0GAMgASgJUgRob3N0EhIKBHBvcnQYBCABKAVSBHBvcnQSIAoLZGVzY3JpcHRpb24YBSAB'
    'KAlSC2Rlc2NyaXB0aW9uEhYKBnN0YXR1cxgGIAEoCVIGc3RhdHVzEjUKFHRsc19jZXJ0X2Zpbm'
    'dlcnByaW50GAcgASgJSABSEnRsc0NlcnRGaW5nZXJwcmludIgBARJLChFsYXN0X2hlYXJ0YmVh'
    'dF9hdBgIIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBIAVIPbGFzdEhlYXJ0YmVhdE'
    'F0iAEBEkEKDGxhc3Rfc3luY19hdBgJIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBI'
    'AlIKbGFzdFN5bmNBdIgBARI/Cg1yZWdpc3RlcmVkX2F0GAogASgLMhouZ29vZ2xlLnByb3RvYn'
    'VmLlRpbWVzdGFtcFIMcmVnaXN0ZXJlZEF0EkAKC2FwcHJvdmVkX2F0GAsgASgLMhouZ29vZ2xl'
    'LnByb3RvYnVmLlRpbWVzdGFtcEgDUgphcHByb3ZlZEF0iAEBEjgKFmFwcHJvdmVkX2J5X3Byb2'
    'ZpbGVfaWQYDCABKAlIBFITYXBwcm92ZWRCeVByb2ZpbGVJZIgBARJGCg5kZWZlZGVyYXRlZF9h'
    'dBgNIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBIBVINZGVmZWRlcmF0ZWRBdIgBAR'
    'JUCgtzdGF0dXNfZW51bRgOIAEoDjIuLnZvaWNlLnMycy52MS5GZWRlcmF0aW9uTm9kZVJlZ2lz'
    'dHJhdGlvblN0YXR1c0gGUgpzdGF0dXNFbnVtiAEBQhcKFV90bHNfY2VydF9maW5nZXJwcmludE'
    'IUChJfbGFzdF9oZWFydGJlYXRfYXRCDwoNX2xhc3Rfc3luY19hdEIOCgxfYXBwcm92ZWRfYXRC'
    'GQoXX2FwcHJvdmVkX2J5X3Byb2ZpbGVfaWRCEQoPX2RlZmVkZXJhdGVkX2F0Qg4KDF9zdGF0dX'
    'NfZW51bQ==');

@$core.Deprecated('Use registerNodeRequestDescriptor instead')
const RegisterNodeRequest$json = {
  '1': 'RegisterNodeRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'host', '3': 2, '4': 1, '5': 9, '10': 'host'},
    {'1': 'port', '3': 3, '4': 1, '5': 5, '10': 'port'},
    {'1': 'description', '3': 4, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'tls_cert_fingerprint',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'tlsCertFingerprint'
    },
  ],
};

/// Descriptor for `RegisterNodeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerNodeRequestDescriptor = $convert.base64Decode(
    'ChNSZWdpc3Rlck5vZGVSZXF1ZXN0EhIKBG5hbWUYASABKAlSBG5hbWUSEgoEaG9zdBgCIAEoCV'
    'IEaG9zdBISCgRwb3J0GAMgASgFUgRwb3J0EiAKC2Rlc2NyaXB0aW9uGAQgASgJUgtkZXNjcmlw'
    'dGlvbhIwChR0bHNfY2VydF9maW5nZXJwcmludBgFIAEoCVISdGxzQ2VydEZpbmdlcnByaW50');

@$core.Deprecated('Use approveNodeRequestDescriptor instead')
const ApproveNodeRequest$json = {
  '1': 'ApproveNodeRequest',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {
      '1': 'approver_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'approverProfileId'
    },
  ],
};

/// Descriptor for `ApproveNodeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List approveNodeRequestDescriptor = $convert.base64Decode(
    'ChJBcHByb3ZlTm9kZVJlcXVlc3QSFwoHbm9kZV9pZBgBIAEoCVIGbm9kZUlkEi4KE2FwcHJvdm'
    'VyX3Byb2ZpbGVfaWQYAiABKAlSEWFwcHJvdmVyUHJvZmlsZUlk');

@$core.Deprecated('Use deactivateNodeRequestDescriptor instead')
const DeactivateNodeRequest$json = {
  '1': 'DeactivateNodeRequest',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
  ],
};

/// Descriptor for `DeactivateNodeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deactivateNodeRequestDescriptor =
    $convert.base64Decode(
        'ChVEZWFjdGl2YXRlTm9kZVJlcXVlc3QSFwoHbm9kZV9pZBgBIAEoCVIGbm9kZUlk');

@$core.Deprecated('Use listNodesRequestDescriptor instead')
const ListNodesRequest$json = {
  '1': 'ListNodesRequest',
  '2': [
    {
      '1': 'status_filter',
      '3': 1,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'statusFilter',
      '17': true
    },
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 3, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'status_filter_enum',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.s2s.v1.FederationNodeRegistrationStatus',
      '9': 1,
      '10': 'statusFilterEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_status_filter'},
    {'1': '_status_filter_enum'},
  ],
};

/// Descriptor for `ListNodesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listNodesRequestDescriptor = $convert.base64Decode(
    'ChBMaXN0Tm9kZXNSZXF1ZXN0EigKDXN0YXR1c19maWx0ZXIYASABKAlIAFIMc3RhdHVzRmlsdG'
    'VyiAEBEhsKCXBhZ2Vfc2l6ZRgCIAEoBVIIcGFnZVNpemUSHQoKcGFnZV90b2tlbhgDIAEoCVIJ'
    'cGFnZVRva2VuEmEKEnN0YXR1c19maWx0ZXJfZW51bRgEIAEoDjIuLnZvaWNlLnMycy52MS5GZW'
    'RlcmF0aW9uTm9kZVJlZ2lzdHJhdGlvblN0YXR1c0gBUhBzdGF0dXNGaWx0ZXJFbnVtiAEBQhAK'
    'Dl9zdGF0dXNfZmlsdGVyQhUKE19zdGF0dXNfZmlsdGVyX2VudW0=');

@$core.Deprecated('Use federationNodeListDescriptor instead')
const FederationNodeList$json = {
  '1': 'FederationNodeList',
  '2': [
    {
      '1': 'nodes',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.s2s.v1.FederationNode',
      '10': 'nodes'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `FederationNodeList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List federationNodeListDescriptor = $convert.base64Decode(
    'ChJGZWRlcmF0aW9uTm9kZUxpc3QSMgoFbm9kZXMYASADKAsyHC52b2ljZS5zMnMudjEuRmVkZX'
    'JhdGlvbk5vZGVSBW5vZGVzEiYKD25leHRfcGFnZV90b2tlbhgCIAEoCVINbmV4dFBhZ2VUb2tl'
    'bg==');

@$core.Deprecated('Use getNodeStatusRequestDescriptor instead')
const GetNodeStatusRequest$json = {
  '1': 'GetNodeStatusRequest',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
  ],
};

/// Descriptor for `GetNodeStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getNodeStatusRequestDescriptor =
    $convert.base64Decode(
        'ChRHZXROb2RlU3RhdHVzUmVxdWVzdBIXCgdub2RlX2lkGAEgASgJUgZub2RlSWQ=');

@$core.Deprecated('Use federationNodeStatusDescriptor instead')
const FederationNodeStatus$json = {
  '1': 'FederationNodeStatus',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'status', '3': 2, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'last_heartbeat_at',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'lastHeartbeatAt',
      '17': true
    },
    {
      '1': 'last_sync_at',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'lastSyncAt',
      '17': true
    },
    {
      '1': 'status_enum',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.voice.s2s.v1.FederationNodeRegistrationStatus',
      '9': 2,
      '10': 'statusEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_last_heartbeat_at'},
    {'1': '_last_sync_at'},
    {'1': '_status_enum'},
  ],
};

/// Descriptor for `FederationNodeStatus`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List federationNodeStatusDescriptor = $convert.base64Decode(
    'ChRGZWRlcmF0aW9uTm9kZVN0YXR1cxIXCgdub2RlX2lkGAEgASgJUgZub2RlSWQSFgoGc3RhdH'
    'VzGAIgASgJUgZzdGF0dXMSSwoRbGFzdF9oZWFydGJlYXRfYXQYAyABKAsyGi5nb29nbGUucHJv'
    'dG9idWYuVGltZXN0YW1wSABSD2xhc3RIZWFydGJlYXRBdIgBARJBCgxsYXN0X3N5bmNfYXQYBC'
    'ABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSAFSCmxhc3RTeW5jQXSIAQESVAoLc3Rh'
    'dHVzX2VudW0YBSABKA4yLi52b2ljZS5zMnMudjEuRmVkZXJhdGlvbk5vZGVSZWdpc3RyYXRpb2'
    '5TdGF0dXNIAlIKc3RhdHVzRW51bYgBAUIUChJfbGFzdF9oZWFydGJlYXRfYXRCDwoNX2xhc3Rf'
    'c3luY19hdEIOCgxfc3RhdHVzX2VudW0=');

@$core.Deprecated('Use defederateRequestDescriptor instead')
const DefederateRequest$json = {
  '1': 'DefederateRequest',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `DefederateRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List defederateRequestDescriptor = $convert.base64Decode(
    'ChFEZWZlZGVyYXRlUmVxdWVzdBIXCgdub2RlX2lkGAEgASgJUgZub2RlSWQSFgoGcmVhc29uGA'
    'IgASgJUgZyZWFzb24=');

@$core.Deprecated('Use registerNodeResponseDescriptor instead')
const RegisterNodeResponse$json = {
  '1': 'RegisterNodeResponse',
  '2': [
    {
      '1': 'federation_node',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.FederationNode',
      '10': 'federationNode'
    },
  ],
};

/// Descriptor for `RegisterNodeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerNodeResponseDescriptor = $convert.base64Decode(
    'ChRSZWdpc3Rlck5vZGVSZXNwb25zZRJFCg9mZWRlcmF0aW9uX25vZGUYASABKAsyHC52b2ljZS'
    '5zMnMudjEuRmVkZXJhdGlvbk5vZGVSDmZlZGVyYXRpb25Ob2Rl');

@$core.Deprecated('Use approveNodeResponseDescriptor instead')
const ApproveNodeResponse$json = {
  '1': 'ApproveNodeResponse',
  '2': [
    {
      '1': 'federation_node',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.FederationNode',
      '10': 'federationNode'
    },
  ],
};

/// Descriptor for `ApproveNodeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List approveNodeResponseDescriptor = $convert.base64Decode(
    'ChNBcHByb3ZlTm9kZVJlc3BvbnNlEkUKD2ZlZGVyYXRpb25fbm9kZRgBIAEoCzIcLnZvaWNlLn'
    'Mycy52MS5GZWRlcmF0aW9uTm9kZVIOZmVkZXJhdGlvbk5vZGU=');

@$core.Deprecated('Use deactivateNodeResponseDescriptor instead')
const DeactivateNodeResponse$json = {
  '1': 'DeactivateNodeResponse',
};

/// Descriptor for `DeactivateNodeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deactivateNodeResponseDescriptor =
    $convert.base64Decode('ChZEZWFjdGl2YXRlTm9kZVJlc3BvbnNl');

@$core.Deprecated('Use listNodesResponseDescriptor instead')
const ListNodesResponse$json = {
  '1': 'ListNodesResponse',
  '2': [
    {
      '1': 'federation_node_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.FederationNodeList',
      '10': 'federationNodeList'
    },
  ],
};

/// Descriptor for `ListNodesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listNodesResponseDescriptor = $convert.base64Decode(
    'ChFMaXN0Tm9kZXNSZXNwb25zZRJSChRmZWRlcmF0aW9uX25vZGVfbGlzdBgBIAEoCzIgLnZvaW'
    'NlLnMycy52MS5GZWRlcmF0aW9uTm9kZUxpc3RSEmZlZGVyYXRpb25Ob2RlTGlzdA==');

@$core.Deprecated('Use getNodeStatusResponseDescriptor instead')
const GetNodeStatusResponse$json = {
  '1': 'GetNodeStatusResponse',
  '2': [
    {
      '1': 'federation_node_status',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.FederationNodeStatus',
      '10': 'federationNodeStatus'
    },
  ],
};

/// Descriptor for `GetNodeStatusResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getNodeStatusResponseDescriptor = $convert.base64Decode(
    'ChVHZXROb2RlU3RhdHVzUmVzcG9uc2USWAoWZmVkZXJhdGlvbl9ub2RlX3N0YXR1cxgBIAEoCz'
    'IiLnZvaWNlLnMycy52MS5GZWRlcmF0aW9uTm9kZVN0YXR1c1IUZmVkZXJhdGlvbk5vZGVTdGF0'
    'dXM=');

@$core.Deprecated('Use defederateResponseDescriptor instead')
const DefederateResponse$json = {
  '1': 'DefederateResponse',
};

/// Descriptor for `DefederateResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List defederateResponseDescriptor =
    $convert.base64Decode('ChJEZWZlZGVyYXRlUmVzcG9uc2U=');
