import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/live_gateway_harness.dart';

void main() {
  _T106LiveTestBinding();

  testWidgets('T106 live binding permits native loopback HTTP', (tester) async {
    final statusCode = await tester.runAsync(() async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      try {
        final served = server.first.then((request) async {
          request.response.statusCode = HttpStatus.noContent;
          await request.response.close();
        });
        final response = await http
            .get(Uri.parse('http://${server.address.address}:${server.port}/'))
            .timeout(const Duration(seconds: 10));
        await served.timeout(const Duration(seconds: 10));
        return response.statusCode;
      } finally {
        await server.close(force: true);
      }
    });

    expect(statusCode, HttpStatus.noContent);
  });

  testWidgets(
    'account soft-delete revokes sessions, hides fresh DM, and keeps one local marker',
    (tester) async {
      late ProviderContainer container;
      late void Function() closeRoomSubscription;
      late InMemoryMessageCacheStore cache;
      late AuthSession firstA;
      late AuthSession secondA;
      late AuthSession b;
      late VoiceChat dm;
      late VoiceAuthClient authClient;
      late VoiceChatsClient chatsClient;
      late VoiceMessagesClient messages;
      late _RecordingHttpClient recorder;
      late ChatRoomController room;

      await tester.runAsync(() async {
        final probe = await probeLiveGateway();
        expect(
          probe,
          isA<LiveGatewayReady>(),
          reason: probe is LiveGatewayUnavailable ? probe.reason : null,
        );
        final ctx = (probe as LiveGatewayReady).context;
        authClient = ctx.authClient();
        chatsClient = ctx.chatsClient();

        final emailA = qaUniqueEmail('t106-deleted-a');
        final registeredA = await authClient.register(
          email: emailA,
          password: qaPassword,
        );
        expect(registeredA, isA<AuthSessionOk>(), reason: '$registeredA');
        firstA = (registeredA as AuthSessionOk).session;
        await ctx.allowOpenGamingPrivacy(firstA);

        final loggedInA = await authClient.login(
          email: emailA,
          password: qaPassword,
        );
        expect(loggedInA, isA<AuthSessionOk>(), reason: '$loggedInA');
        secondA = (loggedInA as AuthSessionOk).session;
        expect(secondA.accessToken, isNot(firstA.accessToken));

        b = await ctx.registerUser('t106-surviving-b');
        dm = await ctx.createDmBetween(firstA, b);
        messages = ctx.messagesClient();
        for (var index = 1; index <= 2; index++) {
          final baselineSent = await messages.sendMessage(
            authorization: firstA.authorizationHeader,
            chatId: dm.id,
            content:
                't106 baseline $index ${DateTime.now().microsecondsSinceEpoch}',
            clientMessageId: qaClientMessageId(),
          );
          expect(
            baselineSent,
            isA<MessagesApiOk<VoiceMessage>>(),
            reason: '$baselineSent',
          );
        }

        recorder = _RecordingHttpClient(ctx.httpClient);
        final storage = InMemoryAuthSessionStorage();
        cache = InMemoryMessageCacheStore();
        final authController = AuthController(
          authClient: VoiceAuthClient(
            gateway: GatewayHttpClient(
              httpClient: recorder,
              config: ctx.config,
            ),
          ),
          storage: storage,
          guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
        )..state = AuthState(session: b);
        container = ProviderContainer(
          overrides: [
            authControllerProvider.overrideWith((_) => authController),
            authSessionStorageProvider.overrideWithValue(storage),
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            gatewayConfigProvider.overrideWithValue(ctx.config),
            httpClientProvider.overrideWithValue(recorder),
            messageCacheStoreProvider.overrideWithValue(cache),
            realtimeAutoConnectProvider.overrideWithValue(false),
          ],
        );
        container.read(selectedChatIdProvider.notifier).state =
            't106-not-yet-selected';
        final initialHistoryLoaded = Completer<void>();
        final roomSubscription = container.listen<ChatRoomState>(
          chatRoomControllerProvider(dm.id),
          (_, state) {
            if (!initialHistoryLoaded.isCompleted &&
                state.messages.length == 1 &&
                state.hasMore &&
                state.nextCursor != null &&
                state.nextCursor!.isNotEmpty) {
              initialHistoryLoaded.complete();
            }
          },
          fireImmediately: true,
        );
        closeRoomSubscription = roomSubscription.close;
        room = container.read(chatRoomControllerProvider(dm.id).notifier);
        container.read(selectedChatIdProvider.notifier).state = dm.id;
        await initialHistoryLoaded.future.timeout(const Duration(seconds: 20));
      });
      addTearDown(container.dispose);
      addTearDown(closeRoomSubscription);
      await tester.pump();

      final beforeDelete = container.read(chatRoomControllerProvider(dm.id));
      expect(beforeDelete.messages, hasLength(1));
      final baselineMessages = beforeDelete.messages
          .map((m) => m.toJson())
          .toList();
      final baselineCursor = beforeDelete.nextCursor;
      final baselineHasMore = beforeDelete.hasMore;
      expect(baselineCursor, isNotNull);
      expect(baselineCursor, isNotEmpty);
      expect(baselineHasMore, isTrue);
      final baselineCache = (await cache.getMessages(
        profileId: b.activeProfileId,
        chatId: dm.id,
      )).map((m) => m.toJson()).toList();
      expect(baselineCache, baselineMessages);
      expect(recorder.messageGetCountFor(dm.id), 1);

      await tester.runAsync(() async {
        final deleted = await authClient.deleteAccount(
          session: firstA,
          password: qaPassword,
        );
        expect(deleted, isA<AuthApiOk<void>>(), reason: '$deleted');

        final oldSessionInbox = await chatsClient.listChats(
          authorization: secondA.authorizationHeader,
        );
        expect(
          oldSessionInbox,
          isA<ChatsApiFailure>(),
          reason: '$oldSessionInbox',
        );
        expect((oldSessionInbox as ChatsApiFailure).statusCode, 401);

        final deletedSender = await messages.sendMessage(
          authorization: secondA.authorizationHeader,
          chatId: dm.id,
          content: 't106 deleted sender must fail',
          clientMessageId: qaClientMessageId(),
        );
        expect(
          deletedSender,
          isA<MessagesApiFailure>(),
          reason: '$deletedSender',
        );
        expect((deletedSender as MessagesApiFailure).statusCode, 401);

        final survivingPeer = await messages.sendMessage(
          authorization: b.authorizationHeader,
          chatId: dm.id,
          content: 't106 surviving peer must fail',
          clientMessageId: qaClientMessageId(),
        );
        expect(
          survivingPeer,
          isA<MessagesApiFailure>(),
          reason: '$survivingPeer',
        );
        expect((survivingPeer as MessagesApiFailure).statusCode, 403);

        await container.read(inboxReconcilerProvider.notifier).reconcile();
        final freshInbox = container
            .read(inboxReconcilerProvider)
            .snapshotFor(b.activeProfileId);
        expect(freshInbox, isNotNull);
        final snapshot = freshInbox!;
        for (final scope in InboxScope.values) {
          expect(snapshot[scope].isComplete, isTrue);
        }
        expect(
          snapshot[InboxScope.main].items.any((item) => item.chatId == dm.id),
          isFalse,
        );

        for (var observation = 1; observation <= 2; observation++) {
          await room.loadInitial();
          final afterDelete = container.read(chatRoomControllerProvider(dm.id));
          expect(afterDelete.isDmPeerDeleted, isTrue);
          expect(
            afterDelete.messages.map((m) => m.toJson()).toList(),
            baselineMessages,
          );
          expect(afterDelete.nextCursor, baselineCursor);
          expect(afterDelete.hasMore, baselineHasMore);
          expect(
            (await cache.getMessages(
              profileId: b.activeProfileId,
              chatId: dm.id,
            )).map((m) => m.toJson()).toList(),
            baselineCache,
          );
          expect(recorder.messageGetCountFor(dm.id), observation + 1);
        }
      });
      await tester.pump();

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(body: ChatRoomPanel(chatId: dm.id)),
          ),
        ),
      );
      await tester.pump();
      final marker = find.byKey(
        const ValueKey<String>('chat_room_dm_peer_deleted'),
      );
      expect(marker, findsOneWidget);
      expect(
        find.descendant(of: marker, matching: find.text('User deleted')),
        findsOneWidget,
      );
      expect(
        container
            .read(chatRoomControllerProvider(dm.id))
            .messages
            .map((m) => m.toJson())
            .toList(),
        baselineMessages,
      );
      expect(
        (await cache.getMessages(
          profileId: b.activeProfileId,
          chatId: dm.id,
        )).map((m) => m.toJson()).toList(),
        baselineCache,
      );
    },
    skip: !runLiveIntegration,
  );
}

class _RecordingHttpClient extends http.BaseClient {
  _RecordingHttpClient(this._inner);

  final http.Client _inner;
  final List<Uri> _messageGetUris = [];

  int messageGetCountFor(String chatId) => _messageGetUris
      .where((uri) => uri.queryParameters['chat_id'] == chatId)
      .length;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) {
    if (request.method == 'GET' && request.url.path == '/api/v1/messages') {
      final uri = request.url.replace(
        queryParameters: {...request.url.queryParameters, 'page_size': '1'},
      );
      _messageGetUris.add(uri);
      final shapedRequest = http.Request('GET', uri)
        ..headers.addAll(request.headers)
        ..followRedirects = request.followRedirects
        ..maxRedirects = request.maxRedirects
        ..persistentConnection = request.persistentConnection;
      return _inner.send(shapedRequest);
    }
    return _inner.send(request);
  }
}

class _T106LiveTestBinding extends AutomatedTestWidgetsFlutterBinding {
  @override
  bool get overrideHttpClient => false;
}
