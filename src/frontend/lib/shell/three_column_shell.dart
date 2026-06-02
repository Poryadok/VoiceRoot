import 'package:flutter/material.dart';

import '../theme/voice_colors.dart';

/// Desktop: [active rail | chat list | open chat] per docs/features/navigation.md.
/// Narrow width: stacked column ([navMobileStack]) preserving the same child keys.
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
  });

  final Widget? header;
  final Widget? railChild;
  final Widget? listChild;
  final Widget? mainChild;
  final int listFlex;
  final int mainFlex;
  final bool showMainOnlyOnNarrow;
  final Widget? mobileRailChild;

  static const Key navActiveRail = Key('nav_active_rail');
  static const Key navChatList = Key('nav_chat_list');
  static const Key navOpenChat = Key('nav_open_chat');
  static const Key navMobileStack = Key('nav_mobile_stack');
  static const Key navMobileStrip = Key('nav_mobile_strip');

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);

    Widget railContent() =>
        railChild ??
        ColoredBox(color: voice.muted, child: const SizedBox.expand());

    Widget listContent() =>
        listChild ??
        ColoredBox(color: voice.surface, child: const SizedBox.expand());

    Widget mainContent() =>
        mainChild ??
        ColoredBox(color: voice.canvas, child: const SizedBox.expand());

    Widget rail() =>
        Expanded(flex: 1, key: navActiveRail, child: railContent());

    Widget list() =>
        Expanded(flex: listFlex, key: navChatList, child: listContent());

    Widget main() =>
        Expanded(flex: mainFlex, key: navOpenChat, child: mainContent());

    return LayoutBuilder(
      builder: (context, c) {
        final narrow = c.maxWidth < 600;
        final body = narrow
            ? Column(
                key: navMobileStack,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: showMainOnlyOnNarrow
                    ? [
                        SizedBox(
                          key: navMobileStrip,
                          height: 48,
                          child: mobileRailChild ?? railContent(),
                        ),
                        Expanded(key: navOpenChat, child: mainContent()),
                      ]
                    : [
                        SizedBox(
                          key: navActiveRail,
                          height: 48,
                          child: mobileRailChild ?? railContent(),
                        ),
                        Expanded(key: navChatList, child: listContent()),
                      ],
              )
            : Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [rail(), list(), main()],
              );

        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            ?header,
            Expanded(child: body),
          ],
        );
      },
    );
  }
}
