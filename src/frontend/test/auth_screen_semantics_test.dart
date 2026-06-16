import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// Phase 18 a11y: explicit Semantics labels on login controls (docs/features/accessibility.md).
void main() {
  testWidgets('login form exposes screen reader labels', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const AuthScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.bySemanticsLabel('Sign in to Voice'), findsAtLeastNWidgets(1));
    expect(find.bySemanticsLabel('Email address'), findsAtLeastNWidgets(1));
    expect(find.bySemanticsLabel('Password'), findsAtLeastNWidgets(1));
    expect(find.bySemanticsLabel('Log in'), findsAtLeastNWidgets(1));
    expect(find.bySemanticsLabel('Create account'), findsAtLeastNWidgets(1));
  });
}
