import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/notifications_client.dart';
import 'auth_providers.dart';
import '../firebase_options.dart';
import 'in_app_notifications.dart';
import 'push_background_handler.dart';
import 'push_notification_handler.dart';
import 'push_notifications_bootstrap.dart';
import 'push_platform.dart';

export 'push_background_handler.dart' show firebaseMessagingBackgroundHandler;

enum PushPermissionStatus {
  granted,
  denied,
  notDetermined,
  unsupported,
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
        unawaited(_teardown(unregister: true));
      }
    }, fireImmediately: true);
  }

  final Ref _ref;
  ProviderSubscription<AuthState>? _authSub;
  StreamSubscription<RemoteMessage>? _foregroundSub;
  StreamSubscription<String>? _tokenRefreshSub;
  StreamSubscription<RemoteMessage>? _openedSub;
  String? _deviceTokenId;
  String? _lastAuthorization;

  void dispose() {
    _authSub?.close();
    unawaited(_teardown(unregister: false));
  }

  Future<void> _teardown({required bool unregister}) async {
    _foregroundSub?.cancel();
    _foregroundSub = null;
    _tokenRefreshSub?.cancel();
    _tokenRefreshSub = null;
    _openedSub?.cancel();
    _openedSub = null;

    if (unregister) {
      final deviceTokenId = _deviceTokenId;
      final authorization = _lastAuthorization;
      _deviceTokenId = null;
      _lastAuthorization = null;
      if (deviceTokenId != null &&
          deviceTokenId.isNotEmpty &&
          authorization != null) {
        try {
          const bootstrap = PushNotificationsBootstrap();
          await bootstrap.unregisterToken(
            client: _ref.read(voiceNotificationsClientProvider),
            authorization: authorization,
            deviceTokenId: deviceTokenId,
          );
        } catch (_) {
          // Best-effort unregister on logout; registration will upsert on next login.
        }
      }
    }
  }

  Future<void> _syncToken() async {
    final auth = _ref.read(authControllerProvider);
    final header = auth.session?.authorizationHeader;
    if (header == null) return;
    if (DefaultFirebaseOptions.usesDevPlaceholder) return;

    try {
      await _ensureFirebase();
    } catch (_) {
      return;
    }

    final messaging = FirebaseMessaging.instance;
    try {
      final permission = await getPermissionStatus();
      if (permission != PushPermissionStatus.granted) {
        return;
      }
      FirebaseMessaging.onBackgroundMessage(firebaseMessagingBackgroundHandler);

      final platform = pushPlatformForTarget();
      final pushService = pushServiceForTarget();
      final token = await _resolvePushToken(messaging, pushService);
      if (token == null || token.isEmpty) return;

      const bootstrap = PushNotificationsBootstrap();
      final deviceTokenId = await bootstrap.registerToken(
        client: _ref.read(voiceNotificationsClientProvider),
        authorization: header,
        platform: platform,
        token: token,
        pushService: pushService,
      );
      _deviceTokenId = deviceTokenId;
      _lastAuthorization = header;

      _foregroundSub ??=
          FirebaseMessaging.onMessage.listen(_onForegroundMessage);
      _openedSub ??=
          FirebaseMessaging.onMessageOpenedApp.listen(_onOpenedMessage);
      await _handleInitialMessage(messaging);

      _tokenRefreshSub ??= messaging.onTokenRefresh.listen((_) async {
        final currentHeader =
            _ref.read(authControllerProvider).session?.authorizationHeader;
        if (currentHeader == null) return;
        final refreshed = await _resolvePushToken(
          messaging,
          pushServiceForTarget(),
        );
        if (refreshed == null || refreshed.isEmpty) return;
        final newId = await bootstrap.registerToken(
          client: _ref.read(voiceNotificationsClientProvider),
          authorization: currentHeader,
          platform: pushPlatformForTarget(),
          token: refreshed,
          pushService: pushServiceForTarget(),
        );
        _deviceTokenId = newId;
        _lastAuthorization = currentHeader;
      });
    } catch (_) {
      // FCM unavailable (misconfigured project, blocked network, etc.).
    }
  }

  Future<PushPermissionStatus> getPermissionStatus() async {
    if (kIsWeb && DefaultFirebaseOptions.usesDevPlaceholder) {
      return PushPermissionStatus.unsupported;
    }
    try {
      await _ensureFirebase();
    } catch (_) {
      return PushPermissionStatus.unsupported;
    }
    final settings = await FirebaseMessaging.instance.getNotificationSettings();
    return switch (settings.authorizationStatus) {
      AuthorizationStatus.authorized ||
      AuthorizationStatus.provisional => PushPermissionStatus.granted,
      AuthorizationStatus.denied => PushPermissionStatus.denied,
      AuthorizationStatus.notDetermined => PushPermissionStatus.notDetermined,
    };
  }

  /// Shows the OS permission prompt after the in-app explainer, then registers FCM.
  Future<PushPermissionStatus> requestPermissionAndRegister() async {
    if (DefaultFirebaseOptions.usesDevPlaceholder) {
      return PushPermissionStatus.unsupported;
    }
    try {
      await _ensureFirebase();
    } catch (_) {
      return PushPermissionStatus.unsupported;
    }
    final messaging = FirebaseMessaging.instance;
    await messaging.requestPermission();
    await _syncToken();
    return getPermissionStatus();
  }

  void _onForegroundMessage(RemoteMessage message) {
    _dispatchPushData(message.data);
  }

  void _onOpenedMessage(RemoteMessage message) {
    _dispatchPushData(message.data, openChat: true);
  }

  Future<void> _handleInitialMessage(FirebaseMessaging messaging) async {
    final initial = await messaging.getInitialMessage();
    if (initial != null) {
      _onOpenedMessage(initial);
    }
  }

  void _dispatchPushData(Map<String, dynamic> data, {bool openChat = false}) {
    final normalized = data.map((k, v) => MapEntry(k, v.toString()));
    handlePushPayloadMap(normalized, (notificationData) {
      _ref.read(inAppNotificationControllerProvider)?.onPushNotificationData(
            notificationData,
            navigateToChat: openChat,
          );
    });
  }

  Future<void> _ensureFirebase() async {
    if (Firebase.apps.isNotEmpty) return;
    await Firebase.initializeApp(options: DefaultFirebaseOptions.currentPlatform);
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
