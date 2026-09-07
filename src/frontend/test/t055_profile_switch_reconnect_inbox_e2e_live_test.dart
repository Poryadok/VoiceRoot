import 'dart:async';
import 'dart:convert';
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

      final selectedDm = await _createDmWhenAvailable(
        chats: chats,
        authorization: rawB.authorizationHeader,
        otherProfileId: mainPeer.activeProfileId,
      );
      expect(selectedDm, isA<ChatsApiOk<VoiceChat>>(), reason: '$selectedDm');
      final selectedChatId = (selectedDm as ChatsApiOk<VoiceChat>).data.id;
      final mainAltDm = await _createDmWhenAvailable(
        chats: chats,
        authorization: rawB.authorizationHeader,
        otherProfileId: mainAltProfileId,
      );
      expect(mainAltDm, isA<ChatsApiOk<VoiceChat>>(), reason: '$mainAltDm');
      final mainAltChatId = (mainAltDm as ChatsApiOk<VoiceChat>).data.id;

      final archivedDm = await _createDmWhenAvailable(
        chats: chats,
        authorization: rawB.authorizationHeader,
        otherProfileId: archivePeer.activeProfileId,
      );
      expect(archivedDm, isA<ChatsApiOk<VoiceChat>>(), reason: '$archivedDm');
      final archivedChatId = (archivedDm as ChatsApiOk<VoiceChat>).data.id;
      final archivedAltDm = await _createDmWhenAvailable(
        chats: chats,
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

      final requestDm = await _createDmWhenAvailable(
        chats: chats,
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
      final requestAltDm = await _createDmWhenAvailable(
        chats: chats,
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
          inboxReconcilerProvider.overrideWith(
            (ref) => _TaggedInboxReconcilerController(ref),
          ),
          realtimeTransportFactoryProvider.overrideWithValue(
            _RelayRealtimeTransportFactory(relay),
          ),
          // Suppressing only automatic initial-A connection makes the
          // coordinator handoff the sole initial live WS action in this test.
          realtimeAutoConnectProvider.overrideWithValue(false),
        ],
      );
      addTearDown(() async {
        await recorder.settle().timeout(
          const Duration(seconds: 3),
          onTimeout: () => throw TestFailure(
            'timed out draining recorder: ${recorder.settleDiagnostic}',
          ),
        );
        container.dispose();
      });

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
      final reconnectMainPageFailed = Completer<void>();
      final reconnectHealthyScopesDone = Completer<void>();
      var waitingForReconnectSnapshot = false;
      final reconnectStartedScopes = <InboxScope>{};
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
          if (waitingForReconnectSnapshot) {
            for (final scope in InboxScope.values) {
              if (!snapshot[scope].isComplete)
                reconnectStartedScopes.add(scope);
            }
          }
          final main = snapshot[InboxScope.main];
          if (waitingForReconnectSnapshot &&
              main.hasError &&
              main.errorStatusCode == HttpStatus.serviceUnavailable &&
              !reconnectMainPageFailed.isCompleted) {
            reconnectMainPageFailed.complete();
          }
          if (waitingForReconnectSnapshot &&
              reconnectStartedScopes.contains(InboxScope.requests) &&
              reconnectStartedScopes.contains(InboxScope.archive) &&
              snapshot[InboxScope.requests].isComplete &&
              snapshot[InboxScope.archive].isComplete &&
              !reconnectHealthyScopesDone.isCompleted) {
            reconnectHealthyScopesDone.complete();
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
      final bInboxRequests = recorder.chatRequests
          .where(
            (request) =>
                request.isInboxReconciliation &&
                request.authorization == bAuthorization,
          )
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
      await recorder
          .waitForCompletedReadResponses(1)
          .timeout(
            const Duration(seconds: 12),
            onTimeout: () => throw TestFailure(
              'timed out completing selected baseline read',
            ),
          );

      final requestCountBeforeTransportLoss = recorder.requests.length;
      final completedReadsBeforeReconnect = recorder.completedReadResponses;
      waitingForReconnectSnapshot = true;
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
      recorder.failNextInboxReconciliationPage(
        authorization: bAuthorization,
        inbox: 'main',
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
      await reconnectMainPageFailed.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out failing reconnect main page 2'),
      );
      await reconnectHealthyScopesDone.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () => throw TestFailure(
          'timed out completing healthy reconnect inbox scopes',
        ),
      );
      final currentReconnectHello = container.read(
        realtimeHelloBindingProvider,
      );
      expect(currentReconnectHello, isNotNull);
      expect(
        currentReconnectHello!.generation,
        acceptedReconnectHello.generation,
      );
      expect(
        currentReconnectHello.bindingGeneration,
        acceptedReconnectHello.bindingGeneration,
      );
      expect(currentReconnectHello.profileId, bProfileId);
      expect(currentReconnectHello.authorization, bAuthorization);

      final reconnectRequests = recorder.requests
          .skip(requestCountBeforeTransportLoss)
          .toList(growable: false);
      final reconnectInboxRequests = reconnectRequests
          .where(
            (request) =>
                request.method == 'GET' &&
                request.uri.path == '/api/v1/chats' &&
                request.isInboxReconciliation &&
                request.authorization == bAuthorization,
          )
          .toList(growable: false);
      expect(
        reconnectInboxRequests,
        hasLength(6),
        reason: _reconnectRequestDiagnostic(reconnectRequests),
      );
      expect(
        reconnectInboxRequests.every(
          (request) => request.uri.queryParameters['page_size'] == '1',
        ),
        isTrue,
      );
      for (final inbox in reconnectCursors.keys.where(
        (inbox) => inbox != 'main',
      )) {
        final pageRequests = reconnectInboxRequests
            .where((request) => request.inbox == inbox)
            .toList(growable: false);
        expect(pageRequests, hasLength(2));
        expect(
          pageRequests.map((request) => request.uri.queryParameters['cursor']),
          containsAllInOrder([null, reconnectCursors[inbox]]),
        );
      }
      final reconnectMainRequests = reconnectInboxRequests
          .where((request) => request.inbox == 'main')
          .toList(growable: false);
      expect(
        reconnectMainRequests.map(
          (request) => request.uri.queryParameters['cursor'],
        ),
        containsAllInOrder([null, reconnectCursors['main']]),
      );
      expect(recorder.injectedInboxFailures, 1);
      final failedReconnectSnapshot = container
          .read(inboxReconcilerProvider)
          .snapshotFor(bProfileId)!;
      final failedMain = failedReconnectSnapshot[InboxScope.main];
      expect(failedMain.items, hasLength(2));
      expect(failedMain.items.map((item) => item.chatId).toSet(), {
        selectedChatId,
        mainAltChatId,
      });
      expect(failedMain.isLoading, isFalse);
      expect(failedMain.isComplete, isFalse);
      expect(failedMain.hasError, isTrue);
      expect(failedMain.errorStatusCode, HttpStatus.serviceUnavailable);
      expect(failedMain.failedCursor, reconnectCursors['main']);
      expect(failedMain.nextCursor, reconnectCursors['main']);
      expect(failedReconnectSnapshot[InboxScope.requests].isComplete, isTrue);
      expect(failedReconnectSnapshot[InboxScope.requests].hasError, isFalse);
      expect(failedReconnectSnapshot[InboxScope.archive].isComplete, isTrue);
      expect(failedReconnectSnapshot[InboxScope.archive].hasError, isFalse);
      expect(failedReconnectSnapshot[InboxScope.requests].items, hasLength(2));
      expect(
        failedReconnectSnapshot[InboxScope.requests].items
            .map((item) => item.chatId)
            .toSet(),
        {requestChatId, requestAltChatId},
      );
      expect(failedReconnectSnapshot[InboxScope.archive].items, hasLength(2));
      expect(
        failedReconnectSnapshot[InboxScope.archive].items
            .map((item) => item.chatId)
            .toSet(),
        {archivedChatId, archivedAltChatId},
      );

      await Future<void>.microtask(() {});
      final mainCursorsBeforeExplicitRetry = recorder.requests
          .skip(requestCountBeforeTransportLoss)
          .where(
            (request) =>
                request.isInboxReconciliation &&
                request.authorization == bAuthorization &&
                request.inbox == 'main',
          )
          .map((request) => request.uri.queryParameters['cursor'])
          .toList(growable: false);
      expect(
        mainCursorsBeforeExplicitRetry,
        orderedEquals([null, reconnectCursors['main']]),
        reason: 'a failed page must not retry before explicit user action',
      );

      await container
          .read(inboxReconcilerProvider.notifier)
          .retry(InboxScope.main);
      final completedReconnectSnapshot = container
          .read(inboxReconcilerProvider)
          .snapshotFor(bProfileId)!;
      final completedMain = completedReconnectSnapshot[InboxScope.main];
      expect(completedMain.items, hasLength(2));
      expect(completedMain.items.map((item) => item.chatId).toSet(), {
        selectedChatId,
        mainAltChatId,
      });
      expect(completedMain.isComplete, isTrue);
      expect(completedMain.hasError, isFalse);
      expect(completedMain.nextCursor, isNull);
      expect(completedMain.failedCursor, isNull);
      final mainCursorsAfterExplicitRetry = recorder.requests
          .skip(requestCountBeforeTransportLoss)
          .where(
            (request) =>
                request.isInboxReconciliation &&
                request.authorization == bAuthorization &&
                request.inbox == 'main',
          )
          .map((request) => request.uri.queryParameters['cursor'])
          .toList(growable: false);
      expect(
        mainCursorsAfterExplicitRetry,
        orderedEquals([
          null,
          reconnectCursors['main'],
          reconnectCursors['main'],
        ]),
      );
      await selectedDeltaLoaded.future.timeout(
        const Duration(seconds: 12),
        onTimeout: () =>
            throw TestFailure('timed out loading selected reconnect delta'),
      );
      await recorder
          .waitForCompletedReadResponses(completedReadsBeforeReconnect + 1)
          .timeout(
            const Duration(seconds: 12),
            onTimeout: () => throw TestFailure(
              'timed out completing selected reconnect read',
            ),
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
    timeout: const Timeout(Duration(minutes: 2)),
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

const _inboxReconciliationZoneKey = #t055InboxReconciliation;

String _reconnectRequestDiagnostic(Iterable<_RecordedRequest> requests) {
  final paths = requests
      .map(
        (request) =>
            '${request.requestOrigin}:${request.uri.path}?inbox=${request.inbox}&cursor=${request.uri.queryParameters['cursor']}&page_size=${request.uri.queryParameters['page_size']}',
      )
      .join(', ');
  return 'reconnect inbox requests: $paths';
}

class _TaggedInboxReconcilerController extends InboxReconcilerController {
  _TaggedInboxReconcilerController(super.ref) : super(pageSize: 1);

  @override
  Future<void> reconcile() {
    return _runTagged(super.reconcile);
  }

  @override
  Future<void> retry(InboxScope scope) {
    return _runTagged(() => super.retry(scope));
  }

  Future<void> _runTagged(Future<void> Function() action) {
    return runZoned(action, zoneValues: {_inboxReconciliationZoneKey: true});
  }
}

class _RecordedRequest {
  const _RecordedRequest({
    required this.method,
    required this.uri,
    required this.authorization,
    required this.beforeBHello,
    required this.isInboxReconciliation,
  });

  final String method;
  final Uri uri;
  final String? authorization;
  final bool beforeBHello;
  final bool isInboxReconciliation;

  String? get inbox => uri.queryParameters['inbox'];

  String get requestOrigin =>
      isInboxReconciliation ? 'inbox-reconciler' : 'other';
}

class _RecordingHttpClient extends http.BaseClient {
  _RecordingHttpClient(this._delegate);

  final http.Client _delegate;
  final requests = <_RecordedRequest>[];
  final responseStartedRequests = <_RecordedRequest>[];
  final responseCompletedRequests = <_RecordedRequest>[];
  final firstMessageRequest = Completer<void>();
  final _readResponseWaiters = <Completer<void>>[];
  final _pendingReadResponseCompletions = <Object>{};
  final _settleWaiters = <Completer<void>>[];
  _InboxPageFailure? _nextInboxPageFailure;
  var _activeRequests = 0;
  var bHelloObserved = false;
  var injectedInboxFailures = 0;

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

  Iterable<_RecordedRequest>
  get responseStartedInboxReconciliationChatRequests =>
      responseStartedChatRequests.where(
        (request) => request.isInboxReconciliation,
      );

  int get completedReadResponses => responseCompletedRequests
      .where(
        (request) =>
            request.method == 'POST' &&
            request.uri.path == '/api/v1/messages/read',
      )
      .length;

  String get settleDiagnostic =>
      'active=$_activeRequests pending_read=${_pendingReadResponseCompletions.length}';

  Future<void> waitForCompletedReadResponses(int requiredCount) async {
    while (completedReadResponses < requiredCount) {
      final waiter = Completer<void>();
      _readResponseWaiters.add(waiter);
      await waiter.future;
    }
  }

  Future<void> settle() async {
    while (_activeRequests > 0 || _pendingReadResponseCompletions.isNotEmpty) {
      final waiter = Completer<void>();
      _settleWaiters.add(waiter);
      await waiter.future;
    }
    await Future<void>.microtask(() {});
  }

  void failNextInboxReconciliationPage({
    required String authorization,
    required String inbox,
  }) {
    if (_nextInboxPageFailure != null) {
      throw StateError('an inbox page failure is already armed');
    }
    _nextInboxPageFailure = _InboxPageFailure(
      authorization: authorization,
      inbox: inbox,
    );
  }

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    _activeRequests++;
    try {
      final effectiveRequest = request;
      final recorded = _RecordedRequest(
        method: effectiveRequest.method,
        uri: effectiveRequest.url,
        authorization: _header(effectiveRequest.headers, 'Authorization'),
        beforeBHello: !bHelloObserved,
        isInboxReconciliation:
            Zone.current[_inboxReconciliationZoneKey] == true,
      );
      requests.add(recorded);
      if (effectiveRequest.method == 'GET' &&
          effectiveRequest.url.path == '/api/v1/messages') {
        if (!firstMessageRequest.isCompleted) firstMessageRequest.complete();
      }
      final pendingFailure = _nextInboxPageFailure;
      if (pendingFailure != null && pendingFailure.matches(recorded)) {
        _nextInboxPageFailure = null;
        injectedInboxFailures++;
        responseStartedRequests.add(recorded);
        return http.StreamedResponse(
          http.ByteStream.fromBytes(
            utf8.encode('{"error":"t055 injected page failure"}'),
          ),
          HttpStatus.serviceUnavailable,
          request: effectiveRequest,
          headers: const {'content-type': 'application/json'},
        );
      }
      final response = await _delegate.send(effectiveRequest);
      responseStartedRequests.add(recorded);
      if (recorded.method != 'POST' ||
          recorded.uri.path != '/api/v1/messages/read') {
        return response;
      }
      final readResponse = Object();
      _pendingReadResponseCompletions.add(readResponse);
      return http.StreamedResponse(
        response.stream.transform(
          StreamTransformer.fromHandlers(
            handleDone: (sink) {
              _finishReadResponse(recorded, readResponse);
              sink.close();
            },
            handleError: (error, stackTrace, sink) {
              _finishReadResponse(
                recorded,
                readResponse,
                error: error,
                stackTrace: stackTrace,
              );
              sink.addError(error, stackTrace);
            },
          ),
        ),
        response.statusCode,
        contentLength: response.contentLength,
        request: response.request,
        headers: response.headers,
        isRedirect: response.isRedirect,
        persistentConnection: response.persistentConnection,
        reasonPhrase: response.reasonPhrase,
      );
    } finally {
      _activeRequests--;
      _notifySettled();
    }
  }

  void _notifySettled() {
    if (_activeRequests > 0 || _pendingReadResponseCompletions.isNotEmpty) {
      return;
    }
    for (final waiter in _settleWaiters) {
      if (!waiter.isCompleted) waiter.complete();
    }
    _settleWaiters.clear();
  }

  void _finishReadResponse(
    _RecordedRequest recorded,
    Object readResponse, {
    Object? error,
    StackTrace? stackTrace,
  }) {
    if (!_pendingReadResponseCompletions.remove(readResponse)) return;
    if (error == null) {
      responseCompletedRequests.add(recorded);
      for (final waiter in _readResponseWaiters) {
        if (!waiter.isCompleted) waiter.complete();
      }
    } else {
      for (final waiter in _readResponseWaiters) {
        if (!waiter.isCompleted) waiter.completeError(error, stackTrace);
      }
    }
    _readResponseWaiters.clear();
    _notifySettled();
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

class _InboxPageFailure {
  const _InboxPageFailure({required this.authorization, required this.inbox});

  final String authorization;
  final String inbox;

  bool matches(_RecordedRequest request) {
    return request.isInboxReconciliation &&
        request.method == 'GET' &&
        request.uri.path == '/api/v1/chats' &&
        request.authorization == authorization &&
        request.inbox == inbox &&
        request.uri.queryParameters['cursor'] != null &&
        request.uri.queryParameters['page_size'] == '1';
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

Future<ChatsApiResult<VoiceChat>> _createDmWhenAvailable({
  required VoiceChatsClient chats,
  required String authorization,
  required String otherProfileId,
}) async {
  final deadline = DateTime.now().add(const Duration(seconds: 30));
  while (true) {
    final result = await chats.createDm(
      authorization: authorization,
      otherProfileId: otherProfileId,
    );
    if (result is! ChatsApiFailure ||
        result.statusCode != HttpStatus.internalServerError ||
        !DateTime.now().isBefore(deadline)) {
      return result;
    }
    await Future<void>.delayed(const Duration(milliseconds: 250));
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
  var _clientTerminated = false;
  var _upstreamTerminated = false;
  var _clientCloseInitiated = false;
  var _upstreamCloseInitiated = false;

  void start() {
    _clientFrames = client.listen(
      _forwardToUpstream,
      onDone: _onClientTerminal,
      onError: (_, _) => _onClientTerminal(),
    );
    _upstreamFrames = upstream.listen(
      _forwardToClient,
      onDone: _onUpstreamTerminal,
      onError: (_, _) => _onUpstreamTerminal(),
    );
  }

  void _forwardToUpstream(dynamic frame) {
    if (_disposed || _upstreamTerminated || _upstreamCloseInitiated) return;
    try {
      upstream.add(frame);
    } on StateError {
      _onUpstreamTerminal();
    }
  }

  void _forwardToClient(dynamic frame) {
    if (_disposed || _clientTerminated || _clientCloseInitiated) return;
    try {
      client.add(frame);
    } on StateError {
      _onClientTerminal();
    }
  }

  void _onClientTerminal() {
    if (_clientTerminated) return;
    _clientTerminated = true;
    if (!clientClosed.isCompleted) clientClosed.complete();
    // A WebSocket can close synchronously while the first subscription is
    // being installed. Defer peer shutdown until start() installs both sides.
    scheduleMicrotask(() => unawaited(_closeUpstream()));
  }

  void _onUpstreamTerminal() {
    if (_upstreamTerminated) return;
    _upstreamTerminated = true;
    scheduleMicrotask(() => unawaited(_closeClient()));
  }

  Future<void> _closeUpstream() async {
    if (_disposed || _upstreamCloseInitiated) return;
    _upstreamCloseInitiated = true;
    await _LiveWebSocketRelay._settle(upstream.close());
  }

  Future<void> _closeClient() async {
    if (_disposed || _clientCloseInitiated) return;
    _clientCloseInitiated = true;
    await _LiveWebSocketRelay._settle(client.close());
  }

  Future<void> closeClientTransport() async {
    await client.close(WebSocketStatus.goingAway, 'test transport loss');
    await clientClosed.future;
  }

  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    _clientTerminated = true;
    _upstreamTerminated = true;
    _clientCloseInitiated = true;
    _upstreamCloseInitiated = true;
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
