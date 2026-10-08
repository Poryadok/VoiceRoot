import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart'
    show FontLoader, LogicalKeyboardKey, rootBundle;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/group_members_management_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/chat/pinned_messages_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('authorized full-list unpin removes the selected row', (
    tester,
  ) async {
    var unpinCalls = 0;
    await tester.pumpWidget(
      _testApp(
        permission: (_) async => true,
        child: PinnedMessagesPanel(
          chatId: 'chat-one',
          spaceId: 'space-one',
          messages: [_message()],
          onOpenMessage: _openMessage,
          onUnpin: (id) async {
            expect(id, 'pin-one');
            unpinCalls++;
            return const PinMutationResult.success();
          },
        ),
      ),
    );
    await tester.pumpAndSettle();

    final unpin = find.byTooltip('Unpin message');
    expect(unpin, findsOneWidget);
    await tester.tap(unpin);
    await tester.pumpAndSettle();

    expect(unpinCalls, 1);
    expect(
      find.byKey(const ValueKey('chat_pinned_message_pin-one')),
      findsNothing,
    );
  });

  testWidgets(
    'unpin action exposes its accessible label and activates by key',
    (tester) async {
      var unpinCalls = 0;
      final semantics = tester.ensureSemantics();
      await tester.pumpWidget(
        _testApp(
          permission: (_) async => true,
          child: PinnedMessagesPanel(
            chatId: 'chat-one',
            spaceId: 'space-one',
            messages: [_message()],
            onOpenMessage: _openMessage,
            onUnpin: (_) async {
              unpinCalls++;
              return const PinMutationResult.success();
            },
          ),
        ),
      );
      await tester.pumpAndSettle();

      final unpin = find.byTooltip('Unpin message');
      expect(unpin, findsOneWidget);
      expect(tester.getSemantics(unpin).tooltip, 'Unpin message');
      for (var index = 0; index < 6; index++) {
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      }
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();

      expect(unpinCalls, 1);
      expect(
        find.byKey(const ValueKey('chat_pinned_message_pin-one')),
        findsNothing,
      );
      semantics.dispose();
    },
  );

  testWidgets('permission loading and denial never expose an unpin action', (
    tester,
  ) async {
    final pending = Completer<bool>();
    await tester.pumpWidget(
      _testApp(
        permission: (_) => pending.future,
        child: PinnedMessagesPanel(
          chatId: 'chat-one',
          spaceId: 'space-one',
          messages: [_message()],
          onOpenMessage: _openMessage,
          onUnpin: (_) async => const PinMutationResult.success(),
        ),
      ),
    );
    await tester.pump();
    expect(
      find.byKey(const Key('chat_pinned_permission_loading')),
      findsOneWidget,
    );
    expect(find.byTooltip('Unpin message'), findsNothing);

    pending.complete(false);
    await tester.pumpAndSettle();
    expect(find.byTooltip('Unpin message'), findsNothing);
  });

  testWidgets(
    'permission lookup error can be retried without granting access',
    (tester) async {
      var calls = 0;
      await tester.pumpWidget(
        _testApp(
          permission: (_) async {
            if (calls++ == 0) throw StateError('private transport detail');
            return false;
          },
          child: PinnedMessagesPanel(
            chatId: 'chat-one',
            spaceId: 'space-one',
            messages: [_message()],
            onOpenMessage: _openMessage,
            onUnpin: (_) async => const PinMutationResult.success(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(
        find.byKey(const Key('chat_pinned_permission_error')),
        findsOneWidget,
      );
      expect(find.text('Could not complete this action.'), findsOneWidget);
      expect(find.text('private transport detail'), findsNothing);
      await tester.tap(find.byTooltip('Try again'));
      await tester.pumpAndSettle();

      expect(calls, 2);
      expect(find.byTooltip('Unpin message'), findsNothing);
    },
  );

  testWidgets(
    'standalone-group unpin requires the exact owner or admin actor',
    (tester) async {
      await tester.pumpWidget(
        _testApp(
          permission: (_) async => false,
          overrides: [
            groupMembersManagementProvider('chat-one').overrideWith(
              (ref) => _TestGroupMembersController(ref, 'chat-one', const [
                ChatMember(profileId: 'prof-test', role: 'admin'),
                ChatMember(profileId: 'profile-other', role: 'owner'),
              ]),
            ),
          ],
          child: PinnedMessagesPanel(
            chatId: 'chat-one',
            isGroup: true,
            messages: [_message()],
            onOpenMessage: _openMessage,
            onUnpin: (_) async => const PinMutationResult.success(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byTooltip('Unpin message'), findsOneWidget);
    },
  );

  testWidgets('standalone-group role for a different actor grants no unpin', (
    tester,
  ) async {
    await tester.pumpWidget(
      _testApp(
        permission: (_) async => false,
        overrides: [
          groupMembersManagementProvider('chat-two').overrideWith(
            (ref) => _TestGroupMembersController(ref, 'chat-two', const [
              ChatMember(profileId: 'profile-other', role: 'owner'),
            ]),
          ),
        ],
        child: PinnedMessagesPanel(
          chatId: 'chat-two',
          isGroup: true,
          messages: [_message()],
          onOpenMessage: _openMessage,
          onUnpin: (_) async => const PinMutationResult.success(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byTooltip('Unpin message'), findsNothing);
  });

  testWidgets('server permission denial keeps the pin and hides diagnostics', (
    tester,
  ) async {
    var permissionCalls = 0;
    await tester.pumpWidget(
      _testApp(
        permission: (_) async => permissionCalls++ == 0,
        child: PinnedMessagesPanel(
          chatId: 'chat-one',
          spaceId: 'space-one',
          messages: [_message()],
          onOpenMessage: _openMessage,
          onUnpin: (_) async => const PinMutationResult.failure(
            message: 'upstream permission details',
            errorCode: 'permission_denied',
            statusCode: 403,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Unpin message'));
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('chat_pinned_message_pin-one')),
      findsOneWidget,
    );
    expect(
      find.text('Missing permission: TEXT_CHAT_PIN_MESSAGES'),
      findsOneWidget,
    );
    expect(find.text('upstream permission details'), findsNothing);
    expect(permissionCalls, 2);
    expect(find.text('Try again'), findsNothing);
  });

  testWidgets('stale unpin completion clears busy state without changing pin', (
    tester,
  ) async {
    final completion = Completer<PinMutationResult>();
    await tester.pumpWidget(
      _testApp(
        permission: (_) async => true,
        child: PinnedMessagesPanel(
          chatId: 'chat-one',
          spaceId: 'space-one',
          messages: [_message()],
          onOpenMessage: _openMessage,
          onUnpin: (_) => completion.future,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Unpin message'));
    await tester.pump();
    expect(find.byKey(const Key('chat_pinned_unpin_busy')), findsOneWidget);

    completion.complete(const PinMutationResult.stale());
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('chat_pinned_unpin_busy')), findsNothing);
    expect(
      find.byKey(const ValueKey('chat_pinned_message_pin-one')),
      findsOneWidget,
    );
    expect(find.text('upstream'), findsNothing);
  });

  test('ResourceExhausted uses the approved maximum-five message', () {
    final l10n = lookupAppLocalizations(const Locale('en'));
    const result = PinMutationResult.failure(
      message: 'pin limit reached',
      errorCode: 'resource_exhausted',
      statusCode: 429,
    );

    expect(pinMutationErrorText(l10n, result), 'You can pin up to 5 messages.');
  });

  for (final capture in [
    (name: 'H', size: const Size(1280, 800)),
    (name: 'V', size: const Size(390, 844)),
  ]) {
    testWidgets('production-font pin management capture ${capture.name}', (
      tester,
    ) async {
      final captureDir = Platform.environment['VOICE_PIN_MANAGER_CAPTURE_DIR'];
      if (captureDir == null || captureDir.isEmpty) return;
      await _loadCaptureFonts();
      tester.view.physicalSize = capture.size;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(
        _testApp(
          captureBoundary: true,
          permission: (_) async => true,
          child: Builder(
            builder: (context) => Center(
              child: FilledButton(
                onPressed: () => PinnedMessagesPanel.show(
                  context,
                  chatId: 'chat-one',
                  spaceId: 'space-one',
                  messages: [_message()],
                  onOpenMessage: _openMessage,
                  onUnpin: (_) async => const PinMutationResult.success(),
                ),
                child: const Text('Open pinned messages'),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('Open pinned messages'));
      await tester.pumpAndSettle();
      expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
      expect(find.text('Pinned text'), findsOneWidget);
      expect(find.byTooltip('Unpin message'), findsOneWidget);
      await _writeCapture(
        tester,
        captureDir,
        'pin_management_${capture.name.toLowerCase()}',
      );
    });
  }
}

Future<bool> _openMessage(String _) async => true;

VoiceMessage _message() => const VoiceMessage(
  id: 'pin-one',
  chatId: 'chat-one',
  senderProfileId: 'profile-one',
  content: 'Pinned text',
  isPinned: true,
);

Widget _testApp({
  required Future<bool> Function(SpacePermissionQuery query) permission,
  required Widget child,
  List<Override> overrides = const [],
  bool captureBoundary = false,
}) {
  final app = ProviderScope(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
      ...overrides,
      spacePermissionProvider.overrideWith((ref, query) => permission(query)),
    ],
    child: MaterialApp(
      theme: voiceTestTheme().copyWith(
        textTheme: voiceTestTheme().textTheme.apply(fontFamily: 'Noto Sans'),
      ),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(body: child),
    ),
  );
  return captureBoundary
      ? RepaintBoundary(key: _captureBoundaryKey, child: app)
      : app;
}

const _captureBoundaryKey = Key('pin_management_capture');

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

Future<void> _writeCapture(
  WidgetTester tester,
  String captureDir,
  String fileName,
) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('Flutter did not encode the capture');
      final directory = Directory(captureDir)..createSync(recursive: true);
      await File(
        '${directory.path}${Platform.pathSeparator}$fileName.png',
      ).writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (captured != true) throw StateError('Flutter capture did not complete');
}

class _TestGroupMembersController extends GroupMembersManagementController {
  // Keep the explicit positional delegation required by the base constructor.
  // ignore: use_super_parameters
  _TestGroupMembersController(Ref ref, String chatId, List<ChatMember> members)
    : super(ref, chatId) {
    state = GroupMembersManagementState(members: members);
  }
}
