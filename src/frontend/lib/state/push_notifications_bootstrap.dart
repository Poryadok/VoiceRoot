import '../backend/notifications_client.dart';

/// Wires FCM token refresh to Gateway device registration (Phase 6).
class PushNotificationsBootstrap {
  const PushNotificationsBootstrap();

  Future<String?> registerToken({
    required VoiceNotificationsClient client,
    required String authorization,
    required String platform,
    required String token,
    String pushService = 'fcm',
  }) async {
    final result = await client.registerDevice(
      authorization: authorization,
      platform: platform,
      token: token,
      pushService: pushService,
    );
    return switch (result) {
      NotificationsApiOk(:final data) => data,
      NotificationsApiFailure(:final message) => throw StateError(message),
    };
  }

  Future<void> unregisterToken({
    required VoiceNotificationsClient client,
    required String authorization,
    required String deviceTokenId,
  }) async {
    final result = await client.unregisterDevice(
      authorization: authorization,
      deviceTokenId: deviceTokenId,
    );
    if (result is NotificationsApiFailure) {
      throw StateError(result.message);
    }
  }
}
