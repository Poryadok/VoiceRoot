import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/gen/voice/chat/v1/chat.pb.dart' as chat_pb;
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'mute and unmute failures keep local state and show safe feedback',
    (tester) async {
      const chatId = 'chat-mute-failure';
      final chats = _TrackingVoiceChatsClient(
        pages: [
          ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: chatId,
                  type: 'CHAT_TYPE_GROUP',
                  creatorProfileId: 'p1',
                  name: 'Mute Failure Target',
                ),
              ),
            ],
          ),
        ],
      )..muteError = 'private_mute_backend_detail';
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          onboardingControllerProvider.overrideWith(
            TestCompletedOnboardingController.new,
          ),
          voiceChatsClientProvider.overrideWith((ref) => chats),
          chatFoldersProvider.overrideWith(
            (_) async => const FolderListData(folders: []),
          ),
          quickAccessListProvider.overrideWith(
            (_) async => const QuickAccessListData(items: []),
          ),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatListBody(showHeader: false)),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(ChatListBody.rowActionsButtonKey(chatId)));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(ChatListBody.muteActionKey(chatId)));
      await tester.pumpAndSettle();

      expect(chats.muteCalls, hasLength(1));
      expect(chats.muteCalls.single.$2, isNotNull);
      expect(container.read(chatMutedUntilProvider), isEmpty);
      expect(find.text('Could not complete this action.'), findsOneWidget);
      expect(find.text('private_mute_backend_detail'), findsNothing);

      tester.state<ScaffoldMessengerState>(find.byType(ScaffoldMessenger))
        ..hideCurrentSnackBar();
      await tester.pumpAndSettle();
      final mutedUntil = DateTime.utc(9999, 12, 31);
      container.read(chatMutedUntilProvider.notifier).state = {
        chatId: mutedUntil,
      };
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(ChatListBody.rowActionsButtonKey(chatId)));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(ChatListBody.muteActionKey(chatId)));
      await tester.pumpAndSettle();

      expect(chats.muteCalls, hasLength(2));
      expect(chats.muteCalls.last.$2, isNull);
      expect(container.read(chatMutedUntilProvider), {chatId: mutedUntil});
      expect(find.text('Could not complete this action.'), findsOneWidget);
      expect(find.text('private_mute_backend_detail'), findsNothing);
    },
  );

  testWidgets('chat row overflow opens actions and archives the chat', (
    tester,
  ) async {
    const chatId = 'chat-archive-overflow';
    final chats = _TrackingVoiceChatsClient(
      pages: [
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: chatId,
                type: 'CHAT_TYPE_GROUP',
                creatorProfileId: 'p1',
                name: 'Archive Target',
              ),
            ),
          ],
        ),
      ],
    );
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        onboardingControllerProvider.overrideWith(
          TestCompletedOnboardingController.new,
        ),
        voiceChatsClientProvider.overrideWith((ref) => chats),
        chatFoldersProvider.overrideWith(
          (_) async => const FolderListData(folders: []),
        ),
        quickAccessListProvider.overrideWith(
          (_) async => const QuickAccessListData(items: []),
        ),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListBody(showHeader: false)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final overflowButton = find.byKey(ChatListBody.rowActionsButtonKey(chatId));
    expect(overflowButton, findsOneWidget);
    final overflowSemantics = tester
        .getSemantics(overflowButton)
        .getSemanticsData();
    expect(overflowSemantics.tooltip, 'Archive');
    expect(overflowSemantics.hasAction(SemanticsAction.tap), isTrue);
    expect(overflowSemantics.flagsCollection.isButton, isTrue);

    await tester.tap(overflowButton);
    await tester.pumpAndSettle();
    final archiveAction = find.byKey(ChatListBody.archiveActionKey(chatId));
    expect(archiveAction, findsOneWidget);
    await tester.tap(archiveAction);
    await tester.pumpAndSettle();

    expect(chats.archived, [chatId]);
    expect(find.byKey(ChatListBody.tileKey(chatId)), findsNothing);
  });

  test('folder ListChats mapping restores persisted pin after reload', () {
    final item = chatListItemFromProto(
      chat_pb.ChatListItem(
        chat: chat_pb.Chat(id: 'chat-pin-1'),
        isPinned: true,
      ),
    );

    expect(item.isPinned, isTrue);
  });

  testWidgets('Chat row actions show pin when folder selected', (tester) async {
    const folderId = 'folder-custom';
    const chatId = 'chat-pin-1';
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        onboardingControllerProvider.overrideWith(
          TestCompletedOnboardingController.new,
        ),
        voiceChatsClientProvider.overrideWith(
          (ref) => FakeVoiceChatsClient(
            pages: [
              ChatListData(
                items: [
                  ChatListItem(
                    chat: VoiceChat(
                      id: chatId,
                      type: 'CHAT_TYPE_GROUP',
                      creatorProfileId: 'p1',
                      name: 'Pin Target',
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
        chatFoldersProvider.overrideWith(
          (_) async => FolderListData(
            folders: [
              VoiceFolder(id: folderId, name: 'Custom', folderType: 'custom'),
            ],
          ),
        ),
        quickAccessListProvider.overrideWith(
          (_) async => const QuickAccessListData(items: []),
        ),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatFolderIdProvider.notifier).state = folderId;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListBody(showHeader: false)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(ChatListBody.quickAccessButtonKey(chatId)),
      findsOneWidget,
      reason: 'Quick Access must be directly discoverable on each chat row',
    );

    await tester.longPress(find.text('Pin Target'));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatListBody.pinActionKey(chatId)), findsOneWidget);
    expect(
      find.byKey(ChatListBody.quickAccessActionKey(chatId)),
      findsOneWidget,
    );
    expect(
      find.byKey(ChatListBody.removeFromFolderActionKey(chatId)),
      findsOneWidget,
    );
    expect(find.byKey(ChatListBody.archiveActionKey(chatId)), findsOneWidget);
  });

  testWidgets('Group row can be archived and added to a custom folder', (
    tester,
  ) async {
    const chatId = 'chat-group-1';
    const folderId = 'folder-custom';
    final chats = _TrackingVoiceChatsClient(
      pages: [
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: chatId,
                type: 'CHAT_TYPE_GROUP',
                creatorProfileId: 'p1',
                name: 'Group Target',
              ),
            ),
          ],
        ),
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: chatId,
                type: 'CHAT_TYPE_GROUP',
                creatorProfileId: 'p1',
                name: 'Group Target',
              ),
            ),
          ],
        ),
      ],
    );
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        onboardingControllerProvider.overrideWith(
          TestCompletedOnboardingController.new,
        ),
        voiceChatsClientProvider.overrideWith((ref) => chats),
        chatFoldersProvider.overrideWith(
          (_) async => FolderListData(
            folders: [
              VoiceFolder(id: folderId, name: 'Custom', folderType: 'custom'),
            ],
          ),
        ),
        quickAccessListProvider.overrideWith(
          (_) async => const QuickAccessListData(items: []),
        ),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListBody(showHeader: false)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.longPress(find.text('Group Target'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatListBody.addToFolderActionKey(chatId)));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(Key('chat_list_add_to_folder_$folderId')));
    await tester.pumpAndSettle();
    expect(chats.added, [(folderId, chatId)]);

    await tester.longPress(find.text('Group Target'));
    await tester.pumpAndSettle();
    chats.archiveError = 'private_archive_row_backend_detail';
    await tester.tap(find.byKey(ChatListBody.archiveActionKey(chatId)));
    await tester.pumpAndSettle();
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('private_archive_row_backend_detail'), findsNothing);
  });

  testWidgets('System folder row cannot remove implicit membership', (
    tester,
  ) async {
    const folderId = 'folder-dm';
    const chatId = 'chat-dm-1';
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        onboardingControllerProvider.overrideWith(
          TestCompletedOnboardingController.new,
        ),
        voiceChatsClientProvider.overrideWith(
          (ref) => FakeVoiceChatsClient(
            pages: [
              ChatListData(
                items: [
                  ChatListItem(
                    chat: VoiceChat(
                      id: chatId,
                      type: 'CHAT_TYPE_GROUP',
                      creatorProfileId: 'p1',
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
        chatFoldersProvider.overrideWith(
          (_) async => FolderListData(
            folders: [
              VoiceFolder(id: folderId, name: 'DMs', folderType: 'system'),
            ],
          ),
        ),
        quickAccessListProvider.overrideWith(
          (_) async => const QuickAccessListData(items: []),
        ),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatFolderIdProvider.notifier).state = folderId;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListBody(showHeader: false)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.longPress(find.byKey(ChatListBody.tileKey(chatId)));
    await tester.pumpAndSettle();

    expect(
      find.byKey(ChatListBody.removeFromFolderActionKey(chatId)),
      findsNothing,
    );
  });

  testWidgets('adding a chat to a folder hides API failure details', (
    tester,
  ) async {
    const folderId = 'folder-custom';
    const chatId = 'chat-add-failure';
    final chats = _TrackingVoiceChatsClient(
      pages: [
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: chatId,
                type: 'CHAT_TYPE_GROUP',
                creatorProfileId: 'p1',
                name: 'Add Failure Target',
              ),
            ),
          ],
        ),
      ],
    )..addError = 'private_folder_membership_diagnostic';
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        onboardingControllerProvider.overrideWith(
          TestCompletedOnboardingController.new,
        ),
        voiceChatsClientProvider.overrideWith((ref) => chats),
        chatFoldersProvider.overrideWith(
          (_) async => FolderListData(
            folders: [
              VoiceFolder(id: folderId, name: 'Custom', folderType: 'custom'),
            ],
          ),
        ),
        quickAccessListProvider.overrideWith(
          (_) async => const QuickAccessListData(items: []),
        ),
      ],
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListBody(showHeader: false)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.longPress(find.text('Add Failure Target'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatListBody.addToFolderActionKey(chatId)));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(const Key('chat_list_add_to_folder_$folderId')),
    );
    await tester.pumpAndSettle();

    expect(chats.added, [(folderId, chatId)]);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('private_folder_membership_diagnostic'), findsNothing);
  });

  testWidgets('removing a chat from a folder hides API failure details', (
    tester,
  ) async {
    const folderId = 'folder-custom';
    const chatId = 'chat-remove-failure';
    final chats = _TrackingVoiceChatsClient(
      pages: [
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: chatId,
                type: 'CHAT_TYPE_GROUP',
                creatorProfileId: 'p1',
                name: 'Remove Failure Target',
              ),
            ),
          ],
        ),
      ],
    )..removeError = 'private_folder_membership_diagnostic';
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        onboardingControllerProvider.overrideWith(
          TestCompletedOnboardingController.new,
        ),
        voiceChatsClientProvider.overrideWith((ref) => chats),
        chatFoldersProvider.overrideWith(
          (_) async => FolderListData(
            folders: [
              VoiceFolder(id: folderId, name: 'Custom', folderType: 'custom'),
            ],
          ),
        ),
        quickAccessListProvider.overrideWith(
          (_) async => const QuickAccessListData(items: []),
        ),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatFolderIdProvider.notifier).state = folderId;
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListBody(showHeader: false)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.longPress(find.text('Remove Failure Target'));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(ChatListBody.removeFromFolderActionKey(chatId)),
    );
    await tester.pumpAndSettle();

    expect(chats.removed, [(folderId, chatId)]);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('private_folder_membership_diagnostic'), findsNothing);
  });
}

class _TrackingVoiceChatsClient extends FakeVoiceChatsClient {
  _TrackingVoiceChatsClient({required super.pages});

  final List<(String, String)> added = [];
  final List<(String, String)> removed = [];
  final List<String> archived = [];
  final List<(String, DateTime?)> muteCalls = [];
  String? archiveError;
  String? addError;
  String? removeError;
  String? muteError;

  @override
  Future<ChatsApiResult<void>> muteChat({
    required String authorization,
    required String chatId,
    DateTime? mutedUntil,
  }) async {
    muteCalls.add((chatId, mutedUntil));
    if (muteError case final error?) {
      return ChatsApiFailure(message: error);
    }
    return const ChatsApiOk(null);
  }

  @override
  Future<ChatsApiResult<void>> addChatToFolder({
    required String authorization,
    required String folderId,
    required String chatId,
  }) async {
    added.add((folderId, chatId));
    if (addError case final error?) {
      return ChatsApiFailure(message: error);
    }
    return const ChatsApiOk(null);
  }

  @override
  Future<ChatsApiResult<void>> removeChatFromFolder({
    required String authorization,
    required String folderId,
    required String chatId,
  }) async {
    removed.add((folderId, chatId));
    if (removeError case final error?) {
      return ChatsApiFailure(message: error);
    }
    return const ChatsApiOk(null);
  }

  @override
  Future<ChatsApiResult<void>> archiveChat({
    required String authorization,
    required String chatId,
    required bool archived,
  }) async {
    if (archiveError case final error?) {
      return ChatsApiFailure(message: error);
    }
    if (archived) this.archived.add(chatId);
    return const ChatsApiOk(null);
  }
}
