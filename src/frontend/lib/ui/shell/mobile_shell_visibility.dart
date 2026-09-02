/// R2-A04 mobile shell chrome visibility (navigation.md §1.6a).
bool shouldShowMobileShellTabs({
  required bool narrow,
  required bool chatOpen,
  required double keyboardInsetBottom,
}) {
  return narrow && !chatOpen && keyboardInsetBottom <= 0;
}

/// Active strip collapses when the keyboard is open (§1.6a) or a full-screen
/// route covers the shell (matchmaking, settings sub-routes, …).
bool shouldShowMobileChatStrip({
  required bool narrow,
  required bool chatOpen,
  required double keyboardInsetBottom,
  required bool shellOverlayActive,
}) {
  return narrow &&
      chatOpen &&
      keyboardInsetBottom <= 0 &&
      !shellOverlayActive;
}
