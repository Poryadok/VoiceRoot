import 'dart:convert';

import 'gateway_http.dart';

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
