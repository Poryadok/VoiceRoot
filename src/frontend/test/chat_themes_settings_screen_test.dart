import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/subscription_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/chat_theme_preference.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/ui/settings/chat_themes_settings_screen.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/settings/appearance_settings_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('Appearance exposes the documented Chat Themes entry', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('', 404)),
      ),
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const AppearanceSettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Chat themes'), findsOneWidget);
  });

  testWidgets('selected chat theme previews, applies and resets locally', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async => _activeSubscription),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const ChatThemesSettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.violet)),
      findsOneWidget,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
          .onPressed,
      isNull,
    );
    await tester.tap(
      find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.violet)),
    );
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
          .onPressed,
      isNotNull,
    );
    expect(await container.read(chatThemePreferenceProvider.future), isEmpty);

    await tester.tap(find.byKey(ChatThemesSettingsScreen.applyKey));
    await tester.pumpAndSettle();
    expect(await container.read(chatThemePreferenceProvider.future), {
      'chat-a': ChatTheme.violet,
    });
    expect(
      tester
          .widget<OutlinedButton>(find.byKey(ChatThemesSettingsScreen.resetKey))
          .onPressed,
      isNotNull,
    );

    await tester.tap(find.byKey(ChatThemesSettingsScreen.resetKey));
    await tester.pumpAndSettle();
    expect(await container.read(chatThemePreferenceProvider.future), isEmpty);
  });

  testWidgets('when no chat is selected Settings first chooses a chat', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async => _activeSubscription),
        chatListControllerProvider.overrideWith(_ThemeChatListController.new),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const ChatThemesSettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('chat_themes_choose_chat-b')), findsOneWidget);
    await tester.tap(find.byKey(const Key('chat_themes_choose_chat-b')));
    await tester.pumpAndSettle();
    expect(find.byKey(ChatThemesSettingsScreen.chatPickerKey), findsNothing);
    expect(
      find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.ocean)),
      findsOneWidget,
    );
  });

  testWidgets('Chat Info opens themes for its own chat', (tester) async {
    SharedPreferences.setMockInitialValues({
      chatThemePreferencePrefKey: '{"chat-b":"midnight"}',
    });
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async => _activeSubscription),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatInfoPanel(chatId: 'chat-b')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatInfoPanel.chatThemesKey), findsOneWidget);
    await tester.tap(find.byKey(ChatInfoPanel.chatThemesKey));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatThemesSettingsScreen.screenKey), findsOneWidget);
    expect(find.byKey(ChatThemesSettingsScreen.chatPickerKey), findsNothing);
    expect(
      tester
          .getSemantics(
            find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.midnight)),
          )
          .flagsCollection
          .isSelected
          .toString(),
      'Tristate.isTrue',
    );
  });

  testWidgets('theme options support keyboard focus and activation', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async => _activeSubscription),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const ChatThemesSettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final option = find.byKey(
      ChatThemesSettingsScreen.themeKey(ChatTheme.violet),
    );
    final inkWell = find.descendant(of: option, matching: find.byType(InkWell));
    final focus = Focus.of(tester.element(inkWell));
    focus.requestFocus();
    await tester.pump();
    expect(focus.hasFocus, isTrue);

    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();

    expect(
      tester.getSemantics(option).flagsCollection.isSelected.toString(),
      'Tristate.isTrue',
    );
    expect(await container.read(chatThemePreferenceProvider.future), isEmpty);
  });

  testWidgets('Settings follows active-chat changes without carrying a draft', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({
      chatThemePreferencePrefKey: '{"chat-a":"violet","chat-b":"midnight"}',
    });
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async => _activeSubscription),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const ChatThemesSettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      tester
          .getSemantics(
            find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.violet)),
          )
          .flagsCollection
          .isSelected
          .toString(),
      'Tristate.isTrue',
    );

    await tester.tap(
      find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.sunset)),
    );
    await tester.pumpAndSettle();
    container.read(selectedChatIdProvider.notifier).state = 'chat-b';
    await tester.pumpAndSettle();

    expect(
      tester
          .getSemantics(
            find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.midnight)),
          )
          .flagsCollection
          .isSelected
          .toString(),
      'Tristate.isTrue',
    );
    expect(await container.read(chatThemePreferenceProvider.future), {
      'chat-a': ChatTheme.violet,
      'chat-b': ChatTheme.midnight,
    });
  });

  testWidgets('a profile switch clears the prior profile chat target', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async => _activeSubscription),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const ChatThemesSettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.ocean)),
      findsOneWidget,
    );

    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'replacement-access',
        refreshToken: 'replacement-refresh',
        accountId: 'acc-test',
        activeProfileId: 'prof-replacement',
        expiresInSeconds: 900,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatThemesSettingsScreen.chatPickerKey), findsOneWidget);
    expect(
      find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.ocean)),
      findsNothing,
    );
  });

  testWidgets('subscription failure is private and can be retried', (
    tester,
  ) async {
    var requests = 0;
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async {
          requests++;
          if (requests == 1) throw StateError('private subscription detail');
          return _activeSubscription;
        }),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const ChatThemesSettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Could not load subscription'), findsOneWidget);
    expect(find.textContaining('private subscription detail'), findsNothing);
    expect(
      tester
          .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
          .onPressed,
      isNull,
    );
    await tester.tap(find.text('Retry').first);
    await tester.pumpAndSettle();

    expect(requests, 2);
    expect(
      tester
          .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
          .onPressed,
      isNull,
    );
    await tester.tap(
      find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.violet)),
    );
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
          .onPressed,
      isNotNull,
    );
  });

  testWidgets(
    'a retained Plus value cannot enable another account chat theme',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      final accountA = Completer<VoiceSubscription?>();
      final accountBInitial = Completer<VoiceSubscription?>();
      final accountBRetry = Completer<VoiceSubscription?>();
      var accountBRequests = 0;
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('', 404)),
          ),
          subscriptionProvider.overrideWith((ref) {
            final accountId = ref.watch(
              authControllerProvider.select(
                (state) => state.session?.accountId,
              ),
            );
            if (accountId == 'acc-test') return accountA.future;
            if (accountId == 'account-b') {
              accountBRequests++;
              return accountBRequests == 1
                  ? accountBInitial.future
                  : accountBRetry.future;
            }
            return Future.value(null);
          }),
          chatListControllerProvider.overrideWith(_ThemeChatListController.new),
        ],
      );
      addTearDown(container.dispose);
      container.read(selectedChatIdProvider.notifier).state = 'chat-a';

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const ChatThemesSettingsScreen(),
          ),
        ),
      );
      accountA.complete(
        const VoiceSubscription(
          id: 'subscription-a',
          accountId: 'acc-test',
          plan: 'premium',
          billingPeriod: 'month',
          status: 'active',
        ),
      );
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
            .onPressed,
        isNull,
      );

      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'account-b-access',
          refreshToken: 'account-b-refresh',
          accountId: 'account-b',
          activeProfileId: 'profile-b',
          expiresInSeconds: 900,
        ),
      );
      await tester.pumpAndSettle();

      final pendingB = container.read(subscriptionProvider);
      expect(pendingB.isLoading, isTrue);
      expect(pendingB.valueOrNull?.accountId, 'acc-test');
      expect(accountBRequests, 1);
      expect(
        find.byKey(ChatThemesSettingsScreen.chatPickerKey),
        findsOneWidget,
      );
      await tester.tap(find.byKey(const Key('chat_themes_choose_chat-b')));
      await tester.pumpAndSettle();
      await tester.tap(
        find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.violet)),
      );
      await tester.pump();

      final applyWhileBPending = tester
          .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
          .onPressed;
      if (applyWhileBPending != null) {
        await tester.ensureVisible(
          find.byKey(ChatThemesSettingsScreen.applyKey),
        );
        await tester.tap(find.byKey(ChatThemesSettingsScreen.applyKey));
        await tester.pumpAndSettle();
      }
      expect(applyWhileBPending, isNull);
      expect(await container.read(chatThemePreferenceProvider.future), isEmpty);

      accountBInitial.completeError(StateError('private subscription detail'));
      await tester.pumpAndSettle();
      final failedB = container.read(subscriptionProvider);
      expect(failedB.hasError, isTrue);
      expect(failedB.valueOrNull?.accountId, 'acc-test');
      expect(find.text('Could not load subscription'), findsOneWidget);
      expect(
        tester
            .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
            .onPressed,
        isNull,
      );
      expect(await container.read(chatThemePreferenceProvider.future), isEmpty);

      await tester.tap(find.text('Retry').first);
      await tester.pump();
      expect(accountBRequests, 2);
      accountBRetry.complete(null);
      await tester.pumpAndSettle();

      final recoveredB = container.read(subscriptionProvider);
      expect(recoveredB.hasValue, isTrue);
      expect(recoveredB.valueOrNull, isNull);
      expect(find.text('Could not load subscription'), findsNothing);
      expect(
        tester
            .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
            .onPressed,
        isNull,
      );
      expect(await container.read(chatThemePreferenceProvider.future), isEmpty);
    },
  );

  testWidgets('a lapsed Plus keeps a saved theme but disables changes', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({
      chatThemePreferencePrefKey: '{"chat-a":"midnight"}',
    });
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async => null),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const ChatThemesSettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      tester
          .getSemantics(
            find.byKey(ChatThemesSettingsScreen.themeKey(ChatTheme.midnight)),
          )
          .flagsCollection
          .isSelected
          .toString(),
      'Tristate.isTrue',
    );
    expect(find.text('Selected'), findsOneWidget);
    expect(
      tester
          .widget<FilledButton>(find.byKey(ChatThemesSettingsScreen.applyKey))
          .onPressed,
      isNull,
    );
    expect(
      find.byKey(const Key('chat_themes_conversation_preview')),
      findsOneWidget,
    );
  });
}

const _activeSubscription = VoiceSubscription(
  id: 'subscription-1',
  accountId: 'acc-test',
  plan: 'premium',
  billingPeriod: 'month',
  status: 'active',
);

class _ThemeChatListController extends ChatListController {
  _ThemeChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-a',
            type: 'CHAT_TYPE_GROUP',
            creatorProfileId: 'prof-test',
            name: 'Alpha',
          ),
        ),
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-b',
            type: 'CHAT_TYPE_GROUP',
            creatorProfileId: 'prof-test',
            name: 'Beta',
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}
