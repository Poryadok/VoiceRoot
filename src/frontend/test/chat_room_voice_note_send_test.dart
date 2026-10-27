import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_proto_json.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/gen/voice/file/v1/file.pb.dart' as file_pb;
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/voice_note_capture.dart';
import 'package:voice_frontend/ui/core/voice_send_button.dart';

import 'support/auth_test_overrides.dart';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  for (final failure in [null, 'request', 'put', 'confirm']) {
    testWidgets('recorded voice sends confirmed file after $failure recovery', (
      tester,
    ) async {
      final recorder = _Recorder();
      final script = _UploadScript(failure);
      final sends = <MessageAttachment>[];
      await _mount(tester, recorder, script, sends);
      final gesture = await tester.startGesture(
        tester.getCenter(find.byKey(const Key('voice_note_mic'))),
      );
      await tester.pump();
      await gesture.up();
      await tester.pumpAndSettle();
      expect(
        recorder.discards,
        1,
        reason: 'local file ownership ends after bytes are retained',
      );
      if (failure != null) {
        expect(sends, isEmpty);
        expect(
          find.byKey(ChatRoomPanel.attachmentUploadFailureKey),
          findsOneWidget,
        );
        await tester.tap(find.text('Try again'));
        await tester.pumpAndSettle();
        expect(script.uploads, hasLength(2));
      }
      expect(sends, hasLength(1));
      expect(sends.single.fileId, 'confirmed-file');
      expect(sends.single.type, 'voice_message');
      expect(sends.single.durationSeconds, 3);
      for (final request in script.uploads) {
        expect(request.intent, file_pb.UploadIntent.UPLOAD_INTENT_AUDIO);
        expect(request.mimeType, 'audio/wav');
        expect(request.originalName, 'voice.wav');
        expect(request.sizeBytes.toInt(), _voiceBytes.length);
        expect(request.contextChat.id, 'chat-abc');
      }
      expect(script.putBodies, everyElement(_voiceBytes));
    });
  }

  testWidgets('text in flight retains confirmed voice without replaying text', (
    tester,
  ) async {
    final recorder = _Recorder();
    final script = _UploadScript(null)
      ..ticketGate = Completer<void>()
      ..textGate = Completer<void>();
    final sends = <MessageAttachment>[];
    final container = await _mount(tester, recorder, script, sends);
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('voice_note_mic'))),
    );
    await tester.pump();
    await gesture.up();
    await tester.pump(const Duration(milliseconds: 100));
    final textSend = container
        .read(chatRoomControllerProvider('chat-abc').notifier)
        .sendMessage('other text');
    await tester.pump();
    script.ticketGate!.complete();
    await tester.pump(const Duration(milliseconds: 100));
    expect(sends, isEmpty);
    expect(find.byKey(ChatRoomPanel.voiceSendRetryKey), findsOneWidget);
    script.textGate!.complete();
    await textSend;
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatRoomPanel.voiceSendRetryKey));
    await tester.pumpAndSettle();
    expect(sends, hasLength(1));
    expect(script.uploads, hasLength(1));
    expect(
      script.messageBodies.where((body) => body['content'] == 'other text'),
      hasLength(1),
    );
    expect(
      script.messageBodies.where(
        (body) => body['content_type'] == 'MESSAGE_CONTENT_TYPE_VOICE',
      ),
      hasLength(1),
    );
  });

  testWidgets('offline flip retains confirmed voice for retry without upload', (
    tester,
  ) async {
    final recorder = _Recorder();
    final script = _UploadScript(null)..ticketGate = Completer<void>();
    final sends = <MessageAttachment>[];
    final container = await _mount(tester, recorder, script, sends);
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('voice_note_mic'))),
    );
    await tester.pump();
    await gesture.up();
    await tester.pump(const Duration(milliseconds: 100));
    container.read(_offlineProvider.notifier).state = true;
    script.ticketGate!.complete();
    await tester.pumpAndSettle();
    expect(sends, isEmpty);
    expect(find.byKey(ChatRoomPanel.voiceSendRetryKey), findsOneWidget);
    container.read(_offlineProvider.notifier).state = false;
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatRoomPanel.voiceSendRetryKey));
    await tester.pumpAndSettle();
    expect(sends, hasLength(1));
    expect(script.uploads, hasLength(1));
    expect(sends.single.fileId, 'confirmed-file');
  });

  testWidgets('chat navigation invalidates an in-flight voice upload', (
    tester,
  ) async {
    final recorder = _Recorder();
    final script = _UploadScript(null)..ticketGate = Completer<void>();
    final sends = <MessageAttachment>[];
    final container = await _mount(tester, recorder, script, sends);
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('voice_note_mic'))),
    );
    await tester.pump();
    await gesture.up();
    await tester.pump(const Duration(milliseconds: 100));
    expect(script.uploads, hasLength(1));
    container.read(selectedChatIdProvider.notifier).state = 'another-chat';
    await tester.pump();
    script.ticketGate!.complete();
    await tester.pumpAndSettle();
    expect(script.putBodies, isEmpty);
    expect(sends, isEmpty);
    expect(recorder.discards, 1);
  });

  testWidgets('voice upload leaves a newly typed text draft intact', (
    tester,
  ) async {
    final recorder = _Recorder();
    final script = _UploadScript(null)..ticketGate = Completer<void>();
    final sends = <MessageAttachment>[];
    await _mount(tester, recorder, script, sends);
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('voice_note_mic'))),
    );
    await tester.pump();
    await gesture.up();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.enterText(find.byKey(ChatRoomPanel.inputKey), 'next draft');
    expect(
      tester
          .widget<VoiceSendButton>(find.byKey(ChatRoomPanel.sendKey))
          .onPressed,
      isNull,
    );
    script.ticketGate!.complete();
    await tester.pumpAndSettle();
    expect(sends, hasLength(1));
    expect(
      tester.widget<TextField>(find.byType(TextField).first).controller!.text,
      'next draft',
    );
  });

  testWidgets('cancelled capture never uploads or sends', (tester) async {
    final recorder = _Recorder();
    final script = _UploadScript(null);
    final sends = <MessageAttachment>[];
    await _mount(tester, recorder, script, sends);
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('voice_note_mic'))),
    );
    await tester.pump();
    await gesture.moveBy(const Offset(-90, 0));
    await gesture.up();
    await tester.pumpAndSettle();
    expect(recorder.cancels, greaterThan(0));
    expect(script.uploads, isEmpty);
    expect(sends, isEmpty);
  });

  testWidgets('failed voice upload can be discarded without sending', (
    tester,
  ) async {
    final recorder = _Recorder();
    final script = _UploadScript('put');
    final sends = <MessageAttachment>[];
    await _mount(tester, recorder, script, sends);
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('voice_note_mic'))),
    );
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatRoomPanel.attachmentUploadCancelKey));
    await tester.pumpAndSettle();
    expect(find.byKey(ChatRoomPanel.attachmentUploadFailureKey), findsNothing);
    expect(script.uploads, hasLength(1));
    expect(sends, isEmpty);
  });

  testWidgets(
    'send retry reuses voice payload and UUID without uploading again',
    (tester) async {
      final recorder = _Recorder();
      final script = _UploadScript('send');
      final sends = <MessageAttachment>[];
      await _mount(tester, recorder, script, sends);
      final gesture = await tester.startGesture(
        tester.getCenter(find.byKey(const Key('voice_note_mic'))),
      );
      await tester.pump();
      await gesture.up();
      await tester.pumpAndSettle();
      expect(sends, isEmpty);
      expect(find.byKey(ChatRoomPanel.voiceSendRetryKey), findsOneWidget);
      await tester.tap(find.byKey(ChatRoomPanel.voiceSendRetryKey));
      await tester.pumpAndSettle();
      expect(sends, hasLength(1));
      expect(script.uploads, hasLength(1));
      expect(script.messageBodies, hasLength(2));
      expect(script.messageBodies[0], script.messageBodies[1]);
    },
  );
}

final _voiceBytes = _waveBytes();
final _offlineProvider = StateProvider<bool>((ref) => false);

Uint8List _waveBytes() {
  const dataLength = 16000 * 2 * 7 ~/ 2;
  final bytes = Uint8List(44 + dataLength);
  final header = ByteData.sublistView(bytes);
  bytes.setRange(0, 4, ascii.encode('RIFF'));
  header.setUint32(4, bytes.length - 8, Endian.little);
  bytes.setRange(8, 16, ascii.encode('WAVEfmt '));
  header.setUint32(16, 16, Endian.little);
  header.setUint16(20, 1, Endian.little);
  header.setUint16(22, 1, Endian.little);
  header.setUint32(24, 16000, Endian.little);
  header.setUint32(28, 32000, Endian.little);
  header.setUint16(32, 2, Endian.little);
  header.setUint16(34, 16, Endian.little);
  bytes.setRange(36, 40, ascii.encode('data'));
  header.setUint32(40, dataLength, Endian.little);
  return bytes;
}

Future<ProviderContainer> _mount(
  WidgetTester tester,
  _Recorder recorder,
  _UploadScript script,
  List<MessageAttachment> sends,
) async {
  tester.view.physicalSize = const Size(1280, 800);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(client: MockClient(script.handle)),
      isDeviceOfflineProvider.overrideWith(
        (ref) => ref.watch(_offlineProvider),
      ),
      voiceMessagesClientProvider.overrideWith(
        (ref) =>
            VoiceMessagesClient(gateway: ref.watch(gatewayHttpClientProvider)),
      ),
      chatRoomControllerProvider(
        'chat-abc',
      ).overrideWith((ref) => _Room(ref, 'chat-abc')),
      chatListProvider.overrideWith(
        (ref) async => const ChatListData(items: []),
      ),
    ],
  );
  script.sends = sends;
  addTearDown(container.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: ChatRoomPanel(
            chatId: 'chat-abc',
            voiceNoteRecorderFactory: () => recorder,
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return container;
}

class _Recorder implements VoiceNoteRecorder {
  int discards = 0, cancels = 0;
  @override
  Stream<double> get amplitudes => const Stream.empty();
  @override
  Future<bool> hasPermission() async => true;
  @override
  Future<void> start() async {}
  @override
  Future<CapturedVoiceNote> stop() async => CapturedVoiceNote(
    file: XFile.fromData(_voiceBytes, path: 'voice.wav', mimeType: 'audio/wav'),
    duration: const Duration(milliseconds: 3500),
    discard: () async {
      discards++;
    },
  );
  @override
  Future<void> cancel() async {
    cancels++;
  }

  @override
  Future<void> dispose() async {}
}

class _Room extends ChatRoomController {
  _Room(super.ref, super.chatId) : super();
  @override
  Future<void> loadInitial() async {}
}

class _UploadScript {
  _UploadScript(this.failure);
  final String? failure;
  bool failed = false;
  final uploads = <file_pb.RequestUploadRequest>[];
  final putBodies = <List<int>>[];
  List<MessageAttachment> sends = [];
  final messageBodies = <Map<String, dynamic>>[];
  Completer<void>? ticketGate;
  Completer<void>? textGate;
  Future<http.Response> handle(http.Request request) async {
    if (request.url.path == '/api/v1/messages/send') {
      final body = jsonDecode(request.body) as Map<String, dynamic>;
      messageBodies.add(body);
      if (body['content'] == 'other text') {
        await textGate?.future;
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'text-message',
              'chat': {'id': 'chat-abc'},
              'sender_profile_id': 'prof-test',
              'content': 'other text',
              'created_at': '2026-10-28T00:00:00Z',
            },
          }),
          200,
          headers: const {'content-type': 'application/json'},
        );
      }
      expect(body['content_type'], 'MESSAGE_CONTENT_TYPE_VOICE');
      expect(body['content'] ?? '', isEmpty);
      expect(body['client_message_id'], isNotEmpty);
      if (_fail('send')) return _failure();
      sends.addAll(
        (jsonDecode(body['attachments_json'] as String) as List).map(
          (value) => MessageAttachment.fromJson(value as Map<String, dynamic>),
        ),
      );
      return http.Response(
        jsonEncode({
          'message': {
            'id': 'message-1',
            'chat': {'id': 'chat-abc'},
            'sender_profile_id': 'prof-test',
            'content_type': 'MESSAGE_CONTENT_TYPE_VOICE',
            'attachments_json': body['attachments_json'],
            'created_at': '2026-10-28T00:00:00Z',
          },
        }),
        200,
        headers: const {'content-type': 'application/json'},
      );
    }
    if (request.url.path == '/api/v1/files/upload') {
      uploads.add(
        decodeGatewayProto(file_pb.RequestUploadRequest.create, request.body),
      );
      await ticketGate?.future;
      if (_fail('request')) return _failure();
      return _response(
        file_pb.RequestUploadResponse(
          uploadResponse: file_pb.UploadResponse(
            fileId: 'ticket-file',
            presignedPutUrl: 'https://upload.test/file',
          ),
        ),
      );
    }
    if (request.method == 'PUT') {
      putBodies.add(request.bodyBytes);
      expect(request.headers['content-type'], 'audio/wav');
      if (_fail('put')) return _failure();
      return http.Response('', 200);
    }
    if (request.url.path.endsWith('/confirm')) {
      final body = jsonDecode(request.body) as Map;
      expect(body['file_id'], 'ticket-file');
      if (_fail('confirm')) return _failure();
      return _response(
        file_pb.ConfirmUploadResponse(
          fileMetadata: file_pb.FileMetadata(
            id: 'confirmed-file',
            fileType: 'audio',
            status: 'ready',
            originalName: 'voice.wav',
            sizeBytes: Int64(_voiceBytes.length),
          ),
        ),
      );
    }
    return http.Response('{}', 404);
  }

  bool _fail(String phase) {
    if (failure != phase || failed) return false;
    return failed = true;
  }

  http.Response _failure() => http.Response(
    '{"error_code":"upload_failed"}',
    503,
    headers: const {'content-type': 'application/json'},
  );
  http.Response _response(dynamic message) => http.Response(
    encodeGatewayProto(message),
    200,
    headers: const {'content-type': 'application/json'},
  );
}
