import 'dart:async';
import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../l10n/app_localizations.dart';

/// Local recording; the recipient owns the file until [discard] completes.
class CapturedVoiceNote {
  CapturedVoiceNote({
    required this.file,
    required this.duration,
    required this.discard,
  });
  final XFile file;
  final Duration duration;
  final Future<void> Function() discard;
}

abstract interface class VoiceNoteRecorder {
  Stream<double> get amplitudes;
  Future<bool> hasPermission();
  Future<void> start();
  Future<CapturedVoiceNote> stop();
  Future<void> cancel();
  Future<void> dispose();
}

class VoiceNoteCapture extends StatefulWidget {
  const VoiceNoteCapture({
    super.key,
    required this.recorder,
    required this.onCaptured,
  });

  /// Stable for this widget's keyed lifetime; capture owns cancel/dispose.
  /// Key by conversation/viewer so navigation cannot deliver stale results.
  final VoiceNoteRecorder recorder;
  final ValueChanged<CapturedVoiceNote> onCaptured;
  @override
  State<VoiceNoteCapture> createState() => _VoiceNoteCaptureState();
}

enum _CapturePhase { idle, permission, recording, processing }

enum _CaptureError { permission, tooShort, failed }

class _VoiceNoteCaptureState extends State<VoiceNoteCapture>
    with WidgetsBindingObserver {
  _CapturePhase _phase = _CapturePhase.idle;
  _CaptureError? _error;
  bool _held = false, _locked = false, _cancelled = false;
  Offset _origin = Offset.zero;
  int? _pointer;
  Timer? _cap, _ticker;
  final Stopwatch _clock = Stopwatch();
  Future<void> _operation = Future.value();
  StreamSubscription<double>? _amplitudeSubscription;
  double _amplitude = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _amplitudeSubscription = widget.recorder.amplitudes.listen((value) {
      if (mounted && _phase == _CapturePhase.recording && value.isFinite) {
        setState(() => _amplitude = ((value + 60) / 60).clamp(0, 1));
      }
    }, onError: (Object _) {});
  }

  void _timersOff() {
    _cap?.cancel();
    _ticker?.cancel();
    _clock.stop();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) _cancel();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _cancelled = true;
    _held = false;
    _timersOff();
    unawaited(_amplitudeSubscription?.cancel());
    // Serialize cleanup after an in-flight permission/start/stop operation.
    unawaited(
      _operation.whenComplete(() async {
        try {
          await widget.recorder.cancel();
        } catch (_) {}
        try {
          await widget.recorder.dispose();
        } catch (_) {}
      }),
    );
    super.dispose();
  }

  Future<void> _start() async {
    setState(() {
      _phase = _CapturePhase.permission;
      _error = null;
    });
    try {
      final allowed = await widget.recorder.hasPermission();
      if (!mounted) return;
      if (!allowed) {
        setState(() {
          _phase = _CapturePhase.idle;
          _error = _CaptureError.permission;
        });
        return;
      }
      if (_cancelled || (!_held && !_locked)) {
        setState(() => _phase = _CapturePhase.idle);
        return;
      }
      await widget.recorder.start();
      if (!mounted || _cancelled) {
        await widget.recorder.cancel();
        if (mounted) setState(() => _phase = _CapturePhase.idle);
        return;
      }
      _clock
        ..reset()
        ..start();
      setState(() => _phase = _CapturePhase.recording);
      _cap = Timer(const Duration(seconds: 60), _finish);
      _ticker = Timer.periodic(const Duration(milliseconds: 100), (_) {
        if (mounted) setState(() {});
      });
      if (!_held && !_locked) _finish();
    } catch (_) {
      try {
        await widget.recorder.cancel();
      } catch (_) {}
      if (mounted) {
        setState(() {
          _phase = _CapturePhase.idle;
          _error = _CaptureError.failed;
        });
      }
    }
  }

  void _finish() {
    if (_phase != _CapturePhase.recording) return;
    _timersOff();
    setState(() => _phase = _CapturePhase.processing);
    _operation = () async {
      try {
        final note = await widget.recorder.stop();
        if (!mounted || _cancelled) {
          await note.discard();
          if (mounted) setState(() => _phase = _CapturePhase.idle);
          return;
        }
        if (note.duration < const Duration(seconds: 1)) {
          await note.discard();
          if (mounted) {
            setState(() {
              _phase = _CapturePhase.idle;
              _error = _CaptureError.tooShort;
            });
          }
          return;
        }
        setState(() {
          _phase = _CapturePhase.idle;
          _locked = false;
        });
        widget.onCaptured(note);
      } catch (_) {
        try {
          await widget.recorder.cancel();
        } catch (_) {}
        if (mounted) {
          setState(() {
            _phase = _CapturePhase.idle;
            _error = _CaptureError.failed;
          });
        }
      }
    }();
  }

  void _cancel() {
    _held = false;
    _cancelled = true;
    _locked = false;
    _timersOff();
    if (_phase != _CapturePhase.recording) return;
    setState(() => _phase = _CapturePhase.processing);
    _operation = () async {
      try {
        await widget.recorder.cancel();
        if (mounted) setState(() => _phase = _CapturePhase.idle);
      } catch (_) {
        if (mounted) {
          setState(() {
            _phase = _CapturePhase.idle;
            _error = _CaptureError.failed;
          });
        }
      }
    }();
  }

  void _activate() {
    if (_phase == _CapturePhase.idle) {
      _held = false;
      _cancelled = false;
      _locked = true;
      _operation = _start();
    } else if (_phase == _CapturePhase.recording) {
      _finish();
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final recording = _phase == _CapturePhase.recording;
    final busy =
        _phase == _CapturePhase.permission ||
        _phase == _CapturePhase.processing;
    final error = switch (_error) {
      _CaptureError.permission => l10n.composerVoicePermission,
      _CaptureError.tooShort => l10n.composerVoiceTooShort,
      _CaptureError.failed => l10n.composerVoiceError,
      null => null,
    };
    return Focus(
      onKeyEvent: (node, event) {
        if (event is! KeyDownEvent) return KeyEventResult.ignored;
        if (event.logicalKey == LogicalKeyboardKey.escape) {
          _cancel();
          return KeyEventResult.handled;
        }
        if (node.hasPrimaryFocus &&
            (event.logicalKey == LogicalKeyboardKey.space ||
                event.logicalKey == LogicalKeyboardKey.enter)) {
          _activate();
          return KeyEventResult.handled;
        }
        return KeyEventResult.ignored;
      },
      child: SizedBox(
        width: recording || busy || error != null ? 180 : 48,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (error != null)
              Semantics(
                liveRegion: true,
                child: Text(error, key: const Key('voice_note_error')),
              ),
            if (busy) ...[
              const LinearProgressIndicator(key: Key('voice_note_processing')),
              Text(l10n.composerVoiceProcessing),
            ],
            if (recording) ...[
              Text(_locked ? l10n.composerVoiceLocked : l10n.composerVoiceHold),
              Text('0:${_clock.elapsed.inSeconds.toString().padLeft(2, '0')}'),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  for (var i = 0; i < 9; i++)
                    Container(
                      width: 3,
                      height: 3 + _amplitude * (i.isEven ? 22 : 12),
                      margin: const EdgeInsets.all(2),
                      color: Theme.of(context).colorScheme.primary,
                    ),
                ],
              ),
            ],
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (recording)
                  IconButton(
                    key: const Key('voice_note_cancel'),
                    tooltip: l10n.composerVoiceCancel,
                    onPressed: _cancel,
                    icon: const Icon(Icons.close),
                  ),
                if (recording && _locked)
                  IconButton(
                    key: const Key('voice_note_stop'),
                    tooltip: l10n.composerVoiceStop,
                    onPressed: _finish,
                    icon: const Icon(Icons.stop),
                  )
                else
                  Listener(
                    key: const Key('voice_note_mic'),
                    onPointerDown: (event) {
                      if (_phase != _CapturePhase.idle) return;
                      _pointer = event.pointer;
                      _origin = event.position;
                      _held = true;
                      _cancelled = false;
                      _locked = false;
                      _operation = _start();
                    },
                    onPointerMove: (event) {
                      if (event.pointer != _pointer) return;
                      if (!_held || _cancelled || _locked) return;
                      final delta = event.position - _origin;
                      if (delta.dx <= -64) {
                        _cancel();
                      } else if (delta.dy <= -64) {
                        setState(() => _locked = true);
                      }
                    },
                    onPointerUp: (event) {
                      if (event.pointer != _pointer) return;
                      _pointer = null;
                      _held = false;
                      if (!_locked) _finish();
                    },
                    onPointerCancel: (event) {
                      if (event.pointer == _pointer) _cancel();
                    },
                    child: Semantics(
                      button: true,
                      onTap: _activate,
                      label: l10n.composerVoiceHold,
                      child: Tooltip(
                        message: l10n.composerVoiceHold,
                        child: const SizedBox(
                          width: 48,
                          height: 48,
                          child: Icon(Icons.mic),
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
