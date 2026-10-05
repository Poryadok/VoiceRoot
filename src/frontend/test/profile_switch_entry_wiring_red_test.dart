import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/profile_switch_coordinator.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/profile/create_profile_sheet.dart';
import 'package:voice_frontend/ui/core/voice_bottom_sheet.dart';
import 'package:voice_frontend/ui/profile/profile_avatar_menu.dart';
import 'package:voice_frontend/ui/profile/profile_avatar_switcher.dart';
import 'package:voice_frontend/ui/profile/profile_edit_sheet.dart';
import 'package:voice_frontend/ui/shell/desktop_shell_rail.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

/// T-053 Cycle 2b RED contract.
///
/// These are the four mounted product entry paths from multi-profile.md: the
/// desktop rail menu, the mobile avatar menu, mobile swipe, and profile
/// creation. They use the real coordinator provider; only HTTP, durable
/// storage, and the Realtime boundary are external test boundaries.
void main() {
  group('mounted profile-switch entry wiring', () {
    testWidgets('desktop rail menu selection owns one coordinator transition', (
      tester,
    ) async {
      final harness = _EntryHarness();
      _disposeHarnessAfterWidget(tester, harness);
      await _pumpVoiceApp(tester, harness, const Size(1280, 800));

      expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
      await tester.tap(find.byKey(ProfileAvatarMenuButton.railKey));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Gaming').last);

      await _expectOnePausedCoordinatorTransition(tester, harness);
      await _completeCoordinatorTransition(tester, harness);
      expect(
        harness.container.read(authControllerProvider).activeProfileId,
        'profile-alt',
      );
      await _disposeMountedHarness(tester, harness);
    });

    testWidgets(
      'mobile avatar tap menu selection owns one coordinator transition',
      (tester) async {
        final harness = _EntryHarness();
        _disposeHarnessAfterWidget(tester, harness);
        await _pumpVoiceApp(tester, harness, const Size(390, 800));

        await tester.tap(find.byKey(ProfileAvatarSwitcher.switcherKey));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Gaming').last);

        await _expectOnePausedCoordinatorTransition(tester, harness);
        await _completeCoordinatorTransition(tester, harness);
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-alt',
        );
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets('mobile avatar swipe owns one coordinator transition', (
      tester,
    ) async {
      final harness = _EntryHarness();
      _disposeHarnessAfterWidget(tester, harness);
      await _pumpVoiceApp(tester, harness, const Size(390, 800));

      await tester.fling(
        find.byKey(ProfileAvatarSwitcher.switcherKey),
        const Offset(-200, 0),
        1000,
      );

      await _expectOnePausedCoordinatorTransition(tester, harness);
      await _completeCoordinatorTransition(tester, harness);
      expect(
        harness.container.read(authControllerProvider).activeProfileId,
        'profile-alt',
      );
      await _disposeMountedHarness(tester, harness);
    });

    testWidgets(
      'successful CreateProfileSheet switches through coordinator before closing',
      (tester) async {
        final harness = _EntryHarness(profiles: const [_primaryProfile]);
        _disposeHarnessAfterWidget(tester, harness);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: harness.container,
            child: MaterialApp(
              theme: voiceTestTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Scaffold(
                body: Builder(
                  builder: (context) => FilledButton(
                    key: const Key('open_create_profile_sheet'),
                    onPressed: () => showCreateProfileSheet(context),
                    child: const Text('Open create profile'),
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.tap(find.byKey(const Key('open_create_profile_sheet')));
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );
        await tester.enterText(
          find.byKey(CreateProfileSheet.usernameFieldKey),
          'creator_tag',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));

        await _expectOnePausedCoordinatorTransition(
          tester,
          harness,
          expectedProfileId: 'profile-created',
        );
        expect(harness.createRequests, 1);
        expect(harness.createBodies.single['username'], 'creator_tag');
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);

        await _completeCoordinatorTransition(tester, harness);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsNothing);
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-created',
        );
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets(
      'retry after acknowledged create and switch failure reuses profile receipt',
      (tester) async {
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          switchFailuresRemaining: 1,
          tier: 'premium',
        );
        _disposeHarnessAfterWidget(tester, harness);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: harness.container,
            child: MaterialApp(
              theme: voiceTestTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Scaffold(body: CreateProfileSheet()),
            ),
          ),
        );
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );

        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await tester.pumpAndSettle();

        expect(harness.createRequests, 1);
        expect(harness.authSwitchRequests, 1);
        expect(harness.profileCreated, isTrue);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);

        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await _expectOnePausedCoordinatorTransition(
          tester,
          harness,
          expectedProfileId: 'profile-created',
          expectedAuthSwitchRequests: 2,
        );

        expect(harness.createRequests, 1);
        await _completeCoordinatorTransition(tester, harness);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsNothing);
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets(
      'free-tier limit preserves recovery after the second profile was created',
      (tester) async {
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          switchFailuresRemaining: 1,
        );
        _disposeHarnessAfterWidget(tester, harness);
        await _pumpCreateProfileSheet(tester, harness);
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await tester.pumpAndSettle();

        expect(harness.createRequests, 1);
        expect(harness.profileCreated, isTrue);
        expect(find.byKey(CreateProfileSheet.submitKey), findsOneWidget);
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets('acknowledged create receipt is not reused by another account', (
      tester,
    ) async {
      final harness = _EntryHarness(
        profiles: const [_primaryProfile],
        switchFailuresRemaining: 1,
        tier: 'premium',
      );
      _disposeHarnessAfterWidget(tester, harness);
      await _pumpCreateProfileSheet(tester, harness);
      await tester.enterText(
        find.byKey(CreateProfileSheet.displayNameFieldKey),
        'Created profile',
      );
      await tester.tap(find.byKey(CreateProfileSheet.submitKey));
      await tester.pumpAndSettle();
      expect(harness.createRequests, 1);
      expect(harness.authSwitchRequests, 1);

      harness.container.read(authControllerProvider.notifier).state = AuthState(
        session: _sessionFor('profile-primary', accountId: 'account-2'),
      );
      await tester.tap(find.byKey(CreateProfileSheet.submitKey));
      await tester.pumpAndSettle();

      expect(harness.createRequests, 1);
      expect(harness.authSwitchRequests, 1);
      expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
      expect(
        find.text(
          'The signed-in account or active profile changed. Reopen profile creation to continue.',
        ),
        findsOneWidget,
      );
      await _disposeMountedHarness(tester, harness);
    });

    testWidgets('acknowledged create receipt is not reused by another profile', (
      tester,
    ) async {
      final harness = _EntryHarness(
        profiles: const [_primaryProfile],
        switchFailuresRemaining: 1,
        tier: 'premium',
      );
      _disposeHarnessAfterWidget(tester, harness);
      await _pumpCreateProfileSheet(tester, harness);
      await tester.enterText(
        find.byKey(CreateProfileSheet.displayNameFieldKey),
        'Created profile',
      );
      await tester.tap(find.byKey(CreateProfileSheet.submitKey));
      await tester.pumpAndSettle();
      expect(harness.createRequests, 1);
      expect(harness.authSwitchRequests, 1);

      harness.container.read(authControllerProvider.notifier).state = AuthState(
        session: _sessionFor('profile-alt'),
      );
      await tester.tap(find.byKey(CreateProfileSheet.submitKey));
      await tester.pumpAndSettle();

      expect(harness.createRequests, 1);
      expect(harness.authSwitchRequests, 1);
      expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
      expect(
        find.text(
          'The signed-in account or active profile changed. Reopen profile creation to continue.',
        ),
        findsOneWidget,
      );
      await _disposeMountedHarness(tester, harness);
    });

    testWidgets('create receipt from another account is never switched to', (
      tester,
    ) async {
      final harness = _EntryHarness(
        profiles: const [_primaryProfile],
        createResponseAccountId: 'account-foreign',
        tier: 'premium',
      );
      _disposeHarnessAfterWidget(tester, harness);
      await _pumpCreateProfileSheet(tester, harness);
      await tester.enterText(
        find.byKey(CreateProfileSheet.displayNameFieldKey),
        'Created profile',
      );
      await tester.tap(find.byKey(CreateProfileSheet.submitKey));
      await tester.pumpAndSettle();

      expect(harness.createRequests, 1);
      expect(harness.authSwitchRequests, 0);
      expect(harness.avatarRequests, 0);
      expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
      await tester.tap(find.byKey(CreateProfileSheet.submitKey));
      await tester.pumpAndSettle();
      expect(harness.createRequests, 1);
      expect(harness.authSwitchRequests, 0);
      await _disposeMountedHarness(tester, harness);
    });

    testWidgets(
      'acknowledged same-account receipt with empty ID is never switched or retried',
      (tester) async {
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          createResponseProfileId: '',
          tier: 'premium',
        );
        _disposeHarnessAfterWidget(tester, harness);
        await _pumpCreateProfileSheet(tester, harness, avatar: _testAvatar());
        await tester.tap(find.byKey(CreateProfileSheet.avatarButtonKey));
        await tester.pump();
        final profileListRequests = harness.profileListRequests;
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );

        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await tester.pumpAndSettle();

        expect(harness.createRequests, 1);
        expect(harness.authSwitchRequests, 0);
        expect(harness.avatarPresignRequests, 0);
        expect(harness.avatarPutRequests, 0);
        expect(harness.profileUpdateRequests, 0);
        expect(harness.profileListRequests, profileListRequests);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
        expect(
          find.text(
            'Profile created, but setup is incomplete. Try again to finish.',
          ),
          findsOneWidget,
        );

        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await tester.pumpAndSettle();

        expect(harness.createRequests, 1);
        expect(harness.authSwitchRequests, 0);
        expect(harness.avatarPresignRequests, 0);
        expect(harness.avatarPutRequests, 0);
        expect(harness.profileUpdateRequests, 0);
        expect(harness.profileListRequests, profileListRequests);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets(
      'delayed create response cannot continue in a changed account',
      (tester) async {
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          pauseCreate: true,
          tier: 'premium',
        );
        _disposeHarnessAfterWidget(tester, harness);
        await _pumpCreateProfileSheet(tester, harness);
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await harness.createRequestEntered!.future;

        harness.container
            .read(authControllerProvider.notifier)
            .state = AuthState(
          session: _sessionFor('profile-primary', accountId: 'account-2'),
        );
        harness.releaseCreateRequest!.complete();
        await tester.pumpAndSettle();

        expect(harness.createRequests, 1);
        expect(harness.authSwitchRequests, 0);
        expect(harness.avatarRequests, 0);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets(
      'receipt recovery supports keyboard focus, semantics, and H/V capture',
      (tester) async {
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = const Size(390, 844);
        final captureTheme = await _captureProfileCreateTheme(tester);
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          switchFailuresRemaining: 1,
          tier: 'premium',
          useRealTheme: true,
          tokenCatalog: _profileCreateCaptureCatalog,
        );
        _disposeHarnessAfterWidget(tester, harness);
        final triggerFocus = FocusNode();
        addTearDown(triggerFocus.dispose);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: harness.container,
            child: MaterialApp(
              theme: captureTheme ?? voiceTestTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Scaffold(
                body: Builder(
                  builder: (context) => FilledButton(
                    key: const Key('open_create_profile_sheet'),
                    focusNode: triggerFocus,
                    onPressed: () => showVoiceBottomSheet<bool>(
                      context: context,
                      child: RepaintBoundary(
                        key: _profileCreateCaptureBoundaryKey,
                        child: const CreateProfileSheet(),
                      ),
                    ),
                    child: const Text('Open create profile'),
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        triggerFocus.requestFocus();
        await tester.pump();
        expect(triggerFocus.hasPrimaryFocus, isTrue);
        await tester.sendKeyEvent(LogicalKeyboardKey.enter);
        await tester.pumpAndSettle();
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
        expect(tester.view.physicalSize, const Size(390, 844));
        expect(tester.view.devicePixelRatio, 1);

        final semantics = tester.ensureSemantics();
        expect(find.bySemanticsLabel('Profile tag'), findsOneWidget);
        await tester.tap(find.byKey(CreateProfileSheet.displayNameFieldKey));
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        expect(
          tester
              .widget<EditableText>(
                find.descendant(
                  of: find.byKey(CreateProfileSheet.usernameFieldKey),
                  matching: find.byType(EditableText),
                ),
              )
              .focusNode
              .hasFocus,
          isTrue,
        );

        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );
        await tester.enterText(
          find.byKey(CreateProfileSheet.usernameFieldKey),
          'created_tag',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await tester.pumpAndSettle();
        expect(harness.createRequests, 1);
        expect(
          tester
              .widget<TextField>(
                find.byKey(CreateProfileSheet.usernameFieldKey),
              )
              .enabled,
          isFalse,
        );
        await _captureProfileCreate(tester, 'profile-create-v.png');

        tester.view.physicalSize = const Size(844, 390);
        await tester.pumpAndSettle();
        await tester.ensureVisible(find.byKey(CreateProfileSheet.submitKey));
        expect(tester.view.physicalSize, const Size(844, 390));
        expect(tester.view.devicePixelRatio, 1);
        await _captureProfileCreate(tester, 'profile-create-h.png');

        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(find.byKey(CreateProfileSheet.sheetKey), findsNothing);
        expect(triggerFocus.hasPrimaryFocus, isTrue);
        semantics.dispose();
        await _disposeMountedHarness(tester, harness);
      },
    );

    for (final stage in ['presign', 'put', 'update']) {
      testWidgets(
        'retry after avatar $stage failure reuses the acknowledged profile',
        (tester) async {
          final harness = _EntryHarness(
            profiles: const [_primaryProfile],
            failAvatarStage: stage,
            tier: 'premium',
          );
          _disposeHarnessAfterWidget(tester, harness);
          await _pumpCreateProfileSheet(tester, harness, avatar: _testAvatar());
          await tester.tap(find.byKey(CreateProfileSheet.avatarButtonKey));
          await tester.pump();
          await tester.enterText(
            find.byKey(CreateProfileSheet.displayNameFieldKey),
            'Created profile',
          );
          await tester.tap(find.byKey(CreateProfileSheet.submitKey));
          await _expectOnePausedCoordinatorTransition(
            tester,
            harness,
            expectedProfileId: 'profile-created',
          );
          await _completeCoordinatorTransition(tester, harness);

          expect(harness.createRequests, 1);
          expect(harness.avatarPresignRequests, 1);
          expect(harness.avatarPutRequests, stage == 'presign' ? 0 : 1);
          expect(harness.profileUpdateRequests, stage == 'update' ? 1 : 0);
          expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);

          await tester.tap(find.byKey(CreateProfileSheet.submitKey));
          await tester.pumpAndSettle();

          expect(harness.createRequests, 1);
          expect(harness.authSwitchRequests, 1);
          expect(harness.avatarPresignRequests, stage == 'presign' ? 2 : 2);
          expect(harness.avatarPutRequests, stage == 'presign' ? 1 : 2);
          expect(harness.profileUpdateRequests, stage == 'update' ? 2 : 1);
          expect(find.byKey(CreateProfileSheet.sheetKey), findsNothing);
          await _disposeMountedHarness(tester, harness);
        },
      );
    }

    for (final stage in ['presign', 'put', 'update']) {
      testWidgets(
        'avatar $stage completion after account change has no later side effects',
        (tester) async {
          final harness = _EntryHarness(
            profiles: const [_primaryProfile],
            pauseAvatarStage: stage,
            tier: 'premium',
          );
          _disposeHarnessAfterWidget(tester, harness);
          await _pumpCreateProfileSheet(tester, harness, avatar: _testAvatar());
          await tester.tap(find.byKey(CreateProfileSheet.avatarButtonKey));
          await tester.pump();
          await tester.enterText(
            find.byKey(CreateProfileSheet.displayNameFieldKey),
            'Created profile',
          );
          await tester.tap(find.byKey(CreateProfileSheet.submitKey));
          await _expectOnePausedCoordinatorTransition(
            tester,
            harness,
            expectedProfileId: 'profile-created',
          );
          harness.realtime.complete();
          await tester.pump();
          await harness.avatarStageEntered!.future;

          harness.container
              .read(authControllerProvider.notifier)
              .state = AuthState(
            session: _sessionFor('profile-created', accountId: 'account-2'),
          );
          harness.releaseAvatarStage!.complete();
          await tester.pumpAndSettle();

          expect(harness.createRequests, 1);
          expect(harness.avatarPresignRequests, 1);
          expect(harness.avatarPutRequests, stage == 'presign' ? 0 : 1);
          expect(harness.profileUpdateRequests, stage == 'update' ? 1 : 0);
          expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
          await _disposeMountedHarness(tester, harness);
        },
      );
    }

    testWidgets(
      'avatar presign completion after same-account profile change has no later side effects',
      (tester) async {
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          pauseAvatarStage: 'presign',
          tier: 'premium',
        );
        _disposeHarnessAfterWidget(tester, harness);
        await _pumpCreateProfileSheet(tester, harness, avatar: _testAvatar());
        await tester.tap(find.byKey(CreateProfileSheet.avatarButtonKey));
        await tester.pump();
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await _expectOnePausedCoordinatorTransition(
          tester,
          harness,
          expectedProfileId: 'profile-created',
        );
        harness.realtime.complete();
        await tester.pump();
        await harness.avatarStageEntered!.future;
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-created',
        );

        harness.container.read(authControllerProvider.notifier).state =
            AuthState(session: _sessionFor('profile-alt'));
        harness.releaseAvatarStage!.complete();
        await tester.pumpAndSettle();

        expect(harness.createRequests, 1);
        expect(harness.authSwitchRequests, 1);
        expect(harness.avatarPresignRequests, 1);
        expect(harness.avatarPutRequests, 0);
        expect(harness.profileUpdateRequests, 0);
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-alt',
        );
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets(
      'creating a second profile keeps the routed shell mounted while switching',
      (tester) async {
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          useRealTheme: true,
        );
        _disposeHarnessAfterWidget(tester, harness);
        await _pumpVoiceApp(tester, harness, const Size(1280, 800));

        expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
        await tester.tap(find.byKey(ProfileAvatarMenuButton.railKey));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Create profile').last);
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Created profile',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await _expectOnePausedCoordinatorTransition(
          tester,
          harness,
          expectedProfileId: 'profile-created',
        );

        expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
        await _completeCoordinatorTransition(tester, harness);
        expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
        expect(find.byKey(CreateProfileSheet.sheetKey), findsNothing);
        expect(
          tester
              .widget<ProfileAvatarMenuButton>(
                find.byKey(ProfileAvatarMenuButton.railKey),
              )
              .profile
              ?.id,
          'profile-created',
        );
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-created',
        );
        await _disposeMountedHarness(tester, harness);
      },
    );

    testWidgets(
      'failed CreateProfileSheet keeps its picked avatar and never switches',
      (tester) async {
        final harness = _EntryHarness(
          profiles: const [_primaryProfile],
          rejectCreate: true,
        );
        _disposeHarnessAfterWidget(tester, harness);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: harness.container,
            child: MaterialApp(
              theme: voiceTestTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Scaffold(
                body: CreateProfileSheet(
                  avatarPicker: () async => ProfileAvatarFile(
                    bytes: base64Decode(
                      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL8+wAAAABJRU5ErkJggg==',
                    ),
                    contentType: 'image/png',
                    name: 'avatar.png',
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(CreateProfileSheet.avatarButtonKey));
        await tester.pump();
        await tester.enterText(
          find.byKey(CreateProfileSheet.displayNameFieldKey),
          'Rejected profile',
        );
        await tester.tap(find.byKey(CreateProfileSheet.submitKey));
        await tester.pumpAndSettle();

        expect(harness.createRequests, 1);
        expect(harness.authSwitchRequests, 0);
        expect(harness.realtime.handoffs, isEmpty);
        expect(harness.avatarRequests, 0);
        expect(
          harness.container.read(profileSwitchInProgressProvider),
          isFalse,
        );
        expect(find.byKey(CreateProfileSheet.sheetKey), findsOneWidget);
        expect(find.text('Could not save profile: Try again'), findsOneWidget);
        expect(
          tester
              .widget<CircleAvatar>(
                find.descendant(
                  of: find.byKey(CreateProfileSheet.sheetKey),
                  matching: find.byType(CircleAvatar),
                ),
              )
              .backgroundImage,
          isA<MemoryImage>(),
        );
        expect(
          harness.container.read(authControllerProvider).activeProfileId,
          'profile-primary',
        );
        await _disposeMountedHarness(tester, harness);
      },
    );
  });
}

Future<void> _pumpVoiceApp(
  WidgetTester tester,
  _EntryHarness harness,
  Size surfaceSize,
) async {
  final messenger =
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  const voipChannel = MethodChannel('voice/voip');
  messenger.setMockMethodCallHandler(voipChannel, (_) async => null);
  await tester.binding.setSurfaceSize(surfaceSize);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: harness.container,
      child: const VoiceApp(locale: Locale('en')),
    ),
  );
  await tester.pumpAndSettle();
}

void _disposeHarnessAfterWidget(WidgetTester tester, _EntryHarness harness) {
  addTearDown(harness.dispose);
}

Future<void> _disposeMountedHarness(
  WidgetTester tester,
  _EntryHarness harness,
) async {
  await tester.pumpWidget(const SizedBox.shrink());
  harness.dispose();
  await tester.pump();
}

Future<void> _pumpCreateProfileSheet(
  WidgetTester tester,
  _EntryHarness harness, {
  ProfileAvatarFile? avatar,
}) async {
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: harness.container,
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: CreateProfileSheet(
            avatarPicker: avatar == null ? null : () async => avatar,
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

const _profileCreateCaptureEnvironmentKey = 'VOICE_PROFILE_CREATE_CAPTURE_DIR';
const _profileCreateCaptureBoundaryKey = Key('profile_create_capture_boundary');
var _profileCreateCaptureFontsLoaded = false;
VoiceTokenCatalog? _profileCreateCaptureCatalog;

Future<ThemeData?> _captureProfileCreateTheme(WidgetTester tester) async {
  final directory = Platform.environment[_profileCreateCaptureEnvironmentKey];
  if (directory == null || directory.isEmpty) return null;
  if (!_profileCreateCaptureFontsLoaded) {
    final loaded = await tester.runAsync(() async {
      final noto = FontLoader(VoiceTheme.fontFamily)
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
      await noto.load();
      final flutterRoot = Platform.environment['FLUTTER_ROOT'];
      if (flutterRoot == null || flutterRoot.isEmpty) {
        throw StateError('FLUTTER_ROOT is required to load Material Icons');
      }
      final iconFile = File(
        [
          flutterRoot,
          'bin',
          'cache',
          'artifacts',
          'material_fonts',
          'MaterialIcons-Regular.otf',
        ].join(Platform.pathSeparator),
      );
      final bytes = await iconFile.readAsBytes();
      await (FontLoader('MaterialIcons')..addFont(
            Future<ByteData>.value(
              ByteData.sublistView(Uint8List.fromList(bytes)),
            ),
          ))
          .load();
      return true;
    });
    if (loaded != true) {
      throw StateError('Production font loading failed');
    }
    _profileCreateCaptureFontsLoaded = true;
  }
  final catalog = await tester.runAsync(VoiceTokenCatalog.load);
  if (catalog == null) {
    throw StateError('Production design tokens did not load');
  }
  _profileCreateCaptureCatalog = catalog;
  return VoiceTheme.build(
    catalog: catalog,
    mode: VoiceThemeMode.dark,
    profileAccent: catalog.profileAccentAt(0),
  );
}

Future<void> _captureProfileCreate(WidgetTester tester, String filename) async {
  final directory = Platform.environment[_profileCreateCaptureEnvironmentKey];
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_profileCreateCaptureBoundaryKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(
      pixelRatio: tester.view.devicePixelRatio,
    );
    try {
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('PNG encoding returned null');
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (captured != true) throw StateError('Profile create capture failed');
}

ProfileAvatarFile _testAvatar() => ProfileAvatarFile(
  bytes: base64Decode(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL8+wAAAABJRU5ErkJggg==',
  ),
  contentType: 'image/png',
  name: 'avatar.png',
);

Future<void> _expectOnePausedCoordinatorTransition(
  WidgetTester tester,
  _EntryHarness harness, {
  String expectedProfileId = 'profile-alt',
  int expectedAuthSwitchRequests = 1,
}) async {
  await tester.pump();

  expect(harness.authSwitchRequests, expectedAuthSwitchRequests);
  expect(harness.realtime.handoffs, hasLength(1));
  expect(
    harness.realtime.handoffs.single.nextSession.activeProfileId,
    expectedProfileId,
  );
  expect(harness.container.read(profileSwitchInProgressProvider), isTrue);
}

Future<void> _completeCoordinatorTransition(
  WidgetTester tester,
  _EntryHarness harness,
) async {
  harness.realtime.complete();
  await tester.pumpAndSettle();
  expect(harness.container.read(profileSwitchInProgressProvider), isFalse);
}

const _primaryProfile = VoiceProfile(
  id: 'profile-primary',
  accountId: 'account-1',
  username: 'primary',
  discriminator: '0001',
  displayName: 'Primary',
  isPrimary: true,
);

const _altProfile = VoiceProfile(
  id: 'profile-alt',
  accountId: 'account-1',
  username: 'gaming',
  discriminator: '0002',
  displayName: 'Gaming',
);

const _createdProfile = VoiceProfile(
  id: 'profile-created',
  accountId: 'account-1',
  username: 'created',
  discriminator: '0003',
  displayName: 'Created profile',
);

const _primarySession = AuthSession(
  accessToken: 'access-primary',
  refreshToken: 'refresh-primary',
  expiresInSeconds: 900,
  accountId: 'account-1',
  activeProfileId: 'profile-primary',
);

class _EntryHarness {
  _EntryHarness({
    this.profiles = const [_primaryProfile, _altProfile],
    this.rejectCreate = false,
    this.switchFailuresRemaining = 0,
    this.failAvatarStage,
    this.pauseAvatarStage,
    this.pauseCreate = false,
    this.tier = 'free',
    this.useRealTheme = false,
    this.tokenCatalog,
    this.createResponseAccountId,
    this.createResponseProfileId,
  }) {
    final client = MockClient(_respond);
    storage = _MemoryAuthStorage(_primarySession);
    realtime = _PausedProfileSwitchRealtimeBoundary();
    container = ProviderContainer(
      overrides: [
        if (useRealTheme || tokenCatalog != null)
          voiceTokenCatalogProvider.overrideWith(
            (ref) async => tokenCatalog ?? testVoiceTokenCatalog,
          )
        else
          ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(storage),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        realtimeAutoConnectProvider.overrideWithValue(false),
        onboardingControllerProvider.overrideWith(
          _CompletedOnboardingController.new,
        ),
        authControllerProvider.overrideWith((ref) {
          final controller = AuthController(
            authClient: ref.watch(voiceAuthClientProvider),
            storage: ref.watch(authSessionStorageProvider),
            guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
          );
          controller.state = const AuthState(session: _primarySession);
          return controller;
        }),
        profileSwitchRealtimeBoundaryProvider.overrideWithValue(realtime),
        subscriptionTierProvider.overrideWith((_) => tier),
      ],
    );
  }

  final List<VoiceProfile> profiles;
  final bool rejectCreate;
  int switchFailuresRemaining;
  final String tier;
  final bool useRealTheme;
  final VoiceTokenCatalog? tokenCatalog;
  final String? createResponseAccountId;
  final String? createResponseProfileId;
  late final ProviderContainer container;
  late final _MemoryAuthStorage storage;
  late final _PausedProfileSwitchRealtimeBoundary realtime;
  var authSwitchRequests = 0;
  var createRequests = 0;
  var profileListRequests = 0;
  final List<Map<String, dynamic>> createBodies = [];
  final bool pauseCreate;
  Completer<void>? createRequestEntered;
  Completer<void>? releaseCreateRequest;
  var avatarRequests = 0;
  var avatarPresignRequests = 0;
  var avatarPutRequests = 0;
  var profileUpdateRequests = 0;
  String? failAvatarStage;
  final String? pauseAvatarStage;
  Completer<void>? avatarStageEntered;
  Completer<void>? releaseAvatarStage;
  var avatarStageFailureRemaining = 1;
  var profileCreated = false;
  var _disposed = false;

  Future<http.Response> _respond(http.Request request) async {
    if (request.url.path == '/health') return http.Response('OK', 200);
    if (request.url.path == '/api/v1/auth/switch-profile') {
      authSwitchRequests++;
      if (switchFailuresRemaining > 0) {
        switchFailuresRemaining--;
        return http.Response('switch unavailable', 503);
      }
      final profileId =
          (jsonDecode(request.body) as Map<String, dynamic>)['profile_id']
              as String?;
      final session = switch (profileId) {
        'profile-alt' => _sessionFor('profile-alt'),
        'profile-created' => _sessionFor('profile-created'),
        _ => null,
      };
      if (session == null) return http.Response('unknown profile', 400);
      return http.Response(jsonEncode(session.toJson()), 200);
    }
    if (request.url.path == '/api/v1/users/profiles' &&
        request.method == 'POST') {
      createRequests++;
      createBodies.add(jsonDecode(request.body) as Map<String, dynamic>);
      if (pauseCreate) {
        createRequestEntered ??= Completer<void>();
        releaseCreateRequest ??= Completer<void>();
        createRequestEntered!.complete();
        await releaseCreateRequest!.future;
      }
      if (rejectCreate) {
        return http.Response(
          jsonEncode({'error': 'create_denied', 'message': 'create denied'}),
          403,
        );
      }
      profileCreated = true;
      final responseProfile = _profileJson(_createdProfile);
      if (createResponseProfileId != null) {
        responseProfile['id'] = createResponseProfileId;
      }
      if (createResponseAccountId != null) {
        responseProfile['account_id'] = createResponseAccountId;
      }
      return http.Response(jsonEncode({'profile': responseProfile}), 200);
    }
    if (request.url.path == '/api/v1/users/profiles') {
      profileListRequests++;
      return http.Response(
        jsonEncode({
          'profile_list': {
            'profiles': [
              ...profiles,
              if (profileCreated) _createdProfile,
            ].map(_profileJson).toList(),
          },
        }),
        200,
      );
    }
    if (request.url.path.startsWith('/api/v1/users/profiles/')) {
      final id = request.url.path.split('/').last;
      final profile = [
        ...profiles,
        _createdProfile,
      ].where((item) => item.id == id);
      if (profile.isEmpty) return http.Response('not found', 404);
      return http.Response(
        jsonEncode({'profile': _profileJson(profile.single)}),
        200,
      );
    }
    if (request.url.path == '/api/v1/users/me/avatar/presigned-upload' &&
        request.method == 'POST') {
      avatarRequests++;
      avatarPresignRequests++;
      await _pauseAvatarStage('presign');
      if (_failAvatarStage('presign')) {
        return http.Response(
          jsonEncode({'error': 'presign_failed', 'message': 'stage failed'}),
          503,
        );
      }
      return http.Response(
        jsonEncode({
          'http_method': 'PUT',
          'upload_url': 'http://upload.test/avatar',
          'required_headers': {'Content-Type': 'image/png'},
          'max_bytes': '5242880',
          'public_url': 'https://cdn.test/avatar.png',
          'object_key': 'avatars/profile-created/avatar.png',
        }),
        200,
      );
    }
    if (request.url.path == '/api/v1/users/me' && request.method == 'PATCH') {
      profileUpdateRequests++;
      avatarRequests++;
      await _pauseAvatarStage('update');
      if (_failAvatarStage('update')) {
        return http.Response(
          jsonEncode({'error': 'update_failed', 'message': 'stage failed'}),
          503,
        );
      }
      return http.Response(
        jsonEncode({'profile': _profileJson(_createdProfile)}),
        200,
      );
    }
    if (request.url.host == 'upload.test' && request.method == 'PUT') {
      avatarRequests++;
      avatarPutRequests++;
      await _pauseAvatarStage('put');
      if (_failAvatarStage('put')) {
        return http.Response('upload failed', 503);
      }
      return http.Response('', 200);
    }
    if (request.url.path.contains('avatar')) avatarRequests++;
    return http.Response('not found', 404);
  }

  bool _failAvatarStage(String stage) {
    if (failAvatarStage != stage || avatarStageFailureRemaining == 0) {
      return false;
    }
    avatarStageFailureRemaining--;
    return true;
  }

  Future<void> _pauseAvatarStage(String stage) async {
    if (pauseAvatarStage != stage) return;
    avatarStageEntered ??= Completer<void>();
    releaseAvatarStage ??= Completer<void>();
    if (!avatarStageEntered!.isCompleted) avatarStageEntered!.complete();
    await releaseAvatarStage!.future;
  }

  void dispose() {
    if (_disposed) return;
    _disposed = true;
    container.dispose();
  }
}

class _CompletedOnboardingController extends OnboardingController {
  @override
  OnboardingUiState build() => const OnboardingUiState(completed: true);
}

AuthSession _sessionFor(String profileId, {String accountId = 'account-1'}) =>
    AuthSession(
      accessToken: 'access-$profileId',
      refreshToken: 'refresh-$profileId',
      expiresInSeconds: 900,
      accountId: accountId,
      activeProfileId: profileId,
    );

Map<String, Object?> _profileJson(VoiceProfile profile) => {
  'id': profile.id,
  'account_id': profile.accountId,
  'username': profile.username,
  'discriminator': profile.discriminator,
  'display_name': profile.displayName,
  'is_primary': profile.isPrimary,
  'verification_type': profile.verificationType,
};

class _MemoryAuthStorage implements AuthSessionStorage {
  _MemoryAuthStorage(this._session);

  AuthSession? _session;

  @override
  Future<void> clear() async => _session = null;

  @override
  Future<AuthSession?> read() async => _session;

  @override
  Future<void> write(AuthSession session) async => _session = session;
}

class _PausedProfileSwitchRealtimeBoundary
    implements ProfileSwitchRealtimeBoundary {
  @override
  final Set<String> activeSubscriptions = {};

  final List<ProfileSwitchHandoff> handoffs = [];
  final Completer<void> _completion = Completer<void>();

  @override
  Future<void> retireAndReconnect(ProfileSwitchHandoff handoff) {
    handoffs.add(handoff);
    return _completion.future;
  }

  void complete() {
    if (!_completion.isCompleted) _completion.complete();
  }
}
