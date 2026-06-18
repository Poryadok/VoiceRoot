import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:video_player/video_player.dart';

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
  static const Key replyButtonKey = Key('story_viewer_reply');
  static const Key viewCountKey = Key('story_viewer_view_count');

  final List<String> storyIds;
  final int initialIndex;
  final String? profileId;

  @override
  ConsumerState<StoryViewerScreen> createState() => _StoryViewerScreenState();
}

class _StoryViewerScreenState extends ConsumerState<StoryViewerScreen> {
  late int _index;
  final _markedViewed = <String>{};

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

  Future<void> _reply(StoryData story) async {
    final auth = ref.read(authorizationHeaderProvider);
    final storyId = story.id;
    if (auth == null || storyId.isEmpty) return;

    final l10n = AppLocalizations.of(context)!;
    final controller = TextEditingController();
    final text = await showDialog<String>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.storyViewerReply),
          content: TextField(
            controller: controller,
            autofocus: true,
            maxLines: 3,
            decoration: InputDecoration(hintText: dialogL10n.storyViewerReplyHint),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: Text(dialogL10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(controller.text.trim()),
              child: Text(dialogL10n.storyViewerReply),
            ),
          ],
        );
      },
    );
    controller.dispose();
    if (text == null || text.isEmpty || !mounted) return;

    final result = await ref.read(voiceStoriesClientProvider).replyToStory(
          authorization: auth,
          storyId: storyId,
          text: text,
        );
    if (!mounted) return;
    switch (result) {
      case StoriesApiOk():
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.storyViewerReplySent)),
        );
      case StoriesApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
    }
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
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;

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
            final isAuthor = activeProfileId == story.authorProfileId;
            return _StoryContent(
              story: story,
              isAuthor: isAuthor,
              onReact: _react,
              onReply: () => _reply(story),
              reactButtonKey: StoryViewerScreen.reactButtonKey,
              replyButtonKey: StoryViewerScreen.replyButtonKey,
              viewCountKey: StoryViewerScreen.viewCountKey,
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
    required this.isAuthor,
    required this.onReact,
    required this.onReply,
    required this.reactButtonKey,
    required this.replyButtonKey,
    required this.viewCountKey,
  });

  final StoryData story;
  final bool isAuthor;
  final VoidCallback onReact;
  final VoidCallback onReply;
  final Key reactButtonKey;
  final Key replyButtonKey;
  final Key viewCountKey;

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
                return _StoryVideoPlayer(url: url);
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

    final showViewCount = isAuthor;

    return Stack(
      fit: StackFit.expand,
      children: [
        ColoredBox(color: voice.elevated, child: Center(child: body)),
        if (showViewCount)
          Positioned(
            top: 8,
            left: 16,
            child: Text(
              key: viewCountKey,
              l10n.storyViewerViewCount(story.viewCount),
              style: TextStyle(color: voice.textSecondary),
            ),
          ),
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
              if (!isAuthor)
                IconButton(
                  key: replyButtonKey,
                  tooltip: l10n.storyViewerReply,
                  onPressed: onReply,
                  icon: const Icon(Icons.reply_outlined),
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

class _StoryVideoPlayer extends StatefulWidget {
  const _StoryVideoPlayer({required this.url});

  final String url;

  @override
  State<_StoryVideoPlayer> createState() => _StoryVideoPlayerState();
}

class _StoryVideoPlayerState extends State<_StoryVideoPlayer> {
  late final VideoPlayerController _controller;
  var _initialized = false;

  @override
  void initState() {
    super.initState();
    _controller = VideoPlayerController.networkUrl(Uri.parse(widget.url))
      ..initialize().then((_) {
        if (!mounted) return;
        setState(() => _initialized = true);
        _controller.play();
      });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_initialized) {
      return const Center(child: CircularProgressIndicator());
    }
    return AspectRatio(
      aspectRatio: _controller.value.aspectRatio,
      child: VideoPlayer(_controller),
    );
  }
}
