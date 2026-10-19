import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/discover_hint_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';

import 'support/live_gateway_harness.dart';
import 'support/voice_test_theme.dart';

// Only local storage and the native test binding are substituted. AuthScreen,
// AuthController and every Auth request use the real disposable Gateway.
void main() {
  _LiveBinding();
  testWidgets(
    'two users register, verify delivered mail and password-login via AuthScreen',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final identities = <String>{};

      for (final label in ['first', 'second']) {
        final email = qaUniqueEmail('a1-ui-$label');
        final client = http.Client();
        ProviderContainer makeContainer() => ProviderContainer(
          overrides: [
            gatewayConfigProvider.overrideWithValue(
              GatewayConfig(baseUrl: liveGatewayBaseUrl()),
            ),
            httpClientProvider.overrideWithValue(client),
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            discoverHintStorageProvider.overrideWithValue(
              InMemoryDiscoverHintStorage(),
            ),
          ],
        );
        var container = makeContainer();
        Future<void> mount() => tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              theme: voiceTestTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: const AuthScreen(),
            ),
          ),
        );
        try {
          await tester.runAsync(clearLiveAuthRateLimit);
          await mount();
          await _wait(
            tester,
            () => !container.read(authControllerProvider).isRestoring,
          );
          await tester.enterText(find.byKey(AuthScreen.emailFieldKey), email);
          await tester.enterText(
            find.byKey(AuthScreen.passwordFieldKey),
            qaPassword,
          );
          await tester.tap(find.byKey(AuthScreen.registerButtonKey));
          await _wait(
            tester,
            () => container
                .read(authControllerProvider)
                .isEmailVerificationPending,
          );
          final pending = container.read(authControllerProvider).session!;
          expect(container.read(authControllerProvider).isGuest, isTrue);
          final code = await tester.runAsync(
            () => waitForLiveVerificationCode(
              httpClient: client,
              stubBaseUrl: liveAuthMailStubBaseUrl(),
              email: email,
            ),
          );
          expect(code, isNotNull);
          await tester.enterText(
            find.byKey(AuthScreen.verificationCodeFieldKey),
            code!,
          );
          await tester.tap(find.byKey(AuthScreen.verificationButtonKey));
          await _wait(tester, () {
            final state = container.read(authControllerProvider);
            return state.session != null &&
                !state.isGuest &&
                !state.isSubmitting;
          });
          final regular = container.read(authControllerProvider).session!;
          expect(regular.accountId, pending.accountId);
          expect(regular.activeProfileId, pending.activeProfileId);
          expect(identities.add(regular.accountId), isTrue);

          // A new local session proves the actual password login form. No token
          // from registration is copied into the new controller/storage.
          await tester.pumpWidget(const SizedBox.shrink());
          container.dispose();
          container = makeContainer();
          await mount();
          await _wait(
            tester,
            () => !container.read(authControllerProvider).isRestoring,
          );
          await tester.enterText(find.byKey(AuthScreen.emailFieldKey), email);
          await tester.enterText(
            find.byKey(AuthScreen.passwordFieldKey),
            qaPassword,
          );
          await tester.tap(find.byKey(AuthScreen.loginButtonKey));
          await _wait(tester, () {
            final state = container.read(authControllerProvider);
            return state.session != null &&
                !state.isGuest &&
                !state.isSubmitting;
          });
          expect(
            container.read(authControllerProvider).session!.accountId,
            regular.accountId,
          );
          expect(
            container.read(authControllerProvider).session!.activeProfileId,
            regular.activeProfileId,
          );
        } finally {
          await tester.pumpWidget(const SizedBox.shrink());
          container.dispose();
          client.close();
        }
      }
      expect(identities, hasLength(2));
    },
    skip: !runLiveIntegration,
    timeout: const Timeout(Duration(minutes: 6)),
  );
}

Future<void> _wait(WidgetTester tester, bool Function() ready) async {
  final deadline = DateTime.now().add(const Duration(seconds: 60));
  while (!ready() && DateTime.now().isBefore(deadline)) {
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 100)),
    );
    await tester.pump();
  }
  expect(
    ready(),
    isTrue,
    reason: 'Public Auth UI did not reach the expected state',
  );
  expect(tester.takeException(), isNull);
}

class _LiveBinding extends AutomatedTestWidgetsFlutterBinding {
  @override
  bool get overrideHttpClient => false;
}
