import 'dart:async';

import 'package:flutter/material.dart';
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
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/ui/chat/pinned_messages_panel.dart';

import 'support/gateway_test_client.dart';
import 'support/voice_test_theme.dart';

void main() {
  group('ChatRoomController pin mutations reconcile token rotation', () {
    final scenarios =
        <
          ({
            String name,
            bool currentlyPinned,
            MessagesApiResult<void> mutationResult,
            bool serverPinned,
          })
        >[
          (
            name: 'pin success',
            currentlyPinned: false,
            mutationResult: const MessagesApiOk<void>(null),
            serverPinned: true,
          ),
          (
            name: 'pin failure',
            currentlyPinned: false,
            mutationResult: const MessagesApiFailure(
              message: 'pin failed',
              statusCode: 503,
            ),
            serverPinned: false,
          ),
          (
            name: 'unpin success',
            currentlyPinned: true,
            mutationResult: const MessagesApiOk<void>(null),
            serverPinned: false,
          ),
          (
            name: 'unpin failure',
            currentlyPinned: true,
            mutationResult: const MessagesApiFailure(
              message: 'unpin failed',
              statusCode: 503,
            ),
            serverPinned: true,
          ),
        ];
    for (final scenario in scenarios) {
      test('${scenario.name} uses the refreshed actor snapshot', () async {
        final auth = _TestAuthController();
        final message = _message();
        final client = _PinRaceMessagesClient(
          refreshedPins: scenario.serverPinned ? [message] : const [],
        );
        final container = _container(auth: auth, messages: client);
        addTearDown(container.dispose);
        final subscription = container.listen<ChatRoomState>(
          chatRoomControllerProvider('chat-one'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(subscription.close);
        await pumpEventQueue();

        final controller = container.read(
          chatRoomControllerProvider('chat-one').notifier,
        );
        controller.state = ChatRoomState(
          messages: [message],
          pinnedMessages: scenario.currentlyPinned ? [message] : const [],
        );
        final readsBeforeMutation = client.pinnedReads.length;

        final mutation = controller.togglePinWithResult(
          message.id,
          currentlyPinned: scenario.currentlyPinned,
        );
        expect(
          controller.state.pinnedMessages.map((item) => item.id),
          scenario.currentlyPinned ? isEmpty : [message.id],
        );

        if (scenario.name == 'unpin success') {
          controller.state = controller.state.copyWith(
            messages: [
              ...controller.state.messages,
              _message(id: 'live-during-unpin'),
            ],
          );
        }
        controller.state = controller.state.copyWith(
          pinnedMessages: [_message(id: 'newer-local-pin-snapshot')],
        );
        auth.rotateSameProfileToken();
        client.completeMutation(scenario.mutationResult);
        final result = await mutation;
        await pumpEventQueue();

        expect(client.mutationAuthorization, 'Bearer access-a');
        expect(client.pinnedReads.length, readsBeforeMutation + 1);
        expect(
          client.pinnedReads.last.authorization,
          'Bearer access-a-rotated',
        );
        expect(client.pinnedReads.last.chatId, 'chat-one');
        expect(
          controller.state.pinnedMessages.map((item) => item.id),
          scenario.serverPinned ? [message.id] : isEmpty,
        );
        expect(controller.state.messages.first.isPinned, scenario.serverPinned);
        if (scenario.name == 'unpin success') {
          expect(controller.state.messages.map((item) => item.id), [
            message.id,
            'live-during-unpin',
          ]);
        }
        expect(
          result.succeeded,
          scenario.serverPinned == !scenario.currentlyPinned,
        );
      });
    }

    for (final scenario in scenarios.where((item) => item.currentlyPinned)) {
      testWidgets(
        '${scenario.name} keeps the open list aligned with the room after refresh',
        (tester) async {
          final auth = _TestAuthController();
          final message = _message();
          final client = _PinRaceMessagesClient(
            refreshedPins: scenario.serverPinned ? [message] : const [],
          );
          final container = _container(auth: auth, messages: client);
          addTearDown(container.dispose);
          final subscription = container.listen<ChatRoomState>(
            chatRoomControllerProvider('chat-one'),
            (_, _) {},
            fireImmediately: true,
          );
          addTearDown(subscription.close);
          await tester.pump();

          final controller = container.read(
            chatRoomControllerProvider('chat-one').notifier,
          );
          late Future<PinMutationResult> mutation;
          controller.state = ChatRoomState(
            messages: [message],
            pinnedMessages: [message],
          );
          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: MaterialApp(
                theme: voiceTestTheme(),
                locale: const Locale('en'),
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                home: Scaffold(
                  body: PinnedMessagesPanel(
                    chatId: 'chat-one',
                    messages: [message],
                    onOpenMessage: (_) async => true,
                    onUnpin: (id) => mutation = controller.togglePinWithResult(
                      id,
                      currentlyPinned: true,
                    ),
                  ),
                ),
              ),
            ),
          );
          await tester.pump();

          await tester.tap(find.byTooltip('Unpin message'));
          await tester.pump();
          expect(
            find.byKey(const Key('chat_pinned_unpin_busy')),
            findsOneWidget,
          );
          auth.rotateSameProfileToken();
          var mutationCompleted = false;
          final completion = mutation.then((_) {
            mutationCompleted = true;
          });
          client.completeMutation(scenario.mutationResult);
          for (var frame = 0; frame < 8 && !mutationCompleted; frame++) {
            await tester.pump(const Duration(milliseconds: 16));
          }
          expect(mutationCompleted, isTrue);
          await completion;

          expect(
            find.byKey(const ValueKey('chat_pinned_message_pin-one')),
            scenario.serverPinned ? findsOneWidget : findsNothing,
          );
          expect(
            controller.state.pinnedMessages.map((item) => item.id),
            scenario.serverPinned ? [message.id] : isEmpty,
          );
          if (scenario.serverPinned) {
            expect(
              find.text(
                lookupAppLocalizations(const Locale('en')).commonActionFailed,
              ),
              findsOneWidget,
            );
          }
        },
      );
    }

    test(
      'profile replacement discards old mutation without refreshing as new actor',
      () async {
        final auth = _TestAuthController();
        final message = _message();
        final client = _PinRaceMessagesClient(refreshedPins: [message]);
        final container = _container(auth: auth, messages: client);
        addTearDown(container.dispose);
        final subscription = container.listen<ChatRoomState>(
          chatRoomControllerProvider('chat-one'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(subscription.close);
        await pumpEventQueue();

        final controller = container.read(
          chatRoomControllerProvider('chat-one').notifier,
        );
        controller.state = ChatRoomState(
          messages: [message],
          pinnedMessages: [message],
        );
        final readsBeforeMutation = client.pinnedReads.length;
        final mutation = controller.togglePinWithResult(
          message.id,
          currentlyPinned: true,
        );
        auth.replaceProfile();
        client.completeMutation(const MessagesApiOk<void>(null));

        final result = await mutation;
        await pumpEventQueue();

        expect(result.stale, isTrue);
        expect(client.pinnedReads.length, readsBeforeMutation);
        expect(client.pinnedReads.last.authorization, 'Bearer access-a');
        expect(controller.state.pinnedMessages, isEmpty);
      },
    );
  });
}

ProviderContainer _container({
  required _TestAuthController auth,
  required _PinRaceMessagesClient messages,
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith((ref) => auth),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceMessagesClientProvider.overrideWithValue(messages),
      realtimeHubProvider.overrideWithValue(_TestRealtimeHub()),
      messageCacheStoreProvider.overrideWithValue(InMemoryMessageCacheStore()),
      isDeviceOfflineProvider.overrideWith((ref) => false),
    ],
  );
}

VoiceMessage _message({String id = 'pin-one'}) => VoiceMessage(
  id: id,
  chatId: 'chat-one',
  senderProfileId: 'profile-a',
  content: 'Pinned message',
  createdAt: DateTime.parse('2024-01-01T00:00:00Z'),
);

AuthState _authState(
  String profileId,
  String accessToken,
  String refreshToken,
) {
  return AuthState(
    session: AuthSession(
      accessToken: accessToken,
      refreshToken: refreshToken,
      accountId: 'account-one',
      activeProfileId: profileId,
      expiresInSeconds: 900,
    ),
  );
}

class _TestAuthController extends AuthController {
  _TestAuthController()
    : super(
        authClient: VoiceAuthClient(
          gateway: gatewayHttpForTest(
            MockClient((_) async => http.Response('{}', 404)),
          ),
        ),
        storage: InMemoryAuthSessionStorage(),
        guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
      ) {
    state = _authState('profile-a', 'access-a', 'refresh-a');
  }

  void rotateSameProfileToken() {
    state = _authState('profile-a', 'access-a-rotated', 'refresh-a-rotated');
  }

  void replaceProfile() {
    state = _authState('profile-b', 'access-b', 'refresh-b');
  }
}

class _PinRaceMessagesClient extends VoiceMessagesClient {
  _PinRaceMessagesClient({required this.refreshedPins})
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final Completer<MessagesApiResult<void>> _mutation =
      Completer<MessagesApiResult<void>>();
  final List<VoiceMessage> refreshedPins;
  final List<({String authorization, String chatId})> pinnedReads = [];
  String? mutationAuthorization;

  void completeMutation(MessagesApiResult<void> result) =>
      _mutation.complete(result);

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
  Future<MessagesApiResult<MessageListData>> getPinnedMessages({
    required String authorization,
    required String chatId,
  }) {
    pinnedReads.add((authorization: authorization, chatId: chatId));
    return Future.value(
      MessagesApiOk(MessageListData(messages: refreshedPins)),
    );
  }

  @override
  Future<MessagesApiResult<void>> pinMessage({
    required String authorization,
    required String messageId,
    required String chatId,
  }) {
    mutationAuthorization = authorization;
    return _mutation.future;
  }

  @override
  Future<MessagesApiResult<void>> unpinMessage({
    required String authorization,
    required String messageId,
    required String chatId,
  }) {
    mutationAuthorization = authorization;
    return _mutation.future;
  }

  @override
  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async => const MessagesApiOk(null);
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

  @override
  Future<void> dispose() async => _events.close();
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
