import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/push_notifications_controller.dart';
import 'package:voice_frontend/backend/notification_settings_models.dart';
import 'package:voice_frontend/settings/notification_quiet_hours_storage.dart';
import 'package:voice_frontend/ui/matchmaking/game_catalog_screen.dart';
import 'package:voice_frontend/ui/settings/notification_settings_screen.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';
import 'package:voice_frontend/ui/stories/story_archive_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

/// System text scale covered by docs/features/accessibility.md (×1.5 smoke).
const _textScale = 1.5;

class _InMemoryQuietHoursStorage extends NotificationQuietHoursStorage {
  @override
  Future<VoiceQuietHours> read(String profileId) async =>
      VoiceQuietHours.defaults;

  @override
  Future<void> write(String profileId, VoiceQuietHours quietHours) async {}
}

class _FakePushNotificationsController extends PushNotificationsController {
  _FakePushNotificationsController(super.ref);

  @override
  Future<PushPermissionStatus> getPermissionStatus() async =>
      PushPermissionStatus.notDetermined;
}

void main() {
  testWidgets('SettingsSheet has no overflow at text scale 1.5', (tester) async {
    await tester.pumpWidget(
      _scaledTestApp(
        client: MockClient((_) async => http.Response('{}', 404)),
        home: const SettingsSheet(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SettingsSheet.sheetKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('NotificationSettingsScreen has no overflow at text scale 1.5', (
    tester,
  ) async {
    final quietStorage = _InMemoryQuietHoursStorage();
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
      if (req.url.path == '/api/v1/notifications/quiet-hours' &&
          req.method == 'GET') {
        return http.Response(
          jsonEncode({
            'quiet_hours': {
              'enabled': false,
              'start_time': '22:00',
              'end_time': '07:00',
              'timezone': 'UTC',
              'override_mentions': false,
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/subscription/me') {
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
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          notificationQuietHoursStorageProvider.overrideWithValue(quietStorage),
          pushNotificationsControllerProvider.overrideWith(
            (ref) => _FakePushNotificationsController(ref),
          ),
        ],
        child: _scaledMaterialApp(const NotificationSettingsScreen()),
      ),
    );

    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 50));
      if (find.byKey(NotificationSettingsScreen.screenKey).evaluate().isNotEmpty) {
        break;
      }
    }

    expect(find.byKey(NotificationSettingsScreen.screenKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('GameCatalogScreen has no overflow at text scale 1.5', (
    tester,
  ) async {
    await tester.pumpWidget(
      _scaledTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/matchmaking/games') {
            return http.Response(
              jsonEncode({
                'game_catalog': {
                  'games': [
                    {
                      'id': 'game-1',
                      'name': 'Counter-Strike 2',
                      'slug': 'cs2',
                      'icon_url': '',
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
        home: const GameCatalogScreen(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(GameCatalogScreen.screenKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('StoryArchiveScreen has no overflow at text scale 1.5', (
    tester,
  ) async {
    await tester.pumpWidget(
      _scaledTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/stories/archive') {
            return http.Response(
              jsonEncode({'archive': {'stories': []}}),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
        home: const StoryArchiveScreen(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(StoryArchiveScreen.screenKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}

Widget _scaledTestApp({
  required http.Client client,
  required Widget home,
}) {
  return ProviderScope(
    overrides: voiceAppTestOverrides(client: client),
    child: _scaledMaterialApp(home),
  );
}

Widget _scaledMaterialApp(Widget home) {
  return MaterialApp(
    theme: voiceTestTheme(),
    locale: const Locale('en'),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    builder: (context, child) {
      final media = MediaQuery.of(context);
      return MediaQuery(
        data: media.copyWith(
          textScaler: const TextScaler.linear(_textScale),
          size: const Size(390, 844),
        ),
        child: child!,
      );
    },
    home: Scaffold(body: home),
  );
}
