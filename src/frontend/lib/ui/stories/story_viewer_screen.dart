import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:video_player/video_player.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/stories_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_emoji_style.dart';
import '../../theme/voice_theme_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_state_panel.dart';
import '../core/voice_skeleton.dart';
import '../report/report_sheet.dart';
import '../social/profile_detail_sheet.dart';
import 'lfp_story_card.dart';
import 'story_game_tag_chip.dart';
import 'story_text_style.dart';
import 'story_viewers_sheet.dart';
import 'story_author_session.dart';

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
  static const Key moreButtonKey = Key('story_viewer_more');
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
  late final List<String> _storyIds;
  bool _deleting = false;
  int _mutationEpoch = 0;
  final _markedViewed = <String>{};

  @override
  void initState() {
    super.initState();
    _storyIds = List.of(widget.storyIds);
    _index = _storyIds.isEmpty
        ? 0
        : widget.initialIndex.clamp(0, _storyIds.length - 1);
    WidgetsBinding.instance.addPostFrameCallback((_) => _markCurrentViewed());
  }

  String get _currentStoryId => _storyIds.isEmpty ? '' : _storyIds[_index];

  Future<void> _markCurrentViewed() async {
    final storyId = _currentStoryId;
    if (storyId.isEmpty || _markedViewed.contains(storyId)) return;
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await ref
        .read(voiceStoriesClientProvider)
        .markViewed(authorization: auth, storyId: storyId);
    if (result is StoriesApiOk<void>) {
      _markedViewed.add(storyId);
    }
  }

  void _next() {
    if (_deleting) return;
    if (_index < _storyIds.length - 1) {
      setState(() => _index++);
      _markCurrentViewed();
    } else {
      Navigator.of(context).maybePop();
    }
  }

  void _previous() {
    if (_deleting) return;
    if (_index > 0) {
      setState(() => _index--);
      _markCurrentViewed();
    }
  }

  Future<void> _delete(StoryData story) async {
    if (_deleting || ref.read(profileSwitchInProgressProvider)) return;
    final session = StoryAuthorSession.capture(ref);
    final epoch = _mutationEpoch;
    if (session.profileId != story.authorProfileId) return;
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.storyViewerDeleteConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: Text(l10n.commonDelete),
          ),
        ],
      ),
    );
    if (confirmed != true ||
        !mounted ||
        !session.isCurrent(ref) ||
        epoch != _mutationEpoch ||
        ref.read(profileSwitchInProgressProvider)) {
      return;
    }
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null ||
        ref.read(authControllerProvider).activeProfileId !=
            story.authorProfileId) {
      return;
    }
    setState(() => _deleting = true);
    final result = await ref
        .read(voiceStoriesClientProvider)
        .deleteStory(authorization: auth, storyId: story.id);
    if (!mounted) return;
    setState(() => _deleting = false);
    if (!session.isCurrent(ref) ||
        epoch != _mutationEpoch ||
        ref.read(profileSwitchInProgressProvider)) {
      return;
    }
    switch (result) {
      case StoriesApiOk():
        ref.invalidate(storyDetailProvider(story.id));
        ref.invalidate(storyFeedProvider);
        ref.invalidate(storyArchiveProvider);
        ref.invalidate(profileStoriesProvider(story.authorProfileId));
        setState(() {
          _storyIds.removeWhere((id) => id == story.id);
          _index = _storyIds.isEmpty
              ? 0
              : _index.clamp(0, _storyIds.length - 1);
        });
        if (_storyIds.isEmpty) {
          Navigator.of(context).maybePop();
        } else {
          _markCurrentViewed();
        }
      case StoriesApiFailure(:final statusCode):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              commonActionErrorMessage(l10n, statusCode: statusCode),
            ),
          ),
        );
    }
  }

  void _openAuthor(String profileId) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (_) => ProfileDetailSheet(profileId: profileId),
    );
  }

  Future<void> _react() async {
    final auth = ref.read(authorizationHeaderProvider);
    final storyId = _currentStoryId;
    if (auth == null || storyId.isEmpty) return;

    final emoji = await _pickReactionEmoji();
    if (emoji == null || !mounted) return;

    final result = await ref
        .read(voiceStoriesClientProvider)
        .reactToStory(authorization: auth, storyId: storyId, emoji: emoji);
    if (!mounted) return;
    final l10n = AppLocalizations.of(context)!;
    switch (result) {
      case StoriesApiOk():
        ref.invalidate(storyReactionsProvider(storyId));
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(l10n.storyReactSent)));
      case StoriesApiFailure(:final statusCode):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              commonActionErrorMessage(l10n, statusCode: statusCode),
            ),
          ),
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
                  tooltip: emoji,
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
    final text = await showDialog<String>(
      context: context,
      builder: (_) => const _StoryReplyDialog(),
    );
    if (text == null || text.isEmpty || !mounted) return;

    final result = await ref
        .read(voiceStoriesClientProvider)
        .replyToStory(authorization: auth, storyId: storyId, text: text);
    if (!mounted) return;
    switch (result) {
      case StoriesApiOk():
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(l10n.storyViewerReplySent)));
      case StoriesApiFailure(:final statusCode):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              commonActionErrorMessage(l10n, statusCode: statusCode),
            ),
          ),
        );
    }
  }

  void _report() {
    final storyId = _currentStoryId;
    if (storyId.isEmpty) return;
    ReportSheet.show(context, target: ReportStoryTarget(storyId: storyId));
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final storyId = _currentStoryId;
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    ref.listen<bool>(profileSwitchInProgressProvider, (_, switching) {
      if (switching) _mutationEpoch++;
    });

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
      appBar: storyAsync.valueOrNull == null
          ? AppBar(
              actions: [
                IconButton(
                  key: const Key('story_viewer_close'),
                  tooltip: MaterialLocalizations.of(context).closeButtonTooltip,
                  onPressed: () => Navigator.of(context).maybePop(),
                  icon: const Icon(Icons.close),
                ),
              ],
            )
          : null,
      body: SafeArea(
        child: Focus(
          autofocus: true,
          onKeyEvent: (_, event) {
            if (event is! KeyDownEvent) return KeyEventResult.ignored;
            if (event.logicalKey == LogicalKeyboardKey.arrowRight) {
              _next();
            } else if (event.logicalKey == LogicalKeyboardKey.arrowLeft) {
              _previous();
            } else if (event.logicalKey == LogicalKeyboardKey.escape) {
              Navigator.of(context).maybePop();
            } else {
              return KeyEventResult.ignored;
            }
            return KeyEventResult.handled;
          },
          child: GestureDetector(
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
                message:
                    storyViewerErrorMessage(l10n, error) ==
                        l10n.storyViewerLoadError
                    ? null
                    : storyViewerErrorMessage(l10n, error),
                icon: Icons.cloud_off_outlined,
                actionLabel: l10n.commonRetry,
                onAction: () => ref.invalidate(storyDetailProvider(storyId)),
              ),
              data: (story) {
                if (story == null) {
                  return VoiceStatePanel(
                    title: l10n.storyViewerLoadError,
                    icon: Icons.cloud_off_outlined,
                    actionLabel: l10n.commonRetry,
                    onAction: () =>
                        ref.invalidate(storyDetailProvider(storyId)),
                  );
                }
                final isAuthor = activeProfileId == story.authorProfileId;
                final onMedia =
                    story.isLookingForParty ||
                    story.type == 'photo' ||
                    story.type == 'video';
                final headerColor = onMedia ? Colors.white : voice.textPrimary;
                return Stack(
                  fit: StackFit.expand,
                  children: [
                    _StoryContent(
                      story: story,
                      storyId: storyId,
                      isAuthor: isAuthor,
                      onReact: _react,
                      onReply: () => _reply(story),
                      reactButtonKey: StoryViewerScreen.reactButtonKey,
                      replyButtonKey: StoryViewerScreen.replyButtonKey,
                      viewCountKey: StoryViewerScreen.viewCountKey,
                    ),
                    Positioned(
                      top: 8,
                      left: 12,
                      right: 12,
                      child: IconTheme(
                        data: IconThemeData(color: headerColor),
                        child: Column(
                          children: [
                            Semantics(
                              key: const Key('story_viewer_progress'),
                              container: true,
                              explicitChildNodes: true,
                              label: l10n.storyViewerProgress(
                                _index + 1,
                                _storyIds.length,
                              ),
                              child: Row(
                                children: [
                                  for (var i = 0; i < _storyIds.length; i++)
                                    Expanded(
                                      child: Padding(
                                        padding: const EdgeInsets.symmetric(
                                          horizontal: 2,
                                        ),
                                        child: InkWell(
                                          key: Key('story_viewer_segment_$i'),
                                          onTap: _deleting
                                              ? null
                                              : () {
                                                  setState(() => _index = i);
                                                  _markCurrentViewed();
                                                },
                                          child: SizedBox(
                                            height: 16,
                                            child: Center(
                                              child: Container(
                                                height: 3,
                                                decoration: BoxDecoration(
                                                  color: i <= _index
                                                      ? headerColor
                                                      : headerColor.withValues(
                                                          alpha: .45,
                                                        ),
                                                  borderRadius:
                                                      BorderRadius.circular(3),
                                                ),
                                              ),
                                            ),
                                          ),
                                        ),
                                      ),
                                    ),
                                ],
                              ),
                            ),
                            Row(
                              children: [
                                Expanded(
                                  child: _StoryAuthorHeader(
                                    story: story,
                                    foregroundColor: headerColor,
                                    onTap: () =>
                                        _openAuthor(story.authorProfileId),
                                  ),
                                ),
                                if (isAuthor)
                                  IconButton(
                                    key: const Key('story_viewer_delete'),
                                    tooltip: l10n.commonDelete,
                                    onPressed: _deleting
                                        ? null
                                        : () => _delete(story),
                                    icon: const Icon(Icons.delete_outline),
                                  ),
                                PopupMenuButton<void>(
                                  key: StoryViewerScreen.moreButtonKey,
                                  icon: Icon(
                                    Icons.more_horiz,
                                    color: headerColor,
                                  ),
                                  itemBuilder: (_) => [
                                    PopupMenuItem<void>(
                                      key: StoryViewerScreen.reportButtonKey,
                                      onTap: _report,
                                      child: Text(l10n.reportTitle),
                                    ),
                                  ],
                                ),
                                IconButton(
                                  key: const Key('story_viewer_close'),
                                  tooltip: MaterialLocalizations.of(
                                    context,
                                  ).closeButtonTooltip,
                                  onPressed: () =>
                                      Navigator.of(context).maybePop(),
                                  icon: const Icon(Icons.close),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
        ),
      ),
    );
  }
}

class _StoryAuthorHeader extends ConsumerWidget {
  const _StoryAuthorHeader({
    required this.story,
    required this.onTap,
    required this.foregroundColor,
  });
  final StoryData story;
  final VoidCallback onTap;
  final Color foregroundColor;
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final profile = ref
        .watch(profileProvider(story.authorProfileId))
        .valueOrNull;
    final name = profile?.displayName ?? story.authorProfileId;
    return TextButton(
      key: const Key('story_viewer_author'),
      onPressed: onTap,
      child: Row(
        children: [
          CircleAvatar(
            radius: 20,
            child: Text(
              name.isEmpty ? '?' : name.characters.first.toUpperCase(),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(
                    context,
                  ).textTheme.titleSmall?.copyWith(color: foregroundColor),
                ),
                if (story.createdAt case final createdAt?)
                  Text(
                    '${MaterialLocalizations.of(context).formatShortDate(createdAt.toLocal())} · ${MaterialLocalizations.of(context).formatTimeOfDay(TimeOfDay.fromDateTime(createdAt.toLocal()))}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: Theme.of(
                      context,
                    ).textTheme.bodySmall?.copyWith(color: foregroundColor),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _StoryReplyDialog extends StatefulWidget {
  const _StoryReplyDialog();

  @override
  State<_StoryReplyDialog> createState() => _StoryReplyDialogState();
}

class _StoryReplyDialogState extends State<_StoryReplyDialog> {
  late final TextEditingController _controller;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return AlertDialog(
      title: Text(l10n.storyViewerReply),
      content: TextField(
        controller: _controller,
        autofocus: true,
        maxLines: 3,
        decoration: InputDecoration(hintText: l10n.storyViewerReplyHint),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.commonCancel),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(_controller.text.trim()),
          child: Text(l10n.storyViewerReply),
        ),
      ],
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
    final reactionsAsync = isAuthor
        ? ref.watch(storyReactionsProvider(storyId))
        : null;
    final reactionAggregates = reactionsAsync == null
        ? const <({String emoji, int count})>[]
        : reactionsAsync.when(
            data: aggregateStoryReactions,
            loading: () => const <({String emoji, int count})>[],
            error: (_, _) => const <({String emoji, int count})>[],
          );

    if (story.isLookingForParty) {
      final catalog = ref.watch(voiceTokenCatalogProvider).valueOrNull;
      return Stack(
        fit: StackFit.expand,
        children: [
          DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [
                  catalog?.profileAccentAt(0) ?? voice.profileAccent,
                  catalog?.profileAccentAt(2) ?? voice.profileAccent,
                  catalog?.profileAccentAt(5) ?? voice.profileAccent,
                ],
              ),
            ),
          ),
          ColoredBox(color: voice.overlay),
          Positioned(
            left: 24,
            right: 24,
            bottom: 126,
            child: Align(
              alignment: Alignment.bottomLeft,
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 620),
                child: LfpStoryCard(
                  story: story,
                  isAuthor: isAuthor,
                  embedded: true,
                ),
              ),
            ),
          ),
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
            loading: () => Center(
              child: SizedBox(
                width: 280,
                child: VoiceListSkeleton(rowCount: 3),
              ),
            ),
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
              style: Theme.of(
                context,
              ).textTheme.headlineSmall?.copyWith(color: voice.textPrimary),
              textAlign: TextAlign.center,
            ),
          ),
        );
        return Stack(
          fit: StackFit.expand,
          children: [
            ColoredBox(
              color: bg,
              child: Center(child: body),
            ),
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
        ColoredBox(
          color: voice.elevated,
          child: Center(child: body),
        ),
        ColoredBox(color: voice.overlay),
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
    final onMedia =
        story.isLookingForParty ||
        story.type == 'photo' ||
        story.type == 'video';
    final foreground = onMedia ? Colors.white : voice.textPrimary;
    return [
      if (!story.isLookingForParty && gameTag != null && gameTag.isNotEmpty)
        Positioned(
          top: 132,
          left: 16,
          right: 16,
          child: Align(
            alignment: Alignment.topLeft,
            child: StoryGameTagChip(gameTag: gameTag),
          ),
        ),
      if (showViewCount)
        Positioned(
          top: 96,
          left: 16,
          child: Material(
            color: Colors.transparent,
            child: InkWell(
              key: viewCountKey,
              onTap: story.expiresAt?.isBefore(DateTime.now().toUtc()) == true
                  ? null
                  : () => StoryViewersSheet.show(context, storyId: storyId),
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
      if (showViewCount)
        Consumer(
          builder: (context, ref, _) {
            final reactions = ref.watch(storyReactionsProvider(storyId));
            if (!reactions.hasError) return const SizedBox.shrink();
            return Positioned(
              left: 16,
              right: 16,
              bottom: 76,
              child: Align(
                alignment: Alignment.bottomLeft,
                child: TextButton.icon(
                  onPressed: () =>
                      ref.invalidate(storyReactionsProvider(storyId)),
                  icon: const Icon(Icons.refresh),
                  label: Text(
                    '${commonActionErrorMessage(l10n)} ${l10n.commonRetry}',
                  ),
                ),
              ),
            );
          },
        ),
      Positioned(
        left: 16,
        right: 16,
        bottom: 24,
        child: Row(
          children: [
            if (!isAuthor)
              Expanded(
                child: OutlinedButton.icon(
                  key: replyButtonKey,
                  onPressed: onReply,
                  style: OutlinedButton.styleFrom(
                    foregroundColor: foreground,
                    shape: const StadiumBorder(),
                    alignment: Alignment.centerLeft,
                    side: BorderSide(color: foreground.withValues(alpha: .7)),
                  ),
                  icon: const Icon(Icons.reply_outlined),
                  label: Text(l10n.storyViewerReplyHint),
                ),
              ),
            if (isAuthor) const Spacer(),
            const SizedBox(width: 8),
            IconButton(
              key: reactButtonKey,
              tooltip: l10n.storyReactTooltip,
              style: IconButton.styleFrom(foregroundColor: foreground),
              onPressed: onReact,
              icon: const Icon(Icons.favorite_border),
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
      return const Center(
        child: SizedBox(width: 280, child: VoiceListSkeleton(rowCount: 3)),
      );
    }
    return AspectRatio(
      aspectRatio: _controller.value.aspectRatio,
      child: VideoPlayer(_controller),
    );
  }
}
