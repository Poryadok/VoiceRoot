import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:video_player/video_player.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_emoji_style.dart';
import '../api_error_messages.dart';
import '../core/voice_state_panel.dart';
import '../core/voice_skeleton.dart';
import '../report/report_sheet.dart';
import 'lfp_story_card.dart';
import 'story_game_tag_chip.dart';
import 'story_text_style.dart';
import 'story_viewers_sheet.dart';

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
  static const Key reactPickerKey = Key('story_viewer_react_picker');
  static const Key reactionsRowKey = Key('story_viewer_reactions_row');
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

    final emoji = await _pickReactionEmoji();
    if (emoji == null || !mounted) return;

    final result = await ref.read(voiceStoriesClientProvider).reactToStory(
          authorization: auth,
          storyId: storyId,
          emoji: emoji,
        );
    if (!mounted) return;
    final l10n = AppLocalizations.of(context)!;
    switch (result) {
      case StoriesApiOk():
        ref.invalidate(storyReactionsProvider(storyId));
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.storyReactSent)),
        );
      case StoriesApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
    }
  }

  Future<String?> _pickReactionEmoji() {
    const choices = ['👍', '❤️', '🔥', '😂', '🎉'];
    return showModalBottomSheet<String>(
      context: context,
      builder: (context) => SafeArea(
        key: StoryViewerScreen.reactPickerKey,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 12),
          child: Wrap(
            alignment: WrapAlignment.center,
            spacing: 12,
            runSpacing: 12,
            children: [
              for (final emoji in choices)
                IconButton(
                  key: Key('story_viewer_react_emoji_$emoji'),
                  onPressed: () => Navigator.of(context).pop(emoji),
                  icon: Text(
                    emoji,
                    style: VoiceEmojiStyle.textStyle(fontSize: 28),
                  ),
                ),
            ],
          ),
        ),
      ),
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
          loading: () => const VoiceListSkeleton(rowCount: 4),
          error: (error, _) => VoiceStatePanel(
            title: l10n.storyViewerLoadError,
            message: storyViewerErrorMessage(l10n, error),
            icon: Icons.cloud_off_outlined,
            actionLabel: l10n.commonRetry,
            onAction: () => ref.invalidate(storyDetailProvider(storyId)),
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
              storyId: storyId,
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
    required this.storyId,
    required this.isAuthor,
    required this.onReact,
    required this.onReply,
    required this.reactButtonKey,
    required this.replyButtonKey,
    required this.viewCountKey,
  });

  final StoryData story;
  final String storyId;
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
    final reactionsAsync =
        isAuthor ? ref.watch(storyReactionsProvider(storyId)) : null;
    final reactionAggregates = reactionsAsync == null
        ? const <({String emoji, int count})>[]
        : reactionsAsync.when(
            data: aggregateStoryReactions,
            loading: () => const <({String emoji, int count})>[],
            error: (_, _) => const <({String emoji, int count})>[],
          );

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
        final bg = storyTextBackgroundColor(voice, story.textStyleJson);
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
        return Stack(
          fit: StackFit.expand,
          children: [
            ColoredBox(color: bg, child: Center(child: body)),
            ..._overlayChildren(
              context,
              l10n,
              voice,
              showViewCount: isAuthor,
              reactionAggregates: reactionAggregates,
            ),
          ],
        );
    }

    return Stack(
      fit: StackFit.expand,
      children: [
        ColoredBox(color: voice.elevated, child: Center(child: body)),
        ..._overlayChildren(
          context,
          l10n,
          voice,
          showViewCount: isAuthor,
          reactionAggregates: reactionAggregates,
        ),
      ],
    );
  }

  List<Widget> _overlayChildren(
    BuildContext context,
    AppLocalizations l10n,
    VoiceColors voice, {
    bool showViewCount = false,
    List<({String emoji, int count})> reactionAggregates = const [],
  }) {
    final gameTag = story.gameTag;
    return [
      if (gameTag != null && gameTag.isNotEmpty)
        Positioned(
          top: 48,
          left: 16,
          right: 16,
          child: Align(
            alignment: Alignment.topLeft,
            child: StoryGameTagChip(gameTag: gameTag),
          ),
        ),
      if (showViewCount)
        Positioned(
          top: 8,
          left: 16,
          child: Material(
            color: Colors.transparent,
            child: InkWell(
              key: viewCountKey,
              onTap: () => StoryViewersSheet.show(context, storyId: storyId),
              borderRadius: BorderRadius.circular(8),
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
                child: Text(
                  l10n.storyViewerViewCount(story.viewCount),
                  style: TextStyle(color: voice.textSecondary),
                ),
              ),
            ),
          ),
        ),
      if (showViewCount && reactionAggregates.isNotEmpty)
        Positioned(
          left: 16,
          right: 16,
          bottom: 72,
          child: Wrap(
            key: StoryViewerScreen.reactionsRowKey,
            spacing: 6,
            runSpacing: 6,
            children: [
              for (final reaction in reactionAggregates)
                _StoryReactionChip(
                  emoji: reaction.emoji,
                  count: reaction.count,
                  voice: voice,
                ),
            ],
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
              icon: const Icon(Icons.add_reaction_outlined),
            ),
          ],
        ),
      ),
    ];
  }
}

class _StoryReactionChip extends StatelessWidget {
  const _StoryReactionChip({
    required this.emoji,
    required this.count,
    required this.voice,
  });

  final String emoji;
  final int count;
  final VoiceColors voice;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: voice.elevated,
      borderRadius: BorderRadius.circular(12),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(emoji, style: VoiceEmojiStyle.textStyle(fontSize: 14)),
            if (count > 1) ...[
              const SizedBox(width: 4),
              Text(
                '$count',
                style: Theme.of(context).textTheme.labelSmall?.copyWith(
                      color: voice.textSecondary,
                      fontWeight: FontWeight.w600,
                    ),
              ),
            ],
          ],
        ),
      ),
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
