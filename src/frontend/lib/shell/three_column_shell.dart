import 'package:flutter/material.dart';

import '../theme/voice_colors.dart';
import '../theme/voice_layout.dart';

/// Desktop shell per docs/features/navigation.md.
/// Supports legacy 3-column (rail | list | main) and extended layout with
/// navigation column, optional space tree, chat, and side panel.
class ThreeColumnShell extends StatelessWidget {
  const ThreeColumnShell({
    super.key,
    this.header,
    this.railChild,
    this.listChild,
    this.mainChild,
    this.listFlex = 1,
    this.mainFlex = 2,
    this.showMainOnlyOnNarrow = false,
    this.mobileRailChild,
    this.navigationChild,
    this.navigationCollapsed = false,
    this.middleChild,
    this.sidePanelChild,
    this.navigationExpandedWidth = VoiceLayout.desktopNavigationWidth,
    this.navigationCollapsedWidth = 72,
    this.sidePanelWidth = VoiceLayout.desktopSidePanelWidth,
  });

  final Widget? header;
  final Widget? railChild;
  final Widget? listChild;
  final Widget? mainChild;
  final int listFlex;
  final int mainFlex;
  final bool showMainOnlyOnNarrow;
  final Widget? mobileRailChild;

  /// When set, enables the new shell layout (navigation | middle? | main | side?).
  final Widget? navigationChild;
  final bool navigationCollapsed;
  final Widget? middleChild;
  final Widget? sidePanelChild;
  final double navigationExpandedWidth;
  final double navigationCollapsedWidth;
  final double sidePanelWidth;

  static const Key navActiveRail = Key('nav_active_rail');
  static const Key navChatList = Key('nav_chat_list');
  static const Key navOpenChat = Key('nav_open_chat');
  static const Key navSpaceTree = Key('nav_space_tree');
  static const Key navSidePanel = Key('nav_side_panel');
  static const Key navMobileStack = Key('nav_mobile_stack');
  static const Key navMobileStrip = Key('nav_mobile_strip');

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);

    Widget columnDivider() =>
        VerticalDivider(width: 1, color: voice.borderDefault);

    Widget mainContent() => ColoredBox(
      color: voice.canvas,
      child: mainChild ?? const SizedBox.expand(),
    );

    Widget legacyRail() =>
        railChild ??
        ColoredBox(color: voice.muted, child: const SizedBox.expand());

    Widget legacyList() => ColoredBox(
      color: voice.surface,
      child: listChild ?? const SizedBox.expand(),
    );

    Widget navigationColumn() {
      final width = navigationCollapsed
          ? navigationCollapsedWidth
          : navigationExpandedWidth;
      return Semantics(
        label: 'Navigation',
        container: true,
        explicitChildNodes: true,
        child: SizedBox(
          key: navActiveRail,
          width: width,
          child: navigationChild,
        ),
      );
    }

    Widget middleColumn() => Expanded(
      flex: listFlex,
      child: Semantics(
        label: 'Chat list',
        container: true,
        explicitChildNodes: true,
        child: ColoredBox(
          key: navSpaceTree,
          color: voice.surface,
          child: middleChild!,
        ),
      ),
    );

    Widget legacyListColumn() => Expanded(
      flex: listFlex,
      child: Semantics(
        label: 'Chat list',
        container: true,
        explicitChildNodes: true,
        child: ColoredBox(
          key: navChatList,
          color: voice.surface,
          child: listChild ?? const SizedBox.expand(),
        ),
      ),
    );

    Widget mainColumn() => Expanded(
      flex: mainFlex,
      child: Semantics(
        label: 'Conversation',
        container: true,
        explicitChildNodes: true,
        child: ColoredBox(
          key: navOpenChat,
          color: voice.canvas,
          child: mainChild ?? const SizedBox.expand(),
        ),
      ),
    );

    Widget sideColumn() => SizedBox(
      key: navSidePanel,
      width: sidePanelWidth,
      child: ColoredBox(
        color: voice.surface,
        child: sidePanelChild!,
      ),
    );

    Widget desktopBody({required bool narrow}) {
      if (navigationChild != null) {
        final children = <Widget>[
          navigationColumn(),
          columnDivider(),
        ];
        if (middleChild != null) {
          children.add(middleColumn());
          children.add(columnDivider());
        }
        children.add(mainColumn());
        if (sidePanelChild != null) {
          children.add(columnDivider());
          children.add(sideColumn());
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: children,
        );
      }

      return Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(key: navActiveRail, child: legacyRail()),
          columnDivider(),
          legacyListColumn(),
          columnDivider(),
          mainColumn(),
        ],
      );
    }

    Widget narrowBody() {
      if (navigationChild != null) {
        if (showMainOnlyOnNarrow) {
          return Column(
            key: navMobileStack,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              SizedBox(
                key: navMobileStrip,
                height: VoiceLayout.mobileStripHeight,
                child: mobileRailChild ?? navigationColumn(),
              ),
              Expanded(
                child: Semantics(
                  label: 'Conversation',
                  container: true,
                  explicitChildNodes: true,
                  child: ColoredBox(
                    key: navOpenChat,
                    color: voice.canvas,
                    child: mainContent(),
                  ),
                ),
              ),
            ],
          );
        }
        if (middleChild != null && !showMainOnlyOnNarrow) {
          return Column(
            key: navMobileStack,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              SizedBox(
                key: navMobileStrip,
                height: VoiceLayout.mobileStripHeight,
                child: mobileRailChild ?? navigationColumn(),
              ),
              Divider(height: 1, color: voice.borderDefault),
              Expanded(key: navSpaceTree, child: middleChild!),
            ],
          );
        }
        return Column(
          key: navMobileStack,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Expanded(key: navActiveRail, child: navigationChild!),
            if (mainChild != null && showMainOnlyOnNarrow) ...[
              Divider(height: 1, color: voice.borderDefault),
              Expanded(
                child: Semantics(
                  label: 'Conversation',
                  container: true,
                  explicitChildNodes: true,
                  child: ColoredBox(
                    key: navOpenChat,
                    color: voice.canvas,
                    child: mainContent(),
                  ),
                ),
              ),
            ],
          ],
        );
      }

      return Column(
        key: navMobileStack,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: showMainOnlyOnNarrow
            ? [
                SizedBox(
                  key: navMobileStrip,
                  height: VoiceLayout.mobileStripHeight,
                  child: mobileRailChild ?? legacyRail(),
                ),
                Expanded(
                  child: Semantics(
                    label: 'Conversation',
                    container: true,
                    explicitChildNodes: true,
                    child: ColoredBox(
                      key: navOpenChat,
                      color: voice.canvas,
                      child: mainContent(),
                    ),
                  ),
                ),
              ]
            : [
                SizedBox(
                  key: navActiveRail,
                  height: VoiceLayout.mobileStripHeight,
                  child: mobileRailChild ?? legacyRail(),
                ),
                Divider(height: 1, color: voice.borderDefault),
                Expanded(key: navChatList, child: legacyList()),
              ],
      );
    }

    return LayoutBuilder(
      builder: (context, c) {
        final narrow = VoiceLayout.isNarrow(c.maxWidth);
        final body = narrow ? narrowBody() : desktopBody(narrow: narrow);

        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            ?header,
            if (header != null) Divider(height: 1, color: voice.borderDefault),
            Expanded(child: body),
          ],
        );
      },
    );
  }
}
