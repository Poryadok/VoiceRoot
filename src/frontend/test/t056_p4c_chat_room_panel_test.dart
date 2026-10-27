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
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as messaging_pb;
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/ui/chat/chat_composer_text_field.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/music_playback.dart';
import 'package:voice_frontend/ui/core/voice_send_button.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/markdown_test_helpers.dart';
import 'support/voice_test_theme.dart';

const _dmPeerDeletedMarkerKey = ValueKey<String>('chat_room_dm_peer_deleted');
const _expiredAttachmentPlaceholderKey = ValueKey<String>(
  'expired_attachment_placeholder_file-expired',
);
const _expiredAttachmentTooltip =
    'Файл удалён. Подписка сохраняет файлы навсегда';

void main() {
  for (final transition in ['same-profile-login', 'failed-profile-switch']) {
    for (final pending in [false, true]) {
      testWidgets('Music retires $transition with pending=$pending', (
        tester,
      ) async {
        final playback = _SessionMusicPlayback();
        final harness = await _pumpPanel(
          tester,
          page: _page(
            attachments: const [
              MessageAttachment(fileId: 'music', type: 'music'),
            ],
          ),
          fileMetadataResult: const FilesApiOk(
            FileMetadataData(
              fileId: 'music',
              fileType: 'audio',
              status: 'ready',
              originalName: 'track.mp3',
              sizeBytes: 4,
              isMusic: true,
            ),
          ),
          musicPlayback: () => playback,
        );
        final bytes = Uint8List.fromList([0xff, 0xfb, 0x90, 0]);
        final deferred = Completer<FilesApiResult<Uint8List>>();
        harness.files.downloadBytes = bytes;
        if (pending) harness.files.pendingDownload = deferred;
        await tester.tap(find.byKey(const ValueKey('music_toggle_music')));
        await tester.pump();
        if (!pending) await tester.pumpAndSettle();
        if (transition == 'same-profile-login') {
          final auth = harness.container.read(authControllerProvider.notifier);
          await auth.applySession(
            const AuthSession(
              accessToken: 'new-login',
              refreshToken: 'new-refresh',
              accountId: 'acc-test',
              activeProfileId: 'prof-test',
              expiresInSeconds: 900,
            ),
          );
        } else {
          harness.container
                  .read(profileSwitchInProgressProvider.notifier)
                  .state =
              true;
        }
        await tester.pump();
        if (transition == 'failed-profile-switch') {
          harness.container
                  .read(profileSwitchInProgressProvider.notifier)
                  .state =
              false;
          await tester.pump();
        }
        if (pending) deferred.complete(FilesApiOk(bytes));
        await tester.pumpAndSettle();
        expect(bytes, everyElement(0));
        expect(playback.opens, pending ? 0 : 1);
        if (!pending) expect(playback.closed, isTrue);
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
        harness.container.dispose();
      });
    }
  }
  testWidgets('expired attachment shows the documented placeholder', (
    tester,
  ) async {
    final harness = await _pumpPanel(
      tester,
      locale: const Locale('ru'),
      page: _page(
        attachments: const [
          MessageAttachment(
            fileId: 'file-expired',
            type: 'document',
            name: 'retained.pdf',
          ),
        ],
      ),
      fileMetadataResult: const FilesApiOk(
        FileMetadataData(
          fileId: 'file-expired',
          fileType: 'document',
          status: 'expired',
          originalName: 'retained.pdf',
        ),
      ),
    );

    expect(find.byKey(_expiredAttachmentPlaceholderKey), findsOneWidget);
    expect(find.text('retained.pdf'), findsNothing);
    expect(harness.files.metadataRequests, ['file-expired']);
    expect(
      harness.container
          .read(chatRoomControllerProvider('chat-1'))
          .messages
          .single
          .attachments
          .single
          .fileId,
      'file-expired',
    );

    await tester.tap(find.byKey(_expiredAttachmentPlaceholderKey));
    await tester.pumpAndSettle();
    expect(find.text(_expiredAttachmentTooltip), findsOneWidget);
  });

  testWidgets('ready attachment keeps its regular rendering', (tester) async {
    final harness = await _pumpPanel(
      tester,
      page: _page(
        attachments: const [
          MessageAttachment(
            fileId: 'file-ready',
            type: 'document',
            name: 'ready.pdf',
          ),
        ],
      ),
      fileMetadataResult: const FilesApiOk(
        FileMetadataData(
          fileId: 'file-ready',
          fileType: 'document',
          status: 'ready',
          originalName: 'ready.pdf',
        ),
      ),
    );

    expect(find.byKey(_expiredAttachmentPlaceholderKey), findsNothing);
    expect(find.text('ready.pdf'), findsOneWidget);
    expect(harness.files.metadataRequests, ['file-ready']);
  });

  testWidgets('unavailable metadata does not imply an expired attachment', (
    tester,
  ) async {
    final harness = await _pumpPanel(
      tester,
      page: _page(
        attachments: const [
          MessageAttachment(
            fileId: 'file-unavailable',
            type: 'document',
            name: 'unavailable.pdf',
          ),
        ],
      ),
      fileMetadataResult: const FilesApiFailure(
        message: 'not_found',
        statusCode: 404,
      ),
    );

    expect(find.byKey(_expiredAttachmentPlaceholderKey), findsNothing);
    expect(find.text('unavailable.pdf'), findsOneWidget);
    expect(harness.files.metadataRequests, ['file-unavailable']);
  });

  testWidgets('session reload does not show a previous expired result', (
    tester,
  ) async {
    final harness = await _pumpPanel(
      tester,
      page: _page(
        attachments: const [
          MessageAttachment(
            fileId: 'file-expired',
            type: 'document',
            name: 'retained.pdf',
          ),
        ],
      ),
      fileMetadataResult: const FilesApiOk(
        FileMetadataData(
          fileId: 'file-expired',
          fileType: 'document',
          status: 'expired',
          originalName: 'retained.pdf',
        ),
      ),
    );
    expect(find.byKey(_expiredAttachmentPlaceholderKey), findsOneWidget);

    final pendingMetadata = Completer<FilesApiResult<FileMetadataData>>();
    harness.files.pendingMetadataResult = pendingMetadata;
    final auth = harness.container.read(authControllerProvider.notifier);
    auth.state = auth.state.copyWith(
      session: const AuthSession(
        accessToken: 'other-session',
        refreshToken: 'other-refresh',
        accountId: 'acc-test',
        activeProfileId: 'prof-test',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();

    expect(harness.files.metadataRequests, ['file-expired', 'file-expired']);
    expect(
      harness.container
          .read(fileAttachmentMetadataProvider('file-expired'))
          .valueOrNull
          ?.status,
      'expired',
    );
    expect(find.byKey(_expiredAttachmentPlaceholderKey), findsNothing);

    pendingMetadata.complete(
      const FilesApiOk(
        FileMetadataData(
          fileId: 'file-expired',
          fileType: 'document',
          status: 'ready',
          originalName: 'retained.pdf',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(_expiredAttachmentPlaceholderKey), findsNothing);
    expect(find.text('retained.pdf'), findsOneWidget);
  });

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

  testWidgets('deleted DM marker is localized through the English app locale', (
    tester,
  ) async {
    final harness = await _pumpPanel(tester, locale: const Locale('en'));

    expect(
      AppLocalizations.of(tester.element(find.byKey(_dmPeerDeletedMarkerKey))),
      isNotNull,
    );
    expect(
      find.descendant(
        of: find.byKey(_dmPeerDeletedMarkerKey),
        matching: find.text('User deleted'),
      ),
      findsOneWidget,
    );
    expect(
      harness.container
          .read(chatRoomControllerProvider('chat-1'))
          .isDmPeerDeleted,
      isTrue,
    );
  });

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

  for (final invalid in ['empty', 'oversize', 'not-audio']) {
    testWidgets('Music rejects $invalid before upload or send', (tester) async {
      final harness = await _pumpPanel(
        tester,
        page: _page(),
        attachmentPicker: () async => ChatAttachmentFile(
          bytes: Uint8List(
            invalid == 'empty'
                ? 0
                : invalid == 'oversize'
                ? 10 * 1024 * 1024 + 1
                : 1,
          ),
          contentType: invalid == 'not-audio' ? 'text/plain' : 'audio/mpeg',
          name: invalid == 'not-audio' ? 'note.txt' : 'track.mp3',
        ),
      );
      await tester.tap(find.byKey(ChatRoomPanel.attachKey));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('composer_attach_music')));
      await tester.pumpAndSettle();
      expect(harness.files.requestUploadCalls, 0);
      expect(harness.files.putBytesCalls, 0);
      expect(harness.messages.sendCalls, 0);
      expect(find.textContaining('10 MB'), findsOneWidget);
    });
  }

  for (final attachEntry in const <String, Key>{
    'document': Key('composer_attach_document'),
    'music': Key('composer_attach_music'),
    'photo or video': Key('composer_attach_photo_or_video'),
  }.entries) {
    testWidgets(
      'selected ${attachEntry.key} is discarded when the DM becomes deleted while picker is pending',
      (tester) async {
        final selected = Completer<ChatAttachmentFile?>();
        var pickerCalls = 0;
        final harness = await _pumpPanel(
          tester,
          page: _page(),
          attachmentPicker: () {
            pickerCalls++;
            return selected.future;
          },
        );
        final cacheBaseline = harness.cache.mutationSignatures();

        await tester.tap(find.byKey(ChatRoomPanel.attachKey));
        await tester.pump();
        expect(find.byKey(const Key('composer_attach_menu')), findsOneWidget);
        await tester.tap(find.byKey(attachEntry.value));
        await tester.pump();
        await tester.pump();
        expect(pickerCalls, 1);

        final controller = harness.container.read(
          chatRoomControllerProvider('chat-1').notifier,
        );
        controller.state = controller.state.copyWith(isDmPeerDeleted: true);
        await tester.pump();

        selected.complete(
          ChatAttachmentFile(
            bytes: Uint8List.fromList([1]),
            contentType: 'text/plain',
            name: 'late-${attachEntry.key}.txt',
          ),
        );
        await tester.pump();
        await tester.pump();

        expect(harness.files.requestUploadCalls, 0);
        expect(harness.files.putBytesCalls, 0);
        expect(harness.files.confirmUploadCalls, 0);
        expect(harness.messages.sendCalls, 0);
        expect(harness.messages.forwardCalls, 0);
        expect(harness.cache.mutationSignatures(), cacheBaseline);
        expect(find.byKey(_dmPeerDeletedMarkerKey), findsOneWidget);
        final composer = tester.widget<ChatComposerTextField>(
          find.byKey(ChatRoomPanel.inputKey),
        );
        expect(composer.readOnly, isTrue);
        expect(composer.onSend, isNull);
      },
    );
  }

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
  FilesApiResult<FileMetadataData>? fileMetadataResult,
  MusicPlayback Function()? musicPlayback,
}) async {
  final messages = _UiMessagesClient(
    page:
        page ??
        _page(peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED),
  );
  final realtime = _UiRealtimeHub();
  final cache = _UiCacheStore();
  final files = _UiFilesClient(metadataResult: fileMetadataResult);
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => httpResponse404()),
      ),
      voiceMessagesClientProvider.overrideWithValue(messages),
      voiceFilesClientProvider.overrideWithValue(files),
      if (musicPlayback != null)
        musicPlaybackFactoryProvider.overrideWithValue(musicPlayback),
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

MessageListData _page({
  messaging_pb.DmPeerState? peerState,
  List<MessageAttachment> attachments = const [],
}) {
  return MessageListData(
    messages: [
      VoiceMessage(
        id: 'msg-1',
        chatId: 'chat-1',
        senderProfileId: 'peer-1',
        content: 'History survives',
        attachments: attachments,
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
    bool sendSilent = false,
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
  _UiFilesClient({this.metadataResult})
    : super(
        gateway: gatewayHttpForTest(MockClient((_) async => httpResponse404())),
      );

  final FilesApiResult<FileMetadataData>? metadataResult;
  Completer<FilesApiResult<FileMetadataData>>? pendingMetadataResult;
  final metadataRequests = <String>[];
  var requestUploadCalls = 0;
  var putBytesCalls = 0;
  var confirmUploadCalls = 0;
  Completer<FilesApiResult<Uint8List>>? pendingDownload;
  Uint8List? downloadBytes;

  @override
  Future<FilesApiResult<Uint8List>> fetchFileBytes({
    required String authorization,
    required String fileId,
    FileUrlVariant variant = FileUrlVariant.original,
    int? maxBytes,
  }) async {
    expect(maxBytes, 10 * 1024 * 1024);
    if (pendingDownload != null) return pendingDownload!.future;
    return FilesApiOk(downloadBytes!);
  }

  @override
  Future<FilesApiResult<FileMetadataData>> getFileMetadata({
    required String authorization,
    required String fileId,
  }) async {
    metadataRequests.add(fileId);
    final pending = pendingMetadataResult;
    if (pending != null) {
      pendingMetadataResult = null;
      return pending.future;
    }
    return metadataResult ??
        FilesApiFailure(message: 'unexpected_metadata', statusCode: 404);
  }

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
    bool music = false,
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

class _SessionMusicPlayback extends MusicPlayback {
  int opens = 0;
  @override
  bool closed = false;
  @override
  bool get failed => false;
  @override
  bool playing = false;
  @override
  Duration get position => Duration.zero;
  @override
  Duration get duration => const Duration(seconds: 1);
  @override
  String? get title => null;
  @override
  String? get artist => null;
  @override
  String? get album => null;
  @override
  Future<void> open(Uint8List bytes) async {
    opens++;
    playing = true;
  }

  @override
  void pause() {
    playing = false;
  }

  @override
  void resume() {
    playing = true;
  }

  @override
  void seek(Duration position) {}
  @override
  Future<void> close() async {
    closed = true;
    playing = false;
  }

  @override
  void dispose() {
    unawaited(close());
    super.dispose();
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
