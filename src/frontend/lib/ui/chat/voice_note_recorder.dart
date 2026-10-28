import 'dart:async';
import 'package:file_selector/file_selector.dart';
import 'package:record/record.dart';
import 'voice_note_capture.dart';
import 'voice_note_file_io.dart'
    if (dart.library.js_interop) 'voice_note_file_web.dart'
    as files;

/// WAV is supported on every record 6.2 target, including browser encoders.
/// This is a local source format, not a File Service upload contract.
class LocalVoiceNoteRecorder implements VoiceNoteRecorder {
  AudioRecorder? _recorder;
  AudioRecorder get _backend {
    if (_disposed) throw StateError('Recorder disposed');
    return _recorder ??= AudioRecorder();
  }

  final Stopwatch _clock = Stopwatch();
  String? _path;
  bool _recording = false, _disposed = false, _readingAmplitude = false;
  int _pollGeneration = 0;
  Timer? _amplitudeTimer;
  late final _amplitudeController = StreamController<double>.broadcast(
    onListen: _beginPolling,
    onCancel: _stopPolling,
  );

  // record 6.2 onAmplitudeChanged starts an internal timer on getter access,
  // including while idle, and cannot synchronously stop it on listener cancel.
  // Own the polling timer so removing the mic always releases it immediately.
  @override
  Stream<double> get amplitudes => _amplitudeController.stream;

  void _beginPolling() {
    if (!_recording ||
        _disposed ||
        !_amplitudeController.hasListener ||
        _amplitudeTimer != null) {
      return;
    }
    _amplitudeTimer = Timer.periodic(
      const Duration(milliseconds: 100),
      (_) => unawaited(_readAmplitude()),
    );
  }

  void _stopPolling() {
    _pollGeneration++;
    _amplitudeTimer?.cancel();
    _amplitudeTimer = null;
  }

  Future<void> _readAmplitude() async {
    final recorder = _recorder;
    if (recorder == null || _readingAmplitude || !_recording || _disposed)
      return;
    final generation = _pollGeneration;
    _readingAmplitude = true;
    try {
      final value = await recorder.getAmplitude();
      if (!_disposed &&
          _recording &&
          generation == _pollGeneration &&
          _amplitudeController.hasListener) {
        _amplitudeController.add(value.current);
      }
    } catch (_) {
      // Metering failure must not discard an otherwise valid recording.
    } finally {
      _readingAmplitude = false;
    }
  }

  @override
  Future<bool> hasPermission() async => _backend.hasPermission();

  @override
  Future<void> start() async {
    if (_disposed) throw StateError('Recorder disposed');
    _path = await files.newVoiceNotePath();
    await _backend.start(
      const RecordConfig(
        encoder: AudioEncoder.wav,
        numChannels: 1,
        sampleRate: 16000,
      ),
      path: _path!,
    );
    _clock
      ..reset()
      ..start();
    _recording = true;
    _beginPolling();
  }

  @override
  Future<CapturedVoiceNote> stop() async {
    final recorder = _recorder;
    if (_disposed || recorder == null || !_recording) {
      throw StateError('Recorder not started');
    }
    _recording = false;
    _stopPolling();
    _clock.stop();
    final duration = _clock.elapsed;
    final path = await recorder.stop();
    if (path == null || path.isEmpty) throw StateError('No recorded audio');
    _path = null;
    return CapturedVoiceNote(
      file: XFile(path, name: 'voice.wav', mimeType: 'audio/wav'),
      duration: duration,
      discard: () => files.discardVoiceNote(path),
    );
  }

  @override
  Future<void> cancel() async {
    if (_disposed) return;
    _recording = false;
    _stopPolling();
    _clock.stop();
    await _recorder?.cancel();
    final path = _path;
    _path = null;
    if (path != null && path.isNotEmpty) await files.discardVoiceNote(path);
  }

  @override
  Future<void> dispose() async {
    if (_disposed) return;
    _recording = false;
    _disposed = true;
    _stopPolling();
    await _amplitudeController.close();
    await _recorder?.dispose();
  }
}
