import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../routing/stories_routes.dart';
import '../../state/auth_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import 'highlight_edit_sheet.dart';
import 'story_text_style.dart';

/// The owner's expired stories; retention remains server-owned.
class StoryArchiveScreen extends ConsumerWidget {
  const StoryArchiveScreen({super.key});
  static const Key screenKey = Key('story_archive_screen');
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final archive = ref.watch(storyArchiveProvider);
    return Scaffold(
      key: screenKey,
      backgroundColor: voice.canvas,
      appBar: AppBar(title: Text(l10n.storyArchiveTitle)),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 12),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    l10n.storyArchivePrivate,
                    style: TextStyle(color: voice.textSecondary, fontSize: 12),
                  ),
                ),
                const SizedBox(width: 12),
                FilledButton(
                  key: const Key('story_archive_create'),
                  onPressed: () => StoriesRoutes.openCreate(context),
                  child: Text(l10n.storyCreateTitle),
                ),
              ],
            ),
          ),
          Expanded(
            child: archive.when(
              loading: () => const VoiceListSkeleton(),
              error: (error, _) => VoiceStatePanel(
                title: l10n.storyArchiveLoadError,
                message: storyArchiveErrorMessage(l10n, error),
                icon: Icons.cloud_off_outlined,
                actionLabel: l10n.commonRetry,
                onAction: () => ref.invalidate(storyArchiveProvider),
              ),
              data: (stories) => stories.isEmpty
                  ? VoiceStatePanel(
                      title: l10n.storyArchiveEmpty,
                      icon: Icons.inventory_2_outlined,
                    )
                  : LayoutBuilder(
                      builder: (context, constraints) {
                        final mobile = constraints.maxWidth < 600;
                        return GridView.builder(
                          padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                          gridDelegate:
                              SliverGridDelegateWithFixedCrossAxisCount(
                                crossAxisCount: mobile ? 2 : 3,
                                crossAxisSpacing: 10,
                                mainAxisSpacing: 10,
                                mainAxisExtent:
                                    (mobile ? 180 : 150) *
                                    math.max(
                                      1,
                                      MediaQuery.textScalerOf(context).scale(1),
                                    ),
                              ),
                          itemCount: stories.length,
                          itemBuilder: (_, index) => _ArchiveStoryTile(
                            story: stories[index],
                            onOpen: () => StoriesRoutes.openViewer(
                              context,
                              storyIds: stories
                                  .map((story) => story.id)
                                  .toList(),
                              initialIndex: index,
                            ),
                          ),
                        );
                      },
                    ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ArchiveStoryTile extends StatelessWidget {
  const _ArchiveStoryTile({required this.story, required this.onOpen});
  final StoryData story;
  final VoidCallback onOpen;
  @override
  Widget build(BuildContext context) {
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
    final background = storyTextBackgroundColor(voice, story.textStyleJson);
    // Preview backgrounds can be light even in the dark theme.
    final foreground = background.computeLuminance() > 0.179
        ? Colors.black
        : Colors.white;
    return Material(
      color: background,
      borderRadius: BorderRadius.circular(8),
      clipBehavior: Clip.antiAlias,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: InkWell(
              key: Key('story_archive_tile_${story.id}'),
              onTap: onOpen,
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Align(
                  alignment: Alignment.bottomLeft,
                  child: Text(
                    label,
                    maxLines: 3,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: foreground,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ),
            ),
          ),
          ColoredBox(
            color: voice.surface,
            child: TextButton.icon(
              key: Key('story_archive_add_highlight_${story.id}'),
              onPressed: () async {
                final create = await showVoiceBottomSheet<bool>(
                  context: context,
                  initialSize: 0.5,
                  scrollable: false,
                  child: _ArchiveHighlightPicker(storyId: story.id),
                );
                if (create == true && context.mounted) {
                  await HighlightEditSheet.showCreate(
                    context,
                    initialStoryIds: [story.id],
                  );
                }
              },
              icon: const Icon(Icons.add, size: 16),
              label: Text(
                l10n.storyArchiveAddToHighlight,
                textAlign: TextAlign.center,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ArchiveHighlightPicker extends ConsumerStatefulWidget {
  const _ArchiveHighlightPicker({required this.storyId});
  final String storyId;
  @override
  ConsumerState<_ArchiveHighlightPicker> createState() =>
      _ArchiveHighlightPickerState();
}

class _ArchiveHighlightPickerState
    extends ConsumerState<_ArchiveHighlightPicker> {
  bool _saving = false;
  String? _error;
  Future<void> _add(String highlightId, String profileId) async {
    if (_saving) return;
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    final result = await ref
        .read(voiceStoriesClientProvider)
        .addToHighlight(
          authorization: auth,
          highlightId: highlightId,
          storyId: widget.storyId,
        );
    if (!mounted) return;
    switch (result) {
      case StoriesApiOk():
        ref.invalidate(profileHighlightsProvider(profileId));
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(AppLocalizations.of(context)!.storyHighlightSaved),
          ),
        );
        Navigator.of(context).pop(false);
      case StoriesApiFailure(:final statusCode):
        setState(() {
          _saving = false;
          _error = commonActionErrorMessage(
            AppLocalizations.of(context)!,
            statusCode: statusCode,
          );
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final profileId = ref.watch(authControllerProvider).activeProfileId;
    final highlights = profileId == null
        ? const AsyncData<List<HighlightData>>([])
        : ref.watch(profileHighlightsProvider(profileId));
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l10n.storyHighlightSelectHighlight,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            Expanded(
              child: highlights.when(
                loading: () => const VoiceListSkeleton(),
                error: (error, _) => ListView(
                  children: [
                    Text(l10n.storyArchiveLoadError),
                    const SizedBox(height: 8),
                    Text(commonActionErrorMessage(l10n)),
                    TextButton(
                      onPressed: () =>
                          ref.invalidate(profileHighlightsProvider(profileId!)),
                      child: Text(l10n.commonRetry),
                    ),
                  ],
                ),
                data: (items) => ListView(
                  children: [
                    for (final item in items)
                      ListTile(
                        title: Text(item.name),
                        enabled: !_saving,
                        onTap: () => _add(item.id, profileId!),
                      ),
                  ],
                ),
              ),
            ),
            if (_saving) const LinearProgressIndicator(),
            if (_error != null)
              Semantics(
                liveRegion: true,
                child: Text(
                  _error!,
                  style: TextStyle(color: VoiceColors.of(context).error),
                ),
              ),
            TextButton.icon(
              onPressed: _saving || profileId == null
                  ? null
                  : () => Navigator.of(context).pop(true),
              icon: const Icon(Icons.add),
              label: Text(l10n.storyHighlightCreate),
            ),
            TextButton(
              onPressed: _saving ? null : () => Navigator.of(context).pop(),
              child: Text(l10n.commonCancel),
            ),
          ],
        ),
      ),
    );
  }
}
