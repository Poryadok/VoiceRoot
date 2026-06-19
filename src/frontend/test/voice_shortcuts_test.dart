import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/state/shell_providers.dart';

void main() {
  test('composer and search focus request providers bump', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    expect(container.read(composerFocusRequestProvider), 0);
    container.read(composerFocusRequestProvider.notifier).state++;
    expect(container.read(composerFocusRequestProvider), 1);

    expect(container.read(globalSearchFocusRequestProvider), 0);
    container.read(globalSearchFocusRequestProvider.notifier).state++;
    expect(container.read(globalSearchFocusRequestProvider), 1);
  });
}
