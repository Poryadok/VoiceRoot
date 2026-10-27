import 'package:flutter/material.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../a11y/focus_trap.dart';
import '../a11y/voice_focus_return.dart';

enum ComposerSendAction { silent, schedule, online }

/// Send options (§3.6c): desktop popup above Send, mobile bottom sheet.
Future<ComposerSendAction?> showComposerSendMenu(
  BuildContext context, {
  required bool isDm,
  required bool scheduledMessagesEnabled,
  BuildContext? anchorContext,
}) async {
  final focusReturn = VoiceFocusReturn.capture();
  final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
  Widget body(BuildContext routeContext) => VoiceFocusTrap(
    onEscape: () => Navigator.of(routeContext).pop(),
    child: SingleChildScrollView(
      child: ComposerSendMenu(
        isDm: isDm,
        scheduledMessagesEnabled: scheduledMessagesEnabled,
        mobile: narrow,
        onSelected: (action) => Navigator.of(routeContext).pop(action),
      ),
    ),
  );
  try {
    if (narrow) {
      return await showModalBottomSheet<ComposerSendAction>(
        context: context,
        isScrollControlled: true,
        backgroundColor: Colors.transparent,
        builder: (routeContext) => SafeArea(child: body(routeContext)),
      );
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
    return await showDialog<ComposerSendAction>(
      context: context,
      useSafeArea: false,
      barrierColor: Colors.transparent,
      builder: (routeContext) => CustomSingleChildLayout(
        delegate: _SendMenuLayout(anchor, MediaQuery.paddingOf(routeContext)),
        child: body(routeContext),
      ),
    );
  } finally {
    focusReturn.restore();
  }
}

class ComposerSendMenu extends StatelessWidget {
  const ComposerSendMenu({
    super.key,
    required this.isDm,
    required this.scheduledMessagesEnabled,
    required this.onSelected,
    this.mobile = false,
  });

  static const menuKey = Key('composer_send_menu');
  final bool isDm;
  final bool scheduledMessagesEnabled;
  final bool mobile;
  final ValueChanged<ComposerSendAction> onSelected;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    return Material(
      key: menuKey,
      color: colors.canvas,
      elevation: mobile ? 0 : 8,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(6),
        side: BorderSide(color: colors.borderDefault),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (mobile) ...[
              Center(
                child: Container(
                  width: 36,
                  height: 4,
                  decoration: BoxDecoration(
                    color: colors.borderDefault,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.all(16),
                child: Text(
                  l10n.composerSendOptions,
                  style: Theme.of(context).textTheme.titleSmall,
                ),
              ),
            ],
            ListTile(
              key: const Key('composer_send_silent'),
              autofocus: true,
              leading: const Icon(Icons.dark_mode_outlined, size: 20),
              title: Text(l10n.composerSendSilent),
              onTap: () => onSelected(ComposerSendAction.silent),
            ),
            if (scheduledMessagesEnabled) ...[
              ListTile(
                key: const Key('composer_send_schedule'),
                leading: const Icon(Icons.schedule, size: 20),
                title: Text(l10n.composerSendSchedule),
                onTap: () => onSelected(ComposerSendAction.schedule),
              ),
              if (isDm)
                ListTile(
                  key: const Key('composer_send_online'),
                  leading: const Icon(Icons.online_prediction, size: 20),
                  title: Text(l10n.composerSendOnline),
                  onTap: () => onSelected(ComposerSendAction.online),
                ),
              if (mobile && !isDm)
                Padding(
                  padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                  child: Text(
                    l10n.composerSendOnlineDmOnly,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: colors.textSecondary,
                    ),
                  ),
                ),
            ],
          ],
        ),
      ),
    );
  }
}

class _SendMenuLayout extends SingleChildLayoutDelegate {
  const _SendMenuLayout(this.anchor, this.padding);
  final Rect anchor;
  final EdgeInsets padding;
  static const margin = 8.0;
  @override
  BoxConstraints getConstraintsForChild(BoxConstraints constraints) {
    final width = (constraints.maxWidth - padding.horizontal - margin * 2)
        .clamp(0.0, 260.0);
    return BoxConstraints.tightFor(width: width).copyWith(
      maxHeight: (constraints.maxHeight - padding.vertical - margin * 2).clamp(
        0.0,
        double.infinity,
      ),
    );
  }

  @override
  Offset getPositionForChild(Size size, Size childSize) => Offset(
    (anchor.right - childSize.width).clamp(
      padding.left + margin,
      size.width - padding.right - margin - childSize.width,
    ),
    (anchor.top - margin - childSize.height).clamp(
      padding.top + margin,
      size.height - padding.bottom - margin - childSize.height,
    ),
  );
  @override
  bool shouldRelayout(_SendMenuLayout oldDelegate) =>
      anchor != oldDelegate.anchor || padding != oldDelegate.padding;
}
