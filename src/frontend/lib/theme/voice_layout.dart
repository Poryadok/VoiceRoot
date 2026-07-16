/// Shared responsive layout breakpoints ([docs/design/brand.md]).
abstract final class VoiceLayout {
  /// Width below which the shell uses mobile navigation patterns.
  static const double narrowBreakpoint = 600;

  /// Penpot `Screen/Shell/Desktop` column widths ([screens.md](../../docs/design/screens.md)).
  static const double desktopNavigationWidth = 320;
  static const double desktopChatListWidth = 320;
  static const double desktopSidePanelWidth = 300;
  static const double authFormMaxWidth = 400;

  /// Minimum touch target per brand.md mobile density rules.
  static const double minTouchTarget = 44;

  /// Height of the horizontal mobile chat icon strip.
  static const double mobileStripHeight = 52;

  static bool isNarrow(double width) => width < narrowBreakpoint;
}
