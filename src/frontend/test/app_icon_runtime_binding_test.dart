import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/subscription_client.dart';
import 'package:voice_frontend/services/app_icon_runtime_binding.dart';
import 'package:voice_frontend/services/windows_desktop_host.dart';
import 'package:voice_frontend/settings/app_icon_preference.dart';
import 'package:voice_frontend/state/subscription_providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({'voice_app_icon': 'coral'});
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
  });

  tearDown(() => debugDefaultTargetPlatformOverride = null);

  test(
    'restores a Plus icon at startup and fences stale updates after lapse',
    () async {
      var plus = true;
      final host = _ControlledHost();
      final container = _container(
        host,
        subscription: () async => plus ? _plusSubscription() : null,
      );
      addTearDown(container.dispose);
      final runtime = container.read(appIconRuntimeBindingProvider);

      await container.read(appIconPreferenceProvider.future);
      await container.read(subscriptionProvider.future);
      await runtime.applyCurrent();
      expect(host.currentIcon, 'coral');

      host.holdNext = true;
      host.nextCallStarted = Completer<void>();
      final olderApply = runtime.applyCurrent();
      await host.nextCallStarted.future;
      expect(host.requested.last, 'coral');

      plus = false;
      container.invalidate(subscriptionProvider);
      await container.read(subscriptionProvider.future);
      final latestApply = runtime.applyCurrent();
      host.releaseHeldCall.complete();
      await olderApply;
      await latestApply;

      expect(host.currentIcon, 'voice_sky');
      expect(host.requested.last, 'voice_sky');
      expect(
        (await SharedPreferences.getInstance()).getString(
          appIconPreferencePrefKey,
        ),
        'coral',
      );
    },
  );

  test('host failures remain visible and can be retried', () async {
    final host = _ControlledHost();
    final container = _container(
      host,
      subscription: () async => _plusSubscription(),
    );
    addTearDown(container.dispose);
    final runtime = container.read(appIconRuntimeBindingProvider);
    await container.read(appIconPreferenceProvider.future);
    await container.read(subscriptionProvider.future);
    host.failNext = true;

    await runtime.applyCurrent();
    expect(runtime.failed, isTrue);

    await runtime.retryCurrent();
    expect(runtime.failed, isFalse);
    expect(host.currentIcon, 'coral');
  });
}

ProviderContainer _container(
  WindowsDesktopHost host, {
  required Future<VoiceSubscription?> Function() subscription,
}) => ProviderContainer(
  overrides: [
    windowsDesktopHostProvider.overrideWithValue(host),
    subscriptionProvider.overrideWith((ref) => subscription()),
  ],
);

VoiceSubscription _plusSubscription() => const VoiceSubscription(
  id: 'subscription-1',
  accountId: 'account-1',
  plan: 'premium',
  billingPeriod: 'monthly',
  status: 'active',
);

class _ControlledHost extends RecordingWindowsDesktopHost {
  final requested = <String>[];
  String? currentIcon;
  bool holdNext = false;
  bool failNext = false;
  Completer<void> nextCallStarted = Completer<void>();
  final releaseHeldCall = Completer<void>();

  @override
  Future<void> setAppIcon(String iconId) async {
    requested.add(iconId);
    if (!nextCallStarted.isCompleted) nextCallStarted.complete();
    if (holdNext) {
      holdNext = false;
      await releaseHeldCall.future;
    }
    if (failNext) {
      failNext = false;
      throw StateError('fixture host failure');
    }
    currentIcon = iconId;
  }
}
