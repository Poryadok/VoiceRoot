import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../../routing/stories_routes.dart';
import '../core/voice_skeleton.dart';

/// Profile highlights row (stories (docs/features/stories.md)).
class HighlightsSection extends ConsumerWidget {
  const HighlightsSection({
    super.key,
    required this.profileId,
    this.onHighlightTap,
  });

  static const Key sectionKey = Key('highlights_section');

  final String profileId;
  final void Function(String highlightId, List<String> storyIds)? onHighlightTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final highlightsAsync = ref.watch(profileHighlightsProvider(profileId));

    return highlightsAsync.when(
      loading: () => const SizedBox(
        height: 88,
        child: VoiceListSkeleton(rowCount: 1),
      ),
      error: (_, _) => const SizedBox.shrink(),
      data: (highlights) {
        if (highlights.isEmpty) return const SizedBox.shrink();
        return Column(
          key: HighlightsSection.sectionKey,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
              child: Text(
                l10n.storyHighlightsTitle,
                style: Theme.of(context).textTheme.titleSmall,
              ),
            ),
            SizedBox(
              height: 104,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 16),
                itemCount: highlights.length,
                separatorBuilder: (_, _) => const SizedBox(width: 12),
                itemBuilder: (context, index) {
                  final highlight = highlights[index];
                  return _HighlightChip(
                    name: highlight.name,
                    visibility: highlight.visibility,
                    onTap: () {
                      if (onHighlightTap != null) {
                        onHighlightTap!(
                          highlight.id,
                          highlight.storyIds,
                        );
                        return;
                      }
                      if (highlight.storyIds.isEmpty) return;
                      StoriesRoutes.openViewer(
                        context,
                        storyIds: highlight.storyIds,
                        profileId: profileId,
                      );
                    },
                  );
                },
              ),
            ),
            Divider(color: voice.borderDefault, height: 1),
          ],
        );
      },
    );
  }
}

class _HighlightChip extends StatelessWidget {
  const _HighlightChip({
    required this.name,
    required this.visibility,
    required this.onTap,
  });

  final String name;
  final String visibility;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: SizedBox(
        width: 72,
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(color: voice.borderStrong, width: 2),
                color: voice.muted,
              ),
              child: Icon(Icons.bookmark, color: voice.textSecondary),
            ),
            const SizedBox(height: 4),
            Text(
              name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.labelSmall,
            ),
            const SizedBox(height: 2),
            Text(
              key: const Key('highlight_visibility_badge'),
              l10n.storyHighlightVisibility(_visibilityLabel(l10n, visibility)),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: voice.textSecondary,
                    fontSize: 10,
                  ),
            ),
          ],
        ),
      ),
    );
  }

  static String _visibilityLabel(AppLocalizations l10n, String value) {
    return switch (value) {
      'friends' => l10n.storyVisibilityFriends,
      'close_friends' => l10n.storyVisibilityCloseFriends,
      'everyone' || 'public' => l10n.storyVisibilityEveryone,
      _ => l10n.storyVisibilityEveryone,
    };
  }
}
