import 'package:flutter/material.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/matchmaking_search_controller.dart';

class RatedTeammate {
  const RatedTeammate({required this.profileId, required this.displayName});

  final String profileId;
  final String displayName;
}

/// Explains why matchmaking stopped or changed and offers a recovery action.
class MatchmakingRecoveryCard extends StatelessWidget {
  const MatchmakingRecoveryCard({
    super.key,
    required this.reason,
    required this.onAction,
  });

  static const Key cardKey = Key('queue_search_recovery');
  static const Key actionButtonKey = Key('queue_search_recovery_action');
  static const Key timeoutStateKey = Key('queue_search_timeout');
  static const Key declinedStateKey = Key('queue_search_declined');

  final SearchRecoveryReason reason;
  final VoidCallback onAction;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final (title, body, actionLabel) = switch (reason) {
      SearchRecoveryReason.timeout => (
        l10n.queueSearchTimeoutTitle,
        l10n.queueSearchTimeoutBody,
        l10n.queueSearchRecoveryReturnToQueue,
      ),
      SearchRecoveryReason.declined => (
        l10n.queueSearchDeclinedTitle,
        l10n.queueSearchDeclinedBody,
        l10n.queueSearchRecoveryContinueSearch,
      ),
    };

    return Card(
      key: reason == SearchRecoveryReason.timeout
          ? MatchmakingRecoveryCard.timeoutStateKey
          : MatchmakingRecoveryCard.declinedStateKey,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(title, style: theme.textTheme.titleSmall),
            const SizedBox(height: 8),
            Text(body),
            const SizedBox(height: 12),
            FilledButton(
              key: MatchmakingRecoveryCard.actionButtonKey,
              onPressed: onAction,
              child: Text(actionLabel),
            ),
          ],
        ),
      ),
    );
  }
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
  final Future<bool> Function(String profileId, int stars)? onRate;
  final Future<bool> Function(String profileId)? onBan;
  final VoidCallback? onSkipAll;
  final VoidCallback? onDone;

  static Key starButtonKey(int stars) => Key('match_rating_star_$stars');
  static const Key submitButtonKey = Key('match_rating_submit');
  static Key skipTeammateKey(String profileId) =>
      Key('match_rating_skip_$profileId');
  static const Key skipAllButtonKey = Key('match_rating_skip_all');
  static Key banButtonKey(String profileId) =>
      Key('match_rating_ban_$profileId');

  @override
  State<MatchRatingOverlay> createState() => _MatchRatingOverlayState();
}

class _MatchRatingOverlayState extends State<MatchRatingOverlay> {
  final Map<String, int?> _stars = {};
  final Set<String> _skipped = {};
  // Preserve acknowledged work across a retry of a later vote or ban.
  final Set<String> _rated = {};
  final Set<String> _banDecided = {};
  final Set<String> _pendingBans = {};
  final Set<String> _banned = {};
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
      var succeeded = true;
      for (final teammate in widget.teammates) {
        final id = teammate.profileId;
        if (_skipped.contains(id)) continue;
        final stars = _stars[id];
        if (stars == null) continue;
        if (!_rated.contains(id)) {
          final rated = await widget.onRate?.call(id, stars) ?? true;
          if (!mounted) return;
          if (!rated) {
            succeeded = false;
            continue;
          }
          _rated.add(id);
        }
        if (stars <= 2 && !_banDecided.contains(id) && !_banned.contains(id)) {
          final ban = await _confirmBan(teammate);
          if (!mounted) return;
          _banDecided.add(id);
          if (ban) _pendingBans.add(id);
        }
        if (_pendingBans.contains(id)) {
          final banned = await widget.onBan?.call(id) ?? true;
          if (!mounted) return;
          if (banned) {
            _pendingBans.remove(id);
            _banned.add(id);
          } else {
            succeeded = false;
          }
        }
      }
      if (succeeded) widget.onDone?.call();
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _ban(String profileId) async {
    if (_submitting || _banned.contains(profileId)) return;
    setState(() => _submitting = true);
    try {
      final banned = await widget.onBan?.call(profileId) ?? true;
      if (!mounted) return;
      if (banned) {
        _banned.add(profileId);
        _pendingBans.remove(profileId);
      }
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
              Text(l10n.matchRatingTitle, style: theme.textTheme.titleLarge),
              const SizedBox(height: 8),
              Text(l10n.matchRatingSubtitle, style: theme.textTheme.bodyMedium),
              const SizedBox(height: 16),
              Expanded(
                child: ListView.separated(
                  itemCount: widget.teammates.length,
                  separatorBuilder: (context, index) =>
                      const Divider(height: 24),
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
                                  onPressed:
                                      _submitting ||
                                          _rated.contains(teammate.profileId)
                                      ? null
                                      : () {
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
                            onPressed:
                                _submitting ||
                                    _rated.contains(teammate.profileId)
                                ? null
                                : () {
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
                            onPressed:
                                _submitting ||
                                    _banned.contains(teammate.profileId)
                                ? null
                                : () => _ban(teammate.profileId),
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
                onPressed: _submitting
                    ? null
                    : widget.onSkipAll ?? widget.onDone,
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
