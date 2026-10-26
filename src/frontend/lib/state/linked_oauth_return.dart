import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';

import '../backend/auth_session.dart';
import '../backend/gateway_http.dart';
import '../backend/linked_oauth_client.dart';
export '../backend/linked_oauth_client.dart' show LinkedOAuthOutcome;

/// Safe URL for history replacement/telemetry/OS dispatch. Preserve explicit #
/// so a subsequent navigation cannot inherit a provider fragment.
Uri stripLinkedOAuthReturnSecrets(Uri uri) => Uri.parse(
  uri.toString().split(RegExp(r'[?#]')).first,
).replace(fragment: '');

/// Must be tab-scoped (sessionStorage on web), never shared preferences/localStorage.
abstract interface class LinkedOAuthTabStore {
  String? read();
  void write(String value);
  void clear();
}

enum LinkedOAuthPhase {
  idle,
  starting,
  awaitingReturn,
  consuming,
  complete,
  failed,
}

@immutable
class LinkedOAuthReturnState {
  const LinkedOAuthReturnState(this.phase, {this.outcome, this.errorCode});
  final LinkedOAuthPhase phase;
  final LinkedOAuthOutcome? outcome;
  final String? errorCode;
}

/// Settings owns presentation and authoritative account/User rereads.
/// Call bindSession for every auth change, including logout and token refresh.
/// Call handleReturn before telemetry, routing, or OS forwarding of the raw URI.
class LinkedOAuthReturnController extends ChangeNotifier {
  LinkedOAuthReturnController({
    required LinkedOAuthClient client,
    required LinkedOAuthTabStore store,
    required this.returnUri,
    required this.surface,
    required this.navigateSameTab,
    required this.scrubLocation,
    required this.rereadAuthoritativeState,
    DateTime Function()? clock,
  }) : _client = client,
       _store = store,
       _clock = clock ?? DateTime.now {
    if (!returnUri.isAbsolute ||
        returnUri.hasQuery ||
        returnUri.hasFragment ||
        returnUri.userInfo.isNotEmpty ||
        !(surface == 'web' &&
                returnUri.scheme == 'https' &&
                returnUri.host.isNotEmpty ||
            surface == 'windows' && returnUri.scheme == 'voice')) {
      throw ArgumentError('An exact configured OAuth return URI is required');
    }
  }

  final LinkedOAuthClient _client;
  final LinkedOAuthTabStore _store;
  final DateTime Function() _clock;
  final Uri returnUri;
  final String surface;
  final Future<bool> Function(Uri uri) navigateSameTab;
  final void Function(Uri cleanUri) scrubLocation;
  final Future<void> Function() rereadAuthoritativeState;
  LinkedOAuthReturnState _state = const LinkedOAuthReturnState(
    LinkedOAuthPhase.idle,
  );
  LinkedOAuthReturnState get state => _state;
  AuthSession? _session;
  String? _binding;
  int _generation = 0;
  bool _bound = false;
  bool _disposed = false;

  void bindSession(AuthSession? session, {required String sessionIdentity}) {
    final binding = session == null
        ? null
        : sha256
              .convert(
                utf8.encode(
                  jsonEncode([
                    sessionIdentity,
                    session.accountId,
                    session.activeProfileId,
                    session.accessToken,
                    session.refreshToken,
                  ]),
                ),
              )
              .toString();
    if (_bound && binding == _binding) return;
    final changed = _bound;
    _bound = true;
    _session = session;
    _binding = binding;
    _generation++;
    if (changed || session == null) _clear();
    // Reload can recover only the original credential and session installation.
    final pending = _pending();
    if (pending != null && pending['binding'] != binding) _clear();
    _set(const LinkedOAuthReturnState(LinkedOAuthPhase.idle));
  }

  Future<void> start(String provider) async {
    final session = _session;
    if (session == null || session.accountType != 'regular') {
      _fail('invalid_token');
      return;
    }
    final generation = ++_generation;
    if (!_clear()) {
      _fail('oauth_storage_unavailable');
      return;
    }
    _set(const LinkedOAuthReturnState(LinkedOAuthPhase.starting));
    final random = Random.secure();
    final verifier = base64UrlEncode(
      List<int>.generate(32, (_) => random.nextInt(256)),
    ).replaceAll('=', '');
    final challenge = base64UrlEncode(
      sha256.convert(ascii.encode(verifier)).bytes,
    ).replaceAll('=', '');
    final result = await _client.start(
      provider: provider,
      authorization: session.authorizationHeader,
      surface: surface,
      challenge: challenge,
    );
    if (!_current(generation)) return;
    switch (result) {
      case GatewayHttpFailure(:final error):
        _fail(error.errorCode);
      case GatewayHttpOk(:final data):
        if (!_clock().isBefore(data.expiresAt)) {
          _fail('result_expired');
          return;
        }
        try {
          _store.write(
            jsonEncode({
              'binding': _binding,
              'provider': provider,
              'attempt_id': data.id,
              'verifier': verifier,
              'expires_at': data.expiresAt.toIso8601String(),
              'return_uri': returnUri.toString(),
              'surface': surface,
            }),
          );
          _set(const LinkedOAuthReturnState(LinkedOAuthPhase.awaitingReturn));
          final launched = await navigateSameTab(data.authorizationUri);
          if (_current(generation) && !launched) {
            final cancellationGeneration = _generation + 1;
            await cancel();
            if (_current(cancellationGeneration)) _fail('launch_failed');
          }
        } catch (_) {
          if (_current(generation)) {
            _clear();
            _fail('launch_failed');
          }
        }
    }
  }

  Future<void> handleReturn(Uri uri) async {
    // Synchronous and first: no fragment/query survives to telemetry or OS handoff.
    // Explicit # prevents fragment inheritance during a redirect/navigation.
    try {
      scrubLocation(stripLinkedOAuthReturnSecrets(uri));
    } catch (_) {
      _generation++;
      _clear();
      _fail('invalid_link_attempt');
      return;
    }
    if (uri.fragment.isNotEmpty) {
      _generation++;
      _clear();
      _fail('invalid_link_attempt');
      return;
    }
    final pending = _pending();
    final session = _session;
    Map<String, List<String>> params;
    try {
      params = uri.queryParametersAll;
    } on FormatException {
      _fail('invalid_link_attempt');
      return;
    }
    final cleanRoute = stripLinkedOAuthReturnSecrets(uri).toString();
    final expectedRoute = stripLinkedOAuthReturnSecrets(returnUri).toString();
    if (pending == null ||
        session == null ||
        pending['binding'] != _binding ||
        pending['return_uri'] != returnUri.toString() ||
        pending['surface'] != surface ||
        cleanRoute != expectedRoute ||
        params.length != 2 ||
        params['attempt_id']?.length != 1 ||
        params['result_handle']?.length != 1 ||
        params['attempt_id']?.single != pending['attempt_id'] ||
        !RegExp(
          r'^[A-Za-z0-9_-]{43}$',
        ).hasMatch(params['result_handle']?.single ?? '')) {
      _fail('invalid_link_attempt');
      return;
    }
    final expiry = DateTime.tryParse(pending['expires_at'] as String);
    if (expiry == null || !_clock().isBefore(expiry)) {
      _clear();
      _fail('result_expired');
      return;
    }
    final generation = ++_generation;
    // Claim locally once, before awaiting. Backend enforces result TTL and one-use.
    if (!_clear()) {
      _fail('oauth_storage_unavailable');
      return;
    }
    _set(const LinkedOAuthReturnState(LinkedOAuthPhase.consuming));
    final result = await _client.consume(
      provider: pending['provider'] as String,
      authorization: session.authorizationHeader,
      attemptId: pending['attempt_id'] as String,
      resultHandle: params['result_handle']!.single,
      verifier: pending['verifier'] as String,
    );
    if (!_current(generation)) return;
    switch (result) {
      case GatewayHttpOk(:final data):
        _set(LinkedOAuthReturnState(LinkedOAuthPhase.complete, outcome: data));
        if (data == LinkedOAuthOutcome.success) await _reread(generation);
      case GatewayHttpFailure(:final error):
        _fail(error.errorCode);
        if (error.errorCode == 'result_consumed' ||
            error.errorCode == 'network_error' ||
            error.errorCode == 'invalid_response' ||
            error.statusCode >= 500) {
          // Auth may have committed before its reply was lost. Reconcile reads,
          // never replay the one-use result or infer a successful source.
          await _reread(generation);
        }
    }
  }

  Future<void> cancel() async {
    // Once consume has been sent, keep its authoritative reconciliation alive.
    // The Settings consumer disables Cancel in this phase.
    if (_state.phase == LinkedOAuthPhase.consuming) return;
    final pending = _pending();
    final session = _session;
    final generation = ++_generation;
    _clear();
    if (pending == null || session == null || pending['binding'] != _binding) {
      _set(const LinkedOAuthReturnState(LinkedOAuthPhase.idle));
      return;
    }
    final result = await _client.cancel(
      provider: pending['provider'] as String,
      authorization: session.authorizationHeader,
      attemptId: pending['attempt_id'] as String,
    );
    if (!_current(generation)) return;
    switch (result) {
      case GatewayHttpOk<void>():
        _set(
          const LinkedOAuthReturnState(
            LinkedOAuthPhase.complete,
            outcome: LinkedOAuthOutcome.cancelled,
          ),
        );
      case GatewayHttpFailure(:final error):
        _fail(error.errorCode);
        if (error.errorCode == 'result_consumed') await _reread(generation);
    }
  }

  Future<void> _reread(int generation) async {
    try {
      await rereadAuthoritativeState();
    } catch (_) {
      if (_current(generation)) _fail('authoritative_reload_failed');
    }
  }

  Map<String, dynamic>? _pending() {
    try {
      final raw = _store.read();
      if (raw == null) return null;
      final value = jsonDecode(raw);
      if (value is Map<String, dynamic> &&
          const [
            'binding',
            'provider',
            'attempt_id',
            'verifier',
            'expires_at',
            'return_uri',
            'surface',
          ].every((key) => value[key] is String) &&
          linkedOAuthAttemptId.hasMatch(value['attempt_id'] as String) &&
          const ['twitch', 'youtube'].contains(value['provider']) &&
          RegExp(
            r'^[A-Za-z0-9_-]{43}$',
          ).hasMatch(value['verifier'] as String)) {
        return value;
      }
      _clear();
    } catch (_) {
      _clear();
    }
    return null;
  }

  bool _current(int generation) => !_disposed && generation == _generation;
  bool _clear() {
    try {
      _store.clear();
      return true;
    } catch (_) {
      return false;
    }
  }

  void _fail(String code) =>
      _set(LinkedOAuthReturnState(LinkedOAuthPhase.failed, errorCode: code));
  void _set(LinkedOAuthReturnState state) {
    if (_disposed) return;
    _state = state;
    notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    _generation++;
    super.dispose();
  }
}
