import 'package:file_selector/file_selector.dart';
import 'package:record/record.dart';
import 'voice_note_capture.dart';
import 'voice_note_file_io.dart'
    if (dart.library.js_interop) 'voice_note_file_web.dart'
    as files;

/// WAV is supported on every record 6.2 target, including browser encoders.
/// This is a local source format, not a File Service upload contract.
class LocalVoiceNoteRecorder implements VoiceNoteRecorder {
  final AudioRecorder _recorder = AudioRecorder();
  final Stopwatch _clock = Stopwatch();
  String? _path;
  @override
  late final Stream<double> amplitudes = _recorder
      .onAmplitudeChanged(const Duration(milliseconds: 100))
      .map((value) => value.current);

  @override
  Future<bool> hasPermission() => _recorder.hasPermission();

  @override
  Future<void> start() async {
    _path = await files.newVoiceNotePath();
    await _recorder.start(
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
  }

  @override
  Future<CapturedVoiceNote> stop() async {
    _clock.stop();
    final duration = _clock.elapsed;
    final path = await _recorder.stop();
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
    _clock.stop();
    await _recorder.cancel();
    final path = _path;
    _path = null;
    if (path != null && path.isNotEmpty) await files.discardVoiceNote(path);
  }

  @override
  Future<void> dispose() => _recorder.dispose();
}
