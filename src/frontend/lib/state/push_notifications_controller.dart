import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/notifications_client.dart';
import 'auth_providers.dart';
import 'in_app_notifications.dart';
import 'push_notifications.dart';
import 'push_notifications_bootstrap.dart';
import 'push_platform.dart';

/// Background FCM handler (Android/iOS); must be top-level.
@pragma('vm:entry-point')
Future<void> firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  // Payload is processed when the app resumes; keep handler registered for OS delivery.
}

final voiceNotificationsClientProvider = Provider<VoiceNotificationsClient>((ref) {
  return VoiceNotificationsClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final pushNotificationsControllerProvider =
    Provider<PushNotificationsController>((ref) {
  final controller = PushNotificationsController(ref);
  ref.onDispose(controller.dispose);
  return controller;
});

/// Registers FCM tokens and maps foreground push payloads to in-app notifications.
class PushNotificationsController {
  PushNotificationsController(this._ref) {
    _authSub = _ref.listen<AuthState>(authControllerProvider, (prev, next) {
      if (next.isAuthenticated &&
          (prev?.session?.accessToken != next.session?.accessToken)) {
        unawaited(_syncToken());
      }
      if (!next.isAuthenticated && prev?.isAuthenticated == true) {
        unawaited(_teardown());
      }
    }, fireImmediately: true);
  }

  final Ref _ref;
  ProviderSubscription<AuthState>? _authSub;
  StreamSubscription<RemoteMessage>? _foregroundSub;
  StreamSubscription<String>? _tokenRefreshSub;

  void dispose() {
    _authSub?.close();
    _foregroundSub?.cancel();
    _tokenRefreshSub?.cancel();
  }

  Future<void> _teardown() async {
    _foregroundSub?.cancel();
    _foregroundSub = null;
    _tokenRefreshSub?.cancel();
    _tokenRefreshSub = null;
  }

  Future<void> _syncToken() async {
    final auth = _ref.read(authControllerProvider);
    final header = auth.session?.authorizationHeader;
    if (header == null) return;

    try {
      await _ensureFirebase();
    } catch (_) {
      return;
    }

    final messaging = FirebaseMessaging.instance;
    await messaging.requestPermission();
    FirebaseMessaging.onBackgroundMessage(firebaseMessagingBackgroundHandler);

    final platform = pushPlatformForTarget();
    final pushService = pushServiceForTarget();
    final token = await _resolvePushToken(messaging, pushService);
    if (token == null || token.isEmpty) return;

    const bootstrap = PushNotificationsBootstrap();
    await bootstrap.registerToken(
      client: _ref.read(voiceNotificationsClientProvider),
      authorization: header,
      platform: platform,
      token: token,
      pushService: pushService,
    );

    _foregroundSub ??= FirebaseMessaging.onMessage.listen(_onForegroundMessage);
    _tokenRefreshSub ??= messaging.onTokenRefresh.listen((_) async {
      final currentHeader =
          _ref.read(authControllerProvider).session?.authorizationHeader;
      if (currentHeader == null) return;
      final refreshed = await _resolvePushToken(
        messaging,
        pushServiceForTarget(),
      );
      if (refreshed == null || refreshed.isEmpty) return;
      await bootstrap.registerToken(
        client: _ref.read(voiceNotificationsClientProvider),
        authorization: currentHeader,
        platform: pushPlatformForTarget(),
        token: refreshed,
        pushService: pushServiceForTarget(),
      );
    });
  }

  void _onForegroundMessage(RemoteMessage message) {
    final data = message.data.map((k, v) => MapEntry(k, v.toString()));
    final frame = fcmDataToRealtimeNotification(data);
    if (frame == null) return;
    _ref
        .read(inAppNotificationControllerProvider)
        ?.onPushNotificationData(frame.data);
  }

  Future<void> _ensureFirebase() async {
    if (Firebase.apps.isNotEmpty) return;
    await Firebase.initializeApp();
  }

  Future<String?> _resolvePushToken(
    FirebaseMessaging messaging,
    String pushService,
  ) async {
    if (pushService == 'apns') {
      for (var attempt = 0; attempt < 5; attempt++) {
        final apnsToken = await messaging.getAPNSToken();
        if (apnsToken != null && apnsToken.isNotEmpty) {
          return apnsToken;
        }
        await Future<void>.delayed(const Duration(milliseconds: 300));
      }
      return null;
    }
    return messaging.getToken();
  }
}
