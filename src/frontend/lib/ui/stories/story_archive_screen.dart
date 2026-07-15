import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';

/// Owner archive of expired stories with add-to-highlight action.
class StoryArchiveScreen extends ConsumerWidget {
  const StoryArchiveScreen({super.key});

  static const Key screenKey = Key('story_archive_screen');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final archiveAsync = ref.watch(storyArchiveProvider);

    return Scaffold(
      key: screenKey,
      appBar: AppBar(title: Text(l10n.storyArchiveTitle)),
      body: archiveAsync.when(
        loading: () => const VoiceListSkeleton(),
        error: (error, _) => VoiceStatePanel(
          title: l10n.storyArchiveLoadError,
          message: storyArchiveErrorMessage(l10n, error),
          icon: Icons.cloud_off_outlined,
          actionLabel: l10n.commonRetry,
          onAction: () => ref.invalidate(storyArchiveProvider),
        ),
        data: (stories) {
          if (stories.isEmpty) {
            return VoiceStatePanel(
              title: l10n.storyArchiveEmpty,
              icon: Icons.inventory_2_outlined,
            );
          }
          return ListView.separated(
            padding: const EdgeInsets.all(16),
            itemCount: stories.length,
            separatorBuilder: (_, _) => const SizedBox(height: 8),
            itemBuilder: (context, index) {
              final story = stories[index];
              return _ArchiveStoryTile(story: story);
            },
          );
        },
      ),
    );
  }
}

class _ArchiveStoryTile extends ConsumerWidget {
  const _ArchiveStoryTile({required this.story});

  final StoryData story;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final preview = story.textContent?.trim();
    final label = preview != null && preview.isNotEmpty
        ? preview
        : switch (story.type) {
            'photo' => l10n.storyCreateTypePhoto,
            'video' => l10n.storyCreateTypeVideo,
            _ => l10n.storyCreateTypeText,
          };

    return Card(
      color: voice.elevated,
      child: ListTile(
        key: Key('story_archive_tile_${story.id}'),
        title: Text(
          label,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(color: voice.textPrimary),
        ),
        subtitle: Text(
          story.type,
          style: TextStyle(color: voice.textSecondary),
        ),
        trailing: TextButton(
          key: Key('story_archive_add_highlight_${story.id}'),
          onPressed: () => _pickHighlight(context, ref, story.id),
          child: Text(l10n.storyArchiveAddToHighlight),
        ),
      ),
    );
  }

  Future<void> _pickHighlight(
    BuildContext context,
    WidgetRef ref,
    String storyId,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (profileId == null) return;

    final highlights =
        await ref.read(profileHighlightsProvider(profileId).future);
    if (!context.mounted) return;

    if (highlights.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.storyHighlightSelectHighlight)),
      );
      return;
    }

    final highlightId = await showVoiceBottomSheet<String>(
      context: context,
      initialSize: 0.45,
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.all(16),
              child: Text(
                l10n.storyHighlightSelectHighlight,
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            Expanded(
              child: ListView.builder(
                itemCount: highlights.length,
                itemBuilder: (ctx, index) {
                  final highlight = highlights[index];
                  return ListTile(
                    title: Text(highlight.name),
                    onTap: () => Navigator.of(ctx).pop(highlight.id),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );

    if (highlightId == null || !context.mounted) return;

    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;

    final result = await ref.read(voiceStoriesClientProvider).addToHighlight(
          authorization: auth,
          highlightId: highlightId,
          storyId: storyId,
        );

    if (!context.mounted) return;
    switch (result) {
      case StoriesApiOk():
        ref.invalidate(profileHighlightsProvider(profileId));
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.storyHighlightSaved)),
        );
      case StoriesApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
    }
  }
}
