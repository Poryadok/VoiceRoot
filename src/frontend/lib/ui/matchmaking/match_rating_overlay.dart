import 'package:flutter/material.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/matchmaking_search_controller.dart';
import '../../theme/voice_colors.dart';

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
  final _modalScope = FocusScopeNode(
    traversalEdgeBehavior: TraversalEdgeBehavior.closedLoop,
  );
  final Map<String, int?> _stars = {};
  final Set<String> _skipped = {};
  // Preserve acknowledged work across a retry of a later vote or ban.
  final Set<String> _rated = {};
  final Set<String> _banDecided = {};
  final Set<String> _pendingBans = {};
  final Set<String> _banned = {};
  bool _submitting = false;
  bool _failed = false;

  @override
  void dispose() {
    _modalScope.dispose();
    super.dispose();
  }

  bool get _canSubmit =>
      _stars.entries.any(
        (entry) => entry.value != null && !_skipped.contains(entry.key),
      ) ||
      widget.teammates.every(
        (teammate) => _skipped.contains(teammate.profileId),
      );

  @override
  void initState() {
    super.initState();
    for (final teammate in widget.teammates) {
      _stars[teammate.profileId] = null;
    }
  }

  Future<void> _submit() async {
    if (_submitting || !_canSubmit) return;
    setState(() {
      _submitting = true;
      _failed = false;
    });
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
      if (mounted) _failed = !succeeded;
    } catch (_) {
      if (mounted) _failed = true;
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
        _banDecided.add(profileId);
      } else {
        _failed = true;
      }
    } catch (_) {
      if (mounted) _failed = true;
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
    final colors = VoiceColors.of(context);
    return BlockSemantics(
      child: Stack(
        children: [
          Positioned.fill(
            child: ModalBarrier(color: colors.overlay, dismissible: false),
          ),
          SafeArea(
            child: LayoutBuilder(
              builder: (context, constraints) {
                final narrow = constraints.maxWidth < 600;
                return Center(
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 24,
                    ),
                    child: FocusScope(
                      autofocus: true,
                      node: _modalScope,
                      child: Semantics(
                        scopesRoute: true,
                        namesRoute: true,
                        label: l10n.matchRatingTitle,
                        explicitChildNodes: true,
                        child: Material(
                          key: const Key('match_rating_dialog'),
                          color: colors.canvas,
                          elevation: 12,
                          borderRadius: BorderRadius.circular(8),
                          clipBehavior: Clip.antiAlias,
                          child: SizedBox(
                            width: narrow ? 358 : 480,
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              crossAxisAlignment: CrossAxisAlignment.stretch,
                              children: [
                                Padding(
                                  padding: const EdgeInsets.fromLTRB(
                                    16,
                                    8,
                                    16,
                                    8,
                                  ),
                                  child: Wrap(
                                    alignment: WrapAlignment.spaceBetween,
                                    crossAxisAlignment:
                                        WrapCrossAlignment.center,
                                    spacing: 8,
                                    children: [
                                      Text(
                                        l10n.matchRatingTitle,
                                        style: Theme.of(
                                          context,
                                        ).textTheme.titleMedium,
                                      ),
                                      TextButton(
                                        autofocus: true,
                                        key:
                                            MatchRatingOverlay.skipAllButtonKey,
                                        onPressed: _submitting
                                            ? null
                                            : widget.onSkipAll ?? widget.onDone,
                                        child: Text(l10n.matchRatingSkipAll),
                                      ),
                                    ],
                                  ),
                                ),
                                Divider(height: 1, color: colors.borderDefault),
                                Flexible(
                                  child: SingleChildScrollView(
                                    child: Column(
                                      children: [
                                        for (final teammate in widget.teammates)
                                          _player(context, teammate, narrow),
                                        if (_failed)
                                          Padding(
                                            padding: const EdgeInsets.all(16),
                                            child: Semantics(
                                              liveRegion: true,
                                              child: Text(
                                                l10n.matchRatingSubmitError,
                                                style: TextStyle(
                                                  color: colors.error,
                                                ),
                                              ),
                                            ),
                                          ),
                                      ],
                                    ),
                                  ),
                                ),
                                Divider(height: 1, color: colors.borderDefault),
                                Padding(
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 16,
                                    vertical: 12,
                                  ),
                                  child: Align(
                                    alignment: Alignment.centerRight,
                                    child: FilledButton(
                                      key: MatchRatingOverlay.submitButtonKey,
                                      onPressed: _submitting || !_canSubmit
                                          ? null
                                          : _submit,
                                      child: _submitting
                                          ? SizedBox(
                                              width: 20,
                                              height: 20,
                                              child: CircularProgressIndicator(
                                                strokeWidth: 2,
                                                semanticsLabel:
                                                    l10n.matchRatingSubmit,
                                              ),
                                            )
                                          : Text(l10n.matchRatingSubmit),
                                    ),
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _player(BuildContext context, RatedTeammate teammate, bool narrow) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final colors = VoiceColors.of(context);
    final id = teammate.profileId;
    final selected = _stars[id] ?? 0;
    final skipped = _skipped.contains(id);
    final disabled = _submitting || _rated.contains(id);
    final stars = Semantics(
      label: '${teammate.displayName}: $selected / 5',
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (var value = 1; value <= 5; value++)
            Semantics(
              selected: selected == value,
              child: IconButton(
                key: widget.teammates.length == 1
                    ? MatchRatingOverlay.starButtonKey(value)
                    : null,
                constraints: const BoxConstraints(minWidth: 40, minHeight: 40),
                padding: const EdgeInsets.all(8),
                tooltip: '$value',
                onPressed: disabled
                    ? null
                    : () => setState(() {
                        _stars[id] = value;
                        _banDecided.remove(id);
                      }),
                icon: Icon(
                  selected >= value ? Icons.star : Icons.star_border,
                  size: 20,
                  color: colors.warning,
                ),
              ),
            ),
        ],
      ),
    );
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              ExcludeSemantics(
                child: CircleAvatar(
                  radius: 20,
                  backgroundColor: colors.profileAccent,
                  foregroundColor: colors.textPrimary,
                  child: Text(
                    teammate.displayName.isEmpty
                        ? '?'
                        : teammate.displayName.characters.first,
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  teammate.displayName,
                  style: theme.textTheme.titleSmall,
                ),
              ),
              if (!narrow && !skipped) stars,
            ],
          ),
          if (skipped)
            Padding(
              padding: const EdgeInsets.only(left: 52, top: 8),
              child: Text(l10n.matchRatingSkipped),
            )
          else ...[
            Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              alignment: WrapAlignment.end,
              children: [
                if (narrow) stars,
                TextButton(
                  key: MatchRatingOverlay.skipTeammateKey(id),
                  onPressed: disabled
                      ? null
                      : () => setState(() => _skipped.add(id)),
                  child: Text(l10n.matchRatingSkipTeammate),
                ),
              ],
            ),
            if (selected > 0 &&
                selected <= 2 &&
                !_banDecided.contains(id) &&
                !_submitting)
              Container(
                key: Key('match_rating_suggestion_$id'),
                margin: const EdgeInsets.only(top: 8),
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: colors.surface,
                  border: Border(
                    left: BorderSide(color: colors.warning, width: 3),
                  ),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      l10n.matchRatingBanMessage(teammate.displayName),
                      style: theme.textTheme.bodyMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Wrap(
                      alignment: WrapAlignment.end,
                      spacing: 8,
                      children: [
                        TextButton(
                          onPressed: () => setState(() => _banDecided.add(id)),
                          child: Text(l10n.matchRatingBanCancel),
                        ),
                        OutlinedButton(
                          key: MatchRatingOverlay.banButtonKey(id),
                          onPressed: () => _ban(id),
                          child: Text(l10n.matchRatingBanConfirm),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
          ],
        ],
      ),
    );
  }
}
