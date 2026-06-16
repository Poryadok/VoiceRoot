import 'dart:convert';

import 'package:flutter/material.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

/// Looking-for-party story card (display only — no join/invite actions).
class LfpStoryCard extends StatelessWidget {
  const LfpStoryCard({super.key, required this.story});

  static const Key cardKey = Key('lfp_story_card');

  final StoryData story;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final criteria = _parseCriteria(story.lfpCriteriaJson);

    return Card(
      key: LfpStoryCard.cardKey,
      color: voice.elevated,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
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
              _InfoRow(
                label: l10n.storyLfpGame,
                value: story.gameTag!,
              ),
            ],
            if (criteria.isNotEmpty) ...[
              const SizedBox(height: 8),
              ...criteria.entries.map(
                (e) => _InfoRow(label: e.key, value: '${e.value}'),
              ),
            ],
            if (story.textContent != null && story.textContent!.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(
                story.textContent!,
                style: TextStyle(color: voice.textSecondary),
              ),
            ],
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

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '$label: ',
            style: TextStyle(
              color: voice.textSecondary,
              fontWeight: FontWeight.w600,
            ),
          ),
          Expanded(
            child: Text(value, style: TextStyle(color: voice.textPrimary)),
          ),
        ],
      ),
    );
  }
}
