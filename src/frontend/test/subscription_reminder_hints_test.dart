import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/subscription_reminder_hints.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/ui/settings/subscription_settings_screen.dart';
import 'package:voice_frontend/ui/notifications/subscription_reminder_banner.dart';
import 'support/voice_test_theme.dart';

const profile = '22222222-2222-4222-8222-222222222222';
Map<String, dynamic> payload(DateTime end) => {
  'schema_version': 1,
  'type': 'subscription_grace_reminder',
  'notification_id': '11111111-1111-5111-8111-111111111111',
  'target_profile_id': profile,
  'plan': 'premium',
  'day': 3,
  'window_end': end.toUtc().toIso8601String(),
};

void main() {
  testWidgets('personal action opens existing subscription management screen', (
    tester,
  ) async {
    final hints = SubscriptionReminderHints(profile);
    hints.onData(payload(DateTime.now().add(const Duration(hours: 1))));
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          subscriptionReminderHintsProvider.overrideWith((_) => hints),
          subscriptionProvider.overrideWith((_) async => null),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SubscriptionReminderBannerHost()),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(const Key('subscription_grace_reminder_manage')),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(SubscriptionSettingsScreen.screenKey), findsOneWidget);
  });

  testWidgets(
    'Space reminder preserves context without substituting personal billing',
    (tester) async {
      const space = '44444444-4444-4444-8444-444444444444';
      final hints = SubscriptionReminderHints(profile);
      hints.onData({
        ...payload(DateTime.now().add(const Duration(hours: 1))),
        'plan': 'space_pro',
        'space_id': space,
      });
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            subscriptionReminderHintsProvider.overrideWith((_) => hints),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: SubscriptionReminderBannerHost()),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.textContaining(space), findsOneWidget);
      expect(
        find.byKey(const Key('subscription_grace_reminder_manage')),
        findsNothing,
      );
    },
  );
  test(
    'primary binding, shape, expiry and duplicate collapse after dismissal',
    () {
      final now = DateTime.utc(2026, 10, 27);
      final controller = SubscriptionReminderHints(profile, now: () => now);
      addTearDown(controller.dispose);
      final data = payload(now.add(const Duration(hours: 1)));
      expect(controller.onData(data), isTrue);
      expect(controller.onData(data), isFalse);
      expect(controller.state.single.day, 3);
      controller.dismiss(controller.state.single.id);
      expect(controller.onData(data), isFalse);
      expect(controller.state, isEmpty);
      final anotherProfile = SubscriptionReminderHints(
        '33333333-3333-4333-8333-333333333333',
      );
      addTearDown(anotherProfile.dispose);
      expect(anotherProfile.onData(data), isFalse);
      for (final bad in [
        {...data, 'window_end': now.toIso8601String()},
        {...data, 'target_profile_id': 'invalid'},
        {...data, 'day': 2},
        {...data, 'schema_version': 2},
        {...data, 'aggregate_id': 'private-payer'},
        {...data, 'plan': 'space_pro'},
        {...data, 'space_id': profile},
      ]) {
        expect(
          SubscriptionReminderHint.parse(
            bad,
            activeProfileId: profile,
            now: now,
          ),
          isNull,
        );
      }
      final space = {
        ...data,
        'plan': 'space_pro',
        'space_id': '44444444-4444-4444-8444-444444444444',
      };
      expect(
        SubscriptionReminderHint.parse(
          space,
          activeProfileId: profile,
          now: now,
        )?.spaceId,
        space['space_id'],
      );
    },
  );

  testWidgets('expires visible banner at exact boundary', (tester) async {
    final start = DateTime.utc(2026, 10, 27);
    var now = start;
    final controller = SubscriptionReminderHints(profile, now: () => now);
    addTearDown(controller.dispose);
    expect(
      controller.onData(payload(start.add(const Duration(seconds: 1)))),
      isTrue,
    );
    now = start.add(const Duration(seconds: 1));
    await tester.pump(const Duration(seconds: 1));
    expect(controller.state, isEmpty);
  });

  testWidgets('renders payment issue and invokes supported management action', (
    tester,
  ) async {
    final hint = SubscriptionReminderHint.parse(
      payload(DateTime.now().add(const Duration(hours: 1))),
      activeProfileId: profile,
      now: DateTime.now(),
    )!;
    var managed = false;
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: SubscriptionReminderBanner(
            hint: hint,
            onDismiss: () {},
            onManage: () => managed = true,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(const Key('subscription_grace_reminder_banner')),
      findsOneWidget,
    );
    expect(find.text('Payment failed'), findsOneWidget);
    await tester.tap(
      find.byKey(const Key('subscription_grace_reminder_manage')),
    );
    expect(managed, isTrue);
  });
}
