import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:intl/intl.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/profile/manage_profiles_sheet.dart';
import 'package:voice_frontend/ui/profile/profile_downgrade_picker_screen.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';
import 'package:voice_frontend/ui/settings/subscription_settings_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

const List<Size> kV3Sizes = [Size(1280, 800), Size(390, 844)];

http.Response _subscriptionResponse({
  required String plan,
  required String status,
  String billingPeriod = 'monthly',
  String? providerSubscriptionId,
}) {
  return http.Response(
    jsonEncode({
      'subscription': {
        'id': 'sub-v3',
        'plan': plan,
        'status': status,
        'billing_period': billingPeriod,
        'provider_subscription_id': ?providerSubscriptionId,
      },
    }),
    200,
  );
}

http.Response _profilesResponse(int count) {
  return http.Response(
    jsonEncode({
      'profile_list': {
        'profiles': [
          for (var i = 0; i < count; i++)
            {
              'id': 'p-v3-$i',
              'display_name': 'V3 Profile ${i + 1}',
              'is_primary': i == 0,
            },
        ],
      },
    }),
    200,
  );
}

Map<String, Object?> _billingEventWire(
  String id,
  String type, {
  DateTime? occurredAt,
}) {
  return {
    'id': id,
    'type': type,
    if (occurredAt != null) 'occurred_at': occurredAt.toIso8601String(),
    'payload_json': '{"amount_minor":"1999"}',
  };
}

http.Response _billingHistoryResponse(
  List<Map<String, Object?>> events, {
  String? nextCursor,
}) {
  return http.Response(
    jsonEncode({
      'billing_history_list': {
        'events': events,
        'next_cursor': ?nextCursor,
      },
    }),
    200,
  );
}

Widget _subscriptionApp(http.Client client) {
  return ProviderScope(
    overrides: voiceAppTestOverrides(client: client),
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const SubscriptionSettingsScreen(),
    ),
  );
}

class _ProfilesWarmer extends ConsumerWidget {
  const _ProfilesWarmer();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(myProfilesProvider);
    return const SizedBox();
  }
}

Widget _subscriptionWarmedApp(http.Client client) {
  return ProviderScope(
    overrides: voiceAppTestOverrides(client: client),
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const Stack(
        children: [_ProfilesWarmer(), SubscriptionSettingsScreen()],
      ),
    ),
  );
}

Widget _settingsSheetApp(http.Client client) {
  return ProviderScope(
    overrides: voiceAppTestOverrides(client: client),
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const Scaffold(body: SettingsSheet()),
    ),
  );
}

// Mirrors protoTimestampToDateTime -> toLocal() on the rendered event date.
DateTime _wireToRenderedDate(DateTime utc) {
  final seconds = utc.millisecondsSinceEpoch ~/ 1000;
  return DateTime.fromMillisecondsSinceEpoch(seconds * 1000, isUtc: true)
      .toLocal();
}

String _enDate(DateTime rendered) =>
    DateFormat.yMMMd('en').format(rendered);

void main() {
  testWidgets(
    'free plan hides billing history and banner, keeps manage profiles',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(plan: 'free', status: 'cancelled');
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        expect(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
          findsNothing,
          reason: '$size billing history hidden for free',
        );
        expect(find.text('Billing history'), findsNothing);
        expect(
          find.byKey(SubscriptionSettingsScreen.paymentFailureBannerKey),
          findsNothing,
          reason: '$size no payment failure banner on free',
        );
        expect(
          find.byKey(SubscriptionSettingsScreen.manageProfilesKey),
          findsOneWidget,
          reason: '$size manage profiles always visible',
        );
        expect(find.text('Manage profiles'), findsOneWidget);
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'active premium shows billing history button without the banner',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(
              plan: 'premium',
              status: 'active',
              providerSubscriptionId: 'paddle-v3',
            );
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;
        expect(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
          findsOneWidget,
          reason: '$size billing history for subscribed',
        );
        expect(find.text(l10n.subscriptionBillingHistory), findsOneWidget);
        expect(
          find.byKey(SubscriptionSettingsScreen.paymentFailureBannerKey),
          findsNothing,
          reason: '$size banner only during grace',
        );
        expect(find.text(l10n.subscriptionPaymentFailed), findsNothing);
        expect(find.text(l10n.subscriptionCancel), findsOneWidget);
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'grace period renders payment failure banner with update payment action',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(
              plan: 'premium',
              status: 'grace_period',
              providerSubscriptionId: 'paddle-v3',
            );
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;
        expect(
          find.byKey(SubscriptionSettingsScreen.paymentFailureBannerKey),
          findsOneWidget,
          reason: '$size grace banner',
        );
        expect(find.text(l10n.subscriptionPaymentFailed), findsOneWidget);
        expect(
          find.byKey(SubscriptionSettingsScreen.updatePaymentKey),
          findsOneWidget,
          reason: '$size update payment for provider id',
        );
        expect(
          find.byKey(SubscriptionSettingsScreen.updatePaymentKey).hitTestable(),
          findsOneWidget,
          reason: '$size update payment actionable',
        );
        expect(find.text(l10n.subscriptionUpdatePayment), findsOneWidget);
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'grace without provider id keeps banner but drops update payment action',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(
              plan: 'premium',
              status: 'grace_period',
            );
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;
        expect(
          find.byKey(SubscriptionSettingsScreen.paymentFailureBannerKey),
          findsOneWidget,
        );
        expect(
          find.text(l10n.subscriptionPaymentFailed),
          findsOneWidget,
          reason: '$size banner copy',
        );
        expect(
          find.byKey(SubscriptionSettingsScreen.updatePaymentKey),
          findsNothing,
          reason: '$size no update payment without provider id',
        );
        expect(find.text(l10n.subscriptionManageBilling), findsNothing);
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'billing history tap loads transactions and shows empty gateway copy',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        var historyCalls = 0;
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(plan: 'premium', status: 'active');
          }
          if (req.url.path == '/api/v1/subscription/billing/history' &&
              req.method == 'GET') {
            historyCalls++;
            return _billingHistoryResponse([]);
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
        );
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;
        expect(
          find.text(l10n.subscriptionBillingEmpty),
          findsOneWidget,
          reason: '$size empty billing activity copy',
        );
        expect(
          historyCalls,
          1,
          reason: '$size history route hit once, GET only',
        );
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'billing history stub without history list also renders empty copy',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(plan: 'premium', status: 'active');
          }
          if (req.url.path == '/api/v1/subscription/billing/history' &&
              req.method == 'GET') {
            return http.Response('{}', 200);
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
        );
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;
        expect(find.text(l10n.subscriptionBillingEmpty), findsOneWidget);
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'billing history renders event rows with rendered localized dates',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final failedAt = DateTime.utc(2025, 11, 1);
        final createdWire = _billingEventWire(
          'ev-v3-created',
          'subscription_created',
        );
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(
              plan: 'premium',
              status: 'active',
              providerSubscriptionId: 'paddle-v3',
            );
          }
          if (req.url.path == '/api/v1/subscription/billing/history' &&
              req.method == 'GET') {
            return _billingHistoryResponse(
              [
                _billingEventWire(
                  'ev-v3-failed',
                  'payment_failed',
                  occurredAt: failedAt,
                ),
                createdWire,
              ],
              nextCursor: 'c-v3',
            );
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
        );
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;
        expect(
          find.text(l10n.subscriptionBillingEvent),
          findsNWidgets(2),
          reason: '$size one row per history event, static label',
        );
        expect(
          find.text(_enDate(_wireToRenderedDate(failedAt))),
          findsOneWidget,
          reason: '$size occurred_at date rendered',
        );
        expect(
          find.textContaining('payment_failed'),
          findsNothing,
          reason: '$size wire type not rendered raw',
        );
        expect(
          find.text(l10n.subscriptionBillingEmpty),
          findsNothing,
          reason: '$size non-empty list',
        );
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'billing history loading state keeps skeleton until response arrives',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final pending = Completer<void>();
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(plan: 'premium', status: 'active');
          }
          if (req.url.path == '/api/v1/subscription/billing/history' &&
              req.method == 'GET') {
            await pending.future;
            return _billingHistoryResponse(
              [
                _billingEventWire(
                  'ev-v3-late',
                  'payment_success',
                  occurredAt: DateTime.utc(2026, 1, 15),
                ),
              ],
            );
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
        );
        await tester.pump();
        expect(
          find.byType(VoiceListSkeleton),
          findsOneWidget,
          reason: '$size billing loading skeleton',
        );
        pending.complete();
        await tester.pumpAndSettle();
        expect(find.byType(VoiceListSkeleton), findsNothing);
        expect(find.text('Billing event'), findsOneWidget);
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'billing history failure keeps dedicated copy and hides diagnostics',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        const genericSecret = 'provider diagnostic=billing-private-111';
        const unavailableSecret = 'provider diagnostic=billing-unavailable-222';
        var historyFails = 0;
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(plan: 'premium', status: 'active');
          }
          if (req.url.path == '/api/v1/subscription/billing/history' &&
              req.method == 'GET') {
            historyFails++;
            return http.Response(
              jsonEncode({
                'error_code': 'provider_error',
                'message': historyFails == 1
                    ? genericSecret
                    : unavailableSecret,
              }),
              historyFails == 1 ? 500 : 503,
            );
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;

        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
        );
        await tester.pumpAndSettle();
        expect(find.text(l10n.subscriptionBillingUnavailable), findsOneWidget);
        expect(find.textContaining(genericSecret), findsNothing);
        expect(find.text(l10n.subscriptionLoadError), findsNothing);

        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
        );
        await tester.pumpAndSettle();
        expect(
          find.text(l10n.subscriptionBillingUnavailable),
          findsOneWidget,
          reason: '$size dedicated copy on 503 too',
        );
        expect(find.textContaining(unavailableSecret), findsNothing);
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'update payment action is disabled while billing transactions load',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final pending = Completer<void>();
        var billingRequests = 0;
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(
              plan: 'premium',
              status: 'grace_period',
              providerSubscriptionId: 'paddle-v3',
            );
          }
          if (req.url.path == '/api/v1/subscription/billing/history' &&
              req.method == 'GET') {
            billingRequests++;
            await pending.future;
            return _billingHistoryResponse([
              _billingEventWire('ev-v3-grace', 'payment_failed'),
            ]);
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
        );
        await tester.pump();
        expect(
          find.byKey(SubscriptionSettingsScreen.updatePaymentKey),
          findsOneWidget,
          reason: '$size banner stays during load',
        );
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
        );
        await tester.pump();
        expect(
          billingRequests,
          1,
          reason: '$size re-tap while loading issues no new request',
        );
        expect(
          find.byType(VoiceListSkeleton),
          findsOneWidget,
          reason: '$size skeleton persists while loading',
        );
        pending.complete();
        await tester.pumpAndSettle();
        expect(
          find.byKey(SubscriptionSettingsScreen.updatePaymentKey).hitTestable(),
          findsOneWidget,
          reason: '$size restored after load',
        );
        expect(find.text('Billing event'), findsOneWidget);
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'premium manage profiles tap opens the manage profiles bottom sheet',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(
              plan: 'premium',
              status: 'active',
              providerSubscriptionId: 'paddle-v3',
            );
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionWarmedApp(client));
        await tester.pumpAndSettle();
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.manageProfilesKey),
        );
        await tester.pumpAndSettle();
        expect(
          find.byKey(ManageProfilesSheet.sheetKey),
          findsOneWidget,
          reason: '$size bottom sheet opened without pop',
        );
        expect(
          find.byKey(ProfileDowngradePickerScreen.pickerKey),
          findsNothing,
          reason: '$size premium never routes to picker',
        );
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'free plan with two profiles still opens the manage profiles sheet',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(plan: 'free', status: 'cancelled');
          }
          if (req.url.path == '/api/v1/users/profiles' &&
              req.method == 'GET') {
            return _profilesResponse(2);
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionWarmedApp(client));
        await tester.pumpAndSettle();
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.manageProfilesKey),
        );
        await tester.pumpAndSettle();
        expect(find.byKey(ManageProfilesSheet.sheetKey), findsOneWidget);
        expect(
          find.byKey(ProfileDowngradePickerScreen.pickerKey),
          findsNothing,
          reason: '$size within free profile limit',
        );
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'free plan over the profile limit routes manage profiles to picker',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(plan: 'free', status: 'cancelled');
          }
          if (req.url.path == '/api/v1/users/profiles' &&
              req.method == 'GET') {
            return _profilesResponse(3);
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionWarmedApp(client));
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;
        await tester.tap(
          find.byKey(SubscriptionSettingsScreen.manageProfilesKey),
        );
        await tester.pumpAndSettle();
        expect(
          find.byKey(ProfileDowngradePickerScreen.pickerKey),
          findsOneWidget,
          reason: '$size downgrade picker route',
        );
        expect(
          find.text(l10n.downgradeProfilePickerTitle),
          findsOneWidget,
          reason: '$size picker title',
        );
        expect(
          find.byKey(ManageProfilesSheet.sheetKey),
          findsNothing,
          reason: '$size picker replaces sheet route',
        );
        Navigator.of(
          tester.element(
            find.byKey(ProfileDowngradePickerScreen.pickerKey),
          ),
        ).pop();
        await tester.pumpAndSettle();
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'manage profiles and billing controls reach through settings sheet nav',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return _subscriptionResponse(
              plan: 'premium',
              status: 'active',
              providerSubscriptionId: 'paddle-v3',
            );
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_settingsSheetApp(client));
        await tester.pumpAndSettle();
        expect(
          find.byKey(const Key('settings_subscription')),
          findsOneWidget,
          reason: '$size sheet row',
        );
        await tester.tap(find.byKey(const Key('settings_subscription')));
        await tester.pumpAndSettle();
        expect(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
          findsOneWidget,
          reason: '$size billing control after nav',
        );
        expect(
          find.byKey(SubscriptionSettingsScreen.manageProfilesKey),
          findsOneWidget,
          reason: '$size manage profiles control after nav',
        );
        await tester.binding.setSurfaceSize(null);
      }
    },
  );

  testWidgets(
    'subscription load error panel shows no new billing or banner controls',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in kV3Sizes) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/subscription/me' &&
              req.method == 'GET') {
            return http.Response('{"error":"upstream"}', 503);
          }
          return http.Response('Not Found', 404);
        });
        addTearDown(client.close);
        await tester.pumpWidget(_subscriptionApp(client));
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SubscriptionSettingsScreen)),
        )!;
        expect(find.text(l10n.subscriptionLoadError), findsOneWidget);
        expect(find.text(l10n.subscriptionRetry), findsOneWidget);
        expect(
          find.byKey(SubscriptionSettingsScreen.billingHistoryKey),
          findsNothing,
          reason: '$size no billing control in error state',
        );
        expect(
          find.byKey(SubscriptionSettingsScreen.manageProfilesKey),
          findsNothing,
          reason: '$size no manage profiles control in error state',
        );
        expect(
          find.byKey(SubscriptionSettingsScreen.paymentFailureBannerKey),
          findsNothing,
          reason: '$size no banner in error state',
        );
        await tester.binding.setSurfaceSize(null);
      }
    },
  );
}
