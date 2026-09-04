import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/chat/chat_archive_screen.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/inbox_reconciler_fakes.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'keeps a cached row visible while offline reconciliation exposes error and retry',
    (tester) async {
      final chats = InboxReconcilerChatsFake()
        ..enqueue(
          InboxChatPageScript(
            inbox: 'main',
            cursor: null,
            result: ChatsApiOk(
              ChatListData(
                items: [inboxChatItem('cached-row', preview: 'saved locally')],
                nextCursor: 'main-next',
              ),
            ),
          ),
        )
        ..enqueue(
          const InboxChatPageScript(
            inbox: 'requests',
            cursor: null,
            result: ChatsApiOk(ChatListData(items: [])),
          ),
        )
        ..enqueue(
          const InboxChatPageScript(
            inbox: 'archive',
            cursor: null,
            result: ChatsApiOk(ChatListData(items: [])),
          ),
        )
        ..enqueue(
          const InboxChatPageScript(
            inbox: 'main',
            cursor: 'main-next',
            result: ChatsApiFailure(message: 'offline', statusCode: 503),
          ),
        );
      await tester.pumpWidget(_chatListApp(chats: chats, offline: true));
      await tester.pumpAndSettle();

      expect(find.byKey(ChatListBody.tileKey('cached-row')), findsOneWidget);
      expect(find.text('Could not load chats'), findsOneWidget);
      expect(find.text('Try again'), findsOneWidget);

      chats.enqueue(
        InboxChatPageScript(
          inbox: 'main',
          cursor: 'main-next',
          result: ChatsApiOk(
            ChatListData(items: [inboxChatItem('cached-row')]),
          ),
        ),
      );
      final callsBeforeRetry = chats.calls.length;
      await tester.tap(find.text('Try again'));
      await tester.pumpAndSettle();
      expect(chats.calls, hasLength(callsBeforeRetry + 1));
      expect(chats.calls.last.cursor, 'main-next');
      expect(chats.calls.last.inbox, 'main');
      expect(chats.calls.last.authorization, 'Bearer test-access');
      expect(chats.calls.last.profileId, 'prof-test');
      expect(chats.unmatchedCalls, isEmpty);
      expect(chats.pendingScripts, 0);
    },
  );

  testWidgets(
    'renders the first cached page during progressive reconciliation',
    (tester) async {
      await tester.pumpWidget(_chatListApp(chats: _progressiveScripts()));
      await tester.pump();

      expect(find.byKey(ChatListBody.tileKey('main-page-1')), findsOneWidget);
      expect(find.byKey(ChatListBody.listKey), findsOneWidget);
      expect(find.byType(VoiceListSkeleton), findsNothing);
    },
  );

  testWidgets('initial failed reconciliation shows the existing retry state', (
    tester,
  ) async {
    await tester.pumpWidget(_chatListApp(chats: _failedFirstPageScripts()));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatListBody.unavailableKey), findsOneWidget);
    expect(find.text('Could not load chats'), findsWidgets);
    expect(find.text('Try again'), findsOneWidget);
  });

  testWidgets('archive keeps cached rows with a partial-page retry state', (
    tester,
  ) async {
    final chats = InboxReconcilerChatsFake()
      ..enqueue(
        const InboxChatPageScript(
          inbox: 'main',
          cursor: null,
          result: ChatsApiOk(ChatListData(items: [])),
        ),
      )
      ..enqueue(
        const InboxChatPageScript(
          inbox: 'requests',
          cursor: null,
          result: ChatsApiOk(ChatListData(items: [])),
        ),
      )
      ..enqueue(
        InboxChatPageScript(
          inbox: 'archive',
          cursor: null,
          result: ChatsApiOk(
            ChatListData(
              items: [inboxChatItem('cached-archive', preview: 'archived')],
              nextCursor: 'archive-next',
            ),
          ),
        ),
      )
      ..enqueue(
        const InboxChatPageScript(
          inbox: 'archive',
          cursor: 'archive-next',
          result: ChatsApiFailure(message: 'offline', statusCode: 503),
        ),
      )
      ..enqueue(
        InboxChatPageScript(
          inbox: 'archive',
          cursor: 'archive-next',
          result: ChatsApiOk(
            ChatListData(items: [inboxChatItem('archive-after-retry')]),
          ),
        ),
      );
    await tester.pumpWidget(_archiveApp(chats));
    await tester.pumpAndSettle();

    expect(
      find.byKey(ChatArchiveScreen.tileKey('cached-archive')),
      findsOneWidget,
    );
    expect(find.text('Try again'), findsOneWidget);
    expect(find.text('Could not load archived chats'), findsOneWidget);

    final callsBeforeRetry = chats.calls.length;
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(chats.calls, hasLength(callsBeforeRetry + 1));
    expect(chats.calls.last.inbox, 'archive');
    expect(chats.calls.last.cursor, 'archive-next');
    expect(chats.calls.last.authorization, 'Bearer test-access');
    expect(chats.calls.last.profileId, 'prof-test');
    expect(chats.unmatchedCalls, isEmpty);
    expect(chats.pendingScripts, 0);
  });

  testWidgets('main does not merge legacy rows owned by another profile', (
    tester,
  ) async {
    await tester.pumpWidget(
      _chatListApp(
        chats: _manualFirstPageScripts(),
        legacyState: ChatListState(
          items: [inboxChatItem('old-profile-main')],
          profileId: 'another-profile',
        ),
      ),
    );
    await tester.pump();

    expect(find.byKey(ChatListBody.tileKey('old-profile-main')), findsNothing);
  });

  testWidgets('archive does not merge legacy rows owned by another profile', (
    tester,
  ) async {
    await tester.pumpWidget(
      _archiveApp(
        _manualFirstPageScripts(),
        legacyState: ChatListState(
          items: [inboxChatItem('old-profile-archive')],
          profileId: 'another-profile',
        ),
      ),
    );
    await tester.pump();

    expect(
      find.byKey(ChatArchiveScreen.tileKey('old-profile-archive')),
      findsNothing,
    );
  });

  testWidgets('profile handoff hides legacy rows before the new scope exists', (
    tester,
  ) async {
    final chats = InboxReconcilerChatsFake(
      profileByAuthorization: const {
        'Bearer test-access': 'prof-test',
        'Bearer access-b': 'profile-b',
      },
    );
    for (final inbox in ['main', 'requests', 'archive']) {
      chats.enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: null,
          manual: true,
          result: const ChatsApiOk(ChatListData(items: [])),
        ),
      );
    }
    await tester.pumpWidget(
      _chatListApp(
        chats: chats,
        legacyState: ChatListState(
          items: [inboxChatItem('profile-a-row')],
          profileId: 'prof-test',
        ),
      ),
    );
    await tester.pump();
    expect(find.byKey(ChatListBody.tileKey('profile-a-row')), findsOneWidget);
    for (final inbox in ['main', 'requests', 'archive']) {
      chats.enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: null,
          profileId: 'profile-b',
          authorization: 'Bearer access-b',
          manual: true,
          result: const ChatsApiOk(ChatListData(items: [])),
        ),
      );
    }
    final container = ProviderScope.containerOf(
      tester.element(find.byType(ChatListBody)),
    );
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'access-b',
        refreshToken: 'refresh-b',
        accountId: 'account-1',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();

    expect(find.byKey(ChatListBody.tileKey('profile-a-row')), findsNothing);
  });

  testWidgets('logout hides archived legacy rows', (tester) async {
    await tester.pumpWidget(
      _archiveApp(
        _manualFirstPageScripts(),
        legacyState: ChatListState(
          items: [inboxChatItem('archive-before-logout')],
          profileId: 'prof-test',
        ),
      ),
    );
    await tester.pump();
    expect(
      find.byKey(ChatArchiveScreen.tileKey('archive-before-logout')),
      findsOneWidget,
    );
    final container = ProviderScope.containerOf(
      tester.element(find.byType(ChatArchiveScreen)),
    );
    container.read(authControllerProvider.notifier).state = const AuthState();
    await tester.pump();

    expect(
      find.byKey(ChatArchiveScreen.tileKey('archive-before-logout')),
      findsNothing,
    );
  });

  testWidgets('request accept removes the reconciler-owned row', (
    tester,
  ) async {
    final chats = _MutationChatsFake();
    for (final inbox in ['main', 'archive']) {
      chats.enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: null,
          result: const ChatsApiOk(ChatListData(items: [])),
        ),
      );
    }
    chats.enqueue(
      InboxChatPageScript(
        inbox: 'requests',
        cursor: null,
        result: ChatsApiOk(
          ChatListData(items: [inboxChatItem('request-action')]),
        ),
      ),
    );
    await tester.pumpWidget(_chatListApp(chats: chats, inbox: 'requests'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Accept'));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatListBody.tileKey('request-action')), findsNothing);
    expect(chats.acceptedChatIds, ['request-action']);
  });

  testWidgets(
    'late profile A request accept cannot remove the same chat from profile B',
    (tester) async {
      final chats = _MutationChatsFake(
        profileByAuthorization: const {
          'Bearer test-access': 'prof-test',
          'Bearer access-b': 'profile-b',
        },
      );
      for (final inbox in ['main', 'archive']) {
        chats.enqueue(
          InboxChatPageScript(
            inbox: inbox,
            cursor: null,
            result: const ChatsApiOk(ChatListData(items: [])),
          ),
        );
      }
      chats.enqueue(
        InboxChatPageScript(
          inbox: 'requests',
          cursor: null,
          result: ChatsApiOk(
            ChatListData(
              items: [inboxChatItem('same-chat', preview: 'profile A')],
            ),
          ),
        ),
      );
      await tester.pumpWidget(_chatListApp(chats: chats, inbox: 'requests'));
      await tester.pumpAndSettle();

      chats.deferredAccept = Completer<ChatsApiResult<void>>();
      await tester.tap(find.text('Accept'));
      await tester.pump();

      for (final inbox in ['main', 'archive']) {
        chats.enqueue(
          InboxChatPageScript(
            inbox: inbox,
            cursor: null,
            profileId: 'profile-b',
            authorization: 'Bearer access-b',
            result: const ChatsApiOk(ChatListData(items: [])),
          ),
        );
      }
      chats.enqueue(
        InboxChatPageScript(
          inbox: 'requests',
          cursor: null,
          profileId: 'profile-b',
          authorization: 'Bearer access-b',
          result: ChatsApiOk(
            ChatListData(
              items: [inboxChatItem('same-chat', preview: 'profile B')],
            ),
          ),
        ),
      );
      final container = ProviderScope.containerOf(
        tester.element(find.byType(ChatListBody)),
      );
      final callsBeforeProfileB = chats.calls.length;
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.connecting;
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'access-b',
          refreshToken: 'refresh-b',
          accountId: 'account-1',
          activeProfileId: 'profile-b',
          expiresInSeconds: 900,
        ),
      );
      await tester.pump();
      expect(chats.calls, hasLength(callsBeforeProfileB));
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.connected;
      await tester.pump();
      final profileBCalls = chats.calls.skip(callsBeforeProfileB).toList();
      expect(profileBCalls, hasLength(3));
      expect(profileBCalls.map((call) => call.inbox).toSet(), {
        'main',
        'requests',
        'archive',
      });
      expect(find.text('profile B'), findsOneWidget);

      chats.deferredAccept!.complete(const ChatsApiOk(null));
      await tester.pumpAndSettle();

      expect(find.byKey(ChatListBody.tileKey('same-chat')), findsOneWidget);
      expect(find.text('profile B'), findsOneWidget);
      expect(chats.acceptedChatIds, ['same-chat']);
    },
  );

  testWidgets('request accept completion after unmount is ignored', (
    tester,
  ) async {
    final chats = _MutationChatsFake();
    for (final inbox in ['main', 'archive']) {
      chats.enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: null,
          result: const ChatsApiOk(ChatListData(items: [])),
        ),
      );
    }
    chats.enqueue(
      InboxChatPageScript(
        inbox: 'requests',
        cursor: null,
        result: ChatsApiOk(
          ChatListData(items: [inboxChatItem('unmounted-action')]),
        ),
      ),
    );
    await tester.pumpWidget(_chatListApp(chats: chats, inbox: 'requests'));
    await tester.pumpAndSettle();

    chats.deferredAccept = Completer<ChatsApiResult<void>>();
    await tester.tap(find.text('Accept'));
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    chats.deferredAccept!.complete(const ChatsApiOk(null));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
  });

  testWidgets('unarchive removes the reconciler-owned archive row', (
    tester,
  ) async {
    final chats = _MutationChatsFake();
    for (final inbox in ['main', 'requests']) {
      chats.enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: null,
          result: const ChatsApiOk(ChatListData(items: [])),
        ),
      );
    }
    chats.enqueue(
      InboxChatPageScript(
        inbox: 'archive',
        cursor: null,
        result: ChatsApiOk(
          ChatListData(items: [inboxChatItem('archive-action')]),
        ),
      ),
    );
    await tester.pumpWidget(_archiveApp(chats));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Unarchive'));
    await tester.pumpAndSettle();

    expect(
      find.byKey(ChatArchiveScreen.tileKey('archive-action')),
      findsNothing,
    );
    expect(chats.unarchivedChatIds, ['archive-action']);
  });

  testWidgets('custom folder keeps legacy membership rows and load more', (
    tester,
  ) async {
    await tester.pumpWidget(
      _chatListApp(
        chats: _manualFirstPageScripts(),
        selectedFolderId: 'custom-folder',
        legacyState: ChatListState(
          items: [inboxChatItem('custom-folder-row')],
          nextCursor: 'folder-next',
          profileId: 'prof-test',
        ),
      ),
    );
    await tester.pump();

    expect(
      find.byKey(ChatListBody.tileKey('custom-folder-row')),
      findsOneWidget,
    );
    expect(find.byKey(ChatListBody.loadMoreKey), findsOneWidget);
  });

  testWidgets('custom folder drops a late initial page after folder switch', (
    tester,
  ) async {
    final chats = _manualFirstPageScripts()
      ..enqueue(
        const InboxChatPageScript(
          inbox: 'main',
          cursor: null,
          folderId: 'custom-folder-a',
          manual: true,
          result: ChatsApiOk(ChatListData(items: [])),
        ),
      );
    await tester.pumpWidget(
      _chatListApp(
        chats: chats,
        selectedFolderId: 'custom-folder-a',
        legacyState: ChatListState(
          items: [inboxChatItem('existing-folder-row')],
          profileId: 'prof-test',
        ),
      ),
    );
    await tester.pump();
    final container = ProviderScope.containerOf(
      tester.element(find.byType(ChatListBody)),
    );
    final controller =
        container.read(chatListControllerProvider.notifier)
            as _NoAutoChatListController;
    final loadInitial = controller.loadInitialFromTest();
    await tester.pump();
    final lateCall = chats.findCall(
      inbox: 'main',
      cursor: null,
      profileId: 'prof-test',
    )!;

    container.read(selectedChatFolderIdProvider.notifier).state =
        'custom-folder-b';
    await tester.pump();
    await chats.completeCall(
      lateCall,
      result: ChatsApiOk(
        ChatListData(items: [inboxChatItem('late-folder-a-row')]),
      ),
    );
    await loadInitial;
    await tester.pump();

    expect(
      container
          .read(chatListControllerProvider)
          .items
          .map((item) => item.chatId),
      isNot(contains('late-folder-a-row')),
    );
    expect(
      container.read(dmPeerProfileByChatIdProvider),
      isNot(contains('late-folder-a-row')),
    );
  });

  testWidgets(
    'custom folder drops a late load-more page after profile switch',
    (tester) async {
      final chats = InboxReconcilerChatsFake(
        profileByAuthorization: const {
          'Bearer test-access': 'prof-test',
          'Bearer access-b': 'profile-b',
        },
      );
      for (final inbox in ['main', 'requests', 'archive']) {
        chats.enqueue(
          InboxChatPageScript(
            inbox: inbox,
            cursor: null,
            manual: true,
            result: const ChatsApiOk(ChatListData(items: [])),
          ),
        );
      }
      chats.enqueue(
        const InboxChatPageScript(
          inbox: 'main',
          cursor: 'folder-next',
          folderId: 'custom-folder',
          manual: true,
          result: ChatsApiOk(ChatListData(items: [])),
        ),
      );
      await tester.pumpWidget(
        _chatListApp(
          chats: chats,
          selectedFolderId: 'custom-folder',
          allowLegacyLoadMore: true,
          legacyState: ChatListState(
            items: [inboxChatItem('profile-a-folder-row')],
            nextCursor: 'folder-next',
            profileId: 'prof-test',
          ),
        ),
      );
      await tester.pump();
      final container = ProviderScope.containerOf(
        tester.element(find.byType(ChatListBody)),
      );
      final loadMore = container
          .read(chatListControllerProvider.notifier)
          .loadMore();
      await tester.pump();
      final lateCall = chats.findCall(
        inbox: 'main',
        cursor: 'folder-next',
        profileId: 'prof-test',
      )!;

      _enqueueProfileBFirstPages(chats);
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'access-b',
          refreshToken: 'refresh-b',
          accountId: 'account-1',
          activeProfileId: 'profile-b',
          expiresInSeconds: 900,
        ),
      );
      await tester.pump();
      await chats.completeCall(
        lateCall,
        result: ChatsApiOk(
          ChatListData(items: [inboxChatItem('late-profile-a-folder-row')]),
        ),
      );
      await loadMore;
      await tester.pump();

      final legacy = container.read(chatListControllerProvider);
      expect(
        legacy.items.map((item) => item.chatId),
        isNot(contains('late-profile-a-folder-row')),
      );
      expect(
        container.read(dmPeerProfileByChatIdProvider),
        isNot(contains('late-profile-a-folder-row')),
      );
    },
  );

  testWidgets('archive drops a late load-more page after profile switch', (
    tester,
  ) async {
    final chats = InboxReconcilerChatsFake(
      profileByAuthorization: const {
        'Bearer test-access': 'prof-test',
        'Bearer access-b': 'profile-b',
      },
    );
    for (final inbox in ['main', 'requests', 'archive']) {
      chats.enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: null,
          manual: true,
          result: const ChatsApiOk(ChatListData(items: [])),
        ),
      );
    }
    chats.enqueue(
      const InboxChatPageScript(
        inbox: 'archive',
        cursor: 'archive-next',
        manual: true,
        result: ChatsApiOk(ChatListData(items: [])),
      ),
    );
    await tester.pumpWidget(
      _archiveApp(
        chats,
        allowLegacyLoadMore: true,
        legacyState: ChatListState(
          items: [inboxChatItem('profile-a-archive-row')],
          nextCursor: 'archive-next',
          profileId: 'prof-test',
        ),
      ),
    );
    await tester.pump();
    final container = ProviderScope.containerOf(
      tester.element(find.byType(ChatArchiveScreen)),
    );
    final loadMore = container
        .read(chatArchiveListControllerProvider.notifier)
        .loadMore();
    await tester.pump();
    final lateCall = chats.findCall(
      inbox: 'archive',
      cursor: 'archive-next',
      profileId: 'prof-test',
    )!;

    _enqueueProfileBFirstPages(chats);
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'access-b',
        refreshToken: 'refresh-b',
        accountId: 'account-1',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();
    await chats.completeCall(
      lateCall,
      result: ChatsApiOk(
        ChatListData(items: [inboxChatItem('late-profile-a-archive-row')]),
      ),
    );
    await loadMore;
    await tester.pump();

    expect(
      container
          .read(chatArchiveListControllerProvider)
          .items
          .map((item) => item.chatId),
      isNot(contains('late-profile-a-archive-row')),
    );
  });
}

Widget _archiveApp(
  InboxReconcilerChatsFake chats, {
  ChatListState? legacyState,
  bool allowLegacyLoadMore = false,
}) {
  return ProviderScope(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(chats),
      chatListControllerProvider.overrideWith(
        (ref) => _NoAutoChatListController(ref),
      ),
      chatArchiveListControllerProvider.overrideWith(
        (ref) =>
            _NoAutoArchiveListController(ref, legacyState, allowLegacyLoadMore),
      ),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const _ReconcilerDrivenArchiveScreen(),
    ),
  );
}

Widget _chatListApp({
  required InboxReconcilerChatsFake chats,
  bool offline = false,
  ChatListState? legacyState,
  String inbox = 'main',
  String? selectedFolderId,
  bool allowLegacyLoadMore = false,
}) {
  return ProviderScope(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(chats),
      chatInboxProvider.overrideWith((ref) => inbox),
      selectedChatFolderIdProvider.overrideWith((ref) => selectedFolderId),
      isDeviceOfflineProvider.overrideWith((ref) => offline),
      chatListControllerProvider.overrideWith(
        (ref) =>
            _NoAutoChatListController(ref, legacyState, allowLegacyLoadMore),
      ),
      chatArchiveListControllerProvider.overrideWith(
        (ref) => _NoAutoArchiveListController(ref),
      ),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const Scaffold(body: _ReconcilerDrivenChatListBody()),
    ),
  );
}

class _ReconcilerDrivenChatListBody extends ConsumerStatefulWidget {
  const _ReconcilerDrivenChatListBody();

  @override
  ConsumerState<_ReconcilerDrivenChatListBody> createState() =>
      _ReconcilerDrivenChatListBodyState();
}

class _NoAutoChatListController extends ChatListController {
  _NoAutoChatListController(
    super.ref, [
    ChatListState? initial,
    this.allowLoadMore = false,
  ]) {
    if (initial != null) state = initial;
  }

  final bool allowLoadMore;

  @override
  Future<void> loadInitial() async {}

  Future<void> loadInitialFromTest() => super.loadInitial();

  @override
  Future<void> loadMore() async {
    if (allowLoadMore) await super.loadMore();
  }
}

class _NoAutoArchiveListController extends ChatArchiveListController {
  _NoAutoArchiveListController(
    super.ref, [
    ChatListState? initial,
    this.allowLoadMore = false,
  ]) {
    if (initial != null) state = initial;
  }

  final bool allowLoadMore;

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {
    if (allowLoadMore) await super.loadMore();
  }
}

class _ReconcilerDrivenChatListBodyState
    extends ConsumerState<_ReconcilerDrivenChatListBody> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref.read(inboxReconcilerProvider.notifier).reconcile();
    });
  }

  @override
  Widget build(BuildContext context) {
    ref.watch(inboxReconcilerProvider);
    return const ChatListBody(showHeader: false);
  }
}

class _ReconcilerDrivenArchiveScreen extends ConsumerStatefulWidget {
  const _ReconcilerDrivenArchiveScreen();

  @override
  ConsumerState<_ReconcilerDrivenArchiveScreen> createState() =>
      _ReconcilerDrivenArchiveScreenState();
}

class _ReconcilerDrivenArchiveScreenState
    extends ConsumerState<_ReconcilerDrivenArchiveScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) ref.read(inboxReconcilerProvider.notifier).reconcile();
    });
  }

  @override
  Widget build(BuildContext context) {
    ref.watch(inboxReconcilerProvider);
    return const ChatArchiveScreen();
  }
}

InboxReconcilerChatsFake _progressiveScripts() {
  final chats = InboxReconcilerChatsFake();
  for (final inbox in ['main', 'requests', 'archive']) {
    chats
      ..enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: null,
          result: ChatsApiOk(
            ChatListData(
              items: [inboxChatItem('$inbox-page-1')],
              nextCursor: '$inbox-next',
            ),
          ),
        ),
      )
      ..enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: '$inbox-next',
          manual: true,
          result: ChatsApiOk(ChatListData(items: [])),
        ),
      );
  }
  return chats;
}

InboxReconcilerChatsFake _failedFirstPageScripts() {
  final chats = InboxReconcilerChatsFake();
  chats.enqueue(
    const InboxChatPageScript(
      inbox: 'main',
      cursor: null,
      result: ChatsApiFailure(message: 'gateway unavailable', statusCode: 503),
    ),
  );
  for (final inbox in ['requests', 'archive']) {
    chats.enqueue(
      InboxChatPageScript(
        inbox: inbox,
        cursor: null,
        result: const ChatsApiFailure(
          message: 'gateway unavailable',
          statusCode: 503,
        ),
      ),
    );
  }
  return chats;
}

InboxReconcilerChatsFake _manualFirstPageScripts() {
  final chats = InboxReconcilerChatsFake();
  for (final inbox in ['main', 'requests', 'archive']) {
    chats.enqueue(
      InboxChatPageScript(
        inbox: inbox,
        cursor: null,
        manual: true,
        result: const ChatsApiOk(ChatListData(items: [])),
      ),
    );
  }
  return chats;
}

void _enqueueProfileBFirstPages(InboxReconcilerChatsFake chats) {
  for (final inbox in ['main', 'requests', 'archive']) {
    chats.enqueue(
      InboxChatPageScript(
        inbox: inbox,
        cursor: null,
        profileId: 'profile-b',
        authorization: 'Bearer access-b',
        manual: true,
        result: const ChatsApiOk(ChatListData(items: [])),
      ),
    );
  }
}

class _MutationChatsFake extends InboxReconcilerChatsFake {
  _MutationChatsFake({super.profileByAuthorization});

  final acceptedChatIds = <String>[];
  final unarchivedChatIds = <String>[];
  Completer<ChatsApiResult<void>>? deferredAccept;

  @override
  Future<ChatsApiResult<void>> acceptDmRequest({
    required String authorization,
    required String chatId,
  }) async {
    acceptedChatIds.add(chatId);
    final pending = deferredAccept;
    if (pending != null) return pending.future;
    return const ChatsApiOk(null);
  }

  @override
  Future<ChatsApiResult<void>> archiveChat({
    required String authorization,
    required String chatId,
    required bool archived,
  }) async {
    if (!archived) unarchivedChatIds.add(chatId);
    return const ChatsApiOk(null);
  }
}
