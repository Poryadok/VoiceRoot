import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';
import 'package:voice_frontend/ui/settings/subscription_settings_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

http.Response _subscriptionResponse({
  required String plan,
  required String status,
  String billingPeriod = 'monthly',
  String? currentPeriodEnd,
}) {
  return http.Response(
    jsonEncode({
      'subscription': {
        'id': 'sub-test',
        'plan': plan,
        'status': status,
        'billing_period': billingPeriod,
        'current_period_end': ?currentPeriodEnd,
      },
    }),
    200,
  );
}

Future<void> _pumpSubscriptionScreen(
  WidgetTester tester, {
  required MockClient client,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: voiceAppTestOverrides(client: client),
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const SubscriptionSettingsScreen(),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('settings sheet shows subscription section', (tester) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/subscription/me' && req.method == 'GET') {
        return _subscriptionResponse(plan: 'free', status: 'cancelled');
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
    expect(find.text('Free'), findsOneWidget);
  });

  testWidgets('free plan shows non-blocking upsell and tier-0 note', (
    tester,
  ) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/subscription/me' && req.method == 'GET') {
        return _subscriptionResponse(plan: 'free', status: 'cancelled');
      }
      return http.Response('Not Found', 404);
    });

    await _pumpSubscriptionScreen(tester, client: client);

    expect(find.byKey(SubscriptionSettingsScreen.planStateKey), findsOneWidget);
    expect(find.text('Free'), findsWidgets);
    expect(find.byKey(SubscriptionSettingsScreen.freeTierNoteKey), findsOneWidget);
    expect(
      find.text('Messages and chats stay free on the Free plan.'),
      findsOneWidget,
    );
    expect(find.byKey(SubscriptionSettingsScreen.upgradeSectionKey), findsOneWidget);
    expect(find.text('Premium — monthly'), findsOneWidget);
    expect(find.text('Premium — yearly (−20%)'), findsOneWidget);
  });

  testWidgets('active premium shows plan state without upsell', (tester) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/subscription/me' && req.method == 'GET') {
        return _subscriptionResponse(
          plan: 'premium',
          status: 'active',
          billingPeriod: 'yearly',
        );
      }
      return http.Response('Not Found', 404);
    });

    await _pumpSubscriptionScreen(tester, client: client);

    expect(find.text('Premium'), findsWidgets);
    expect(find.text('Billing: Yearly'), findsOneWidget);
    expect(find.byKey(SubscriptionSettingsScreen.upgradeSectionKey), findsNothing);
    expect(find.byKey(SubscriptionSettingsScreen.freeTierNoteKey), findsNothing);
  });

  testWidgets('grace period shows payment issue state', (tester) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/subscription/me' && req.method == 'GET') {
        return _subscriptionResponse(plan: 'premium', status: 'grace_period');
      }
      return http.Response('Not Found', 404);
    });

    await _pumpSubscriptionScreen(tester, client: client);

    expect(find.text('Premium — payment issue'), findsOneWidget);
    expect(
      find.text('Update your payment method to keep Premium benefits.'),
      findsOneWidget,
    );
    expect(find.byKey(SubscriptionSettingsScreen.upgradeSectionKey), findsNothing);
  });

  testWidgets('subscription load error shows retry panel', (tester) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/subscription/me' && req.method == 'GET') {
        return http.Response('{"error":"upstream"}', 503);
      }
      return http.Response('Not Found', 404);
    });

    await _pumpSubscriptionScreen(tester, client: client);

    expect(find.text('Could not load subscription'), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);
  });
}
