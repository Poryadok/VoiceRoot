import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as messaging_pb;
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/gateway_test_client.dart';

void main() {
  group('T056 P4b ChatRoomController terminal DM state', () {
    test('REST DELETED preserves loaded rows and paging', () async {
      final cache = _RecordingCacheStore();
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(
            ids: const ['msg-1'],
            cursor: 'cursor-older',
            hasMore: true,
            peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
          ),
        ],
      );
      final container = _container(messages: messages, cache: cache);
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isTrue);
      expect(state.messages.map((message) => message.id), ['msg-1']);
      expect(state.nextCursor, 'cursor-older');
      expect(state.hasMore, isTrue);
      expect(await cache.cachedIds(), ['msg-1']);
    });

    test(
      'same-profile token refresh preserves deleted binding and live idempotency',
      () async {
        final auth = _MutableAuthController();
        final hub = _TestRealtimeHub();
        final cache = _RecordingCacheStore();
        final messages = _ScriptedMessagesClient(
          pages: [
            _page(
              ids: const ['msg-1'],
              cursor: 'cursor-current',
              hasMore: true,
              peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
            ),
          ],
        );
        final container = _container(
          auth: auth,
          messages: messages,
          cache: cache,
          realtimeHub: hub,
        );
        addTearDown(container.dispose);

        final subscription = await _loadRoom(container);
        addTearDown(subscription.close);
        await pumpEventQueue();
        var state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isTrue);
        final historyIds = [...state.messages.map((message) => message.id)];
        final historyCursor = state.nextCursor;
        final historyHasMore = state.hasMore;
        final cacheIds = await cache.cachedIdsFor(profileId: 'profile-a');
        final cacheMutations = cache.mutationSignatures();

        auth.state = _authState(
          'profile-a',
          'access-a-rotated',
          refreshToken: 'refresh-a-rotated',
        );
        await pumpEventQueue();

        state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isTrue);
        expect(state.messages.map((message) => message.id), historyIds);
        expect(state.nextCursor, historyCursor);
        expect(state.hasMore, historyHasMore);
        expect(await cache.cachedIdsFor(profileId: 'profile-a'), cacheIds);
        expect(cache.mutationSignatures(), cacheMutations);

        hub.addFrame(
          const RealtimeFrame(
            op: 'dm_peer_deleted',
            data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
          ),
        );
        await pumpEventQueue();

        state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isTrue);
        expect(state.messages.map((message) => message.id), historyIds);
        expect(state.nextCursor, historyCursor);
        expect(state.hasMore, historyHasMore);
        expect(await cache.cachedIdsFor(profileId: 'profile-a'), cacheIds);
        expect(cache.mutationSignatures(), cacheMutations);
      },
    );

    test('profile change and sign-out clear deleted binding', () async {
      final auth = _MutableAuthController();
      final cache = _RecordingCacheStore();
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(
            ids: const ['profile-a-message'],
            cursor: 'profile-a-cursor',
            hasMore: true,
            peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
          ),
          _page(
            ids: const ['profile-b-message'],
            cursor: 'profile-b-cursor',
            hasMore: true,
            peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
          ),
        ],
      );
      final container = _container(
        auth: auth,
        messages: messages,
        cache: cache,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      await pumpEventQueue();
      expect(
        container.read(chatRoomControllerProvider('chat-1')).isDmPeerDeleted,
        isTrue,
      );

      auth.state = _authState('profile-b', 'access-b');
      await pumpEventQueue();
      expect(
        container.read(chatRoomControllerProvider('chat-1')).isDmPeerDeleted,
        isFalse,
      );

      await container
          .read(chatRoomControllerProvider('chat-1').notifier)
          .loadInitial();
      await pumpEventQueue();
      expect(
        container.read(chatRoomControllerProvider('chat-1')).isDmPeerDeleted,
        isTrue,
      );

      auth.state = const AuthState();
      await pumpEventQueue();
      expect(
        container.read(chatRoomControllerProvider('chat-1')).isDmPeerDeleted,
        isFalse,
      );
    });

    test('empty delta still processes response-level DELETED', () async {
      final hub = _TestRealtimeHub();
      final cache = _RecordingCacheStore();
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(ids: const ['msg-1'], cursor: 'cursor-current', hasMore: true),
          _page(peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED),
        ],
      );
      final container = _container(
        messages: messages,
        cache: cache,
        realtimeHub: hub,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      await pumpEventQueue();
      final initialIds = [
        ...container
            .read(chatRoomControllerProvider('chat-1'))
            .messages
            .map((message) => message.id),
      ];
      final initialCursor = container
          .read(chatRoomControllerProvider('chat-1'))
          .nextCursor;
      final initialHasMore = container
          .read(chatRoomControllerProvider('chat-1'))
          .hasMore;
      final initialCacheIds = await cache.cachedIds();
      final initialCacheMutations = cache.mutationSignatures();
      hub.addFrame(
        const RealtimeFrame(
          op: 'message_create',
          data: {'chat_id': 'chat-1', 'message_id': 'new-message'},
        ),
      );
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isTrue);
      expect(state.messages.map((message) => message.id), initialIds);
      expect(state.nextCursor, initialCursor);
      expect(state.hasMore, initialHasMore);
      expect(await cache.cachedIds(), initialCacheIds);
      expect(cache.mutationSignatures(), initialCacheMutations);
      expect(messages.calls.last.afterMessageId, 'msg-1');
    });

    test('older DELETED does not change loaded history or cursor', () async {
      final cache = _RecordingCacheStore();
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(ids: const ['msg-2'], cursor: 'cursor-current', hasMore: true),
          _page(
            ids: const ['msg-1'],
            cursor: 'cursor-should-not-apply',
            hasMore: false,
            peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
          ),
        ],
      );
      final container = _container(messages: messages, cache: cache);
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      await pumpEventQueue();
      final initialIds = [
        ...container
            .read(chatRoomControllerProvider('chat-1'))
            .messages
            .map((message) => message.id),
      ];
      final initialCursor = container
          .read(chatRoomControllerProvider('chat-1'))
          .nextCursor;
      final initialHasMore = container
          .read(chatRoomControllerProvider('chat-1'))
          .hasMore;
      final initialCacheIds = await cache.cachedIds();
      final initialCacheMutations = cache.mutationSignatures();
      await container
          .read(chatRoomControllerProvider('chat-1').notifier)
          .loadOlderMessages();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isTrue);
      expect(state.messages.map((message) => message.id), initialIds);
      expect(state.nextCursor, initialCursor);
      expect(state.hasMore, initialHasMore);
      expect(await cache.cachedIds(), initialCacheIds);
      expect(cache.mutationSignatures(), initialCacheMutations);
      expect(state.isLoadingOlder, isFalse);
    });

    test(
      'repeated REST and live deletion notifications are idempotent',
      () async {
        final hub = _TestRealtimeHub();
        final cache = _RecordingCacheStore();
        final messages = _ScriptedMessagesClient(
          pages: [
            _page(ids: const ['msg-1']),
            _page(
              ids: const ['msg-1'],
              peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
            ),
            _page(
              ids: const ['msg-1'],
              peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
            ),
          ],
        );
        final container = _container(
          messages: messages,
          cache: cache,
          realtimeHub: hub,
        );
        addTearDown(container.dispose);

        final subscription = await _loadRoom(container);
        addTearDown(subscription.close);
        final controller = container.read(
          chatRoomControllerProvider('chat-1').notifier,
        );
        await controller.loadInitial();
        await controller.loadInitial();
        await pumpEventQueue();
        final mutationsBeforeLive = cache.mutationSignatures();

        for (var i = 0; i < 2; i++) {
          hub.addFrame(
            const RealtimeFrame(
              op: 'dm_peer_deleted',
              data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
            ),
          );
        }
        await pumpEventQueue();

        final state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isTrue);
        expect(state.messages.map((message) => message.id), ['msg-1']);
        expect(cache.mutationSignatures(), mutationsBeforeLive);
        expect(await cache.cachedIds(), ['msg-1']);
      },
    );

    test('cached history alone never infers deleted peer', () async {
      final cache = _RecordingCacheStore();
      await cache.replaceChatMessages(
        profileId: 'profile-a',
        chatId: 'chat-1',
        messages: [_message('cached-1')],
      );
      final messages = _ScriptedMessagesClient();
      final container = _container(
        messages: messages,
        cache: cache,
        offline: true,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.messages.map((message) => message.id), ['cached-1']);
      expect(state.isOfflineCache, isTrue);
      expect(state.isDmPeerDeleted, isFalse);
      expect(messages.calls, isEmpty);
    });

    test(
      'late profile-A DELETED response cannot affect profile-B room',
      () async {
        final auth = _MutableAuthController();
        final pending = Completer<MessagesApiResult<MessageListData>>();
        final messages = _ScriptedMessagesClient(pending: pending);
        final cache = _RecordingCacheStore();
        final container = _container(
          auth: auth,
          messages: messages,
          cache: cache,
        );
        addTearDown(container.dispose);

        final subscription = container.listen<ChatRoomState>(
          chatRoomControllerProvider('chat-1'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(subscription.close);
        await pumpEventQueue();

        final controller = container.read(
          chatRoomControllerProvider('chat-1').notifier,
        );
        auth.state = _authState('profile-b', 'access-b');
        controller.state = ChatRoomState(
          messages: [_message('profile-b-message')],
          nextCursor: 'profile-b-cursor',
          hasMore: true,
        );
        pending.complete(
          MessagesApiOk(
            _page(
              ids: const ['profile-a-message'],
              cursor: 'profile-a-cursor',
              hasMore: true,
              peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
            ),
          ),
        );
        await pumpEventQueue();

        final state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isFalse);
        expect(state.messages.map((message) => message.id), [
          'profile-b-message',
        ]);
        expect(state.nextCursor, 'profile-b-cursor');
        expect(state.hasMore, isTrue);
        expect(cache.mutationSignatures(), isEmpty);
      },
    );

    test(
      'matching profile-B live DELETED waits for profile-B-bound history',
      () async {
        final auth = _MutableAuthController();
        final hub = _TestRealtimeHub();
        final cache = _RecordingCacheStore();
        final messages = _ScriptedMessagesClient(
          pages: [
            _page(
              ids: const ['profile-a-message'],
              cursor: 'profile-a-cursor',
              hasMore: true,
            ),
            _page(
              ids: const ['profile-b-message'],
              cursor: 'profile-b-cursor',
              hasMore: true,
            ),
          ],
        );
        final container = _container(
          auth: auth,
          messages: messages,
          cache: cache,
          realtimeHub: hub,
        );
        addTearDown(container.dispose);

        final subscription = await _loadRoom(container);
        addTearDown(subscription.close);
        await pumpEventQueue();
        expect(
          container
              .read(chatRoomControllerProvider('chat-1'))
              .messages
              .map((message) => message.id),
          ['profile-a-message'],
        );

        auth.state = _authState('profile-b', 'access-b');
        hub.addFrame(
          const RealtimeFrame(
            op: 'dm_peer_deleted',
            data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-b'},
          ),
        );
        await pumpEventQueue();

        var state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isFalse);
        expect(state.messages.map((message) => message.id), [
          'profile-a-message',
        ]);
        expect(state.nextCursor, 'profile-a-cursor');
        expect(state.hasMore, isTrue);

        await container
            .read(chatRoomControllerProvider('chat-1').notifier)
            .loadInitial();
        await pumpEventQueue();

        state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isFalse);
        expect(state.messages.map((message) => message.id), [
          'profile-b-message',
        ]);
        expect(state.nextCursor, 'profile-b-cursor');
        expect(state.hasMore, isTrue);

        final bHistoryIds = [...state.messages.map((message) => message.id)];
        final bCursor = state.nextCursor;
        final bHasMore = state.hasMore;
        final bCacheIds = await cache.cachedIdsFor(profileId: 'profile-b');
        final bCacheMutations = cache.mutationSignatures();
        hub.addFrame(
          const RealtimeFrame(
            op: 'dm_peer_deleted',
            data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-b'},
          ),
        );
        await pumpEventQueue();

        state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isTrue);
        expect(state.messages.map((message) => message.id), bHistoryIds);
        expect(state.nextCursor, bCursor);
        expect(state.hasMore, bHasMore);
        expect(await cache.cachedIdsFor(profileId: 'profile-b'), bCacheIds);
        expect(cache.mutationSignatures(), bCacheMutations);
      },
    );

    test('old-profile and nonselected live events are ignored', () async {
      final hub = _TestRealtimeHub();
      final auth = _MutableAuthController();
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(ids: const ['msg-1']),
        ],
      );
      final container = _container(
        auth: auth,
        messages: messages,
        cache: _RecordingCacheStore(),
        realtimeHub: hub,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      auth.state = _authState('profile-b', 'access-b');
      for (final frame in [
        const RealtimeFrame(
          op: 'dm_peer_deleted',
          data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
        ),
        const RealtimeFrame(
          op: 'dm_peer_deleted',
          data: {'chat_id': 'other-chat', 'recipient_profile_id': 'profile-b'},
        ),
        const RealtimeFrame(op: 'dm_peer_deleted', data: {'chat_id': 'chat-1'}),
      ]) {
        hub.addFrame(frame);
      }
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isFalse);
      expect(state.messages.map((message) => message.id), ['msg-1']);
    });

    test(
      'live DELETED requires current profile, chat, and loaded history',
      () async {
        final hub = _TestRealtimeHub();
        final messages = _ScriptedMessagesClient(
          pages: [
            _page(ids: const ['msg-1']),
          ],
        );
        final container = _container(
          messages: messages,
          cache: _RecordingCacheStore(),
          realtimeHub: hub,
        );
        addTearDown(container.dispose);

        final subscription = await _loadRoom(container);
        addTearDown(subscription.close);
        hub.addFrame(
          const RealtimeFrame(
            op: 'dm_peer_deleted',
            data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
          ),
        );
        await pumpEventQueue();

        final state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isTrue);
        expect(state.messages.map((message) => message.id), ['msg-1']);
      },
    );

    test('live DELETED is ignored before any history is loaded', () async {
      final hub = _TestRealtimeHub();
      final messages = _ScriptedMessagesClient(pages: [_page()]);
      final container = _container(
        messages: messages,
        cache: _RecordingCacheStore(),
        realtimeHub: hub,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      hub.addFrame(
        const RealtimeFrame(
          op: 'dm_peer_deleted',
          data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
        ),
      );
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isFalse);
      expect(state.messages, isEmpty);
    });
  });

  group('T053 P4b mutation completions respect active auth generation', () {
    final mutations = <_DeferredMutationCase>[
      _DeferredMutationCase(
        name: 'send',
        kind: _DeferredMutationKind.send,
        invoke: (controller) => controller.sendMessage('stale send'),
      ),
      _DeferredMutationCase(
        name: 'edit',
        kind: _DeferredMutationKind.edit,
        invoke: (controller) =>
            controller.editMessage('message-a', 'stale edit'),
      ),
      _DeferredMutationCase(
        name: 'delete',
        kind: _DeferredMutationKind.delete,
        invoke: (controller) =>
            controller.deleteMessage('message-a', forMe: false),
      ),
      _DeferredMutationCase(
        name: 'reaction failure',
        kind: _DeferredMutationKind.addReaction,
        invoke: (controller) => controller.toggleReaction(
          'message-a',
          '👍',
          currentlyReacted: false,
        ),
      ),
      _DeferredMutationCase(
        name: 'pin',
        kind: _DeferredMutationKind.pin,
        invoke: (controller) =>
            controller.togglePin('message-a', currentlyPinned: false),
      ),
      _DeferredMutationCase(
        name: 'unpin',
        kind: _DeferredMutationKind.unpin,
        invoke: (controller) =>
            controller.togglePin('message-a', currentlyPinned: true),
      ),
    ];

    for (final transition in _StaleAuthTransition.values) {
      for (final mutation in mutations) {
        test(
          '${mutation.name} completion cannot mutate ${transition.label} state or cache',
          () async {
            final auth = _MutableAuthController();
            final cache = _RecordingCacheStore();
            final messages = _DeferredMutationMessagesClient(mutation.kind);
            final container = _container(
              auth: auth,
              messages: messages,
              cache: cache,
            );
            addTearDown(container.dispose);
            final subscription = await _loadRoom(container);
            addTearDown(subscription.close);
            final controller = container.read(
              chatRoomControllerProvider('chat-1').notifier,
            );
            controller.state = ChatRoomState(
              messages: [_message('message-a')],
              pinnedMessages: [_message('message-a')],
              errorMessage: 'error-a',
            );

            final completion = mutation.invoke(controller);
            expect(messages.wasRequested, isTrue);
            expect(messages.requestedAuthorization, 'Bearer access-a');

            final currentProfileId = transition.apply(auth);
            final currentMessage = _message(
              'current-${mutation.kind.name}-${transition.name}',
            );
            final currentPinned = _message(
              'pinned-${mutation.kind.name}-${transition.name}',
            );
            controller.state = ChatRoomState(
              messages: [currentMessage],
              pinnedMessages: [currentPinned],
              errorMessage: 'error-current',
            );
            await cache.replaceChatMessages(
              profileId: currentProfileId,
              chatId: 'chat-1',
              messages: [currentMessage],
            );
            final cacheMutations = cache.mutationSignatures();

            messages.completeStaleResult();
            await completion;
            await pumpEventQueue();

            final state = container.read(chatRoomControllerProvider('chat-1'));
            expect(state.messages, [currentMessage]);
            expect(state.pinnedMessages, [currentPinned]);
            expect(state.errorMessage, 'error-current');
            expect(await cache.cachedIdsFor(profileId: currentProfileId), [
              currentMessage.id,
            ]);
            expect(cache.mutationSignatures(), cacheMutations);
          },
        );
      }

      test(
        'late pinned refresh cannot mutate ${transition.label} pinned state',
        () async {
          final auth = _MutableAuthController();
          final cache = _RecordingCacheStore();
          final messages = _DeferredMutationMessagesClient(
            _DeferredMutationKind.refreshPinned,
          );
          final container = _container(
            auth: auth,
            messages: messages,
            cache: cache,
          );
          addTearDown(container.dispose);
          final subscription = await _loadRoom(container);
          addTearDown(subscription.close);
          expect(messages.wasRequested, isTrue);
          expect(messages.requestedAuthorization, 'Bearer access-a');

          final currentProfileId = transition.apply(auth);
          final currentMessage = _message('current-refresh-${transition.name}');
          final currentPinned = _message('pinned-refresh-${transition.name}');
          final controller = container.read(
            chatRoomControllerProvider('chat-1').notifier,
          );
          controller.state = ChatRoomState(
            messages: [currentMessage],
            pinnedMessages: [currentPinned],
            errorMessage: 'error-current',
          );
          await cache.replaceChatMessages(
            profileId: currentProfileId,
            chatId: 'chat-1',
            messages: [currentMessage],
          );
          final cacheMutations = cache.mutationSignatures();

          messages.completeStaleResult();
          await pumpEventQueue();

          final state = container.read(chatRoomControllerProvider('chat-1'));
          expect(state.messages, [currentMessage]);
          expect(state.pinnedMessages, [currentPinned]);
          expect(state.errorMessage, 'error-current');
          expect(await cache.cachedIdsFor(profileId: currentProfileId), [
            currentMessage.id,
          ]);
          expect(cache.mutationSignatures(), cacheMutations);
        },
      );
    }
  });
}

ProviderContainer _container({
  required VoiceMessagesClient messages,
  required _RecordingCacheStore cache,
  _MutableAuthController? auth,
  _TestRealtimeHub? realtimeHub,
  bool offline = false,
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(
        (ref) => auth ?? _MutableAuthController(),
      ),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceMessagesClientProvider.overrideWithValue(messages),
      realtimeHubProvider.overrideWithValue(realtimeHub ?? _TestRealtimeHub()),
      messageCacheStoreProvider.overrideWithValue(cache),
      isDeviceOfflineProvider.overrideWith((ref) => offline),
    ],
  );
}

Future<ProviderSubscription<ChatRoomState>> _loadRoom(
  ProviderContainer container,
) async {
  final subscription = container.listen<ChatRoomState>(
    chatRoomControllerProvider('chat-1'),
    (_, _) {},
    fireImmediately: true,
  );
  await pumpEventQueue();
  return subscription;
}

MessageListData _page({
  List<String> ids = const [],
  String? cursor,
  bool hasMore = false,
  messaging_pb.DmPeerState? peerState,
}) {
  return MessageListData(
    messages: ids.map(_message).toList(growable: false),
    nextCursor: cursor,
    hasMore: hasMore,
    dmPeerState: peerState,
  );
}

VoiceMessage _message(String id) {
  return VoiceMessage(
    id: id,
    chatId: 'chat-1',
    senderProfileId: 'peer-1',
    content: 'message $id',
    createdAt: DateTime.parse('2024-01-01T00:00:00Z'),
  );
}

AuthState _authState(
  String profileId,
  String accessToken, {
  String? refreshToken,
}) {
  return AuthState(
    session: AuthSession(
      accessToken: accessToken,
      refreshToken: refreshToken ?? 'refresh-$profileId',
      accountId: 'account-1',
      activeProfileId: profileId,
      expiresInSeconds: 900,
    ),
  );
}

class _MutableAuthController extends AuthController {
  _MutableAuthController()
    : super(
        authClient: VoiceAuthClient(
          gateway: gatewayHttpForTest(
            MockClient((_) async => http.Response('{}', 404)),
          ),
        ),
        storage: InMemoryAuthSessionStorage(),
        guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
      ) {
    state = _authState('profile-a', 'access-a');
  }
}

class _ScriptedMessagesClient extends VoiceMessagesClient {
  _ScriptedMessagesClient({
    List<MessageListData> pages = const [],
    this.pending,
  }) : _pages = [...pages],
       super(
         gateway: gatewayHttpForTest(
           MockClient((_) async => http.Response('{}', 500)),
         ),
       );

  final List<MessageListData> _pages;
  final Completer<MessagesApiResult<MessageListData>>? pending;
  final calls = <_GetMessagesCall>[];

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) {
    calls.add(
      _GetMessagesCall(
        authorization: authorization,
        chatId: chatId,
        afterMessageId: afterMessageId,
        lastMessageId: lastMessageId,
        cursor: cursor,
      ),
    );
    if (pending != null) return pending!.future;
    if (_pages.isEmpty) {
      return Future.value(const MessagesApiOk(MessageListData(messages: [])));
    }
    return Future.value(MessagesApiOk(_pages.removeAt(0)));
  }

  @override
  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async => const MessagesApiOk(null);
}

enum _DeferredMutationKind {
  send,
  edit,
  delete,
  addReaction,
  pin,
  unpin,
  refreshPinned,
}

enum _StaleAuthTransition {
  profileSwitch('profile switch', 'profile-b'),
  tokenRotation('same-profile token rotation', 'profile-a');

  const _StaleAuthTransition(this.label, this.profileId);

  final String label;
  final String profileId;

  String apply(_MutableAuthController auth) {
    switch (this) {
      case _StaleAuthTransition.profileSwitch:
        auth.state = _authState('profile-b', 'access-b');
      case _StaleAuthTransition.tokenRotation:
        auth.state = _authState(
          'profile-a',
          'access-a-rotated',
          refreshToken: 'refresh-a-rotated',
        );
    }
    return profileId;
  }
}

class _DeferredMutationCase {
  const _DeferredMutationCase({
    required this.name,
    required this.kind,
    required this.invoke,
  });

  final String name;
  final _DeferredMutationKind kind;
  final Future<String?> Function(ChatRoomController controller) invoke;
}

/// A controlled external REST boundary.  The controller under test and its
/// providers stay real; each case intentionally withholds only one RPC result.
class _DeferredMutationMessagesClient extends VoiceMessagesClient {
  _DeferredMutationMessagesClient(this.kind)
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final _DeferredMutationKind kind;
  final _send = Completer<MessagesApiResult<VoiceMessage>>();
  final _edit = Completer<MessagesApiResult<VoiceMessage>>();
  final _delete = Completer<MessagesApiResult<void>>();
  final _addReaction = Completer<MessagesApiResult<void>>();
  final _pin = Completer<MessagesApiResult<void>>();
  final _unpin = Completer<MessagesApiResult<void>>();
  final _pinned = Completer<MessagesApiResult<MessageListData>>();
  var wasRequested = false;
  String? requestedAuthorization;

  void _markRequested(String authorization) {
    wasRequested = true;
    requestedAuthorization ??= authorization;
  }

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async => const MessagesApiOk(MessageListData(messages: []));

  @override
  Future<MessagesApiResult<VoiceMessage>> sendMessage({
    required String authorization,
    required String chatId,
    required String content,
    List<MessageAttachment> attachments = const [],
    List<MessageMention> mentions = const [],
    String? clientMessageId,
    String? threadParentId,
    bool isE2e = false,
  }) {
    _markRequested(authorization);
    return _send.future;
  }

  @override
  Future<MessagesApiResult<VoiceMessage>> editMessage({
    required String authorization,
    required String messageId,
    required String content,
  }) {
    _markRequested(authorization);
    return _edit.future;
  }

  @override
  Future<MessagesApiResult<void>> deleteMessage({
    required String authorization,
    required String messageId,
    String scope = 'everyone',
  }) {
    _markRequested(authorization);
    return _delete.future;
  }

  @override
  Future<MessagesApiResult<void>> addReaction({
    required String authorization,
    required String messageId,
    required String emoji,
  }) {
    _markRequested(authorization);
    return _addReaction.future;
  }

  @override
  Future<MessagesApiResult<void>> pinMessage({
    required String authorization,
    required String messageId,
    required String chatId,
  }) {
    _markRequested(authorization);
    return _pin.future;
  }

  @override
  Future<MessagesApiResult<void>> unpinMessage({
    required String authorization,
    required String messageId,
    required String chatId,
  }) {
    _markRequested(authorization);
    return _unpin.future;
  }

  @override
  Future<MessagesApiResult<MessageListData>> getPinnedMessages({
    required String authorization,
    required String chatId,
  }) {
    if (kind == _DeferredMutationKind.refreshPinned) {
      _markRequested(authorization);
      return _pinned.future;
    }
    return Future.value(const MessagesApiOk(MessageListData(messages: [])));
  }

  @override
  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async => const MessagesApiOk(null);

  void completeStaleResult() {
    switch (kind) {
      case _DeferredMutationKind.send:
        _send.complete(MessagesApiOk(_message('stale-send')));
      case _DeferredMutationKind.edit:
        _edit.complete(
          MessagesApiOk(_message('message-a').copyWith(content: 'stale edit')),
        );
      case _DeferredMutationKind.delete:
        _delete.complete(const MessagesApiOk(null));
      case _DeferredMutationKind.addReaction:
        _addReaction.complete(
          const MessagesApiFailure(message: 'stale reaction failure'),
        );
      case _DeferredMutationKind.pin:
        _pin.complete(const MessagesApiOk(null));
      case _DeferredMutationKind.unpin:
        _unpin.complete(const MessagesApiOk(null));
      case _DeferredMutationKind.refreshPinned:
        _pinned.complete(MessagesApiOk(_page(ids: const ['stale-pinned'])));
    }
  }
}

class _GetMessagesCall {
  const _GetMessagesCall({
    required this.authorization,
    required this.chatId,
    this.afterMessageId,
    this.lastMessageId,
    this.cursor,
  });

  final String authorization;
  final String chatId;
  final String? afterMessageId;
  final String? lastMessageId;
  final String? cursor;
}

class _TestRealtimeHub extends RealtimeHub {
  _TestRealtimeHub() : super(_UnwiredRef());

  final _events = StreamController<RealtimeFrame>.broadcast();

  @override
  Stream<RealtimeFrame> get events => _events.stream;

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  void addFrame(RealtimeFrame frame) => _events.add(frame);

  @override
  Future<void> dispose() async => _events.close();
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}

class _CacheMutation {
  const _CacheMutation({
    required this.kind,
    this.profileId,
    this.chatId,
    this.messageIds = const [],
  });

  final String kind;
  final String? profileId;
  final String? chatId;
  final List<String> messageIds;

  String get signature =>
      [kind, profileId ?? '', chatId ?? '', ...messageIds].join('|');
}

class _RecordingCacheStore implements MessageCacheStore {
  final _delegate = InMemoryMessageCacheStore();
  final mutations = <_CacheMutation>[];

  Future<List<String>> cachedIds() => cachedIdsFor(profileId: 'profile-a');

  Future<List<String>> cachedIdsFor({
    required String profileId,
    String chatId = 'chat-1',
  }) async => (await _delegate.getMessages(
    profileId: profileId,
    chatId: chatId,
  )).map((message) => message.id).toList(growable: false);

  List<String> mutationSignatures() =>
      mutations.map((mutation) => mutation.signature).toList(growable: false);

  @override
  Future<void> clearAll() async {
    mutations.add(const _CacheMutation(kind: 'clearAll'));
    await _delegate.clearAll();
  }

  @override
  Future<void> clearProfile(String profileId) async {
    mutations.add(_CacheMutation(kind: 'clearProfile', profileId: profileId));
    await _delegate.clearProfile(profileId);
  }

  @override
  Future<List<VoiceMessage>> getMessages({
    required String profileId,
    required String chatId,
  }) => _delegate.getMessages(profileId: profileId, chatId: chatId);

  @override
  Future<void> replaceChatMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) async {
    mutations.add(
      _CacheMutation(
        kind: 'replace',
        profileId: profileId,
        chatId: chatId,
        messageIds: messages
            .map((message) => message.id)
            .toList(growable: false),
      ),
    );
    await _delegate.replaceChatMessages(
      profileId: profileId,
      chatId: chatId,
      messages: messages,
    );
  }

  @override
  Future<void> upsertMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) async {
    mutations.add(
      _CacheMutation(
        kind: 'upsert',
        profileId: profileId,
        chatId: chatId,
        messageIds: messages
            .map((message) => message.id)
            .toList(growable: false),
      ),
    );
    await _delegate.upsertMessages(
      profileId: profileId,
      chatId: chatId,
      messages: messages,
    );
  }
}
