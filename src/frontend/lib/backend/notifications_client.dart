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

/// Gateway client for /api/v1/notifications/** (Phase 6 FCM).
class VoiceNotificationsClient {
  VoiceNotificationsClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  /// Registers an FCM device token for the active profile.
  Future<NotificationsApiResult<void>> registerDevice({
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
      GatewayHttpOk() => const NotificationsApiOk(null),
      GatewayHttpFailure(:final error) => NotificationsApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
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
