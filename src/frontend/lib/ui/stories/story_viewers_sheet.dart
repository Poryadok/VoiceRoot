import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/social_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';

/// Bottom sheet listing story viewers (author-only).
class StoryViewersSheet extends ConsumerWidget {
  const StoryViewersSheet({
    super.key,
    required this.storyId,
  });

  static const Key sheetKey = Key('story_viewers_sheet');

  static Future<void> show(BuildContext context, {required String storyId}) {
    return showVoiceBottomSheet<void>(
      context: context,
      initialSize: 0.5,
      minSize: 0.35,
      maxSize: 0.85,
      child: StoryViewersSheet(storyId: storyId),
    );
  }

  final String storyId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final viewersAsync = ref.watch(storyViewersProvider(storyId));

    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              l10n.storyViewersTitle,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            viewersAsync.when(
              loading: () => const SizedBox(
                height: 240,
                child: VoiceListSkeleton(rowCount: 4),
              ),
              error: (error, _) => VoiceStatePanel(
                title: l10n.storyViewersLoadError,
                message: storyViewersErrorMessage(l10n, error),
                icon: Icons.cloud_off_outlined,
                actionLabel: l10n.commonRetry,
                onAction: () => ref.invalidate(storyViewersProvider(storyId)),
              ),
              data: (viewerIds) {
                if (viewerIds.isEmpty) {
                  return VoiceStatePanel(
                    title: l10n.storyViewersEmpty,
                    icon: Icons.visibility_off_outlined,
                  );
                }
                return ListView.separated(
                  shrinkWrap: true,
                  itemCount: viewerIds.length,
                  separatorBuilder: (_, _) =>
                      Divider(color: voice.borderDefault, height: 1),
                  itemBuilder: (context, index) {
                    final profileId = viewerIds[index];
                    return _ViewerRow(profileId: profileId);
                  },
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _ViewerRow extends ConsumerWidget {
  const _ViewerRow({required this.profileId});

  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    final profileAsync = ref.watch(profileProvider(profileId));

    return profileAsync.when(
      loading: () => const ListTile(
        leading: CircleAvatar(child: SizedBox.shrink()),
        title: Text('…'),
      ),
      error: (_, _) => ListTile(
        title: Text(profileId, style: TextStyle(color: voice.textSecondary)),
      ),
      data: (profile) {
        final name = profile?.displayName ?? profileId;
        final handle = profile?.handle;
        return ListTile(
          key: Key('story_viewer_$profileId'),
          leading: CircleAvatar(
            backgroundColor: voice.muted,
            child: Text(
              name.isNotEmpty ? name[0].toUpperCase() : '?',
              style: TextStyle(color: voice.textPrimary),
            ),
          ),
          title: Text(name, style: TextStyle(color: voice.textPrimary)),
          subtitle: handle != null
              ? Text(handle, style: TextStyle(color: voice.textSecondary))
              : null,
        );
      },
    );
  }
}
