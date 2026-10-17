import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_proto_json.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/gen/voice/common/v1/common.pb.dart' as common_pb;
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as messaging_pb;
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/create_group_friends_provider.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/composer_panels.dart';
import 'package:voice_frontend/ui/chat/create_group_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _rootKey = Key('attach_group_capture_root');
const _chatId = 'attach-reference';

// Exercise the real Gateway parser and controller with a complete terminal page.
// The controller rejects a response without page presence even for an empty list.
String _emptyScheduledResponse() => encodeGatewayProto(
  messaging_pb.ListScheduledMessagesResponse(
    page: common_pb.CursorPageResponse(hasMore: false, nextCursor: ''),
  ),
);

Future<ThemeData> _captureTheme(WidgetTester tester, bool capturing) async {
  if (!capturing) return voiceTestTheme();
  final theme = await tester.runAsync(() async {
    await (FontLoader(VoiceTheme.fontFamily)
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf')))
        .load();
    final flutterRoot = Platform.environment['FLUTTER_ROOT'];
    if (flutterRoot == null || flutterRoot.isEmpty) {
      throw StateError('FLUTTER_ROOT is required for capture icon fonts');
    }
    final bytes = await File(
      '$flutterRoot/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
    ).readAsBytes();
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future.value(ByteData.sublistView(bytes)))).load();
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) throw StateError('Capture theme loading did not finish');
  return theme;
}

Future<void> _capture(
  WidgetTester tester,
  String? directory,
  String name,
) async {
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_rootKey),
  );
  final written = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('Capture PNG encoding failed');
      final file = File('$directory/$name.png');
      await file.parent.create(recursive: true);
      await file.writeAsBytes(data.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  expect(written, isTrue, reason: 'Actual PNG capture must finish');
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));
  test('capture scheduled response preserves the required terminal page', () {
    final decoded = decodeGatewayProto(
      messaging_pb.ListScheduledMessagesResponse.create,
      _emptyScheduledResponse(),
    );
    expect(decoded.hasPage(), isTrue);
    expect(decoded.page.hasMore, isFalse);
    expect(decoded.page.nextCursor, isEmpty);
    expect(decoded.scheduledMessages, isEmpty);
  });
  for (final wide in [true, false]) {
    testWidgets(
      'AttachMenu and CreateGroup reference states ${wide ? 'H' : 'V'}',
      (tester) async {
        tester.view.physicalSize = wide
            ? const Size(1280, 800)
            : const Size(390, 844);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final directory =
            Platform.environment['VOICE_ATTACH_GROUP_CAPTURE_DIR'];
        final theme = await _captureTheme(
          tester,
          directory != null && directory.isNotEmpty,
        );
        final scheduledRequests = <http.Request>[];
        final unexpectedRequests = <String>[];
        final client = MockClient((request) async {
          if (request.method == 'GET' &&
              request.url.path == '/api/v1/messages/scheduled') {
            scheduledRequests.add(request);
            expect(request.url.queryParameters['chat_id'], _chatId);
            expect(request.headers['authorization'], 'Bearer test-access');
            return http.Response(_emptyScheduledResponse(), 200);
          }
          unexpectedRequests.add('${request.method} ${request.url}');
          return http.Response('{}', 404);
        });
        _CaptureRealtime? hub;
        final container = ProviderContainer(
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
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            httpClientProvider.overrideWithValue(client),
            realtimeHubProvider.overrideWith(
              (ref) => hub = _CaptureRealtime(ref),
            ),
            realtimeAutoConnectProvider.overrideWithValue(false),
            realtimeLinkStatusProvider.overrideWith(
              (ref) => RealtimeLinkStatus.connected,
            ),
            isDeviceOfflineProvider.overrideWith((ref) => false),
            groupActiveCallProvider(_chatId).overrideWith((ref) async => null),
            subscriptionProvider.overrideWith((ref) async => null),
            chatListControllerProvider.overrideWith(_CaptureChatList.new),
            chatRoomControllerProvider(
              _chatId,
            ).overrideWith((ref) => _CaptureRoom(ref, _chatId)),
            createGroupFriendsProvider.overrideWith(
              (ref) async => ['alex', 'maria', 'chris'],
            ),
            profileProvider.overrideWith(
              (ref, id) async => VoiceProfile(
                id: id,
                accountId: 'account-$id',
                username: id,
                discriminator: '0001',
                displayName: switch (id) {
                  'alex' => 'Alex',
                  'maria' => 'Maria',
                  _ => 'Chris',
                },
              ),
            ),
          ],
        );
        addTearDown(() async {
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpAndSettle();
          container.dispose();
          await hub?.dispose();
          client.close();
        });
        container.read(selectedChatIdProvider.notifier).state = _chatId;
        Widget app(Widget home) => RepaintBoundary(
          key: _rootKey,
          child: UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              debugShowCheckedModeBanner: false,
              theme: theme,
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: home,
            ),
          ),
        );
        await tester.pumpWidget(
          app(const Scaffold(body: ChatRoomPanel(chatId: _chatId))),
        );
        await tester.pumpAndSettle();
        final room = container.read(chatRoomControllerProvider(_chatId));
        expect(scheduledRequests, isEmpty);
        expect(unexpectedRequests, isEmpty);
        expect(room.isLoadingScheduledMessages, isFalse);
        expect(room.scheduledMessagesError, isNull);
        expect(
          find.byKey(ChatRoomPanel.scheduledMessagesKey),
          findsNothing,
          reason:
              'The normal reference room has no scheduled-message error banner.',
        );
        final trigger = tester.getRect(find.byKey(ChatRoomPanel.attachKey));
        await tester.tap(find.byKey(ChatRoomPanel.attachKey));
        await tester.pumpAndSettle();
        expect(find.byKey(ComposerAttachMenuBody.menuKey), findsOneWidget);
        if (wide) {
          final popup = tester.getRect(
            find.byKey(ComposerAttachMenuBody.menuKey),
          );
          expect(popup.width, 260);
          expect(popup.bottom, closeTo(trigger.top - 8, .01));
          expect(popup.left, lessThanOrEqualTo(trigger.left));
          expect(
            popup.right,
            1272,
            reason:
                'The actual room trigger is near the right viewport edge; the popup must clamp inside its 8 px margin.',
          );
        } else {
          expect(find.byType(BottomSheet), findsOneWidget);
        }
        expect(tester.takeException(), isNull);
        await _capture(tester, directory, 'attach-menu-${wide ? 'h' : 'v'}');
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(find.byKey(ComposerAttachMenuBody.menuKey), findsNothing);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
        await tester.pumpWidget(
          app(
            Scaffold(
              body: Builder(
                builder: (context) => TextButton(
                  onPressed: () => CreateGroupSheet.show(context),
                  child: const Text('Create group'),
                ),
              ),
            ),
          ),
        );
        await tester.tap(find.text('Create group'));
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(CreateGroupSheet.nameFieldKey),
          'Design Review',
        );
        await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('alex')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('maria')));
        await tester.pumpAndSettle();
        expect(
          find.byKey(const Key('create_group_selected_alex')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('create_group_selected_maria')),
          findsOneWidget,
        );
        expect(
          tester
              .widget<FilledButton>(find.byKey(CreateGroupSheet.submitKey))
              .onPressed,
          isNotNull,
        );
        expect(tester.takeException(), isNull);
        await _capture(tester, directory, 'create-group-${wide ? 'h' : 'v'}');
        expect(unexpectedRequests, isEmpty);
        await tester.tap(find.byKey(CreateGroupSheet.closeKey));
        await tester.pumpAndSettle();
        expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      },
    );
  }
}

class _CaptureRealtime extends RealtimeHub {
  _CaptureRealtime(super.ref);
  @override
  Future<void> ensureConnected() async {}
  @override
  void ensureSubscribed(String chatId) {}
}

class _CaptureChatList extends ChatListController {
  _CaptureChatList(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: _chatId,
            type: 'CHAT_TYPE_GROUP',
            name: 'Design Review',
            creatorProfileId: 'prof-test',
          ),
        ),
      ],
    );
  }
  @override
  Future<void> loadInitial() async {}
}

class _CaptureRoom extends ChatRoomController {
  _CaptureRoom(super.ref, super.chatId) : super() {
    state = ChatRoomState(
      historyProfileId: 'prof-test',
      realtimeStatus: RealtimeLinkStatus.connected,
      messages: [
        VoiceMessage(
          id: 'reference-message',
          chatId: _chatId,
          senderProfileId: 'alex',
          content: 'Send the materials after the meeting',
          createdAt: DateTime.utc(2026, 10, 14, 12),
        ),
      ],
    );
  }
  @override
  Future<void> loadInitial() async {}
}
