import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../settings/app_icon_preference.dart';
import '../state/subscription_providers.dart';
import 'app_shell_icon.dart';
import 'windows_desktop_host.dart';

final appIconRuntimeBindingProvider =
    ChangeNotifierProvider<AppIconRuntimeBinding>((ref) {
      final binding = AppIconRuntimeBinding(ref);
      ref.listen(
        appIconPreferenceProvider,
        (_, _) => unawaited(binding.applyCurrent()),
      );
      ref.listen(
        subscriptionProvider,
        (_, _) => unawaited(binding.applyCurrent()),
      );
      unawaited(binding.applyCurrent());
      return binding;
    });

class AppIconRuntimeBinding extends ChangeNotifier {
  AppIconRuntimeBinding(this._ref);

  final Ref _ref;
  Future<void> _queue = Future<void>.value();
  int _requestGeneration = 0;
  bool _applying = false;
  bool _failed = false;
  AppIconPreference? _applied;

  bool get applying => _applying;
  bool get failed => _failed;
  AppIconPreference? get applied => _applied;
  bool get supported =>
      kIsWeb || defaultTargetPlatform == TargetPlatform.windows;

  Future<void> applySelection(AppIconPreference selected) async {
    final entitlement = _ref.read(subscriptionProvider);
    if (selected.requiresPlus && entitlement.valueOrNull?.isPremium != true) {
      throw StateError(
        'An active or grace-period Voice Plus subscription is required',
      );
    }
    try {
      await _ref
          .read(appIconPreferenceProvider.notifier)
          .setPreference(selected);
      await _enqueue(_effectiveSelection(selected));
    } on Object {
      _failed = true;
      notifyListeners();
    }
  }

  Future<void> retryCurrent() async {
    if (_ref.read(appIconPreferenceProvider).hasError) {
      await _ref.read(appIconPreferenceProvider.notifier).retrySave();
    }
    await applyCurrent();
  }

  Future<void> applyCurrent() {
    if (!supported) {
      _applying = false;
      _failed = false;
      return Future<void>.value();
    }
    final selected =
        _ref.read(appIconPreferenceProvider).valueOrNull ??
        AppIconPreference.voiceSky;
    return _enqueue(_effectiveSelection(selected));
  }

  AppIconPreference _effectiveSelection(AppIconPreference selected) {
    final plus = _ref.read(subscriptionProvider).valueOrNull?.isPremium == true;
    return effectiveAppIcon(selected, isPlusActiveOrGrace: plus);
  }

  Future<void> _enqueue(AppIconPreference icon) {
    final generation = ++_requestGeneration;
    _applying = true;
    _failed = false;
    notifyListeners();

    final operation = _queue.then((_) => _applyHostIcon(icon));
    _queue = operation.catchError((_) {});
    return operation.then<void>(
      (_) {
        if (generation != _requestGeneration) return;
        _applied = icon;
        _applying = false;
        _failed = false;
        notifyListeners();
      },
      onError: (Object error, StackTrace stackTrace) {
        if (generation == _requestGeneration) {
          _applying = false;
          _failed = true;
          notifyListeners();
        }
        // The screen observes [failed] and offers an explicit retry. Startup and
        // entitlement listeners must not leak asynchronous host failures.
      },
    );
  }

  Future<void> _applyHostIcon(AppIconPreference icon) async {
    if (kIsWeb) {
      if (!supportsWebAppShellIcon) {
        throw UnsupportedError('Runtime browser tab icons are unavailable');
      }
      await applyWebAppShellIcon(icon);
      return;
    }
    if (defaultTargetPlatform == TargetPlatform.windows) {
      final host = _ref.read(windowsDesktopHostProvider);
      if (!host.supportsAppIcon) {
        throw UnsupportedError('Runtime Windows app icons are unavailable');
      }
      await host.setAppIcon(icon.id);
      return;
    }
    throw UnsupportedError('Runtime app icons are unavailable on this host');
  }
}
