// This is a generated file - do not edit.
//
// Generated from voice/notification/v1/notification.proto.

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

@$core.Deprecated('Use devicePlatformDescriptor instead')
const DevicePlatform$json = {
  '1': 'DevicePlatform',
  '2': [
    {'1': 'DEVICE_PLATFORM_UNSPECIFIED', '2': 0},
    {'1': 'DEVICE_PLATFORM_ANDROID', '2': 1},
    {'1': 'DEVICE_PLATFORM_IOS', '2': 2},
    {'1': 'DEVICE_PLATFORM_WEB', '2': 3},
    {'1': 'DEVICE_PLATFORM_DESKTOP', '2': 4},
  ],
};

/// Descriptor for `DevicePlatform`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List devicePlatformDescriptor = $convert.base64Decode(
    'Cg5EZXZpY2VQbGF0Zm9ybRIfChtERVZJQ0VfUExBVEZPUk1fVU5TUEVDSUZJRUQQABIbChdERV'
    'ZJQ0VfUExBVEZPUk1fQU5EUk9JRBABEhcKE0RFVklDRV9QTEFURk9STV9JT1MQAhIXChNERVZJ'
    'Q0VfUExBVEZPUk1fV0VCEAMSGwoXREVWSUNFX1BMQVRGT1JNX0RFU0tUT1AQBA==');

@$core.Deprecated('Use pushServiceDescriptor instead')
const PushService$json = {
  '1': 'PushService',
  '2': [
    {'1': 'PUSH_SERVICE_UNSPECIFIED', '2': 0},
    {'1': 'PUSH_SERVICE_FCM', '2': 1},
    {'1': 'PUSH_SERVICE_APNS', '2': 2},
    {'1': 'PUSH_SERVICE_VOIP_APNS', '2': 3},
  ],
};

/// Descriptor for `PushService`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List pushServiceDescriptor = $convert.base64Decode(
    'CgtQdXNoU2VydmljZRIcChhQVVNIX1NFUlZJQ0VfVU5TUEVDSUZJRUQQABIUChBQVVNIX1NFUl'
    'ZJQ0VfRkNNEAESFQoRUFVTSF9TRVJWSUNFX0FQTlMQAhIaChZQVVNIX1NFUlZJQ0VfVk9JUF9B'
    'UE5TEAM=');

@$core.Deprecated('Use notificationScopeTypeDescriptor instead')
const NotificationScopeType$json = {
  '1': 'NotificationScopeType',
  '2': [
    {'1': 'NOTIFICATION_SCOPE_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'NOTIFICATION_SCOPE_TYPE_GLOBAL', '2': 1},
    {'1': 'NOTIFICATION_SCOPE_TYPE_SPACE', '2': 2},
    {'1': 'NOTIFICATION_SCOPE_TYPE_CHANNEL', '2': 3},
    {'1': 'NOTIFICATION_SCOPE_TYPE_CHAT', '2': 4},
  ],
};

/// Descriptor for `NotificationScopeType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List notificationScopeTypeDescriptor = $convert.base64Decode(
    'ChVOb3RpZmljYXRpb25TY29wZVR5cGUSJwojTk9USUZJQ0FUSU9OX1NDT1BFX1RZUEVfVU5TUE'
    'VDSUZJRUQQABIiCh5OT1RJRklDQVRJT05fU0NPUEVfVFlQRV9HTE9CQUwQARIhCh1OT1RJRklD'
    'QVRJT05fU0NPUEVfVFlQRV9TUEFDRRACEiMKH05PVElGSUNBVElPTl9TQ09QRV9UWVBFX0NIQU'
    '5ORUwQAxIgChxOT1RJRklDQVRJT05fU0NPUEVfVFlQRV9DSEFUEAQ=');

@$core.Deprecated('Use clientNotificationCategoryDescriptor instead')
const ClientNotificationCategory$json = {
  '1': 'ClientNotificationCategory',
  '2': [
    {'1': 'CLIENT_NOTIFICATION_CATEGORY_UNSPECIFIED', '2': 0},
    {'1': 'CLIENT_NOTIFICATION_CATEGORY_MESSAGE', '2': 1},
    {'1': 'CLIENT_NOTIFICATION_CATEGORY_DM', '2': 2},
    {'1': 'CLIENT_NOTIFICATION_CATEGORY_FRIEND', '2': 3},
    {'1': 'CLIENT_NOTIFICATION_CATEGORY_CALL', '2': 4},
    {'1': 'CLIENT_NOTIFICATION_CATEGORY_MATCH', '2': 5},
    {'1': 'CLIENT_NOTIFICATION_CATEGORY_SPACE', '2': 6},
    {'1': 'CLIENT_NOTIFICATION_CATEGORY_SYSTEM', '2': 7},
  ],
};

/// Descriptor for `ClientNotificationCategory`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List clientNotificationCategoryDescriptor = $convert.base64Decode(
    'ChpDbGllbnROb3RpZmljYXRpb25DYXRlZ29yeRIsCihDTElFTlRfTk9USUZJQ0FUSU9OX0NBVE'
    'VHT1JZX1VOU1BFQ0lGSUVEEAASKAokQ0xJRU5UX05PVElGSUNBVElPTl9DQVRFR09SWV9NRVNT'
    'QUdFEAESIwofQ0xJRU5UX05PVElGSUNBVElPTl9DQVRFR09SWV9ETRACEicKI0NMSUVOVF9OT1'
    'RJRklDQVRJT05fQ0FURUdPUllfRlJJRU5EEAMSJQohQ0xJRU5UX05PVElGSUNBVElPTl9DQVRF'
    'R09SWV9DQUxMEAQSJgoiQ0xJRU5UX05PVElGSUNBVElPTl9DQVRFR09SWV9NQVRDSBAFEiYKIk'
    'NMSUVOVF9OT1RJRklDQVRJT05fQ0FURUdPUllfU1BBQ0UQBhInCiNDTElFTlRfTk9USUZJQ0FU'
    'SU9OX0NBVEVHT1JZX1NZU1RFTRAH');

@$core.Deprecated('Use registerDeviceRequestDescriptor instead')
const RegisterDeviceRequest$json = {
  '1': 'RegisterDeviceRequest',
  '2': [
    {'1': 'platform', '3': 1, '4': 1, '5': 9, '10': 'platform'},
    {'1': 'token', '3': 2, '4': 1, '5': 9, '10': 'token'},
    {'1': 'push_service', '3': 3, '4': 1, '5': 9, '10': 'pushService'},
    {
      '1': 'platform_enum',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.notification.v1.DevicePlatform',
      '9': 0,
      '10': 'platformEnum',
      '17': true
    },
    {
      '1': 'push_service_enum',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.voice.notification.v1.PushService',
      '9': 1,
      '10': 'pushServiceEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_platform_enum'},
    {'1': '_push_service_enum'},
  ],
};

/// Descriptor for `RegisterDeviceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerDeviceRequestDescriptor = $convert.base64Decode(
    'ChVSZWdpc3RlckRldmljZVJlcXVlc3QSGgoIcGxhdGZvcm0YASABKAlSCHBsYXRmb3JtEhQKBX'
    'Rva2VuGAIgASgJUgV0b2tlbhIhCgxwdXNoX3NlcnZpY2UYAyABKAlSC3B1c2hTZXJ2aWNlEk8K'
    'DXBsYXRmb3JtX2VudW0YBCABKA4yJS52b2ljZS5ub3RpZmljYXRpb24udjEuRGV2aWNlUGxhdG'
    'Zvcm1IAFIMcGxhdGZvcm1FbnVtiAEBElMKEXB1c2hfc2VydmljZV9lbnVtGAUgASgOMiIudm9p'
    'Y2Uubm90aWZpY2F0aW9uLnYxLlB1c2hTZXJ2aWNlSAFSD3B1c2hTZXJ2aWNlRW51bYgBAUIQCg'
    '5fcGxhdGZvcm1fZW51bUIUChJfcHVzaF9zZXJ2aWNlX2VudW0=');

@$core.Deprecated('Use unregisterDeviceRequestDescriptor instead')
const UnregisterDeviceRequest$json = {
  '1': 'UnregisterDeviceRequest',
  '2': [
    {'1': 'device_token_id', '3': 1, '4': 1, '5': 9, '10': 'deviceTokenId'},
  ],
};

/// Descriptor for `UnregisterDeviceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unregisterDeviceRequestDescriptor =
    $convert.base64Decode(
        'ChdVbnJlZ2lzdGVyRGV2aWNlUmVxdWVzdBImCg9kZXZpY2VfdG9rZW5faWQYASABKAlSDWRldm'
        'ljZVRva2VuSWQ=');

@$core.Deprecated('Use getNotificationSettingsRequestDescriptor instead')
const GetNotificationSettingsRequest$json = {
  '1': 'GetNotificationSettingsRequest',
  '2': [
    {
      '1': 'scope_type',
      '3': 1,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'scopeType',
      '17': true
    },
    {
      '1': 'scope_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'scopeId',
      '17': true
    },
    {
      '1': 'scope_type_enum',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.notification.v1.NotificationScopeType',
      '9': 2,
      '10': 'scopeTypeEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_scope_type'},
    {'1': '_scope_id'},
    {'1': '_scope_type_enum'},
  ],
};

/// Descriptor for `GetNotificationSettingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getNotificationSettingsRequestDescriptor = $convert.base64Decode(
    'Ch5HZXROb3RpZmljYXRpb25TZXR0aW5nc1JlcXVlc3QSIgoKc2NvcGVfdHlwZRgBIAEoCUgAUg'
    'lzY29wZVR5cGWIAQESHgoIc2NvcGVfaWQYAiABKAlIAVIHc2NvcGVJZIgBARJZCg9zY29wZV90'
    'eXBlX2VudW0YAyABKA4yLC52b2ljZS5ub3RpZmljYXRpb24udjEuTm90aWZpY2F0aW9uU2NvcG'
    'VUeXBlSAJSDXNjb3BlVHlwZUVudW2IAQFCDQoLX3Njb3BlX3R5cGVCCwoJX3Njb3BlX2lkQhIK'
    'EF9zY29wZV90eXBlX2VudW0=');

@$core.Deprecated('Use updateNotificationSettingsRequestDescriptor instead')
const UpdateNotificationSettingsRequest$json = {
  '1': 'UpdateNotificationSettingsRequest',
  '2': [
    {
      '1': 'settings',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.notification.v1.NotificationSettings',
      '10': 'settings'
    },
  ],
};

/// Descriptor for `UpdateNotificationSettingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateNotificationSettingsRequestDescriptor =
    $convert.base64Decode(
        'CiFVcGRhdGVOb3RpZmljYXRpb25TZXR0aW5nc1JlcXVlc3QSRwoIc2V0dGluZ3MYASABKAsyKy'
        '52b2ljZS5ub3RpZmljYXRpb24udjEuTm90aWZpY2F0aW9uU2V0dGluZ3NSCHNldHRpbmdz');

@$core.Deprecated('Use notificationSettingsDescriptor instead')
const NotificationSettings$json = {
  '1': 'NotificationSettings',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'scope_type', '3': 2, '4': 1, '5': 9, '10': 'scopeType'},
    {
      '1': 'scope_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'scopeId',
      '17': true
    },
    {'1': 'enabled', '3': 4, '4': 1, '5': 8, '10': 'enabled'},
    {
      '1': 'suppress_types_json',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'suppressTypesJson'
    },
    {
      '1': 'mute_until',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'muteUntil',
      '17': true
    },
    {
      '1': 'scope_type_enum',
      '3': 8,
      '4': 1,
      '5': 14,
      '6': '.voice.notification.v1.NotificationScopeType',
      '9': 2,
      '10': 'scopeTypeEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_scope_id'},
    {'1': '_mute_until'},
    {'1': '_scope_type_enum'},
  ],
  '9': [
    {'1': 5, '2': 6},
  ],
  '10': ['mute_until_rfc3339'],
};

/// Descriptor for `NotificationSettings`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List notificationSettingsDescriptor = $convert.base64Decode(
    'ChROb3RpZmljYXRpb25TZXR0aW5ncxIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQSHQ'
    'oKc2NvcGVfdHlwZRgCIAEoCVIJc2NvcGVUeXBlEh4KCHNjb3BlX2lkGAMgASgJSABSB3Njb3Bl'
    'SWSIAQESGAoHZW5hYmxlZBgEIAEoCFIHZW5hYmxlZBIuChNzdXBwcmVzc190eXBlc19qc29uGA'
    'YgASgJUhFzdXBwcmVzc1R5cGVzSnNvbhI+CgptdXRlX3VudGlsGAcgASgLMhouZ29vZ2xlLnBy'
    'b3RvYnVmLlRpbWVzdGFtcEgBUgltdXRlVW50aWyIAQESWQoPc2NvcGVfdHlwZV9lbnVtGAggAS'
    'gOMiwudm9pY2Uubm90aWZpY2F0aW9uLnYxLk5vdGlmaWNhdGlvblNjb3BlVHlwZUgCUg1zY29w'
    'ZVR5cGVFbnVtiAEBQgsKCV9zY29wZV9pZEINCgtfbXV0ZV91bnRpbEISChBfc2NvcGVfdHlwZV'
    '9lbnVtSgQIBRAGUhJtdXRlX3VudGlsX3JmYzMzMzk=');

@$core.Deprecated('Use setQuietHoursRequestDescriptor instead')
const SetQuietHoursRequest$json = {
  '1': 'SetQuietHoursRequest',
  '2': [
    {'1': 'enabled', '3': 1, '4': 1, '5': 8, '10': 'enabled'},
    {'1': 'start_time', '3': 2, '4': 1, '5': 9, '10': 'startTime'},
    {'1': 'end_time', '3': 3, '4': 1, '5': 9, '10': 'endTime'},
    {'1': 'timezone', '3': 4, '4': 1, '5': 9, '10': 'timezone'},
    {
      '1': 'override_mentions',
      '3': 5,
      '4': 1,
      '5': 8,
      '10': 'overrideMentions'
    },
  ],
};

/// Descriptor for `SetQuietHoursRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setQuietHoursRequestDescriptor = $convert.base64Decode(
    'ChRTZXRRdWlldEhvdXJzUmVxdWVzdBIYCgdlbmFibGVkGAEgASgIUgdlbmFibGVkEh0KCnN0YX'
    'J0X3RpbWUYAiABKAlSCXN0YXJ0VGltZRIZCghlbmRfdGltZRgDIAEoCVIHZW5kVGltZRIaCgh0'
    'aW1lem9uZRgEIAEoCVIIdGltZXpvbmUSKwoRb3ZlcnJpZGVfbWVudGlvbnMYBSABKAhSEG92ZX'
    'JyaWRlTWVudGlvbnM=');

@$core.Deprecated('Use sendNotificationRequestDescriptor instead')
const SendNotificationRequest$json = {
  '1': 'SendNotificationRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'notification_type',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'notificationType'
    },
    {'1': 'title', '3': 3, '4': 1, '5': 9, '10': 'title'},
    {'1': 'body', '3': 4, '4': 1, '5': 9, '10': 'body'},
    {'1': 'payload_json', '3': 5, '4': 1, '5': 9, '10': 'payloadJson'},
    {
      '1': 'notification_category',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.notification.v1.ClientNotificationCategory',
      '9': 0,
      '10': 'notificationCategory',
      '17': true
    },
  ],
  '8': [
    {'1': '_notification_category'},
  ],
};

/// Descriptor for `SendNotificationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendNotificationRequestDescriptor = $convert.base64Decode(
    'ChdTZW5kTm90aWZpY2F0aW9uUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSW'
    'QSKwoRbm90aWZpY2F0aW9uX3R5cGUYAiABKAlSEG5vdGlmaWNhdGlvblR5cGUSFAoFdGl0bGUY'
    'AyABKAlSBXRpdGxlEhIKBGJvZHkYBCABKAlSBGJvZHkSIQoMcGF5bG9hZF9qc29uGAUgASgJUg'
    'twYXlsb2FkSnNvbhJrChVub3RpZmljYXRpb25fY2F0ZWdvcnkYBiABKA4yMS52b2ljZS5ub3Rp'
    'ZmljYXRpb24udjEuQ2xpZW50Tm90aWZpY2F0aW9uQ2F0ZWdvcnlIAFIUbm90aWZpY2F0aW9uQ2'
    'F0ZWdvcnmIAQFCGAoWX25vdGlmaWNhdGlvbl9jYXRlZ29yeQ==');

@$core.Deprecated('Use sendBulkNotificationRequestDescriptor instead')
const SendBulkNotificationRequest$json = {
  '1': 'SendBulkNotificationRequest',
  '2': [
    {'1': 'profile_ids', '3': 1, '4': 3, '5': 9, '10': 'profileIds'},
    {
      '1': 'notification_type',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'notificationType'
    },
    {'1': 'title', '3': 3, '4': 1, '5': 9, '10': 'title'},
    {'1': 'body', '3': 4, '4': 1, '5': 9, '10': 'body'},
    {'1': 'payload_json', '3': 5, '4': 1, '5': 9, '10': 'payloadJson'},
    {
      '1': 'notification_category',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.notification.v1.ClientNotificationCategory',
      '9': 0,
      '10': 'notificationCategory',
      '17': true
    },
  ],
  '8': [
    {'1': '_notification_category'},
  ],
};

/// Descriptor for `SendBulkNotificationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendBulkNotificationRequestDescriptor = $convert.base64Decode(
    'ChtTZW5kQnVsa05vdGlmaWNhdGlvblJlcXVlc3QSHwoLcHJvZmlsZV9pZHMYASADKAlSCnByb2'
    'ZpbGVJZHMSKwoRbm90aWZpY2F0aW9uX3R5cGUYAiABKAlSEG5vdGlmaWNhdGlvblR5cGUSFAoF'
    'dGl0bGUYAyABKAlSBXRpdGxlEhIKBGJvZHkYBCABKAlSBGJvZHkSIQoMcGF5bG9hZF9qc29uGA'
    'UgASgJUgtwYXlsb2FkSnNvbhJrChVub3RpZmljYXRpb25fY2F0ZWdvcnkYBiABKA4yMS52b2lj'
    'ZS5ub3RpZmljYXRpb24udjEuQ2xpZW50Tm90aWZpY2F0aW9uQ2F0ZWdvcnlIAFIUbm90aWZpY2'
    'F0aW9uQ2F0ZWdvcnmIAQFCGAoWX25vdGlmaWNhdGlvbl9jYXRlZ29yeQ==');

@$core.Deprecated('Use relayNotificationRequestDescriptor instead')
const RelayNotificationRequest$json = {
  '1': 'RelayNotificationRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'payload_json', '3': 2, '4': 1, '5': 9, '10': 'payloadJson'},
  ],
};

/// Descriptor for `RelayNotificationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List relayNotificationRequestDescriptor =
    $convert.base64Decode(
        'ChhSZWxheU5vdGlmaWNhdGlvblJlcXVlc3QSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudE'
        'lkEiEKDHBheWxvYWRfanNvbhgCIAEoCVILcGF5bG9hZEpzb24=');

@$core.Deprecated('Use registerDeviceResponseDescriptor instead')
const RegisterDeviceResponse$json = {
  '1': 'RegisterDeviceResponse',
  '2': [
    {'1': 'device_token_id', '3': 1, '4': 1, '5': 9, '10': 'deviceTokenId'},
  ],
};

/// Descriptor for `RegisterDeviceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerDeviceResponseDescriptor =
    $convert.base64Decode(
        'ChZSZWdpc3RlckRldmljZVJlc3BvbnNlEiYKD2RldmljZV90b2tlbl9pZBgBIAEoCVINZGV2aW'
        'NlVG9rZW5JZA==');

@$core.Deprecated('Use unregisterDeviceResponseDescriptor instead')
const UnregisterDeviceResponse$json = {
  '1': 'UnregisterDeviceResponse',
};

/// Descriptor for `UnregisterDeviceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unregisterDeviceResponseDescriptor =
    $convert.base64Decode('ChhVbnJlZ2lzdGVyRGV2aWNlUmVzcG9uc2U=');

@$core.Deprecated('Use getNotificationSettingsResponseDescriptor instead')
const GetNotificationSettingsResponse$json = {
  '1': 'GetNotificationSettingsResponse',
  '2': [
    {
      '1': 'notification_settings',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.notification.v1.NotificationSettings',
      '10': 'notificationSettings'
    },
  ],
};

/// Descriptor for `GetNotificationSettingsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getNotificationSettingsResponseDescriptor =
    $convert.base64Decode(
        'Ch9HZXROb3RpZmljYXRpb25TZXR0aW5nc1Jlc3BvbnNlEmAKFW5vdGlmaWNhdGlvbl9zZXR0aW'
        '5ncxgBIAEoCzIrLnZvaWNlLm5vdGlmaWNhdGlvbi52MS5Ob3RpZmljYXRpb25TZXR0aW5nc1IU'
        'bm90aWZpY2F0aW9uU2V0dGluZ3M=');

@$core.Deprecated('Use updateNotificationSettingsResponseDescriptor instead')
const UpdateNotificationSettingsResponse$json = {
  '1': 'UpdateNotificationSettingsResponse',
  '2': [
    {
      '1': 'notification_settings',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.notification.v1.NotificationSettings',
      '10': 'notificationSettings'
    },
  ],
};

/// Descriptor for `UpdateNotificationSettingsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateNotificationSettingsResponseDescriptor =
    $convert.base64Decode(
        'CiJVcGRhdGVOb3RpZmljYXRpb25TZXR0aW5nc1Jlc3BvbnNlEmAKFW5vdGlmaWNhdGlvbl9zZX'
        'R0aW5ncxgBIAEoCzIrLnZvaWNlLm5vdGlmaWNhdGlvbi52MS5Ob3RpZmljYXRpb25TZXR0aW5n'
        'c1IUbm90aWZpY2F0aW9uU2V0dGluZ3M=');

@$core.Deprecated('Use setQuietHoursResponseDescriptor instead')
const SetQuietHoursResponse$json = {
  '1': 'SetQuietHoursResponse',
};

/// Descriptor for `SetQuietHoursResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setQuietHoursResponseDescriptor =
    $convert.base64Decode('ChVTZXRRdWlldEhvdXJzUmVzcG9uc2U=');

@$core.Deprecated('Use sendNotificationResponseDescriptor instead')
const SendNotificationResponse$json = {
  '1': 'SendNotificationResponse',
};

/// Descriptor for `SendNotificationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendNotificationResponseDescriptor =
    $convert.base64Decode('ChhTZW5kTm90aWZpY2F0aW9uUmVzcG9uc2U=');

@$core.Deprecated('Use sendBulkNotificationResponseDescriptor instead')
const SendBulkNotificationResponse$json = {
  '1': 'SendBulkNotificationResponse',
};

/// Descriptor for `SendBulkNotificationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendBulkNotificationResponseDescriptor =
    $convert.base64Decode('ChxTZW5kQnVsa05vdGlmaWNhdGlvblJlc3BvbnNl');

@$core.Deprecated('Use relayNotificationResponseDescriptor instead')
const RelayNotificationResponse$json = {
  '1': 'RelayNotificationResponse',
};

/// Descriptor for `RelayNotificationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List relayNotificationResponseDescriptor =
    $convert.base64Decode('ChlSZWxheU5vdGlmaWNhdGlvblJlc3BvbnNl');
