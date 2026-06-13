// This is a generated file - do not edit.
//
// Generated from voice/s2s/v1/s2s.proto.

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

@$core.Deprecated('Use federationPushEventTypeDescriptor instead')
const FederationPushEventType$json = {
  '1': 'FederationPushEventType',
  '2': [
    {'1': 'FEDERATION_PUSH_EVENT_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'FEDERATION_PUSH_EVENT_TYPE_MENTION', '2': 1},
    {'1': 'FEDERATION_PUSH_EVENT_TYPE_DM', '2': 2},
    {'1': 'FEDERATION_PUSH_EVENT_TYPE_MATCH_FOUND', '2': 3},
    {'1': 'FEDERATION_PUSH_EVENT_TYPE_CALL_INCOMING', '2': 4},
  ],
};

/// Descriptor for `FederationPushEventType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List federationPushEventTypeDescriptor = $convert.base64Decode(
    'ChdGZWRlcmF0aW9uUHVzaEV2ZW50VHlwZRIqCiZGRURFUkFUSU9OX1BVU0hfRVZFTlRfVFlQRV'
    '9VTlNQRUNJRklFRBAAEiYKIkZFREVSQVRJT05fUFVTSF9FVkVOVF9UWVBFX01FTlRJT04QARIh'
    'Ch1GRURFUkFUSU9OX1BVU0hfRVZFTlRfVFlQRV9ETRACEioKJkZFREVSQVRJT05fUFVTSF9FVk'
    'VOVF9UWVBFX01BVENIX0ZPVU5EEAMSLAooRkVERVJBVElPTl9QVVNIX0VWRU5UX1RZUEVfQ0FM'
    'TF9JTkNPTUlORxAE');

@$core.Deprecated('Use authenticateUserRequestDescriptor instead')
const AuthenticateUserRequest$json = {
  '1': 'AuthenticateUserRequest',
  '2': [
    {'1': 'auth_token', '3': 1, '4': 1, '5': 9, '10': 'authToken'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `AuthenticateUserRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authenticateUserRequestDescriptor =
    $convert.base64Decode(
        'ChdBdXRoZW50aWNhdGVVc2VyUmVxdWVzdBIdCgphdXRoX3Rva2VuGAEgASgJUglhdXRoVG9rZW'
        '4SGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQ=');

@$core.Deprecated('Use authenticateUserResponseDescriptor instead')
const AuthenticateUserResponse$json = {
  '1': 'AuthenticateUserResponse',
  '2': [
    {'1': 'ok', '3': 1, '4': 1, '5': 8, '10': 'ok'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_ids', '3': 4, '4': 3, '5': 9, '10': 'roleIds'},
    {'1': 'expires_at', '3': 5, '4': 1, '5': 3, '10': 'expiresAt'},
    {'1': 'error', '3': 6, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `AuthenticateUserResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authenticateUserResponseDescriptor = $convert.base64Decode(
    'ChhBdXRoZW50aWNhdGVVc2VyUmVzcG9uc2USDgoCb2sYASABKAhSAm9rEh0KCmFjY291bnRfaW'
    'QYAiABKAlSCWFjY291bnRJZBIdCgpwcm9maWxlX2lkGAMgASgJUglwcm9maWxlSWQSGQoIcm9s'
    'ZV9pZHMYBCADKAlSB3JvbGVJZHMSHQoKZXhwaXJlc19hdBgFIAEoA1IJZXhwaXJlc0F0EhQKBW'
    'Vycm9yGAYgASgJUgVlcnJvcg==');

@$core.Deprecated('Use eventStreamRequestDescriptor instead')
const EventStreamRequest$json = {
  '1': 'EventStreamRequest',
  '2': [
    {
      '1': 'subscribe',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.SubscribeRequest',
      '9': 0,
      '10': 'subscribe'
    },
    {
      '1': 'heartbeat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.Heartbeat',
      '9': 0,
      '10': 'heartbeat'
    },
    {
      '1': 'ack',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.Ack',
      '9': 0,
      '10': 'ack'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `EventStreamRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List eventStreamRequestDescriptor = $convert.base64Decode(
    'ChJFdmVudFN0cmVhbVJlcXVlc3QSPgoJc3Vic2NyaWJlGAEgASgLMh4udm9pY2UuczJzLnYxLl'
    'N1YnNjcmliZVJlcXVlc3RIAFIJc3Vic2NyaWJlEjcKCWhlYXJ0YmVhdBgCIAEoCzIXLnZvaWNl'
    'LnMycy52MS5IZWFydGJlYXRIAFIJaGVhcnRiZWF0EiUKA2FjaxgDIAEoCzIRLnZvaWNlLnMycy'
    '52MS5BY2tIAFIDYWNrQgkKB3BheWxvYWQ=');

@$core.Deprecated('Use subscribeRequestDescriptor instead')
const SubscribeRequest$json = {
  '1': 'SubscribeRequest',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'node_secret', '3': 2, '4': 1, '5': 9, '10': 'nodeSecret'},
    {'1': 'space_ids', '3': 3, '4': 3, '5': 9, '10': 'spaceIds'},
  ],
};

/// Descriptor for `SubscribeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List subscribeRequestDescriptor = $convert.base64Decode(
    'ChBTdWJzY3JpYmVSZXF1ZXN0EhcKB25vZGVfaWQYASABKAlSBm5vZGVJZBIfCgtub2RlX3NlY3'
    'JldBgCIAEoCVIKbm9kZVNlY3JldBIbCglzcGFjZV9pZHMYAyADKAlSCHNwYWNlSWRz');

@$core.Deprecated('Use heartbeatDescriptor instead')
const Heartbeat$json = {
  '1': 'Heartbeat',
  '2': [
    {'1': 'timestamp', '3': 1, '4': 1, '5': 3, '10': 'timestamp'},
  ],
};

/// Descriptor for `Heartbeat`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List heartbeatDescriptor = $convert
    .base64Decode('CglIZWFydGJlYXQSHAoJdGltZXN0YW1wGAEgASgDUgl0aW1lc3RhbXA=');

@$core.Deprecated('Use ackDescriptor instead')
const Ack$json = {
  '1': 'Ack',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
  ],
};

/// Descriptor for `Ack`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ackDescriptor =
    $convert.base64Decode('CgNBY2sSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW50SWQ=');

@$core.Deprecated('Use eventStreamResponseDescriptor instead')
const EventStreamResponse$json = {
  '1': 'EventStreamResponse',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {'1': 'timestamp', '3': 2, '4': 1, '5': 3, '10': 'timestamp'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'role_changed',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.RoleChangedEvent',
      '9': 0,
      '10': 'roleChanged'
    },
    {
      '1': 'user_banned',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.UserBannedEvent',
      '9': 0,
      '10': 'userBanned'
    },
    {
      '1': 'user_unbanned',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.UserUnbannedEvent',
      '9': 0,
      '10': 'userUnbanned'
    },
    {
      '1': 'defederated',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.DefederatedEvent',
      '9': 0,
      '10': 'defederated'
    },
    {
      '1': 'space_deleted',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.voice.s2s.v1.SpaceDeletedEvent',
      '9': 0,
      '10': 'spaceDeleted'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `EventStreamResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List eventStreamResponseDescriptor = $convert.base64Decode(
    'ChNFdmVudFN0cmVhbVJlc3BvbnNlEhkKCGV2ZW50X2lkGAEgASgJUgdldmVudElkEhwKCXRpbW'
    'VzdGFtcBgCIAEoA1IJdGltZXN0YW1wEhkKCHNwYWNlX2lkGAMgASgJUgdzcGFjZUlkEkMKDHJv'
    'bGVfY2hhbmdlZBgKIAEoCzIeLnZvaWNlLnMycy52MS5Sb2xlQ2hhbmdlZEV2ZW50SABSC3JvbG'
    'VDaGFuZ2VkEkAKC3VzZXJfYmFubmVkGAsgASgLMh0udm9pY2UuczJzLnYxLlVzZXJCYW5uZWRF'
    'dmVudEgAUgp1c2VyQmFubmVkEkYKDXVzZXJfdW5iYW5uZWQYDCABKAsyHy52b2ljZS5zMnMudj'
    'EuVXNlclVuYmFubmVkRXZlbnRIAFIMdXNlclVuYmFubmVkEkIKC2RlZmVkZXJhdGVkGA0gASgL'
    'Mh4udm9pY2UuczJzLnYxLkRlZmVkZXJhdGVkRXZlbnRIAFILZGVmZWRlcmF0ZWQSRgoNc3BhY2'
    'VfZGVsZXRlZBgOIAEoCzIfLnZvaWNlLnMycy52MS5TcGFjZURlbGV0ZWRFdmVudEgAUgxzcGFj'
    'ZURlbGV0ZWRCCQoHcGF5bG9hZA==');

@$core.Deprecated('Use roleChangedEventDescriptor instead')
const RoleChangedEvent$json = {
  '1': 'RoleChangedEvent',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'added_roles', '3': 3, '4': 3, '5': 9, '10': 'addedRoles'},
    {'1': 'removed_roles', '3': 4, '4': 3, '5': 9, '10': 'removedRoles'},
  ],
};

/// Descriptor for `RoleChangedEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleChangedEventDescriptor = $convert.base64Decode(
    'ChBSb2xlQ2hhbmdlZEV2ZW50Eh0KCmFjY291bnRfaWQYASABKAlSCWFjY291bnRJZBIdCgpwcm'
    '9maWxlX2lkGAIgASgJUglwcm9maWxlSWQSHwoLYWRkZWRfcm9sZXMYAyADKAlSCmFkZGVkUm9s'
    'ZXMSIwoNcmVtb3ZlZF9yb2xlcxgEIAMoCVIMcmVtb3ZlZFJvbGVz');

@$core.Deprecated('Use userBannedEventDescriptor instead')
const UserBannedEvent$json = {
  '1': 'UserBannedEvent',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'reason', '3': 3, '4': 1, '5': 9, '10': 'reason'},
    {'1': 'banned_until', '3': 4, '4': 1, '5': 3, '10': 'bannedUntil'},
  ],
};

/// Descriptor for `UserBannedEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userBannedEventDescriptor = $convert.base64Decode(
    'Cg9Vc2VyQmFubmVkRXZlbnQSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudElkEh0KCnByb2'
    'ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZBIWCgZyZWFzb24YAyABKAlSBnJlYXNvbhIhCgxiYW5u'
    'ZWRfdW50aWwYBCABKANSC2Jhbm5lZFVudGls');

@$core.Deprecated('Use userUnbannedEventDescriptor instead')
const UserUnbannedEvent$json = {
  '1': 'UserUnbannedEvent',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `UserUnbannedEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userUnbannedEventDescriptor = $convert.base64Decode(
    'ChFVc2VyVW5iYW5uZWRFdmVudBIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW50SWQSHQoKcH'
    'JvZmlsZV9pZBgCIAEoCVIJcHJvZmlsZUlk');

@$core.Deprecated('Use defederatedEventDescriptor instead')
const DefederatedEvent$json = {
  '1': 'DefederatedEvent',
  '2': [
    {'1': 'reason', '3': 1, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `DefederatedEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List defederatedEventDescriptor = $convert
    .base64Decode('ChBEZWZlZGVyYXRlZEV2ZW50EhYKBnJlYXNvbhgBIAEoCVIGcmVhc29u');

@$core.Deprecated('Use spaceDeletedEventDescriptor instead')
const SpaceDeletedEvent$json = {
  '1': 'SpaceDeletedEvent',
};

/// Descriptor for `SpaceDeletedEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceDeletedEventDescriptor =
    $convert.base64Decode('ChFTcGFjZURlbGV0ZWRFdmVudA==');

@$core.Deprecated('Use syncSnapshotRequestDescriptor instead')
const SyncSnapshotRequest$json = {
  '1': 'SyncSnapshotRequest',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `SyncSnapshotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List syncSnapshotRequestDescriptor = $convert.base64Decode(
    'ChNTeW5jU25hcHNob3RSZXF1ZXN0EhcKB25vZGVfaWQYASABKAlSBm5vZGVJZBIZCghzcGFjZV'
    '9pZBgCIAEoCVIHc3BhY2VJZA==');

@$core.Deprecated('Use syncSnapshotResponseDescriptor instead')
const SyncSnapshotResponse$json = {
  '1': 'SyncSnapshotResponse',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'roles',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.s2s.v1.RoleEntry',
      '10': 'roles'
    },
    {
      '1': 'bans',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.voice.s2s.v1.BanEntry',
      '10': 'bans'
    },
  ],
};

/// Descriptor for `SyncSnapshotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List syncSnapshotResponseDescriptor = $convert.base64Decode(
    'ChRTeW5jU25hcHNob3RSZXNwb25zZRIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBItCgVyb2'
    'xlcxgCIAMoCzIXLnZvaWNlLnMycy52MS5Sb2xlRW50cnlSBXJvbGVzEioKBGJhbnMYAyADKAsy'
    'Fi52b2ljZS5zMnMudjEuQmFuRW50cnlSBGJhbnM=');

@$core.Deprecated('Use roleEntryDescriptor instead')
const RoleEntry$json = {
  '1': 'RoleEntry',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_ids', '3': 3, '4': 3, '5': 9, '10': 'roleIds'},
  ],
};

/// Descriptor for `RoleEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleEntryDescriptor = $convert.base64Decode(
    'CglSb2xlRW50cnkSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudElkEh0KCnByb2ZpbGVfaW'
    'QYAiABKAlSCXByb2ZpbGVJZBIZCghyb2xlX2lkcxgDIAMoCVIHcm9sZUlkcw==');

@$core.Deprecated('Use banEntryDescriptor instead')
const BanEntry$json = {
  '1': 'BanEntry',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'banned_until', '3': 3, '4': 1, '5': 3, '10': 'bannedUntil'},
  ],
};

/// Descriptor for `BanEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List banEntryDescriptor = $convert.base64Decode(
    'CghCYW5FbnRyeRIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW50SWQSHQoKcHJvZmlsZV9pZB'
    'gCIAEoCVIJcHJvZmlsZUlkEiEKDGJhbm5lZF91bnRpbBgDIAEoA1ILYmFubmVkVW50aWw=');

@$core.Deprecated('Use notifyUserRequestDescriptor instead')
const NotifyUserRequest$json = {
  '1': 'NotifyUserRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'type', '3': 3, '4': 1, '5': 9, '10': 'type'},
    {'1': 'preview', '3': 4, '4': 1, '5': 9, '10': 'preview'},
    {'1': 'deep_link', '3': 5, '4': 1, '5': 9, '10': 'deepLink'},
    {
      '1': 'type_enum',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.s2s.v1.FederationPushEventType',
      '9': 0,
      '10': 'typeEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_type_enum'},
  ],
};

/// Descriptor for `NotifyUserRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List notifyUserRequestDescriptor = $convert.base64Decode(
    'ChFOb3RpZnlVc2VyUmVxdWVzdBIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW50SWQSGQoIc3'
    'BhY2VfaWQYAiABKAlSB3NwYWNlSWQSEgoEdHlwZRgDIAEoCVIEdHlwZRIYCgdwcmV2aWV3GAQg'
    'ASgJUgdwcmV2aWV3EhsKCWRlZXBfbGluaxgFIAEoCVIIZGVlcExpbmsSRwoJdHlwZV9lbnVtGA'
    'YgASgOMiUudm9pY2UuczJzLnYxLkZlZGVyYXRpb25QdXNoRXZlbnRUeXBlSABSCHR5cGVFbnVt'
    'iAEBQgwKCl90eXBlX2VudW0=');

@$core.Deprecated('Use notifyUserResponseDescriptor instead')
const NotifyUserResponse$json = {
  '1': 'NotifyUserResponse',
  '2': [
    {'1': 'accepted', '3': 1, '4': 1, '5': 8, '10': 'accepted'},
    {'1': 'error', '3': 2, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `NotifyUserResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List notifyUserResponseDescriptor = $convert.base64Decode(
    'ChJOb3RpZnlVc2VyUmVzcG9uc2USGgoIYWNjZXB0ZWQYASABKAhSCGFjY2VwdGVkEhQKBWVycm'
    '9yGAIgASgJUgVlcnJvcg==');
