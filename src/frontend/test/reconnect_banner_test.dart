import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/state/chat_providers.dart';

void main() {
  group('reconnectBannerVisibleProvider', () {
    ProviderContainer newContainer() {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      container.listen(reconnectBannerVisibleProvider, (_, _) {});
      return container;
    }

    test('shows banner 2s after disconnect from connected', () {
      fakeAsync((async) {
        final container = newContainer();

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        async.elapse(Duration.zero);
        expect(container.read(reconnectBannerVisibleProvider), isFalse);

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;
        async.elapse(const Duration(seconds: 1));
        expect(container.read(reconnectBannerVisibleProvider), isFalse);

        async.elapse(const Duration(seconds: 1));
        expect(container.read(reconnectBannerVisibleProvider), isTrue);
      });
    });

    test('hides banner 1s after successful reconnect', () {
      fakeAsync((async) {
        final container = newContainer();

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;
        async.elapse(reconnectBannerShowDelay);
        expect(container.read(reconnectBannerVisibleProvider), isTrue);

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        async.elapse(const Duration(milliseconds: 500));
        expect(container.read(reconnectBannerVisibleProvider), isTrue);

        async.elapse(const Duration(milliseconds: 500));
        expect(container.read(reconnectBannerVisibleProvider), isFalse);
      });
    });

    test('does not show banner when reconnect succeeds within 2s', () {
      fakeAsync((async) {
        final container = newContainer();

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;
        async.elapse(const Duration(seconds: 1));
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        async.elapse(const Duration(seconds: 2));

        expect(container.read(reconnectBannerVisibleProvider), isFalse);
      });
    });

    test('does not show banner during initial connect before first hello', () {
      fakeAsync((async) {
        final container = newContainer();

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connecting;
        async.elapse(reconnectBannerShowDelay);
        expect(container.read(reconnectBannerVisibleProvider), isFalse);

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        async.elapse(Duration.zero);
        expect(container.read(reconnectBannerVisibleProvider), isFalse);
      });
    });
  });
}
