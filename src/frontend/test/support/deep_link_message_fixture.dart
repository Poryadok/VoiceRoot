import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'fake_voice_api_clients.dart';

/// Seeds an existing target rather than testing a jump against empty history.
Override deepLinkMessageHistoryOverride({
  required String chatId,
  required String messageId,
}) => voiceMessagesClientProvider.overrideWithValue(
  _DeepLinkMessageClient(
    VoiceMessage(
      id: messageId,
      chatId: chatId,
      senderProfileId: 'deep-link-peer',
      content: 'Deep link target $messageId',
    ),
  ),
);

class _DeepLinkMessageClient extends FakeVoiceMessagesClient {
  _DeepLinkMessageClient(this.message);

  final VoiceMessage message;

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async => MessagesApiOk(
    MessageListData(messages: chatId == message.chatId ? [message] : const []),
  );
}

/// Checks the rendered destination after the one-shot request is consumed.
Future<void> expectDeepLinkMessageVisible(
  WidgetTester tester, {
  required ProviderContainer container,
  required String chatId,
  required String messageId,
}) async {
  final profileId = container.read(authControllerProvider).activeProfileId;
  final target = find.byKey(ValueKey('view:$profileId:$messageId'));
  for (var frame = 0; frame < 30; frame++) {
    if (container.read(pendingChatMessageScrollProvider(chatId)) == null &&
        target.evaluate().isNotEmpty) {
      break;
    }
    await tester.pump(const Duration(milliseconds: 100));
  }
  expect(container.read(pendingChatMessageScrollProvider(chatId)), isNull);
  expect(container.read(pendingChatMessageHighlightProvider(chatId)), isNull);
  expect(target, findsOneWidget);
  final box = tester.renderObject<RenderBox>(target);
  final bounds = box.localToGlobal(Offset.zero) & box.size;
  final viewport = tester.getRect(find.byKey(ChatRoomPanel.messagesKey));
  expect(bounds.top, greaterThanOrEqualTo(viewport.top));
  expect(bounds.bottom, lessThanOrEqualTo(viewport.bottom));
}
