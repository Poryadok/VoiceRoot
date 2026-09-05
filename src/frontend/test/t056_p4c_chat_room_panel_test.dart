import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as messaging_pb;
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/ui/chat/chat_composer_text_field.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/core/voice_send_button.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/markdown_test_helpers.dart';
import 'support/voice_test_theme.dart';

const _dmPeerDeletedMarkerKey = ValueKey<String>('chat_room_dm_peer_deleted');

void main() {
  testWidgets(
    'deleted DM renders one stable marker without changing loaded messages',
    (tester) async {
      final harness = await _pumpPanel(tester, locale: const Locale('ru'));
      final room = harness.container.read(chatRoomControllerProvider('chat-1'));
      final ids = room.messages.map((message) => message.id).toList();

      _expectDeletedMarker(tester);
      expect(room.isDmPeerDeleted, isTrue);
      expect(room.messages, hasLength(1));
      expect(room.messages.map((message) => message.id), ids);

      harness.realtime.emit(_deletedFrame());
      harness.realtime.emit(_deletedFrame());
      await tester.pumpAndSettle();

      _expectDeletedMarker(tester);
      final after = harness.container.read(
        chatRoomControllerProvider('chat-1'),
      );
      expect(after.messages.map((message) => message.id), ids);
      expect(after.messages, hasLength(1));
    },
  );

  testWidgets('deleted marker is absent without loaded history', (
    tester,
  ) async {
    final harness = await _pumpPanel(
      tester,
      page: const MessageListData(
        messages: [],
        nextCursor: 'cursor-empty',
        hasMore: true,
        dmPeerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
      ),
    );

    final room = harness.container.read(chatRoomControllerProvider('chat-1'));
    expect(room.messages, isEmpty);
    expect(room.isDmPeerDeleted, isFalse);
    expect(find.byKey(_dmPeerDeletedMarkerKey), findsNothing);
    expect(find.text('User deleted'), findsNothing);
  });

  testWidgets(
    'deleted DM makes composer controls read-only and no-op before side effects',
    (tester) async {
      final picked = <bool>[];
      final harness = await _pumpPanel(
        tester,
        attachmentPicker: () async {
          picked.add(true);
          return ChatAttachmentFile(
            bytes: Uint8List.fromList([1]),
            contentType: 'text/plain',
            name: 'blocked.txt',
          );
        },
      );
      final mutationBaseline = harness.cache.mutationSignatures();

      final composer = tester.widget<ChatComposerTextField>(
        find.byKey(ChatRoomPanel.inputKey),
      );
      expect(composer.readOnly, isTrue);
      expect(composer.onSend, isNull);
      expect(
        tester
            .widget<VoiceSendButton>(find.byKey(ChatRoomPanel.sendKey))
            .onPressed,
        isNull,
      );
      expect(
        tester
            .widget<IconButton>(find.byKey(ChatRoomPanel.emojiPickerKey))
            .onPressed,
        isNull,
      );
      expect(
        tester
            .widget<IconButton>(find.byKey(ChatRoomPanel.attachKey))
            .onPressed,
        isNull,
      );

      await tester.tap(find.byKey(ChatRoomPanel.sendKey));
      await tester.tap(find.byKey(ChatRoomPanel.attachKey));
      await tester.pumpAndSettle();

      expect(harness.realtime.typingStopCalls, 0);
      expect(harness.messages.sendCalls, 0);
      expect(harness.messages.forwardCalls, 0);
      expect(picked, isEmpty);
      expect(harness.cache.mutationSignatures(), mutationBaseline);
    },
  );

  testWidgets(
    'stale enabled send callback exits before mention lookup and side effects',
    (tester) async {
      final mentionLookups = <bool>[];
      final harness = await _pumpPanel(
        tester,
        page: _page(),
        mentionLookups: mentionLookups,
      );
      final sendCallback = tester
          .widget<ChatComposerTextField>(find.byKey(ChatRoomPanel.inputKey))
          .onSend!;
      final cacheBaseline = harness.cache.mutationSignatures();

      final controller = harness.container.read(
        chatRoomControllerProvider('chat-1').notifier,
      );
      controller.state = controller.state.copyWith(isDmPeerDeleted: true);
      await tester.pump();

      sendCallback();
      await tester.pumpAndSettle();

      expect(harness.realtime.typingStopCalls, 0);
      expect(mentionLookups, isEmpty);
      expect(harness.messages.sendCalls, 0);
      expect(harness.messages.forwardCalls, 0);
      expect(harness.cache.mutationSignatures(), cacheBaseline);
    },
  );

  testWidgets(
    'stale enabled attach callback exits before picker and upload side effects',
    (tester) async {
      final picked = <bool>[];
      final harness = await _pumpPanel(
        tester,
        page: _page(),
        attachmentPicker: () async {
          picked.add(true);
          return ChatAttachmentFile(
            bytes: Uint8List.fromList([1]),
            contentType: 'text/plain',
            name: 'stale.txt',
          );
        },
      );
      final attachCallback = tester
          .widget<IconButton>(find.byKey(ChatRoomPanel.attachKey))
          .onPressed!;
      final cacheBaseline = harness.cache.mutationSignatures();

      final controller = harness.container.read(
        chatRoomControllerProvider('chat-1').notifier,
      );
      attachCallback();
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('composer_attach_menu')), findsOneWidget);

      controller.state = controller.state.copyWith(isDmPeerDeleted: true);
      await tester.pump();
      await tester.tap(find.byKey(const Key('composer_attach_document')));
      await tester.pumpAndSettle();

      expect(picked, isEmpty);
      expect(harness.messages.sendCalls, 0);
      expect(harness.messages.forwardCalls, 0);
      expect(harness.files.requestUploadCalls, 0);
      expect(harness.files.putBytesCalls, 0);
      expect(harness.files.confirmUploadCalls, 0);
      expect(harness.cache.mutationSignatures(), cacheBaseline);
    },
  );

  testWidgets('deleted DM message actions hide Forward without initiating it', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    final harness = await _pumpPanel(tester);
    await tester.longPress(messagePlainTextFinder('History survives'));
    await tester.pumpAndSettle();

    expect(find.text('Forward'), findsNothing);
    expect(harness.messages.forwardCalls, 0);
  });
}

Future<_UiHarness> _pumpPanel(
  WidgetTester tester, {
  MessageListData? page,
  Locale locale = const Locale('en'),
  ChatAttachmentPicker? attachmentPicker,
  List<bool>? mentionLookups,
}) async {
  final messages = _UiMessagesClient(
    page:
        page ??
        _page(peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED),
  );
  final realtime = _UiRealtimeHub();
  final cache = _UiCacheStore();
  final files = _UiFilesClient();
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => httpResponse404()),
      ),
      voiceMessagesClientProvider.overrideWithValue(messages),
      voiceFilesClientProvider.overrideWithValue(files),
      realtimeHubProvider.overrideWithValue(realtime),
      messageCacheStoreProvider.overrideWithValue(cache),
      presenceProvider.overrideWith((ref, id) => null),
      if (mentionLookups != null)
        groupMembersProvider('chat-1').overrideWith((ref) async {
          mentionLookups.add(true);
          return const MemberListData(members: []);
        }),
    ],
  );
  addTearDown(container.dispose);

  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: locale,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: ChatRoomPanel(
            chatId: 'chat-1',
            attachmentPicker: attachmentPicker,
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return _UiHarness(
    container: container,
    messages: messages,
    files: files,
    realtime: realtime,
    cache: cache,
  );
}

MessageListData _page({messaging_pb.DmPeerState? peerState}) {
  return MessageListData(
    messages: [
      VoiceMessage(
        id: 'msg-1',
        chatId: 'chat-1',
        senderProfileId: 'peer-1',
        content: 'History survives',
        createdAt: DateTime.parse('2024-01-01T00:00:00Z'),
      ),
    ],
    nextCursor: 'cursor-older',
    hasMore: true,
    dmPeerState: peerState,
  );
}

RealtimeFrame _deletedFrame() {
  return const RealtimeFrame(
    op: 'dm_peer_deleted',
    data: {'chat_id': 'chat-1', 'recipient_profile_id': 'prof-test'},
  );
}

void _expectDeletedMarker(WidgetTester tester) {
  final marker = find.byKey(_dmPeerDeletedMarkerKey);
  expect(marker, findsOneWidget);
  expect(
    find.descendant(of: marker, matching: find.text('Пользователь удалён')),
    findsOneWidget,
  );
}

http.Response httpResponse404() => http.Response('{}', 404);

class _UiHarness {
  const _UiHarness({
    required this.container,
    required this.messages,
    required this.files,
    required this.realtime,
    required this.cache,
  });

  final ProviderContainer container;
  final _UiMessagesClient messages;
  final _UiFilesClient files;
  final _UiRealtimeHub realtime;
  final _UiCacheStore cache;
}

class _UiMessagesClient extends VoiceMessagesClient {
  _UiMessagesClient({required this.page})
    : super(
        gateway: gatewayHttpForTest(MockClient((_) async => httpResponse404())),
      );

  final MessageListData page;
  var sendCalls = 0;
  var forwardCalls = 0;

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async => MessagesApiOk(page);

  @override
  Future<MessagesApiResult<MessageListData>> getPinnedMessages({
    required String authorization,
    required String chatId,
  }) async => const MessagesApiOk(MessageListData(messages: []));

  @override
  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async => const MessagesApiOk(null);

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
  }) async {
    sendCalls++;
    return const MessagesApiFailure(message: 'unexpected_send');
  }

  @override
  Future<MessagesApiResult<VoiceMessage>> forwardMessage({
    required String authorization,
    required String sourceMessageId,
    required String targetChatId,
    String? commentary,
    bool withoutAttribution = false,
  }) async {
    forwardCalls++;
    return const MessagesApiFailure(message: 'unexpected_forward');
  }
}

class _UiFilesClient extends VoiceFilesClient {
  _UiFilesClient()
    : super(
        gateway: gatewayHttpForTest(MockClient((_) async => httpResponse404())),
      );

  var requestUploadCalls = 0;
  var putBytesCalls = 0;
  var confirmUploadCalls = 0;

  @override
  Future<FilesApiResult<FileUploadTicket>> requestUpload({
    required String authorization,
    required String originalName,
    required String mimeType,
    required int sizeBytes,
    String? chatId,
    String? chatType,
    String? storyId,
    bool isE2e = false,
  }) async {
    requestUploadCalls++;
    return const FilesApiFailure(message: 'unexpected_upload_request');
  }

  @override
  Future<FilesApiResult<void>> putBytes({
    required Uri uploadUrl,
    required Uint8List bytes,
    required String mimeType,
  }) async {
    putBytesCalls++;
    return const FilesApiFailure(message: 'unexpected_put');
  }

  @override
  Future<FilesApiResult<FileMetadataData>> confirmUpload({
    required String authorization,
    required String fileId,
    required Uint8List bytes,
  }) async {
    confirmUploadCalls++;
    return const FilesApiFailure(message: 'unexpected_confirm');
  }
}

class _UiRealtimeHub extends RealtimeHub {
  _UiRealtimeHub() : super(_UiUnwiredRef());

  final _events = StreamController<RealtimeFrame>.broadcast();
  var typingStartCalls = 0;
  var typingStopCalls = 0;

  @override
  Stream<RealtimeFrame> get events => _events.stream;

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  void typingStart(String chatId) => typingStartCalls++;

  @override
  void typingStop(String chatId) => typingStopCalls++;

  void emit(RealtimeFrame frame) => _events.add(frame);

  @override
  Future<void> dispose() => _events.close();
}

class _UiUnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}

class _UiCacheStore implements MessageCacheStore {
  final _delegate = InMemoryMessageCacheStore();
  final mutations = <String>[];

  List<String> mutationSignatures() => List.unmodifiable(mutations);

  @override
  Future<void> clearAll() async {
    mutations.add('clearAll');
    await _delegate.clearAll();
  }

  @override
  Future<void> clearProfile(String profileId) async {
    mutations.add('clearProfile:$profileId');
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
      'replace:$profileId:$chatId:${messages.map((m) => m.id).join(',')}',
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
      'upsert:$profileId:$chatId:${messages.map((m) => m.id).join(',')}',
    );
    await _delegate.upsertMessages(
      profileId: profileId,
      chatId: chatId,
      messages: messages,
    );
  }
}
