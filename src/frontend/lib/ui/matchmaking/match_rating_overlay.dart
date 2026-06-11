import 'package:flutter/material.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';

class RatedTeammate {
  const RatedTeammate({
    required this.profileId,
    required this.displayName,
  });

  final String profileId;
  final String displayName;
}

/// Post-match teammate rating sheet (1–5 stars per participant).
class MatchRatingOverlay extends StatefulWidget {
  const MatchRatingOverlay({
    super.key,
    required this.match,
    required this.raterProfileId,
    required this.teammates,
    this.onRate,
    this.onBan,
    this.onSkipAll,
    this.onDone,
  });

  final MatchData match;
  final String raterProfileId;
  final List<RatedTeammate> teammates;
  final Future<void> Function(String profileId, int stars)? onRate;
  final Future<void> Function(String profileId)? onBan;
  final VoidCallback? onSkipAll;
  final VoidCallback? onDone;

  static Key starButtonKey(int stars) => Key('match_rating_star_$stars');
  static const Key submitButtonKey = Key('match_rating_submit');
  static Key skipTeammateKey(String profileId) =>
      Key('match_rating_skip_$profileId');
  static const Key skipAllButtonKey = Key('match_rating_skip_all');
  static Key banButtonKey(String profileId) => Key('match_rating_ban_$profileId');

  @override
  State<MatchRatingOverlay> createState() => _MatchRatingOverlayState();
}

class _MatchRatingOverlayState extends State<MatchRatingOverlay> {
  final Map<String, int?> _stars = {};
  final Set<String> _skipped = {};
  bool _submitting = false;

  @override
  void initState() {
    super.initState();
    for (final teammate in widget.teammates) {
      _stars[teammate.profileId] = null;
    }
  }

  Future<void> _submit() async {
    if (_submitting) return;
    setState(() => _submitting = true);
    try {
      for (final teammate in widget.teammates) {
        if (_skipped.contains(teammate.profileId)) continue;
        final stars = _stars[teammate.profileId];
        if (stars == null) continue;
        await widget.onRate?.call(teammate.profileId, stars);
        if (stars <= 2) {
          final ban = await _confirmBan(teammate);
          if (ban) {
            await widget.onBan?.call(teammate.profileId);
          }
        }
      }
      widget.onDone?.call();
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<bool> _confirmBan(RatedTeammate teammate) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.matchRatingBanTitle),
        content: Text(l10n.matchRatingBanMessage(teammate.displayName)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.matchRatingBanCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.matchRatingBanConfirm),
          ),
        ],
      ),
    );
    return confirmed ?? false;
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);

    return Material(
      color: theme.colorScheme.surface.withValues(alpha: 0.98),
      child: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.matchRatingTitle,
                style: theme.textTheme.titleLarge,
              ),
              const SizedBox(height: 8),
              Text(
                l10n.matchRatingSubtitle,
                style: theme.textTheme.bodyMedium,
              ),
              const SizedBox(height: 16),
              Expanded(
                child: ListView.separated(
                  itemCount: widget.teammates.length,
                  separatorBuilder: (context, index) => const Divider(height: 24),
                  itemBuilder: (context, index) {
                    final teammate = widget.teammates[index];
                    final skipped = _skipped.contains(teammate.profileId);
                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          teammate.displayName,
                          style: theme.textTheme.titleSmall,
                        ),
                        const SizedBox(height: 8),
                        if (skipped)
                          Text(l10n.matchRatingSkipped)
                        else ...[
                          Row(
                            children: [
                              for (var stars = 1; stars <= 5; stars++)
                                IconButton(
                                  key: widget.teammates.length == 1
                                      ? MatchRatingOverlay.starButtonKey(stars)
                                      : null,
                                  onPressed: () {
                                    setState(() {
                                      _stars[teammate.profileId] = stars;
                                    });
                                  },
                                  icon: Icon(
                                    (_stars[teammate.profileId] ?? 0) >= stars
                                        ? Icons.star
                                        : Icons.star_border,
                                  ),
                                  tooltip: '$stars',
                                ),
                            ],
                          ),
                          TextButton(
                            key: MatchRatingOverlay.skipTeammateKey(
                              teammate.profileId,
                            ),
                            onPressed: () {
                              setState(() {
                                _skipped.add(teammate.profileId);
                              });
                            },
                            child: Text(l10n.matchRatingSkipTeammate),
                          ),
                          TextButton(
                            key: MatchRatingOverlay.banButtonKey(
                              teammate.profileId,
                            ),
                            onPressed: () =>
                                widget.onBan?.call(teammate.profileId),
                            child: Text(l10n.matchRatingBanAction),
                          ),
                        ],
                      ],
                    );
                  },
                ),
              ),
              TextButton(
                key: MatchRatingOverlay.skipAllButtonKey,
                onPressed: widget.onSkipAll ?? widget.onDone,
                child: Text(l10n.matchRatingSkipAll),
              ),
              const SizedBox(height: 8),
              FilledButton(
                key: MatchRatingOverlay.submitButtonKey,
                onPressed: _submitting ? null : _submit,
                child: _submitting
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(l10n.matchRatingSubmit),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
