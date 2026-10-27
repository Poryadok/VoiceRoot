import 'dart:async';
import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/voice_note_capture.dart';

class FakeRecorder implements VoiceNoteRecorder {
  @override
  Stream<double> get amplitudes => const Stream.empty();
  bool allowed = true;
  int starts = 0, stops = 0, cancels = 0, discards = 0, disposes = 0;
  Duration duration = const Duration(seconds: 2);
  Completer<bool>? permission;
  Completer<void>? starting;
  Completer<CapturedVoiceNote>? stopped;
  bool failStart = false, failStop = false;
  @override
  Future<bool> hasPermission() async =>
      permission == null ? allowed : permission!.future;
  @override
  Future<void> start() async {
    starts++;
    if (starting != null) await starting!.future;
    if (failStart) throw StateError('start');
  }

  @override
  Future<CapturedVoiceNote> stop() async {
    stops++;
    if (failStop) throw StateError('stop');
    return stopped == null
        ? CapturedVoiceNote(
            file: XFile.fromData(
              Uint8List.fromList([1, 2, 3]),
              name: 'voice.wav',
              mimeType: 'audio/wav',
            ),
            duration: duration,
            discard: () async {
              discards++;
            },
          )
        : stopped!.future;
  }

  @override
  Future<void> cancel() async {
    cancels++;
  }

  @override
  Future<void> dispose() async {
    disposes++;
  }
}

void main() {
  late FakeRecorder recorder;
  late List<CapturedVoiceNote> captured;
  Future<void> mount(WidgetTester tester) async {
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: Center(
            child: VoiceNoteCapture(
              recorder: recorder,
              onCaptured: captured.add,
            ),
          ),
        ),
      ),
    );
  }

  Future<TestGesture> hold(WidgetTester tester) async {
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('voice_note_mic'))),
    );
    await tester.pump();
    return gesture;
  }

  setUp(() {
    recorder = FakeRecorder();
    captured = [];
  });
  for (final activation in [
    LogicalKeyboardKey.enter,
    LogicalKeyboardKey.space,
  ]) {
    testWidgets(
      'keyboard activation of cancel with ${activation.keyLabel} never emits audio',
      (tester) async {
        await mount(tester);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.sendKeyEvent(LogicalKeyboardKey.space);
        await tester.pump();
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        await tester.sendKeyEvent(activation);
        await tester.pump();
        expect(recorder.cancels, 1);
        expect(captured, isEmpty);
      },
    );
  }
  testWidgets(
    'cancel during delayed start releases microphone and allows retry',
    (tester) async {
      recorder.starting = Completer<void>();
      await mount(tester);
      final gesture = await hold(tester);
      await gesture.moveBy(const Offset(-90, 0));
      await gesture.up();
      recorder.starting!.complete();
      await tester.pump();
      expect(recorder.cancels, 1);
      expect(captured, isEmpty);
      recorder.starting = null;
      final retry = await hold(tester);
      await retry.up();
      await tester.pump();
      expect(recorder.starts, 2);
      expect(captured, hasLength(1));
    },
  );
  testWidgets('keyboard records hands free and Escape cancels', (tester) async {
    await mount(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.sendKeyEvent(LogicalKeyboardKey.space);
    await tester.pump();
    expect(recorder.starts, 1);
    expect(find.byKey(const Key('voice_note_stop')), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pump();
    expect(recorder.cancels, 1);
    expect(captured, isEmpty);
  });
  testWidgets('start failure shows error and retry can record', (tester) async {
    recorder.failStart = true;
    await mount(tester);
    final first = await hold(tester);
    await first.up();
    await tester.pump();
    expect(find.byKey(const Key('voice_note_error')), findsOneWidget);
    recorder.failStart = false;
    final retry = await hold(tester);
    await retry.up();
    await tester.pump();
    expect(captured, hasLength(1));
  });
  testWidgets(
    'hold starts and release returns local file and measured duration',
    (tester) async {
      await mount(tester);
      final gesture = await hold(tester);
      expect(recorder.starts, 1);
      await gesture.up();
      await tester.pump();
      expect(captured.single.duration, const Duration(seconds: 2));
      expect(await captured.single.file.readAsBytes(), [1, 2, 3]);
    },
  );
  testWidgets('swipe up locks; release keeps recording until stop', (
    tester,
  ) async {
    await mount(tester);
    final gesture = await hold(tester);
    await gesture.moveBy(const Offset(0, -90));
    await tester.pump();
    await gesture.up();
    await tester.pump();
    expect(recorder.stops, 0);
    await tester.tap(find.byKey(const Key('voice_note_stop')));
    await tester.pump();
    expect(captured, hasLength(1));
  });
  testWidgets('swipe left cancels without emitting a file', (tester) async {
    await mount(tester);
    final gesture = await hold(tester);
    await gesture.moveBy(const Offset(-90, 0));
    await tester.pump();
    await gesture.up();
    await tester.pump();
    expect(recorder.cancels, 1);
    expect(captured, isEmpty);
  });
  testWidgets('locked recording auto stops at 60 seconds', (tester) async {
    recorder.duration = const Duration(seconds: 60);
    await mount(tester);
    final gesture = await hold(tester);
    await gesture.moveBy(const Offset(0, -90));
    await gesture.up();
    await tester.pump(const Duration(seconds: 60));
    await tester.pump();
    expect(recorder.stops, 1);
    expect(captured.single.duration, const Duration(seconds: 60));
  });
  testWidgets('under one second discards and displays minimum hint', (
    tester,
  ) async {
    recorder.duration = const Duration(milliseconds: 999);
    await mount(tester);
    final gesture = await hold(tester);
    await gesture.up();
    await tester.pump();
    expect(captured, isEmpty);
    expect(recorder.discards, 1);
    expect(find.byKey(const Key('voice_note_error')), findsOneWidget);
  });
  testWidgets('permission denied shows recovery and never records', (
    tester,
  ) async {
    recorder.allowed = false;
    await mount(tester);
    final gesture = await hold(tester);
    await gesture.up();
    await tester.pump();
    expect(recorder.starts, 0);
    expect(find.byKey(const Key('voice_note_error')), findsOneWidget);
  });
  testWidgets(
    'release during permission prompt does not start late recording',
    (tester) async {
      recorder.permission = Completer<bool>();
      await mount(tester);
      final gesture = await hold(tester);
      await gesture.up();
      recorder.permission!.complete(true);
      await tester.pump();
      expect(recorder.starts, 0);
      expect(captured, isEmpty);
    },
  );
  testWidgets('cancel button cancels locked recording', (tester) async {
    await mount(tester);
    final gesture = await hold(tester);
    await gesture.moveBy(const Offset(0, -90));
    await gesture.up();
    await tester.pump();
    await tester.tap(find.byKey(const Key('voice_note_cancel')));
    await tester.pump();
    expect(recorder.cancels, 1);
    expect(captured, isEmpty);
  });
  testWidgets('stop failure reports error and releases capture', (
    tester,
  ) async {
    recorder.failStop = true;
    await mount(tester);
    final gesture = await hold(tester);
    await gesture.up();
    await tester.pump();
    expect(captured, isEmpty);
    expect(find.byKey(const Key('voice_note_error')), findsOneWidget);
    expect(recorder.cancels, 1);
  });
  testWidgets(
    'processing blocks another recording and disposal releases recorder',
    (tester) async {
      recorder.stopped = Completer<CapturedVoiceNote>();
      await mount(tester);
      final gesture = await hold(tester);
      await gesture.up();
      await tester.pump();
      expect(find.byKey(const Key('voice_note_processing')), findsOneWidget);
      await tester.pumpWidget(const SizedBox());
      recorder.stopped!.complete(
        CapturedVoiceNote(
          file: XFile.fromData(Uint8List(1)),
          duration: recorder.duration,
          discard: () async {
            recorder.discards++;
          },
        ),
      );
      await tester.pump();
      expect(captured, isEmpty);
      expect(recorder.discards, 1);
      expect(recorder.disposes, 1);
    },
  );
}
