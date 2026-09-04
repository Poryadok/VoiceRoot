import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
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
}

Widget _archiveApp(
  InboxReconcilerChatsFake chats, {
  ChatListState? legacyState,
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
        (ref) => _NoAutoArchiveListController(ref, legacyState),
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
}) {
  return ProviderScope(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(chats),
      isDeviceOfflineProvider.overrideWith((ref) => offline),
      chatListControllerProvider.overrideWith(
        (ref) => _NoAutoChatListController(ref, legacyState),
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
  _NoAutoChatListController(super.ref, [ChatListState? initial]) {
    if (initial != null) state = initial;
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _NoAutoArchiveListController extends ChatArchiveListController {
  _NoAutoArchiveListController(super.ref, [ChatListState? initial]) {
    if (initial != null) state = initial;
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
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
