import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/scheduled_messages_strip.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';

import 'support/voice_test_theme.dart';

ScheduledMessage _row(String id, {bool encrypted = false}) => ScheduledMessage(
  id: id,
  sendWhenOnline: true,
  status: ScheduledMessageStatus.SCHEDULED_MESSAGE_STATUS_PENDING,
  payload: ScheduledMessagePayload(content: 'Preview $id', isE2e: encrypted),
);

Widget _app(Widget child, {double scale = 1}) => MaterialApp(
  theme: voiceTestTheme(),
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  builder: (context, child) => MediaQuery(
    data: MediaQuery.of(context).copyWith(textScaler: TextScaler.linear(scale)),
    child: child!,
  ),
  home: Scaffold(
    body: Align(alignment: Alignment.bottomCenter, child: child),
  ),
);

void main() {
  testWidgets('empty hides strip; loading and list failure provide recovery', (
    tester,
  ) async {
    var retried = 0;
    Widget strip({bool loading = false, bool failed = false}) =>
        ScheduledMessagesStrip(
          messages: const [],
          loading: loading,
          loadFailed: failed,
          onEdit: (_) {},
          onCancel: (_) {},
          onSendNow: (_) {},
          onRetryLoad: () => retried++,
        );
    await tester.pumpWidget(_app(strip()));
    expect(find.byKey(ScheduledMessagesStrip.toggleKey), findsNothing);
    await tester.pumpWidget(_app(strip(loading: true)));
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
    await tester.pumpWidget(_app(strip(failed: true)));
    await tester.tap(find.byKey(const ValueKey('scheduled_retry_load')));
    expect(retried, 1);
  });

  testWidgets(
    'active edit prevents conflicting mutations while retaining recovery',
    (tester) async {
      await tester.pumpWidget(
        _app(
          ScheduledMessagesStrip(
            messages: [_row('one'), _row('two')],
            editingId: 'one',
            failedIds: {'one'},
            onEdit: (_) {},
            onCancel: (_) {},
            onSendNow: (_) {},
            onRetry: (_) {},
          ),
        ),
      );
      for (final action in ['edit', 'cancel', 'send']) {
        expect(
          tester
              .widget<TextButton>(
                find.byKey(ValueKey('scheduled_${action}_one')),
              )
              .onPressed,
          isNull,
        );
      }
      expect(
        tester
            .widget<TextButton>(
              find.byKey(const ValueKey('scheduled_retry_one')),
            )
            .onPressed,
        isNotNull,
      );
      expect(
        tester
            .widget<TextButton>(
              find.byKey(const ValueKey('scheduled_send_two')),
            )
            .onPressed,
        isNotNull,
      );
    },
  );

  final captureDir = Platform.environment['VOICE_SCHEDULED_CAPTURE_DIR'];
  if (captureDir != null) {
    for (final width in [900.0, 390.0]) {
      testWidgets('production-font H/V capture $width', (tester) async {
        tester.view.physicalSize = Size(width, width == 900 ? 560 : 844);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        late ThemeData theme;
        await tester.runAsync(() async {
          final icons = FontLoader('MaterialIcons')
            ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
          await icons.load();
          final noto = FontLoader('Noto Sans')
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'));
          await noto.load();
          final catalog = await VoiceTokenCatalog.load();
          theme = await VoiceTheme.build(
            catalog: catalog,
            mode: VoiceThemeMode.light,
            profileAccent: catalog.profileAccentAt(0),
          );
        });
        final rows = [
          _row('one')..payload.content = 'Напомнить про демо',
          _row('two')..payload.content = 'Ссылка на новый макет',
        ];
        for (var i = 0; i < rows.length; i++) {
          rows[i].scheduledAt = dateTimeToProtoTimestamp(
            DateTime.utc(2026, 9, 12 + i, 9, 30),
          );
        }
        const boundaryKey = Key('scheduled_capture');
        await tester.pumpWidget(
          RepaintBoundary(
            key: boundaryKey,
            child: MaterialApp(
              theme: theme,
              debugShowCheckedModeBanner: false,
              locale: const Locale('ru'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Scaffold(
                body: Column(
                  children: [
                    const ListTile(title: Text('Alex')),
                    const Expanded(child: SizedBox()),
                    ScheduledMessagesStrip(
                      messages: rows,
                      failedIds: width == 390 ? {'one'} : {},
                      onEdit: (_) {},
                      onCancel: (_) {},
                      onSendNow: (_) {},
                      onRetry: (_) {},
                      onDismissError: (_) {},
                    ),
                    const Padding(
                      padding: EdgeInsets.all(16),
                      child: TextField(
                        decoration: InputDecoration(hintText: 'Сообщение'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        final boundary = tester.renderObject<RenderRepaintBoundary>(
          find.byKey(boundaryKey),
        );
        await tester.runAsync(() async {
          final captured = await boundary.toImage(pixelRatio: 1);
          final png = await captured.toByteData(format: ui.ImageByteFormat.png);
          Directory(captureDir).createSync(recursive: true);
          await File(
            '$captureDir/scheduled-${width.toInt()}.png',
          ).writeAsBytes(png!.buffer.asUint8List());
          captured.dispose();
        });
      });
    }
  }
  testWidgets('collapse retains pending count and reopens pending rows only', (
    tester,
  ) async {
    final sent = _row('sent')
      ..status = ScheduledMessageStatus.SCHEDULED_MESSAGE_STATUS_SENT;
    await tester.pumpWidget(
      _app(
        ScheduledMessagesStrip(
          messages: [_row('one'), sent],
          onEdit: (_) {},
          onCancel: (_) {},
          onSendNow: (_) {},
        ),
      ),
    );
    expect(find.text('Scheduled · 1'), findsOneWidget);
    expect(find.text('Preview one'), findsOneWidget);
    expect(find.text('Preview sent'), findsNothing);
    await tester.tap(find.byKey(ScheduledMessagesStrip.toggleKey));
    await tester.pump();
    expect(find.text('Preview one'), findsNothing);
    expect(find.text('Scheduled · 1'), findsOneWidget);
    await tester.tap(find.byKey(ScheduledMessagesStrip.toggleKey));
    await tester.pump();
    expect(find.text('Preview one'), findsOneWidget);
  });

  testWidgets('row actions carry identity and busy rows cannot submit again', (
    tester,
  ) async {
    final actions = <String>[];
    Widget strip(Set<String> busy) => ScheduledMessagesStrip(
      messages: [_row('one')],
      busyIds: busy,
      onEdit: (row) => actions.add('edit:${row.id}'),
      onCancel: (row) => actions.add('cancel:${row.id}'),
      onSendNow: (row) => actions.add('send:${row.id}'),
    );
    await tester.pumpWidget(_app(strip({})));
    await tester.tap(find.byKey(const ValueKey('scheduled_edit_one')));
    await tester.tap(find.byKey(const ValueKey('scheduled_cancel_one')));
    await tester.tap(find.byKey(const ValueKey('scheduled_send_one')));
    expect(actions, ['edit:one', 'cancel:one', 'send:one']);
    await tester.pumpWidget(_app(strip({'one'})));
    for (final action in ['edit', 'cancel', 'send']) {
      expect(
        tester
            .widget<TextButton>(find.byKey(ValueKey('scheduled_${action}_one')))
            .onPressed,
        isNull,
      );
    }
  });

  testWidgets('failure recovery belongs to selected row and can dismiss', (
    tester,
  ) async {
    final actions = <String>[];
    await tester.pumpWidget(
      _app(
        ScheduledMessagesStrip(
          messages: [_row('one'), _row('two')],
          failedIds: {'two'},
          onEdit: (_) {},
          onCancel: (_) {},
          onSendNow: (_) {},
          onRetry: (id) => actions.add('retry:$id'),
          onDismissError: (id) => actions.add('dismiss:$id'),
        ),
      ),
    );
    final failed = find.byKey(const ValueKey('scheduled_row_two'));
    expect(
      find.descendant(of: failed, matching: find.text('Retry')),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey('scheduled_retry_two')));
    await tester.tap(find.byKey(const ValueKey('scheduled_dismiss_two')));
    expect(actions, ['retry:two', 'dismiss:two']);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('scheduled_row_one')),
        matching: find.text('Retry'),
      ),
      findsNothing,
    );
  });

  testWidgets('encrypted payload is never previewed or editable', (
    tester,
  ) async {
    await tester.pumpWidget(
      _app(
        ScheduledMessagesStrip(
          messages: [_row('ciphertext', encrypted: true)],
          onEdit: (_) {},
          onCancel: (_) {},
          onSendNow: (_) {},
        ),
      ),
    );
    expect(find.text('Preview ciphertext'), findsNothing);
    expect(
      find.byKey(const ValueKey('scheduled_edit_ciphertext')),
      findsNothing,
    );
    expect(
      find.byKey(const ValueKey('scheduled_cancel_ciphertext')),
      findsOneWidget,
    );
  });

  for (final width in [390.0, 900.0]) {
    testWidgets('H/V $width with scaled text and long queue preserves layout', (
      tester,
    ) async {
      tester.view.physicalSize = Size(width, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      await tester.pumpWidget(
        _app(
          ScheduledMessagesStrip(
            messages: List.generate(30, (i) => _row('$i')),
            onEdit: (_) {},
            onCancel: (_) {},
            onSendNow: (_) {},
          ),
          scale: 1.5,
        ),
      );
      expect(tester.takeException(), isNull);
      expect(
        tester.getSize(find.byType(ScheduledMessagesStrip)).height,
        lessThan(500),
      );
      await tester.drag(
        find.byKey(ScheduledMessagesStrip.listKey),
        const Offset(0, -400),
      );
      await tester.pump();
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('keyboard activates collapse and row actions expose semantics', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      _app(
        ScheduledMessagesStrip(
          messages: [_row('one')],
          onEdit: (_) {},
          onCancel: (_) {},
          onSendNow: (_) {},
        ),
      ),
    );
    expect(
      tester
          .getSemantics(find.byKey(const ValueKey('scheduled_send_one')))
          .label,
      contains('Send now'),
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(find.text('Preview one'), findsNothing);
    semantics.dispose();
  });
}
