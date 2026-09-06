import 'dart:async';

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

    ProviderContainer newContainerWithInitialStatus(RealtimeLinkStatus status) {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      container.read(realtimeLinkStatusProvider.notifier).state = status;
      container.listen(reconnectBannerVisibleProvider, (_, _) {});
      return container;
    }

    test('initial connected status is hidden without a provider error', () {
      fakeAsync((async) {
        final errors = <Object>[];
        ProviderContainer? container;
        runZonedGuarded(() {
          container = newContainerWithInitialStatus(
            RealtimeLinkStatus.connected,
          );
          async.elapse(Duration.zero);
        }, (error, _) => errors.add(error));

        expect(errors, isEmpty);
        expect(container!.read(reconnectBannerVisibleProvider), isFalse);
      });
    });

    test(
      'initial reconnecting shows after 2s then hides 1s after connected',
      () {
        fakeAsync((async) {
          final errors = <Object>[];
          ProviderContainer? container;
          runZonedGuarded(() {
            container = newContainerWithInitialStatus(
              RealtimeLinkStatus.reconnecting,
            );
            async.elapse(const Duration(seconds: 1));
            expect(container!.read(reconnectBannerVisibleProvider), isFalse);
            async.elapse(const Duration(seconds: 1));
            expect(container!.read(reconnectBannerVisibleProvider), isTrue);
            container!.read(realtimeLinkStatusProvider.notifier).state =
                RealtimeLinkStatus.connected;
            async.elapse(const Duration(milliseconds: 999));
            expect(container!.read(reconnectBannerVisibleProvider), isTrue);
            async.elapse(const Duration(milliseconds: 1));
            expect(container!.read(reconnectBannerVisibleProvider), isFalse);
          }, (error, _) => errors.add(error));

          expect(errors, isEmpty);
        });
      },
    );

    for (final status in [
      RealtimeLinkStatus.connecting,
      RealtimeLinkStatus.reconnecting,
    ]) {
      test(
        'initial connected then immediate $status retains connection history',
        () {
          fakeAsync((async) {
            final errors = <Object>[];
            late ProviderContainer container;
            runZonedGuarded(() {
              container = newContainerWithInitialStatus(
                RealtimeLinkStatus.connected,
              );
              container.read(realtimeLinkStatusProvider.notifier).state =
                  status;
              async.elapse(const Duration(seconds: 1));
              expect(container.read(reconnectBannerVisibleProvider), isFalse);
              async.elapse(const Duration(seconds: 1));
              expect(container.read(reconnectBannerVisibleProvider), isTrue);
            }, (error, _) => errors.add(error));

            expect(errors, isEmpty);
          });
        },
      );
    }

    test(
      'initial disconnected and connecting remain hidden without provider errors',
      () {
        fakeAsync((async) {
          for (final status in [
            RealtimeLinkStatus.disconnected,
            RealtimeLinkStatus.connecting,
          ]) {
            final errors = <Object>[];
            ProviderContainer? container;
            runZonedGuarded(() {
              container = newContainerWithInitialStatus(status);
              async.elapse(reconnectBannerShowDelay);
            }, (error, _) => errors.add(error));
            expect(errors, isEmpty);
            expect(container!.read(reconnectBannerVisibleProvider), isFalse);
          }
        });
      },
    );

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
