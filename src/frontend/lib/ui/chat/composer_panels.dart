import 'package:flutter/material.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_emoji_style.dart';
import '../../theme/voice_layout.dart';
import '../a11y/focus_trap.dart';
import '../core/voice_bottom_sheet.dart';

/// Attach menu actions per [screen-controls.md] §3.6a (subset shipped in client).
enum ComposerAttachAction {
  photoOrVideo,
  document,
}

/// Transient emoji panel (§3.6b) — not SideHost.
Future<void> showComposerEmojiPanel(
  BuildContext context, {
  required void Function(String emoji) onSelected,
  VoidCallback? onDismiss,
}) async {
  final l10n = AppLocalizations.of(context)!;
  final emoji = await _showComposerOverlay<String>(
    context: context,
    onDismiss: onDismiss,
    child: ComposerEmojiPanelBody(
      title: l10n.composerEmojiPanelTitle,
      onSelected: (value) => Navigator.of(context).pop(value),
    ),
  );
  if (emoji != null) onSelected(emoji);
}

/// Attach popup (§3.6a) with focus trap (§3.6e).
Future<ComposerAttachAction?> showComposerAttachMenu(
  BuildContext context, {
  VoidCallback? onDismiss,
}) async {
  final l10n = AppLocalizations.of(context)!;
  return _showComposerOverlay<ComposerAttachAction>(
    context: context,
    onDismiss: onDismiss,
    child: ComposerAttachMenuBody(
      onSelected: (action) => Navigator.of(context).pop(action),
      photoOrVideoLabel: l10n.composerAttachPhotoOrVideo,
      documentLabel: l10n.composerAttachDocument,
    ),
  );
}

Future<T?> _showComposerOverlay<T>({
  required BuildContext context,
  required Widget child,
  VoidCallback? onDismiss,
}) {
  void dismiss([T? result]) {
    Navigator.of(context).pop(result);
    onDismiss?.call();
  }

  final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
  if (narrow) {
    return showVoiceBottomSheet<T>(
      context: context,
      initialSize: 0.45,
      minSize: 0.3,
      maxSize: 0.7,
      scrollable: false,
      onDismiss: onDismiss,
      child: child,
    );
  }

  return showDialog<T>(
    context: context,
    builder: (ctx) => VoiceFocusTrap(
      onEscape: () => dismiss(),
      child: child,
    ),
  );
}

class ComposerEmojiPanelBody extends StatelessWidget {
  const ComposerEmojiPanelBody({
    super.key,
    required this.title,
    required this.onSelected,
  });

  static const panelKey = Key('composer_emoji_panel');
  static const choices = ['👍', '❤️', '😂', '😮', '😢', '🎉', '🔥', '👀'];

  final String title;
  final ValueChanged<String> onSelected;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: title,
      child: Padding(
        key: panelKey,
        padding: const EdgeInsets.all(12),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(title, style: Theme.of(context).textTheme.titleSmall),
            const SizedBox(height: 8),
            Wrap(
              alignment: WrapAlignment.center,
              spacing: 4,
              runSpacing: 4,
              children: [
                for (final emoji in choices)
                  Semantics(
                    button: true,
                    label: emoji,
                    child: IconButton(
                      key: Key('composer_emoji_$emoji'),
                      onPressed: () => onSelected(emoji),
                      icon: Text(
                        emoji,
                        style: VoiceEmojiStyle.textStyle(fontSize: 28),
                      ),
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class ComposerAttachMenuBody extends StatelessWidget {
  const ComposerAttachMenuBody({
    super.key,
    required this.onSelected,
    required this.photoOrVideoLabel,
    required this.documentLabel,
  });

  static const menuKey = Key('composer_attach_menu');

  final ValueChanged<ComposerAttachAction> onSelected;
  final String photoOrVideoLabel;
  final String documentLabel;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Theme.of(context).dialogBackgroundColor,
      borderRadius: BorderRadius.circular(12),
      child: Semantics(
        container: true,
        child: Padding(
          key: menuKey,
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ListTile(
                key: const Key('composer_attach_photo_or_video'),
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
            ],
          ),
        ),
      ),
    );
  }
}
