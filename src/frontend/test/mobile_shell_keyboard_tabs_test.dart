import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/shell/mobile_shell_visibility.dart';

void main() {
  test('shouldShowMobileShellTabs hides tabs when keyboard is open', () {
    expect(
      shouldShowMobileShellTabs(
        narrow: true,
        chatOpen: false,
        keyboardInsetBottom: 0,
      ),
      isTrue,
    );
    expect(
      shouldShowMobileShellTabs(
        narrow: true,
        chatOpen: false,
        keyboardInsetBottom: 300,
      ),
      isFalse,
    );
  });

  test('shouldShowMobileShellTabs hides tabs when chat is open or wide', () {
    expect(
      shouldShowMobileShellTabs(
        narrow: true,
        chatOpen: true,
        keyboardInsetBottom: 0,
      ),
      isFalse,
    );
    expect(
      shouldShowMobileShellTabs(
        narrow: false,
        chatOpen: false,
        keyboardInsetBottom: 0,
      ),
      isFalse,
    );
  });

  test('shouldShowMobileChatStrip hides strip when keyboard is open', () {
    expect(
      shouldShowMobileChatStrip(
        narrow: true,
        chatOpen: true,
        keyboardInsetBottom: 0,
        shellOverlayActive: false,
      ),
      isTrue,
    );
    expect(
      shouldShowMobileChatStrip(
        narrow: true,
        chatOpen: true,
        keyboardInsetBottom: 280,
        shellOverlayActive: false,
      ),
      isFalse,
    );
  });

  test('shouldShowMobileChatStrip requires narrow open chat', () {
    expect(
      shouldShowMobileChatStrip(
        narrow: false,
        chatOpen: true,
        keyboardInsetBottom: 0,
        shellOverlayActive: false,
      ),
      isFalse,
    );
    expect(
      shouldShowMobileChatStrip(
        narrow: true,
        chatOpen: false,
        keyboardInsetBottom: 0,
        shellOverlayActive: false,
      ),
      isFalse,
    );
  });

  test('shouldShowMobileChatStrip hides strip on full-screen shell overlay', () {
    expect(
      shouldShowMobileChatStrip(
        narrow: true,
        chatOpen: true,
        keyboardInsetBottom: 0,
        shellOverlayActive: true,
      ),
      isFalse,
    );
  });
}
