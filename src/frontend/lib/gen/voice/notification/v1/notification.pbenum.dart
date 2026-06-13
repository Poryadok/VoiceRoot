// This is a generated file - do not edit.
//
// Generated from voice/notification/v1/notification.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for RegisterDeviceRequest.platform (string).
class DevicePlatform extends $pb.ProtobufEnum {
  static const DevicePlatform DEVICE_PLATFORM_UNSPECIFIED =
      DevicePlatform._(0, _omitEnumNames ? '' : 'DEVICE_PLATFORM_UNSPECIFIED');
  static const DevicePlatform DEVICE_PLATFORM_ANDROID =
      DevicePlatform._(1, _omitEnumNames ? '' : 'DEVICE_PLATFORM_ANDROID');
  static const DevicePlatform DEVICE_PLATFORM_IOS =
      DevicePlatform._(2, _omitEnumNames ? '' : 'DEVICE_PLATFORM_IOS');
  static const DevicePlatform DEVICE_PLATFORM_WEB =
      DevicePlatform._(3, _omitEnumNames ? '' : 'DEVICE_PLATFORM_WEB');
  static const DevicePlatform DEVICE_PLATFORM_DESKTOP =
      DevicePlatform._(4, _omitEnumNames ? '' : 'DEVICE_PLATFORM_DESKTOP');

  static const $core.List<DevicePlatform> values = <DevicePlatform>[
    DEVICE_PLATFORM_UNSPECIFIED,
    DEVICE_PLATFORM_ANDROID,
    DEVICE_PLATFORM_IOS,
    DEVICE_PLATFORM_WEB,
    DEVICE_PLATFORM_DESKTOP,
  ];

  static final $core.List<DevicePlatform?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static DevicePlatform? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const DevicePlatform._(super.value, super.name);
}

/// Canonical values for RegisterDeviceRequest.push_service (string).
class PushService extends $pb.ProtobufEnum {
  static const PushService PUSH_SERVICE_UNSPECIFIED =
      PushService._(0, _omitEnumNames ? '' : 'PUSH_SERVICE_UNSPECIFIED');
  static const PushService PUSH_SERVICE_FCM =
      PushService._(1, _omitEnumNames ? '' : 'PUSH_SERVICE_FCM');
  static const PushService PUSH_SERVICE_APNS =
      PushService._(2, _omitEnumNames ? '' : 'PUSH_SERVICE_APNS');
  static const PushService PUSH_SERVICE_VOIP_APNS =
      PushService._(3, _omitEnumNames ? '' : 'PUSH_SERVICE_VOIP_APNS');

  static const $core.List<PushService> values = <PushService>[
    PUSH_SERVICE_UNSPECIFIED,
    PUSH_SERVICE_FCM,
    PUSH_SERVICE_APNS,
    PUSH_SERVICE_VOIP_APNS,
  ];

  static final $core.List<PushService?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static PushService? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PushService._(super.value, super.name);
}

/// Canonical values for NotificationSettings.scope_type and GetNotificationSettingsRequest.scope_type (strings).
class NotificationScopeType extends $pb.ProtobufEnum {
  static const NotificationScopeType NOTIFICATION_SCOPE_TYPE_UNSPECIFIED =
      NotificationScopeType._(
          0, _omitEnumNames ? '' : 'NOTIFICATION_SCOPE_TYPE_UNSPECIFIED');
  static const NotificationScopeType NOTIFICATION_SCOPE_TYPE_GLOBAL =
      NotificationScopeType._(
          1, _omitEnumNames ? '' : 'NOTIFICATION_SCOPE_TYPE_GLOBAL');
  static const NotificationScopeType NOTIFICATION_SCOPE_TYPE_SPACE =
      NotificationScopeType._(
          2, _omitEnumNames ? '' : 'NOTIFICATION_SCOPE_TYPE_SPACE');
  static const NotificationScopeType NOTIFICATION_SCOPE_TYPE_CHANNEL =
      NotificationScopeType._(
          3, _omitEnumNames ? '' : 'NOTIFICATION_SCOPE_TYPE_CHANNEL');
  static const NotificationScopeType NOTIFICATION_SCOPE_TYPE_CHAT =
      NotificationScopeType._(
          4, _omitEnumNames ? '' : 'NOTIFICATION_SCOPE_TYPE_CHAT');

  static const $core.List<NotificationScopeType> values =
      <NotificationScopeType>[
    NOTIFICATION_SCOPE_TYPE_UNSPECIFIED,
    NOTIFICATION_SCOPE_TYPE_GLOBAL,
    NOTIFICATION_SCOPE_TYPE_SPACE,
    NOTIFICATION_SCOPE_TYPE_CHANNEL,
    NOTIFICATION_SCOPE_TYPE_CHAT,
  ];

  static final $core.List<NotificationScopeType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static NotificationScopeType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const NotificationScopeType._(super.value, super.name);
}

/// Broad product buckets for routing/analytics; free-form notification_type stays for Gateway-specific payloads.
class ClientNotificationCategory extends $pb.ProtobufEnum {
  static const ClientNotificationCategory
      CLIENT_NOTIFICATION_CATEGORY_UNSPECIFIED = ClientNotificationCategory._(
          0, _omitEnumNames ? '' : 'CLIENT_NOTIFICATION_CATEGORY_UNSPECIFIED');
  static const ClientNotificationCategory CLIENT_NOTIFICATION_CATEGORY_MESSAGE =
      ClientNotificationCategory._(
          1, _omitEnumNames ? '' : 'CLIENT_NOTIFICATION_CATEGORY_MESSAGE');
  static const ClientNotificationCategory CLIENT_NOTIFICATION_CATEGORY_DM =
      ClientNotificationCategory._(
          2, _omitEnumNames ? '' : 'CLIENT_NOTIFICATION_CATEGORY_DM');
  static const ClientNotificationCategory CLIENT_NOTIFICATION_CATEGORY_FRIEND =
      ClientNotificationCategory._(
          3, _omitEnumNames ? '' : 'CLIENT_NOTIFICATION_CATEGORY_FRIEND');
  static const ClientNotificationCategory CLIENT_NOTIFICATION_CATEGORY_CALL =
      ClientNotificationCategory._(
          4, _omitEnumNames ? '' : 'CLIENT_NOTIFICATION_CATEGORY_CALL');
  static const ClientNotificationCategory CLIENT_NOTIFICATION_CATEGORY_MATCH =
      ClientNotificationCategory._(
          5, _omitEnumNames ? '' : 'CLIENT_NOTIFICATION_CATEGORY_MATCH');
  static const ClientNotificationCategory CLIENT_NOTIFICATION_CATEGORY_SPACE =
      ClientNotificationCategory._(
          6, _omitEnumNames ? '' : 'CLIENT_NOTIFICATION_CATEGORY_SPACE');
  static const ClientNotificationCategory CLIENT_NOTIFICATION_CATEGORY_SYSTEM =
      ClientNotificationCategory._(
          7, _omitEnumNames ? '' : 'CLIENT_NOTIFICATION_CATEGORY_SYSTEM');

  static const $core.List<ClientNotificationCategory> values =
      <ClientNotificationCategory>[
    CLIENT_NOTIFICATION_CATEGORY_UNSPECIFIED,
    CLIENT_NOTIFICATION_CATEGORY_MESSAGE,
    CLIENT_NOTIFICATION_CATEGORY_DM,
    CLIENT_NOTIFICATION_CATEGORY_FRIEND,
    CLIENT_NOTIFICATION_CATEGORY_CALL,
    CLIENT_NOTIFICATION_CATEGORY_MATCH,
    CLIENT_NOTIFICATION_CATEGORY_SPACE,
    CLIENT_NOTIFICATION_CATEGORY_SYSTEM,
  ];

  static final $core.List<ClientNotificationCategory?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 7);
  static ClientNotificationCategory? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ClientNotificationCategory._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
