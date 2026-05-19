import 'package:flutter/material.dart';

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
  });

  final Widget? header;
  final Widget? railChild;
  final Widget? listChild;
  final Widget? mainChild;
  final int listFlex;
  final int mainFlex;

  static const Key navActiveRail = Key('nav_active_rail');
  static const Key navChatList = Key('nav_chat_list');
  static const Key navOpenChat = Key('nav_open_chat');
  static const Key navMobileStack = Key('nav_mobile_stack');

  @override
  Widget build(BuildContext context) {
    Widget rail() => Expanded(
          flex: 1,
          key: navActiveRail,
          child: railChild ??
              const ColoredBox(
                color: Color(0x12000000),
                child: SizedBox.expand(),
              ),
        );

    Widget list() => Expanded(
          flex: listFlex,
          key: navChatList,
          child: listChild ??
              const ColoredBox(
                color: Color(0x24000000),
                child: SizedBox.expand(),
              ),
        );

    Widget main() => Expanded(
          flex: mainFlex,
          key: navOpenChat,
          child: mainChild ??
              const ColoredBox(
                color: Color(0x36000000),
                child: SizedBox.expand(),
              ),
        );

    return LayoutBuilder(
      builder: (context, c) {
        final narrow = c.maxWidth < 600;
        final body = narrow
            ? Column(
                key: navMobileStack,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  rail(),
                  list(),
                  main(),
                ],
              )
            : Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  rail(),
                  list(),
                  main(),
                ],
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
