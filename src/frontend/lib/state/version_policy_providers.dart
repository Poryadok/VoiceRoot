import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/client_version.dart';
import '../backend/gateway_api_error.dart';
import '../services/desktop_updater_service.dart';
import 'gateway_providers.dart';

enum VersionPolicyPhase {
  ok,
  softUpdate,
  forceUpdate,
  desktopReadyToRestart,
}

class VersionPolicyState {
  const VersionPolicyState({
    this.phase = VersionPolicyPhase.ok,
    this.latestVersion,
    this.updateUrl,
    this.releaseNotes,
  });

  final VersionPolicyPhase phase;
  final String? latestVersion;
  final String? updateUrl;
  final String? releaseNotes;

  VersionPolicyState copyWith({
    VersionPolicyPhase? phase,
    String? latestVersion,
    String? updateUrl,
    String? releaseNotes,
  }) {
    return VersionPolicyState(
      phase: phase ?? this.phase,
      latestVersion: latestVersion ?? this.latestVersion,
      updateUrl: updateUrl ?? this.updateUrl,
      releaseNotes: releaseNotes ?? this.releaseNotes,
    );
  }
}

final versionPolicyProvider =
    StateNotifierProvider<VersionPolicyController, VersionPolicyState>((ref) {
      return VersionPolicyController(ref);
    });

class VersionPolicyController extends StateNotifier<VersionPolicyState> {
  VersionPolicyController(this._ref, {bool enablePolling = true})
    : super(const VersionPolicyState()) {
    if (!kIsWeb && enablePolling) {
      unawaited(refresh());
      _timer = Timer.periodic(const Duration(hours: 4), (_) => refresh());
    }
  }

  final Ref _ref;
  Timer? _timer;

  Future<void> refresh() async {
    if (kIsWeb || !ClientVersion.sendVersionHeaders) return;
    final client = _ref.read(voiceGatewayClientProvider);
    final body = await client.fetchVersionBody(
      platform: ClientVersion.platform,
      version: ClientVersion.appVersion,
    );
    if (body == null) return;
    try {
      final decoded = _parseVersionJson(body);
      if (decoded['force_update'] == true) {
        state = VersionPolicyState(
          phase: VersionPolicyPhase.forceUpdate,
          latestVersion: decoded['latest_version'] as String?,
          updateUrl: decoded['update_url'] as String?,
          releaseNotes: decoded['release_notes'] as String?,
        );
        await _maybeStartDesktopUpdate(state);
      } else if (decoded['update_available'] == true) {
        final next = VersionPolicyState(
          phase: VersionPolicyPhase.softUpdate,
          latestVersion: decoded['latest_version'] as String?,
          updateUrl: decoded['update_url'] as String?,
          releaseNotes: decoded['release_notes'] as String?,
        );
        state = next;
        await _maybeStartDesktopUpdate(next);
      } else {
        state = const VersionPolicyState();
      }
    } catch (_) {
      // ignore malformed policy
    }
  }

  Future<void> _maybeStartDesktopUpdate(VersionPolicyState policy) async {
    if (!ClientVersion.usesDesktopAutoUpdater) return;
    final updateUrl = policy.updateUrl;
    if (updateUrl == null || updateUrl.isEmpty) return;
    final updater = _ref.read(desktopUpdaterServiceProvider);
    final status = await updater.checkForUpdate(updateUrl);
    if (status == DesktopUpdateStatus.downloading ||
        status == DesktopUpdateStatus.readyToRestart) {
      state = policy.copyWith(phase: VersionPolicyPhase.desktopReadyToRestart);
    }
  }

  void onGatewayUpgradeRequired(GatewayApiError error) {
    if (error.errorCode != 'client_outdated') return;
    state = VersionPolicyState(
      phase: VersionPolicyPhase.forceUpdate,
      updateUrl: _updateUrlFromError(error),
      releaseNotes: error.message,
    );
  }

  void dismissSoftUpdate() {
    if (state.phase == VersionPolicyPhase.softUpdate ||
        state.phase == VersionPolicyPhase.desktopReadyToRestart) {
      state = const VersionPolicyState();
    }
  }

  Future<void> restartDesktopUpdate() async {
    await _ref.read(desktopUpdaterServiceProvider).restartAndApply();
  }

  void markDesktopReadyToRestart() {
    if (state.phase == VersionPolicyPhase.softUpdate ||
        state.phase == VersionPolicyPhase.forceUpdate) {
      state = state.copyWith(phase: VersionPolicyPhase.desktopReadyToRestart);
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  static Map<String, dynamic> _parseVersionJson(String body) {
    return jsonDecode(body) as Map<String, dynamic>;
  }

  static String? _updateUrlFromError(GatewayApiError error) => error.updateUrl;
}
