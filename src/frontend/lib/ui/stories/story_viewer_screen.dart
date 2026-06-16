import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_state_panel.dart';
import '../report/report_sheet.dart';
import 'lfp_story_card.dart';

/// Full-screen story viewer with reactions and report.
class StoryViewerScreen extends ConsumerStatefulWidget {
  const StoryViewerScreen({
    super.key,
    required this.storyIds,
    this.initialIndex = 0,
    this.profileId,
  });

  static const Key screenKey = Key('story_viewer_screen');
  static const Key reportButtonKey = Key('story_viewer_report');
  static const Key reactButtonKey = Key('story_viewer_react');

  final List<String> storyIds;
  final int initialIndex;
  final String? profileId;

  @override
  ConsumerState<StoryViewerScreen> createState() => _StoryViewerScreenState();
}

class _StoryViewerScreenState extends ConsumerState<StoryViewerScreen> {
  late int _index;
  var _markedViewed = <String>{};

  @override
  void initState() {
    super.initState();
    _index = widget.initialIndex.clamp(0, widget.storyIds.length - 1);
    WidgetsBinding.instance.addPostFrameCallback((_) => _markCurrentViewed());
  }

  String get _currentStoryId =>
      widget.storyIds.isEmpty ? '' : widget.storyIds[_index];

  Future<void> _markCurrentViewed() async {
    final storyId = _currentStoryId;
    if (storyId.isEmpty || _markedViewed.contains(storyId)) return;
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await ref.read(voiceStoriesClientProvider).markViewed(
          authorization: auth,
          storyId: storyId,
        );
    if (result is StoriesApiOk<void>) {
      _markedViewed.add(storyId);
    }
  }

  void _next() {
    if (_index < widget.storyIds.length - 1) {
      setState(() => _index++);
      _markCurrentViewed();
    } else {
      Navigator.of(context).maybePop();
    }
  }

  void _previous() {
    if (_index > 0) {
      setState(() => _index--);
    }
  }

  Future<void> _react() async {
    final auth = ref.read(authorizationHeaderProvider);
    final storyId = _currentStoryId;
    if (auth == null || storyId.isEmpty) return;
    await ref.read(voiceStoriesClientProvider).reactToStory(
          authorization: auth,
          storyId: storyId,
          emoji: '🔥',
        );
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(AppLocalizations.of(context)!.storyReactSent)),
    );
  }

  void _report() {
    final storyId = _currentStoryId;
    if (storyId.isEmpty) return;
    ReportSheet.show(
      context,
      target: ReportStoryTarget(storyId: storyId),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final storyId = _currentStoryId;

    if (storyId.isEmpty) {
      return Scaffold(
        appBar: AppBar(),
        body: VoiceStatePanel(
          title: l10n.storyViewerEmpty,
          icon: Icons.auto_stories_outlined,
        ),
      );
    }

    final storyAsync = ref.watch(storyDetailProvider(storyId));

    return Scaffold(
      key: StoryViewerScreen.screenKey,
      backgroundColor: voice.canvas,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        actions: [
          IconButton(
            key: StoryViewerScreen.reportButtonKey,
            tooltip: l10n.reportTitle,
            icon: const Icon(Icons.flag_outlined),
            onPressed: _report,
          ),
        ],
      ),
      body: GestureDetector(
        onTapUp: (details) {
          final width = MediaQuery.sizeOf(context).width;
          if (details.localPosition.dx > width * 0.6) {
            _next();
          } else if (details.localPosition.dx < width * 0.4) {
            _previous();
          }
        },
        onVerticalDragEnd: (details) {
          if ((details.primaryVelocity ?? 0) > 200) {
            Navigator.of(context).maybePop();
          }
        },
        child: storyAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (_, _) => VoiceStatePanel(
            title: l10n.storyViewerLoadError,
            icon: Icons.cloud_off_outlined,
          ),
          data: (story) {
            if (story == null) {
              return VoiceStatePanel(
                title: l10n.storyViewerLoadError,
                icon: Icons.cloud_off_outlined,
              );
            }
            return _StoryContent(
              story: story,
              onReact: _react,
              reactButtonKey: StoryViewerScreen.reactButtonKey,
            );
          },
        ),
      ),
    );
  }
}

class _StoryContent extends ConsumerWidget {
  const _StoryContent({
    required this.story,
    required this.onReact,
    required this.reactButtonKey,
  });

  final StoryData story;
  final VoidCallback onReact;
  final Key reactButtonKey;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    if (story.isLookingForParty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: LfpStoryCard(story: story),
        ),
      );
    }

    Widget body;
    switch (story.type) {
      case 'photo':
      case 'video':
        final fileId = story.mediaFileId;
        if (fileId == null || fileId.isEmpty) {
          body = Text(
            l10n.storyViewerNoMedia,
            style: TextStyle(color: voice.textPrimary),
          );
        } else {
          final urlAsync = ref.watch(fileAttachmentUrlProvider(fileId));
          body = urlAsync.when(
            data: (url) {
              if (url == null || url.isEmpty) {
                return Text(
                  l10n.storyViewerNoMedia,
                  style: TextStyle(color: voice.textPrimary),
                );
              }
              if (story.type == 'video') {
                return Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(Icons.play_circle_outline,
                        size: 64, color: voice.textSecondary),
                    const SizedBox(height: 8),
                    Text(
                      l10n.storyViewerVideoPlaceholder,
                      style: TextStyle(color: voice.textSecondary),
                    ),
                  ],
                );
              }
              return Image.network(
                url,
                fit: BoxFit.contain,
                errorBuilder: (_, _, _) => Text(
                  l10n.storyViewerNoMedia,
                  style: TextStyle(color: voice.textPrimary),
                ),
              );
            },
            loading: () => const CircularProgressIndicator(),
            error: (_, _) => Text(
              l10n.storyViewerNoMedia,
              style: TextStyle(color: voice.textPrimary),
            ),
          );
        }
      default:
        body = Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Text(
              story.textContent ?? '',
              style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    color: voice.textPrimary,
                  ),
              textAlign: TextAlign.center,
            ),
          ),
        );
    }

    return Stack(
      fit: StackFit.expand,
      children: [
        ColoredBox(color: voice.elevated, child: Center(child: body)),
        Positioned(
          left: 16,
          right: 16,
          bottom: 24,
          child: Row(
            children: [
              Expanded(
                child: Text(
                  story.textContent ?? '',
                  style: TextStyle(color: voice.textPrimary),
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              IconButton(
                key: reactButtonKey,
                tooltip: l10n.storyReactTooltip,
                onPressed: onReact,
                icon: const Text('🔥', style: TextStyle(fontSize: 24)),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
