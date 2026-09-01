import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/state/mobile_opened_chat_strip.dart';

void main() {
  group('MobileOpenedChatStripController', () {
    test('openChat moves chat to front', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      final ctrl = container.read(mobileOpenedChatStripProvider.notifier);

      ctrl.openChat('a');
      ctrl.openChat('b');
      ctrl.openChat('c');
      expect(container.read(mobileOpenedChatStripProvider), ['c', 'b', 'a']);

      ctrl.openChat('a');
      expect(container.read(mobileOpenedChatStripProvider), ['a', 'c', 'b']);
    });

    test('openChat evicts oldest when over limit', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      final ctrl = container.read(mobileOpenedChatStripProvider.notifier);

      for (var i = 0; i < kMobileOpenedChatStripLimit; i++) {
        ctrl.openChat('chat-$i');
      }
      expect(container.read(mobileOpenedChatStripProvider).length,
          kMobileOpenedChatStripLimit);

      ctrl.openChat('chat-new');
      final ids = container.read(mobileOpenedChatStripProvider);
      expect(ids.first, 'chat-new');
      expect(ids.length, kMobileOpenedChatStripLimit);
      expect(ids, isNot(contains('chat-0')));
    });

    test('removeChat drops id from strip', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      final ctrl = container.read(mobileOpenedChatStripProvider.notifier);

      ctrl.openChat('a');
      ctrl.openChat('b');
      ctrl.removeChat('b');
      expect(container.read(mobileOpenedChatStripProvider), ['a']);
    });

    test('clear resets strip', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      final ctrl = container.read(mobileOpenedChatStripProvider.notifier);

      ctrl.openChat('a');
      ctrl.clear();
      expect(container.read(mobileOpenedChatStripProvider), isEmpty);
    });
  });
}
