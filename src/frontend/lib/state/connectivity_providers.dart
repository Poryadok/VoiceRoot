import 'dart:async';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// True when the device has no network connectivity.
final isDeviceOfflineProvider = StateProvider<bool>((ref) => false);

/// Starts listening to platform connectivity and updates [isDeviceOfflineProvider].
final connectivityWatcherProvider = Provider<void>((ref) {
  final connectivity = Connectivity();
  StreamSubscription<List<ConnectivityResult>>? subscription;

  void apply(List<ConnectivityResult> results) {
    final offline =
        results.isEmpty ||
        results.every((result) => result == ConnectivityResult.none);
    ref.read(isDeviceOfflineProvider.notifier).state = offline;
  }

  subscription = connectivity.onConnectivityChanged.listen(apply);
  unawaited(
    connectivity.checkConnectivity().then(apply).catchError((_) {
      // Tests or unsupported platforms may fail; keep default online.
    }),
  );

  ref.onDispose(() => subscription?.cancel());
});

bool connectivityResultsAreOffline(List<ConnectivityResult> results) {
  return results.isEmpty ||
      results.every((result) => result == ConnectivityResult.none);
}
