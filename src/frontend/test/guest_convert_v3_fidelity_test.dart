// UIV3-PANEL-AUTH-GUESTCONVERT fidelity test.
//
// Asserts the guest save-account reminder banner opens the reviewed v3
// convert composition from tmp/penpot-v3-html-gallery/fragments/standalone-auth-foundation.html
// section id="convert" (§2.3–2.4): a centered 420px desktop Dialog at H 1280x800
// with a right-aligned modal-foot row [Отмена][Сохранить аккаунт], and a mobile
// bottom sheet at V 390x844 with the mockup grab handle and stacked
// full-width primary/cancel. The banner carries the documented §2.3 controls
// (#1 Register CTA, #2 Dismiss "Hide banner for today"). Cancel and Dismiss
// return without a convert-guest request; the live OTP/SendOTP acceptance flow
// keeps its own gate (STG-AUTH-CONVERT-OTP) and is not exercised here.
//
// Mirrors guest_convert_modal_test.dart scaffolding but in the ru app locale,
// so every rendered string below is the ru AppLocalizations value.

import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/auth/guest_convert_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _captureBoundaryKey = ValueKey<String>(
  'guest_convert_v3_fidelity_capture',
);

// ru l10n values (existing AppLocalizations keys; never re-derived in shell).
const _title = 'Создайте аккаунт'; // guestConvertTitle
const _subtitle =
    'Укажите email и пароль, чтобы сохранить чаты и профиль.'; // guestConvertSubtitle
const _submit = 'Создать аккаунт'; // guestConvertSubmit
const _cancel = 'Отмена'; // commonCancel (localized per PR #666 lineage)
const _cta = 'Зарегистрироваться'; // guestSaveAccountReminderCta (§2.3 #1)
const _validationFailed =
    // authErrorValidationFailed (ru).
    'Укажите корректный email и пароль не короче 8 символов.';

Future<List<http.Request>> _openGuestConversionFromReminder(
  WidgetTester tester, {
  required Size viewport,
}) async {
  tester.view.physicalSize = viewport;
  tester.view.devicePixelRatio = 1;
  await tester.binding.setSurfaceSize(viewport);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(() async => tester.binding.setSurfaceSize(null));

  final requests = <http.Request>[];
  final client = MockClient((request) async {
    if (request.url.path == '/api/v1/auth/convert-guest') {
      requests.add(request);
    }
    return http.Response('not found', 404);
  });
  await tester.pumpWidget(
    RepaintBoundary(
      key: _captureBoundaryKey,
      child: ProviderScope(
        overrides: guestShellTestOverrides(client: client)
          ..add(
            authControllerProvider.overrideWith((ref) {
              final controller = authenticatedAuthController(ref);
              controller.state = controller.state.copyWith(isGuest: true);
              return controller;
            }),
          )
          ..add(connectivityWatcherProvider.overrideWith((ref) {})),
        child: const VoiceApp(locale: Locale('ru')),
      ),
    ),
  );
  await tester.pumpAndSettle();
  // Guest shell (banner state) renders without layout overflow before the
  // dialog opens.
  expect(tester.takeException(), null);
  expect(find.byKey(const Key('guest_save_account_reminder')), findsOneWidget);
  expect(find.text(_cta), findsOneWidget);
  // §2.3 #2 Dismiss renders with the CTA before any dialog opens.
  expect(
    find.byKey(const Key('guest_save_account_reminder_dismiss')),
    findsOneWidget,
  );
  // Banner lead-in (#1) leads into the v3 modal/sheet.
  await tester.tap(find.byKey(const Key('guest_save_account_reminder_cta')));
  await tester.pumpAndSettle();
  return requests;
}

int _convertRequests(List<http.Request> requests) => requests
    .where((r) => r.url.path == '/api/v1/auth/convert-guest')
    .toList()
    .length;

Future<void> _captureFidelity(WidgetTester tester, String name) async {
  final captureDirectory = Platform.environment['UIV3_CAPTURE_DIR'];
  if (captureDirectory == null || captureDirectory.isEmpty) return;

  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('Flutter did not encode the capture');
      final directory = Directory(captureDirectory);
      await directory.create(recursive: true);
      await File(
        '${directory.path}${Platform.pathSeparator}$name.png',
      ).writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (captured != true) throw StateError('Flutter capture did not complete');
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets(
    'UIV3 H 1280x800: banner tap opens centered convert Dialog with mockup '
    'modal-foot row',
    (tester) async {
      final requests = await _openGuestConversionFromReminder(
        tester,
        viewport: const Size(1280, 800),
      );
      await _captureFidelity(tester, 'guest-convert-v3-H');

      // Banner lead-in stays in the shell behind the open Dialog.
      expect(find.text(_cta), findsOneWidget);
      expect(
        find.byKey(const Key('guest_save_account_reminder_dismiss')),
        findsOneWidget,
      );

      // H variant: centered Dialog (never a BottomSheet), <= 420 wide per
      // mockup .modal width; sheet handle is V-only.
      expect(find.byType(Dialog), findsOneWidget);
      expect(find.byType(BottomSheet), findsNothing);
      expect(find.byKey(GuestConvertSheet.sheetHandleKey), findsNothing);

      final dialogSurface = find
          .descendant(of: find.byType(Dialog), matching: find.byType(Material))
          .first;
      final dialogRect = tester.getRect(dialogSurface);
      expect(dialogRect.center.dx, closeTo(640, 1));
      expect(dialogRect.center.dy, closeTo(400, 1));
      expect(dialogRect.width, lessThanOrEqualTo(420));

      // Modal controls (localized, existing keys).
      expect(find.text(_title), findsOneWidget);
      expect(find.text(_subtitle), findsOneWidget);
      expect(find.text(_submit), findsOneWidget);
      expect(find.text(_cancel), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.modalKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.emailFieldKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.passwordFieldKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.convertButtonKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.cancelButtonKey), findsOneWidget);

      // v3 mockup modal-foot: one right-aligned row, cancel to the left of
      // the primary convert CTA (§2.4 #4 then #3, gap 8).
      final submitRect = tester.getRect(
        find.byKey(GuestConvertSheet.convertButtonKey),
      );
      final cancelRect = tester.getRect(
        find.byKey(GuestConvertSheet.cancelButtonKey),
      );
      expect(cancelRect.center.dy, closeTo(submitRect.center.dy, 12));
      expect(cancelRect.center.dx, lessThan(submitRect.center.dx));
      expect(submitRect.right, closeTo(dialogRect.right - 24, 8));

      // Cancel (#4) closes the modal without a convert-guest request.
      await tester.tap(find.byKey(GuestConvertSheet.cancelButtonKey));
      await tester.pumpAndSettle();
      expect(find.byKey(GuestConvertSheet.modalKey), findsNothing);
      expect(
        find.byKey(const Key('guest_save_account_reminder')),
        findsOneWidget,
      );
      expect(_convertRequests(requests), 0);

      // §2.3 #2 Dismiss hides the banner without any conversion.
      expect(
        find.byKey(const Key('guest_save_account_reminder_dismiss')),
        findsOneWidget,
      );
      await tester.tap(
        find.byKey(const Key('guest_save_account_reminder_dismiss')),
      );
      await tester.pumpAndSettle();
      expect(find.text(_cta), findsNothing);
      expect(
        find.byKey(const Key('guest_save_account_reminder_dismiss')),
        findsNothing,
      );
      expect(_convertRequests(requests), 0);

      expect(tester.takeException(), null);
    },
    variant: const TargetPlatformVariant({TargetPlatform.windows}),
  );

  testWidgets(
    'UIV3 V 390x844: banner tap opens mobile convert bottom sheet with handle '
    'and stacked full-width buttons',
    (tester) async {
      final requests = await _openGuestConversionFromReminder(
        tester,
        viewport: const Size(390, 844),
      );
      await _captureFidelity(tester, 'guest-convert-v3-V');

      // V variant: bottom sheet anchored to the viewport bottom (mockup
      // .sheet left/right/bottom 0), never a Dialog.
      expect(find.byType(BottomSheet), findsOneWidget);
      expect(find.byType(Dialog), findsNothing);
      expect(tester.takeException(), null);
      final sheetRect = tester.getRect(find.byType(BottomSheet));
      final submitRect = tester.getRect(
        find.byKey(GuestConvertSheet.convertButtonKey),
      );
      final cancelRect = tester.getRect(
        find.byKey(GuestConvertSheet.cancelButtonKey),
      );
      expect(sheetRect.width, closeTo(390, 1));
      expect(sheetRect.bottom, closeTo(844, 1));

      // Mockup sheet chrome: grab handle above the heading (V-only).
      expect(find.byKey(GuestConvertSheet.sheetHandleKey), findsOneWidget);

      // Banner entry controls persist in the shell.
      expect(find.text(_cta), findsOneWidget);
      expect(
        find.byKey(const Key('guest_save_account_reminder_dismiss')),
        findsOneWidget,
      );

      expect(find.text(_title), findsOneWidget);
      expect(find.text(_subtitle), findsOneWidget);
      expect(find.text(_submit), findsOneWidget);
      expect(find.text(_cancel), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.modalKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.emailFieldKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.passwordFieldKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.convertButtonKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.cancelButtonKey), findsOneWidget);

      // v3 mockup V footer: primary above, ghost cancel below, both full
      // width (btn.full / btn.ghost full => content column 350 wide).
      expect(cancelRect.center.dy, greaterThan(submitRect.center.dy));
      expect(submitRect.width, greaterThan(300));
      expect(cancelRect.width, greaterThan(300));
      expect(submitRect.center.dx, closeTo(195, 8));
      expect(cancelRect.center.dx, closeTo(195, 8));

      // Cancel closes the sheet without conversion.
      await tester.tap(find.byKey(GuestConvertSheet.cancelButtonKey));
      await tester.pumpAndSettle();
      expect(find.byKey(GuestConvertSheet.modalKey), findsNothing);
      expect(_convertRequests(requests), 0);

      // Dismiss returns without conversion.
      await tester.tap(
        find.byKey(const Key('guest_save_account_reminder_dismiss')),
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const Key('guest_save_account_reminder_dismiss')),
        findsNothing,
      );
      expect(find.text(_cta), findsNothing);
      expect(_convertRequests(requests), 0);

      expect(tester.takeException(), null);
    },
    variant: const TargetPlatformVariant({TargetPlatform.windows}),
  );

  testWidgets(
    'UIV3 ru error/retry: convert validation failure stays localized with '
    'retry and cancel',
    (tester) async {
      var attempts = 0;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            discoverHintStorageProvider.overrideWithValue(
              testDiscoverHintStorage,
            ),
            authControllerProvider.overrideWith((ref) {
              final controller = authenticatedAuthController(ref);
              controller.state = controller.state.copyWith(isGuest: true);
              return controller;
            }),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            httpClientProvider.overrideWithValue(
              MockClient((request) async {
                if (request.url.path == '/api/v1/auth/convert-guest') {
                  attempts++;
                  return http.Response(
                    jsonEncode({'error': 'validation_failed'}),
                    400,
                  );
                }
                return http.Response('not found', 404);
              }),
            ),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('ru'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: GuestConvertSheet()),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text(_title), findsOneWidget);
      expect(find.text(_cancel), findsOneWidget);

      await tester.enterText(
        find.byKey(GuestConvertSheet.emailFieldKey),
        'guest@example.com',
      );
      await tester.enterText(
        find.byKey(GuestConvertSheet.passwordFieldKey),
        'validpass',
      );
      await tester.tap(find.byKey(GuestConvertSheet.convertButtonKey));
      await tester.pumpAndSettle();

      expect(attempts, 1);
      expect(find.byKey(GuestConvertSheet.errorKey), findsOneWidget);
      expect(find.text(_validationFailed), findsOneWidget);
      expect(find.text('validation_failed'), findsNothing);

      // Retry keeps the same localized error and the form editable.
      await tester.tap(find.byKey(GuestConvertSheet.convertButtonKey));
      await tester.pumpAndSettle();
      expect(attempts, 2);
      expect(find.byKey(GuestConvertSheet.errorKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.cancelButtonKey), findsOneWidget);

      expect(tester.takeException(), null);
    },
  );
}
