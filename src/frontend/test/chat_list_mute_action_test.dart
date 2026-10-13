import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';
import 'support/voice_test_theme.dart';

const _chatId = 'chat-mute-action';
const _diagnostic = 'upstream_private_diagnostic_123';
final _existingMute = DateTime.utc(2030, 1, 1);

void main() {
  for (final muted in [false, true]) {
    final action = muted ? 'unmute' : 'mute';
    for (final locale in [const Locale('en'), const Locale('ru')]) {
      testWidgets(
        '$action failure is safe in ${locale.languageCode} and permits explicit retry',
        (tester) async {
          final chats = _MuteChatsClient(failFirst: true);
          final container = await _pumpChatList(
            tester,
            chats,
            muted: muted,
            locale: locale,
          );
          final l10n = AppLocalizations.of(
            tester.element(find.byType(ChatListBody)),
          )!;

          await _toggleMute(tester);

          expect(chats.deadlines, [muted ? null : DateTime.utc(9999, 12, 31)]);
          expect(
            container.read(chatMutedUntilProvider),
            muted ? {_chatId: _existingMute} : isEmpty,
          );
          expect(find.text(l10n.commonActionFailed), findsOneWidget);
          expect(find.textContaining(_diagnostic), findsNothing);
          expect(tester.takeException(), isNull);

          await _toggleMute(tester);

          expect(chats.deadlines, [
            muted ? null : DateTime.utc(9999, 12, 31),
            muted ? null : DateTime.utc(9999, 12, 31),
          ]);
          expect(
            container.read(chatMutedUntilProvider),
            muted ? isEmpty : {_chatId: DateTime.utc(9999, 12, 31)},
          );
          expect(tester.takeException(), isNull);
        },
      );
    }

    testWidgets(
      '$action success updates the mute state without error feedback',
      (tester) async {
        final chats = _MuteChatsClient();
        final container = await _pumpChatList(tester, chats, muted: muted);
        final l10n = AppLocalizations.of(
          tester.element(find.byType(ChatListBody)),
        )!;

        await _toggleMute(tester);

        expect(chats.deadlines, [muted ? null : DateTime.utc(9999, 12, 31)]);
        expect(
          container.read(chatMutedUntilProvider),
          muted ? isEmpty : {_chatId: DateTime.utc(9999, 12, 31)},
        );
        expect(find.text(l10n.commonActionFailed), findsNothing);
        expect(tester.takeException(), isNull);
      },
    );
  }
}

Future<ProviderContainer> _pumpChatList(
  WidgetTester tester,
  _MuteChatsClient chats, {
  required bool muted,
  Locale locale = const Locale('en'),
}) async {
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => throw UnimplementedError()),
      ),
      onboardingControllerProvider.overrideWith(
        TestCompletedOnboardingController.new,
      ),
      voiceChatsClientProvider.overrideWithValue(chats),
      chatFoldersProvider.overrideWith(
        (_) async => const FolderListData(folders: []),
      ),
      quickAccessListProvider.overrideWith(
        (_) async => const QuickAccessListData(items: []),
      ),
    ],
  );
  addTearDown(container.dispose);
  if (muted) {
    container.read(chatMutedUntilProvider.notifier).state = {
      _chatId: _existingMute,
    };
  }
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: locale,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: ChatListBody(showHeader: false)),
      ),
    ),
  );
  await tester.pumpAndSettle();
  expect(find.byKey(ChatListBody.tileKey(_chatId)), findsOneWidget);
  return container;
}

Future<void> _toggleMute(WidgetTester tester) async {
  await tester.tap(find.byKey(ChatListBody.rowActionsButtonKey(_chatId)));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(ChatListBody.muteActionKey(_chatId)));
  await tester.pumpAndSettle();
}

class _MuteChatsClient extends FakeVoiceChatsClient {
  _MuteChatsClient({this.failFirst = false})
    : super(
        pages: [
          ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: _chatId,
                  type: 'CHAT_TYPE_GROUP',
                  creatorProfileId: 'prof-test',
                  name: 'Mute target',
                ),
              ),
            ],
          ),
        ],
      );

  final bool failFirst;
  final deadlines = <DateTime?>[];

  @override
  Future<ChatsApiResult<void>> muteChat({
    required String authorization,
    required String chatId,
    DateTime? mutedUntil,
  }) async {
    expect(authorization, 'Bearer test-access');
    expect(chatId, _chatId);
    deadlines.add(mutedUntil);
    if (failFirst && deadlines.length == 1) {
      return const ChatsApiFailure(message: _diagnostic, statusCode: 500);
    }
    return const ChatsApiOk<void>(null);
  }
}
