import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _dialogKey = Key('profile_remove_friend_dialog');
const _cancelKey = Key('profile_remove_friend_cancel');
const _confirmKey = Key('profile_remove_friend_confirm');

void main() {
  for (final locale in ['en', 'ru']) {
    testWidgets('remove friend asks accessibly in $locale; cancel is safe', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      try {
        final harness = _RemovalHarness();
        await harness.open(tester, locale: Locale(locale));
        await _askToRemove(tester);

        final context = tester.element(find.byKey(_dialogKey));
        final l10n = AppLocalizations.of(context)!;
        expect(find.text(l10n.profileRemoveFriendConfirmTitle), findsOneWidget);
        expect(find.text(l10n.profileRemoveFriendConfirmMessage), findsOneWidget);
        expect(
          find.descendant(
            of: find.byKey(_dialogKey),
            matching: find.bySemanticsLabel(l10n.socialRemoveFriend),
          ),
          findsOneWidget,
        );
        expect(
          find.descendant(
            of: find.byKey(_dialogKey),
            matching: find.bySemanticsLabel(l10n.commonCancel),
          ),
          findsOneWidget,
        );
        expect(harness.removals, isEmpty);

        await tester.tap(find.byKey(_cancelKey));
        await tester.pumpAndSettle();
        expect(harness.removals, isEmpty);
        expect(find.byKey(_dialogKey), findsNothing);
        expect(find.byKey(ProfileDetailSheet.removeFriendKey), findsOneWidget);
        expect(tester.takeException(), isNull);
      } finally {
        semantics.dispose();
      }
    });
  }

  for (final dismissal in ['barrier', 'escape']) {
    testWidgets('$dismissal dismisses confirmation without removing friend', (
      tester,
    ) async {
      final harness = _RemovalHarness();
      await harness.open(tester);
      await _askToRemove(tester);
      if (dismissal == 'barrier') {
        await tester.tapAt(const Offset(4, 4));
      } else {
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      }
      await tester.pumpAndSettle();
      expect(find.byKey(_dialogKey), findsNothing);
      expect(harness.removals, isEmpty);
      expect(find.byKey(ProfileDetailSheet.removeFriendKey), findsOneWidget);
    });
  }

  testWidgets('confirmation sends one removal and closes on success', (
    tester,
  ) async {
    final response = Completer<http.Response>();
    final harness = _RemovalHarness()..response = response.future;
    await harness.open(tester);
    await _askToRemove(tester);
    await tester.tap(find.byKey(_confirmKey));
    await tester.pumpAndSettle();

    expect(harness.removals, hasLength(1));
    expect(harness.removals.single.url.path, '/api/v1/friends/p-target');
    expect(harness.removals.single.method, 'DELETE');
    expect(
      harness.removals.single.headers['authorization'],
      'Bearer test-access',
    );
    final button = tester.widget<OutlinedButton>(
      find.byKey(ProfileDetailSheet.removeFriendKey),
    );
    expect(button.onPressed, isNull);
    await tester.tap(find.byKey(ProfileDetailSheet.removeFriendKey));
    await tester.pump();
    expect(harness.removals, hasLength(1));

    response.complete(http.Response('', 204));
    await tester.pumpAndSettle();
    expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
    expect(find.text('Open profile'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('failed removal stays open and requires fresh confirmation', (
    tester,
  ) async {
    final harness = _RemovalHarness()
      ..response = Future.value(
        http.Response('{"message":"remove-private-diagnostic"}', 500),
      );
    await harness.open(tester);
    await _askToRemove(tester);
    await tester.tap(find.byKey(_confirmKey));
    await tester.pumpAndSettle();
    expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.textContaining('remove-private-diagnostic'), findsNothing);

    harness.response = Future.value(http.Response('', 204));
    await _askToRemove(tester);
    expect(harness.removals, hasLength(1));
    await tester.tap(find.byKey(_confirmKey));
    await tester.pumpAndSettle();
    expect(harness.removals, hasLength(2));
    expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
  });

  for (final change in ['account', 'profile', 'token', 'switch', 'failed-switch']) {
    testWidgets('$change change while confirming cannot remove friend', (
      tester,
    ) async {
      final harness = _RemovalHarness();
      await harness.open(tester);
      await _askToRemove(tester);
      if (change == 'failed-switch') {
        await harness.failProfileSwitch(tester);
      } else {
        harness.changeContext(tester, change);
      }
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(_confirmKey));
      await tester.pumpAndSettle();
      expect(harness.removals, isEmpty);
      expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  for (final change in ['token', 'failed-switch']) {
    for (final status in [204, 500]) {
      testWidgets('$change fences stale $status removal completion', (
        tester,
      ) async {
        final response = Completer<http.Response>();
        final harness = _RemovalHarness()..response = response.future;
        await harness.open(tester);
        await _askToRemove(tester);
        await tester.tap(find.byKey(_confirmKey));
        await tester.pumpAndSettle();
        if (change == 'failed-switch') {
          await harness.failProfileSwitch(tester);
        } else {
          harness.changeContext(tester, change);
        }
        await tester.pumpAndSettle();
        response.complete(
          http.Response('{"message":"stale-remove-private-diagnostic"}', status),
        );
        await tester.pumpAndSettle();
        expect(harness.removals, hasLength(1));
        expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
        expect(find.text('Could not complete this action.'), findsNothing);
        expect(
          find.textContaining('stale-remove-private-diagnostic'),
          findsNothing,
        );
        expect(tester.takeException(), isNull);
      });
    }
  }
}

Future<void> _askToRemove(WidgetTester tester) async {
  await tester.ensureVisible(find.byKey(ProfileDetailSheet.removeFriendKey));
  await tester.tap(find.byKey(ProfileDetailSheet.removeFriendKey));
  await tester.pumpAndSettle();
  expect(find.byKey(_dialogKey), findsOneWidget);
}

class _RemovalHarness {
  final removals = <http.Request>[];
  Future<http.Response> response = Future.value(http.Response('', 204));
  late AuthController authController;
  int switchRequests = 0;

  Future<void> failProfileSwitch(WidgetTester tester) async {
    final container = ProviderScope.containerOf(
      tester.element(find.byKey(ProfileDetailSheet.sheetKey)),
    );
    final session = authController.state.session;
    final generation = authController.sessionInstallGeneration;
    container.read(profileSwitchInProgressProvider.notifier).state = true;
    expect(await authController.switchActiveProfile('other-profile'), isNotNull);
    container.read(profileSwitchInProgressProvider.notifier).state = false;
    expect(authController.state.session, same(session));
    expect(authController.sessionInstallGeneration, greaterThan(generation));
    expect(switchRequests, 1);
  }

  void changeContext(WidgetTester tester, String change) {
    if (change == 'switch') {
      final container = ProviderScope.containerOf(
        tester.element(find.byKey(ProfileDetailSheet.sheetKey)),
      );
      container.read(profileSwitchInProgressProvider.notifier).state = true;
      return;
    }
    authController.state = AuthState(
      session: AuthSession(
        accessToken: change == 'token' ? 'replacement-access' : 'test-access',
        refreshToken: 'test-refresh',
        accountId: change == 'account' ? 'other-account' : 'acc-test',
        activeProfileId: change == 'profile' ? 'other-profile' : 'prof-test',
        expiresInSeconds: 900,
      ),
    );
  }

  Future<void> open(
    WidgetTester tester, {
    Locale locale = const Locale('en'),
  }) async {
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/switch-profile') {
        switchRequests++;
        return http.Response('{"message":"switch-unavailable"}', 503);
      }
      if (request.method == 'DELETE') {
        removals.add(request);
        return response;
      }
      if (request.url.path == '/api/v1/friends') {
        return http.Response(
          jsonEncode({
            'friend_list': {'profile_ids': ['p-target']},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/users/profiles/p-target') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-target',
              'account_id': 'a-target',
              'username': 'target',
              'display_name': 'Target',
              'discriminator': '0001',
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      return http.Response('{}', 200);
    });
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith((ref) {
            authController = authenticatedAuthController(ref);
            return authController;
          }),
          profileSwitchInProgressProvider.overrideWith((ref) => false),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          httpClientProvider.overrideWithValue(client),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: locale,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                onPressed: () => showModalBottomSheet<void>(
                  context: context,
                  isScrollControlled: true,
                  builder: (_) =>
                      const ProfileDetailSheet(profileId: 'p-target'),
                ),
                child: const Text('Open profile'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('Open profile'));
    await tester.pumpAndSettle();
  }
}
