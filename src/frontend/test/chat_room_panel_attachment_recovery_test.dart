import 'dart:io';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_proto_json.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/gen/voice/file/v1/file.pb.dart' as file_pb;
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

enum _UploadFailureAt { request, put, confirm }

void main() {
  setUpAll(_loadCaptureFonts);

  for (final failureAt in _UploadFailureAt.values) {
    testWidgets(
      'manual retry keeps the selected attachment after ${failureAt.name} failure',
      (tester) async {
        tester.view.physicalSize = const Size(1280, 800);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final script = _UploadScript(failureAt);
        final sends = <_SentMessage>[];
        final container = ProviderContainer(
          overrides: [
            ...voiceAppTestOverrides(client: MockClient(script.handle)),
            chatRoomControllerProvider('chat-abc').overrideWith(
              (ref) => _RecordingRoomController(ref, 'chat-abc', sends),
            ),
            chatListProvider.overrideWith(
              (ref) async => const ChatListData(items: []),
            ),
          ],
        );
        addTearDown(container.dispose);
        var pickerCalls = 0;

        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              theme: _captureTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: RepaintBoundary(
                key: _captureBoundaryKey,
                child: Scaffold(
                  body: ChatRoomPanel(
                    chatId: 'chat-abc',
                    attachmentPicker: () async {
                      pickerCalls++;
                      return ChatAttachmentFile(
                        bytes: Uint8List.fromList([1, 2, 3, 4]),
                        contentType: 'application/pdf',
                        name: 'report.pdf',
                      );
                    },
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(ChatRoomPanel.inputKey),
          'draft text',
        );
        await tester.tap(find.byKey(ChatRoomPanel.attachKey));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Document'));
        await tester.pumpAndSettle();

        expect(find.text('Could not upload file. Try again.'), findsOneWidget);
        expect(find.textContaining('private diagnostic'), findsNothing);
        expect(find.textContaining('upload.test'), findsNothing);
        expect(_draftText(tester), 'draft text');
        expect(pickerCalls, 1);
        expect(sends, isEmpty);

        if (failureAt == _UploadFailureAt.request) {
          await _writeCaptureIfRequested(tester, 'attachment-upload-failed-h');
        }

        await tester.tap(find.text('Try again'));
        await tester.pumpAndSettle();

        expect(pickerCalls, 1, reason: 'retry keeps the original selection');
        expect(
          script.requestCalls,
          2,
          reason: 'retry obtains a fresh upload ticket',
        );
        expect(script.putCalls, failureAt == _UploadFailureAt.request ? 1 : 2);
        expect(
          script.confirmCalls,
          failureAt == _UploadFailureAt.confirm ? 2 : 1,
        );
        expect(script.putBodies, everyElement([1, 2, 3, 4]));
        expect(sends, hasLength(1));
        expect(sends.single.content, 'draft text');
        expect(sends.single.attachments, hasLength(1));
        expect(sends.single.attachments.single.fileId, 'file-2');
        expect(_draftText(tester), isEmpty);
        expect(find.textContaining('private diagnostic'), findsNothing);
        expect(find.textContaining('upload.test'), findsNothing);
      },
    );
  }

  testWidgets('narrow attach flow retries in place and can be cancelled', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final script = _UploadScript(_UploadFailureAt.put);
    final sends = <_SentMessage>[];
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient(script.handle)),
        chatRoomControllerProvider('chat-abc').overrideWith(
          (ref) => _RecordingRoomController(ref, 'chat-abc', sends),
        ),
        chatListProvider.overrideWith(
          (ref) async => const ChatListData(items: []),
        ),
      ],
    );
    addTearDown(container.dispose);
    var pickerCalls = 0;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: _captureTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: RepaintBoundary(
            key: _captureBoundaryKey,
            child: Scaffold(
              body: ChatRoomPanel(
                chatId: 'chat-abc',
                attachmentPicker: () async {
                  pickerCalls++;
                  return ChatAttachmentFile(
                    bytes: Uint8List.fromList([1, 2, 3, 4]),
                    contentType: 'application/pdf',
                    name: 'report.pdf',
                  );
                },
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(ChatRoomPanel.inputKey), 'keep draft');
    await tester.tap(find.byKey(ChatRoomPanel.attachKey));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Document'));
    await tester.pumpAndSettle();

    expect(find.text('Could not upload file. Try again.'), findsOneWidget);
    await _writeCaptureIfRequested(tester, 'attachment-upload-failed-v');
    await tester.tap(find.byKey(ChatRoomPanel.attachmentUploadCancelKey));
    await tester.pumpAndSettle();

    expect(find.text('Could not upload file. Try again.'), findsNothing);
    expect(find.text('Try again'), findsNothing);
    expect(_draftText(tester), 'keep draft');
    expect(pickerCalls, 1);
    expect(script.requestCalls, 1);
    expect(script.putCalls, 1);
    expect(script.confirmCalls, 0);
    expect(sends, isEmpty);
  });
}

const _captureBoundaryKey = Key('attachment_recovery_capture');

ThemeData _captureTheme() => voiceTestTheme().copyWith(
  textTheme: voiceTestTheme().textTheme.apply(fontFamily: 'Noto Sans'),
);

Future<void> _loadCaptureFonts() async {
  final materialIcons = FontLoader('MaterialIcons')
    ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
  await materialIcons.load();
  final notoSans = FontLoader('Noto Sans')
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
  await notoSans.load();
}

String _draftText(WidgetTester tester) =>
    tester.widget<TextField>(find.byType(TextField).first).controller!.text;

Future<void> _writeCaptureIfRequested(
  WidgetTester tester,
  String filename,
) async {
  final path = Platform.environment['VOICE_UI_CAPTURE_DIR'];
  if (path == null || path.isEmpty) return;
  // Use the repository's standard Flutter golden rasterization path; in
  // capture mode `--update-goldens` materializes the actual host image, which
  // is copied to the run artifact directory after this test completes.
  await expectLater(
    find.byKey(_captureBoundaryKey),
    matchesGoldenFile('attachment_recovery_capture/$filename.png'),
  );
  final golden = File('test/attachment_recovery_capture/$filename.png');
  if (!golden.existsSync()) {
    throw StateError('Capture golden was not materialized: ${golden.path}');
  }
  final directory = Directory(path)..createSync(recursive: true);
  await golden.copy('${directory.path}${Platform.pathSeparator}$filename.png');
}

final class _UploadScript {
  _UploadScript(this.failAt);

  final _UploadFailureAt failAt;
  int requestCalls = 0;
  int putCalls = 0;
  int confirmCalls = 0;
  final List<List<int>> putBodies = [];

  Future<http.Response> handle(http.Request request) async {
    if (request.url.path == '/api/v1/files/upload') {
      requestCalls++;
      if (failAt == _UploadFailureAt.request && requestCalls == 1) {
        return _failure();
      }
      final fileId = 'file-$requestCalls';
      return _protoResponse(
        file_pb.RequestUploadResponse(
          uploadResponse: file_pb.UploadResponse(
            fileId: fileId,
            presignedPutUrl: 'https://upload.test/$fileId',
            r2Key: 'private-object-key',
          ),
        ),
      );
    }
    if (request.method == 'PUT' && request.url.host == 'upload.test') {
      putCalls++;
      putBodies.add(request.bodyBytes);
      if (failAt == _UploadFailureAt.put && putCalls == 1) return _failure();
      return http.Response('', 200);
    }
    if (request.url.path.startsWith('/api/v1/files/') &&
        request.url.path.endsWith('/confirm')) {
      confirmCalls++;
      if (failAt == _UploadFailureAt.confirm && confirmCalls == 1) {
        return _failure();
      }
      final fileId = request.url.path.split('/')[4];
      return _protoResponse(
        file_pb.ConfirmUploadResponse(
          fileMetadata: file_pb.FileMetadata(
            id: fileId,
            fileType: 'document',
            status: 'ready',
            originalName: 'report.pdf',
            sizeBytes: Int64(4),
          ),
        ),
      );
    }
    return http.Response('{}', 404);
  }

  http.Response _failure() => http.Response(
    '{"error_code":"upload_failed","message":"private diagnostic"}',
    503,
    headers: const {'content-type': 'application/json'},
  );

  http.Response _protoResponse(Object message) => http.Response(
    encodeGatewayProto(message as dynamic),
    200,
    headers: const {'content-type': 'application/json'},
  );
}

final class _SentMessage {
  const _SentMessage(this.content, this.attachments);

  final String content;
  final List<MessageAttachment> attachments;
}

final class _RecordingRoomController extends ChatRoomController {
  _RecordingRoomController(super.ref, super.chatId, this.sends) : super();

  final List<_SentMessage> sends;

  @override
  Future<void> loadInitial() async {}

  @override
  Future<String?> sendMessage(
    String content, {
    List<MessageAttachment> attachments = const [],
    List<MessageMention> mentions = const [],
    String? threadParentId,
  }) async {
    sends.add(_SentMessage(content, List.of(attachments)));
    return null;
  }
}
