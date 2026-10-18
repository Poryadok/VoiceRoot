// This is a generated file - do not edit.
//
// Generated from voice/role/v1/member_admission.proto.

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

@$core.Deprecated('Use membershipBindingDescriptor instead')
const MembershipBinding$json = {
  '1': 'MembershipBinding',
  '2': [
    {
      '1': 'ordinary',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OrdinaryMembershipBinding',
      '9': 0,
      '10': 'ordinary'
    },
    {
      '1': 'community',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.CommunityMembershipBinding',
      '9': 0,
      '10': 'community'
    },
  ],
  '8': [
    {'1': 'source'},
  ],
};

/// Descriptor for `MembershipBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List membershipBindingDescriptor = $convert.base64Decode(
    'ChFNZW1iZXJzaGlwQmluZGluZxJGCghvcmRpbmFyeRgBIAEoCzIoLnZvaWNlLnJvbGUudjEuT3'
    'JkaW5hcnlNZW1iZXJzaGlwQmluZGluZ0gAUghvcmRpbmFyeRJJCgljb21tdW5pdHkYAiABKAsy'
    'KS52b2ljZS5yb2xlLnYxLkNvbW11bml0eU1lbWJlcnNoaXBCaW5kaW5nSABSCWNvbW11bml0eU'
    'IICgZzb3VyY2U=');

@$core.Deprecated('Use ordinaryMembershipBindingDescriptor instead')
const OrdinaryMembershipBinding$json = {
  '1': 'OrdinaryMembershipBinding',
  '2': [
    {
      '1': 'membership_generation',
      '3': 1,
      '4': 1,
      '5': 4,
      '10': 'membershipGeneration'
    },
    {
      '1': 'publication_identity',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'publicationIdentity'
    },
    {'1': 'legacy', '3': 3, '4': 1, '5': 8, '10': 'legacy'},
  ],
};

/// Descriptor for `OrdinaryMembershipBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ordinaryMembershipBindingDescriptor = $convert.base64Decode(
    'ChlPcmRpbmFyeU1lbWJlcnNoaXBCaW5kaW5nEjMKFW1lbWJlcnNoaXBfZ2VuZXJhdGlvbhgBIA'
    'EoBFIUbWVtYmVyc2hpcEdlbmVyYXRpb24SMQoUcHVibGljYXRpb25faWRlbnRpdHkYAiABKAlS'
    'E3B1YmxpY2F0aW9uSWRlbnRpdHkSFgoGbGVnYWN5GAMgASgIUgZsZWdhY3k=');

@$core.Deprecated('Use communityMembershipBindingDescriptor instead')
const CommunityMembershipBinding$json = {
  '1': 'CommunityMembershipBinding',
  '2': [
    {'1': 'owner_generation', '3': 1, '4': 1, '5': 4, '10': 'ownerGeneration'},
    {
      '1': 'membership_generation',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'membershipGeneration'
    },
    {
      '1': 'membership_episode_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'membershipEpisodeId'
    },
  ],
};

/// Descriptor for `CommunityMembershipBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List communityMembershipBindingDescriptor = $convert.base64Decode(
    'ChpDb21tdW5pdHlNZW1iZXJzaGlwQmluZGluZxIpChBvd25lcl9nZW5lcmF0aW9uGAEgASgEUg'
    '9vd25lckdlbmVyYXRpb24SMwoVbWVtYmVyc2hpcF9nZW5lcmF0aW9uGAIgASgEUhRtZW1iZXJz'
    'aGlwR2VuZXJhdGlvbhIyChVtZW1iZXJzaGlwX2VwaXNvZGVfaWQYAyABKAlSE21lbWJlcnNoaX'
    'BFcGlzb2RlSWQ=');
