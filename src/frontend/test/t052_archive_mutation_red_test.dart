import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/ui/chat/chat_archive_screen.dart';

import 'support/gateway_test_client.dart';
import 'support/inbox_reconciler_fakes.dart';

void main() {
  group('T052 archive mutation reconciliation (RED)', () {
    testWidgets(
      'keeps a confirmed archive mutation when a stale archive snapshot completes',
      (tester) async {
        final chats = _ArchiveMutationChatsFake();
        _enqueueSnapshot(chats, archiveItems: const [], mainItems: ['chat-1']);
        _enqueueSnapshot(chats, archiveItems: const [], manualArchive: true);
        final auth = _AuthHarness();
        final container = _container(chats: chats, auth: auth);
        addTearDown(container.dispose);
        InboxChatCall? staleArchiveCall;
        Future<void>? staleReconcile;
        addTearDown(() async {
          final call = staleArchiveCall;
          if (call != null && !call.completed) {
            await chats.completeCall(
              call,
              result: const ChatsApiOk(ChatListData(items: [])),
            );
          }
          await staleReconcile;
        });

        final reconciler = container.read(inboxReconcilerProvider.notifier);
        await reconciler.reconcile();
        final initialArchive = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['profile-a']![InboxScope.archive];
        expect(initialArchive.items, isEmpty);
        expect(initialArchive.isComplete, isTrue);

        staleReconcile = reconciler.reconcile();
        await tester.pump();
        staleArchiveCall = chats.findCall(inbox: 'archive', cursor: null);
        expect(staleArchiveCall, isNotNull);

        final main = container.read(chatListControllerProvider.notifier)
          ..state = ChatListState(
            profileId: 'profile-a',
            items: [inboxChatItem('chat-1')],
          );
        expect(await main.archiveChat('chat-1', archived: true), isNull);
        expect(chats.archiveCalls, [
          const _ArchiveCall(
            authorization: 'Bearer access-a',
            chatId: 'chat-1',
            archived: true,
          ),
        ]);

        final confirmedArchive = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['profile-a']![InboxScope.archive];
        expect(
          confirmedArchive.items.map((item) => item.chatId),
          ['chat-1'],
          reason:
              'the confirmed ArchiveChat mutation must update the authoritative '
              'profile-scoped archive snapshot, not a legacy list',
        );
        expect(confirmedArchive.isComplete, isTrue);

        await tester.pumpWidget(_testApp(container));
        await tester.pump();
        expect(find.byKey(ChatArchiveScreen.tileKey('chat-1')), findsOneWidget);

        await chats.completeCall(
          staleArchiveCall!,
          result: const ChatsApiOk(ChatListData(items: [])),
        );
        await staleReconcile;
        await tester.pump();

        final afterStaleArchive = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['profile-a']![InboxScope.archive];
        expect(afterStaleArchive.items.map((item) => item.chatId), ['chat-1']);
        expect(afterStaleArchive.isComplete, isTrue);
        expect(
          find.byKey(ChatArchiveScreen.tileKey('chat-1')),
          findsOneWidget,
          reason:
              'a stale archive scope response for the same profile/session must not erase the confirmed archive mutation',
        );
        expect(chats.unmatchedCalls, isEmpty);
        expect(
          chats.calls.every(
            (call) =>
                call.profileId == 'profile-a' &&
                call.authorization == 'Bearer access-a',
          ),
          isTrue,
          reason:
              'archive reconciliation must stay scoped to the active session',
        );
      },
    );
  });
}

Widget _testApp(ProviderContainer container) {
  return UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const ChatArchiveScreen(),
    ),
  );
}

ProviderContainer _container({
  required _ArchiveMutationChatsFake chats,
  required _AuthHarness auth,
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith((ref) => auth.controller),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(chats),
      chatListControllerProvider.overrideWith(_NoAutoChatListController.new),
      chatArchiveListControllerProvider.overrideWith(
        _NoAutoArchiveListController.new,
      ),
    ],
  );
}

class _NoAutoChatListController extends ChatListController {
  _NoAutoChatListController(super.ref);

  @override
  Future<void> loadInitial() async {}
}

class _NoAutoArchiveListController extends ChatArchiveListController {
  _NoAutoArchiveListController(super.ref);

  @override
  Future<void> loadInitial() async {}
}

void _enqueueSnapshot(
  _ArchiveMutationChatsFake chats, {
  required List<String> archiveItems,
  List<String> mainItems = const [],
  bool manualArchive = false,
}) {
  for (final scope in InboxScope.values) {
    final ids = switch (scope) {
      InboxScope.main => mainItems,
      InboxScope.archive => archiveItems,
      InboxScope.requests => const <String>[],
    };
    chats.enqueue(
      InboxChatPageScript(
        inbox: scope.name,
        cursor: null,
        profileId: 'profile-a',
        authorization: 'Bearer access-a',
        manual: scope == InboxScope.archive && manualArchive,
        result: ChatsApiOk(
          ChatListData(items: [for (final id in ids) inboxChatItem(id)]),
        ),
      ),
    );
  }
}

class _ArchiveMutationChatsFake extends InboxReconcilerChatsFake {
  _ArchiveMutationChatsFake()
    : super(profileByAuthorization: const {'Bearer access-a': 'profile-a'});

  final List<_ArchiveCall> archiveCalls = [];

  @override
  Future<ChatsApiResult<void>> archiveChat({
    required String authorization,
    required String chatId,
    required bool archived,
  }) async {
    archiveCalls.add(
      _ArchiveCall(
        authorization: authorization,
        chatId: chatId,
        archived: archived,
      ),
    );
    return const ChatsApiOk(null);
  }
}

class _ArchiveCall {
  const _ArchiveCall({
    required this.authorization,
    required this.chatId,
    required this.archived,
  });

  final String authorization;
  final String chatId;
  final bool archived;

  @override
  bool operator ==(Object other) {
    return other is _ArchiveCall &&
        other.authorization == authorization &&
        other.chatId == chatId &&
        other.archived == archived;
  }

  @override
  int get hashCode => Object.hash(authorization, chatId, archived);
}

class _AuthHarness {
  late final AuthController controller =
      AuthController(
          authClient: VoiceAuthClient(
            gateway: gatewayHttpForTest(
              MockClient((_) async => http.Response('{}', 500)),
            ),
          ),
          storage: InMemoryAuthSessionStorage(),
          guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
        )
        ..state = const AuthState(
          session: AuthSession(
            accessToken: 'access-a',
            refreshToken: 'refresh-a',
            accountId: 'account-1',
            activeProfileId: 'profile-a',
            expiresInSeconds: 900,
          ),
        );
}
