import 'dart:async';

import 'package:flutter/material.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';
import '../call/call_modal_overlay.dart';

typedef MatchRespondCallback =
    Future<RespondToMatchData?> Function(bool accept);
typedef MatchRefreshCallback = Future<bool> Function();

/// A server-authored deadline measured from a monotonic clock after receipt.
/// Keeping this object in controller state prevents a widget remount from
/// restarting the user's acceptance window.
class MatchDeadlineClock {
  MatchDeadlineClock({
    required DateTime serverNow,
    required DateTime deadline,
    Duration Function()? elapsed,
  }) : _remainingAtReceipt = deadline.difference(serverNow),
       _elapsed = Stopwatch()..start(),
       _elapsedOverride = elapsed;

  final Duration _remainingAtReceipt;
  final Stopwatch _elapsed;
  final Duration Function()? _elapsedOverride;

  Duration get remaining {
    final value =
        _remainingAtReceipt - (_elapsedOverride?.call() ?? _elapsed.elapsed);
    return value.isNegative ? Duration.zero : value;
  }
}

/// High-priority accept/decline popup when a match is found (Penpot §6.4 · v2).
class MatchFoundOverlay extends StatefulWidget {
  const MatchFoundOverlay({
    super.key,
    required this.match,
    this.onRespond,
    this.deadlineClock,
    this.onRefresh,
  });

  final MatchData match;
  final MatchRespondCallback? onRespond;
  final MatchDeadlineClock? deadlineClock;
  final MatchRefreshCallback? onRefresh;

  static const Key acceptButtonKey = Key('match_found_accept');
  static const Key declineButtonKey = Key('match_found_decline');
  static const Key timerKey = Key('match_found_timer');

  @override
  State<MatchFoundOverlay> createState() => _MatchFoundOverlayState();
}

class _MatchFoundOverlayState extends State<MatchFoundOverlay>
    with WidgetsBindingObserver {
  bool _busy = false;
  bool _deadlineRefreshRequested = false;
  late int? _secondsLeft;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _secondsLeft = _readSecondsLeft();
    _timer = Timer.periodic(const Duration(milliseconds: 200), (_) {
      if (!mounted) return;
      final next = _readSecondsLeft();
      if (next != _secondsLeft) setState(() => _secondsLeft = next);
      if (next == 0) _refreshAtDeadline();
    });
    if (_secondsLeft == 0) unawaited(_refreshAtDeadline());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _timer?.cancel();
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant MatchFoundOverlay oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.match.id != widget.match.id ||
        oldWidget.match.acceptanceDeadlineAt !=
            widget.match.acceptanceDeadlineAt) {
      _deadlineRefreshRequested = false;
    }
    final next = _readSecondsLeft();
    if (next != _secondsLeft) _secondsLeft = next;
    if (next == 0) unawaited(_refreshAtDeadline());
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) unawaited(_refresh());
  }

  int? _readSecondsLeft() {
    final clock = widget.deadlineClock;
    if (clock == null) return null;
    final milliseconds = clock.remaining.inMilliseconds;
    return (milliseconds / Duration.millisecondsPerSecond).ceil();
  }

  Future<void> _refreshAtDeadline() async {
    if (_deadlineRefreshRequested || widget.onRefresh == null) return;
    _deadlineRefreshRequested = true;
    await _refresh();
  }

  Future<bool> _refresh() async {
    final refresh = widget.onRefresh;
    if (refresh == null || !mounted) return false;
    return refresh();
  }

  Future<void> _respond(bool accept) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      if (widget.onRespond != null) {
        await widget.onRespond!(accept);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final radius = context.voiceMetrics.corner('md', fallback: 6);
    final gameName = widget.match.gameName ?? widget.match.gameId;
    return Positioned.fill(
      child: Material(
        color: voice.canvas.withValues(alpha: kVoiceOverlayDimOpacity),
        child: Center(
          child: Container(
            width: 360,
            constraints: const BoxConstraints(minHeight: 300),
            padding: const EdgeInsets.all(24),
            decoration: BoxDecoration(
              color: voice.elevated,
              borderRadius: BorderRadius.circular(radius),
              border: Border.all(color: voice.borderDefault),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  l10n.matchFoundTitle,
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 4),
                Text(
                  l10n.matchFoundSubtitle(gameName, widget.match.mode),
                  textAlign: TextAlign.center,
                  style: Theme.of(
                    context,
                  ).textTheme.bodyMedium?.copyWith(color: voice.textSecondary),
                ),
                const SizedBox(height: 12),
                Text(
                  _secondsLeft == null ? '—' : '${_secondsLeft}s',
                  key: MatchFoundOverlay.timerKey,
                  style: Theme.of(
                    context,
                  ).textTheme.titleSmall?.copyWith(color: voice.textSecondary),
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    Expanded(
                      child: FilledButton(
                        key: MatchFoundOverlay.acceptButtonKey,
                        onPressed: _busy ? null : () => _respond(true),
                        child: Text(l10n.matchFoundAccept),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: OutlinedButton(
                        key: MatchFoundOverlay.declineButtonKey,
                        onPressed: _busy ? null : () => _respond(false),
                        style: OutlinedButton.styleFrom(
                          foregroundColor: voice.error,
                          side: BorderSide(color: voice.error),
                        ),
                        child: Text(l10n.matchFoundDecline),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
