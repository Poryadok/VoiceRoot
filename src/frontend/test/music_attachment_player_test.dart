import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/music_attachment_player.dart';
import 'package:voice_frontend/ui/chat/music_playback.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

class _Playback extends MusicPlayback {
  @override
  bool closed = false;
  @override
  bool failed = false;
  @override
  bool playing = false;
  @override
  Duration position = Duration.zero;
  @override
  Duration duration = const Duration(seconds: 120);
  @override
  String? title = 'Local title';
  @override
  String? artist = 'Local artist';
  @override
  String? album = 'Local album';
  int opens = 0;
  int pauses = 0;
  int closes = 0;
  @override
  Future<void> open(Uint8List bytes) async {
    opens++;
    playing = true;
    notifyListeners();
  }

  @override
  void pause() {
    pauses++;
    playing = false;
    notifyListeners();
  }

  @override
  void resume() {
    playing = true;
    notifyListeners();
  }

  @override
  void seek(Duration value) {
    position = value;
    notifyListeners();
  }

  @override
  Future<void> close() async {
    if (!closed) {
      closed = true;
      closes++;
      playing = false;
    }
  }

  @override
  void dispose() {
    unawaited(close());
    super.dispose();
  }
}

void main() {
  final captureDirectory = Platform.environment['VOICE_MUSIC_CAPTURE_DIR'];
  for (final wide in [true, false]) {
    testWidgets(
      'Music inline ordinary and E2E controls ${wide ? 'H' : 'V'}',
      (tester) async {
        tester.view.physicalSize = wide
            ? const Size(1280, 800)
            : const Size(390, 844);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final theme = await tester.runAsync(() async {
          await (FontLoader(VoiceTheme.fontFamily)
                ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
                ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
                ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
                ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf')))
              .load();
          final flutterRoot = Platform.environment['FLUTTER_ROOT'];
          if (flutterRoot == null) throw StateError('capture_font_setup');
          final icons = await File(
            '$flutterRoot/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
          ).readAsBytes();
          await (FontLoader(
            'MaterialIcons',
          )..addFont(Future.value(ByteData.sublistView(icons)))).load();
          final catalog = await VoiceTokenCatalog.load();
          return VoiceTheme.build(
            catalog: catalog,
            mode: VoiceThemeMode.dark,
            profileAccent: catalog.profileAccentAt(0),
          );
        });
        const boundaryKey = ValueKey('music_capture_root');
        await tester.pumpWidget(
          MaterialApp(
            theme: theme,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: RepaintBoundary(
              key: boundaryKey,
              child: Scaffold(
                body: Center(
                  child: SingleChildScrollView(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        for (final encrypted in [false, true])
                          Padding(
                            padding: const EdgeInsets.all(16),
                            child: MusicAttachmentPlayer(
                              attachment: MessageAttachment(
                                fileId: encrypted ? 'encrypted' : 'ordinary',
                                type: 'music',
                                e2eKeyWire: encrypted ? 'wrapped-key' : null,
                                musicTitle: 'Server title',
                                musicArtist: 'Server artist',
                                musicAlbum: 'Server album',
                                durationSeconds: 120,
                              ),
                              createPlayback: _Playback.new,
                              loadBytes: () async =>
                                  Uint8List.fromList([0xff, 0xfb, 0x90, 0]),
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        );
        for (final id in ['ordinary', 'encrypted']) {
          await tester.tap(find.byKey(ValueKey('music_toggle_$id')));
          await tester.pumpAndSettle();
        }
        expect(find.text('Server title'), findsOneWidget);
        expect(find.text('Local title'), findsOneWidget);
        expect(find.byIcon(Icons.pause), findsNWidgets(2));
        expect(tester.takeException(), isNull);
        final boundary = tester.renderObject<RenderRepaintBoundary>(
          find.byKey(boundaryKey),
        );
        final saved = await tester.runAsync(() async {
          final image = await boundary.toImage(pixelRatio: 1);
          try {
            final bytes = await image.toByteData(
              format: ui.ImageByteFormat.png,
            );
            if (bytes == null) return false;
            final target = File(
              '$captureDirectory/music-${wide ? 'h' : 'v'}.png',
            );
            await target.parent.create(recursive: true);
            await target.writeAsBytes(bytes.buffer.asUint8List(), flush: true);
            return true;
          } finally {
            image.dispose();
          }
        });
        expect(saved, isTrue);
        await tester.pumpWidget(const SizedBox());
      },
      skip: captureDirectory == null || captureDirectory.isEmpty,
    );
  }
  Future<void> mount(
    WidgetTester tester,
    _Playback playback,
    Future<Uint8List> Function() load, {
    bool encrypted = false,
  }) => tester.pumpWidget(
    MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(
        body: MusicAttachmentPlayer(
          attachment: MessageAttachment(
            fileId: 'music',
            type: 'music',
            e2eKeyWire: encrypted ? 'wrapped-key' : null,
            musicTitle: 'Server title',
            musicArtist: 'Server artist',
            durationSeconds: 120,
          ),
          createPlayback: () => playback,
          loadBytes: load,
        ),
      ),
    ),
  );
  Uint8List mp3() => Uint8List.fromList([0xff, 0xfb, 0x90, 0]);

  testWidgets(
    'late decoder failure shows error but ordinary retirement does not',
    (tester) async {
      for (final failed in [false, true]) {
        final playback = _Playback();
        await mount(tester, playback, () async => mp3());
        await tester.tap(find.byKey(const ValueKey('music_toggle_music')));
        await tester.pumpAndSettle();
        playback.failed = failed;
        await playback.close();
        playback.notifyListeners();
        await tester.pump();
        expect(
          find.byKey(const ValueKey('music_error_music')),
          failed ? findsOneWidget : findsNothing,
        );
        await tester.pumpWidget(const SizedBox());
      }
    },
  );

  testWidgets(
    'ordinary player preserves canonical tags and plays pauses seeks disposes',
    (tester) async {
      final playback = _Playback();
      final bytes = mp3();
      await mount(tester, playback, () async => bytes);
      expect(find.text('Server title'), findsOneWidget);
      expect(playback.opens, 0);
      await tester.tap(find.byKey(const ValueKey('music_toggle_music')));
      await tester.pumpAndSettle();
      expect(playback.opens, 1);
      expect(bytes, everyElement(0));
      expect(find.text('Local title'), findsNothing);
      await tester.tap(find.byKey(const ValueKey('music_toggle_music')));
      await tester.pump();
      expect(playback.pauses, 1);
      final slider = tester.widget<Slider>(
        find.byKey(const ValueKey('music_seek_music')),
      );
      slider.onChanged!(30000);
      expect(playback.position, const Duration(seconds: 30));
      await tester.pumpWidget(const SizedBox());
      expect(playback.closes, 1);
    },
  );
  testWidgets(
    'encrypted player never renders public tags and derives local tags only after decrypt',
    (tester) async {
      final playback = _Playback();
      await mount(tester, playback, () async => mp3(), encrypted: true);
      expect(find.text('Server title'), findsNothing);
      expect(find.text('Local title'), findsNothing);
      await tester.tap(find.byKey(const ValueKey('music_toggle_music')));
      await tester.pumpAndSettle();
      expect(find.text('Local title'), findsOneWidget);
      expect(find.text('Local artist'), findsOneWidget);
      expect(find.text('Local album'), findsOneWidget);
      expect(find.text('Server artist'), findsNothing);
    },
  );
  testWidgets('decrypt failure and unsupported bytes never reach playback', (
    tester,
  ) async {
    for (final load in <Future<Uint8List> Function()>[
      () async => throw const FormatException('decrypt failed'),
      () async => Uint8List.fromList([0xff, 0xf1, 0x50, 0x80]),
    ]) {
      final playback = _Playback();
      await mount(tester, playback, load, encrypted: true);
      await tester.tap(find.byKey(const ValueKey('music_toggle_music')));
      await tester.pumpAndSettle();
      expect(playback.opens, 0);
      expect(find.byKey(const ValueKey('music_error_music')), findsOneWidget);
      await tester.pumpWidget(const SizedBox());
    }
  });
  testWidgets('late download after disposal is erased without starting audio', (
    tester,
  ) async {
    final playback = _Playback();
    final pending = Completer<Uint8List>();
    await mount(tester, playback, () => pending.future);
    await tester.tap(find.byKey(const ValueKey('music_toggle_music')));
    await tester.pumpWidget(const SizedBox());
    final bytes = mp3();
    pending.complete(bytes);
    await tester.pump();
    expect(playback.opens, 0);
    expect(bytes, everyElement(0));
  });
}
