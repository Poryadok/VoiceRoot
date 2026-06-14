import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('settings sheet shows subscription section', (tester) async {
    final client = MockClient((req) async {
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
        overrides: voiceAppTestOverrides(client: client),
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

    expect(find.byKey(const Key('settings_subscription')), findsOneWidget);
    expect(find.text('Subscription'), findsOneWidget);
  });
}
