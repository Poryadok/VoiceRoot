import 'dart:async';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/gateway_request_id.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/state/profile_switch_coordinator.dart';

import 'support/live_gateway_harness.dart';

/// T-055 live contract for the real Gateway/Realtime handoff.
///
/// This is deliberately opt-in and VM-only. It does not simulate a socket or
/// Gateway response: the only HTTP wrapper delegates every request to the
/// live Gateway and records its request ordering. The event ordering proves
/// that InboxReconciler cannot start B REST reconciliation before RealtimeHub
/// has accepted B's real `hello` frame.
///
/// It records the real opaque ListChats cursors, including a complete second
/// page for every inbox. A transparent local relay closes one real client
/// transport so the production Hub proves reconnect history catch-up from a
/// second Gateway `hello` without fabricated frames or link state.
void main() {
  test(
    'profile switch accepts real B hello before B inbox REST, then selected history only',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      // Fixture setup uses only public Gateway APIs and completes before the
      // recorder exists. B is a second profile of A's account.
      final a = await ctx.registerUser('t055-account-a');
      final aProfileId = a.activeProfileId;
      final users = VoiceUsersClient(gateway: ctx.gatewayHttp());
      final createdB = await users.createProfile(
        authorization: a.authorizationHeader,
        displayName: 'T055 Alt',
      );
      expect(createdB, isA<UsersApiOk<VoiceProfile>>(), reason: '$createdB');
      final bProfileId = (createdB as UsersApiOk<VoiceProfile>).data.id;
      final rawBResult = await ctx.authClient().switchActiveProfile(
        session: a,
        profileId: bProfileId,
      );
      expect(rawBResult, isA<AuthSessionOk>(), reason: '$rawBResult');
      final rawB = (rawBResult as AuthSessionOk).session;
      await ctx.allowOpenGamingPrivacy(rawB);

      final mainPeer = await ctx.registerUser('t055-main-peer');
      final archivePeer = await ctx.registerUser('t055-archive-peer');
      final stranger = await ctx.registerUser('t055-stranger');
      final chats = ctx.chatsClient();

      final createdMainAlt = await users.createProfile(
        authorization: mainPeer.authorizationHeader,
        displayName: 'T055 Main Alt',
      );
      expect(
        createdMainAlt,
        isA<UsersApiOk<VoiceProfile>>(),
        reason: '$createdMainAlt',
      );
      final mainAltProfileId =
          (createdMainAlt as UsersApiOk<VoiceProfile>).data.id;
      final createdArchiveAlt = await users.createProfile(
        authorization: archivePeer.authorizationHeader,
        displayName: 'T055 Archive Alt',
      );
      expect(
        createdArchiveAlt,
        isA<UsersApiOk<VoiceProfile>>(),
        reason: '$createdArchiveAlt',
      );
      final archiveAltProfileId =
          (createdArchiveAlt as UsersApiOk<VoiceProfile>).data.id;
      final createdStrangerAlt = await users.createProfile(
        authorization: stranger.authorizationHeader,
        displayName: 'T055 Stranger Alt',
      );
      expect(
        createdStrangerAlt,
        isA<UsersApiOk<VoiceProfile>>(),
        reason: '$createdStrangerAlt',
      );
      final strangerAltProfileId =
          (createdStrangerAlt as UsersApiOk<VoiceProfile>).data.id;

      // Privacy settings are profile-owned. Open the two additional targets
      // through the same public session-switch flow used by the application.
      final rawMainAltResult = await ctx.authClient().switchActiveProfile(
        session: mainPeer,
        profileId: mainAltProfileId,
      );
      expect(
        rawMainAltResult,
        isA<AuthSessionOk>(),
        reason: '$rawMainAltResult',
      );
      final rawMainAlt = (rawMainAltResult as AuthSessionOk).session;
      await ctx.allowOpenGamingPrivacy(rawMainAlt);
      final rawMainPeerResult = await ctx.authClient().switchActiveProfile(
        session: rawMainAlt,
        profileId: mainPeer.activeProfileId,
      );
      expect(
        rawMainPeerResult,
        isA<AuthSessionOk>(),
        reason: '$rawMainPeerResult',
      );
      final rawMainPeer = (rawMainPeerResult as AuthSessionOk).session;
      final rawArchiveAltResult = await ctx.authClient().switchActiveProfile(
        session: archivePeer,
        profileId: archiveAltProfileId,
      );
      expect(
        rawArchiveAltResult,
        isA<AuthSessionOk>(),
        reason: '$rawArchiveAltResult',
      );
      await ctx.allowOpenGamingPrivacy(
        (rawArchiveAltResult as AuthSessionOk).session,
      );

      final selectedDm = await chats.createDm(
        authorization: rawB.authorizationHeader,
        otherProfileId: mainPeer.activeProfileId,
      );
      expect(selectedDm, isA<ChatsApiOk<VoiceChat>>(), reason: '$selectedDm');
      final selectedChatId = (selectedDm as ChatsApiOk<VoiceChat>).data.id;
      final mainAltDm = await chats.createDm(
        authorization: rawB.authorizationHeader,
        otherProfileId: mainAltProfileId,
      );
      expect(mainAltDm, isA<ChatsApiOk<VoiceChat>>(), reason: '$mainAltDm');
      final mainAltChatId = (mainAltDm as ChatsApiOk<VoiceChat>).data.id;

      final archivedDm = await chats.createDm(
        authorization: rawB.authorizationHeader,
        otherProfileId: archivePeer.activeProfileId,
      );
      expect(archivedDm, isA<ChatsApiOk<VoiceChat>>(), reason: '$archivedDm');
      final archivedChatId = (archivedDm as ChatsApiOk<VoiceChat>).data.id;
      final archivedAltDm = await chats.createDm(
        authorization: rawB.authorizationHeader,
        otherProfileId: archiveAltProfileId,
      );
      expect(
        archivedAltDm,
        isA<ChatsApiOk<VoiceChat>>(),
        reason: '$archivedAltDm',
      );
      final archivedAltChatId =
          (archivedAltDm as ChatsApiOk<VoiceChat>).data.id;
      final archive = await chats.archiveChat(
        authorization: rawB.authorizationHeader,
        chatId: archivedChatId,
        archived: true,
      );
      expect(archive, isA<ChatsApiOk<void>>(), reason: '$archive');
      final archiveAlt = await chats.archiveChat(
        authorization: rawB.authorizationHeader,
        chatId: archivedAltChatId,
        archived: true,
      );
      expect(archiveAlt, isA<ChatsApiOk<void>>(), reason: '$archiveAlt');

      final requestDm = await chats.createDm(
        authorization: stranger.authorizationHeader,
        otherProfileId: bProfileId,
      );
      expect(requestDm, isA<ChatsApiOk<VoiceChat>>(), reason: '$requestDm');
      final requestChatId = (requestDm as ChatsApiOk<VoiceChat>).data.id;
      final rawStrangerAltResult = await ctx.authClient().switchActiveProfile(
        session: stranger,
        profileId: strangerAltProfileId,
      );
      expect(
        rawStrangerAltResult,
        isA<AuthSessionOk>(),
        reason: '$rawStrangerAltResult',
      );
      final rawStrangerAlt = (rawStrangerAltResult as AuthSessionOk).session;
      final requestAltDm = await chats.createDm(
        authorization: rawStrangerAlt.authorizationHeader,
        otherProfileId: bProfileId,
      );
      expect(
        requestAltDm,
        isA<ChatsApiOk<VoiceChat>>(),
        reason: '$requestAltDm',
      );
      final requestAltChatId = (requestAltDm as ChatsApiOk<VoiceChat>).data.id;

      final baseline = await ctx.messagesClient().sendMessage(
        authorization: rawMainPeer.authorizationHeader,
        chatId: selectedChatId,
        content: 't055-reconnect-baseline',
        clientMessageId: qaClientMessageId(),
      );
      expect(baseline, isA<MessagesApiOk<VoiceMessage>>(), reason: '$baseline');
      final baselineMessageId =
          (baseline as MessagesApiOk<VoiceMessage>).data.id;
      expect(
        baselineMessageId,
        isNotEmpty,
        reason: 'selected history baseline',
      );

      // Validate the public fixture over two actual one-row pages before
      // starting the production-side recorder. The opaque cursors become the
      // exact values expected from the production reconciler below.
      final expectedCursors = <String, String>{};
      expectedCursors['main'] = await _expectTwoInboxItems(
        chats: chats,
        authorization: rawB.authorizationHeader,
        inbox: 'main',
        chatIds: {selectedChatId, mainAltChatId},
      );
      expectedCursors['requests'] = await _expectTwoInboxItems(
        chats: chats,
        authorization: rawB.authorizationHeader,
        inbox: 'requests',
        chatIds: {requestChatId, requestAltChatId},
      );
      expectedCursors['archive'] = await _expectTwoInboxItems(
        chats: chats,
        authorization: rawB.authorizationHeader,
        inbox: 'archive',
        chatIds: {archivedChatId, archivedAltChatId},
      );

      // The setup switch from A to B rotates A's session. Return to A only
      // after every B-owned fixture operation, then seed the coordinator with
      // the freshly issued A session for the actual A -> B handoff below.
      final rawAResult = await ctx.authClient().switchActiveProfile(
        session: rawB,
        profileId: aProfileId,
      );
      expect(rawAResult, isA<AuthSessionOk>(), reason: '$rawAResult');
      final rawA = (rawAResult as AuthSessionOk).session;

      final recorder = _RecordingHttpClient(ctx.httpClient);
      final relay = await _LiveWebSocketRelay.bind();
      addTearDown(relay.dispose);
      final storage = InMemoryAuthSessionStorage();
      final controller = AuthController(
        authClient: VoiceAuthClient(
          gateway: GatewayHttpClient(httpClient: recorder, config: ctx.config),
        ),
        storage: storage,
        guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
      )..state = AuthState(session: rawA);
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith((_) => controller),
          authSessionStorageProvider.overrideWithValue(storage),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(ctx.config),
          httpClientProvider.overrideWithValue(recorder),
          realtimeTransportFactoryProvider.overrideWithValue(
            _RelayRealtimeTransportFactory(relay),
          ),
          // Suppressing only automatic initial-A connection makes the
          // coordinator handoff the sole initial live WS action in this test.
          realtimeAutoConnectProvider.overrideWithValue(false),
        ],
      );
      addTearDown(container.dispose);

      final initialBHello = Completer<RealtimeHelloBinding>();
      final reconnectBHello = Completer<RealtimeHelloBinding>();
      var initialHelloGeneration = 0;
      final helloSubscription = container.listen<RealtimeHelloBinding?>(
        realtimeHelloBindingProvider,
        (_, next) {
          if (next?.profileId == bProfileId &&
              next?.authorization ==
                  container
                      .read(authControllerProvider)
                      .session
                      ?.authorizationHeader) {
            if (!initialBHello.isCompleted) {
              recorder.bHelloObserved = true;
              initialHelloGeneration = next!.generation;
              initialBHello.complete(next);
            } else if (next!.generation > initialHelloGeneration &&
                !reconnectBHello.isCompleted) {
              reconnectBHello.complete(next);
            }
          }
        },
        fireImmediately: true,
      );
      addTearDown(helloSubscription.close);

      final initialInboxDone = Completer<void>();
      final reconnectInboxBegan = Completer<void>();
      final reconnectInboxDone = Completer<void>();
      var waitingForReconnectSnapshot = false;
      var reconnectChatResponsesRequired = -1;
      final inboxSubscription = container.listen<InboxReconcilerState>(
        inboxReconcilerProvider,
        (_, state) {
          final snapshot = state.snapshotFor(bProfileId);
          if (snapshot == null) return;
          final complete = InboxScope.values.every(
            (scope) => snapshot[scope].isComplete,
          );
          if (complete && !initialInboxDone.isCompleted) {
            initialInboxDone.complete();
          }
          if (waitingForReconnectSnapshot && !complete) {
            if (!reconnectInboxBegan.isCompleted) {
              reconnectInboxBegan.complete();
            }
          }
          if (waitingForReconnectSnapshot &&
              reconnectInboxBegan.isCompleted &&
              complete &&
              recorder.responseStartedChatRequests.length >=
                  reconnectChatResponsesRequired &&
              !reconnectInboxDone.isCompleted) {
            reconnectInboxDone.complete();
          }
        },
        fireImmediately: true,
      );
      addTearDown(inboxSubscription.close);
      container.read(inboxReconcilerProvider);

      final switched = await container
          .read(profileSwitchCoordinatorProvider)
          .switchTo(bProfileId);
      expect(
        switched,
        isA<ProfileSwitchApplied>(),
        reason: switch (switched) {
          ProfileSwitchRejected(:final errorCode) =>
            'ProfileSwitchRejected($errorCode)',
          _ => '$switched',
        },
      );

      final acceptedBHello = await initialBHello.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out waiting for accepted B hello'),
      );
      expect(acceptedBHello.bindingGeneration, greaterThan(0));
      expect(recorder.chatRequestsBeforeBHello, isEmpty);
      expect(recorder.messageRequestsBeforeBHello, isEmpty);

      await initialInboxDone.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out waiting for B inbox snapshot'),
      );
      final bAuthorization = container
          .read(authControllerProvider)
          .session!
          .authorizationHeader;
      expect(recorder.chatRequests, hasLength(6));
      final bInboxRequests = recorder.chatRequests
          .where((request) => request.authorization == bAuthorization)
          .toList(growable: false);
      expect(bInboxRequests, hasLength(6));
      expect(bInboxRequests.map((request) => request.inbox).toSet(), {
        'main',
        'requests',
        'archive',
      });
      for (final inbox in expectedCursors.keys) {
        final pageRequests = bInboxRequests
            .where((request) => request.inbox == inbox)
            .toList(growable: false);
        expect(pageRequests, hasLength(2));
        expect(
          pageRequests.map((request) => request.uri.queryParameters['cursor']),
          containsAllInOrder([null, expectedCursors[inbox]]),
        );
        expect(
          pageRequests.every(
            (request) => request.uri.queryParameters['page_size'] == '1',
          ),
          isTrue,
        );
      }
      expect(recorder.messageRequests, isEmpty);
      final bSnapshot = container
          .read(inboxReconcilerProvider)
          .snapshotFor(bProfileId)!;
      expect(bSnapshot[InboxScope.main].items, hasLength(2));
      expect(
        bSnapshot[InboxScope.main].items.map((item) => item.chatId).toSet(),
        {selectedChatId, mainAltChatId},
      );
      expect(bSnapshot[InboxScope.requests].items, hasLength(2));
      expect(
        bSnapshot[InboxScope.requests].items.map((item) => item.chatId).toSet(),
        {requestChatId, requestAltChatId},
      );
      expect(bSnapshot[InboxScope.archive].items, hasLength(2));
      expect(
        bSnapshot[InboxScope.archive].items.map((item) => item.chatId).toSet(),
        {archivedChatId, archivedAltChatId},
      );

      container.read(selectedChatIdProvider.notifier).state = selectedChatId;
      final selectedBaselineLoaded = Completer<void>();
      final selectedDeltaLoaded = Completer<void>();
      String? offlineMessageId;
      final selectedRoom = container.listen<ChatRoomState>(
        chatRoomControllerProvider(selectedChatId),
        (_, next) {
          if (next.messages.any((message) => message.id == baselineMessageId) &&
              !selectedBaselineLoaded.isCompleted) {
            selectedBaselineLoaded.complete();
          }
          final deltaId = offlineMessageId;
          if (deltaId != null &&
              next.messages.any((message) => message.id == deltaId) &&
              !selectedDeltaLoaded.isCompleted) {
            selectedDeltaLoaded.complete();
          }
        },
        fireImmediately: true,
      );
      addTearDown(selectedRoom.close);
      final passiveRoom = container.listen<ChatRoomState>(
        chatRoomControllerProvider(archivedChatId),
        (previous, next) {},
        fireImmediately: true,
      );
      addTearDown(passiveRoom.close);

      await recorder.firstMessageRequest.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out loading selected B history'),
      );
      final history = recorder.messageRequests;
      expect(history, hasLength(1));
      expect(history.single.authorization, bAuthorization);
      expect(history.single.uri.queryParameters['chat_id'], selectedChatId);
      expect(history.single.uri.queryParameters['cursor'], isNull);
      expect(history.single.uri.queryParameters['after_message_id'], isNull);
      expect(history.single.uri.queryParameters['last_message_id'], isNull);
      expect(
        history.single.uri.queryParameters['chat_id'],
        isNot(archivedChatId),
        reason: 'mounted passive room must not request history',
      );
      await selectedBaselineLoaded.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out loading selected baseline'),
      );
      expect(
        container
            .read(chatRoomControllerProvider(selectedChatId))
            .lastMessageId,
        baselineMessageId,
      );

      final requestCountBeforeTransportLoss = recorder.requests.length;
      waitingForReconnectSnapshot = true;
      reconnectChatResponsesRequired =
          recorder.responseStartedChatRequests.length + 6;
      await relay.dropInitialClientTransport();
      await relay.secondUpgradeRequested.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out waiting for reconnect transport'),
      );

      final offlineDelta = await ctx.messagesClient().sendMessage(
        authorization: rawMainPeer.authorizationHeader,
        chatId: selectedChatId,
        content: 't055-reconnect-delta',
        clientMessageId: qaClientMessageId(),
      );
      expect(
        offlineDelta,
        isA<MessagesApiOk<VoiceMessage>>(),
        reason: '$offlineDelta',
      );
      offlineMessageId = (offlineDelta as MessagesApiOk<VoiceMessage>).data.id;
      expect(offlineMessageId, isNotEmpty, reason: 'offline selected delta');

      final reconnectCursors = <String, String>{};
      reconnectCursors['main'] = await _expectTwoInboxItems(
        chats: chats,
        authorization: bAuthorization,
        inbox: 'main',
        chatIds: {selectedChatId, mainAltChatId},
      );
      reconnectCursors['requests'] = await _expectTwoInboxItems(
        chats: chats,
        authorization: bAuthorization,
        inbox: 'requests',
        chatIds: {requestChatId, requestAltChatId},
      );
      reconnectCursors['archive'] = await _expectTwoInboxItems(
        chats: chats,
        authorization: bAuthorization,
        inbox: 'archive',
        chatIds: {archivedChatId, archivedAltChatId},
      );

      relay.releaseSecondUpgrade();
      final acceptedReconnectHello = await reconnectBHello.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out waiting for accepted reconnect hello'),
      );
      expect(
        acceptedReconnectHello.generation,
        greaterThan(acceptedBHello.generation),
      );
      expect(
        acceptedReconnectHello.bindingGeneration,
        acceptedBHello.bindingGeneration,
      );
      await reconnectInboxBegan.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out starting reconnect inbox snapshot'),
      );
      await reconnectInboxDone.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out completing reconnect inbox snapshot'),
      );
      await selectedDeltaLoaded.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out loading selected reconnect delta'),
      );

      final reconnectRequests = recorder.requests
          .skip(requestCountBeforeTransportLoss)
          .toList(growable: false);
      final reconnectInboxRequests = reconnectRequests
          .where(
            (request) =>
                request.method == 'GET' &&
                request.uri.path == '/api/v1/chats' &&
                request.authorization == bAuthorization,
          )
          .toList(growable: false);
      expect(reconnectInboxRequests, hasLength(6));
      for (final inbox in reconnectCursors.keys) {
        final pageRequests = reconnectInboxRequests
            .where((request) => request.inbox == inbox)
            .toList(growable: false);
        expect(pageRequests, hasLength(2));
        expect(
          pageRequests.map((request) => request.uri.queryParameters['cursor']),
          containsAllInOrder([null, reconnectCursors[inbox]]),
        );
      }
      final reconnectSnapshot = container
          .read(inboxReconcilerProvider)
          .snapshotFor(bProfileId)!;
      expect(reconnectSnapshot[InboxScope.main].items, hasLength(2));
      expect(
        reconnectSnapshot[InboxScope.main].items
            .map((item) => item.chatId)
            .toSet(),
        {selectedChatId, mainAltChatId},
      );
      expect(reconnectSnapshot[InboxScope.requests].items, hasLength(2));
      expect(
        reconnectSnapshot[InboxScope.requests].items
            .map((item) => item.chatId)
            .toSet(),
        {requestChatId, requestAltChatId},
      );
      expect(reconnectSnapshot[InboxScope.archive].items, hasLength(2));
      expect(
        reconnectSnapshot[InboxScope.archive].items
            .map((item) => item.chatId)
            .toSet(),
        {archivedChatId, archivedAltChatId},
      );
      final reconnectHistory = reconnectRequests
          .where(
            (request) =>
                request.method == 'GET' &&
                request.uri.path == '/api/v1/messages',
          )
          .toList(growable: false);
      expect(reconnectHistory, hasLength(1));
      expect(
        reconnectHistory.single.uri.queryParameters['chat_id'],
        selectedChatId,
      );
      expect(
        reconnectHistory.single.uri.queryParameters['last_message_id'],
        baselineMessageId,
      );
      expect(reconnectHistory.single.authorization, bAuthorization);
      expect(reconnectHistory.single.uri.queryParameters['cursor'], isNull);
      expect(
        reconnectHistory.single.uri.queryParameters['after_message_id'],
        isNull,
      );
      expect(
        reconnectHistory
            .where(
              (request) =>
                  request.uri.queryParameters['chat_id'] == archivedChatId,
            )
            .isEmpty,
        isTrue,
        reason: 'mounted passive room must not catch up after reconnect',
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}

Future<String> _expectTwoInboxItems({
  required VoiceChatsClient chats,
  required String authorization,
  required String inbox,
  required Set<String> chatIds,
}) async {
  final first = await chats.listChats(
    authorization: authorization,
    inbox: inbox,
    pageSize: 1,
  );
  expect(first, isA<ChatsApiOk<ChatListData>>(), reason: '$first');
  final firstPage = (first as ChatsApiOk<ChatListData>).data;
  expect(firstPage.items, hasLength(1));
  final cursor = firstPage.nextCursor;
  expect(cursor, isNotNull, reason: '$inbox first cursor');
  expect(cursor, isNotEmpty, reason: '$inbox first cursor');

  final second = await chats.listChats(
    authorization: authorization,
    inbox: inbox,
    pageSize: 1,
    cursor: cursor,
  );
  expect(second, isA<ChatsApiOk<ChatListData>>(), reason: '$second');
  final secondPage = (second as ChatsApiOk<ChatListData>).data;
  expect(secondPage.items, hasLength(1));
  expect(secondPage.nextCursor, isNull, reason: '$inbox second cursor');
  expect({
    ...firstPage.items.map((item) => item.chatId),
    ...secondPage.items.map((item) => item.chatId),
  }, chatIds);
  return cursor!;
}

class _RecordedRequest {
  const _RecordedRequest({
    required this.method,
    required this.uri,
    required this.authorization,
    required this.beforeBHello,
  });

  final String method;
  final Uri uri;
  final String? authorization;
  final bool beforeBHello;

  String? get inbox => uri.queryParameters['inbox'];
}

class _RecordingHttpClient extends http.BaseClient {
  _RecordingHttpClient(this._delegate);

  final http.Client _delegate;
  final requests = <_RecordedRequest>[];
  final responseStartedRequests = <_RecordedRequest>[];
  final firstMessageRequest = Completer<void>();
  var bHelloObserved = false;

  Iterable<_RecordedRequest> get chatRequests => requests.where(
    (request) => request.method == 'GET' && request.uri.path == '/api/v1/chats',
  );

  Iterable<_RecordedRequest> get messageRequests => requests.where(
    (request) =>
        request.method == 'GET' && request.uri.path == '/api/v1/messages',
  );

  Iterable<_RecordedRequest> get chatRequestsBeforeBHello =>
      chatRequests.where((request) => request.beforeBHello);

  Iterable<_RecordedRequest> get messageRequestsBeforeBHello =>
      messageRequests.where((request) => request.beforeBHello);

  Iterable<_RecordedRequest> get responseStartedChatRequests =>
      responseStartedRequests.where(
        (request) =>
            request.method == 'GET' && request.uri.path == '/api/v1/chats',
      );

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final effectiveRequest = _withOneRowInboxPage(request);
    final recorded = _RecordedRequest(
      method: effectiveRequest.method,
      uri: effectiveRequest.url,
      authorization: _header(effectiveRequest.headers, 'Authorization'),
      beforeBHello: !bHelloObserved,
    );
    requests.add(recorded);
    if (effectiveRequest.method == 'GET' &&
        effectiveRequest.url.path == '/api/v1/messages') {
      if (!firstMessageRequest.isCompleted) firstMessageRequest.complete();
    }
    final response = await _delegate.send(effectiveRequest);
    responseStartedRequests.add(recorded);
    return response;
  }

  http.BaseRequest _withOneRowInboxPage(http.BaseRequest request) {
    if (request.method != 'GET' || request.url.path != '/api/v1/chats') {
      return request;
    }
    return http.Request(
      request.method,
      request.url.replace(
        queryParameters: {...request.url.queryParameters, 'page_size': '1'},
      ),
    )..headers.addAll(request.headers);
  }

  // The shared live harness owns the underlying client for this test.
  @override
  void close() {}

  String? _header(Map<String, String> headers, String name) {
    for (final entry in headers.entries) {
      if (entry.key.toLowerCase() == name.toLowerCase()) return entry.value;
    }
    return null;
  }
}

/// A VM-only relay. It does not inspect or synthesize WebSocket frames: each
/// accepted client socket gets a real Gateway upstream and both directions are
/// forwarded unchanged. Holding the second *upgrade* keeps the Hub offline
/// while the fixture writes its missed durable message.
class _LiveWebSocketRelay {
  _LiveWebSocketRelay._(this._server) {
    _requests = _server.listen((request) {
      final handler = _handle(request);
      _handlers.add(handler);
      unawaited(
        handler.catchError((Object _) {}).whenComplete(() {
          _handlers.remove(handler);
        }),
      );
    });
  }

  static const _teardownTimeout = Duration(seconds: 3);
  final HttpServer _server;
  late final StreamSubscription<HttpRequest> _requests;
  final _firstPair = Completer<_RelaySocketPair>();
  final _secondUpgradeRequested = Completer<void>();
  final _releaseSecondUpgrade = Completer<void>();
  final _pairs = <_RelaySocketPair>[];
  final _handlers = <Future<void>>[];
  Uri? _upstreamUri;
  var _connectionAttempts = 0;

  static Future<_LiveWebSocketRelay> bind() async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    return _LiveWebSocketRelay._(server);
  }

  Uri get clientUri => Uri(
    scheme: 'ws',
    host: InternetAddress.loopbackIPv4.address,
    port: _server.port,
    path: '/ws',
  );

  Future<void> get secondUpgradeRequested => _secondUpgradeRequested.future;

  void setUpstream(Uri uri) {
    final current = _upstreamUri;
    if (current != null && current != uri) {
      throw StateError(
        'relay upstream changed during one test: $current -> $uri',
      );
    }
    _upstreamUri = uri;
  }

  Future<void> dropInitialClientTransport() async {
    final pair = await _firstPair.future;
    await pair.closeClientTransport();
  }

  void releaseSecondUpgrade() {
    if (!_releaseSecondUpgrade.isCompleted) _releaseSecondUpgrade.complete();
  }

  Future<void> _handle(HttpRequest request) async {
    if (!WebSocketTransformer.isUpgradeRequest(request)) {
      request.response.statusCode = HttpStatus.badRequest;
      await request.response.close();
      return;
    }
    final attempt = ++_connectionAttempts;
    if (attempt == 2) {
      if (!_secondUpgradeRequested.isCompleted) {
        _secondUpgradeRequested.complete();
      }
      await _releaseSecondUpgrade.future;
    }
    final upstreamUri = _upstreamUri;
    if (upstreamUri == null) {
      request.response.statusCode = HttpStatus.serviceUnavailable;
      await request.response.close();
      return;
    }
    final client = await WebSocketTransformer.upgrade(request);
    WebSocket upstream;
    try {
      upstream = await WebSocket.connect(
        upstreamUri.toString(),
        headers: _forwardedHeaders(request.headers),
      );
    } on Object {
      await _settle(
        client.close(
          WebSocketStatus.internalServerError,
          'relay upstream unavailable',
        ),
      );
      rethrow;
    }
    final pair = _RelaySocketPair(client: client, upstream: upstream);
    _pairs.add(pair);
    pair.start();
    if (attempt == 1 && !_firstPair.isCompleted) _firstPair.complete(pair);
  }

  Map<String, String> _forwardedHeaders(HttpHeaders headers) {
    const names = ['authorization', 'x-voice-profile-id', 'x-request-id'];
    return {
      for (final name in names)
        if (headers[name] case final values? when values.isNotEmpty)
          name: values.join(','),
    };
  }

  Future<void> dispose() async {
    releaseSecondUpgrade();
    await _requests.cancel();
    await _server.close(force: true);
    await _settle(Future.wait(_handlers.toList(growable: false)));
    await _settle(
      Future.wait([
        for (final pair in _pairs.toList(growable: false)) pair.dispose(),
      ]),
    );
  }

  static Future<void> _settle(Future<void> future) async {
    try {
      await future.timeout(_teardownTimeout);
    } on TimeoutException {
      // Teardown must not leave a held reconnect upgrade blocking this test.
    } on Object {
      // Socket shutdown after an intentional transport loss is best-effort.
    }
  }
}

class _RelayRealtimeTransportFactory implements RealtimeTransportFactory {
  _RelayRealtimeTransportFactory(this._relay);

  final _LiveWebSocketRelay _relay;

  @override
  Future<VoiceRealtimeConnection> open({
    required Uri uri,
    required AuthSession session,
  }) async {
    _relay.setUpstream(uri);
    return VoiceRealtimeConnection(
      uri: _relay.clientUri,
      headers: {
        'Authorization': session.authorizationHeader,
        'X-Voice-Profile-Id': session.activeProfileId,
        'X-Request-Id': newGatewayRequestId(),
      },
    );
  }
}

class _RelaySocketPair {
  _RelaySocketPair({required this.client, required this.upstream});

  final WebSocket client;
  final WebSocket upstream;
  final clientClosed = Completer<void>();
  StreamSubscription<dynamic>? _clientFrames;
  StreamSubscription<dynamic>? _upstreamFrames;
  var _disposed = false;

  void start() {
    _clientFrames = client.listen(
      upstream.add,
      onDone: () {
        if (!clientClosed.isCompleted) clientClosed.complete();
        unawaited(upstream.close());
      },
      onError: (error, stackTrace) {
        if (!clientClosed.isCompleted) clientClosed.complete();
        unawaited(upstream.close());
      },
    );
    _upstreamFrames = upstream.listen(
      client.add,
      onDone: () => unawaited(client.close()),
      onError: (error, stackTrace) => unawaited(client.close()),
    );
  }

  Future<void> closeClientTransport() async {
    await client.close(WebSocketStatus.goingAway, 'test transport loss');
    await clientClosed.future;
  }

  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    await _LiveWebSocketRelay._settle(
      Future.wait([
        _clientFrames?.cancel() ?? Future<void>.value(),
        _upstreamFrames?.cancel() ?? Future<void>.value(),
      ]),
    );
    await _LiveWebSocketRelay._settle(client.close());
    await _LiveWebSocketRelay._settle(upstream.close());
  }
}
