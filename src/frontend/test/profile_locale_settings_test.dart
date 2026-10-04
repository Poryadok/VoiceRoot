import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('choosing Russian persists it to the active profile', (
    tester,
  ) async {
    final patches = <Map<String, dynamic>>[];
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/users/profiles/prof-test') {
        return http.Response(
          jsonEncode({'profile': _profile('prof-test')}),
          200,
        );
      }
      if (request.method == 'PATCH' && request.url.path == '/api/v1/users/me') {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        patches.add(body);
        return http.Response(
          jsonEncode({
            'profile': _profile('prof-test', locale: body['locale'] as String),
          }),
          200,
        );
      }
      return http.Response('', 404);
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SettingsSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(SettingsSheet.languageKey));
    await tester.tap(find.text('Russian'));
    await tester.pumpAndSettle();

    expect(patches, hasLength(1));
    expect(patches.single['locale'], 'ru');
    expect(container.read(appLocalePreferenceProvider), const Locale('ru'));
  });

  testWidgets('System selection is a local override and sends no reset', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final patches = <Map<String, dynamic>>[];
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/users/profiles/prof-test') {
        return http.Response(
          jsonEncode({'profile': _profile('prof-test', locale: 'ru')}),
          200,
        );
      }
      if (request.method == 'PATCH' && request.url.path == '/api/v1/users/me') {
        patches.add(jsonDecode(request.body) as Map<String, dynamic>);
        return http.Response(
          jsonEncode({'profile': _profile('prof-test', locale: 'ru')}),
          200,
        );
      }
      return http.Response('', 404);
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SettingsSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(SettingsSheet.languageKey));
    expect(find.bySemanticsLabel('System default'), findsOneWidget);
    await tester.tap(find.text('System default'));
    await tester.pumpAndSettle();

    expect(patches, isEmpty);
    expect(container.read(appLocalePreferenceProvider), isNull);
    semantics.dispose();
  });

  testWidgets('active profile locale hydrates and follows profile switches', (
    tester,
  ) async {
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path.startsWith('/api/v1/users/profiles/')) {
        final profileId = request.url.path.split('/').last;
        return http.Response(
          jsonEncode({
            'profile': _profile(
              profileId,
              locale: profileId == 'prof-next' ? 'en' : 'ru',
            ),
          }),
          200,
        );
      }
      return http.Response('', 404);
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(_settingsApp(container));
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), const Locale('ru'));
    await tester.ensureVisible(find.byKey(SettingsSheet.languageKey));
    await tester.tap(find.text('System default'));
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), isNull);

    container.read(authControllerProvider.notifier).state = AuthState(
      session: _session('prof-next'),
    );
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), const Locale('en'));
  });

  testWidgets('same-profile session refresh clears System override', (
    tester,
  ) async {
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/users/profiles/prof-test') {
        return http.Response(
          jsonEncode({'profile': _profile('prof-test', locale: 'ru')}),
          200,
        );
      }
      return http.Response('', 404);
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(_settingsApp(container));
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), const Locale('ru'));

    await tester.ensureVisible(find.byKey(SettingsSheet.languageKey));
    await tester.tap(find.text('System default'));
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), isNull);

    container.read(authControllerProvider.notifier).state = AuthState(
      session: AuthSession(
        accessToken: 'refreshed-token',
        refreshToken: 'refreshed-refresh',
        accountId: 'account-test',
        activeProfileId: 'prof-test',
        expiresInSeconds: 900,
      ),
    );
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), const Locale('ru'));
  });

  testWidgets('failed save keeps locale and retry applies returned profile', (
    tester,
  ) async {
    var patchCount = 0;
    final failedFirstRequest = Completer<http.Response>();
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/users/profiles/prof-test') {
        return http.Response(
          jsonEncode({'profile': _profile('prof-test', locale: 'en')}),
          200,
        );
      }
      if (request.method == 'PATCH' && request.url.path == '/api/v1/users/me') {
        patchCount++;
        if (patchCount == 1) return failedFirstRequest.future;
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'profile': _profile('prof-test', locale: body['locale'] as String),
          }),
          200,
        );
      }
      return http.Response('', 404);
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(_settingsApp(container));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(SettingsSheet.languageKey));
    await tester.tap(find.text('Russian'));
    await tester.pump();
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
    failedFirstRequest.complete(http.Response('', 503));
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), const Locale('en'));
    expect(find.byKey(const Key('settings_language_error')), findsOneWidget);

    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(patchCount, 2);
    expect(container.read(appLocalePreferenceProvider), const Locale('ru'));
  });

  testWidgets('late save cannot replace the locale after a profile switch', (
    tester,
  ) async {
    final heldPatch = Completer<http.Response>();
    final patchStarted = Completer<void>();
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path.startsWith('/api/v1/users/profiles/')) {
        final profileId = request.url.path.split('/').last;
        return http.Response(
          jsonEncode({'profile': _profile(profileId, locale: 'ru')}),
          200,
        );
      }
      if (request.method == 'PATCH' && request.url.path == '/api/v1/users/me') {
        if (!patchStarted.isCompleted) patchStarted.complete();
        return heldPatch.future;
      }
      return http.Response('', 404);
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(_settingsApp(container));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(SettingsSheet.languageKey));
    await tester.tap(find.text('English'));
    await tester.pump();
    await tester.runAsync(() async {
      await patchStarted.future;
    });

    container.read(authControllerProvider.notifier).state = AuthState(
      session: _session('prof-next'),
    );
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), const Locale('ru'));

    heldPatch.complete(
      http.Response(
        jsonEncode({'profile': _profile('prof-test', locale: 'en')}),
        200,
      ),
    );
    await tester.pumpAndSettle();
    expect(container.read(appLocalePreferenceProvider), const Locale('ru'));
  });
}

Widget _settingsApp(ProviderContainer container) => UncontrolledProviderScope(
  container: container,
  child: MaterialApp(
    theme: voiceTestTheme(),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: const Scaffold(body: SettingsSheet()),
  ),
);

AuthSession _session(String profileId) => AuthSession(
  accessToken: 'token-$profileId',
  refreshToken: 'refresh-$profileId',
  accountId: 'account-test',
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

Map<String, dynamic> _profile(String id, {String locale = 'en'}) => {
  'id': id,
  'account_id': 'account-test',
  'username': 'voiceuser',
  'discriminator': '4242',
  'display_name': 'Voice User',
  'locale': locale,
};
