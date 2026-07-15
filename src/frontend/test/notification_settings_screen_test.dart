import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/backend/notification_settings_models.dart';
import 'package:voice_frontend/state/push_notifications_controller.dart';
import 'package:voice_frontend/settings/notification_quiet_hours_storage.dart';
import 'package:voice_frontend/ui/settings/notification_settings_screen.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

class _InMemoryQuietHoursStorage extends NotificationQuietHoursStorage {
  @override
  Future<VoiceQuietHours> read(String profileId) async {
    return VoiceQuietHours.defaults;
  }

  @override
  Future<void> write(String profileId, VoiceQuietHours quietHours) async {}
}

class _FakePushNotificationsController extends PushNotificationsController {
  _FakePushNotificationsController(super.ref);

  @override
  Future<PushPermissionStatus> getPermissionStatus() async {
    return PushPermissionStatus.notDetermined;
  }

  @override
  Future<PushPermissionStatus> requestPermissionAndRegister() async {
    return PushPermissionStatus.granted;
  }
}

void main() {
  testWidgets('settings notifications tile opens notification settings screen', (
    tester,
  ) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/notifications/settings' && req.method == 'GET') {
        return http.Response(
          jsonEncode({
            'notification_settings': {
              'profile_id': 'prof-test',
              'scope_type': 'global',
              'enabled': true,
              'suppress_types_json': '[]',
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/subscription/me' && req.method == 'GET') {
        return http.Response(
          jsonEncode({
            'subscription': {
              'plan': 'free',
              'status': 'cancelled',
              'billing_period': 'monthly',
            },
          }),
          200,
        );
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          notificationQuietHoursStorageProvider.overrideWithValue(
            _InMemoryQuietHoursStorage(),
          ),
          pushNotificationsControllerProvider.overrideWith(
            (ref) => _FakePushNotificationsController(ref),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SettingsSheet()),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.byKey(const Key('settings_notifications')), findsOneWidget);
    expect(find.text('Notifications'), findsOneWidget);

    await tester.tap(find.byKey(const Key('settings_notifications')));
    await tester.pump();
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 50));
      if (find.text('Device notifications').evaluate().isNotEmpty) break;
    }

    expect(find.byKey(NotificationSettingsScreen.screenKey), findsOneWidget);
    expect(find.text('Device notifications'), findsOneWidget);
    expect(find.text('Quiet hours'), findsOneWidget);
  });
}
