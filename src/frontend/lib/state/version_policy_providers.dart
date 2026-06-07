import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/client_version.dart';
import '../backend/gateway_api_error.dart';
import 'gateway_providers.dart';

enum VersionPolicyPhase { ok, softUpdate, forceUpdate }

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
  VersionPolicyController(this._ref) : super(const VersionPolicyState()) {
    if (!kIsWeb) {
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
      } else if (decoded['update_available'] == true) {
        state = VersionPolicyState(
          phase: VersionPolicyPhase.softUpdate,
          latestVersion: decoded['latest_version'] as String?,
          updateUrl: decoded['update_url'] as String?,
          releaseNotes: decoded['release_notes'] as String?,
        );
      } else {
        state = const VersionPolicyState();
      }
    } catch (_) {
      // ignore malformed policy
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
    if (state.phase == VersionPolicyPhase.softUpdate) {
      state = const VersionPolicyState();
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
