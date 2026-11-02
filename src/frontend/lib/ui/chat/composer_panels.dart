import 'package:flutter/material.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_emoji_style.dart';
import '../../theme/voice_layout.dart';
import '../../theme/voice_colors.dart';
import '../a11y/focus_trap.dart';
import '../core/voice_bottom_sheet.dart';

/// Attach menu actions per [screen-controls.md] §3.6a (subset shipped in client).
enum ComposerAttachAction { photoOrVideo, document, music }

/// Transient emoji panel (§3.6b) — not SideHost.
Future<void> showComposerEmojiPanel(
  BuildContext context, {
  required void Function(String emoji) onSelected,
  List<String> recentEmoji = const [],
  BuildContext? anchorContext,
  WidgetBuilder? panelBuilder,
  VoidCallback? onDismiss,
}) async {
  final l10n = AppLocalizations.of(context)!;
  Widget body(BuildContext routeContext) =>
      panelBuilder?.call(routeContext) ??
      ComposerEmojiPanelBody(
        title: l10n.composerEmojiPanelTitle,
        recentEmoji: recentEmoji,
        onSelected: (value) => Navigator.of(routeContext).pop(value),
      );
  try {
    String? emoji;
    if (VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width)) {
      emoji = await showVoiceBottomSheet<String>(
        context: context,
        initialSize: (520 / MediaQuery.sizeOf(context).height).clamp(0.3, 0.8),
        minSize: 0.3,
        maxSize: 0.9,
        scrollable: false,
        child: Builder(builder: body),
      );
    } else {
      final overlay =
          Navigator.of(
                context,
                rootNavigator: true,
              ).overlay!.context.findRenderObject()!
              as RenderBox;
      final targetContext =
          anchorContext ??
          FocusManager.instance.primaryFocus?.context ??
          context;
      final target = targetContext.findRenderObject()! as RenderBox;
      final anchor =
          target.localToGlobal(Offset.zero, ancestor: overlay) & target.size;
      emoji = await showDialog<String>(
        context: context,
        useSafeArea: false,
        barrierColor: Colors.transparent,
        builder: (routeContext) => CustomSingleChildLayout(
          delegate: _AttachMenuLayout(
            anchor,
            MediaQuery.paddingOf(routeContext),
            width: 360,
            alignRight: true,
            maxHeight: 360,
          ),
          child: VoiceFocusTrap(
            onEscape: () => Navigator.of(routeContext).pop(),
            child: body(routeContext),
          ),
        ),
      );
    }
    if (emoji != null) onSelected(emoji);
  } finally {
    onDismiss?.call();
  }
}

/// Attach popup (§3.6a) with focus trap (§3.6e).
Future<ComposerAttachAction?> showComposerAttachMenu(
  BuildContext context, {
  BuildContext? anchorContext,
  VoidCallback? onDismiss,
}) async {
  final l10n = AppLocalizations.of(context)!;
  final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
  Widget body(BuildContext routeContext) => ComposerAttachMenuBody(
    onSelected: (action) => Navigator.of(routeContext).pop(action),
    photoOrVideoLabel: l10n.composerAttachPhotoOrVideo,
    documentLabel: l10n.composerAttachDocument,
    musicLabel: l10n.chatPinnedTypeMusic,
    title: narrow ? l10n.chatAttachFile : null,
  );
  if (narrow) {
    try {
      return await showVoiceBottomSheet<ComposerAttachAction>(
        context: context,
        initialSize: 0.45,
        minSize: 0.3,
        maxSize: 0.7,
        child: Builder(builder: body),
      );
    } finally {
      onDismiss?.call();
    }
  }

  final overlay =
      Navigator.of(
            context,
            rootNavigator: true,
          ).overlay!.context.findRenderObject()!
          as RenderBox;
  final trigger = (anchorContext ?? context).findRenderObject()! as RenderBox;
  final anchor =
      trigger.localToGlobal(Offset.zero, ancestor: overlay) & trigger.size;
  try {
    return await showDialog<ComposerAttachAction>(
      context: context,
      useSafeArea: false,
      barrierColor: Colors.transparent,
      builder: (routeContext) => CustomSingleChildLayout(
        delegate: _AttachMenuLayout(anchor, MediaQuery.paddingOf(routeContext)),
        child: VoiceFocusTrap(
          onEscape: () => Navigator.of(routeContext).pop(),
          child: SingleChildScrollView(child: body(routeContext)),
        ),
      ),
    );
  } finally {
    onDismiss?.call();
  }
}

/// The reference popup is 260 px wide, 8 px above the actual Attach button.
class _AttachMenuLayout extends SingleChildLayoutDelegate {
  const _AttachMenuLayout(
    this.anchor,
    this.padding, {
    this.width = 260,
    this.alignRight = false,
    this.maxHeight = double.infinity,
  });

  final Rect anchor;
  final EdgeInsets padding;
  final double width;
  final bool alignRight;
  final double maxHeight;
  static const margin = 8.0;

  @override
  BoxConstraints getConstraintsForChild(BoxConstraints constraints) {
    final panelWidth = (constraints.maxWidth - padding.horizontal - margin * 2)
        .clamp(0.0, width);
    return BoxConstraints(
      minWidth: panelWidth,
      maxWidth: panelWidth,
      maxHeight: (constraints.maxHeight - padding.vertical - margin * 2).clamp(
        0.0,
        maxHeight,
      ),
    );
  }

  @override
  Offset getPositionForChild(Size size, Size childSize) => Offset(
    (alignRight ? anchor.right - childSize.width : anchor.left).clamp(
      padding.left + margin,
      size.width - padding.right - margin - childSize.width,
    ),
    (anchor.top - margin - childSize.height).clamp(
      padding.top + margin,
      size.height - padding.bottom - margin - childSize.height,
    ),
  );

  @override
  bool shouldRelayout(_AttachMenuLayout oldDelegate) =>
      anchor != oldDelegate.anchor ||
      padding != oldDelegate.padding ||
      width != oldDelegate.width ||
      alignRight != oldDelegate.alignRight ||
      maxHeight != oldDelegate.maxHeight;
}

class ComposerEmojiPanelBody extends StatelessWidget {
  const ComposerEmojiPanelBody({
    super.key,
    required this.title,
    required this.onSelected,
    this.recentEmoji = const [],
    this.showTitle = true,
  });

  static const panelKey = Key('composer_emoji_panel');
  static const choices = [
    '😀',
    '😂',
    '🥹',
    '😍',
    '🤝',
    '🔥',
    '🎉',
    '❤️',
    '👍',
    '👀',
    '💯',
    '🚀',
    '✨',
    '🙌',
    '🤔',
    '😎',
    '👏',
    '🙏',
    '💡',
    '✅',
    '💬',
    '🎧',
    '📎',
    '🌟',
    '😮',
    '😢',
  ];

  final String title;
  final ValueChanged<String> onSelected;
  final List<String> recentEmoji;
  final bool showTitle;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: title,
      child: Material(
        key: panelKey,
        borderRadius: BorderRadius.circular(16),
        clipBehavior: Clip.antiAlias,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (showTitle) ...[
                Text(title, style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: 8),
              ],
              Flexible(
                child: SingleChildScrollView(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      if (recentEmoji.isNotEmpty) ...[
                        Text(AppLocalizations.of(context)!.composerEmojiRecent),
                        _grid(recentEmoji.toSet().toList(), recent: true),
                        const Divider(),
                      ],
                      _grid(choices),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _grid(List<String> emoji, {bool recent = false}) => GridView.count(
    crossAxisCount: 6,
    shrinkWrap: true,
    physics: const NeverScrollableScrollPhysics(),
    children: [
      for (final value in emoji)
        IconButton(
          autofocus: recent
              ? value == emoji.first
              : recentEmoji.isEmpty && value == emoji.first,
          key: Key(recent ? 'composer_recent_$value' : 'composer_emoji_$value'),
          tooltip: value,
          onPressed: () => onSelected(value),
          icon: Text(value, style: VoiceEmojiStyle.textStyle(fontSize: 28)),
        ),
    ],
  );
}

class ComposerAttachMenuBody extends StatelessWidget {
  const ComposerAttachMenuBody({
    super.key,
    required this.onSelected,
    required this.photoOrVideoLabel,
    required this.documentLabel,
    required this.musicLabel,
    this.title,
  });

  static const menuKey = Key('composer_attach_menu');

  final ValueChanged<ComposerAttachAction> onSelected;
  final String photoOrVideoLabel;
  final String documentLabel;
  final String musicLabel;
  final String? title;

  @override
  Widget build(BuildContext context) {
    return Material(
      color:
          Theme.of(context).dialogTheme.backgroundColor ??
          Theme.of(context).colorScheme.surface,
      borderRadius: BorderRadius.circular(12),
      child: Semantics(
        container: true,
        child: Padding(
          key: menuKey,
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (title != null) ...[
                Center(
                  child: Container(
                    width: 36,
                    height: 4,
                    decoration: BoxDecoration(
                      color: VoiceColors.of(context).borderStrong,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
                  child: Semantics(
                    header: true,
                    child: Text(
                      title!,
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                  ),
                ),
              ],
              ListTile(
                key: const Key('composer_attach_photo_or_video'),
                autofocus: true,
                leading: const Icon(Icons.photo_outlined),
                title: Text(photoOrVideoLabel),
                onTap: () => onSelected(ComposerAttachAction.photoOrVideo),
              ),
              ListTile(
                key: const Key('composer_attach_document'),
                leading: const Icon(Icons.description_outlined),
                title: Text(documentLabel),
                onTap: () => onSelected(ComposerAttachAction.document),
              ),
              ListTile(
                key: const Key('composer_attach_music'),
                leading: const Icon(Icons.music_note_outlined),
                title: Text(musicLabel),
                onTap: () => onSelected(ComposerAttachAction.music),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
