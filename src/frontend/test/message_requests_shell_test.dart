import 'dart:async';
import 'dart:io';
import 'dart:ui' show ImageByteFormat, Tristate;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/routing/deep_link_listener.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/state/mobile_opened_chat_strip.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';
import 'package:voice_frontend/ui/shell/chat_rail_sections.dart';
import 'package:voice_frontend/ui/shell/desktop_shell_rail.dart';
import 'package:voice_frontend/ui/shell/manage_folders_sheet.dart';
import 'package:voice_frontend/ui/shell/message_requests_folder.dart';
import 'package:voice_frontend/ui/shell/mobile_shell_drawer.dart';
import 'package:voice_frontend/ui/shell/navigation_panel.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_coach_mark.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_anchor_keys.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/core/voice_avatar.dart';
import 'package:voice_frontend/ui/social/social_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';

const _captureBoundaryKey = Key('message_requests_shell_capture');
const _captureEnv = 'VOICE_CONTACT_REQUESTS_CAPTURE_DIR';
const _voipChannel = MethodChannel('voice/voip');
var _captureFontsLoaded = false;

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('desktop Friends rail opens the real social panel', (
    tester,
  ) async {
    _mockVoip(tester);
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final container = await _pumpVoiceShell(
      tester,
      const Size(1280, 800),
      null,
    );

    expect(container.read(navigationSectionProvider), NavigationSection.chats);
    expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
    expect(find.byKey(const Key('shell_rail_friends')), findsOneWidget);
    expect(find.byType(SocialPanel), findsNothing);

    await tester.tap(find.byKey(const Key('shell_rail_friends')));
    await tester.pumpAndSettle();

    expect(container.read(navigationSectionProvider), NavigationSection.social);
    expect(find.byKey(SocialPanel.panelKey), findsOneWidget);
    expect(find.byKey(NavigationPanel.panelKey), findsOneWidget);
    expect(find.byType(ChatListBody), findsNothing);
  });

  testWidgets(
    'fresh-user spaces coach mark stays visible without blocking Friends navigation',
    (tester) async {
      _mockVoip(tester);
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });
      await _pumpVoiceShell(
        tester,
        const Size(1280, 800),
        null,
        onboardingAtSpaces: true,
      );

      final anchorRect = tester.getRect(find.byKey(OnboardingAnchorKeys.spaces));
      final title = find.text(AppLocalizationsEn().onboardingSpacesTitle);
      final titleRect = tester.getRect(title);
      final cardRect = tester.getRect(find.byKey(OnboardingCoachMark.cardKey));
      final safeTop = tester.view.padding.top + 8;
      final safeBottom =
          tester.view.physicalSize.height - tester.view.padding.bottom - 8;
      const safeLeft = 8.0;
      final safeRight = tester.view.physicalSize.width - safeLeft;
      expect(anchorRect.height, lessThan(120));
      expect(cardRect.left, greaterThanOrEqualTo(safeLeft));
      expect(cardRect.right, lessThanOrEqualTo(safeRight));
      expect(cardRect.top, greaterThanOrEqualTo(safeTop));
      expect(cardRect.bottom, lessThanOrEqualTo(safeBottom));
      expect(titleRect.top, greaterThanOrEqualTo(safeTop));
      expect(titleRect.bottom, lessThanOrEqualTo(safeBottom));
      final skip = find.widgetWithText(
        TextButton,
        AppLocalizationsEn().onboardingSkip,
      );
      final continueButton = find.widgetWithText(
        FilledButton,
        AppLocalizationsEn().onboardingLater,
      );
      final secondaryButton = find.widgetWithText(
        TextButton,
        AppLocalizationsEn().onboardingSpacesFind,
      );
      expect(tester.getTopLeft(skip).dx, greaterThanOrEqualTo(safeLeft));
      expect(tester.getBottomRight(skip).dx, lessThanOrEqualTo(safeRight));
      expect(tester.getTopLeft(skip).dy, greaterThanOrEqualTo(safeTop));
      expect(tester.getBottomRight(skip).dy, lessThanOrEqualTo(safeBottom));
      expect(
        tester.getTopLeft(continueButton).dx,
        greaterThanOrEqualTo(safeLeft),
      );
      expect(
        tester.getBottomRight(continueButton).dx,
        lessThanOrEqualTo(safeRight),
      );
      expect(tester.getTopLeft(continueButton).dy, greaterThanOrEqualTo(safeTop));
      expect(tester.getBottomRight(continueButton).dy, lessThanOrEqualTo(safeBottom));
      expect(
        tester.getTopLeft(secondaryButton).dx,
        greaterThanOrEqualTo(safeLeft),
      );
      expect(
        tester.getBottomRight(secondaryButton).dx,
        lessThanOrEqualTo(safeRight),
      );
      expect(tester.getTopLeft(secondaryButton).dy, greaterThanOrEqualTo(safeTop));
      expect(tester.getBottomRight(secondaryButton).dy, lessThanOrEqualTo(safeBottom));
      expect(
        skip,
        findsOneWidget,
      );

      await tester.tap(find.byKey(const Key('shell_rail_friends')));
      await tester.pumpAndSettle();

      expect(find.byType(SocialPanel), findsOneWidget);
      expect(find.byKey(SocialPanel.panelKey), findsOneWidget);
      expect(find.text(AppLocalizationsEn().onboardingSpacesTitle), findsOneWidget);
      expect(
        find.widgetWithText(TextButton, AppLocalizationsEn().onboardingSkip),
        findsOneWidget,
      );

      await tester.tap(
        find.widgetWithText(TextButton, AppLocalizationsEn().onboardingSkip),
      );
      await tester.pumpAndSettle();
      expect(find.text(AppLocalizationsEn().onboardingSpacesTitle), findsNothing);
    },
  );

  testWidgets(
    'fresh-user can reach Friends by keyboard while spaces coach mark is visible',
    (tester) async {
      _mockVoip(tester);
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });
      final container = await _pumpVoiceShell(
        tester,
        const Size(1280, 800),
        null,
        onboardingAtSpaces: true,
      );

      var focused = false;
      for (var attempt = 0; attempt < 24 && !focused; attempt++) {
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        final primaryContext = FocusManager.instance.primaryFocus?.context;
        primaryContext?.visitAncestorElements((element) {
          if (element.widget.key == const Key('shell_rail_friends')) {
            focused = true;
            return false;
          }
          return true;
        });
      }
      expect(focused, isTrue, reason: FocusManager.instance.primaryFocus?.toStringDeep());

      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();

      expect(container.read(navigationSectionProvider), NavigationSection.social);
      expect(find.byKey(SocialPanel.panelKey), findsOneWidget);
      expect(find.text(AppLocalizationsEn().onboardingSpacesTitle), findsOneWidget);
      expect(find.byKey(OnboardingCoachMark.cardKey), findsOneWidget);
    },
  );

  testWidgets('spaces coach mark fits a narrow viewport', (tester) async {
    _mockVoip(tester);
    await _pumpVoiceShell(
      tester,
      const Size(320, 800),
      null,
      onboardingAtSpaces: true,
    );

    final card = tester.getRect(find.byKey(OnboardingCoachMark.cardKey));
    expect(card.left, greaterThanOrEqualTo(8));
    expect(card.right, lessThanOrEqualTo(312));
    expect(card.top, greaterThanOrEqualTo(8));
    expect(card.bottom, lessThanOrEqualTo(792));
    for (final action in [
      find.widgetWithText(TextButton, AppLocalizationsEn().onboardingSkip),
      find.widgetWithText(TextButton, AppLocalizationsEn().onboardingSpacesFind),
      find.widgetWithText(FilledButton, AppLocalizationsEn().onboardingLater),
    ]) {
      final rect = tester.getRect(action);
      expect(rect.left, greaterThanOrEqualTo(8));
      expect(rect.right, lessThanOrEqualTo(312));
      expect(rect.top, greaterThanOrEqualTo(8));
      expect(rect.bottom, lessThanOrEqualTo(792));
    }
  });

  testWidgets('spaces coach mark scrolls its actions into a short viewport', (
    tester,
  ) async {
    final anchorKey = await _pumpStandaloneCoachMark(
      tester,
      const Size(1280, 220),
      body: List.filled(
        60,
        'Spaces help people find communities and conversations.',
      ).join(' '),
    );

    final anchor = tester.getRect(find.byKey(anchorKey));
    final card = find.byKey(OnboardingCoachMark.cardKey);
    final initialCard = tester.getRect(card);
    expect(initialCard.top, greaterThanOrEqualTo(8));
    expect(initialCard.bottom, lessThanOrEqualTo(212));
    expect(initialCard.top, lessThan(anchor.bottom));

    for (final action in [
      find.text('Skip'),
      find.text('Find spaces'),
      find.text('Later'),
    ]) {
      await tester.ensureVisible(action);
      await tester.pumpAndSettle();
      final cardRect = tester.getRect(card);
      final actionRect = tester.getRect(action);
      expect(actionRect.top, greaterThanOrEqualTo(cardRect.top));
      expect(actionRect.bottom, lessThanOrEqualTo(cardRect.bottom));
      expect(actionRect.top, greaterThanOrEqualTo(8));
      expect(actionRect.bottom, lessThanOrEqualTo(212));
    }
  });

  testWidgets(
    'real shell opens and returns from message requests on wide and narrow layouts',
    (tester) async {
      final captureDir = Platform.environment[_captureEnv];
      final capturing = captureDir != null && captureDir.isNotEmpty;
      final captureTheme = await _themeForCapture(tester, capturing);
      final semantics = tester.ensureSemantics();
      try {
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(_voipChannel, (_) async => null);
        addTearDown(
          () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(_voipChannel, null),
        );
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        final wideChats = _RequestsChatsClient();
        final wideContainer = await _pumpVoiceShell(
          tester,
          const Size(1280, 800),
          captureTheme,
          chats: wideChats,
        );
        expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
        expect(find.byKey(MessageRequestsFolderRailButton.keyId), findsOneWidget);
        expect(
          find.descendant(
            of: find.byKey(DesktopShellRail.railKey),
            matching: find.byType(ChatRailFoldersSection),
          ),
          findsNothing,
        );
        expect(
          find.descendant(
            of: find.byKey(NavigationPanel.panelKey),
            matching: find.byType(ChatRailFoldersSection),
          ),
          findsOneWidget,
        );
        final allFolder = find.byKey(ChatRailFoldersSection.folderKey('all'));
        final requestFolder = find.byKey(MessageRequestsFolderRailButton.keyId);
        expect(find.byTooltip('Requests · 2 unread'), findsOneWidget);
        expect(tester.getCenter(allFolder).dy, tester.getCenter(requestFolder).dy);
        expect(tester.getCenter(requestFolder).dx, greaterThan(tester.getCenter(allFolder).dx));
        await tester.tap(find.byTooltip('Manage folders'));
        await tester.pumpAndSettle();
        expect(find.byKey(ManageFoldersSheet.sheetKey), findsOneWidget);
        Navigator.of(tester.element(find.byKey(ManageFoldersSheet.sheetKey))).pop();
        await tester.pumpAndSettle();
        expect(find.text('2'), findsWidgets);
        wideContainer.read(selectedChatIdProvider.notifier).state = 'ordinary-row';
        wideContainer.read(mobileOpenedChatStripProvider.notifier).openChat('ordinary-row');
        await tester.pumpAndSettle();
        await tester.tap(allFolder);
        await tester.pumpAndSettle();
        expect(wideContainer.read(selectedChatIdProvider), 'ordinary-row');
        await tester.tap(find.byKey(MessageRequestsFolderRailButton.keyId));
        await tester.pumpAndSettle();
        expect(wideContainer.read(selectedChatIdProvider), isNull);
        expect(wideContainer.read(mobileOpenedChatStripProvider), contains('ordinary-row'));
        _expectRequestsWorkspace(tester);
        expect(find.byKey(const Key('message_requests_detail_hint')), findsOneWidget);
        expect(find.text('Select a conversation'), findsNothing);
        await _captureIfRequested(tester, 'contact-requests-wide.png');
        await tester.tap(
          find.descendant(
            of: find.byKey(ChatListBody.tileKey('request-row')),
            matching: find.byType(VoiceAvatar),
          ),
        );
        await tester.pumpAndSettle();
        expect(wideContainer.read(selectedChatIdProvider), 'request-row');
        expect(find.byType(ChatRoomPanel), findsOneWidget);
        expect(wideChats.acceptCalls, 0);
        expect(wideChats.declineCalls, 0);
        await tester.tap(find.byIcon(Icons.arrow_back));
        await tester.pumpAndSettle();
        expect(wideContainer.read(selectedChatFolderIdProvider), isNull);
        expect(wideContainer.read(selectedChatIdProvider), isNull);
        expect(wideContainer.read(mobileOpenedChatStripProvider), contains('ordinary-row'));
        expect(find.text('Direct messages'), findsWidgets);
        expect(find.text('Rowan'), findsNothing);
        expect(find.text('Select a conversation'), findsOneWidget);

        final narrowContainer = await _pumpVoiceShell(
          tester,
          const Size(390, 844),
          captureTheme,
        );
        expect(find.byKey(const Key('mobile_shell_drawer_open')), findsOneWidget);
        await tester.tap(find.byKey(const Key('mobile_shell_drawer_open')));
        await tester.pumpAndSettle();
        expect(find.byKey(MobileShellDrawer.drawerKey), findsOneWidget);
        expect(find.byKey(MessageRequestsFolderDrawerTile.keyId), findsOneWidget);
        expect(find.byType(ChatRailFoldersSection), findsNothing);
        await _captureIfRequested(tester, 'contact-requests-drawer-v.png');
        expect(find.text('2'), findsWidgets);
        expect(
          tester
              .getSemantics(find.byKey(MessageRequestsFolderDrawerTile.keyId))
              .getSemanticsData()
              .hasAction(SemanticsAction.tap),
          isTrue,
        );
        await tester.tap(find.byKey(MessageRequestsFolderDrawerTile.keyId));
        await tester.pumpAndSettle();
        expect(find.byKey(MobileShellDrawer.drawerKey), findsNothing);
        _expectRequestsWorkspace(tester);
        await _captureIfRequested(tester, 'contact-requests-narrow.png');
        await tester.tap(find.byKey(const Key('mobile_shell_drawer_open')));
        await tester.pumpAndSettle();
        final requestTile = tester.widget<ListTile>(
          find.byKey(MessageRequestsFolderDrawerTile.keyId),
        );
        expect(requestTile.selected, isTrue);
        expect(
          tester
                  .getSemantics(find.byKey(MessageRequestsFolderDrawerTile.keyId))
                  .getSemanticsData()
                  .flagsCollection
                  .isSelected ==
              Tristate.isTrue,
          isTrue,
        );
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        if (find.byKey(MobileShellDrawer.drawerKey).evaluate().isNotEmpty) {
          await tester.tapAt(const Offset(380, 400));
          await tester.pumpAndSettle();
        }
        expect(find.byKey(MobileShellDrawer.drawerKey), findsNothing);
        await tester.tap(find.byIcon(Icons.arrow_back));
        await tester.pumpAndSettle();
        expect(narrowContainer.read(selectedChatFolderIdProvider), isNull);
        expect(find.text('Direct messages'), findsWidgets);
        expect(find.text('Rowan'), findsNothing);

        await tester.tap(find.byKey(const Key('mobile_shell_drawer_open')));
        await tester.pumpAndSettle();
        var requestTileFocused = false;
        for (var attempt = 0; attempt < 10 && !requestTileFocused; attempt++) {
          await tester.sendKeyEvent(LogicalKeyboardKey.tab);
          await tester.pump();
          requestTileFocused =
              tester
                  .getSemantics(find.byKey(MessageRequestsFolderDrawerTile.keyId))
                  .getSemanticsData()
                  .flagsCollection
                  .isFocused ==
              Tristate.isTrue;
        }
        expect(requestTileFocused, isTrue);
        await tester.sendKeyEvent(LogicalKeyboardKey.enter);
        await tester.pumpAndSettle();
        expect(find.byKey(MobileShellDrawer.drawerKey), findsNothing);
        expect(narrowContainer.read(chatInboxProvider), 'requests');
        _expectRequestsWorkspace(tester);
        await tester.tap(find.byIcon(Icons.arrow_back));
        await tester.pumpAndSettle();
        expect(narrowContainer.read(chatInboxProvider), 'main');
        expect(find.text('Direct messages'), findsWidgets);
      } finally {
        semantics.dispose();
      }
    },
  );

  testWidgets('request filter remains available without custom or system folders', (
    tester,
  ) async {
    _mockVoip(tester);
    await _pumpVoiceShell(tester, const Size(1280, 800), null, emptyFolders: true);
    expect(find.byKey(MessageRequestsFolderRailButton.keyId), findsOneWidget);
    await tester.tap(find.byKey(MessageRequestsFolderRailButton.keyId));
    await tester.pumpAndSettle();
    _expectRequestsWorkspace(tester);
  });

  for (final layout in ['h', 'v']) {
    testWidgets('actual $layout shell request failure is retryable with keyboard', (
      tester,
    ) async {
      _mockVoip(tester);
      final semantics = tester.ensureSemantics();
      try {
        final pending = Completer<ChatsApiResult<void>>();
        final accepting = layout == 'h';
        final chats = _RequestsChatsClient();
        if (accepting) {
          chats.acceptResponse = pending.future;
        } else {
          chats.declineResponse = pending.future;
        }
        final directory = Platform.environment[_captureEnv];
        final theme = await _themeForCapture(
          tester,
          directory != null && directory.isNotEmpty,
        );
        final container = await _pumpVoiceShell(
          tester,
          layout == 'h' ? const Size(1280, 800) : const Size(390, 844),
          theme,
          chats: chats,
        );
        await _openRequests(tester, narrow: layout == 'v');
        final accept = find.widgetWithText(TextButton, 'Accept');
        final decline = find.widgetWithText(TextButton, 'Decline');
        final action = accepting ? accept : decline;
        final staleTap = tester.widget<TextButton>(action).onPressed!;
        staleTap();
        staleTap();
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 150));
        expect(chats.acceptCalls, accepting ? 1 : 0);
        expect(chats.declineCalls, accepting ? 0 : 1);
        expect(tester.widget<TextButton>(accept).onPressed, isNull);
        expect(tester.widget<TextButton>(decline).onPressed, isNull);
        expect(find.byKey(const Key('chat_request_progress_request-row')), findsOneWidget);
        final rowBounds = tester.getRect(
          find.byKey(ChatListBody.tileKey('request-row')),
        );
        final progressBounds = tester.getRect(
          find.byKey(const Key('chat_request_progress_request-row')),
        );
        for (final control in [accept, decline]) {
          final bounds = tester.getRect(control);
          expect(bounds.left, greaterThanOrEqualTo(rowBounds.left));
          expect(bounds.right, lessThanOrEqualTo(rowBounds.right));
          expect(bounds.overlaps(progressBounds), isFalse);
        }
        expect(progressBounds.left, greaterThanOrEqualTo(rowBounds.left));
        expect(progressBounds.right, lessThanOrEqualTo(rowBounds.right));
        expect(progressBounds.bottom, lessThanOrEqualTo(rowBounds.bottom));
        await _captureIfRequested(tester, 'contact-requests-shell-busy-$layout.png');
        pending.complete(const ChatsApiFailure(message: 'private request failure', statusCode: 503));
        await tester.pumpAndSettle();
        expect(find.text('Could not complete this action.'), findsOneWidget);
        expect(find.textContaining('private request failure'), findsNothing);
        expect(find.text('Rowan'), findsOneWidget);
        expect(tester.widget<TextButton>(accept).onPressed, isNotNull);
        await _captureIfRequested(tester, 'contact-requests-shell-error-$layout.png');
        chats.acceptResponse = null;
        chats.declineResponse = null;
        var focused = false;
        for (var attempt = 0; attempt < 30 && !focused; attempt++) {
          await tester.sendKeyEvent(LogicalKeyboardKey.tab);
          await tester.pump();
          focused = tester.getSemantics(action).getSemanticsData()
              .flagsCollection.isFocused == Tristate.isTrue;
        }
        expect(focused, isTrue);
        await tester.sendKeyEvent(LogicalKeyboardKey.enter);
        await tester.pumpAndSettle();
        expect(chats.acceptCalls, accepting ? 2 : 0);
        expect(chats.declineCalls, accepting ? 0 : 2);
        expect(container.read(chatInboxProvider), 'main');
        expect(find.text('Could not complete this action.'), findsNothing);
        expect(find.text('Rowan'), findsNothing);
        expect(find.byKey(MessageRequestsFolderRailButton.keyId), findsNothing);
        await _captureIfRequested(tester, 'contact-requests-shell-final-$layout.png');
      } finally {
        semantics.dispose();
      }
    });
  }

  for (final accepting in [true, false]) {
    for (final changedContext in ['navigation', 'profile', 'failed-switch', 'new-selection']) {
      for (final succeeded in [false, true]) {
        testWidgets('request accept=$accepting completion success=$succeeded is silent after $changedContext changes', (
          tester,
        ) async {
          _mockVoip(tester);
          final pending = Completer<ChatsApiResult<void>>();
          final chats = _RequestsChatsClient();
          if (accepting) {
            chats.acceptResponse = pending.future;
          } else {
            chats.declineResponse = pending.future;
          }
          final container = await _pumpVoiceShell(
            tester, const Size(1280, 800), null, chats: chats,
          );
          await _openRequests(tester, narrow: false);
          final action = find.widgetWithText(
            TextButton, accepting ? 'Accept' : 'Decline',
          );
          final staleTap = tester.widget<TextButton>(action).onPressed!;
          await tester.tap(action);
          await tester.pump();
          final auth = container.read(authControllerProvider.notifier);
          if (changedContext == 'navigation' || changedContext == 'new-selection') {
            await tester.tap(find.byTooltip('Back to chats'));
            await tester.pumpAndSettle();
            if (changedContext == 'new-selection') {
              await _openRequests(tester, narrow: false, settle: false);
              expect(tester.widget<TextButton>(action).onPressed, isNull);
              staleTap();
              await tester.pump();
              expect(chats.acceptCalls + chats.declineCalls, 1);
            }
          } else if (changedContext == 'profile') {
            auth.state = const AuthState(session: AuthSession(
              accessToken: 'next-token', refreshToken: 'next-refresh',
              accountId: 'next-account', activeProfileId: 'next-profile',
              expiresInSeconds: 900,
            ));
            await tester.pump();
          } else {
            final session = auth.state.session;
            container.read(profileSwitchInProgressProvider.notifier).state = true;
            expect(await auth.switchActiveProfile('next-profile'), isNotNull);
            container.read(profileSwitchInProgressProvider.notifier).state = false;
            expect(auth.state.session, same(session));
            await tester.pump();
          }
          final inboxBefore = container.read(chatInboxProvider);
          final folderBefore = container.read(selectedChatFolderIdProvider);
          pending.complete(succeeded
              ? const ChatsApiOk<void>(null)
              : const ChatsApiFailure(message: 'private stale failure', statusCode: 503));
          await tester.pumpAndSettle();
          expect(chats.acceptCalls, accepting ? 1 : 0);
          expect(chats.declineCalls, accepting ? 0 : 1);
          expect(container.read(chatInboxProvider), inboxBefore);
          expect(container.read(selectedChatFolderIdProvider), folderBefore);
          expect(find.text('Could not complete this action.'), findsNothing);
          expect(find.textContaining('private stale failure'), findsNothing);
        });
      }
    }
  }
}

void _mockVoip(WidgetTester tester) {
  tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
    _voipChannel, (_) async => null,
  );
  addTearDown(() {
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(_voipChannel, null);
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

Future<void> _openRequests(
  WidgetTester tester, {
  required bool narrow,
  bool settle = true,
}) async {
  if (narrow) {
    await tester.tap(find.byKey(const Key('mobile_shell_drawer_open')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(MessageRequestsFolderDrawerTile.keyId));
  } else {
    await tester.tap(find.byKey(MessageRequestsFolderRailButton.keyId));
  }
  if (settle) {
    await tester.pumpAndSettle();
  } else {
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 150));
  }
  _expectRequestsWorkspace(tester);
}

void _expectRequestsWorkspace(WidgetTester tester) {
  expect(
    find.descendant(of: find.byType(ChatListBody), matching: find.text('Message requests')),
    findsOneWidget,
  );
  expect(find.text('Rowan'), findsOneWidget);
  expect(find.byKey(ChatListBody.listKey), findsOneWidget);
}

Future<GlobalKey> _pumpStandaloneCoachMark(
  WidgetTester tester,
  Size size, {
  required String body,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  final anchorKey = GlobalKey();
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: Align(
          alignment: Alignment.topLeft,
          child: SizedBox(key: anchorKey, width: 48, height: 48),
        ),
      ),
    ),
  );
  final entry = OnboardingCoachMark.show(
    context: tester.element(find.byKey(anchorKey)),
    anchorKey: anchorKey,
    title: 'Spaces',
    body: body,
    continueLabel: 'Later',
    skipLabel: 'Skip',
    secondaryLabel: 'Find spaces',
    onContinue: () {},
    onSkip: () {},
    onSecondary: () {},
  ) ??
      (throw StateError('coach mark overlay was not inserted'));
  addTearDown(entry.remove);
  await tester.pumpAndSettle();
  return anchorKey;
}

Future<ProviderContainer> _pumpVoiceShell(
  WidgetTester tester,
  Size size,
  ThemeData? captureTheme, {
  _RequestsChatsClient? chats,
  bool emptyFolders = false,
  bool onboardingAtSpaces = false,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((request) async => http.Response(
          '{}', request.url.path == '/health' ? 200 :
              request.url.path == '/api/v1/auth/switch-profile' ? 503 : 404,
        )),
      ),
      voiceChatsClientProvider.overrideWithValue(chats ?? _RequestsChatsClient()),
      if (onboardingAtSpaces)
        onboardingControllerProvider.overrideWith(
          _IncompleteSpacesOnboardingController.new,
        ),
      activeProfileProvider.overrideWith(
        (_) async => const VoiceProfile(
          id: 'prof-test',
          accountId: 'acc-test',
          username: 'voiceuser',
          discriminator: '4242',
          displayName: 'Voice User',
          isPrimary: true,
        ),
      ),
      for (final profileId in ['next-profile', 'profile-rowan'])
        profileProvider(profileId).overrideWith(
          (_) async => VoiceProfile(
            id: profileId,
            accountId: profileId == 'next-profile' ? 'next-account' : 'account-rowan',
            username: profileId == 'next-profile' ? 'nextuser' : 'rowan',
            discriminator: '4242',
            displayName: profileId == 'next-profile' ? 'Next profile' : 'Rowan',
            isPrimary: true,
          ),
        ),
      chatFoldersProvider.overrideWith(
        (_) async => FolderListData(
          folders: [
            if (!emptyFolders)
              const VoiceFolder(id: 'all', name: 'All chats', folderType: 'system'),
          ],
        ),
      ),
      quickAccessListProvider.overrideWith(
        (_) async => const QuickAccessListData(items: []),
      ),
      presenceProvider.overrideWith((_, _) => null),
      connectivityWatcherProvider.overrideWith((_) {}),
      deepLinkListenerProvider.overrideWith(_NoopDeepLinkListener.new),
      if (captureTheme != null)
        voiceMaterialThemeProvider.overrideWith((_) async => captureTheme),
    ],
  );
  addTearDown(container.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: RepaintBoundary(
        key: _captureBoundaryKey,
        child: const VoiceApp(locale: Locale('en')),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return container;
}

class _IncompleteSpacesOnboardingController extends OnboardingController {
  @override
  OnboardingUiState build() => const OnboardingUiState(
    completedSteps: ['save_account', 'chats_nav'],
    loaded: true,
  );

  @override
  Future<void> load() async {}

  @override
  Future<void> completeStep(String stepId) async {
    state = const OnboardingUiState(completed: true, loaded: true);
  }
}

class _RequestsChatsClient extends FakeVoiceChatsClient {
  _RequestsChatsClient() : super();

  Future<ChatsApiResult<void>>? acceptResponse;
  Future<ChatsApiResult<void>>? declineResponse;
  var acceptCalls = 0;
  var declineCalls = 0;
  var handled = false;

  @override
  Future<ChatsApiResult<void>> acceptDmRequest({
    required String authorization,
    required String chatId,
  }) async {
    acceptCalls++;
    final result = await (acceptResponse ?? Future.value(const ChatsApiOk<void>(null)));
    if (result is ChatsApiOk<void>) {
      handled = true;
    }
    return result;
  }

  @override
  Future<ChatsApiResult<void>> declineDmRequest({
    required String authorization,
    required String chatId,
  }) async {
    declineCalls++;
    final result = await (declineResponse ?? Future.value(const ChatsApiOk<void>(null)));
    if (result is ChatsApiOk<void>) {
      handled = true;
    }
    return result;
  }

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
    String? folderId,
  }) async {
    if (inbox == 'requests') {
      if (handled) return const ChatsApiOk(ChatListData(items: []));
      return const ChatsApiOk(
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: 'request-row',
                type: 'CHAT_TYPE_DM',
                creatorProfileId: 'profile-rowan',
              ),
              unreadCount: 2,
              inbox: 'requests',
              isStranger: true,
              dmPeerProfileId: 'profile-rowan',
              dmPeerDisplayName: 'Rowan',
            ),
          ],
        ),
      );
    }
    return const ChatsApiOk(ChatListData(items: []));
  }

  @override
  Future<ChatsApiResult<FolderListData>> listFolders({
    required String authorization,
  }) async => const ChatsApiOk(
    FolderListData(
      folders: [
        VoiceFolder(id: 'all', name: 'All chats', folderType: 'system'),
      ],
    ),
  );

  @override
  Future<ChatsApiResult<QuickAccessListData>> listQuickAccess({
    required String authorization,
  }) async => const ChatsApiOk(QuickAccessListData(items: []));
}

class _NoopDeepLinkListener extends DeepLinkListener {
  _NoopDeepLinkListener(super.ref);

  @override
  Future<void> start() async {}
}

Future<ThemeData?> _themeForCapture(WidgetTester tester, bool capturing) async {
  if (!capturing) return null;
  if (!_captureFontsLoaded) {
    final loaded = await tester.runAsync(() async {
      final notoSans = FontLoader(VoiceTheme.fontFamily)
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
      await notoSans.load();

      final flutterRoot = Platform.environment['FLUTTER_ROOT'];
      if (flutterRoot == null || flutterRoot.isEmpty) {
        throw StateError('FLUTTER_ROOT is required to load Material Icons');
      }
      final iconFont = File(
        [
          flutterRoot,
          'bin',
          'cache',
          'artifacts',
          'material_fonts',
          'MaterialIcons-Regular.otf',
        ].join(Platform.pathSeparator),
      );
      final iconBytes = await iconFont.readAsBytes();
      final iconData = ByteData.sublistView(Uint8List.fromList(iconBytes));
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) {
      throw StateError('Capture font loading did not finish');
    }
    _captureFontsLoaded = true;
  }

  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) {
    throw StateError('Voice theme asset loading did not finish');
  }
  return theme;
}

Future<void> _captureIfRequested(WidgetTester tester, String filename) async {
  final directory = Platform.environment[_captureEnv];
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final bytes = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ImageByteFormat.png);
      if (data == null) throw StateError('Capture PNG encoding failed');
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      final png = data.buffer.asUint8List();
      await output.writeAsBytes(png, flush: true);
      return png;
    } finally {
      image.dispose();
    }
  });
  if (bytes == null || bytes.isEmpty) {
    throw StateError('Capture did not finish');
  }
}
