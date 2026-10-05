import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

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
    Duration requestElapsed = Duration.zero,
  }) : _remainingAtReceipt = _nonNegative(
         deadline.difference(serverNow) - requestElapsed,
       ),
       _elapsed = Stopwatch()..start(),
       _elapsedOverride = elapsed;

  Duration _remainingAtReceipt;
  final Stopwatch _elapsed;
  final Duration Function()? _elapsedOverride;
  Duration _elapsedAtReceipt = Duration.zero;
  Duration _lastElapsed = Duration.zero;

  static Duration _nonNegative(Duration value) =>
      value.isNegative ? Duration.zero : value;

  Duration _elapsedNow() {
    final value = _elapsedOverride?.call() ?? _elapsed.elapsed;
    if (value > _lastElapsed) _lastElapsed = value;
    return _lastElapsed;
  }

  Duration get remaining {
    return _nonNegative(
      _remainingAtReceipt - (_elapsedNow() - _elapsedAtReceipt),
    );
  }

  void updateFromServer({
    required DateTime serverNow,
    required DateTime deadline,
    Duration requestElapsed = Duration.zero,
  }) {
    final current = remaining;
    final reported = _nonNegative(
      deadline.difference(serverNow) - requestElapsed,
    );
    _remainingAtReceipt = reported < current ? reported : current;
    _elapsedAtReceipt = _elapsedNow();
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
    this.isResponding = false,
  });

  final MatchData match;
  final MatchRespondCallback? onRespond;
  final MatchDeadlineClock? deadlineClock;
  final MatchRefreshCallback? onRefresh;
  final bool isResponding;

  static const Key acceptButtonKey = Key('match_found_accept');
  static const Key declineButtonKey = Key('match_found_decline');
  static const Key timerKey = Key('match_found_timer');
  static const Key modalSemanticsKey = Key('match_found_modal_semantics');

  @override
  State<MatchFoundOverlay> createState() => _MatchFoundOverlayState();
}

class _MatchFoundOverlayState extends State<MatchFoundOverlay>
    with WidgetsBindingObserver {
  bool _busy = false;
  bool _deadlineRefreshInFlight = false;
  bool _deadlineRefreshStopped = false;
  int _deadlineRefreshAttempts = 0;
  Timer? _deadlineRetryTimer;
  late int? _secondsLeft;
  Timer? _timer;
  late final FocusScopeNode _modalScope;
  late final FocusNode _acceptFocusNode;
  late final FocusNode _declineFocusNode;
  FocusNode? _returnFocus;
  Future<bool>? _refreshFlight;

  @override
  void initState() {
    super.initState();
    _modalScope = FocusScopeNode(debugLabel: 'MatchFound modal');
    _acceptFocusNode = FocusNode(debugLabel: 'MatchFound accept');
    _declineFocusNode = FocusNode(debugLabel: 'MatchFound decline');
    _returnFocus = FocusManager.instance.primaryFocus;
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _acceptFocusNode.requestFocus();
    });
    _secondsLeft = _readSecondsLeft();
    _timer = Timer.periodic(const Duration(milliseconds: 200), (_) {
      if (!mounted) return;
      final next = _readSecondsLeft();
      if (next != _secondsLeft) setState(() => _secondsLeft = next);
      if (next == 0 &&
          !_deadlineRefreshStopped &&
          !_deadlineRefreshInFlight &&
          _deadlineRetryTimer == null) {
        _refreshAtDeadline();
      }
    });
    if (_secondsLeft == 0) unawaited(_refreshAtDeadline());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _timer?.cancel();
    _deadlineRetryTimer?.cancel();
    _modalScope.dispose();
    _acceptFocusNode.dispose();
    _declineFocusNode.dispose();
    final returnFocus = _returnFocus;
    if (returnFocus != null && returnFocus.context != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (returnFocus.context != null && returnFocus.canRequestFocus) {
          returnFocus.requestFocus();
        }
      });
    }
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant MatchFoundOverlay oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.match.id != widget.match.id) {
      _deadlineRefreshAttempts = 0;
      _deadlineRefreshStopped = false;
      _deadlineRetryTimer?.cancel();
      _deadlineRetryTimer = null;
    }
    if (widget.match.status != 'pending_accept') {
      _deadlineRetryTimer?.cancel();
      _deadlineRetryTimer = null;
      _deadlineRefreshStopped = true;
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
    if (_deadlineRefreshInFlight ||
        _deadlineRefreshStopped ||
        _deadlineRetryTimer != null ||
        widget.onRefresh == null ||
        !mounted) {
      return;
    }
    _deadlineRefreshInFlight = true;
    try {
      final remainsPending = await _refresh();
      if (!mounted) return;
      if (remainsPending && _deadlineRefreshAttempts < 5) {
        final delay = Duration(
          seconds: 1 << _deadlineRefreshAttempts.clamp(0, 4).toInt(),
        );
        _deadlineRefreshAttempts++;
        _deadlineRetryTimer?.cancel();
        _deadlineRetryTimer = Timer(delay, () {
          _deadlineRetryTimer = null;
          if (mounted) unawaited(_refreshAtDeadline());
        });
      } else {
        _deadlineRefreshStopped = true;
      }
    } catch (_) {
      if (mounted && _deadlineRefreshAttempts < 5) {
        final delay = Duration(
          seconds: 1 << _deadlineRefreshAttempts.clamp(0, 4).toInt(),
        );
        _deadlineRefreshAttempts++;
        _deadlineRetryTimer?.cancel();
        _deadlineRetryTimer = Timer(delay, () {
          _deadlineRetryTimer = null;
          if (mounted) unawaited(_refreshAtDeadline());
        });
      } else {
        _deadlineRefreshStopped = true;
      }
    } finally {
      _deadlineRefreshInFlight = false;
    }
  }

  Future<bool> _refresh() async {
    final refresh = widget.onRefresh;
    if (refresh == null || !mounted) return false;
    final current = _refreshFlight;
    if (current != null) return current;
    late final Future<bool> flight;
    flight = Future<bool>.sync(refresh)
        .then((value) => value, onError: (_) => true)
        .whenComplete(() {
          if (identical(_refreshFlight, flight)) _refreshFlight = null;
        });
    _refreshFlight = flight;
    return flight;
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
      child: FocusScope(
        node: _modalScope,
        autofocus: true,
        child: FocusTraversalGroup(
          child: Focus(
            onKeyEvent: (node, event) {
              if (event is! KeyDownEvent ||
                  event.logicalKey != LogicalKeyboardKey.tab) {
                return KeyEventResult.ignored;
              }
              final primary = FocusManager.instance.primaryFocus;
              if (HardwareKeyboard.instance.isShiftPressed &&
                  identical(primary, _acceptFocusNode)) {
                _declineFocusNode.requestFocus();
                return KeyEventResult.handled;
              }
              if (!HardwareKeyboard.instance.isShiftPressed &&
                  identical(primary, _declineFocusNode)) {
                _acceptFocusNode.requestFocus();
                return KeyEventResult.handled;
              }
              return KeyEventResult.ignored;
            },
            child: Semantics(
              key: MatchFoundOverlay.modalSemanticsKey,
              scopesRoute: true,
              namesRoute: true,
              explicitChildNodes: true,
              liveRegion: true,
              label: l10n.matchFoundTitle,
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
                          style: Theme.of(context).textTheme.bodyMedium
                              ?.copyWith(color: voice.textSecondary),
                        ),
                        const SizedBox(height: 12),
                        Semantics(
                          key: MatchFoundOverlay.timerKey,
                          liveRegion: true,
                          label: l10n.matchFoundSubtitle(
                            gameName,
                            widget.match.mode,
                          ),
                          value: _secondsLeft == null
                              ? null
                              : '${_secondsLeft}s',
                          child: ExcludeSemantics(
                            child: Text(
                              _secondsLeft == null ? '—' : '${_secondsLeft}s',
                              style: Theme.of(context).textTheme.titleSmall
                                  ?.copyWith(color: voice.textSecondary),
                            ),
                          ),
                        ),
                        const SizedBox(height: 16),
                        Row(
                          children: [
                            Expanded(
                              child: FilledButton(
                                key: MatchFoundOverlay.acceptButtonKey,
                                focusNode: _acceptFocusNode,
                                onPressed: _busy || widget.isResponding
                                    ? null
                                    : () => _respond(true),
                                style: ButtonStyle(
                                  side: WidgetStateProperty.resolveWith((
                                    states,
                                  ) {
                                    if (states.contains(WidgetState.focused)) {
                                      return BorderSide(
                                        color: Theme.of(
                                          context,
                                        ).colorScheme.onPrimary,
                                        width: 2,
                                      );
                                    }
                                    return null;
                                  }),
                                ),
                                child: Text(l10n.matchFoundAccept),
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: OutlinedButton(
                                key: MatchFoundOverlay.declineButtonKey,
                                focusNode: _declineFocusNode,
                                onPressed: _busy || widget.isResponding
                                    ? null
                                    : () => _respond(false),
                                style: ButtonStyle(
                                  foregroundColor: WidgetStatePropertyAll(
                                    voice.error,
                                  ),
                                  side: WidgetStateProperty.resolveWith((
                                    states,
                                  ) {
                                    if (states.contains(WidgetState.focused)) {
                                      return BorderSide(
                                        color: Theme.of(
                                          context,
                                        ).colorScheme.primary,
                                        width: 2,
                                      );
                                    }
                                    return BorderSide(color: voice.error);
                                  }),
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
            ),
          ),
        ),
      ),
    );
  }
}
