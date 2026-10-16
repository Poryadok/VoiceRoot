import 'dart:async';
import 'dart:typed_data';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_soloud/flutter_soloud.dart';
import 'music_format.dart';
import 'music_module_ready.dart'
    if (dart.library.js_interop) 'music_module_ready_web.dart';

final musicPlaybackFactoryProvider = Provider<MusicPlayback Function()>(
  (ref) => PullMusicPlayback.new,
);

/// One admitted Music source. No URL/file API or native decoder fallback.
abstract class MusicPlayback extends ChangeNotifier {
  bool get closed;
  bool get failed;
  bool get playing;
  Duration get position;
  Duration get duration;
  String? get title;
  String? get artist;
  String? get album;
  Future<void> open(Uint8List bytes);
  void pause();
  void resume();
  void seek(Duration position);
  Future<void> close();
}

class PullMusicPlayback extends MusicPlayback {
  static Future<void>? _initializing;
  static PullMusicPlayback? _active;
  static int _activation = 0;
  final SoLoud _audio = SoLoud.instance;
  Uint8List? _bytes;
  AudioSource? _source;
  SoundHandle? _handle;
  Timer? _timer;
  bool _closed = false;
  bool _failed = false;
  Future<void>? _closing;
  bool _notifierDisposed = false;
  bool _playing = false;
  Duration _duration = Duration.zero;
  String? _title, _artist, _album;
  int _fed = 0;
  int _generation = 0;
  @override
  bool get playing => _playing;
  @override
  bool get closed => _closed;
  @override
  bool get failed => _failed;
  @override
  Duration get duration => _duration;
  @override
  String? get title => _title;
  @override
  String? get artist => _artist;
  @override
  String? get album => _album;
  @override
  Duration get position => _handle == null
      ? Duration.zero
      : _audio.getIsValidVoiceHandle(_handle!)
      ? _audio.getPosition(_handle!)
      : _duration;

  static String? _tag(String? value) {
    if (value == null ||
        value.length > 1024 ||
        RegExp(r'[\x00-\x1f\x7f]').hasMatch(value))
      return null;
    final clean = value.trim();
    return clean.isEmpty ? null : clean;
  }

  @override
  Future<void> open(Uint8List bytes) async {
    if (_closed || !hasSupportedMusicSignature(bytes))
      throw const FormatException('music_format_invalid');
    final activation = ++_activation;
    final previous = _active;
    if (previous != null && previous != this) await previous.close();
    if (_closed || activation != _activation)
      throw const FormatException('music_load_cancelled');
    _active = this;
    final generation = ++_generation;
    _initializing ??= musicModuleReady().then((_) => _audio.init());
    try {
      await _initializing!.timeout(const Duration(seconds: 10));
    } catch (_) {
      _initializing = null;
      rethrow;
    }
    if (_closed || generation != _generation || _active != this)
      throw const FormatException('music_load_cancelled');
    _bytes = Uint8List.fromList(bytes);
    final ready = Completer<void>();
    void fail() {
      _failed = true;
      if (!ready.isCompleted)
        ready.completeError(const FormatException('music_decode_failed'));
      unawaited(close());
    }

    void feed(int offset) {
      scheduleMicrotask(() {
        final input = _bytes;
        final source = _source;
        if (_closed ||
            generation != _generation ||
            input == null ||
            source == null)
          return;
        if (offset < 0 || offset > input.length) {
          fail();
          return;
        }
        if (offset == input.length) return;
        final end = (offset + 64 * 1024).clamp(0, input.length);
        _fed += end - offset;
        // A bounded complete pass plus duration/seek probes. Reset only on an
        // explicit user seek, never on a decoder's repeated offset callback.
        if (_fed > input.length * 3 + 64 * 1024) {
          fail();
          return;
        }
        try {
          _audio.addPullBufferDataStream(
            source,
            Uint8List.sublistView(input, offset, end),
            offset: offset,
          );
        } catch (_) {
          fail();
        }
      });
    }

    try {
      _source = _audio.setPullBufferStream(
        bufferSizeBytes: 4 * 1024 * 1024,
        audioSizeBytes: bytes.length,
        format: BufferType.auto,
        onMoreDataIsNeeded: feed,
        onAudioDuration: (seconds) {
          if (_closed || generation != _generation) return;
          if (!seconds.isFinite || seconds <= 0 || seconds > 2147483647) {
            fail();
            return;
          }
          _duration = Duration(milliseconds: (seconds * 1000).ceil());
          if (!ready.isCompleted) ready.complete();
          notifyListeners();
        },
        onMetadata: (metadata) {
          if (_closed || generation != _generation) return;
          if (!const {
            DetectedType.mp3WithId3,
            DetectedType.mp3Stream,
            DetectedType.oggFlac,
            DetectedType.oggOpus,
            DetectedType.oggVorbis,
            DetectedType.wav,
          }.contains(metadata.detectedType)) {
            fail();
            return;
          }
          final comments =
              metadata.oggMetadata?.comments ?? const <String, String>{};
          String? comment(String key) {
            final values = comments.entries
                .where((entry) => entry.key.toLowerCase() == key)
                .toList();
            return values.length == 1 ? values.single.value : null;
          }

          _title = _tag(metadata.mp3Metadata?.title ?? comment('title'));
          _artist = _tag(metadata.mp3Metadata?.artist ?? comment('artist'));
          _album = _tag(metadata.mp3Metadata?.album ?? comment('album'));
          notifyListeners();
        },
      );
      _handle = _audio.play(_source!, paused: true);
      feed(0);
      await ready.future.timeout(const Duration(seconds: 10));
      if (_closed || _active != this)
        throw const FormatException('music_load_cancelled');
      resume();
      _timer = Timer.periodic(const Duration(milliseconds: 250), (_) {
        if (_closed || _handle == null) return;
        if (!_audio.getIsValidVoiceHandle(_handle!)) _playing = false;
        notifyListeners();
      });
    } catch (_) {
      await close();
      rethrow;
    }
  }

  @override
  void pause() {
    if (_closed || _handle == null) return;
    _audio.setPause(_handle!, true);
    _playing = false;
    notifyListeners();
  }

  @override
  void resume() {
    if (_closed || _handle == null) return;
    if (!_audio.getIsValidVoiceHandle(_handle!)) {
      _handle = _audio.play(_source!, paused: true);
      _fed = 0;
      _audio.seek(_handle!, Duration.zero);
    }
    _audio.setPause(_handle!, false);
    _playing = true;
    notifyListeners();
  }

  @override
  void seek(Duration position) {
    if (_closed ||
        _handle == null ||
        position < Duration.zero ||
        position > _duration)
      return;
    _fed = 0;
    _audio.seek(_handle!, position);
    notifyListeners();
  }

  @override
  Future<void> close() => _closing ??= _close();

  Future<void> _close() async {
    _closed = true;
    _generation++;
    _timer?.cancel();
    _playing = false;
    final source = _source;
    final handle = _handle;
    _source = null;
    _handle = null;
    final bytes = _bytes;
    _bytes = null;
    if (bytes != null) bytes.fillRange(0, bytes.length, 0);
    try {
      try {
        if (handle != null && _audio.getIsValidVoiceHandle(handle)) {
          await _audio.stop(handle);
        }
      } finally {
        if (source != null) await _audio.disposeSource(source);
      }
    } catch (_) {
      _failed = true;
    } finally {
      if (identical(_active, this)) _active = null;
      if (!_notifierDisposed) notifyListeners();
    }
  }

  @override
  void dispose() {
    _notifierDisposed = true;
    unawaited(close());
    super.dispose();
  }
}
