/// Shared responsive layout breakpoints ([docs/design/brand.md]).
abstract final class VoiceLayout {
  /// Width below which the shell uses mobile navigation patterns.
  static const double narrowBreakpoint = 600;

  static bool isNarrow(double width) => width < narrowBreakpoint;
}
