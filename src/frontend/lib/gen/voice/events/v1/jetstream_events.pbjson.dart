// This is a generated file - do not edit.
//
// Generated from voice/events/v1/jetstream_events.proto.

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

@$core.Deprecated('Use userStreamEventDescriptor instead')
const UserStreamEvent$json = {
  '1': 'UserStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'user_registered',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.UserRegistered',
      '9': 0,
      '10': 'userRegistered'
    },
    {
      '1': 'user_logged_in',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.UserLoggedIn',
      '9': 0,
      '10': 'userLoggedIn'
    },
    {
      '1': 'user_logged_out',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.UserLoggedOut',
      '9': 0,
      '10': 'userLoggedOut'
    },
    {
      '1': 'user_2fa_enabled',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.UserTwoFaEnabled',
      '9': 0,
      '10': 'user2faEnabled'
    },
    {
      '1': 'user_guest_converted',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.UserGuestConverted',
      '9': 0,
      '10': 'userGuestConverted'
    },
    {
      '1': 'user_account_deleted',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.UserAccountDeleted',
      '9': 0,
      '10': 'userAccountDeleted'
    },
    {
      '1': 'user_account_restored',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.UserAccountRestored',
      '9': 0,
      '10': 'userAccountRestored'
    },
    {
      '1': 'profile_created',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ProfileCreated',
      '9': 0,
      '10': 'profileCreated'
    },
    {
      '1': 'profile_switched',
      '3': 18,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ProfileSwitched',
      '9': 0,
      '10': 'profileSwitched'
    },
    {
      '1': 'settings_changed',
      '3': 19,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.SettingsChanged',
      '9': 0,
      '10': 'settingsChanged'
    },
    {
      '1': 'presence_change',
      '3': 20,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.PresenceChange',
      '9': 0,
      '10': 'presenceChange'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `UserStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userStreamEventDescriptor = $convert.base64Decode(
    'Cg9Vc2VyU3RyZWFtRXZlbnQSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW50SWQSOwoLb2NjdXJyZW'
    'RfYXQYAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpvY2N1cnJlZEF0EkoKD3Vz'
    'ZXJfcmVnaXN0ZXJlZBgKIAEoCzIfLnZvaWNlLmV2ZW50cy52MS5Vc2VyUmVnaXN0ZXJlZEgAUg'
    '51c2VyUmVnaXN0ZXJlZBJFCg51c2VyX2xvZ2dlZF9pbhgLIAEoCzIdLnZvaWNlLmV2ZW50cy52'
    'MS5Vc2VyTG9nZ2VkSW5IAFIMdXNlckxvZ2dlZEluEkgKD3VzZXJfbG9nZ2VkX291dBgMIAEoCz'
    'IeLnZvaWNlLmV2ZW50cy52MS5Vc2VyTG9nZ2VkT3V0SABSDXVzZXJMb2dnZWRPdXQSTQoQdXNl'
    'cl8yZmFfZW5hYmxlZBgNIAEoCzIhLnZvaWNlLmV2ZW50cy52MS5Vc2VyVHdvRmFFbmFibGVkSA'
    'BSDnVzZXIyZmFFbmFibGVkElcKFHVzZXJfZ3Vlc3RfY29udmVydGVkGA4gASgLMiMudm9pY2Uu'
    'ZXZlbnRzLnYxLlVzZXJHdWVzdENvbnZlcnRlZEgAUhJ1c2VyR3Vlc3RDb252ZXJ0ZWQSVwoUdX'
    'Nlcl9hY2NvdW50X2RlbGV0ZWQYDyABKAsyIy52b2ljZS5ldmVudHMudjEuVXNlckFjY291bnRE'
    'ZWxldGVkSABSEnVzZXJBY2NvdW50RGVsZXRlZBJaChV1c2VyX2FjY291bnRfcmVzdG9yZWQYEC'
    'ABKAsyJC52b2ljZS5ldmVudHMudjEuVXNlckFjY291bnRSZXN0b3JlZEgAUhN1c2VyQWNjb3Vu'
    'dFJlc3RvcmVkEkoKD3Byb2ZpbGVfY3JlYXRlZBgRIAEoCzIfLnZvaWNlLmV2ZW50cy52MS5Qcm'
    '9maWxlQ3JlYXRlZEgAUg5wcm9maWxlQ3JlYXRlZBJNChBwcm9maWxlX3N3aXRjaGVkGBIgASgL'
    'MiAudm9pY2UuZXZlbnRzLnYxLlByb2ZpbGVTd2l0Y2hlZEgAUg9wcm9maWxlU3dpdGNoZWQSTQ'
    'oQc2V0dGluZ3NfY2hhbmdlZBgTIAEoCzIgLnZvaWNlLmV2ZW50cy52MS5TZXR0aW5nc0NoYW5n'
    'ZWRIAFIPc2V0dGluZ3NDaGFuZ2VkEkoKD3ByZXNlbmNlX2NoYW5nZRgUIAEoCzIfLnZvaWNlLm'
    'V2ZW50cy52MS5QcmVzZW5jZUNoYW5nZUgAUg5wcmVzZW5jZUNoYW5nZUIJCgdwYXlsb2Fk');

@$core.Deprecated('Use userRegisteredDescriptor instead')
const UserRegistered$json = {
  '1': 'UserRegistered',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'type', '3': 2, '4': 1, '5': 9, '10': 'type'},
    {'1': 'method', '3': 3, '4': 1, '5': 9, '10': 'method'},
  ],
};

/// Descriptor for `UserRegistered`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userRegisteredDescriptor = $convert.base64Decode(
    'Cg5Vc2VyUmVnaXN0ZXJlZBIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW50SWQSEgoEdHlwZR'
    'gCIAEoCVIEdHlwZRIWCgZtZXRob2QYAyABKAlSBm1ldGhvZA==');

@$core.Deprecated('Use userLoggedInDescriptor instead')
const UserLoggedIn$json = {
  '1': 'UserLoggedIn',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'device_info_json', '3': 2, '4': 1, '5': 9, '10': 'deviceInfoJson'},
    {'1': 'ip', '3': 3, '4': 1, '5': 9, '10': 'ip'},
  ],
};

/// Descriptor for `UserLoggedIn`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userLoggedInDescriptor = $convert.base64Decode(
    'CgxVc2VyTG9nZ2VkSW4SHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudElkEigKEGRldmljZV'
    '9pbmZvX2pzb24YAiABKAlSDmRldmljZUluZm9Kc29uEg4KAmlwGAMgASgJUgJpcA==');

@$core.Deprecated('Use userLoggedOutDescriptor instead')
const UserLoggedOut$json = {
  '1': 'UserLoggedOut',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'device_info_json', '3': 2, '4': 1, '5': 9, '10': 'deviceInfoJson'},
  ],
};

/// Descriptor for `UserLoggedOut`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userLoggedOutDescriptor = $convert.base64Decode(
    'Cg1Vc2VyTG9nZ2VkT3V0Eh0KCmFjY291bnRfaWQYASABKAlSCWFjY291bnRJZBIoChBkZXZpY2'
    'VfaW5mb19qc29uGAIgASgJUg5kZXZpY2VJbmZvSnNvbg==');

@$core.Deprecated('Use userTwoFaEnabledDescriptor instead')
const UserTwoFaEnabled$json = {
  '1': 'UserTwoFaEnabled',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `UserTwoFaEnabled`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userTwoFaEnabledDescriptor = $convert.base64Decode(
    'ChBVc2VyVHdvRmFFbmFibGVkEh0KCmFjY291bnRfaWQYASABKAlSCWFjY291bnRJZA==');

@$core.Deprecated('Use userGuestConvertedDescriptor instead')
const UserGuestConverted$json = {
  '1': 'UserGuestConverted',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `UserGuestConverted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userGuestConvertedDescriptor =
    $convert.base64Decode(
        'ChJVc2VyR3Vlc3RDb252ZXJ0ZWQSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudElk');

@$core.Deprecated('Use userAccountDeletedDescriptor instead')
const UserAccountDeleted$json = {
  '1': 'UserAccountDeleted',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `UserAccountDeleted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userAccountDeletedDescriptor =
    $convert.base64Decode(
        'ChJVc2VyQWNjb3VudERlbGV0ZWQSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudElk');

@$core.Deprecated('Use userAccountRestoredDescriptor instead')
const UserAccountRestored$json = {
  '1': 'UserAccountRestored',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `UserAccountRestored`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userAccountRestoredDescriptor = $convert.base64Decode(
    'ChNVc2VyQWNjb3VudFJlc3RvcmVkEh0KCmFjY291bnRfaWQYASABKAlSCWFjY291bnRJZA==');

@$core.Deprecated('Use profileCreatedDescriptor instead')
const ProfileCreated$json = {
  '1': 'ProfileCreated',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `ProfileCreated`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List profileCreatedDescriptor = $convert.base64Decode(
    'Cg5Qcm9maWxlQ3JlYXRlZBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQSHQoKYWNjb3'
    'VudF9pZBgCIAEoCVIJYWNjb3VudElk');

@$core.Deprecated('Use profileSwitchedDescriptor instead')
const ProfileSwitched$json = {
  '1': 'ProfileSwitched',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `ProfileSwitched`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List profileSwitchedDescriptor = $convert.base64Decode(
    'Cg9Qcm9maWxlU3dpdGNoZWQSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEh0KCmFjY2'
    '91bnRfaWQYAiABKAlSCWFjY291bnRJZA==');

@$core.Deprecated('Use settingsChangedDescriptor instead')
const SettingsChanged$json = {
  '1': 'SettingsChanged',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'changed_keys_json', '3': 2, '4': 1, '5': 9, '10': 'changedKeysJson'},
  ],
};

/// Descriptor for `SettingsChanged`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List settingsChangedDescriptor = $convert.base64Decode(
    'Cg9TZXR0aW5nc0NoYW5nZWQSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEioKEWNoYW'
    '5nZWRfa2V5c19qc29uGAIgASgJUg9jaGFuZ2VkS2V5c0pzb24=');

@$core.Deprecated('Use presenceChangeDescriptor instead')
const PresenceChange$json = {
  '1': 'PresenceChange',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'status', '3': 2, '4': 1, '5': 9, '10': 'status'},
  ],
};

/// Descriptor for `PresenceChange`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List presenceChangeDescriptor = $convert.base64Decode(
    'Cg5QcmVzZW5jZUNoYW5nZRIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQSFgoGc3RhdH'
    'VzGAIgASgJUgZzdGF0dXM=');

@$core.Deprecated('Use socialStreamEventDescriptor instead')
const SocialStreamEvent$json = {
  '1': 'SocialStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'friend_added',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.FriendAdded',
      '9': 0,
      '10': 'friendAdded'
    },
    {
      '1': 'friend_removed',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.FriendRemoved',
      '9': 0,
      '10': 'friendRemoved'
    },
    {
      '1': 'contact_synced',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ContactSynced',
      '9': 0,
      '10': 'contactSynced'
    },
    {
      '1': 'user_blocked',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.UserBlocked',
      '9': 0,
      '10': 'userBlocked'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `SocialStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List socialStreamEventDescriptor = $convert.base64Decode(
    'ChFTb2NpYWxTdHJlYW1FdmVudBIZCghldmVudF9pZBgBIAEoCVIHZXZlbnRJZBI7CgtvY2N1cn'
    'JlZF9hdBgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCm9jY3VycmVkQXQSQQoM'
    'ZnJpZW5kX2FkZGVkGAogASgLMhwudm9pY2UuZXZlbnRzLnYxLkZyaWVuZEFkZGVkSABSC2ZyaW'
    'VuZEFkZGVkEkcKDmZyaWVuZF9yZW1vdmVkGAsgASgLMh4udm9pY2UuZXZlbnRzLnYxLkZyaWVu'
    'ZFJlbW92ZWRIAFINZnJpZW5kUmVtb3ZlZBJHCg5jb250YWN0X3N5bmNlZBgMIAEoCzIeLnZvaW'
    'NlLmV2ZW50cy52MS5Db250YWN0U3luY2VkSABSDWNvbnRhY3RTeW5jZWQSQQoMdXNlcl9ibG9j'
    'a2VkGA0gASgLMhwudm9pY2UuZXZlbnRzLnYxLlVzZXJCbG9ja2VkSABSC3VzZXJCbG9ja2VkQg'
    'kKB3BheWxvYWQ=');

@$core.Deprecated('Use friendAddedDescriptor instead')
const FriendAdded$json = {
  '1': 'FriendAdded',
  '2': [
    {
      '1': 'requester_profile_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'requesterProfileId'
    },
    {'1': 'target_profile_id', '3': 2, '4': 1, '5': 9, '10': 'targetProfileId'},
  ],
};

/// Descriptor for `FriendAdded`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List friendAddedDescriptor = $convert.base64Decode(
    'CgtGcmllbmRBZGRlZBIwChRyZXF1ZXN0ZXJfcHJvZmlsZV9pZBgBIAEoCVIScmVxdWVzdGVyUH'
    'JvZmlsZUlkEioKEXRhcmdldF9wcm9maWxlX2lkGAIgASgJUg90YXJnZXRQcm9maWxlSWQ=');

@$core.Deprecated('Use friendRemovedDescriptor instead')
const FriendRemoved$json = {
  '1': 'FriendRemoved',
  '2': [
    {'1': 'profile_id_a', '3': 1, '4': 1, '5': 9, '10': 'profileIdA'},
    {'1': 'profile_id_b', '3': 2, '4': 1, '5': 9, '10': 'profileIdB'},
  ],
};

/// Descriptor for `FriendRemoved`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List friendRemovedDescriptor = $convert.base64Decode(
    'Cg1GcmllbmRSZW1vdmVkEiAKDHByb2ZpbGVfaWRfYRgBIAEoCVIKcHJvZmlsZUlkQRIgCgxwcm'
    '9maWxlX2lkX2IYAiABKAlSCnByb2ZpbGVJZEI=');

@$core.Deprecated('Use contactSyncedDescriptor instead')
const ContactSynced$json = {
  '1': 'ContactSynced',
  '2': [
    {'1': 'owner_profile_id', '3': 1, '4': 1, '5': 9, '10': 'ownerProfileId'},
    {'1': 'count', '3': 2, '4': 1, '5': 5, '10': 'count'},
  ],
};

/// Descriptor for `ContactSynced`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List contactSyncedDescriptor = $convert.base64Decode(
    'Cg1Db250YWN0U3luY2VkEigKEG93bmVyX3Byb2ZpbGVfaWQYASABKAlSDm93bmVyUHJvZmlsZU'
    'lkEhQKBWNvdW50GAIgASgFUgVjb3VudA==');

@$core.Deprecated('Use userBlockedDescriptor instead')
const UserBlocked$json = {
  '1': 'UserBlocked',
  '2': [
    {
      '1': 'blocker_account_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'blockerAccountId'
    },
    {
      '1': 'blocked_account_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'blockedAccountId'
    },
  ],
};

/// Descriptor for `UserBlocked`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userBlockedDescriptor = $convert.base64Decode(
    'CgtVc2VyQmxvY2tlZBIsChJibG9ja2VyX2FjY291bnRfaWQYASABKAlSEGJsb2NrZXJBY2NvdW'
    '50SWQSLAoSYmxvY2tlZF9hY2NvdW50X2lkGAIgASgJUhBibG9ja2VkQWNjb3VudElk');

@$core.Deprecated('Use roleStreamEventDescriptor instead')
const RoleStreamEvent$json = {
  '1': 'RoleStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'role_assignment_changed',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.RoleAssignmentChanged',
      '9': 0,
      '10': 'roleAssignmentChanged'
    },
    {
      '1': 'role_definition_changed',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.RoleDefinitionChanged',
      '9': 0,
      '10': 'roleDefinitionChanged'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `RoleStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleStreamEventDescriptor = $convert.base64Decode(
    'Cg9Sb2xlU3RyZWFtRXZlbnQSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW50SWQSOwoLb2NjdXJyZW'
    'RfYXQYAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpvY2N1cnJlZEF0EmAKF3Jv'
    'bGVfYXNzaWdubWVudF9jaGFuZ2VkGAogASgLMiYudm9pY2UuZXZlbnRzLnYxLlJvbGVBc3NpZ2'
    '5tZW50Q2hhbmdlZEgAUhVyb2xlQXNzaWdubWVudENoYW5nZWQSYAoXcm9sZV9kZWZpbml0aW9u'
    'X2NoYW5nZWQYCyABKAsyJi52b2ljZS5ldmVudHMudjEuUm9sZURlZmluaXRpb25DaGFuZ2VkSA'
    'BSFXJvbGVEZWZpbml0aW9uQ2hhbmdlZEIJCgdwYXlsb2Fk');

@$core.Deprecated('Use roleAssignmentChangedDescriptor instead')
const RoleAssignmentChanged$json = {
  '1': 'RoleAssignmentChanged',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_ids', '3': 3, '4': 3, '5': 9, '10': 'roleIds'},
  ],
};

/// Descriptor for `RoleAssignmentChanged`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleAssignmentChangedDescriptor = $convert.base64Decode(
    'ChVSb2xlQXNzaWdubWVudENoYW5nZWQSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSHQoKcH'
    'JvZmlsZV9pZBgCIAEoCVIJcHJvZmlsZUlkEhkKCHJvbGVfaWRzGAMgAygJUgdyb2xlSWRz');

@$core.Deprecated('Use roleDefinitionChangedDescriptor instead')
const RoleDefinitionChanged$json = {
  '1': 'RoleDefinitionChanged',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'role_id', '3': 2, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `RoleDefinitionChanged`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleDefinitionChangedDescriptor = $convert.base64Decode(
    'ChVSb2xlRGVmaW5pdGlvbkNoYW5nZWQSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSFwoHcm'
    '9sZV9pZBgCIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use messageStreamEventDescriptor instead')
const MessageStreamEvent$json = {
  '1': 'MessageStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'message_sent',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MessageSent',
      '9': 0,
      '10': 'messageSent'
    },
    {
      '1': 'message_edited',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MessageEdited',
      '9': 0,
      '10': 'messageEdited'
    },
    {
      '1': 'message_deleted',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MessageDeleted',
      '9': 0,
      '10': 'messageDeleted'
    },
    {
      '1': 'reaction_added',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ReactionAdded',
      '9': 0,
      '10': 'reactionAdded'
    },
    {
      '1': 'message_read',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MessageRead',
      '9': 0,
      '10': 'messageRead'
    },
    {
      '1': 'reaction_removed',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ReactionRemoved',
      '9': 0,
      '10': 'reactionRemoved'
    },
    {
      '1': 'mention_added',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MentionAdded',
      '9': 0,
      '10': 'mentionAdded'
    },
    {
      '1': 'message_pinned',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MessagePinned',
      '9': 0,
      '10': 'messagePinned'
    },
    {
      '1': 'message_unpinned',
      '3': 18,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MessageUnpinned',
      '9': 0,
      '10': 'messageUnpinned'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `MessageStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageStreamEventDescriptor = $convert.base64Decode(
    'ChJNZXNzYWdlU3RyZWFtRXZlbnQSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW50SWQSOwoLb2NjdX'
    'JyZWRfYXQYAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpvY2N1cnJlZEF0EkEK'
    'DG1lc3NhZ2Vfc2VudBgKIAEoCzIcLnZvaWNlLmV2ZW50cy52MS5NZXNzYWdlU2VudEgAUgttZX'
    'NzYWdlU2VudBJHCg5tZXNzYWdlX2VkaXRlZBgLIAEoCzIeLnZvaWNlLmV2ZW50cy52MS5NZXNz'
    'YWdlRWRpdGVkSABSDW1lc3NhZ2VFZGl0ZWQSSgoPbWVzc2FnZV9kZWxldGVkGAwgASgLMh8udm'
    '9pY2UuZXZlbnRzLnYxLk1lc3NhZ2VEZWxldGVkSABSDm1lc3NhZ2VEZWxldGVkEkcKDnJlYWN0'
    'aW9uX2FkZGVkGA0gASgLMh4udm9pY2UuZXZlbnRzLnYxLlJlYWN0aW9uQWRkZWRIAFINcmVhY3'
    'Rpb25BZGRlZBJBCgxtZXNzYWdlX3JlYWQYDiABKAsyHC52b2ljZS5ldmVudHMudjEuTWVzc2Fn'
    'ZVJlYWRIAFILbWVzc2FnZVJlYWQSTQoQcmVhY3Rpb25fcmVtb3ZlZBgPIAEoCzIgLnZvaWNlLm'
    'V2ZW50cy52MS5SZWFjdGlvblJlbW92ZWRIAFIPcmVhY3Rpb25SZW1vdmVkEkQKDW1lbnRpb25f'
    'YWRkZWQYECABKAsyHS52b2ljZS5ldmVudHMudjEuTWVudGlvbkFkZGVkSABSDG1lbnRpb25BZG'
    'RlZBJHCg5tZXNzYWdlX3Bpbm5lZBgRIAEoCzIeLnZvaWNlLmV2ZW50cy52MS5NZXNzYWdlUGlu'
    'bmVkSABSDW1lc3NhZ2VQaW5uZWQSTQoQbWVzc2FnZV91bnBpbm5lZBgSIAEoCzIgLnZvaWNlLm'
    'V2ZW50cy52MS5NZXNzYWdlVW5waW5uZWRIAFIPbWVzc2FnZVVucGlubmVkQgkKB3BheWxvYWQ=');

@$core.Deprecated('Use messageSentDescriptor instead')
const MessageSent$json = {
  '1': 'MessageSent',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'sender_profile_id', '3': 3, '4': 1, '5': 9, '10': 'senderProfileId'},
    {'1': 'has_mentions', '3': 4, '4': 1, '5': 8, '10': 'hasMentions'},
    {
      '1': 'thread_parent_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'threadParentId',
      '17': true
    },
    {'1': 'is_e2e', '3': 6, '4': 1, '5': 8, '10': 'isE2e'},
  ],
  '8': [
    {'1': '_thread_parent_id'},
  ],
};

/// Descriptor for `MessageSent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageSentDescriptor = $convert.base64Decode(
    'CgtNZXNzYWdlU2VudBIdCgptZXNzYWdlX2lkGAEgASgJUgltZXNzYWdlSWQSFwoHY2hhdF9pZB'
    'gCIAEoCVIGY2hhdElkEioKEXNlbmRlcl9wcm9maWxlX2lkGAMgASgJUg9zZW5kZXJQcm9maWxl'
    'SWQSIQoMaGFzX21lbnRpb25zGAQgASgIUgtoYXNNZW50aW9ucxItChB0aHJlYWRfcGFyZW50X2'
    'lkGAUgASgJSABSDnRocmVhZFBhcmVudElkiAEBEhUKBmlzX2UyZRgGIAEoCFIFaXNFMmVCEwoR'
    'X3RocmVhZF9wYXJlbnRfaWQ=');

@$core.Deprecated('Use mentionAddedDescriptor instead')
const MentionAdded$json = {
  '1': 'MentionAdded',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'sender_profile_id', '3': 3, '4': 1, '5': 9, '10': 'senderProfileId'},
    {
      '1': 'mentioned_profile_ids',
      '3': 4,
      '4': 3,
      '5': 9,
      '10': 'mentionedProfileIds'
    },
  ],
};

/// Descriptor for `MentionAdded`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List mentionAddedDescriptor = $convert.base64Decode(
    'CgxNZW50aW9uQWRkZWQSHQoKbWVzc2FnZV9pZBgBIAEoCVIJbWVzc2FnZUlkEhcKB2NoYXRfaW'
    'QYAiABKAlSBmNoYXRJZBIqChFzZW5kZXJfcHJvZmlsZV9pZBgDIAEoCVIPc2VuZGVyUHJvZmls'
    'ZUlkEjIKFW1lbnRpb25lZF9wcm9maWxlX2lkcxgEIAMoCVITbWVudGlvbmVkUHJvZmlsZUlkcw'
    '==');

@$core.Deprecated('Use messageEditedDescriptor instead')
const MessageEdited$json = {
  '1': 'MessageEdited',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'is_e2e', '3': 6, '4': 1, '5': 8, '10': 'isE2e'},
  ],
};

/// Descriptor for `MessageEdited`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageEditedDescriptor = $convert.base64Decode(
    'Cg1NZXNzYWdlRWRpdGVkEh0KCm1lc3NhZ2VfaWQYASABKAlSCW1lc3NhZ2VJZBIXCgdjaGF0X2'
    'lkGAIgASgJUgZjaGF0SWQSFQoGaXNfZTJlGAYgASgIUgVpc0UyZQ==');

@$core.Deprecated('Use messageDeletedDescriptor instead')
const MessageDeleted$json = {
  '1': 'MessageDeleted',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `MessageDeleted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageDeletedDescriptor = $convert.base64Decode(
    'Cg5NZXNzYWdlRGVsZXRlZBIdCgptZXNzYWdlX2lkGAEgASgJUgltZXNzYWdlSWQSFwoHY2hhdF'
    '9pZBgCIAEoCVIGY2hhdElk');

@$core.Deprecated('Use reactionAddedDescriptor instead')
const ReactionAdded$json = {
  '1': 'ReactionAdded',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'emoji', '3': 3, '4': 1, '5': 9, '10': 'emoji'},
    {'1': 'chat_id', '3': 4, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'message_author_profile_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'messageAuthorProfileId'
    },
  ],
};

/// Descriptor for `ReactionAdded`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reactionAddedDescriptor = $convert.base64Decode(
    'Cg1SZWFjdGlvbkFkZGVkEh0KCm1lc3NhZ2VfaWQYASABKAlSCW1lc3NhZ2VJZBIdCgpwcm9maW'
    'xlX2lkGAIgASgJUglwcm9maWxlSWQSFAoFZW1vamkYAyABKAlSBWVtb2ppEhcKB2NoYXRfaWQY'
    'BCABKAlSBmNoYXRJZBI5ChltZXNzYWdlX2F1dGhvcl9wcm9maWxlX2lkGAUgASgJUhZtZXNzYW'
    'dlQXV0aG9yUHJvZmlsZUlk');

@$core.Deprecated('Use reactionRemovedDescriptor instead')
const ReactionRemoved$json = {
  '1': 'ReactionRemoved',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'emoji', '3': 3, '4': 1, '5': 9, '10': 'emoji'},
    {'1': 'chat_id', '3': 4, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `ReactionRemoved`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reactionRemovedDescriptor = $convert.base64Decode(
    'Cg9SZWFjdGlvblJlbW92ZWQSHQoKbWVzc2FnZV9pZBgBIAEoCVIJbWVzc2FnZUlkEh0KCnByb2'
    'ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZBIUCgVlbW9qaRgDIAEoCVIFZW1vamkSFwoHY2hhdF9p'
    'ZBgEIAEoCVIGY2hhdElk');

@$core.Deprecated('Use messageReadDescriptor instead')
const MessageRead$json = {
  '1': 'MessageRead',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `MessageRead`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageReadDescriptor = $convert.base64Decode(
    'CgtNZXNzYWdlUmVhZBIdCgptZXNzYWdlX2lkGAEgASgJUgltZXNzYWdlSWQSFwoHY2hhdF9pZB'
    'gCIAEoCVIGY2hhdElkEh0KCnByb2ZpbGVfaWQYAyABKAlSCXByb2ZpbGVJZA==');

@$core.Deprecated('Use messagePinnedDescriptor instead')
const MessagePinned$json = {
  '1': 'MessagePinned',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'pinned_by', '3': 3, '4': 1, '5': 9, '10': 'pinnedBy'},
  ],
};

/// Descriptor for `MessagePinned`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messagePinnedDescriptor = $convert.base64Decode(
    'Cg1NZXNzYWdlUGlubmVkEh0KCm1lc3NhZ2VfaWQYASABKAlSCW1lc3NhZ2VJZBIXCgdjaGF0X2'
    'lkGAIgASgJUgZjaGF0SWQSGwoJcGlubmVkX2J5GAMgASgJUghwaW5uZWRCeQ==');

@$core.Deprecated('Use messageUnpinnedDescriptor instead')
const MessageUnpinned$json = {
  '1': 'MessageUnpinned',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'unpinned_by', '3': 3, '4': 1, '5': 9, '10': 'unpinnedBy'},
  ],
};

/// Descriptor for `MessageUnpinned`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageUnpinnedDescriptor = $convert.base64Decode(
    'Cg9NZXNzYWdlVW5waW5uZWQSHQoKbWVzc2FnZV9pZBgBIAEoCVIJbWVzc2FnZUlkEhcKB2NoYX'
    'RfaWQYAiABKAlSBmNoYXRJZBIfCgt1bnBpbm5lZF9ieRgDIAEoCVIKdW5waW5uZWRCeQ==');

@$core.Deprecated('Use chatStreamEventDescriptor instead')
const ChatStreamEvent$json = {
  '1': 'ChatStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'chat_created',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ChatCreated',
      '9': 0,
      '10': 'chatCreated'
    },
    {
      '1': 'chat_member_changed',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ChatMemberChanged',
      '9': 0,
      '10': 'chatMemberChanged'
    },
    {
      '1': 'space_tree_changed',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.SpaceTreeChanged',
      '9': 0,
      '10': 'spaceTreeChanged'
    },
    {
      '1': 'space_created',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.SpaceCreated',
      '9': 0,
      '10': 'spaceCreated'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `ChatStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatStreamEventDescriptor = $convert.base64Decode(
    'Cg9DaGF0U3RyZWFtRXZlbnQSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW50SWQSOwoLb2NjdXJyZW'
    'RfYXQYAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpvY2N1cnJlZEF0EkEKDGNo'
    'YXRfY3JlYXRlZBgKIAEoCzIcLnZvaWNlLmV2ZW50cy52MS5DaGF0Q3JlYXRlZEgAUgtjaGF0Q3'
    'JlYXRlZBJUChNjaGF0X21lbWJlcl9jaGFuZ2VkGAsgASgLMiIudm9pY2UuZXZlbnRzLnYxLkNo'
    'YXRNZW1iZXJDaGFuZ2VkSABSEWNoYXRNZW1iZXJDaGFuZ2VkElEKEnNwYWNlX3RyZWVfY2hhbm'
    'dlZBgMIAEoCzIhLnZvaWNlLmV2ZW50cy52MS5TcGFjZVRyZWVDaGFuZ2VkSABSEHNwYWNlVHJl'
    'ZUNoYW5nZWQSRAoNc3BhY2VfY3JlYXRlZBgNIAEoCzIdLnZvaWNlLmV2ZW50cy52MS5TcGFjZU'
    'NyZWF0ZWRIAFIMc3BhY2VDcmVhdGVkQgkKB3BheWxvYWQ=');

@$core.Deprecated('Use chatCreatedDescriptor instead')
const ChatCreated$json = {
  '1': 'ChatCreated',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'type', '3': 2, '4': 1, '5': 9, '10': 'type'},
  ],
};

/// Descriptor for `ChatCreated`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatCreatedDescriptor = $convert.base64Decode(
    'CgtDaGF0Q3JlYXRlZBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQSEgoEdHlwZRgCIAEoCVIEdH'
    'lwZQ==');

@$core.Deprecated('Use chatMemberChangedDescriptor instead')
const ChatMemberChanged$json = {
  '1': 'ChatMemberChanged',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'change', '3': 3, '4': 1, '5': 9, '10': 'change'},
  ],
};

/// Descriptor for `ChatMemberChanged`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatMemberChangedDescriptor = $convert.base64Decode(
    'ChFDaGF0TWVtYmVyQ2hhbmdlZBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQSHQoKcHJvZmlsZV'
    '9pZBgCIAEoCVIJcHJvZmlsZUlkEhYKBmNoYW5nZRgDIAEoCVIGY2hhbmdl');

@$core.Deprecated('Use spaceTreeChangedDescriptor instead')
const SpaceTreeChanged$json = {
  '1': 'SpaceTreeChanged',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'node_id', '3': 2, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'change', '3': 3, '4': 1, '5': 9, '10': 'change'},
  ],
};

/// Descriptor for `SpaceTreeChanged`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceTreeChangedDescriptor = $convert.base64Decode(
    'ChBTcGFjZVRyZWVDaGFuZ2VkEhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEhcKB25vZGVfaW'
    'QYAiABKAlSBm5vZGVJZBIWCgZjaGFuZ2UYAyABKAlSBmNoYW5nZQ==');

@$core.Deprecated('Use spaceCreatedDescriptor instead')
const SpaceCreated$json = {
  '1': 'SpaceCreated',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'owner_profile_id', '3': 2, '4': 1, '5': 9, '10': 'ownerProfileId'},
  ],
};

/// Descriptor for `SpaceCreated`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceCreatedDescriptor = $convert.base64Decode(
    'CgxTcGFjZUNyZWF0ZWQSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSKAoQb3duZXJfcHJvZm'
    'lsZV9pZBgCIAEoCVIOb3duZXJQcm9maWxlSWQ=');

@$core.Deprecated('Use voiceStreamEventDescriptor instead')
const VoiceStreamEvent$json = {
  '1': 'VoiceStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'call_started',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.CallStarted',
      '9': 0,
      '10': 'callStarted'
    },
    {
      '1': 'call_ended',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.CallEnded',
      '9': 0,
      '10': 'callEnded'
    },
    {
      '1': 'screen_share_started',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ScreenShareStarted',
      '9': 0,
      '10': 'screenShareStarted'
    },
    {
      '1': 'call_incoming',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.CallIncoming',
      '9': 0,
      '10': 'callIncoming'
    },
    {
      '1': 'call_accepted',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.CallAccepted',
      '9': 0,
      '10': 'callAccepted'
    },
    {
      '1': 'call_declined',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.CallDeclined',
      '9': 0,
      '10': 'callDeclined'
    },
    {
      '1': 'call_missed',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.CallMissed',
      '9': 0,
      '10': 'callMissed'
    },
    {
      '1': 'voice_state_changed',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.VoiceStateChanged',
      '9': 0,
      '10': 'voiceStateChanged'
    },
    {
      '1': 'screen_share_stopped',
      '3': 18,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ScreenShareStopped',
      '9': 0,
      '10': 'screenShareStopped'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `VoiceStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List voiceStreamEventDescriptor = $convert.base64Decode(
    'ChBWb2ljZVN0cmVhbUV2ZW50EhkKCGV2ZW50X2lkGAEgASgJUgdldmVudElkEjsKC29jY3Vycm'
    'VkX2F0GAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKb2NjdXJyZWRBdBJBCgxj'
    'YWxsX3N0YXJ0ZWQYCiABKAsyHC52b2ljZS5ldmVudHMudjEuQ2FsbFN0YXJ0ZWRIAFILY2FsbF'
    'N0YXJ0ZWQSOwoKY2FsbF9lbmRlZBgLIAEoCzIaLnZvaWNlLmV2ZW50cy52MS5DYWxsRW5kZWRI'
    'AFIJY2FsbEVuZGVkElcKFHNjcmVlbl9zaGFyZV9zdGFydGVkGAwgASgLMiMudm9pY2UuZXZlbn'
    'RzLnYxLlNjcmVlblNoYXJlU3RhcnRlZEgAUhJzY3JlZW5TaGFyZVN0YXJ0ZWQSRAoNY2FsbF9p'
    'bmNvbWluZxgNIAEoCzIdLnZvaWNlLmV2ZW50cy52MS5DYWxsSW5jb21pbmdIAFIMY2FsbEluY2'
    '9taW5nEkQKDWNhbGxfYWNjZXB0ZWQYDiABKAsyHS52b2ljZS5ldmVudHMudjEuQ2FsbEFjY2Vw'
    'dGVkSABSDGNhbGxBY2NlcHRlZBJECg1jYWxsX2RlY2xpbmVkGA8gASgLMh0udm9pY2UuZXZlbn'
    'RzLnYxLkNhbGxEZWNsaW5lZEgAUgxjYWxsRGVjbGluZWQSPgoLY2FsbF9taXNzZWQYECABKAsy'
    'Gy52b2ljZS5ldmVudHMudjEuQ2FsbE1pc3NlZEgAUgpjYWxsTWlzc2VkElQKE3ZvaWNlX3N0YX'
    'RlX2NoYW5nZWQYESABKAsyIi52b2ljZS5ldmVudHMudjEuVm9pY2VTdGF0ZUNoYW5nZWRIAFIR'
    'dm9pY2VTdGF0ZUNoYW5nZWQSVwoUc2NyZWVuX3NoYXJlX3N0b3BwZWQYEiABKAsyIy52b2ljZS'
    '5ldmVudHMudjEuU2NyZWVuU2hhcmVTdG9wcGVkSABSEnNjcmVlblNoYXJlU3RvcHBlZEIJCgdw'
    'YXlsb2Fk');

@$core.Deprecated('Use callStartedDescriptor instead')
const CallStarted$json = {
  '1': 'CallStarted',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'profile_ids', '3': 2, '4': 3, '5': 9, '10': 'profileIds'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'initiator_profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'initiatorProfileId'
    },
    {'1': 'callee_profile_id', '3': 5, '4': 1, '5': 9, '10': 'calleeProfileId'},
    {'1': 'media_kind', '3': 6, '4': 1, '5': 9, '10': 'mediaKind'},
    {'1': 'livekit_room_name', '3': 7, '4': 1, '5': 9, '10': 'livekitRoomName'},
  ],
};

/// Descriptor for `CallStarted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List callStartedDescriptor = $convert.base64Decode(
    'CgtDYWxsU3RhcnRlZBIXCgdyb29tX2lkGAEgASgJUgZyb29tSWQSHwoLcHJvZmlsZV9pZHMYAi'
    'ADKAlSCnByb2ZpbGVJZHMSFwoHY2hhdF9pZBgDIAEoCVIGY2hhdElkEjAKFGluaXRpYXRvcl9w'
    'cm9maWxlX2lkGAQgASgJUhJpbml0aWF0b3JQcm9maWxlSWQSKgoRY2FsbGVlX3Byb2ZpbGVfaW'
    'QYBSABKAlSD2NhbGxlZVByb2ZpbGVJZBIdCgptZWRpYV9raW5kGAYgASgJUgltZWRpYUtpbmQS'
    'KgoRbGl2ZWtpdF9yb29tX25hbWUYByABKAlSD2xpdmVraXRSb29tTmFtZQ==');

@$core.Deprecated('Use callEndedDescriptor instead')
const CallEnded$json = {
  '1': 'CallEnded',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'duration_seconds', '3': 2, '4': 1, '5': 5, '10': 'durationSeconds'},
    {'1': 'profile_ids', '3': 3, '4': 3, '5': 9, '10': 'profileIds'},
    {'1': 'reason', '3': 4, '4': 1, '5': 9, '10': 'reason'},
    {
      '1': 'ended_by_profile_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'endedByProfileId'
    },
  ],
};

/// Descriptor for `CallEnded`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List callEndedDescriptor = $convert.base64Decode(
    'CglDYWxsRW5kZWQSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlkEikKEGR1cmF0aW9uX3NlY29uZH'
    'MYAiABKAVSD2R1cmF0aW9uU2Vjb25kcxIfCgtwcm9maWxlX2lkcxgDIAMoCVIKcHJvZmlsZUlk'
    'cxIWCgZyZWFzb24YBCABKAlSBnJlYXNvbhItChNlbmRlZF9ieV9wcm9maWxlX2lkGAUgASgJUh'
    'BlbmRlZEJ5UHJvZmlsZUlk');

@$core.Deprecated('Use screenShareStartedDescriptor instead')
const ScreenShareStarted$json = {
  '1': 'ScreenShareStarted',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'stream_id', '3': 3, '4': 1, '5': 9, '10': 'streamId'},
    {'1': 'profile_ids', '3': 4, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `ScreenShareStarted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List screenShareStartedDescriptor = $convert.base64Decode(
    'ChJTY3JlZW5TaGFyZVN0YXJ0ZWQSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlkEh0KCnByb2ZpbG'
    'VfaWQYAiABKAlSCXByb2ZpbGVJZBIbCglzdHJlYW1faWQYAyABKAlSCHN0cmVhbUlkEh8KC3By'
    'b2ZpbGVfaWRzGAQgAygJUgpwcm9maWxlSWRz');

@$core.Deprecated('Use screenShareStoppedDescriptor instead')
const ScreenShareStopped$json = {
  '1': 'ScreenShareStopped',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'stream_id', '3': 3, '4': 1, '5': 9, '10': 'streamId'},
    {'1': 'profile_ids', '3': 4, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `ScreenShareStopped`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List screenShareStoppedDescriptor = $convert.base64Decode(
    'ChJTY3JlZW5TaGFyZVN0b3BwZWQSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlkEh0KCnByb2ZpbG'
    'VfaWQYAiABKAlSCXByb2ZpbGVJZBIbCglzdHJlYW1faWQYAyABKAlSCHN0cmVhbUlkEh8KC3By'
    'b2ZpbGVfaWRzGAQgAygJUgpwcm9maWxlSWRz');

@$core.Deprecated('Use callIncomingDescriptor instead')
const CallIncoming$json = {
  '1': 'CallIncoming',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'initiator_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'initiatorProfileId'
    },
    {'1': 'callee_profile_id', '3': 4, '4': 1, '5': 9, '10': 'calleeProfileId'},
    {'1': 'media_kind', '3': 5, '4': 1, '5': 9, '10': 'mediaKind'},
    {'1': 'livekit_room_name', '3': 6, '4': 1, '5': 9, '10': 'livekitRoomName'},
    {
      '1': 'expires_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
  ],
};

/// Descriptor for `CallIncoming`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List callIncomingDescriptor = $convert.base64Decode(
    'CgxDYWxsSW5jb21pbmcSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlkEhcKB2NoYXRfaWQYAiABKA'
    'lSBmNoYXRJZBIwChRpbml0aWF0b3JfcHJvZmlsZV9pZBgDIAEoCVISaW5pdGlhdG9yUHJvZmls'
    'ZUlkEioKEWNhbGxlZV9wcm9maWxlX2lkGAQgASgJUg9jYWxsZWVQcm9maWxlSWQSHQoKbWVkaW'
    'Ffa2luZBgFIAEoCVIJbWVkaWFLaW5kEioKEWxpdmVraXRfcm9vbV9uYW1lGAYgASgJUg9saXZl'
    'a2l0Um9vbU5hbWUSOQoKZXhwaXJlc19hdBgHIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3'
    'RhbXBSCWV4cGlyZXNBdA==');

@$core.Deprecated('Use callAcceptedDescriptor instead')
const CallAccepted$json = {
  '1': 'CallAccepted',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'accepted_by_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'acceptedByProfileId'
    },
    {'1': 'profile_ids', '3': 4, '4': 3, '5': 9, '10': 'profileIds'},
    {'1': 'media_kind', '3': 5, '4': 1, '5': 9, '10': 'mediaKind'},
    {'1': 'livekit_room_name', '3': 6, '4': 1, '5': 9, '10': 'livekitRoomName'},
  ],
};

/// Descriptor for `CallAccepted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List callAcceptedDescriptor = $convert.base64Decode(
    'CgxDYWxsQWNjZXB0ZWQSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlkEhcKB2NoYXRfaWQYAiABKA'
    'lSBmNoYXRJZBIzChZhY2NlcHRlZF9ieV9wcm9maWxlX2lkGAMgASgJUhNhY2NlcHRlZEJ5UHJv'
    'ZmlsZUlkEh8KC3Byb2ZpbGVfaWRzGAQgAygJUgpwcm9maWxlSWRzEh0KCm1lZGlhX2tpbmQYBS'
    'ABKAlSCW1lZGlhS2luZBIqChFsaXZla2l0X3Jvb21fbmFtZRgGIAEoCVIPbGl2ZWtpdFJvb21O'
    'YW1l');

@$core.Deprecated('Use callDeclinedDescriptor instead')
const CallDeclined$json = {
  '1': 'CallDeclined',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'declined_by_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'declinedByProfileId'
    },
    {'1': 'profile_ids', '3': 4, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `CallDeclined`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List callDeclinedDescriptor = $convert.base64Decode(
    'CgxDYWxsRGVjbGluZWQSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlkEhcKB2NoYXRfaWQYAiABKA'
    'lSBmNoYXRJZBIzChZkZWNsaW5lZF9ieV9wcm9maWxlX2lkGAMgASgJUhNkZWNsaW5lZEJ5UHJv'
    'ZmlsZUlkEh8KC3Byb2ZpbGVfaWRzGAQgAygJUgpwcm9maWxlSWRz');

@$core.Deprecated('Use callMissedDescriptor instead')
const CallMissed$json = {
  '1': 'CallMissed',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'initiator_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'initiatorProfileId'
    },
    {'1': 'callee_profile_id', '3': 4, '4': 1, '5': 9, '10': 'calleeProfileId'},
  ],
};

/// Descriptor for `CallMissed`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List callMissedDescriptor = $convert.base64Decode(
    'CgpDYWxsTWlzc2VkEhcKB3Jvb21faWQYASABKAlSBnJvb21JZBIXCgdjaGF0X2lkGAIgASgJUg'
    'ZjaGF0SWQSMAoUaW5pdGlhdG9yX3Byb2ZpbGVfaWQYAyABKAlSEmluaXRpYXRvclByb2ZpbGVJ'
    'ZBIqChFjYWxsZWVfcHJvZmlsZV9pZBgEIAEoCVIPY2FsbGVlUHJvZmlsZUlk');

@$core.Deprecated('Use voiceStateChangedDescriptor instead')
const VoiceStateChanged$json = {
  '1': 'VoiceStateChanged',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'is_muted',
      '3': 3,
      '4': 1,
      '5': 8,
      '9': 0,
      '10': 'isMuted',
      '17': true
    },
    {
      '1': 'is_deafened',
      '3': 4,
      '4': 1,
      '5': 8,
      '9': 1,
      '10': 'isDeafened',
      '17': true
    },
    {
      '1': 'is_video_on',
      '3': 5,
      '4': 1,
      '5': 8,
      '9': 2,
      '10': 'isVideoOn',
      '17': true
    },
    {'1': 'profile_ids', '3': 6, '4': 3, '5': 9, '10': 'profileIds'},
  ],
  '8': [
    {'1': '_is_muted'},
    {'1': '_is_deafened'},
    {'1': '_is_video_on'},
  ],
};

/// Descriptor for `VoiceStateChanged`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List voiceStateChangedDescriptor = $convert.base64Decode(
    'ChFWb2ljZVN0YXRlQ2hhbmdlZBIXCgdyb29tX2lkGAEgASgJUgZyb29tSWQSHQoKcHJvZmlsZV'
    '9pZBgCIAEoCVIJcHJvZmlsZUlkEh4KCGlzX211dGVkGAMgASgISABSB2lzTXV0ZWSIAQESJAoL'
    'aXNfZGVhZmVuZWQYBCABKAhIAVIKaXNEZWFmZW5lZIgBARIjCgtpc192aWRlb19vbhgFIAEoCE'
    'gCUglpc1ZpZGVvT26IAQESHwoLcHJvZmlsZV9pZHMYBiADKAlSCnByb2ZpbGVJZHNCCwoJX2lz'
    'X211dGVkQg4KDF9pc19kZWFmZW5lZEIOCgxfaXNfdmlkZW9fb24=');

@$core.Deprecated('Use moderationStreamEventDescriptor instead')
const ModerationStreamEvent$json = {
  '1': 'ModerationStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'report_created',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.ReportCreated',
      '9': 0,
      '10': 'reportCreated'
    },
    {
      '1': 'sanction_applied',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.SanctionApplied',
      '9': 0,
      '10': 'sanctionApplied'
    },
    {
      '1': 'appeal_submitted',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.AppealSubmitted',
      '9': 0,
      '10': 'appealSubmitted'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `ModerationStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List moderationStreamEventDescriptor = $convert.base64Decode(
    'ChVNb2RlcmF0aW9uU3RyZWFtRXZlbnQSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW50SWQSOwoLb2'
    'NjdXJyZWRfYXQYAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpvY2N1cnJlZEF0'
    'EkcKDnJlcG9ydF9jcmVhdGVkGAogASgLMh4udm9pY2UuZXZlbnRzLnYxLlJlcG9ydENyZWF0ZW'
    'RIAFINcmVwb3J0Q3JlYXRlZBJNChBzYW5jdGlvbl9hcHBsaWVkGAsgASgLMiAudm9pY2UuZXZl'
    'bnRzLnYxLlNhbmN0aW9uQXBwbGllZEgAUg9zYW5jdGlvbkFwcGxpZWQSTQoQYXBwZWFsX3N1Ym'
    '1pdHRlZBgMIAEoCzIgLnZvaWNlLmV2ZW50cy52MS5BcHBlYWxTdWJtaXR0ZWRIAFIPYXBwZWFs'
    'U3VibWl0dGVkQgkKB3BheWxvYWQ=');

@$core.Deprecated('Use reportCreatedDescriptor instead')
const ReportCreated$json = {
  '1': 'ReportCreated',
  '2': [
    {'1': 'report_id', '3': 1, '4': 1, '5': 9, '10': 'reportId'},
    {
      '1': 'reporter_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'reporterProfileId'
    },
  ],
};

/// Descriptor for `ReportCreated`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reportCreatedDescriptor = $convert.base64Decode(
    'Cg1SZXBvcnRDcmVhdGVkEhsKCXJlcG9ydF9pZBgBIAEoCVIIcmVwb3J0SWQSLgoTcmVwb3J0ZX'
    'JfcHJvZmlsZV9pZBgCIAEoCVIRcmVwb3J0ZXJQcm9maWxlSWQ=');

@$core.Deprecated('Use sanctionAppliedDescriptor instead')
const SanctionApplied$json = {
  '1': 'SanctionApplied',
  '2': [
    {'1': 'sanction_id', '3': 1, '4': 1, '5': 9, '10': 'sanctionId'},
    {'1': 'target_account_id', '3': 2, '4': 1, '5': 9, '10': 'targetAccountId'},
    {'1': 'type', '3': 3, '4': 1, '5': 9, '10': 'type'},
  ],
};

/// Descriptor for `SanctionApplied`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sanctionAppliedDescriptor = $convert.base64Decode(
    'Cg9TYW5jdGlvbkFwcGxpZWQSHwoLc2FuY3Rpb25faWQYASABKAlSCnNhbmN0aW9uSWQSKgoRdG'
    'FyZ2V0X2FjY291bnRfaWQYAiABKAlSD3RhcmdldEFjY291bnRJZBISCgR0eXBlGAMgASgJUgR0'
    'eXBl');

@$core.Deprecated('Use appealSubmittedDescriptor instead')
const AppealSubmitted$json = {
  '1': 'AppealSubmitted',
  '2': [
    {'1': 'appeal_id', '3': 1, '4': 1, '5': 9, '10': 'appealId'},
    {'1': 'sanction_id', '3': 2, '4': 1, '5': 9, '10': 'sanctionId'},
  ],
};

/// Descriptor for `AppealSubmitted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List appealSubmittedDescriptor = $convert.base64Decode(
    'Cg9BcHBlYWxTdWJtaXR0ZWQSGwoJYXBwZWFsX2lkGAEgASgJUghhcHBlYWxJZBIfCgtzYW5jdG'
    'lvbl9pZBgCIAEoCVIKc2FuY3Rpb25JZA==');

@$core.Deprecated('Use subscriptionStreamEventDescriptor instead')
const SubscriptionStreamEvent$json = {
  '1': 'SubscriptionStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'plan_started',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.PlanStarted',
      '9': 0,
      '10': 'planStarted'
    },
    {
      '1': 'plan_cancelled',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.PlanCancelled',
      '9': 0,
      '10': 'planCancelled'
    },
    {
      '1': 'payment_success',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.PaymentSuccess',
      '9': 0,
      '10': 'paymentSuccess'
    },
    {
      '1': 'payment_failed',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.PaymentFailed',
      '9': 0,
      '10': 'paymentFailed'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `SubscriptionStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List subscriptionStreamEventDescriptor = $convert.base64Decode(
    'ChdTdWJzY3JpcHRpb25TdHJlYW1FdmVudBIZCghldmVudF9pZBgBIAEoCVIHZXZlbnRJZBI7Cg'
    'tvY2N1cnJlZF9hdBgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCm9jY3VycmVk'
    'QXQSQQoMcGxhbl9zdGFydGVkGAogASgLMhwudm9pY2UuZXZlbnRzLnYxLlBsYW5TdGFydGVkSA'
    'BSC3BsYW5TdGFydGVkEkcKDnBsYW5fY2FuY2VsbGVkGAsgASgLMh4udm9pY2UuZXZlbnRzLnYx'
    'LlBsYW5DYW5jZWxsZWRIAFINcGxhbkNhbmNlbGxlZBJKCg9wYXltZW50X3N1Y2Nlc3MYDCABKA'
    'syHy52b2ljZS5ldmVudHMudjEuUGF5bWVudFN1Y2Nlc3NIAFIOcGF5bWVudFN1Y2Nlc3MSRwoO'
    'cGF5bWVudF9mYWlsZWQYDSABKAsyHi52b2ljZS5ldmVudHMudjEuUGF5bWVudEZhaWxlZEgAUg'
    '1wYXltZW50RmFpbGVkQgkKB3BheWxvYWQ=');

@$core.Deprecated('Use planStartedDescriptor instead')
const PlanStarted$json = {
  '1': 'PlanStarted',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'plan', '3': 2, '4': 1, '5': 9, '10': 'plan'},
  ],
};

/// Descriptor for `PlanStarted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List planStartedDescriptor = $convert.base64Decode(
    'CgtQbGFuU3RhcnRlZBIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW50SWQSEgoEcGxhbhgCIA'
    'EoCVIEcGxhbg==');

@$core.Deprecated('Use planCancelledDescriptor instead')
const PlanCancelled$json = {
  '1': 'PlanCancelled',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'plan', '3': 2, '4': 1, '5': 9, '10': 'plan'},
  ],
};

/// Descriptor for `PlanCancelled`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List planCancelledDescriptor = $convert.base64Decode(
    'Cg1QbGFuQ2FuY2VsbGVkEh0KCmFjY291bnRfaWQYASABKAlSCWFjY291bnRJZBISCgRwbGFuGA'
    'IgASgJUgRwbGFu');

@$core.Deprecated('Use paymentSuccessDescriptor instead')
const PaymentSuccess$json = {
  '1': 'PaymentSuccess',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'provider', '3': 2, '4': 1, '5': 9, '10': 'provider'},
  ],
};

/// Descriptor for `PaymentSuccess`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List paymentSuccessDescriptor = $convert.base64Decode(
    'Cg5QYXltZW50U3VjY2VzcxIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW50SWQSGgoIcHJvdm'
    'lkZXIYAiABKAlSCHByb3ZpZGVy');

@$core.Deprecated('Use paymentFailedDescriptor instead')
const PaymentFailed$json = {
  '1': 'PaymentFailed',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'provider', '3': 2, '4': 1, '5': 9, '10': 'provider'},
  ],
};

/// Descriptor for `PaymentFailed`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List paymentFailedDescriptor = $convert.base64Decode(
    'Cg1QYXltZW50RmFpbGVkEh0KCmFjY291bnRfaWQYASABKAlSCWFjY291bnRJZBIaCghwcm92aW'
    'RlchgCIAEoCVIIcHJvdmlkZXI=');

@$core.Deprecated('Use fileStreamEventDescriptor instead')
const FileStreamEvent$json = {
  '1': 'FileStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'file_uploaded',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.FileUploaded',
      '9': 0,
      '10': 'fileUploaded'
    },
    {
      '1': 'file_scan_result',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.FileScanResult',
      '9': 0,
      '10': 'fileScanResult'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `FileStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileStreamEventDescriptor = $convert.base64Decode(
    'Cg9GaWxlU3RyZWFtRXZlbnQSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW50SWQSOwoLb2NjdXJyZW'
    'RfYXQYAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpvY2N1cnJlZEF0EkQKDWZp'
    'bGVfdXBsb2FkZWQYCiABKAsyHS52b2ljZS5ldmVudHMudjEuRmlsZVVwbG9hZGVkSABSDGZpbG'
    'VVcGxvYWRlZBJLChBmaWxlX3NjYW5fcmVzdWx0GAsgASgLMh8udm9pY2UuZXZlbnRzLnYxLkZp'
    'bGVTY2FuUmVzdWx0SABSDmZpbGVTY2FuUmVzdWx0QgkKB3BheWxvYWQ=');

@$core.Deprecated('Use fileUploadedDescriptor instead')
const FileUploaded$json = {
  '1': 'FileUploaded',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
    {
      '1': 'uploader_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'uploaderProfileId'
    },
  ],
};

/// Descriptor for `FileUploaded`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileUploadedDescriptor = $convert.base64Decode(
    'CgxGaWxlVXBsb2FkZWQSFwoHZmlsZV9pZBgBIAEoCVIGZmlsZUlkEi4KE3VwbG9hZGVyX3Byb2'
    'ZpbGVfaWQYAiABKAlSEXVwbG9hZGVyUHJvZmlsZUlk');

@$core.Deprecated('Use fileScanResultDescriptor instead')
const FileScanResult$json = {
  '1': 'FileScanResult',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
    {'1': 'result', '3': 2, '4': 1, '5': 9, '10': 'result'},
  ],
};

/// Descriptor for `FileScanResult`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileScanResultDescriptor = $convert.base64Decode(
    'Cg5GaWxlU2NhblJlc3VsdBIXCgdmaWxlX2lkGAEgASgJUgZmaWxlSWQSFgoGcmVzdWx0GAIgAS'
    'gJUgZyZXN1bHQ=');

@$core.Deprecated('Use matchmakingStreamEventDescriptor instead')
const MatchmakingStreamEvent$json = {
  '1': 'MatchmakingStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'search_started',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.SearchStarted',
      '9': 0,
      '10': 'searchStarted'
    },
    {
      '1': 'match_found',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MatchFound',
      '9': 0,
      '10': 'matchFound'
    },
    {
      '1': 'match_timeout',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MatchTimeout',
      '9': 0,
      '10': 'matchTimeout'
    },
    {
      '1': 'rating_submitted',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.RatingSubmitted',
      '9': 0,
      '10': 'ratingSubmitted'
    },
    {
      '1': 'search_cancelled',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.SearchCancelled',
      '9': 0,
      '10': 'searchCancelled'
    },
    {
      '1': 'match_completed',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.MatchCompleted',
      '9': 0,
      '10': 'matchCompleted'
    },
    {
      '1': 'search_nudge',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.SearchNudge',
      '9': 0,
      '10': 'searchNudge'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `MatchmakingStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchmakingStreamEventDescriptor = $convert.base64Decode(
    'ChZNYXRjaG1ha2luZ1N0cmVhbUV2ZW50EhkKCGV2ZW50X2lkGAEgASgJUgdldmVudElkEjsKC2'
    '9jY3VycmVkX2F0GAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKb2NjdXJyZWRB'
    'dBJHCg5zZWFyY2hfc3RhcnRlZBgKIAEoCzIeLnZvaWNlLmV2ZW50cy52MS5TZWFyY2hTdGFydG'
    'VkSABSDXNlYXJjaFN0YXJ0ZWQSPgoLbWF0Y2hfZm91bmQYCyABKAsyGy52b2ljZS5ldmVudHMu'
    'djEuTWF0Y2hGb3VuZEgAUgptYXRjaEZvdW5kEkQKDW1hdGNoX3RpbWVvdXQYDCABKAsyHS52b2'
    'ljZS5ldmVudHMudjEuTWF0Y2hUaW1lb3V0SABSDG1hdGNoVGltZW91dBJNChByYXRpbmdfc3Vi'
    'bWl0dGVkGA0gASgLMiAudm9pY2UuZXZlbnRzLnYxLlJhdGluZ1N1Ym1pdHRlZEgAUg9yYXRpbm'
    'dTdWJtaXR0ZWQSTQoQc2VhcmNoX2NhbmNlbGxlZBgOIAEoCzIgLnZvaWNlLmV2ZW50cy52MS5T'
    'ZWFyY2hDYW5jZWxsZWRIAFIPc2VhcmNoQ2FuY2VsbGVkEkoKD21hdGNoX2NvbXBsZXRlZBgPIA'
    'EoCzIfLnZvaWNlLmV2ZW50cy52MS5NYXRjaENvbXBsZXRlZEgAUg5tYXRjaENvbXBsZXRlZBJB'
    'CgxzZWFyY2hfbnVkZ2UYECABKAsyHC52b2ljZS5ldmVudHMudjEuU2VhcmNoTnVkZ2VIAFILc2'
    'VhcmNoTnVkZ2VCCQoHcGF5bG9hZA==');

@$core.Deprecated('Use searchStartedDescriptor instead')
const SearchStarted$json = {
  '1': 'SearchStarted',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'game_id', '3': 3, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'mode', '3': 4, '4': 1, '5': 9, '10': 'mode'},
    {'1': 'region', '3': 5, '4': 1, '5': 9, '10': 'region'},
  ],
};

/// Descriptor for `SearchStarted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchStartedDescriptor = $convert.base64Decode(
    'Cg1TZWFyY2hTdGFydGVkEh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb25JZBIdCgpwcm9maW'
    'xlX2lkGAIgASgJUglwcm9maWxlSWQSFwoHZ2FtZV9pZBgDIAEoCVIGZ2FtZUlkEhIKBG1vZGUY'
    'BCABKAlSBG1vZGUSFgoGcmVnaW9uGAUgASgJUgZyZWdpb24=');

@$core.Deprecated('Use searchCancelledDescriptor instead')
const SearchCancelled$json = {
  '1': 'SearchCancelled',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `SearchCancelled`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchCancelledDescriptor = $convert.base64Decode(
    'Cg9TZWFyY2hDYW5jZWxsZWQSHQoKc2Vzc2lvbl9pZBgBIAEoCVIJc2Vzc2lvbklkEh0KCnByb2'
    'ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZA==');

@$core.Deprecated('Use matchFoundDescriptor instead')
const MatchFound$json = {
  '1': 'MatchFound',
  '2': [
    {'1': 'match_id', '3': 1, '4': 1, '5': 9, '10': 'matchId'},
    {'1': 'profile_ids', '3': 2, '4': 3, '5': 9, '10': 'profileIds'},
    {'1': 'game_id', '3': 3, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'mode', '3': 4, '4': 1, '5': 9, '10': 'mode'},
    {'1': 'region', '3': 5, '4': 1, '5': 9, '10': 'region'},
    {'1': 'session_ids', '3': 6, '4': 3, '5': 9, '10': 'sessionIds'},
    {
      '1': 'chat_id',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'chatId',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 8,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
  ],
  '8': [
    {'1': '_chat_id'},
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `MatchFound`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchFoundDescriptor = $convert.base64Decode(
    'CgpNYXRjaEZvdW5kEhkKCG1hdGNoX2lkGAEgASgJUgdtYXRjaElkEh8KC3Byb2ZpbGVfaWRzGA'
    'IgAygJUgpwcm9maWxlSWRzEhcKB2dhbWVfaWQYAyABKAlSBmdhbWVJZBISCgRtb2RlGAQgASgJ'
    'UgRtb2RlEhYKBnJlZ2lvbhgFIAEoCVIGcmVnaW9uEh8KC3Nlc3Npb25faWRzGAYgAygJUgpzZX'
    'NzaW9uSWRzEhwKB2NoYXRfaWQYByABKAlIAFIGY2hhdElkiAEBEicKDXZvaWNlX3Jvb21faWQY'
    'CCABKAlIAVILdm9pY2VSb29tSWSIAQFCCgoIX2NoYXRfaWRCEAoOX3ZvaWNlX3Jvb21faWQ=');

@$core.Deprecated('Use matchTimeoutDescriptor instead')
const MatchTimeout$json = {
  '1': 'MatchTimeout',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'game_id', '3': 3, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'mode', '3': 4, '4': 1, '5': 9, '10': 'mode'},
  ],
};

/// Descriptor for `MatchTimeout`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchTimeoutDescriptor = $convert.base64Decode(
    'CgxNYXRjaFRpbWVvdXQSHQoKc2Vzc2lvbl9pZBgBIAEoCVIJc2Vzc2lvbklkEh0KCnByb2ZpbG'
    'VfaWQYAiABKAlSCXByb2ZpbGVJZBIXCgdnYW1lX2lkGAMgASgJUgZnYW1lSWQSEgoEbW9kZRgE'
    'IAEoCVIEbW9kZQ==');

@$core.Deprecated('Use searchNudgeDescriptor instead')
const SearchNudge$json = {
  '1': 'SearchNudge',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'game_id', '3': 3, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'mode', '3': 4, '4': 1, '5': 9, '10': 'mode'},
  ],
};

/// Descriptor for `SearchNudge`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchNudgeDescriptor = $convert.base64Decode(
    'CgtTZWFyY2hOdWRnZRIdCgpzZXNzaW9uX2lkGAEgASgJUglzZXNzaW9uSWQSHQoKcHJvZmlsZV'
    '9pZBgCIAEoCVIJcHJvZmlsZUlkEhcKB2dhbWVfaWQYAyABKAlSBmdhbWVJZBISCgRtb2RlGAQg'
    'ASgJUgRtb2Rl');

@$core.Deprecated('Use matchCompletedDescriptor instead')
const MatchCompleted$json = {
  '1': 'MatchCompleted',
  '2': [
    {'1': 'match_id', '3': 1, '4': 1, '5': 9, '10': 'matchId'},
    {'1': 'duration_seconds', '3': 2, '4': 1, '5': 3, '10': 'durationSeconds'},
    {'1': 'profile_ids', '3': 3, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `MatchCompleted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchCompletedDescriptor = $convert.base64Decode(
    'Cg5NYXRjaENvbXBsZXRlZBIZCghtYXRjaF9pZBgBIAEoCVIHbWF0Y2hJZBIpChBkdXJhdGlvbl'
    '9zZWNvbmRzGAIgASgDUg9kdXJhdGlvblNlY29uZHMSHwoLcHJvZmlsZV9pZHMYAyADKAlSCnBy'
    'b2ZpbGVJZHM=');

@$core.Deprecated('Use ratingSubmittedDescriptor instead')
const RatingSubmitted$json = {
  '1': 'RatingSubmitted',
  '2': [
    {'1': 'match_id', '3': 1, '4': 1, '5': 9, '10': 'matchId'},
    {'1': 'rater_profile_id', '3': 2, '4': 1, '5': 9, '10': 'raterProfileId'},
    {'1': 'rated_profile_id', '3': 3, '4': 1, '5': 9, '10': 'ratedProfileId'},
    {'1': 'stars', '3': 4, '4': 1, '5': 5, '10': 'stars'},
  ],
};

/// Descriptor for `RatingSubmitted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ratingSubmittedDescriptor = $convert.base64Decode(
    'Cg9SYXRpbmdTdWJtaXR0ZWQSGQoIbWF0Y2hfaWQYASABKAlSB21hdGNoSWQSKAoQcmF0ZXJfcH'
    'JvZmlsZV9pZBgCIAEoCVIOcmF0ZXJQcm9maWxlSWQSKAoQcmF0ZWRfcHJvZmlsZV9pZBgDIAEo'
    'CVIOcmF0ZWRQcm9maWxlSWQSFAoFc3RhcnMYBCABKAVSBXN0YXJz');

@$core.Deprecated('Use storyStreamEventDescriptor instead')
const StoryStreamEvent$json = {
  '1': 'StoryStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'story_created',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.StoryCreated',
      '9': 0,
      '10': 'storyCreated'
    },
    {
      '1': 'story_viewed',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.StoryViewed',
      '9': 0,
      '10': 'storyViewed'
    },
    {
      '1': 'highlight_added',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.HighlightAdded',
      '9': 0,
      '10': 'highlightAdded'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `StoryStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storyStreamEventDescriptor = $convert.base64Decode(
    'ChBTdG9yeVN0cmVhbUV2ZW50EhkKCGV2ZW50X2lkGAEgASgJUgdldmVudElkEjsKC29jY3Vycm'
    'VkX2F0GAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKb2NjdXJyZWRBdBJECg1z'
    'dG9yeV9jcmVhdGVkGAogASgLMh0udm9pY2UuZXZlbnRzLnYxLlN0b3J5Q3JlYXRlZEgAUgxzdG'
    '9yeUNyZWF0ZWQSQQoMc3Rvcnlfdmlld2VkGAsgASgLMhwudm9pY2UuZXZlbnRzLnYxLlN0b3J5'
    'Vmlld2VkSABSC3N0b3J5Vmlld2VkEkoKD2hpZ2hsaWdodF9hZGRlZBgMIAEoCzIfLnZvaWNlLm'
    'V2ZW50cy52MS5IaWdobGlnaHRBZGRlZEgAUg5oaWdobGlnaHRBZGRlZEIJCgdwYXlsb2Fk');

@$core.Deprecated('Use storyCreatedDescriptor instead')
const StoryCreated$json = {
  '1': 'StoryCreated',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
    {'1': 'author_profile_id', '3': 2, '4': 1, '5': 9, '10': 'authorProfileId'},
  ],
};

/// Descriptor for `StoryCreated`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storyCreatedDescriptor = $convert.base64Decode(
    'CgxTdG9yeUNyZWF0ZWQSGQoIc3RvcnlfaWQYASABKAlSB3N0b3J5SWQSKgoRYXV0aG9yX3Byb2'
    'ZpbGVfaWQYAiABKAlSD2F1dGhvclByb2ZpbGVJZA==');

@$core.Deprecated('Use storyViewedDescriptor instead')
const StoryViewed$json = {
  '1': 'StoryViewed',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
    {'1': 'viewer_profile_id', '3': 2, '4': 1, '5': 9, '10': 'viewerProfileId'},
  ],
};

/// Descriptor for `StoryViewed`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storyViewedDescriptor = $convert.base64Decode(
    'CgtTdG9yeVZpZXdlZBIZCghzdG9yeV9pZBgBIAEoCVIHc3RvcnlJZBIqChF2aWV3ZXJfcHJvZm'
    'lsZV9pZBgCIAEoCVIPdmlld2VyUHJvZmlsZUlk');

@$core.Deprecated('Use highlightAddedDescriptor instead')
const HighlightAdded$json = {
  '1': 'HighlightAdded',
  '2': [
    {'1': 'highlight_id', '3': 1, '4': 1, '5': 9, '10': 'highlightId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `HighlightAdded`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List highlightAddedDescriptor = $convert.base64Decode(
    'Cg5IaWdobGlnaHRBZGRlZBIhCgxoaWdobGlnaHRfaWQYASABKAlSC2hpZ2hsaWdodElkEh0KCn'
    'Byb2ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZA==');

@$core.Deprecated('Use federationStreamEventDescriptor instead')
const FederationStreamEvent$json = {
  '1': 'FederationStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'node_connected',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.NodeConnected',
      '9': 0,
      '10': 'nodeConnected'
    },
    {
      '1': 'node_disconnected',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.NodeDisconnected',
      '9': 0,
      '10': 'nodeDisconnected'
    },
    {
      '1': 'event_synced',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.EventSynced',
      '9': 0,
      '10': 'eventSynced'
    },
    {
      '1': 'sync_failed',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.SyncFailed',
      '9': 0,
      '10': 'syncFailed'
    },
    {
      '1': 'node_defederated',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.NodeDefederated',
      '9': 0,
      '10': 'nodeDefederated'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `FederationStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List federationStreamEventDescriptor = $convert.base64Decode(
    'ChVGZWRlcmF0aW9uU3RyZWFtRXZlbnQSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW50SWQSOwoLb2'
    'NjdXJyZWRfYXQYAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpvY2N1cnJlZEF0'
    'EkcKDm5vZGVfY29ubmVjdGVkGAogASgLMh4udm9pY2UuZXZlbnRzLnYxLk5vZGVDb25uZWN0ZW'
    'RIAFINbm9kZUNvbm5lY3RlZBJQChFub2RlX2Rpc2Nvbm5lY3RlZBgLIAEoCzIhLnZvaWNlLmV2'
    'ZW50cy52MS5Ob2RlRGlzY29ubmVjdGVkSABSEG5vZGVEaXNjb25uZWN0ZWQSQQoMZXZlbnRfc3'
    'luY2VkGAwgASgLMhwudm9pY2UuZXZlbnRzLnYxLkV2ZW50U3luY2VkSABSC2V2ZW50U3luY2Vk'
    'Ej4KC3N5bmNfZmFpbGVkGA0gASgLMhsudm9pY2UuZXZlbnRzLnYxLlN5bmNGYWlsZWRIAFIKc3'
    'luY0ZhaWxlZBJNChBub2RlX2RlZmVkZXJhdGVkGA4gASgLMiAudm9pY2UuZXZlbnRzLnYxLk5v'
    'ZGVEZWZlZGVyYXRlZEgAUg9ub2RlRGVmZWRlcmF0ZWRCCQoHcGF5bG9hZA==');

@$core.Deprecated('Use nodeConnectedDescriptor instead')
const NodeConnected$json = {
  '1': 'NodeConnected',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'host', '3': 2, '4': 1, '5': 9, '10': 'host'},
  ],
};

/// Descriptor for `NodeConnected`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List nodeConnectedDescriptor = $convert.base64Decode(
    'Cg1Ob2RlQ29ubmVjdGVkEhcKB25vZGVfaWQYASABKAlSBm5vZGVJZBISCgRob3N0GAIgASgJUg'
    'Rob3N0');

@$core.Deprecated('Use nodeDisconnectedDescriptor instead')
const NodeDisconnected$json = {
  '1': 'NodeDisconnected',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `NodeDisconnected`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List nodeDisconnectedDescriptor = $convert.base64Decode(
    'ChBOb2RlRGlzY29ubmVjdGVkEhcKB25vZGVfaWQYASABKAlSBm5vZGVJZBIWCgZyZWFzb24YAi'
    'ABKAlSBnJlYXNvbg==');

@$core.Deprecated('Use eventSyncedDescriptor instead')
const EventSynced$json = {
  '1': 'EventSynced',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'event_type', '3': 2, '4': 1, '5': 9, '10': 'eventType'},
    {'1': 'direction', '3': 3, '4': 1, '5': 9, '10': 'direction'},
  ],
};

/// Descriptor for `EventSynced`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List eventSyncedDescriptor = $convert.base64Decode(
    'CgtFdmVudFN5bmNlZBIXCgdub2RlX2lkGAEgASgJUgZub2RlSWQSHQoKZXZlbnRfdHlwZRgCIA'
    'EoCVIJZXZlbnRUeXBlEhwKCWRpcmVjdGlvbhgDIAEoCVIJZGlyZWN0aW9u');

@$core.Deprecated('Use syncFailedDescriptor instead')
const SyncFailed$json = {
  '1': 'SyncFailed',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'error', '3': 2, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `SyncFailed`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List syncFailedDescriptor = $convert.base64Decode(
    'CgpTeW5jRmFpbGVkEhcKB25vZGVfaWQYASABKAlSBm5vZGVJZBIUCgVlcnJvchgCIAEoCVIFZX'
    'Jyb3I=');

@$core.Deprecated('Use nodeDefederatedDescriptor instead')
const NodeDefederated$json = {
  '1': 'NodeDefederated',
  '2': [
    {'1': 'node_id', '3': 1, '4': 1, '5': 9, '10': 'nodeId'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `NodeDefederated`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List nodeDefederatedDescriptor = $convert.base64Decode(
    'Cg9Ob2RlRGVmZWRlcmF0ZWQSFwoHbm9kZV9pZBgBIAEoCVIGbm9kZUlkEhYKBnJlYXNvbhgCIA'
    'EoCVIGcmVhc29u');

@$core.Deprecated('Use botStreamEventDescriptor instead')
const BotStreamEvent$json = {
  '1': 'BotStreamEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'occurred_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'bot_registered',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.BotRegistered',
      '9': 0,
      '10': 'botRegistered'
    },
    {
      '1': 'command_executed',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.CommandExecuted',
      '9': 0,
      '10': 'commandExecuted'
    },
    {
      '1': 'webhook_delivered',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.events.v1.WebhookDelivered',
      '9': 0,
      '10': 'webhookDelivered'
    },
  ],
  '8': [
    {'1': 'payload'},
  ],
};

/// Descriptor for `BotStreamEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botStreamEventDescriptor = $convert.base64Decode(
    'Cg5Cb3RTdHJlYW1FdmVudBIZCghldmVudF9pZBgBIAEoCVIHZXZlbnRJZBI7CgtvY2N1cnJlZF'
    '9hdBgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCm9jY3VycmVkQXQSRwoOYm90'
    'X3JlZ2lzdGVyZWQYCiABKAsyHi52b2ljZS5ldmVudHMudjEuQm90UmVnaXN0ZXJlZEgAUg1ib3'
    'RSZWdpc3RlcmVkEk0KEGNvbW1hbmRfZXhlY3V0ZWQYCyABKAsyIC52b2ljZS5ldmVudHMudjEu'
    'Q29tbWFuZEV4ZWN1dGVkSABSD2NvbW1hbmRFeGVjdXRlZBJQChF3ZWJob29rX2RlbGl2ZXJlZB'
    'gMIAEoCzIhLnZvaWNlLmV2ZW50cy52MS5XZWJob29rRGVsaXZlcmVkSABSEHdlYmhvb2tEZWxp'
    'dmVyZWRCCQoHcGF5bG9hZA==');

@$core.Deprecated('Use botRegisteredDescriptor instead')
const BotRegistered$json = {
  '1': 'BotRegistered',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'owner_account_id', '3': 2, '4': 1, '5': 9, '10': 'ownerAccountId'},
  ],
};

/// Descriptor for `BotRegistered`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botRegisteredDescriptor = $convert.base64Decode(
    'Cg1Cb3RSZWdpc3RlcmVkEhUKBmJvdF9pZBgBIAEoCVIFYm90SWQSKAoQb3duZXJfYWNjb3VudF'
    '9pZBgCIAEoCVIOb3duZXJBY2NvdW50SWQ=');

@$core.Deprecated('Use commandExecutedDescriptor instead')
const CommandExecuted$json = {
  '1': 'CommandExecuted',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'command', '3': 2, '4': 1, '5': 9, '10': 'command'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `CommandExecuted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List commandExecutedDescriptor = $convert.base64Decode(
    'Cg9Db21tYW5kRXhlY3V0ZWQSFQoGYm90X2lkGAEgASgJUgVib3RJZBIYCgdjb21tYW5kGAIgAS'
    'gJUgdjb21tYW5kEhcKB2NoYXRfaWQYAyABKAlSBmNoYXRJZA==');

@$core.Deprecated('Use webhookDeliveredDescriptor instead')
const WebhookDelivered$json = {
  '1': 'WebhookDelivered',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'delivery_id', '3': 2, '4': 1, '5': 9, '10': 'deliveryId'},
    {'1': 'success', '3': 3, '4': 1, '5': 8, '10': 'success'},
  ],
};

/// Descriptor for `WebhookDelivered`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List webhookDeliveredDescriptor = $convert.base64Decode(
    'ChBXZWJob29rRGVsaXZlcmVkEhUKBmJvdF9pZBgBIAEoCVIFYm90SWQSHwoLZGVsaXZlcnlfaW'
    'QYAiABKAlSCmRlbGl2ZXJ5SWQSGAoHc3VjY2VzcxgDIAEoCFIHc3VjY2Vzcw==');
