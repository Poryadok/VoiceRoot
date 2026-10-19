import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/chat_draft_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/space_permissions.dart';
import 'package:voice_frontend/e2e/e2e_message_service.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/chat_draft_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/e2e_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/chat_message_list.dart';
import 'package:voice_frontend/ui/core/voice_state_panel.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/a11y/voice_shortcuts.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/markdown_test_helpers.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('reopens an already loaded failed-send draft from the inbox', (
    tester,
  ) async {
    final harness = _ImmediateRetryHarness();
    addTearDown(harness.container.dispose);
    const key = ChatDraftKey(profileId: 'prof-test', chatId: 'chat-abc');
    final subscription = harness.container.listen(
      chatDraftProvider(key),
      (_, _) {},
    );
    addTearDown(subscription.close);
    await harness.pump(tester);
    await tester.enterText(
      find.byKey(ChatRoomPanel.inputKey),
      'Unsent after failure',
    );
    await tester.tap(find.byKey(ChatRoomPanel.sendKey));
    await tester.pumpAndSettle();
    expect(harness.bodies, hasLength(1));
    expect(harness.composer(tester).text, 'Unsent after failure');

    // Inbox draft preview keeps the provider alive while the room is closed.
    await tester.pumpWidget(const SizedBox());
    await tester.pumpAndSettle();
    expect(
      harness.container.read(chatDraftProvider(key)).valueOrNull,
      'Unsent after failure',
    );
    await harness.pump(tester);
    expect(harness.composer(tester).text, 'Unsent after failure');
  });

  for (final scheduledEnabled in [false, true]) {
    testWidgets('scheduled list request follows capability=$scheduledEnabled', (
      tester,
    ) async {
      final scheduledRequests = <Uri>[];
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            GatewayConfig(
              baseUrl: 'http://api.test',
              scheduledMessagesEnabled: scheduledEnabled,
            ),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/messages/scheduled') {
                scheduledRequests.add(request.url);
                return http.Response('{}', 500);
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          presenceProvider.overrideWith((ref, profileId) => null),
          selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
          chatListControllerProvider.overrideWith(_DmChatListController.new),
          chatRoomControllerProvider('chat-abc').overrideWith(
            (ref) => _SingleMessageRoomController(ref, 'chat-abc'),
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
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(scheduledRequests, hasLength(scheduledEnabled ? 1 : 0));
      if (!scheduledEnabled) {
        expect(
          container
              .read(chatRoomControllerProvider('chat-abc'))
              .scheduledMessagesError,
          isNull,
        );
      }
    });
  }

  for (final threadsEnabled in [false, true]) {
    testWidgets('Reply action follows threads_enabled=$threadsEnabled', (
      tester,
    ) async {
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          presenceProvider.overrideWith((ref, profileId) => null),
          selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
          chatListControllerProvider.overrideWith(
            (ref) =>
                _SpaceChatListController(ref, threadsEnabled: threadsEnabled),
          ),
          chatRoomControllerProvider('chat-abc').overrideWith(
            (ref) => _SingleMessageRoomController(ref, 'chat-abc'),
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
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      container
              .read(chatMessageContextMenuRequestProvider('chat-abc').notifier)
              .state =
          'chat-abc-message';
      await tester.pumpAndSettle();

      final replyAction = find.text(
        AppLocalizations.of(
          tester.element(find.byType(ChatRoomPanel)),
        )!.chatMessageReply,
      );
      expect(replyAction, threadsEnabled ? findsOneWidget : findsNothing);
    });
  }

  testWidgets('disabled thread failure explains recovery and keeps the draft', (
    tester,
  ) async {
    final harness = _ImmediateRetryHarness(
      threadsEnabled: true,
      send: (_, _) async => http.Response(
        jsonEncode({
          'error': 'failed_precondition',
          'message': 'threads are disabled for this chat',
        }),
        412,
      ),
    );
    addTearDown(harness.container.dispose);
    await harness.pump(tester);

    await tester.enterText(find.byKey(ChatRoomPanel.inputKey), 'Keep my draft');
    harness.container.read(chatReplyTargetProvider('chat-abc').notifier).state =
        _retryReply;
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatRoomPanel.sendKey));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byType(ChatRoomPanel)),
    )!;
    expect(find.text(l10n.chatThreadRepliesUnavailable), findsOneWidget);
    expect(harness.composer(tester).text, 'Keep my draft');
    await tester.tap(find.byKey(ChatRoomPanel.replyTargetCancelKey));
    await tester.pumpAndSettle();
    expect(find.text(l10n.chatThreadRepliesUnavailable), findsNothing);
    expect(harness.composer(tester).text, 'Keep my draft');
    expect(harness.container.read(chatReplyTargetProvider('chat-abc')), isNull);
  });

  for (final locale in ['en', 'ru']) {
    testWidgets(
      'immediate text failure in $locale retries the same wire attempt',
      (tester) async {
        final harness = _ImmediateRetryHarness(threadsEnabled: true);
        addTearDown(harness.container.dispose);
        harness.room.state = harness.room.state.copyWith(
          messages: const [_preexistingRetryHistory],
        );
        harness.container
                .read(chatReplyTargetProvider('chat-abc').notifier)
                .state =
            _retryReply;
        await harness.pump(tester, locale: locale);
        expect(harness.room.state.messages.map((message) => message.id), [
          'history-before-retry',
        ]);
        expectMessagePlainText(tester, 'Existing visible history');
        final l10n = AppLocalizations.of(
          tester.element(find.byType(ChatRoomPanel)),
        )!;
        await tester.enterText(
          find.byKey(ChatRoomPanel.inputKey),
          'Retry draft',
        );
        await tester.tap(find.byKey(ChatRoomPanel.sendKey));
        await tester.pumpAndSettle();

        expect(harness.bodies, hasLength(1), reason: 'no automatic resend');
        expect(harness.room.state.messages.map((message) => message.id), [
          'history-before-retry',
        ]);
        expectMessagePlainText(tester, 'Existing visible history');
        expect(find.textContaining(_sendDiagnostic), findsNothing);
        expect(find.text(l10n.commonActionFailed), findsWidgets);
        expect(harness.composer(tester).text, 'Retry draft');
        expect(
          harness.container.read(chatReplyTargetProvider('chat-abc')),
          _retryReply,
        );
        final original =
            jsonDecode(harness.bodies.single) as Map<String, dynamic>;
        expect(
          original['client_message_id'],
          matches(
            RegExp(
              r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
            ),
          ),
        );
        expect(original['thread_parent_id'], 'reply-1');

        final auth = harness.container.read(authControllerProvider.notifier);
        auth.state = const AuthState(
          session: AuthSession(
            accessToken: 'renewed-access',
            refreshToken: 'renewed-refresh',
            accountId: 'acc-test',
            activeProfileId: 'prof-test',
            expiresInSeconds: 900,
          ),
        );
        await tester.pump();
        expect(find.byKey(ChatRoomPanel.sendRetryKey), findsOneWidget);

        final historyLoads = harness.room.historyLoads;
        await tester.tap(find.byKey(ChatRoomPanel.sendRetryKey));
        await tester.pumpAndSettle();
        expect(harness.bodies, hasLength(2));
        expect(harness.bodies.last, harness.bodies.first);
        expect(harness.authorizations, [
          'Bearer test-access',
          'Bearer renewed-access',
        ]);
        expect(
          harness.room.historyLoads,
          historyLoads,
          reason: 'Retry is not history reload',
        );
        expect(harness.composer(tester).text, isEmpty);
        expect(
          harness.container.read(chatReplyTargetProvider('chat-abc')),
          isNull,
        );
        expect(
          harness.room.state.messages.map((m) => m.id),
          unorderedEquals(['history-before-retry', 'accepted-1']),
        );
        expectMessagePlainText(tester, 'Existing visible history');
        expectMessagePlainText(tester, 'Accepted text');
        expect(find.byKey(ChatRoomPanel.sendRetryKey), findsNothing);
      },
    );
  }

  for (final change in ['chat', 'profile', 'account', 'logout', 'relogin']) {
    testWidgets(
      'captured text Retry is inert after $change change and return',
      (tester) async {
        final harness = _ImmediateRetryHarness();
        addTearDown(harness.container.dispose);
        await harness.pump(tester);
        await tester.enterText(
          find.byKey(ChatRoomPanel.inputKey),
          'Private A draft',
        );
        await tester.tap(find.byKey(ChatRoomPanel.sendKey));
        await tester.pumpAndSettle();
        final retry = tester
            .widget<TextButton>(find.byKey(ChatRoomPanel.sendRetryKey))
            .onPressed!;
        final auth = harness.container.read(authControllerProvider.notifier);
        final originalSession = auth.state.session!;
        if (change == 'chat') {
          harness.container.read(selectedChatIdProvider.notifier).state =
              'other-chat';
        } else if (change == 'relogin') {
          (auth as _ImmediateAuthController).installSameActorSession();
        } else {
          auth.state = AuthState(
            session: change == 'logout'
                ? null
                : AuthSession(
                    accessToken: 'other-access',
                    refreshToken: 'other-refresh',
                    accountId: change == 'account'
                        ? 'other-account'
                        : originalSession.accountId,
                    activeProfileId: change == 'profile'
                        ? 'other-profile'
                        : originalSession.activeProfileId,
                    expiresInSeconds: 900,
                  ),
          );
        }
        await tester.pump();
        if (change == 'chat') {
          harness.container.read(selectedChatIdProvider.notifier).state =
              'chat-abc';
        } else {
          auth.state = AuthState(session: originalSession);
        }
        await tester.pump();
        retry();
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 30));
        expect(harness.bodies, hasLength(1));
        expect(find.byKey(ChatRoomPanel.sendRetryKey), findsNothing);
        expect(harness.room.state.errorMessage, isNull);
        expect(find.textContaining(_sendDiagnostic), findsNothing);
      },
    );
  }

  testWidgets(
    'token renewal fences an old response while allowing the same explicit Retry',
    (tester) async {
      final pending = Completer<http.Response>();
      final harness = _ImmediateRetryHarness(
        threadsEnabled: true,
        send: (_, attempt) async =>
            attempt == 1 ? pending.future : _successfulSendResponse(),
      );
      addTearDown(harness.container.dispose);
      await harness.pump(tester);
      await tester.enterText(
        find.byKey(ChatRoomPanel.inputKey),
        'Pending text',
      );
      await tester.tap(find.byKey(ChatRoomPanel.sendKey));
      await tester.pump();
      harness.container
          .read(authControllerProvider.notifier)
          .state = const AuthState(
        session: AuthSession(
          accessToken: 'renewed-access',
          refreshToken: 'renewed-refresh',
          accountId: 'acc-test',
          activeProfileId: 'prof-test',
          expiresInSeconds: 900,
        ),
      );
      await tester.pump();
      pending.complete(_successfulSendResponse());
      await tester.pumpAndSettle();
      expect(harness.room.state.messages, isEmpty);
      expect(harness.bodies, hasLength(1));
      expect(harness.composer(tester).text, 'Pending text');
      await tester.tap(find.byKey(ChatRoomPanel.sendRetryKey));
      await tester.pumpAndSettle();
      expect(harness.bodies, hasLength(2));
      expect(harness.bodies.last, harness.bodies.first);
      expect(harness.authorizations.last, 'Bearer renewed-access');
      expect(harness.room.state.messages.map((m) => m.id), ['accepted-1']);
      expect(harness.composer(tester).text, isEmpty);
    },
  );

  testWidgets(
    'captured text Retry cannot replay a replacement attachment attempt',
    (tester) async {
      final harness = _ImmediateRetryHarness(
        send: (_, attempt) async =>
            attempt <= 2 ? _failedSendResponse() : _successfulSendResponse(),
      );
      addTearDown(harness.container.dispose);
      await harness.pump(tester);
      await tester.enterText(
        find.byKey(ChatRoomPanel.inputKey),
        'Original text',
      );
      await tester.tap(find.byKey(ChatRoomPanel.sendKey));
      await tester.pumpAndSettle();
      final retry = tester
          .widget<TextButton>(find.byKey(ChatRoomPanel.sendRetryKey))
          .onPressed!;
      expect(
        await harness.room.sendMessage(
          'Attachment caption',
          attachments: const [
            MessageAttachment(fileId: 'file-1', type: 'document'),
          ],
        ),
        isNotNull,
      );
      await tester.pumpAndSettle();
      retry();
      await tester.pumpAndSettle();
      expect(harness.bodies, hasLength(2));
      expect(harness.composer(tester).text, 'Original text');
      expect(find.byKey(ChatRoomPanel.sendRetryKey), findsNothing);
      expect(harness.room.canRetryFailedMessage, isTrue);
      expect(await harness.room.retryFailedMessage(), isNull);
      await tester.pumpAndSettle();
      expect(harness.bodies, hasLength(3));
      expect(harness.bodies.last, harness.bodies[1]);
      expect(harness.bodies.last, isNot(harness.bodies.first));
    },
  );

  for (final sameValue in [false, true]) {
    final historyError = sameValue ? _sendDiagnostic : 'permission_denied';
    testWidgets(
      'text Retry preserves unrelated history error with ${sameValue ? 'same' : 'different'} text',
      (tester) async {
        final harness = _ImmediateRetryHarness();
        addTearDown(harness.container.dispose);
        await harness.pump(tester);
        final l10n = AppLocalizations.of(
          tester.element(find.byType(ChatRoomPanel)),
        )!;
        await tester.enterText(
          find.byKey(ChatRoomPanel.inputKey),
          'Failed text',
        );
        await tester.tap(find.byKey(ChatRoomPanel.sendKey));
        await tester.pumpAndSettle();
        harness.room.state = harness.room.state.copyWith(
          errorMessage: historyError,
        );
        await tester.pumpAndSettle();
        final historyLabel = sameValue
            ? l10n.chatRoomLoadError
            : l10n.chatRoomPermissionDenied;
        expect(find.text(historyLabel), findsOneWidget);
        expect(find.text(l10n.commonActionFailed), findsOneWidget);
        await tester.tap(find.byKey(ChatRoomPanel.sendRetryKey));
        await tester.pumpAndSettle();
        expect(harness.room.state.errorMessage, historyError);
        expect(find.text(historyLabel), findsOneWidget);
        expect(find.textContaining(_sendDiagnostic), findsNothing);
        expect(find.byKey(ChatRoomPanel.sendRetryKey), findsNothing);
        expect(harness.bodies, hasLength(2));
        expect(harness.bodies.last, harness.bodies.first);
      },
    );
  }

  testWidgets(
    'text Retry is single-flight and preserves a newer draft and reply',
    (tester) async {
      final pending = Completer<http.Response>();
      final harness = _ImmediateRetryHarness(
        threadsEnabled: true,
        send: (_, attempt) async =>
            attempt == 1 ? _failedSendResponse() : pending.future,
      );
      addTearDown(harness.container.dispose);
      harness.container
              .read(chatReplyTargetProvider('chat-abc').notifier)
              .state =
          _retryReply;
      await harness.pump(tester);
      await tester.enterText(
        find.byKey(ChatRoomPanel.inputKey),
        'Original text',
      );
      await tester.tap(find.byKey(ChatRoomPanel.sendKey));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(ChatRoomPanel.inputKey), 'Newer text');
      const newerReply = VoiceMessage(
        id: 'reply-2',
        chatId: 'chat-abc',
        senderProfileId: 'peer-1',
        content: 'New reply',
      );
      harness.container
              .read(chatReplyTargetProvider('chat-abc').notifier)
              .state =
          newerReply;
      await tester.pump();
      expect(
        harness.container.read(chatReplyTargetProvider('chat-abc')),
        newerReply,
      );
      final retry = tester
          .widget<TextButton>(find.byKey(ChatRoomPanel.sendRetryKey))
          .onPressed!;
      retry();
      retry();
      await tester.pump();
      expect(harness.bodies, hasLength(2));
      expect(harness.bodies.last, harness.bodies.first);
      pending.complete(_successfulSendResponse());
      await tester.pumpAndSettle();
      expect(harness.bodies, hasLength(2));
      expect(harness.composer(tester).text, 'Newer text');
      expect(
        harness.container.read(chatReplyTargetProvider('chat-abc')),
        newerReply,
      );
    },
  );

  testWidgets('text Retry blocks a retained reply after threads are disabled', (
    tester,
  ) async {
    final harness = _ImmediateRetryHarness(threadsEnabled: true);
    addTearDown(harness.container.dispose);
    harness.container.read(chatReplyTargetProvider('chat-abc').notifier).state =
        _retryReply;
    await harness.pump(tester);
    await tester.enterText(
      find.byKey(ChatRoomPanel.inputKey),
      'Retained reply body',
    );
    await tester.tap(find.byKey(ChatRoomPanel.sendKey));
    await tester.pumpAndSettle();
    expect(harness.bodies, hasLength(1));

    harness.container
        .read(chatListControllerProvider.notifier)
        .state = ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
            threadsEnabled: false,
          ),
          dmPeerProfileId: 'peer-1',
        ),
      ],
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatRoomPanel.sendRetryKey));
    await tester.pumpAndSettle();

    expect(harness.bodies, hasLength(1));
    expect(harness.composer(tester).text, 'Retained reply body');
    expect(
      harness.container.read(chatReplyTargetProvider('chat-abc')),
      _retryReply,
    );
    expect(
      find.text(
        AppLocalizations.of(
          tester.element(find.byType(ChatRoomPanel)),
        )!.chatThreadRepliesUnavailable,
      ),
      findsOneWidget,
    );
  });

  testWidgets('late successful text send cannot clear another profile draft', (
    tester,
  ) async {
    final pending = Completer<http.Response>();
    final harness = _ImmediateRetryHarness(send: (_, _) => pending.future);
    addTearDown(harness.container.dispose);
    await harness.pump(tester);
    await tester.enterText(
      find.byKey(ChatRoomPanel.inputKey),
      'Private A text',
    );
    await tester.tap(find.byKey(ChatRoomPanel.sendKey));
    await tester.pump();
    final auth = harness.container.read(authControllerProvider.notifier);
    auth.state = const AuthState(
      session: AuthSession(
        accessToken: 'profile-b',
        refreshToken: 'refresh-b',
        accountId: 'acc-test',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();
    await tester.enterText(
      find.byKey(ChatRoomPanel.inputKey),
      'Private B text',
    );
    pending.complete(_successfulSendResponse());
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 30));
    expect(harness.composer(tester).text, 'Private B text');
    expect(harness.room.state.messages, isEmpty);
    expect(find.byKey(ChatRoomPanel.sendRetryKey), findsNothing);
  });

  test(
    'immediate encrypted retry preserves payload, mentions and UUID after token renewal',
    () async {
      final harness = _ImmediateRetryHarness(encrypted: true);
      addTearDown(() async {
        await pumpEventQueue();
        harness.container.dispose();
      });
      final room = harness.room;
      final mentions = <MessageMention>[
        const MessageMention(type: 'user', targetId: 'peer-1'),
      ];
      expect(
        await room.sendMessage(
          'Encrypted retry',
          mentions: mentions,
          threadParentId: 'reply-1',
        ),
        isNotNull,
      );
      mentions.clear();
      final auth = harness.container.read(authControllerProvider.notifier);
      auth.state = const AuthState(
        session: AuthSession(
          accessToken: 'renewed-access',
          refreshToken: 'renewed-refresh',
          accountId: 'acc-test',
          activeProfileId: 'prof-test',
          expiresInSeconds: 900,
        ),
      );
      expect(await room.retryFailedMessage(), isNull);
      expect(harness.encryptionCalls, 1);
      expect(harness.bodies, hasLength(2));
      expect(harness.bodies.last, harness.bodies.first);
      expect(harness.authorizations, [
        'Bearer test-access',
        'Bearer renewed-access',
      ]);
      final body = jsonDecode(harness.bodies.first) as Map<String, dynamic>;
      expect(body['content'], 'wire-ciphertext-1');
      expect(body['is_e2e'], isTrue);
      expect(body['thread_parent_id'], 'reply-1');
      expect(jsonDecode(body['mentions_json'] as String), [
        {'type': 'user', 'target_id': 'peer-1'},
      ]);
      expect(room.state.messages.map((m) => m.id), ['accepted-1']);
    },
  );

  test(
    'leaving a pending text send releases busy state before returning to chat',
    () async {
      final pending = Completer<http.Response>();
      final harness = _ImmediateRetryHarness(
        send: (_, attempt) async =>
            attempt == 1 ? pending.future : _successfulSendResponse(),
      );
      addTearDown(() async {
        await pumpEventQueue();
        harness.container.dispose();
      });
      final room = harness.room;
      final first = room.sendMessage('Old pending text');
      await Future<void>.delayed(Duration.zero);
      expect(harness.bodies, hasLength(1));
      expect(room.state.isSending, isTrue);
      harness.container.read(selectedChatIdProvider.notifier).state =
          'other-chat';
      expect(room.state.isSending, isFalse);
      expect(room.canRetryFailedMessage, isFalse);
      expect(room.state.errorMessage, isNull);
      harness.container.read(selectedChatIdProvider.notifier).state =
          'chat-abc';
      expect(await room.sendMessage('New text after returning'), isNull);
      pending.complete(_successfulSendResponse());
      expect(await first, kChatActionStaleContext);
      expect(harness.bodies, hasLength(2));
      expect(room.state.isSending, isFalse);
      expect(room.state.messages.map((m) => m.id), ['accepted-1']);
    },
  );

  for (final historyError in ['history-error', _sendDiagnostic]) {
    test(
      'chat invalidation preserves unrelated history error with ${historyError == _sendDiagnostic ? 'same' : 'different'} text',
      () async {
        final harness = _ImmediateRetryHarness();
        addTearDown(harness.container.dispose);
        expect(await harness.room.sendMessage('Failed text'), isNotNull);
        harness.room.state = harness.room.state.copyWith(
          errorMessage: historyError,
        );
        harness.container.read(selectedChatIdProvider.notifier).state =
            'other-chat';
        expect(harness.room.state.errorMessage, historyError);
        expect(harness.room.canRetryFailedMessage, isFalse);
      },
    );

    test(
      'new login invalidation preserves unrelated history error with ${historyError == _sendDiagnostic ? 'same' : 'different'} text',
      () async {
        final harness = _ImmediateRetryHarness();
        addTearDown(harness.container.dispose);
        expect(await harness.room.sendMessage('Failed text'), isNotNull);
        harness.room.state = harness.room.state.copyWith(
          errorMessage: historyError,
        );
        (harness.container.read(authControllerProvider.notifier)
                as _ImmediateAuthController)
            .installSameActorSession();
        expect(harness.room.state.errorMessage, historyError);
        expect(harness.room.canRetryFailedMessage, isFalse);
      },
    );
  }

  test(
    'immediate failed attempt is released after controller disposal',
    () async {
      final harness = _ImmediateRetryHarness();
      final room = harness.room;
      expect(await room.sendMessage('Private text'), isNotNull);
      harness.container.dispose();
      expect(await room.retryFailedMessage(), kChatActionStaleContext);
      expect(harness.bodies, hasLength(1));
    },
  );

  testWidgets(
    'space message moderation has no delete affordance while permission is unresolved or denied',
    (tester) async {
      final moderationPermission = Completer<bool>();
      final query = (
        spaceId: 'space-1',
        permission: SpacePermissions.textChatManageMessages,
        chatId: 'chat-abc',
        voiceRoomId: null,
      );
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatListControllerProvider.overrideWith(_SpaceChatListController.new),
          chatRoomControllerProvider('chat-abc').overrideWith(
            (ref) => _SingleMessageRoomController(ref, 'chat-abc'),
          ),
          spacePermissionProvider(
            query,
          ).overrideWith((ref) => moderationPermission.future),
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
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      container
              .read(chatMessageContextMenuRequestProvider('chat-abc').notifier)
              .state =
          'chat-abc-message';
      await tester.pumpAndSettle();

      final checkingPermission = find.ancestor(
        of: find.text('Checking permissions…'),
        matching: find.byType(ListTile),
      );
      expect(checkingPermission, findsOneWidget);
      expect(find.text('Delete for everyone'), findsNothing);
      expect(
        tester.widget<ListTile>(checkingPermission).onTap,
        isNull,
        reason: 'an unresolved permission check must expose no delete action',
      );
      expect(find.byTooltip('Checking permissions…'), findsOneWidget);

      moderationPermission.complete(false);
      await tester.pumpAndSettle();

      expect(find.text('Delete for everyone'), findsNothing);
      final unavailableModeration = find.ancestor(
        of: find.text('Message moderation is unavailable.'),
        matching: find.byType(ListTile),
      );
      expect(unavailableModeration, findsOneWidget);
      expect(tester.widget<ListTile>(unavailableModeration).onTap, isNull);
      expect(
        find.byTooltip('Message moderation is unavailable.'),
        findsOneWidget,
      );
    },
  );

  testWidgets(
    'Space moderator can delete another member message for everyone',
    (tester) async {
      final query = (
        spaceId: 'space-1',
        permission: SpacePermissions.textChatManageMessages,
        chatId: 'chat-abc',
        voiceRoomId: null,
      );
      late _SingleMessageRoomController roomController;
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatListControllerProvider.overrideWith(_SpaceChatListController.new),
          chatRoomControllerProvider('chat-abc').overrideWith((ref) {
            return roomController = _SingleMessageRoomController(
              ref,
              'chat-abc',
            );
          }),
          spacePermissionProvider(query).overrideWith((ref) async => true),
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
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      container
              .read(chatMessageContextMenuRequestProvider('chat-abc').notifier)
              .state =
          'chat-abc-message';
      await tester.pumpAndSettle();

      final deleteForEveryone = find.ancestor(
        of: find.text('Delete for everyone'),
        matching: find.byType(ListTile),
      );
      expect(deleteForEveryone, findsOneWidget);
      expect(tester.widget<ListTile>(deleteForEveryone).onTap, isNotNull);
      await tester.ensureVisible(deleteForEveryone);
      await tester.tap(deleteForEveryone);
      await tester.pumpAndSettle();

      expect(roomController.deleteMessageCalls, [('chat-abc-message', false)]);
    },
  );

  testWidgets(
    'switching to a chat with no unread messages hides the previous unread separator',
    (tester) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.binding.setSurfaceSize(const Size(900, 600));
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'chat-a'),
          chatListControllerProvider.overrideWith(
            _UnreadTwoChatListController.new,
          ),
          chatRoomControllerProvider(
            'chat-a',
          ).overrideWith((ref) => _SingleMessageRoomController(ref, 'chat-a')),
          chatRoomControllerProvider(
            'chat-b',
          ).overrideWith((ref) => _SingleMessageRoomController(ref, 'chat-b')),
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
            home: Scaffold(
              body: ThreeColumnShell(
                navigationChild: const ChatListBody(showHeader: false),
                mainChild: Consumer(
                  builder: (context, ref, _) {
                    return ChatRoomPanel(
                      chatId: ref.watch(selectedChatIdProvider)!,
                    );
                  },
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(container.read(selectedChatIdProvider), 'chat-a');
      expectMessagePlainText(tester, 'Message for chat-a');
      expect(find.byType(ChatUnreadSeparator), findsOneWidget);

      await tester.tap(find.byKey(ChatListBody.tileKey('chat-b')));
      await tester.pumpAndSettle();

      expect(container.read(selectedChatIdProvider), 'chat-b');
      expectMessagePlainText(tester, 'Message for chat-b');
      expect(find.byType(ChatUnreadSeparator), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets(
    'switching chats keeps a visible room loading state without a progress line',
    (tester) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.binding.setSurfaceSize(const Size(900, 600));
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'chat-a'),
          chatListControllerProvider.overrideWith(_TwoChatListController.new),
          chatRoomControllerProvider(
            'chat-a',
          ).overrideWith((ref) => _EmptyRoomController(ref, 'chat-a')),
          chatRoomControllerProvider(
            'chat-b',
          ).overrideWith((ref) => _LoadingRoomController(ref, 'chat-b')),
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
            home: Scaffold(
              body: ThreeColumnShell(
                navigationChild: const ChatListBody(showHeader: false),
                mainChild: Consumer(
                  builder: (context, ref, _) {
                    return ChatRoomPanel(
                      chatId: ref.watch(selectedChatIdProvider)!,
                    );
                  },
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 1));
      final navigationElement = tester.element(
        find.byKey(ThreeColumnShell.navActiveRail),
      );
      expect(find.byKey(ChatListBody.tileKey('chat-b')), findsOneWidget);
      await tester.tap(find.byKey(ChatListBody.tileKey('chat-b')));
      await tester.pump(const Duration(milliseconds: 1));

      expect(container.read(selectedChatIdProvider), 'chat-b');
      expect(
        tester.element(find.byKey(ThreeColumnShell.navActiveRail)),
        same(navigationElement),
      );
      expect(find.byKey(ChatListBody.tileKey('chat-a')), findsOneWidget);
      expect(find.byKey(ChatListBody.tileKey('chat-b')), findsOneWidget);
      expect(
        find.descendant(
          of: find.byKey(ThreeColumnShell.navOpenChat),
          matching: find.byType(VoiceListSkeleton),
        ),
        findsOneWidget,
      );
      expect(find.byType(LinearProgressIndicator), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump(const Duration(milliseconds: 1));
    },
  );

  testWidgets(
    'hides previous-profile history until active-profile history binds',
    (tester) async {
      late _ProfileBoundRoomController room;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            profileAccentStorageProvider.overrideWithValue(
              testProfileAccentStorage,
            ),
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(_ProfileBAuthController.new),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            httpClientProvider.overrideWithValue(
              MockClient((_) async => http.Response('{}', 404)),
            ),
            realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
            selectedChatIdProvider.overrideWith((ref) => 'other-chat'),
            chatRoomControllerProvider('chat-abc').overrideWith((ref) {
              room = _ProfileBoundRoomController(ref, 'chat-abc');
              return room;
            }),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pump();

      expect(room.state.messages.single.content, 'from profile A');
      expect(room.state.nextCursor, 'profile-a-cursor');
      expect(find.text('from profile A'), findsNothing);
      expect(find.byType(VoiceStatePanel), findsNothing);

      room.bindProfileB();
      await tester.pump();

      expectMessagePlainText(tester, 'from profile B');
      expect(richPlainText(tester), isNot(contains('from profile A')));
      expect(find.byType(VoiceStatePanel), findsNothing);
    },
  );

  testWidgets('ChatRoomPanel composer has no @ mention button', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.url.path == '/api/v1/messages') {
                return http.Response(
                  jsonEncode({
                    'message_list': {'messages': []},
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/messages/read') {
                return http.Response('{}', 200);
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatRoomControllerProvider(
            'chat-abc',
          ).overrideWith((ref) => _EmptyRoomController(ref, 'chat-abc')),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('chat_room_mention_button')), findsNothing);
    expect(find.byIcon(Icons.alternate_email), findsNothing);
  });

  testWidgets('blocked DM send keeps draft and shows neutral error', (
    tester,
  ) async {
    var sendCalls = 0;
    late _EmptyRoomController room;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.url.path == '/api/v1/messages/send') {
                sendCalls++;
                return http.Response(
                  jsonEncode({
                    'error_code': 'permission_denied',
                    'message': 'cannot send messages between blocked accounts',
                  }),
                  403,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatListControllerProvider.overrideWith(
            _BlockedDmChatListController.new,
          ),
          chatListProvider.overrideWith(
            (ref) async => const ChatListData(
              items: [
                ChatListItem(
                  chat: VoiceChat(
                    id: 'chat-abc',
                    type: 'CHAT_TYPE_DM',
                    creatorProfileId: 'prof-test',
                  ),
                  dmPeerProfileId: 'peer-1',
                ),
              ],
            ),
          ),
          chatRoomControllerProvider('chat-abc').overrideWith((ref) {
            room = _EmptyRoomController(ref, 'chat-abc');
            return room;
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('User unavailable'), findsOneWidget);
    expect(find.text('Chat chat-abc'), findsNothing);

    await tester.enterText(
      find.byKey(ChatRoomPanel.inputKey),
      'Draft stays here',
    );
    await tester.tap(find.byKey(ChatRoomPanel.sendKey));
    await tester.pumpAndSettle();

    final input = tester.widget<TextField>(
      find.descendant(
        of: find.byKey(ChatRoomPanel.inputKey),
        matching: find.byType(TextField),
      ),
    );
    expect(sendCalls, 1);
    expect(input.controller?.text, 'Draft stays here');
    expect(
      find.text('Messages are disabled between these accounts'),
      findsOneWidget,
    );
    expect(
      find.textContaining('cannot send messages between blocked accounts'),
      findsNothing,
    );
    expect(room.state.messages, isEmpty);
    expect(find.byType(ChatMessageBubbleTile), findsNothing);
    expect(find.byKey(ChatRoomPanel.messagesKey), findsNothing);
  });

  testWidgets(
    'Send when online retries the same draft and id without an immediate send',
    (tester) async {
      final firstAttemptResult = Completer<String?>();
      final coldChatList = Completer<ChatListData>();
      var immediateSendRequests = 0;
      late _ScheduledRoomController room;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            profileAccentStorageProvider.overrideWithValue(
              testProfileAccentStorage,
            ),
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(
                baseUrl: 'http://api.test',
                scheduledMessagesEnabled: true,
              ),
            ),
            httpClientProvider.overrideWithValue(
              MockClient((request) async {
                if (request.url.path == '/api/v1/messages/send') {
                  immediateSendRequests++;
                }
                return http.Response('{}', 404);
              }),
            ),
            realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
            selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
            chatListControllerProvider.overrideWith(_DmChatListController.new),
            chatListProvider.overrideWith((ref) => coldChatList.future),
            chatRoomControllerProvider('chat-abc').overrideWith((ref) {
              return room = _ScheduledRoomController(
                ref,
                'chat-abc',
                firstAttemptResult: firstAttemptResult,
              );
            }),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final composer = find.descendant(
        of: find.byKey(ChatRoomPanel.inputKey),
        matching: find.byType(TextField),
      );
      await tester.enterText(composer, 'Keep this scheduled draft');
      final chatRoomProviderContainer = ProviderScope.containerOf(
        tester.element(find.byType(ChatRoomPanel)),
        listen: false,
      );
      final asyncChatListBeforeLongPress = chatRoomProviderContainer.read(
        chatListProvider,
      );
      expect(asyncChatListBeforeLongPress.isLoading, isTrue);
      final chatListBeforeLongPress = chatRoomProviderContainer.read(
        chatListControllerProvider,
      );
      final activeProfileBeforeLongPress = chatRoomProviderContainer
          .read(authControllerProvider)
          .activeProfileId;
      final dmReadyBeforeLongPress =
          chatListBeforeLongPress.profileId == activeProfileBeforeLongPress &&
          chatListBeforeLongPress.items.any(
            (item) =>
                item.chatId == 'chat-abc' && item.chat.type == 'CHAT_TYPE_DM',
          );
      expect(
        dmReadyBeforeLongPress,
        isTrue,
        reason: 'DM chat state is ready before scheduled-send menu trigger',
      );
      final chatListController = chatRoomProviderContainer.read(
        chatListControllerProvider.notifier,
      );
      final dmItem = chatListBeforeLongPress.items.single;
      final invalidMenuStates = <ChatListState>[
        const ChatListState(profileId: null),
        const ChatListState(
          profileId: 'other-profile',
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: 'chat-abc',
                type: 'CHAT_TYPE_DM',
                creatorProfileId: 'prof-test',
              ),
              dmPeerProfileId: 'peer-1',
            ),
          ],
        ),
        const ChatListState(profileId: 'prof-test'),
        const ChatListState(
          profileId: 'prof-test',
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: 'chat-abc',
                type: 'CHAT_TYPE_GROUP',
                creatorProfileId: 'prof-test',
              ),
            ),
          ],
        ),
        const ChatListState(
          profileId: 'prof-test',
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: 'chat-abc',
                type: 'CHAT_TYPE_CHANNEL',
                creatorProfileId: 'prof-test',
              ),
            ),
          ],
        ),
      ];
      for (final invalidState in invalidMenuStates) {
        chatListController.state = invalidState;
        await tester.longPress(find.byKey(ChatRoomPanel.sendKey));
        await tester.pumpAndSettle();
        expect(find.byType(BottomSheet), findsOneWidget);
        expect(find.text('Schedule message'), findsOneWidget);
        expect(find.text('Send when online'), findsNothing);
        Navigator.of(tester.element(find.byType(BottomSheet))).pop();
        await tester.pumpAndSettle();
      }
      chatListController.state = ChatListState(
        profileId: 'prof-test',
        items: [dmItem],
      );
      await tester.longPress(find.byKey(ChatRoomPanel.sendKey));
      await tester.pumpAndSettle();
      expect(find.byType(BottomSheet), findsOneWidget);
      final menuListAfterLongPress = chatRoomProviderContainer.read(
        chatListControllerProvider,
      );
      final selectedChatIsDm =
          menuListAfterLongPress.profileId == activeProfileBeforeLongPress &&
          menuListAfterLongPress.items.any(
            (item) =>
                item.chatId == 'chat-abc' && item.chat.type == 'CHAT_TYPE_DM',
          );
      expect(selectedChatIsDm, isTrue);
      await tester.tap(find.text('Send when online'));
      await tester.pump();

      expect(room.scheduledAttempts, hasLength(1));
      expect(room.state.isSending, isTrue);
      expect(find.text('Retry'), findsNothing);
      firstAttemptResult.complete('unavailable');
      await tester.pumpAndSettle();

      expect(
        room.scheduledAttempts.single.content,
        'Keep this scheduled draft',
      );
      expect(room.scheduledAttempts.single.sendWhenOnline, isTrue);
      expect(room.scheduledAttempts.single.scheduledAt, isNull);
      expect(
        find.text(
          'Could not complete the scheduled-message request. Try again.',
        ),
        findsOneWidget,
      );
      expect(immediateSendRequests, 0);
      expect(find.text('Keep this scheduled draft'), findsWidgets);

      await tester.tap(
        find.descendant(
          of: find.byKey(ChatRoomPanel.scheduledCreateRetryKey),
          matching: find.text('Retry'),
        ),
      );
      await tester.pumpAndSettle();

      expect(room.scheduledAttempts, hasLength(2));
      expect(
        room.scheduledAttempts[1].clientMessageId,
        room.scheduledAttempts[0].clientMessageId,
      );
      expect(
        room.scheduledAttempts[1].content,
        room.scheduledAttempts[0].content,
      );
      expect(room.scheduledAttempts[1].sendWhenOnline, isTrue);
      expect(immediateSendRequests, 0);
      expect(
        find.text(
          'Could not complete the scheduled-message request. Try again.',
        ),
        findsNothing,
      );
      expect(find.text('Keep this scheduled draft'), findsNothing);
    },
  );

  test('scheduled list drops a response after auth context changes', () async {
    final requestStarted = Completer<void>();
    final deferredResponse = Completer<http.Response>();
    late AuthController auth;
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith((ref) {
          return auth = authenticatedAuthController(ref);
        }),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(
            baseUrl: 'http://api.test',
            scheduledMessagesEnabled: true,
          ),
        ),
        httpClientProvider.overrideWithValue(
          MockClient((request) async {
            if (request.url.path == '/api/v1/messages/scheduled') {
              if (!requestStarted.isCompleted) requestStarted.complete();
              return deferredResponse.future;
            }
            return http.Response('{}', 404);
          }),
        ),
        realtimeAutoConnectProvider.overrideWithValue(false),
        realtimeEventProvider.overrideWith(
          (ref) => const Stream<RealtimeFrame>.empty(),
        ),
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
      ],
    );
    addTearDown(container.dispose);
    final roomProvider = chatRoomControllerProvider('chat-abc');
    final roomSubscription = container.listen(roomProvider, (_, _) {});
    addTearDown(roomSubscription.close);
    final room = container.read(roomProvider.notifier);

    final load = room.loadScheduledMessages();
    await requestStarted.future;
    auth.state = const AuthState(
      session: AuthSession(
        accessToken: 'replacement-access',
        refreshToken: 'replacement-refresh',
        accountId: 'account-b',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    deferredResponse.complete(
      http.Response(
        jsonEncode({
          'scheduled_messages': [
            {
              'id': 'old-account-schedule',
              'chat': {'id': 'chat-abc'},
              'sender_profile_id': 'profile-a',
              'send_when_online': true,
              'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
            },
          ],
          'page': {'has_more': false},
        }),
        200,
      ),
    );
    await load;

    expect(room.state.scheduledMessages, isEmpty);
    expect(room.state.isLoadingScheduledMessages, isFalse);
    expect(room.state.scheduledMessagesError, isNull);
  });

  test(
    'scheduled E2E retry replays the exact encrypted request after an ambiguous failure',
    () async {
      var encryptionCalls = 0;
      var scheduleCalls = 0;
      String? committedRequestBody;
      final requestBodies = <String>[];
      final client = MockClient((request) async {
        if (request.url.path != '/api/v1/messages/send') {
          return http.Response('{}', 404);
        }
        scheduleCalls++;
        requestBodies.add(request.body);
        if (scheduleCalls == 1) {
          // Model an accepted request whose response was lost. The server's
          // idempotency key can replay only the exact original request body.
          committedRequestBody = request.body;
          return http.Response('{}', 503);
        }
        if (request.body != committedRequestBody) {
          return http.Response('{}', 409);
        }
        return http.Response(
          jsonEncode({
            'scheduled_message': {
              'id': 'scheduled-e2e-1',
              'chat': {'id': 'chat-abc'},
              'sender_profile_id': 'prof-test',
              'client_message_id': 'e2e-scheduled-attempt-1',
              'send_when_online': true,
              'is_e2e': true,
              'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
              'payload': {'content': 'wire-ciphertext-1'},
            },
          }),
          200,
        );
      });
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(
              baseUrl: 'http://api.test',
              scheduledMessagesEnabled: true,
            ),
          ),
          httpClientProvider.overrideWithValue(client),
          isDeviceOfflineProvider.overrideWith((ref) => false),
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeEventProvider.overrideWith(
            (ref) => const Stream<RealtimeFrame>.empty(),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'another-chat'),
          chatListControllerProvider.overrideWith((ref) {
            return _E2eDmChatListController(ref);
          }),
          e2eMessageServiceProvider.overrideWithValue(
            _CountingE2eMessageService(() => ++encryptionCalls),
          ),
        ],
      );
      addTearDown(container.dispose);
      final roomProvider = chatRoomControllerProvider('chat-abc');
      final roomSubscription = container.listen(roomProvider, (_, _) {});
      addTearDown(roomSubscription.close);
      final room = container.read(roomProvider.notifier);

      final firstResult = await room.createScheduledMessage(
        content: 'Encrypted later',
        clientMessageId: 'e2e-scheduled-attempt-1',
        scheduledAt: null,
        sendWhenOnline: true,
      );
      expect(firstResult, isNotNull);
      expect(room.state.scheduledMessages, isEmpty);

      final retryResult = await room.createScheduledMessage(
        content: 'Encrypted later',
        clientMessageId: 'e2e-scheduled-attempt-1',
        scheduledAt: null,
        sendWhenOnline: true,
      );

      expect(retryResult, isNull);
      expect(encryptionCalls, 1);
      expect(scheduleCalls, 2);
      expect(requestBodies, hasLength(2));
      expect(requestBodies[1], requestBodies[0]);
      final sentBody = jsonDecode(requestBodies.first) as Map<String, dynamic>;
      expect(sentBody['content'], 'wire-ciphertext-1');
      expect(sentBody['is_e2e'], isTrue);
      expect(sentBody['client_message_id'], 'e2e-scheduled-attempt-1');
      expect(room.state.scheduledMessages.single.id, 'scheduled-e2e-1');
    },
  );

  test(
    'scheduled E2E create is single-flight while its response is pending',
    () async {
      final requestStarted = Completer<void>();
      final deferredResponse = Completer<http.Response>();
      var encryptionCalls = 0;
      var scheduleCalls = 0;
      final requestBodies = <String>[];
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(
              baseUrl: 'http://api.test',
              scheduledMessagesEnabled: true,
            ),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path != '/api/v1/messages/send') {
                return http.Response('{}', 404);
              }
              scheduleCalls++;
              requestBodies.add(request.body);
              if (!requestStarted.isCompleted) requestStarted.complete();
              return deferredResponse.future;
            }),
          ),
          isDeviceOfflineProvider.overrideWith((ref) => false),
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeEventProvider.overrideWith(
            (ref) => const Stream<RealtimeFrame>.empty(),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'another-chat'),
          chatListControllerProvider.overrideWith((ref) {
            return _E2eDmChatListController(ref);
          }),
          e2eMessageServiceProvider.overrideWithValue(
            _CountingE2eMessageService(() => ++encryptionCalls),
          ),
        ],
      );
      addTearDown(container.dispose);
      final roomProvider = chatRoomControllerProvider('chat-abc');
      final roomSubscription = container.listen(roomProvider, (_, _) {});
      addTearDown(roomSubscription.close);
      final room = container.read(roomProvider.notifier);

      final firstRequest = room.createScheduledMessage(
        content: 'Encrypted later',
        clientMessageId: 'e2e-scheduled-attempt-2',
        scheduledAt: null,
        sendWhenOnline: true,
      );
      await requestStarted.future;
      expect(room.state.isSending, isTrue);

      final concurrentAttempt = await room.createScheduledMessage(
        content: 'Encrypted later',
        clientMessageId: 'e2e-scheduled-attempt-2',
        scheduledAt: null,
        sendWhenOnline: true,
      );
      expect(concurrentAttempt, 'scheduled_send_in_progress');
      expect(scheduleCalls, 1);
      expect(encryptionCalls, 1);
      expect(requestBodies, hasLength(1));

      deferredResponse.complete(
        http.Response(
          jsonEncode({
            'scheduled_message': {
              'id': 'scheduled-e2e-2',
              'chat': {'id': 'chat-abc'},
              'sender_profile_id': 'prof-test',
              'client_message_id': 'e2e-scheduled-attempt-2',
              'send_when_online': true,
              'is_e2e': true,
              'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
              'payload': {'content': 'wire-ciphertext-1'},
            },
          }),
          200,
        ),
      );
      expect(await firstRequest, isNull);

      expect(room.state.isSending, isFalse);
      expect(room.state.scheduledMessages.single.id, 'scheduled-e2e-2');
      expect(scheduleCalls, 1);
      expect(encryptionCalls, 1);
    },
  );

  test(
    'scheduled list completion after controller disposal is ignored',
    () async {
      final requestStarted = Completer<void>();
      final deferredResponse = Completer<http.Response>();
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(
              baseUrl: 'http://api.test',
              scheduledMessagesEnabled: true,
            ),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/messages/scheduled') {
                if (!requestStarted.isCompleted) requestStarted.complete();
                return deferredResponse.future;
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeEventProvider.overrideWith(
            (ref) => const Stream<RealtimeFrame>.empty(),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        ],
      );
      final roomProvider = chatRoomControllerProvider('chat-abc');
      final roomSubscription = container.listen(roomProvider, (_, _) {});
      final room = container.read(roomProvider.notifier);
      final load = room.loadScheduledMessages();
      await requestStarted.future;

      roomSubscription.close();
      container.dispose();
      deferredResponse.complete(http.Response('{}', 200));
      await expectLater(load, completes);
    },
  );
}

const _sendDiagnostic = 'private-upstream-stack-for-text-send';
const _retryReply = VoiceMessage(
  id: 'reply-1',
  chatId: 'chat-abc',
  senderProfileId: 'peer-1',
  content: 'Original reply',
);
const _preexistingRetryHistory = VoiceMessage(
  id: 'history-before-retry',
  chatId: 'chat-abc',
  senderProfileId: 'peer-1',
  content: 'Existing visible history',
);

http.Response _failedSendResponse() =>
    http.Response(jsonEncode({'error': _sendDiagnostic}), 503);

http.Response _successfulSendResponse() => http.Response(
  jsonEncode({
    'message': {
      'id': 'accepted-1',
      'chat': {'id': 'chat-abc'},
      'sender_profile_id': 'prof-test',
      'content': 'Accepted text',
    },
  }),
  200,
);

class _ImmediateRetryHarness {
  _ImmediateRetryHarness({
    Future<http.Response> Function(http.Request, int)? send,
    bool encrypted = false,
    bool threadsEnabled = false,
  }) {
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/messages/send') {
        bodies.add(request.body);
        authorizations.add(request.headers['authorization']);
        return send == null
            ? bodies.length == 1
                  ? _failedSendResponse()
                  : _successfulSendResponse()
            : send(request, bodies.length);
      }
      return http.Response(
        '{}',
        request.url.path == '/api/v1/messages/read' ? 200 : 404,
      );
    });
    container = ProviderContainer(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        authControllerProvider.overrideWith(_ImmediateAuthController.new),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        messageCacheStoreProvider.overrideWithValue(
          InMemoryMessageCacheStore(),
        ),
        presenceProvider.overrideWith((ref, profileId) => null),
        realtimeAutoConnectProvider.overrideWithValue(false),
        realtimeEventProvider.overrideWith(
          (ref) => const Stream<RealtimeFrame>.empty(),
        ),
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        isDeviceOfflineProvider.overrideWith((ref) => false),
        selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
        chatListControllerProvider.overrideWith(
          threadsEnabled
              ? (ref) => _DmChatListController(ref, threadsEnabled: true)
              : _DmChatListController.new,
        ),
        chatE2eEnabledProvider('chat-abc').overrideWith((ref) => encrypted),
        e2eMessageServiceProvider.overrideWithValue(
          _CountingE2eMessageService(() => ++encryptionCalls),
        ),
        chatRoomControllerProvider(
          'chat-abc',
        ).overrideWith((ref) => _ImmediateRoomController(ref, 'chat-abc')),
      ],
    );
    container.listen(chatRoomControllerProvider('chat-abc'), (_, _) {});
  }

  late final ProviderContainer container;
  final bodies = <String>[];
  final authorizations = <String?>[];
  var encryptionCalls = 0;
  _ImmediateRoomController get room =>
      container.read(chatRoomControllerProvider('chat-abc').notifier)
          as _ImmediateRoomController;

  Future<void> pump(WidgetTester tester, {String locale = 'en'}) async {
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: Locale(locale),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  TextEditingController composer(WidgetTester tester) => tester
      .widget<TextField>(
        find.descendant(
          of: find.byKey(ChatRoomPanel.inputKey),
          matching: find.byType(TextField),
        ),
      )
      .controller!;
}

class _ImmediateRoomController extends ChatRoomController {
  _ImmediateRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(messages: [], historyProfileId: 'prof-test');
  }
  var historyLoads = 0;
  @override
  Future<void> loadInitial() async {
    historyLoads++;
  }

  @override
  Future<void> loadScheduledMessages({bool loadMore = false}) async {}
}

/// Models the existing public install-generation distinction: a token refresh
/// does not increment it, while a new installed same-actor login does.
class _ImmediateAuthController extends AuthController {
  _ImmediateAuthController(Ref ref)
    : super(
        authClient: ref.watch(voiceAuthClientProvider),
        storage: ref.watch(authSessionStorageProvider),
        guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
      ) {
    state = const AuthState(
      session: AuthSession(
        accessToken: 'test-access',
        refreshToken: 'test-refresh',
        accountId: 'acc-test',
        activeProfileId: 'prof-test',
        expiresInSeconds: 900,
      ),
    );
  }

  var _installedGeneration = 0;
  @override
  int get sessionInstallGeneration => _installedGeneration;

  void installSameActorSession() {
    _installedGeneration++;
    state = const AuthState(
      session: AuthSession(
        accessToken: 'new-login-access',
        refreshToken: 'new-login-refresh',
        accountId: 'acc-test',
        activeProfileId: 'prof-test',
        expiresInSeconds: 900,
      ),
    );
  }
}

class _EmptyRoomController extends ChatRoomController {
  _EmptyRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(messages: []);
  }

  @override
  Future<void> loadInitial() async {}
}

class _DmChatListController extends ChatListController {
  _DmChatListController(super.ref, {bool threadsEnabled = false}) : super() {
    state = ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
            threadsEnabled: threadsEnabled,
          ),
          dmPeerProfileId: 'peer-1',
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _ScheduledRoomController extends ChatRoomController {
  _ScheduledRoomController(super.ref, super.chatId, {this.firstAttemptResult})
    : super() {
    state = const ChatRoomState(messages: []);
  }

  final Completer<String?>? firstAttemptResult;

  final scheduledAttempts =
      <
        ({
          String content,
          String clientMessageId,
          DateTime? scheduledAt,
          bool sendWhenOnline,
        })
      >[];

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadScheduledMessages({bool loadMore = false}) async {}

  @override
  Future<String?> createScheduledMessage({
    required String content,
    required String clientMessageId,
    required DateTime? scheduledAt,
    required bool sendWhenOnline,
    List<MessageAttachment> attachments = const [],
    List<MessageMention> mentions = const [],
    String? threadParentId,
  }) async {
    scheduledAttempts.add((
      content: content,
      clientMessageId: clientMessageId,
      scheduledAt: scheduledAt,
      sendWhenOnline: sendWhenOnline,
    ));
    if (scheduledAttempts.length == 1 && firstAttemptResult != null) {
      state = state.copyWith(isSending: true, clearScheduledActionError: true);
      final error = await firstAttemptResult!.future;
      state = state.copyWith(
        isSending: false,
        scheduledActionError: error,
        clearScheduledActionError: error == null,
      );
      return error;
    }
    final error = scheduledAttempts.length == 1 ? 'unavailable' : null;
    state = state.copyWith(
      isSending: false,
      scheduledActionError: error,
      clearScheduledActionError: error == null,
    );
    return error;
  }
}

class _LoadingRoomController extends ChatRoomController {
  _LoadingRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(isLoading: true);
  }

  @override
  Future<void> loadInitial() async {}
}

class _BlockedDmChatListController extends ChatListController {
  _BlockedDmChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
          ),
          dmPeerProfileId: 'peer-1',
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _SpaceChatListController extends ChatListController {
  _SpaceChatListController(super.ref, {this.threadsEnabled = false}) : super() {
    state = ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_GROUP',
            creatorProfileId: 'owner',
            spaceId: 'space-1',
            threadsEnabled: threadsEnabled,
          ),
        ),
      ],
    );
  }

  final bool threadsEnabled;

  @override
  Future<void> loadInitial() async {}
}

class _UnreadTwoChatListController extends ChatListController {
  _UnreadTwoChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-a',
            type: 'CHAT_TYPE_GROUP',
            name: 'Chat A',
            creatorProfileId: 'prof-test',
          ),
          unreadCount: 1,
        ),
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-b',
            type: 'CHAT_TYPE_GROUP',
            name: 'Chat B',
            creatorProfileId: 'prof-test',
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _SingleMessageRoomController extends ChatRoomController {
  _SingleMessageRoomController(super.ref, super.chatId) : super() {
    state = ChatRoomState(
      messages: [
        VoiceMessage(
          id: '$chatId-message',
          chatId: chatId,
          senderProfileId: 'peer-1',
          content: 'Message for $chatId',
          createdAt: DateTime.parse('2026-09-20T00:00:00Z'),
        ),
      ],
      historyProfileId: 'prof-test',
    );
  }

  final deleteMessageCalls = <(String, bool)>[];

  @override
  Future<String?> deleteMessage(String messageId, {required bool forMe}) async {
    deleteMessageCalls.add((messageId, forMe));
    return null;
  }

  @override
  Future<void> loadInitial() async {}
}

class _TwoChatListController extends ChatListController {
  _TwoChatListController(super.ref) : super() {
    state = ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-a',
            type: 'CHAT_TYPE_GROUP',
            name: 'Chat A',
            creatorProfileId: 'prof-test',
          ),
        ),
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-b',
            type: 'CHAT_TYPE_GROUP',
            name: 'Chat B',
            creatorProfileId: 'prof-test',
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _ProfileBoundRoomController extends ChatRoomController {
  _ProfileBoundRoomController(super.ref, super.chatId) : super() {
    state = ChatRoomState(
      messages: [_message('profile-a-message', 'from profile A')],
      isLoading: true,
      nextCursor: 'profile-a-cursor',
      hasMore: true,
      historyProfileId: 'profile-a',
    );
  }

  void bindProfileB() {
    state = ChatRoomState(
      messages: [_message('profile-b-message', 'from profile B')],
      nextCursor: 'profile-b-cursor',
      hasMore: true,
      historyProfileId: 'profile-b',
    );
  }
}

class _ProfileBAuthController extends AuthController {
  _ProfileBAuthController(Ref ref)
    : super(
        authClient: ref.watch(voiceAuthClientProvider),
        storage: ref.watch(authSessionStorageProvider),
        guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
      ) {
    state = const AuthState(
      session: AuthSession(
        accessToken: 'profile-b-access',
        refreshToken: 'profile-b-refresh',
        accountId: 'account-1',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
  }
}

VoiceMessage _message(String id, String content) => VoiceMessage(
  id: id,
  chatId: 'chat-abc',
  senderProfileId: 'peer-1',
  content: content,
  createdAt: DateTime.parse('2026-09-20T00:00:00Z'),
);

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

class _E2eDmChatListController extends ChatListController {
  _E2eDmChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
            e2eEnabled: true,
          ),
          dmPeerProfileId: 'peer-1',
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _CountingE2eMessageService extends E2eMessageService {
  _CountingE2eMessageService(this._nextCall);

  final int Function() _nextCall;

  @override
  Future<String> encryptOutgoing({
    required String localProfileId,
    required String peerProfileId,
    required String plaintext,
    String? authorization,
    String? chatId,
  }) async => 'wire-ciphertext-${_nextCall()}';
}
