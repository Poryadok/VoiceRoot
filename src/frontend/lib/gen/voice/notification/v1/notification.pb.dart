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
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import 'notification.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'notification.pbenum.dart';

class RegisterDeviceRequest extends $pb.GeneratedMessage {
  factory RegisterDeviceRequest({
    $core.String? platform,
    $core.String? token,
    $core.String? pushService,
    DevicePlatform? platformEnum,
    PushService? pushServiceEnum,
  }) {
    final result = create();
    if (platform != null) result.platform = platform;
    if (token != null) result.token = token;
    if (pushService != null) result.pushService = pushService;
    if (platformEnum != null) result.platformEnum = platformEnum;
    if (pushServiceEnum != null) result.pushServiceEnum = pushServiceEnum;
    return result;
  }

  RegisterDeviceRequest._();

  factory RegisterDeviceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterDeviceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterDeviceRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'platform')
    ..aOS(2, _omitFieldNames ? '' : 'token')
    ..aOS(3, _omitFieldNames ? '' : 'pushService')
    ..aE<DevicePlatform>(4, _omitFieldNames ? '' : 'platformEnum',
        enumValues: DevicePlatform.values)
    ..aE<PushService>(5, _omitFieldNames ? '' : 'pushServiceEnum',
        enumValues: PushService.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterDeviceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterDeviceRequest copyWith(
          void Function(RegisterDeviceRequest) updates) =>
      super.copyWith((message) => updates(message as RegisterDeviceRequest))
          as RegisterDeviceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterDeviceRequest create() => RegisterDeviceRequest._();
  @$core.override
  RegisterDeviceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterDeviceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterDeviceRequest>(create);
  static RegisterDeviceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get platform => $_getSZ(0);
  @$pb.TagNumber(1)
  set platform($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPlatform() => $_has(0);
  @$pb.TagNumber(1)
  void clearPlatform() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get token => $_getSZ(1);
  @$pb.TagNumber(2)
  set token($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearToken() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get pushService => $_getSZ(2);
  @$pb.TagNumber(3)
  set pushService($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPushService() => $_has(2);
  @$pb.TagNumber(3)
  void clearPushService() => $_clearField(3);

  /// Preferred over string platform when set; see enum DevicePlatform (docs/REPOSITORIES.md).
  @$pb.TagNumber(4)
  DevicePlatform get platformEnum => $_getN(3);
  @$pb.TagNumber(4)
  set platformEnum(DevicePlatform value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPlatformEnum() => $_has(3);
  @$pb.TagNumber(4)
  void clearPlatformEnum() => $_clearField(4);

  @$pb.TagNumber(5)
  PushService get pushServiceEnum => $_getN(4);
  @$pb.TagNumber(5)
  set pushServiceEnum(PushService value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPushServiceEnum() => $_has(4);
  @$pb.TagNumber(5)
  void clearPushServiceEnum() => $_clearField(5);
}

class UnregisterDeviceRequest extends $pb.GeneratedMessage {
  factory UnregisterDeviceRequest({
    $core.String? deviceTokenId,
  }) {
    final result = create();
    if (deviceTokenId != null) result.deviceTokenId = deviceTokenId;
    return result;
  }

  UnregisterDeviceRequest._();

  factory UnregisterDeviceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnregisterDeviceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnregisterDeviceRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'deviceTokenId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnregisterDeviceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnregisterDeviceRequest copyWith(
          void Function(UnregisterDeviceRequest) updates) =>
      super.copyWith((message) => updates(message as UnregisterDeviceRequest))
          as UnregisterDeviceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnregisterDeviceRequest create() => UnregisterDeviceRequest._();
  @$core.override
  UnregisterDeviceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnregisterDeviceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnregisterDeviceRequest>(create);
  static UnregisterDeviceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get deviceTokenId => $_getSZ(0);
  @$pb.TagNumber(1)
  set deviceTokenId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDeviceTokenId() => $_has(0);
  @$pb.TagNumber(1)
  void clearDeviceTokenId() => $_clearField(1);
}

class GetNotificationSettingsRequest extends $pb.GeneratedMessage {
  factory GetNotificationSettingsRequest({
    $core.String? scopeType,
    $core.String? scopeId,
    NotificationScopeType? scopeTypeEnum,
  }) {
    final result = create();
    if (scopeType != null) result.scopeType = scopeType;
    if (scopeId != null) result.scopeId = scopeId;
    if (scopeTypeEnum != null) result.scopeTypeEnum = scopeTypeEnum;
    return result;
  }

  GetNotificationSettingsRequest._();

  factory GetNotificationSettingsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetNotificationSettingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetNotificationSettingsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'scopeType')
    ..aOS(2, _omitFieldNames ? '' : 'scopeId')
    ..aE<NotificationScopeType>(3, _omitFieldNames ? '' : 'scopeTypeEnum',
        enumValues: NotificationScopeType.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNotificationSettingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNotificationSettingsRequest copyWith(
          void Function(GetNotificationSettingsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetNotificationSettingsRequest))
          as GetNotificationSettingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetNotificationSettingsRequest create() =>
      GetNotificationSettingsRequest._();
  @$core.override
  GetNotificationSettingsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetNotificationSettingsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetNotificationSettingsRequest>(create);
  static GetNotificationSettingsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get scopeType => $_getSZ(0);
  @$pb.TagNumber(1)
  set scopeType($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasScopeType() => $_has(0);
  @$pb.TagNumber(1)
  void clearScopeType() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get scopeId => $_getSZ(1);
  @$pb.TagNumber(2)
  set scopeId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasScopeId() => $_has(1);
  @$pb.TagNumber(2)
  void clearScopeId() => $_clearField(2);

  @$pb.TagNumber(3)
  NotificationScopeType get scopeTypeEnum => $_getN(2);
  @$pb.TagNumber(3)
  set scopeTypeEnum(NotificationScopeType value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasScopeTypeEnum() => $_has(2);
  @$pb.TagNumber(3)
  void clearScopeTypeEnum() => $_clearField(3);
}

class UpdateNotificationSettingsRequest extends $pb.GeneratedMessage {
  factory UpdateNotificationSettingsRequest({
    NotificationSettings? settings,
  }) {
    final result = create();
    if (settings != null) result.settings = settings;
    return result;
  }

  UpdateNotificationSettingsRequest._();

  factory UpdateNotificationSettingsRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateNotificationSettingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateNotificationSettingsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOM<NotificationSettings>(1, _omitFieldNames ? '' : 'settings',
        subBuilder: NotificationSettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateNotificationSettingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateNotificationSettingsRequest copyWith(
          void Function(UpdateNotificationSettingsRequest) updates) =>
      super.copyWith((message) =>
              updates(message as UpdateNotificationSettingsRequest))
          as UpdateNotificationSettingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateNotificationSettingsRequest create() =>
      UpdateNotificationSettingsRequest._();
  @$core.override
  UpdateNotificationSettingsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateNotificationSettingsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateNotificationSettingsRequest>(
          create);
  static UpdateNotificationSettingsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationSettings get settings => $_getN(0);
  @$pb.TagNumber(1)
  set settings(NotificationSettings value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSettings() => $_has(0);
  @$pb.TagNumber(1)
  void clearSettings() => $_clearField(1);
  @$pb.TagNumber(1)
  NotificationSettings ensureSettings() => $_ensure(0);
}

class NotificationSettings extends $pb.GeneratedMessage {
  factory NotificationSettings({
    $core.String? profileId,
    $core.String? scopeType,
    $core.String? scopeId,
    $core.bool? enabled,
    $core.String? suppressTypesJson,
    $1.Timestamp? muteUntil,
    NotificationScopeType? scopeTypeEnum,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (scopeType != null) result.scopeType = scopeType;
    if (scopeId != null) result.scopeId = scopeId;
    if (enabled != null) result.enabled = enabled;
    if (suppressTypesJson != null) result.suppressTypesJson = suppressTypesJson;
    if (muteUntil != null) result.muteUntil = muteUntil;
    if (scopeTypeEnum != null) result.scopeTypeEnum = scopeTypeEnum;
    return result;
  }

  NotificationSettings._();

  factory NotificationSettings.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory NotificationSettings.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'NotificationSettings',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'scopeType')
    ..aOS(3, _omitFieldNames ? '' : 'scopeId')
    ..aOB(4, _omitFieldNames ? '' : 'enabled')
    ..aOS(6, _omitFieldNames ? '' : 'suppressTypesJson')
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'muteUntil',
        subBuilder: $1.Timestamp.create)
    ..aE<NotificationScopeType>(8, _omitFieldNames ? '' : 'scopeTypeEnum',
        enumValues: NotificationScopeType.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NotificationSettings clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NotificationSettings copyWith(void Function(NotificationSettings) updates) =>
      super.copyWith((message) => updates(message as NotificationSettings))
          as NotificationSettings;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static NotificationSettings create() => NotificationSettings._();
  @$core.override
  NotificationSettings createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static NotificationSettings getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<NotificationSettings>(create);
  static NotificationSettings? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get scopeType => $_getSZ(1);
  @$pb.TagNumber(2)
  set scopeType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasScopeType() => $_has(1);
  @$pb.TagNumber(2)
  void clearScopeType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get scopeId => $_getSZ(2);
  @$pb.TagNumber(3)
  set scopeId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasScopeId() => $_has(2);
  @$pb.TagNumber(3)
  void clearScopeId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get enabled => $_getBF(3);
  @$pb.TagNumber(4)
  set enabled($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasEnabled() => $_has(3);
  @$pb.TagNumber(4)
  void clearEnabled() => $_clearField(4);

  @$pb.TagNumber(6)
  $core.String get suppressTypesJson => $_getSZ(4);
  @$pb.TagNumber(6)
  set suppressTypesJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(6)
  $core.bool hasSuppressTypesJson() => $_has(4);
  @$pb.TagNumber(6)
  void clearSuppressTypesJson() => $_clearField(6);

  /// UTC instant; public API uses Timestamp per docs/REPOSITORIES.md.
  @$pb.TagNumber(7)
  $1.Timestamp get muteUntil => $_getN(5);
  @$pb.TagNumber(7)
  set muteUntil($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasMuteUntil() => $_has(5);
  @$pb.TagNumber(7)
  void clearMuteUntil() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensureMuteUntil() => $_ensure(5);

  @$pb.TagNumber(8)
  NotificationScopeType get scopeTypeEnum => $_getN(6);
  @$pb.TagNumber(8)
  set scopeTypeEnum(NotificationScopeType value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasScopeTypeEnum() => $_has(6);
  @$pb.TagNumber(8)
  void clearScopeTypeEnum() => $_clearField(8);
}

class SetQuietHoursRequest extends $pb.GeneratedMessage {
  factory SetQuietHoursRequest({
    $core.bool? enabled,
    $core.String? startTime,
    $core.String? endTime,
    $core.String? timezone,
    $core.bool? overrideMentions,
  }) {
    final result = create();
    if (enabled != null) result.enabled = enabled;
    if (startTime != null) result.startTime = startTime;
    if (endTime != null) result.endTime = endTime;
    if (timezone != null) result.timezone = timezone;
    if (overrideMentions != null) result.overrideMentions = overrideMentions;
    return result;
  }

  SetQuietHoursRequest._();

  factory SetQuietHoursRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetQuietHoursRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetQuietHoursRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'enabled')
    ..aOS(2, _omitFieldNames ? '' : 'startTime')
    ..aOS(3, _omitFieldNames ? '' : 'endTime')
    ..aOS(4, _omitFieldNames ? '' : 'timezone')
    ..aOB(5, _omitFieldNames ? '' : 'overrideMentions')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetQuietHoursRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetQuietHoursRequest copyWith(void Function(SetQuietHoursRequest) updates) =>
      super.copyWith((message) => updates(message as SetQuietHoursRequest))
          as SetQuietHoursRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetQuietHoursRequest create() => SetQuietHoursRequest._();
  @$core.override
  SetQuietHoursRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetQuietHoursRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetQuietHoursRequest>(create);
  static SetQuietHoursRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get enabled => $_getBF(0);
  @$pb.TagNumber(1)
  set enabled($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEnabled() => $_has(0);
  @$pb.TagNumber(1)
  void clearEnabled() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get startTime => $_getSZ(1);
  @$pb.TagNumber(2)
  set startTime($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStartTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearStartTime() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get endTime => $_getSZ(2);
  @$pb.TagNumber(3)
  set endTime($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEndTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearEndTime() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get timezone => $_getSZ(3);
  @$pb.TagNumber(4)
  set timezone($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTimezone() => $_has(3);
  @$pb.TagNumber(4)
  void clearTimezone() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get overrideMentions => $_getBF(4);
  @$pb.TagNumber(5)
  set overrideMentions($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasOverrideMentions() => $_has(4);
  @$pb.TagNumber(5)
  void clearOverrideMentions() => $_clearField(5);
}

class SendNotificationRequest extends $pb.GeneratedMessage {
  factory SendNotificationRequest({
    $core.String? profileId,
    $core.String? notificationType,
    $core.String? title,
    $core.String? body,
    $core.String? payloadJson,
    ClientNotificationCategory? notificationCategory,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (notificationType != null) result.notificationType = notificationType;
    if (title != null) result.title = title;
    if (body != null) result.body = body;
    if (payloadJson != null) result.payloadJson = payloadJson;
    if (notificationCategory != null)
      result.notificationCategory = notificationCategory;
    return result;
  }

  SendNotificationRequest._();

  factory SendNotificationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendNotificationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendNotificationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'notificationType')
    ..aOS(3, _omitFieldNames ? '' : 'title')
    ..aOS(4, _omitFieldNames ? '' : 'body')
    ..aOS(5, _omitFieldNames ? '' : 'payloadJson')
    ..aE<ClientNotificationCategory>(
        6, _omitFieldNames ? '' : 'notificationCategory',
        enumValues: ClientNotificationCategory.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendNotificationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendNotificationRequest copyWith(
          void Function(SendNotificationRequest) updates) =>
      super.copyWith((message) => updates(message as SendNotificationRequest))
          as SendNotificationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendNotificationRequest create() => SendNotificationRequest._();
  @$core.override
  SendNotificationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendNotificationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendNotificationRequest>(create);
  static SendNotificationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get notificationType => $_getSZ(1);
  @$pb.TagNumber(2)
  set notificationType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNotificationType() => $_has(1);
  @$pb.TagNumber(2)
  void clearNotificationType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get title => $_getSZ(2);
  @$pb.TagNumber(3)
  set title($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTitle() => $_has(2);
  @$pb.TagNumber(3)
  void clearTitle() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get body => $_getSZ(3);
  @$pb.TagNumber(4)
  set body($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasBody() => $_has(3);
  @$pb.TagNumber(4)
  void clearBody() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get payloadJson => $_getSZ(4);
  @$pb.TagNumber(5)
  set payloadJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPayloadJson() => $_has(4);
  @$pb.TagNumber(5)
  void clearPayloadJson() => $_clearField(5);

  @$pb.TagNumber(6)
  ClientNotificationCategory get notificationCategory => $_getN(5);
  @$pb.TagNumber(6)
  set notificationCategory(ClientNotificationCategory value) =>
      $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasNotificationCategory() => $_has(5);
  @$pb.TagNumber(6)
  void clearNotificationCategory() => $_clearField(6);
}

class SendBulkNotificationRequest extends $pb.GeneratedMessage {
  factory SendBulkNotificationRequest({
    $core.Iterable<$core.String>? profileIds,
    $core.String? notificationType,
    $core.String? title,
    $core.String? body,
    $core.String? payloadJson,
    ClientNotificationCategory? notificationCategory,
  }) {
    final result = create();
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (notificationType != null) result.notificationType = notificationType;
    if (title != null) result.title = title;
    if (body != null) result.body = body;
    if (payloadJson != null) result.payloadJson = payloadJson;
    if (notificationCategory != null)
      result.notificationCategory = notificationCategory;
    return result;
  }

  SendBulkNotificationRequest._();

  factory SendBulkNotificationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendBulkNotificationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendBulkNotificationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'profileIds')
    ..aOS(2, _omitFieldNames ? '' : 'notificationType')
    ..aOS(3, _omitFieldNames ? '' : 'title')
    ..aOS(4, _omitFieldNames ? '' : 'body')
    ..aOS(5, _omitFieldNames ? '' : 'payloadJson')
    ..aE<ClientNotificationCategory>(
        6, _omitFieldNames ? '' : 'notificationCategory',
        enumValues: ClientNotificationCategory.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendBulkNotificationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendBulkNotificationRequest copyWith(
          void Function(SendBulkNotificationRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SendBulkNotificationRequest))
          as SendBulkNotificationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendBulkNotificationRequest create() =>
      SendBulkNotificationRequest._();
  @$core.override
  SendBulkNotificationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendBulkNotificationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendBulkNotificationRequest>(create);
  static SendBulkNotificationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get profileIds => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get notificationType => $_getSZ(1);
  @$pb.TagNumber(2)
  set notificationType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNotificationType() => $_has(1);
  @$pb.TagNumber(2)
  void clearNotificationType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get title => $_getSZ(2);
  @$pb.TagNumber(3)
  set title($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTitle() => $_has(2);
  @$pb.TagNumber(3)
  void clearTitle() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get body => $_getSZ(3);
  @$pb.TagNumber(4)
  set body($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasBody() => $_has(3);
  @$pb.TagNumber(4)
  void clearBody() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get payloadJson => $_getSZ(4);
  @$pb.TagNumber(5)
  set payloadJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPayloadJson() => $_has(4);
  @$pb.TagNumber(5)
  void clearPayloadJson() => $_clearField(5);

  @$pb.TagNumber(6)
  ClientNotificationCategory get notificationCategory => $_getN(5);
  @$pb.TagNumber(6)
  set notificationCategory(ClientNotificationCategory value) =>
      $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasNotificationCategory() => $_has(5);
  @$pb.TagNumber(6)
  void clearNotificationCategory() => $_clearField(6);
}

/// Internal / federation: target is Auth account UUID (same as JWT claim user_id).
class RelayNotificationRequest extends $pb.GeneratedMessage {
  factory RelayNotificationRequest({
    $core.String? accountId,
    $core.String? payloadJson,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (payloadJson != null) result.payloadJson = payloadJson;
    return result;
  }

  RelayNotificationRequest._();

  factory RelayNotificationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RelayNotificationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RelayNotificationRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'payloadJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RelayNotificationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RelayNotificationRequest copyWith(
          void Function(RelayNotificationRequest) updates) =>
      super.copyWith((message) => updates(message as RelayNotificationRequest))
          as RelayNotificationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RelayNotificationRequest create() => RelayNotificationRequest._();
  @$core.override
  RelayNotificationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RelayNotificationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RelayNotificationRequest>(create);
  static RelayNotificationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get payloadJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set payloadJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPayloadJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearPayloadJson() => $_clearField(2);
}

class RegisterDeviceResponse extends $pb.GeneratedMessage {
  factory RegisterDeviceResponse({
    $core.String? deviceTokenId,
  }) {
    final result = create();
    if (deviceTokenId != null) result.deviceTokenId = deviceTokenId;
    return result;
  }

  RegisterDeviceResponse._();

  factory RegisterDeviceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterDeviceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterDeviceResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'deviceTokenId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterDeviceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterDeviceResponse copyWith(
          void Function(RegisterDeviceResponse) updates) =>
      super.copyWith((message) => updates(message as RegisterDeviceResponse))
          as RegisterDeviceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterDeviceResponse create() => RegisterDeviceResponse._();
  @$core.override
  RegisterDeviceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterDeviceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterDeviceResponse>(create);
  static RegisterDeviceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get deviceTokenId => $_getSZ(0);
  @$pb.TagNumber(1)
  set deviceTokenId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDeviceTokenId() => $_has(0);
  @$pb.TagNumber(1)
  void clearDeviceTokenId() => $_clearField(1);
}

class UnregisterDeviceResponse extends $pb.GeneratedMessage {
  factory UnregisterDeviceResponse() => create();

  UnregisterDeviceResponse._();

  factory UnregisterDeviceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnregisterDeviceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnregisterDeviceResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnregisterDeviceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnregisterDeviceResponse copyWith(
          void Function(UnregisterDeviceResponse) updates) =>
      super.copyWith((message) => updates(message as UnregisterDeviceResponse))
          as UnregisterDeviceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnregisterDeviceResponse create() => UnregisterDeviceResponse._();
  @$core.override
  UnregisterDeviceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnregisterDeviceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnregisterDeviceResponse>(create);
  static UnregisterDeviceResponse? _defaultInstance;
}

class GetNotificationSettingsResponse extends $pb.GeneratedMessage {
  factory GetNotificationSettingsResponse({
    NotificationSettings? notificationSettings,
  }) {
    final result = create();
    if (notificationSettings != null)
      result.notificationSettings = notificationSettings;
    return result;
  }

  GetNotificationSettingsResponse._();

  factory GetNotificationSettingsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetNotificationSettingsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetNotificationSettingsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOM<NotificationSettings>(
        1, _omitFieldNames ? '' : 'notificationSettings',
        subBuilder: NotificationSettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNotificationSettingsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNotificationSettingsResponse copyWith(
          void Function(GetNotificationSettingsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetNotificationSettingsResponse))
          as GetNotificationSettingsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetNotificationSettingsResponse create() =>
      GetNotificationSettingsResponse._();
  @$core.override
  GetNotificationSettingsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetNotificationSettingsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetNotificationSettingsResponse>(
          create);
  static GetNotificationSettingsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationSettings get notificationSettings => $_getN(0);
  @$pb.TagNumber(1)
  set notificationSettings(NotificationSettings value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasNotificationSettings() => $_has(0);
  @$pb.TagNumber(1)
  void clearNotificationSettings() => $_clearField(1);
  @$pb.TagNumber(1)
  NotificationSettings ensureNotificationSettings() => $_ensure(0);
}

class UpdateNotificationSettingsResponse extends $pb.GeneratedMessage {
  factory UpdateNotificationSettingsResponse({
    NotificationSettings? notificationSettings,
  }) {
    final result = create();
    if (notificationSettings != null)
      result.notificationSettings = notificationSettings;
    return result;
  }

  UpdateNotificationSettingsResponse._();

  factory UpdateNotificationSettingsResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateNotificationSettingsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateNotificationSettingsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..aOM<NotificationSettings>(
        1, _omitFieldNames ? '' : 'notificationSettings',
        subBuilder: NotificationSettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateNotificationSettingsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateNotificationSettingsResponse copyWith(
          void Function(UpdateNotificationSettingsResponse) updates) =>
      super.copyWith((message) =>
              updates(message as UpdateNotificationSettingsResponse))
          as UpdateNotificationSettingsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateNotificationSettingsResponse create() =>
      UpdateNotificationSettingsResponse._();
  @$core.override
  UpdateNotificationSettingsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateNotificationSettingsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateNotificationSettingsResponse>(
          create);
  static UpdateNotificationSettingsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  NotificationSettings get notificationSettings => $_getN(0);
  @$pb.TagNumber(1)
  set notificationSettings(NotificationSettings value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasNotificationSettings() => $_has(0);
  @$pb.TagNumber(1)
  void clearNotificationSettings() => $_clearField(1);
  @$pb.TagNumber(1)
  NotificationSettings ensureNotificationSettings() => $_ensure(0);
}

class SetQuietHoursResponse extends $pb.GeneratedMessage {
  factory SetQuietHoursResponse() => create();

  SetQuietHoursResponse._();

  factory SetQuietHoursResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetQuietHoursResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetQuietHoursResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetQuietHoursResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetQuietHoursResponse copyWith(
          void Function(SetQuietHoursResponse) updates) =>
      super.copyWith((message) => updates(message as SetQuietHoursResponse))
          as SetQuietHoursResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetQuietHoursResponse create() => SetQuietHoursResponse._();
  @$core.override
  SetQuietHoursResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetQuietHoursResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetQuietHoursResponse>(create);
  static SetQuietHoursResponse? _defaultInstance;
}

class SendNotificationResponse extends $pb.GeneratedMessage {
  factory SendNotificationResponse() => create();

  SendNotificationResponse._();

  factory SendNotificationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendNotificationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendNotificationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendNotificationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendNotificationResponse copyWith(
          void Function(SendNotificationResponse) updates) =>
      super.copyWith((message) => updates(message as SendNotificationResponse))
          as SendNotificationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendNotificationResponse create() => SendNotificationResponse._();
  @$core.override
  SendNotificationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendNotificationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendNotificationResponse>(create);
  static SendNotificationResponse? _defaultInstance;
}

class SendBulkNotificationResponse extends $pb.GeneratedMessage {
  factory SendBulkNotificationResponse() => create();

  SendBulkNotificationResponse._();

  factory SendBulkNotificationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendBulkNotificationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendBulkNotificationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendBulkNotificationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendBulkNotificationResponse copyWith(
          void Function(SendBulkNotificationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SendBulkNotificationResponse))
          as SendBulkNotificationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendBulkNotificationResponse create() =>
      SendBulkNotificationResponse._();
  @$core.override
  SendBulkNotificationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendBulkNotificationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendBulkNotificationResponse>(create);
  static SendBulkNotificationResponse? _defaultInstance;
}

class RelayNotificationResponse extends $pb.GeneratedMessage {
  factory RelayNotificationResponse() => create();

  RelayNotificationResponse._();

  factory RelayNotificationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RelayNotificationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RelayNotificationResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.notification.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RelayNotificationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RelayNotificationResponse copyWith(
          void Function(RelayNotificationResponse) updates) =>
      super.copyWith((message) => updates(message as RelayNotificationResponse))
          as RelayNotificationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RelayNotificationResponse create() => RelayNotificationResponse._();
  @$core.override
  RelayNotificationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RelayNotificationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RelayNotificationResponse>(create);
  static RelayNotificationResponse? _defaultInstance;
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
