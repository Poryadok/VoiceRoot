import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

import 'support/live_gateway_harness.dart';

const _runExistingAccountLive = String.fromEnvironment(
  'VOICE_RUN_EXISTING_ACCOUNT_DM_WS',
  defaultValue: '',
);

/// Staging-only check using existing accounts; unlike the shared registration
/// harness, this test never creates accounts or changes privacy settings.
///
/// Set VOICE_LIVE_SENDER_EMAIL/PASSWORD and VOICE_LIVE_RECEIVER_EMAIL/PASSWORD
/// in the process environment, then run:
///
/// flutter test test/existing_accounts_dm_ws_live_test.dart \
///   --dart-define=VOICE_RUN_EXISTING_ACCOUNT_DM_WS=true \
///   --dart-define=VOICE_API_BASE_URL=https://voice.comrade.click
void main() {
  test(
    'existing mutual DM delivers a sent message over the receiver WebSocket',
    () async {
      final senderEmail = _requiredEnvironment('VOICE_LIVE_SENDER_EMAIL');
      final senderPassword = _requiredEnvironment('VOICE_LIVE_SENDER_PASSWORD');
      final receiverEmail = _requiredEnvironment('VOICE_LIVE_RECEIVER_EMAIL');
      final receiverPassword = _requiredEnvironment(
        'VOICE_LIVE_RECEIVER_PASSWORD',
      );
      final senderTotpCode = _optionalEnvironment(
        'VOICE_LIVE_SENDER_TOTP_CODE',
      );
      final receiverTotpCode = _optionalEnvironment(
        'VOICE_LIVE_RECEIVER_TOTP_CODE',
      );

      final config = GatewayConfig(baseUrl: liveGatewayBaseUrl());
      final httpClient = http.Client();
      final gateway = GatewayHttpClient(httpClient: httpClient, config: config);
      final auth = VoiceAuthClient(gateway: gateway);
      AuthSession? senderSession;
      AuthSession? receiverSession;
      addTearDown(() async {
        final cleanupErrors = <String>[];
        final receiver = receiverSession;
        if (receiver != null) {
          final error = await _logoutTestSession(config, receiver, 'receiver');
          if (error != null) cleanupErrors.add(error);
        }
        final sender = senderSession;
        if (sender != null) {
          final error = await _logoutTestSession(config, sender, 'sender');
          if (error != null) cleanupErrors.add(error);
        }
        httpClient.close();
        expect(cleanupErrors, isEmpty, reason: cleanupErrors.join('; '));
      });

      senderSession = _expectLogin(
        await auth.login(
          email: senderEmail,
          password: senderPassword,
          totpCode: senderTotpCode,
        ),
        'sender',
      );
      receiverSession = _expectLogin(
        await auth.login(
          email: receiverEmail,
          password: receiverPassword,
          totpCode: receiverTotpCode,
        ),
        'receiver',
      );

      final chats = VoiceChatsClient(gateway: gateway);
      final senderDms = await _listMutualDmPeers(
        chats,
        senderSession,
        receiverSession.activeProfileId,
      );
      final receiverDms = await _listMutualDmPeers(
        chats,
        receiverSession,
        senderSession.activeProfileId,
      );
      final receiverChatIds = receiverDms.map((item) => item.chatId).toSet();
      final sharedDmIds = senderDms
          .where((item) => receiverChatIds.contains(item.chatId))
          .map((item) => item.chatId)
          .toSet();
      if (sharedDmIds.length != 1) {
        fail(
          'Expected exactly one existing mutual DM between the supplied '
          'accounts; found ${sharedDmIds.length}. No message was sent.',
        );
      }
      final chatId = sharedDmIds.single;

      final realtime = VoiceRealtimeConnection(
        uri: gatewayWebSocketUri(config.baseUrl),
        headers: {'Authorization': receiverSession.authorizationHeader},
      );
      addTearDown(realtime.dispose);
      await realtime.connect();
      await waitForOp(realtime.events, 'hello');
      realtime.sendSubscribe(chatId);
      await waitForOp(
        realtime.events,
        'subscribe_ack',
        where: (frame) => frame.data?['chat_id'] == chatId,
      );

      final seenMessageFrames = <RealtimeFrame>[];
      final messageFrame = Completer<RealtimeFrame>();
      String? sentMessageId;
      final frameSubscription = realtime.events.listen((frame) {
        if (frame.op != 'message_create' || frame.data?['chat_id'] != chatId) {
          return;
        }
        seenMessageFrames.add(frame);
        if (sentMessageId != null &&
            frame.data?['message_id'] == sentMessageId &&
            !messageFrame.isCompleted) {
          messageFrame.complete(frame);
        }
      });
      addTearDown(frameSubscription.cancel);

      final messages = VoiceMessagesClient(gateway: gateway);
      final clientMessageId = qaClientMessageId();
      final content = 'A1 no-refresh delivery check $clientMessageId';
      final sendResult = await messages.sendMessage(
        authorization: senderSession.authorizationHeader,
        chatId: chatId,
        content: content,
        clientMessageId: clientMessageId,
      );
      expect(
        sendResult,
        isA<MessagesApiOk<VoiceMessage>>(),
        reason: 'send in the identified existing DM',
      );
      final sent = (sendResult as MessagesApiOk<VoiceMessage>).data;
      expect(sent.chatId, chatId);
      expect(sent.content, content);

      sentMessageId = sent.id;
      for (final frame in seenMessageFrames) {
        if (frame.data?['message_id'] == sentMessageId &&
            !messageFrame.isCompleted) {
          messageFrame.complete(frame);
        }
      }
      final frame = await messageFrame.future.timeout(
        const Duration(seconds: 8),
        onTimeout: () => throw TestFailure(
          'timed out waiting for the message_create matching the sent message',
        ),
      );
      expect(frame.data?['chat_id'], chatId);
      expect(frame.data?['message_id'], sent.id);
      expect(frame.data?['sender_profile_id'], senderSession.activeProfileId);
      expect(frame.sequence, isNotNull);
    },
    timeout: const Timeout(Duration(minutes: 2)),
    skip: _runExistingAccountLive == 'true'
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_EXISTING_ACCOUNT_DM_WS=true',
  );
}

String _requiredEnvironment(String name) {
  final value = _optionalEnvironment(name);
  if (value == null || value.isEmpty) {
    fail('Set $name in the test process environment.');
  }
  return value;
}

String? _optionalEnvironment(String name) {
  final value = Platform.environment[name]?.trim();
  return value == null || value.isEmpty ? null : value;
}

AuthSession _expectLogin(AuthSessionResult result, String role) {
  if (result is AuthSessionOk) return result.session;
  if (result is AuthSessionFailure) {
    fail(
      '$role login failed: HTTP ${result.statusCode ?? 'unknown'} '
      '(error ${result.errorCode ?? 'unknown'}).',
    );
  }
  fail('$role login returned an unexpected result.');
}

Future<String?> _logoutTestSession(
  GatewayConfig config,
  AuthSession session,
  String role,
) async {
  final cleanupClient = http.Client();
  try {
    final auth = VoiceAuthClient(
      gateway: GatewayHttpClient(httpClient: cleanupClient, config: config),
    );
    final error = await auth.logout(session: session);
    return error == null ? null : '$role logout failed: $error';
  } on Object catch (error) {
    return '$role logout request failed: ${error.runtimeType}';
  } finally {
    cleanupClient.close();
  }
}

Future<List<ChatListItem>> _listMutualDmPeers(
  VoiceChatsClient chats,
  AuthSession session,
  String otherProfileId,
) async {
  final items = <ChatListItem>[];
  final seenCursors = <String>{};
  String? cursor;
  do {
    final result = await chats.listChats(
      authorization: session.authorizationHeader,
      inbox: 'main',
      pageSize: 100,
      cursor: cursor,
    );
    if (result is! ChatsApiOk<ChatListData>) {
      fail('ListChats failed for one account; no message was sent.');
    }
    items.addAll(
      result.data.items.where(
        (item) => item.chat.isDm && item.dmPeerProfileId == otherProfileId,
      ),
    );
    cursor = result.data.nextCursor;
    if (cursor != null && !seenCursors.add(cursor)) {
      fail('ListChats repeated a cursor; no message was sent.');
    }
  } while (cursor != null && cursor.isNotEmpty);
  return items;
}
