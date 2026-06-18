import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_state_panel.dart';
import 'highlight_edit_sheet.dart';

/// Owner highlights management (create / edit / delete).
class StoryHighlightsScreen extends ConsumerWidget {
  const StoryHighlightsScreen({super.key, required this.profileId});

  static const Key screenKey = Key('story_highlights_screen');

  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final highlightsAsync = ref.watch(profileHighlightsProvider(profileId));

    return Scaffold(
      key: screenKey,
      appBar: AppBar(title: Text(l10n.storyHighlightsManageTitle)),
      floatingActionButton: FloatingActionButton.extended(
        key: const Key('story_highlight_create_fab'),
        onPressed: () => HighlightEditSheet.showCreate(context),
        icon: const Icon(Icons.add),
        label: Text(l10n.storyHighlightCreate),
      ),
      body: highlightsAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (_, _) => VoiceStatePanel(
          title: l10n.storyArchiveLoadError,
          icon: Icons.cloud_off_outlined,
        ),
        data: (highlights) {
          if (highlights.isEmpty) {
            return VoiceStatePanel(
              title: l10n.storyHighlightsEmpty,
              icon: Icons.bookmark_border,
            );
          }
          return ListView.separated(
            padding: const EdgeInsets.all(16),
            itemCount: highlights.length,
            separatorBuilder: (_, _) => const SizedBox(height: 8),
            itemBuilder: (context, index) {
              final highlight = highlights[index];
              return _HighlightTile(
                highlight: highlight,
                profileId: profileId,
              );
            },
          );
        },
      ),
    );
  }
}

class _HighlightTile extends ConsumerWidget {
  const _HighlightTile({
    required this.highlight,
    required this.profileId,
  });

  final HighlightData highlight;
  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return Card(
      color: voice.elevated,
      child: ListTile(
        key: Key('story_highlight_tile_${highlight.id}'),
        leading: Icon(Icons.bookmark, color: voice.profileAccent),
        title: Text(
          highlight.name,
          style: TextStyle(color: voice.textPrimary),
        ),
        subtitle: Text(
          l10n.storyHighlightStoryCount(highlight.storyIds.length),
          style: TextStyle(color: voice.textSecondary),
        ),
        onTap: () => HighlightEditSheet.showEdit(
          context,
          highlight: highlight,
        ),
        trailing: IconButton(
          key: Key('story_highlight_delete_${highlight.id}'),
          tooltip: l10n.storyHighlightDelete,
          icon: Icon(Icons.delete_outline, color: voice.error),
          onPressed: () => _confirmDelete(context, ref),
        ),
      ),
    );
  }

  Future<void> _confirmDelete(BuildContext context, WidgetRef ref) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.storyHighlightDelete),
        content: Text(l10n.storyHighlightDeleteConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.storyHighlightDelete),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;

    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;

    final result = await ref.read(voiceStoriesClientProvider).deleteHighlight(
          authorization: auth,
          highlightId: highlight.id,
        );

    if (!context.mounted) return;
    switch (result) {
      case StoriesApiOk():
        ref.invalidate(profileHighlightsProvider(profileId));
      case StoriesApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
    }
  }
}
