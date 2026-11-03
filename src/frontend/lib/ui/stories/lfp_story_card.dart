import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import 'story_game_tag_chip.dart';

/// Looking-for-party story card with join/write actions.
class LfpStoryCard extends StatefulWidget {
  const LfpStoryCard({
    super.key,
    required this.story,
    this.onJoin,
    this.onInvite,
    this.onWrite,
    this.isAuthor = false,
    this.embedded = false,
    this.now = DateTime.now,
  });

  static const Key cardKey = Key('lfp_story_card');
  static const Key joinKey = Key('lfp_story_join');
  static const Key inviteKey = Key('lfp_story_invite');
  static const Key writeKey = Key('lfp_story_write');

  final StoryData story;
  final VoidCallback? onJoin;
  final VoidCallback? onInvite;
  final VoidCallback? onWrite;
  final bool isAuthor;
  final bool embedded;
  final DateTime Function() now;

  @override
  State<LfpStoryCard> createState() => _LfpStoryCardState();
}

class _LfpStoryCardState extends State<LfpStoryCard> {
  Timer? _expiryTimer;
  StoryData get story => widget.story;
  bool get embedded => widget.embedded;
  bool get isAuthor => widget.isAuthor;
  bool get _expired =>
      story.expiresAt != null &&
      !story.expiresAt!.isAfter(widget.now().toUtc());

  @override
  void initState() {
    super.initState();
    _scheduleExpiry();
  }

  @override
  void didUpdateWidget(LfpStoryCard oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.story.expiresAt != story.expiresAt ||
        oldWidget.now != widget.now) {
      _scheduleExpiry();
    }
  }

  void _scheduleExpiry() {
    _expiryTimer?.cancel();
    final expiresAt = story.expiresAt;
    if (expiresAt == null || _expired) return;
    _expiryTimer = Timer(expiresAt.difference(widget.now().toUtc()), () {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _expiryTimer?.cancel();
    super.dispose();
  }

  void _respond(bool invite) {
    if (_expired || isAuthor) {
      setState(() {});
      return;
    }
    final action = invite ? widget.onInvite : widget.onJoin;
    if (action != null) {
      action();
      return;
    }
    if (invite) {
      LfpStoryActions.invite(context, story);
    } else {
      LfpStoryActions.join(context, story);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final baseVoice = VoiceColors.of(context);
    final voice = embedded
        ? baseVoice.copyWith(
            textPrimary: Colors.white,
            textSecondary: Colors.white,
          )
        : baseVoice;
    final criteria = _parseCriteria(story.lfpCriteriaJson);
    final expired = _expired;

    return Card(
      key: LfpStoryCard.cardKey,
      color: embedded ? Colors.transparent : voice.elevated,
      elevation: embedded ? 0 : null,
      margin: embedded ? EdgeInsets.zero : null,
      child: Padding(
        padding: embedded ? EdgeInsets.zero : const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (!embedded)
              Row(
                children: [
                  Icon(Icons.groups_outlined, color: voice.profileAccent),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      l10n.storyLfpTitle,
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        color: voice.textPrimary,
                      ),
                    ),
                  ),
                ],
              ),
            if (story.gameTag != null && story.gameTag!.isNotEmpty) ...[
              const SizedBox(height: 8),
              if (embedded)
                StoryGameTagChip(gameTag: story.gameTag!)
              else
                _InfoRow(
                  label: l10n.storyLfpGame,
                  value: story.gameTag!,
                  color: voice.textPrimary,
                ),
            ],
            if (!embedded && criteria.isNotEmpty) ...[
              const SizedBox(height: 8),
              ...criteria.entries.map(
                (e) => _InfoRow(
                  label: e.key,
                  value: '${e.value}',
                  color: voice.textPrimary,
                ),
              ),
            ],
            if (embedded)
              Wrap(
                spacing: 8,
                children: [
                  for (final key in ['mode', 'region'])
                    if (criteria[key] case final String value)
                      Text(value, style: TextStyle(color: voice.textSecondary)),
                ],
              ),
            if (story.textContent != null && story.textContent!.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(
                story.textContent!,
                style: embedded
                    ? Theme.of(context).textTheme.headlineSmall?.copyWith(
                        color: voice.textPrimary,
                        fontSize: 20,
                        height: 1.4,
                        fontWeight: FontWeight.w600,
                      )
                    : TextStyle(color: voice.textSecondary),
              ),
            ],
            SizedBox(height: embedded ? 0 : 16),
            if (expired) ...[
              Text(
                l10n.storyLfpInactive,
                style: TextStyle(color: voice.textSecondary),
              ),
              const SizedBox(height: 8),
            ],
            if (!isAuthor)
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  FilledButton(
                    key: LfpStoryCard.joinKey,
                    onPressed: expired ? null : () => _respond(false),
                    child: Text(l10n.storyLfpJoin),
                  ),
                  FilledButton.tonal(
                    key: LfpStoryCard.inviteKey,
                    style: embedded
                        ? FilledButton.styleFrom(
                            backgroundColor: Colors.white,
                            foregroundColor: Colors.black87,
                          )
                        : null,
                    onPressed: expired ? null : () => _respond(true),
                    child: Text(l10n.storyLfpInvite),
                  ),
                  OutlinedButton(
                    key: LfpStoryCard.writeKey,
                    style: embedded
                        ? OutlinedButton.styleFrom(
                            foregroundColor: Colors.white,
                            side: const BorderSide(color: Colors.white),
                          )
                        : null,
                    onPressed:
                        widget.onWrite ??
                        () => LfpStoryActions.write(context, story),
                    child: Text(l10n.storyLfpWrite),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Map<String, Object?> _parseCriteria(String? raw) {
    if (raw == null || raw.isEmpty) return const {};
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map<String, dynamic>) return decoded;
    } catch (_) {}
    return const {};
  }
}

/// Existing Story discovery requests and private DM navigation.
abstract final class LfpStoryActions {
  static Future<void> join(BuildContext context, StoryData story) async {
    ProviderContainer? container;
    try {
      container = ProviderScope.containerOf(context, listen: false);
    } catch (_) {
      return;
    }
    final ref = _RefReader(container);
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null || story.id.isEmpty) return;

    final result = await ref
        .read(voiceStoriesClientProvider)
        .respondToLfpStory(
          authorization: auth,
          storyId: story.id,
          responseType: 'JOIN',
        );
    if (!context.mounted) return;
    if (result is StoriesApiFailure) {
      final l10n = AppLocalizations.of(context)!;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            commonActionErrorMessage(l10n, statusCode: result.statusCode),
          ),
        ),
      );
      return;
    }
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(AppLocalizations.of(context)!.storyLfpJoinSent)),
    );
  }

  static Future<void> invite(BuildContext context, StoryData story) async {
    ProviderContainer? container;
    try {
      container = ProviderScope.containerOf(context, listen: false);
    } catch (_) {
      return;
    }
    final ref = _RefReader(container);
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null || story.id.isEmpty) return;

    final result = await ref
        .read(voiceStoriesClientProvider)
        .respondToLfpStory(
          authorization: auth,
          storyId: story.id,
          responseType: 'INVITE',
        );
    if (!context.mounted) return;
    if (result is StoriesApiFailure) {
      final l10n = AppLocalizations.of(context)!;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            commonActionErrorMessage(l10n, statusCode: result.statusCode),
          ),
        ),
      );
      return;
    }
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(AppLocalizations.of(context)!.storyLfpInviteSent)),
    );
  }

  static Future<void> write(BuildContext context, StoryData story) async {
    ProviderContainer? container;
    try {
      container = ProviderScope.containerOf(context, listen: false);
    } catch (_) {
      return;
    }
    final ref = _RefReader(container);
    final authorId = story.authorProfileId;
    if (authorId.isEmpty) return;
    final err = await ref.read(chatActionsProvider).openDmWithProfile(authorId);
    if (!context.mounted) return;
    if (err != null) {
      final l10n = AppLocalizations.of(context)!;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(commonActionErrorMessage(l10n))));
    }
  }
}

/// Minimal ref-like reader for static navigation helpers.
class _RefReader {
  _RefReader(this._container);

  final ProviderContainer _container;

  T read<T>(ProviderListenable<T> provider) => _container.read(provider);
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({
    required this.label,
    required this.value,
    required this.color,
  });

  final String label;
  final String value;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '$label: ',
            style: TextStyle(color: color, fontWeight: FontWeight.w600),
          ),
          Expanded(
            child: Text(value, style: TextStyle(color: color)),
          ),
        ],
      ),
    );
  }
}
