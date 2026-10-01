import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart' show gatewayHttpForTest;

void main() {
  test(
    'successful block/unblock increments visibility revision, failures do not',
    () async {
      var statusCode = 204;
      final users = _CountingUsersClient();
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('', statusCode)),
          ),
          voiceUsersClientProvider.overrideWithValue(users),
        ],
      );
      addTearDown(container.dispose);

      expect(container.read(socialBlockVisibilityRevisionProvider), 0);
      expect(
        await container.read(socialActionsProvider).blockAccount('account-1'),
        isNull,
      );
      expect(container.read(socialBlockVisibilityRevisionProvider), 1);

      await container.read(profileProvider('peer-profile').future);
      expect(users.getProfileCalls, 1);

      expect(
        await container
            .read(socialActionsProvider)
            .unblockAccount('account-1', blockedProfileId: 'peer-profile'),
        isNull,
      );
      expect(container.read(socialBlockVisibilityRevisionProvider), 2);
      await container.read(profileProvider('peer-profile').future);
      expect(users.getProfileCalls, 2);

      statusCode = 500;
      expect(
        await container.read(socialActionsProvider).unblockAccount('account-1'),
        isNotNull,
      );
      expect(container.read(socialBlockVisibilityRevisionProvider), 2);
    },
  );

  test(
    'definitive block and unblock failures preserve mounted history',
    () async {
      const chatId = 'failed-social-mutation-chat';
      var offline = false;
      var statusCode = 400;
      final cachedHistory = [
        _message(chatId, 'visible-before-block', 'peer-profile'),
        _message(chatId, 'visible-before-unblock', 'peer-profile'),
      ];
      final filteredHistory = [
        _message(chatId, 'visible-before-block', 'prof-test'),
      ];
      final cache = InMemoryMessageCacheStore();
      final messages = _CountingMessagesClient([
        MessagesApiOk(MessageListData(messages: cachedHistory)),
        MessagesApiOk(MessageListData(messages: cachedHistory)),
        MessagesApiOk(MessageListData(messages: cachedHistory)),
        MessagesApiOk(MessageListData(messages: cachedHistory)),
        MessagesApiOk(MessageListData(messages: filteredHistory)),
        MessagesApiOk(MessageListData(messages: filteredHistory)),
      ]);
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('', statusCode)),
          ),
          voiceMessagesClientProvider.overrideWithValue(messages),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
          realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
          isDeviceOfflineProvider.overrideWith((ref) => offline),
          chatListControllerProvider.overrideWith(_EmptyChatListController.new),
          messageCacheStoreProvider.overrideWithValue(cache),
        ],
      );
      addTearDown(container.dispose);
      container.read(selectedChatIdProvider.notifier).state = chatId;
      final roomProvider = chatRoomControllerProvider(chatId);
      final subscription = container.listen<ChatRoomState>(
        roomProvider,
        (_, _) {},
      );
      addTearDown(subscription.close);
      await pumpEventQueue();
      expect(
        container.read(roomProvider).messages.map((message) => message.id),
        ['visible-before-block', 'visible-before-unblock'],
      );
      expect(
        (await cache.getMessages(
          profileId: container.read(authControllerProvider).activeProfileId!,
          chatId: chatId,
        )).map((message) => message.id),
        ['visible-before-block', 'visible-before-unblock'],
      );

      final actions = container.read(socialActionsProvider);
      expect(
        await actions.blockAccount(
          'blocked-account',
          blockedProfileId: 'peer-profile',
        ),
        isNotNull,
      );
      await pumpEventQueue();
      expect(container.read(socialBlockVisibilityRevisionProvider), 0);
      expect(
        container.read(roomProvider).messages.map((message) => message.id),
        ['visible-before-block', 'visible-before-unblock'],
        reason: 'a definitive rejection restores the prior visible state',
      );
      expect(
        (await cache.getMessages(
          profileId: container.read(authControllerProvider).activeProfileId!,
          chatId: chatId,
        )).map((message) => message.id),
        ['visible-before-block', 'visible-before-unblock'],
        reason: 'selected-room server history repopulates the purged cache',
      );

      expect(
        await actions.unblockAccount(
          'blocked-account',
          blockedProfileId: 'peer-profile',
        ),
        isNotNull,
      );
      await pumpEventQueue();
      expect(container.read(socialBlockVisibilityRevisionProvider), 0);
      expect(
        container.read(roomProvider).messages.map((message) => message.id),
        ['visible-before-block', 'visible-before-unblock'],
        reason: 'unblock rejection also restores the previous barrier state',
      );
      offline = true;
      await container.read(roomProvider.notifier).loadInitial();
      expect(
        container.read(roomProvider).messages.map((message) => message.id),
        ['visible-before-block', 'visible-before-unblock'],
        reason: 'restored cache remains available for an offline reopen',
      );

      offline = false;
      statusCode = 204;
      expect(
        await actions.blockAccount(
          'blocked-account',
          blockedProfileId: 'peer-profile',
        ),
        isNull,
      );
      await pumpEventQueue();
      expect(container.read(socialBlockVisibilityRevisionProvider), 1);
      expect(
        container.read(roomProvider).messages.map((message) => message.id),
        ['visible-before-block'],
      );

      statusCode = 400;
      expect(
        await actions.unblockAccount(
          'blocked-account',
          blockedProfileId: 'peer-profile',
        ),
        isNotNull,
      );
      await pumpEventQueue();
      expect(
        container.read(socialBlockVisibilityBarrierProvider).revision,
        1,
        reason:
            'definitive rejection leaves the committed block barrier intact',
      );
      expect(
        container.read(socialBlockVisibilityRevisionProvider),
        1,
        reason: 'failed unblock does not bump the visibility refresh trigger',
      );
      offline = true;
      await container.read(roomProvider.notifier).loadInitial();
      expect(
        container.read(roomProvider).messages.map((message) => message.id),
        ['visible-before-block'],
        reason: 'failed unblock refills only the currently filtered history',
      );
    },
  );

  test(
    'social block transactions serialize through definitive rejection',
    () async {
      final firstResponse = Completer<http.Response>();
      final firstRequestStarted = Completer<void>();
      final secondResponse = Completer<http.Response>();
      final secondRequestStarted = Completer<void>();
      var requestCount = 0;
      var activeMutationRequests = 0;
      var maxConcurrentMutationRequests = 0;
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (!request.url.path.startsWith('/api/v1/friends/blocks')) {
                return http.Response('{}', 500);
              }
              requestCount++;
              activeMutationRequests++;
              if (activeMutationRequests > maxConcurrentMutationRequests) {
                maxConcurrentMutationRequests = activeMutationRequests;
              }
              try {
                if (requestCount == 1) {
                  firstRequestStarted.complete();
                  return await firstResponse.future;
                }
                secondRequestStarted.complete();
                return await secondResponse.future;
              } finally {
                activeMutationRequests--;
              }
            }),
          ),
          messageCacheStoreProvider.overrideWithValue(
            InMemoryMessageCacheStore(),
          ),
        ],
      );
      addTearDown(container.dispose);

      final actions = container.read(socialActionsProvider);
      final rejected = actions.blockAccount('rejected-account');
      await firstRequestStarted.future;
      final accepted = actions.blockAccount('accepted-account');
      await pumpEventQueue();

      expect(
        maxConcurrentMutationRequests,
        1,
        reason: 'social mutation HTTP requests must not overlap',
      );
      expect(container.read(socialBlockMutationPendingProvider), isTrue);

      firstResponse.complete(http.Response('', 400));
      expect(await rejected, isNotNull);
      await secondRequestStarted.future;
      expect(
        container.read(socialBlockMutationPendingProvider),
        isTrue,
        reason: 'a queued mutation keeps the fail-closed fence raised',
      );
      secondResponse.complete(http.Response('', 204));
      expect(await accepted, isNull);
      expect(requestCount, 2);
      expect(container.read(socialBlockMutationPendingProvider), isFalse);
      expect(container.read(socialBlockVisibilityBarrierProvider).revision, 1);
      expect(container.read(socialBlockVisibilityRevisionProvider), 1);
    },
  );

  test(
    'visibility revision refreshes only the selected room in place',
    () async {
      const chatId = 'selected-chat';
      final messages = _CountingMessagesClient();
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          voiceMessagesClientProvider.overrideWithValue(messages),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
          realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
          isDeviceOfflineProvider.overrideWith((ref) => false),
          chatListControllerProvider.overrideWith(_EmptyChatListController.new),
          messageCacheStoreProvider.overrideWithValue(
            InMemoryMessageCacheStore(),
          ),
        ],
      );
      addTearDown(container.dispose);
      container.read(selectedChatIdProvider.notifier).state = chatId;

      final roomProvider = chatRoomControllerProvider(chatId);
      final originalController = container.read(roomProvider.notifier);
      final subscription = container.listen<ChatRoomState>(
        roomProvider,
        (_, _) {},
      );
      addTearDown(subscription.close);
      await pumpEventQueue();
      expect(messages.getMessagesCalls, 1);

      container.read(socialBlockVisibilityRevisionProvider.notifier).state++;
      await pumpEventQueue();

      expect(messages.getMessagesCalls, 2);
      expect(container.read(roomProvider.notifier), same(originalController));
    },
  );

  test('visibility revision does not reload a background room', () async {
    const chatId = 'background-chat';
    final messages = _CountingMessagesClient();
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        voiceMessagesClientProvider.overrideWithValue(messages),
        realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
        realtimeLinkStatusProvider.overrideWith(
          (ref) => RealtimeLinkStatus.disconnected,
        ),
        realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
        isDeviceOfflineProvider.overrideWith((ref) => false),
        chatListControllerProvider.overrideWith(_EmptyChatListController.new),
        messageCacheStoreProvider.overrideWithValue(
          InMemoryMessageCacheStore(),
        ),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatIdProvider.notifier).state = 'another-chat';

    final roomProvider = chatRoomControllerProvider(chatId);
    final subscription = container.listen<ChatRoomState>(
      roomProvider,
      (_, _) {},
    );
    addTearDown(subscription.close);
    await pumpEventQueue();
    expect(messages.getMessagesCalls, 0);

    container.read(socialBlockVisibilityRevisionProvider.notifier).state++;
    await pumpEventQueue();

    expect(messages.getMessagesCalls, 0);
  });

  test(
    'block hides cached peer messages through refresh failure and offline load',
    () async {
      const chatId = 'shared-chat';
      var offline = false;
      final messages = _CountingMessagesClient([
        MessagesApiOk(
          MessageListData(
            messages: [
              _message(chatId, 'own-message', 'prof-test'),
              _message(chatId, 'peer-message', 'blocked-profile'),
            ],
          ),
        ),
        const MessagesApiFailure(
          message: 'temporarily unavailable',
          errorCode: 'network_error',
        ),
      ]);
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('', 204)),
          ),
          voiceMessagesClientProvider.overrideWithValue(messages),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
          realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
          isDeviceOfflineProvider.overrideWith((ref) => offline),
          chatListControllerProvider.overrideWith(_EmptyChatListController.new),
          messageCacheStoreProvider.overrideWithValue(
            InMemoryMessageCacheStore(),
          ),
        ],
      );
      addTearDown(container.dispose);
      container.read(selectedChatIdProvider.notifier).state = chatId;
      final roomProvider = chatRoomControllerProvider(chatId);
      final subscription = container.listen<ChatRoomState>(
        roomProvider,
        (_, _) {},
      );
      addTearDown(subscription.close);
      await pumpEventQueue();
      expect(
        container
            .read(roomProvider)
            .messages
            .map((message) => message.senderProfileId),
        contains('blocked-profile'),
      );

      expect(
        await container
            .read(socialActionsProvider)
            .blockAccount(
              'blocked-account',
              blockedProfileId: 'blocked-profile',
            ),
        isNull,
      );
      await pumpEventQueue();

      expect(
        container
            .read(roomProvider)
            .messages
            .map((message) => message.senderProfileId),
        isNot(contains('blocked-profile')),
      );
      expect(
        container
            .read(roomProvider)
            .messages
            .map((message) => message.senderProfileId),
        contains('prof-test'),
      );
      final activeProfileId = container
          .read(authControllerProvider)
          .activeProfileId!;
      final cachedMessages = await container
          .read(messageCacheStoreProvider)
          .getMessages(profileId: activeProfileId, chatId: chatId);
      expect(
        cachedMessages.map((message) => message.senderProfileId),
        isNot(contains('blocked-profile')),
      );

      offline = true;
      await container.read(roomProvider.notifier).loadInitial();
      expect(
        container
            .read(roomProvider)
            .messages
            .map((message) => message.senderProfileId),
        isNot(contains('blocked-profile')),
      );

      await container
          .read(socialActionsProvider)
          .blockAccount('unknown-account');
      await pumpEventQueue();
      expect(
        container.read(roomProvider).messages.map((m) => m.senderProfileId),
        ['prof-test'],
      );
    },
  );

  test(
    'block invalidates sibling profiles and offline history in every room',
    () async {
      const selectedChatId = 'selected-shared-chat';
      const backgroundChatId = 'background-shared-chat';
      const lateOpenedChatId = 'late-opened-chat';
      var offline = false;
      final messages = _CountingMessagesClient([
        MessagesApiOk(
          MessageListData(
            messages: [
              _message(selectedChatId, 'selected-own', 'prof-test'),
              _message(selectedChatId, 'blocked-primary', 'blocked-profile'),
              _message(selectedChatId, 'blocked-sibling', 'sibling-profile'),
            ],
          ),
        ),
        const MessagesApiFailure(
          message: 'temporarily unavailable',
          errorCode: 'network_error',
        ),
      ]);
      final cache = InMemoryMessageCacheStore();
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('', 204)),
          ),
          voiceMessagesClientProvider.overrideWithValue(messages),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
          realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
          isDeviceOfflineProvider.overrideWith((ref) => offline),
          chatListControllerProvider.overrideWith(_EmptyChatListController.new),
          messageCacheStoreProvider.overrideWithValue(cache),
        ],
      );
      addTearDown(container.dispose);

      final profileId = container.read(authControllerProvider).activeProfileId!;
      await cache.replaceChatMessages(
        profileId: profileId,
        chatId: backgroundChatId,
        messages: [
          _message(backgroundChatId, 'background-own', 'prof-test'),
          _message(backgroundChatId, 'background-sibling', 'sibling-profile'),
        ],
      );
      await cache.replaceChatMessages(
        profileId: profileId,
        chatId: lateOpenedChatId,
        messages: [
          _message(lateOpenedChatId, 'late-own', 'prof-test'),
          _message(lateOpenedChatId, 'late-sibling', 'sibling-profile'),
        ],
      );
      await cache.replaceChatMessages(
        profileId: 'other-profile-cache',
        chatId: 'other-profile-chat',
        messages: [
          _message(
            'other-profile-chat',
            'other-profile-peer',
            'sibling-profile',
          ),
        ],
      );

      container.read(selectedChatIdProvider.notifier).state = selectedChatId;
      final selectedProvider = chatRoomControllerProvider(selectedChatId);
      final backgroundProvider = chatRoomControllerProvider(backgroundChatId);
      final selectedSubscription = container.listen<ChatRoomState>(
        selectedProvider,
        (_, _) {},
      );
      final backgroundSubscription = container.listen<ChatRoomState>(
        backgroundProvider,
        (_, _) {},
      );
      addTearDown(selectedSubscription.close);
      addTearDown(backgroundSubscription.close);
      await pumpEventQueue();
      expect(
        container.read(selectedProvider).messages.map((m) => m.id),
        contains('blocked-sibling'),
      );

      offline = true;
      await container.read(backgroundProvider.notifier).loadInitial();
      expect(
        container.read(backgroundProvider).messages.map((m) => m.id),
        contains('background-sibling'),
      );

      expect(
        await container
            .read(socialActionsProvider)
            .blockAccount(
              'blocked-account',
              blockedProfileId: 'blocked-profile',
            ),
        isNull,
      );
      await pumpEventQueue();

      expect(
        container.read(selectedProvider).messages.map((m) => m.id),
        isNot(contains('blocked-sibling')),
      );
      expect(
        container.read(backgroundProvider).messages.map((m) => m.id),
        contains('background-own'),
      );
      expect(
        container.read(backgroundProvider).messages.map((m) => m.id),
        isNot(contains('background-sibling')),
      );
      await container.read(backgroundProvider.notifier).loadInitial();
      expect(
        container.read(backgroundProvider).messages.map((m) => m.id),
        isNot(contains('background-sibling')),
      );

      final lateProvider = chatRoomControllerProvider(lateOpenedChatId);
      await container.read(lateProvider.notifier).loadInitial();
      expect(container.read(lateProvider).messages, isEmpty);
      expect(
        await cache.getMessages(
          profileId: 'other-profile-cache',
          chatId: 'other-profile-chat',
        ),
        isEmpty,
      );
    },
  );

  test(
    'delayed old-profile cache write cannot cross a block refresh',
    () async {
      const chatId = 'gated-shared-chat';
      var offline = false;
      var blockRequests = 0;
      final messages = _CountingMessagesClient([
        MessagesApiOk(
          MessageListData(
            messages: [_message(chatId, 'old-a-peer', 'sibling-profile')],
          ),
        ),
        MessagesApiOk(
          MessageListData(
            messages: [_message(chatId, 'fresh-b-own', 'profile-b')],
          ),
        ),
      ]);
      final cache = _GatedMessageCacheStore();
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async {
              blockRequests++;
              return http.Response('', 204);
            }),
          ),
          voiceMessagesClientProvider.overrideWithValue(messages),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
          realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
          isDeviceOfflineProvider.overrideWith((ref) => offline),
          chatListControllerProvider.overrideWith(_EmptyChatListController.new),
          messageCacheStoreProvider.overrideWithValue(cache),
        ],
      );
      addTearDown(container.dispose);
      container.read(selectedChatIdProvider.notifier).state = 'some-other-chat';
      final roomProvider = chatRoomControllerProvider(chatId);
      final subscription = container.listen<ChatRoomState>(
        roomProvider,
        (_, _) {},
      );
      addTearDown(subscription.close);

      final oldProfileLoad = container
          .read(roomProvider.notifier)
          .loadInitial();
      await cache.oldWriteStarted.future;

      final block = container
          .read(socialActionsProvider)
          .blockAccount('blocked-account', blockedProfileId: 'blocked-profile');
      await pumpEventQueue();
      expect(blockRequests, 0, reason: 'purge waits for the gated old write');

      cache.releaseOldWrite.complete();
      expect(await block, isNull);
      expect(blockRequests, 1);

      final auth = container.read(authControllerProvider);
      container.read(authControllerProvider.notifier).state = auth.copyWith(
        session: AuthSession(
          accessToken: auth.session!.accessToken,
          refreshToken: auth.session!.refreshToken,
          accountId: auth.session!.accountId,
          activeProfileId: 'profile-b',
          expiresInSeconds: auth.session!.expiresInSeconds,
          accountType: auth.session!.accountType,
          emailVerificationRequired: auth.session!.emailVerificationRequired,
        ),
      );
      await pumpEventQueue();
      final newProfileLoad = container
          .read(roomProvider.notifier)
          .loadInitial();
      await newProfileLoad;
      final cacheKey = 'profile-b\u0000$chatId';
      expect(
        container.read(
          socialBlockFilteredHistoryRevisionByChatProvider,
        )[cacheKey],
        1,
        reason: 'profile B has its own filtered-history cache marker',
      );
      await oldProfileLoad;

      expect(
        await cache.getMessages(profileId: 'prof-test', chatId: chatId),
        isEmpty,
        reason: 'stale profile A data is purged before the block request',
      );
      expect(
        (await cache.getMessages(
          profileId: 'profile-b',
          chatId: chatId,
        )).map((message) => message.id),
        ['fresh-b-own'],
      );

      offline = true;
      final reopened = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          voiceMessagesClientProvider.overrideWithValue(
            _CountingMessagesClient(),
          ),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
          realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
          isDeviceOfflineProvider.overrideWith((ref) => offline),
          chatListControllerProvider.overrideWith(_EmptyChatListController.new),
          messageCacheStoreProvider.overrideWithValue(cache),
        ],
      );
      addTearDown(reopened.dispose);
      final reopenedRoom = chatRoomControllerProvider(chatId);
      await reopened.read(reopenedRoom.notifier).loadInitial();
      expect(reopened.read(reopenedRoom).messages, isEmpty);
    },
  );

  test('cache purge failure prevents sending the block request', () async {
    var blockRequests = 0;
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(
          MockClient((_) async {
            blockRequests++;
            return http.Response('', 204);
          }),
        ),
        messageCacheStoreProvider.overrideWithValue(
          _FailingClearMessageCacheStore(),
        ),
      ],
    );
    addTearDown(container.dispose);

    expect(
      await container
          .read(socialActionsProvider)
          .blockAccount('blocked-account', blockedProfileId: 'blocked-profile'),
      'cache_clear_failed',
    );
    expect(blockRequests, 0);
    expect(container.read(socialBlockVisibilityRevisionProvider), 0);
  });
}

class _CountingUsersClient extends VoiceUsersClient {
  _CountingUsersClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  var getProfileCalls = 0;

  @override
  Future<UsersApiResult<VoiceProfile>> getProfile({
    required String authorization,
    required String profileId,
  }) async {
    getProfileCalls++;
    return UsersApiOk(
      VoiceProfile(
        id: profileId,
        accountId: 'account-1',
        username: 'peer',
        discriminator: '0001',
        displayName: 'Peer',
      ),
    );
  }
}

class _GatedMessageCacheStore extends InMemoryMessageCacheStore {
  final oldWriteStarted = Completer<void>();
  final releaseOldWrite = Completer<void>();
  var _gated = false;

  @override
  Future<void> replaceChatMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) async {
    if (!_gated &&
        profileId == 'prof-test' &&
        messages.any((message) => message.id == 'old-a-peer')) {
      _gated = true;
      oldWriteStarted.complete();
      await releaseOldWrite.future;
    }
    await super.replaceChatMessages(
      profileId: profileId,
      chatId: chatId,
      messages: messages,
    );
  }
}

class _FailingClearMessageCacheStore extends InMemoryMessageCacheStore {
  @override
  Future<void> clearAll() async {
    throw StateError('cache unavailable');
  }
}

class _CountingMessagesClient extends VoiceMessagesClient {
  _CountingMessagesClient([
    List<MessagesApiResult<MessageListData>> replies = const [],
  ]) : _replies = [...replies],
       super(
         gateway: gatewayHttpForTest(
           MockClient((_) async => http.Response('{}', 500)),
         ),
       );

  final List<MessagesApiResult<MessageListData>> _replies;
  var getMessagesCalls = 0;

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async {
    getMessagesCalls++;
    if (_replies.isNotEmpty) return _replies.removeAt(0);
    return const MessagesApiFailure(message: 'not loaded');
  }
}

VoiceMessage _message(String chatId, String id, String senderProfileId) {
  return VoiceMessage(
    id: id,
    chatId: chatId,
    senderProfileId: senderProfileId,
    content: id,
    createdAt: DateTime.utc(2026, 1, 1),
  );
}

class _EmptyChatListController extends ChatListController {
  _EmptyChatListController(super.ref) : super();

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _FakeRealtimeHub extends RealtimeHub {
  _FakeRealtimeHub(super.ref);

  @override
  void ensureSubscribed(String chatId) {}
}
