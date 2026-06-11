import 'package:flutter/material.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';

typedef MatchRespondCallback = Future<RespondToMatchData?> Function(bool accept);

/// High-priority accept/decline popup when a match is found.
class MatchFoundOverlay extends StatefulWidget {
  const MatchFoundOverlay({
    super.key,
    required this.match,
    this.onRespond,
  });

  final MatchData match;
  final MatchRespondCallback? onRespond;

  static const Key acceptButtonKey = Key('match_found_accept');
  static const Key declineButtonKey = Key('match_found_decline');

  @override
  State<MatchFoundOverlay> createState() => _MatchFoundOverlayState();
}

class _MatchFoundOverlayState extends State<MatchFoundOverlay> {
  bool _busy = false;

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
    final radius = context.voiceMetrics.corner('lg', fallback: 8);
    final gameName = widget.match.gameName ?? widget.match.gameId;
    return Positioned.fill(
      child: Material(
        color: voice.canvas.withValues(alpha: 0.88),
        child: Center(
          child: Container(
            width: 360,
            padding: const EdgeInsets.all(24),
            decoration: BoxDecoration(
              color: voice.elevated,
              borderRadius: BorderRadius.circular(radius),
              border: Border.all(color: voice.borderDefault),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.groups_2_outlined, size: 48, color: voice.profileAccent),
                const SizedBox(height: 16),
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
                const SizedBox(height: 20),
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
