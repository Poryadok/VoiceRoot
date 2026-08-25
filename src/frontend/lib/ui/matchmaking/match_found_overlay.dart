import 'dart:async';

import 'package:flutter/material.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';
import '../call/call_modal_overlay.dart';

typedef MatchRespondCallback = Future<RespondToMatchData?> Function(bool accept);

/// High-priority accept/decline popup when a match is found (Penpot §6.4 · v2).
class MatchFoundOverlay extends StatefulWidget {
  const MatchFoundOverlay({
    super.key,
    required this.match,
    this.onRespond,
    this.acceptTimeoutSeconds = 30,
  });

  final MatchData match;
  final MatchRespondCallback? onRespond;
  final int acceptTimeoutSeconds;

  static const Key acceptButtonKey = Key('match_found_accept');
  static const Key declineButtonKey = Key('match_found_decline');
  static const Key timerKey = Key('match_found_timer');

  @override
  State<MatchFoundOverlay> createState() => _MatchFoundOverlayState();
}

class _MatchFoundOverlayState extends State<MatchFoundOverlay> {
  bool _busy = false;
  late int _secondsLeft;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _secondsLeft = widget.acceptTimeoutSeconds;
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      if (_secondsLeft <= 0) return;
      setState(() => _secondsLeft -= 1);
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
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
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: voice.textSecondary,
                      ),
                ),
                const SizedBox(height: 12),
                Text(
                  '${_secondsLeft}s',
                  key: MatchFoundOverlay.timerKey,
                  style: Theme.of(context).textTheme.titleSmall?.copyWith(
                        color: voice.textSecondary,
                      ),
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
