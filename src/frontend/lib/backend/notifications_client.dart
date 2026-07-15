import 'dart:convert';

import '../gen/voice/notification/v1/notification.pb.dart' as notif_pb;
import 'gateway_http.dart';
import 'gateway_proto_json.dart';
import 'notification_settings_models.dart';
import 'proto_mappers.dart';

sealed class NotificationsApiResult<T> {
  const NotificationsApiResult();
}

final class NotificationsApiOk<T> extends NotificationsApiResult<T> {
  const NotificationsApiOk(this.data);
  final T data;
}

final class NotificationsApiFailure extends NotificationsApiResult<Never> {
  const NotificationsApiFailure({
    required this.message,
    this.statusCode,
  });

  final String message;
  final int? statusCode;
}

/// Gateway client for /api/v1/notifications/** (notifications (docs/features/notifications.md) FCM).
class VoiceNotificationsClient {
  VoiceNotificationsClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  /// Registers an FCM device token for the active profile.
  /// Returns the persisted `device_token_id` when the gateway includes it.
  Future<NotificationsApiResult<String?>> registerDevice({
    required String authorization,
    required String platform,
    required String token,
    String pushService = 'fcm',
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/notifications/register-device'),
      authorization: authorization,
      body: {
        'platform': platform,
        'token': token,
        'push_service': pushService,
      },
      allowNoContent: true,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => NotificationsApiOk(_deviceTokenIdFromBody(data)),
      GatewayHttpFailure(:final error) => NotificationsApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }

  String? _deviceTokenIdFromBody(dynamic body) {
    if (body == null) return null;
    if (body is Map<String, dynamic>) {
      final id = body['device_token_id'] ?? body['deviceTokenId'];
      if (id is String && id.isNotEmpty) return id;
      return null;
    }
    if (body is String && body.trim().isNotEmpty) {
      try {
        final decoded = jsonDecode(body);
        return _deviceTokenIdFromBody(decoded);
      } catch (_) {
        return null;
      }
    }
    return null;
  }

  Future<NotificationsApiResult<VoiceNotificationSettings>> getSettings({
    required String authorization,
    String scopeType = 'global',
    String? scopeId,
  }) async {
    final query = <String, String>{'scope_type': scopeType};
    if (scopeId != null && scopeId.isNotEmpty) {
      query['scope_id'] = scopeId;
    }
    final result = await _gateway.getProto(
      _gateway.replace(
        path: '/api/v1/notifications/settings',
        queryParameters: query,
      ),
      authorization: authorization,
      createEmpty: notif_pb.GetNotificationSettingsResponse.create,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => NotificationsApiOk(
        _settingsFromProto(
          data.hasNotificationSettings()
              ? data.notificationSettings
              : notif_pb.NotificationSettings(),
        ),
      ),
      GatewayHttpFailure(:final error) => NotificationsApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }

  Future<NotificationsApiResult<VoiceNotificationSettings>> updateSettings({
    required String authorization,
    required VoiceNotificationSettings settings,
  }) async {
    final body = notif_pb.UpdateNotificationSettingsRequest(
      settings: _settingsToProto(settings),
    );
    final result = await _gateway.putJson(
      uri: _gateway.resolve('/api/v1/notifications/settings'),
      authorization: authorization,
      body: jsonDecode(encodeGatewayProto(body)) as Map<String, dynamic>,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => NotificationsApiOk(
        _settingsFromResponseMap(data, fallback: settings),
      ),
      GatewayHttpFailure(:final error) => NotificationsApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }

  Future<NotificationsApiResult<void>> setQuietHours({
    required String authorization,
    required VoiceQuietHours quietHours,
  }) async {
    final body = notif_pb.SetQuietHoursRequest(
      enabled: quietHours.enabled,
      startTime: quietHours.startTime,
      endTime: quietHours.endTime,
      timezone: quietHours.timezone,
      overrideMentions: quietHours.overrideMentions,
    );
    final result = await _gateway.putJson(
      uri: _gateway.resolve('/api/v1/notifications/quiet-hours'),
      authorization: authorization,
      body: jsonDecode(encodeGatewayProto(body)) as Map<String, dynamic>,
      allowNoContent: true,
    );
    return switch (result) {
      GatewayHttpOk() => const NotificationsApiOk(null),
      GatewayHttpFailure(:final error) => NotificationsApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }

  VoiceNotificationSettings _settingsFromResponseMap(
    Map<String, dynamic> data, {
    required VoiceNotificationSettings fallback,
  }) {
    final raw = data['notification_settings'] ?? data['notificationSettings'];
    if (raw is Map<String, dynamic>) {
      final proto = notif_pb.NotificationSettings();
      proto.mergeFromProto3Json(raw, ignoreUnknownFields: true);
      return _settingsFromProto(proto);
    }
    return fallback;
  }

  VoiceNotificationSettings _settingsFromProto(
    notif_pb.NotificationSettings proto,
  ) {
    return VoiceNotificationSettings(
      profileId: proto.profileId,
      scopeType: proto.scopeType.isEmpty ? 'global' : proto.scopeType,
      scopeId: proto.hasScopeId() ? proto.scopeId : null,
      enabled: proto.enabled,
      suppressedTypes: VoiceNotificationSettings.parseSuppressedTypes(
        proto.suppressTypesJson,
      ),
      muteUntil: proto.hasMuteUntil()
          ? protoTimestampToDateTime(proto.muteUntil)
          : null,
    );
  }

  notif_pb.NotificationSettings _settingsToProto(
    VoiceNotificationSettings settings,
  ) {
    final proto = notif_pb.NotificationSettings(
      profileId: settings.profileId,
      scopeType: settings.scopeType,
      enabled: settings.enabled,
      suppressTypesJson: settings.suppressTypesJson,
    );
    if (settings.scopeId != null && settings.scopeId!.isNotEmpty) {
      proto.scopeId = settings.scopeId!;
    }
    if (settings.muteUntil != null) {
      proto.muteUntil = dateTimeToProtoTimestamp(settings.muteUntil!);
    }
    return proto;
  }

  /// Removes a registered device token for the active profile.
  Future<NotificationsApiResult<void>> unregisterDevice({
    required String authorization,
    required String deviceTokenId,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/notifications/unregister-device'),
      authorization: authorization,
      body: {'device_token_id': deviceTokenId},
      allowNoContent: true,
    );
    return switch (result) {
      GatewayHttpOk() => const NotificationsApiOk(null),
      GatewayHttpFailure(:final error) => NotificationsApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }
}
