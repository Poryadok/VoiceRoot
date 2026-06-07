import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend/auth_client.dart';
import '../backend/auth_session.dart';
import '../backend/auth_session_storage.dart';
import '../backend/discover_hint_storage.dart';
import '../backend/gateway_http.dart';
import '../ui/auth/auth_errors.dart';
import 'gateway_providers.dart';
import 'version_policy_providers.dart';

final discoverHintStorageProvider = Provider<DiscoverHintStorage>((ref) {
  throw UnimplementedError(
    'Override discoverHintStorageProvider in ProviderScope',
  );
});

class AuthState {
  const AuthState({
    this.session,
    this.isRestoring = false,
    this.isSubmitting = false,
    this.errorKey,
    this.pendingDiscoverHint = false,
  });

  final AuthSession? session;
  final bool isRestoring;
  final bool isSubmitting;

  /// API or client [AuthErrorKeys] value for localized UI message.
  final String? errorKey;

  /// True after login/register; cleared after discover snackbar is shown.
  final bool pendingDiscoverHint;

  bool get isAuthenticated => session != null;

  String? get activeProfileId => session?.activeProfileId;

  AuthState copyWith({
    AuthSession? session,
    bool clearSession = false,
    bool? isRestoring,
    bool? isSubmitting,
    String? errorKey,
    bool clearError = false,
    bool? pendingDiscoverHint,
    bool clearDiscoverHint = false,
  }) {
    return AuthState(
      session: clearSession ? null : (session ?? this.session),
      isRestoring: isRestoring ?? this.isRestoring,
      isSubmitting: isSubmitting ?? this.isSubmitting,
      errorKey: clearError ? null : (errorKey ?? this.errorKey),
      pendingDiscoverHint: clearDiscoverHint
          ? false
          : (pendingDiscoverHint ?? this.pendingDiscoverHint),
    );
  }
}

final authSessionStorageProvider = Provider<AuthSessionStorage>((ref) {
  throw UnimplementedError(
    'Override authSessionStorageProvider in ProviderScope',
  );
});


class AuthController extends StateNotifier<AuthState> {
  AuthController({
    required VoiceAuthClient authClient,
    required AuthSessionStorage storage,
  }) : _authClient = authClient,
       _storage = storage,
       super(const AuthState()) {
    _scheduleProactiveRefresh();
  }

  final VoiceAuthClient _authClient;
  final AuthSessionStorage _storage;
  Timer? _refreshTimer;

  Future<void> restore() async {
    final saved = await _storage.read();
    if (saved == null) {
      state = state.copyWith(isRestoring: false, clearError: true);
      return;
    }
    state = state.copyWith(session: saved, isRestoring: true, clearError: true);
    final refreshed = await _authClient.refresh(
      refreshToken: saved.refreshToken,
    );
    switch (refreshed) {
      case AuthSessionOk(:final session):
        await _persist(session);
        state = state.copyWith(
          session: session,
          isRestoring: false,
          clearError: true,
          clearDiscoverHint: true,
        );
      case AuthSessionFailure():
        await _storage.clear();
        state = state.copyWith(
          clearSession: true,
          isRestoring: false,
          clearError: true,
        );
    }
  }

  Future<void> register({required String email, required String password}) =>
      _authenticate(
        () => _authClient.register(email: email, password: password),
      );

  Future<void> login({required String email, required String password}) =>
      _authenticate(() => _authClient.login(email: email, password: password));

  void setClientError(String errorKey) {
    state = state.copyWith(errorKey: errorKey, isSubmitting: false);
  }

  Future<void> logout() async {
    final current = state.session;
    state = state.copyWith(isSubmitting: true, clearError: true);
    if (current != null) {
      await _authClient.logout(session: current);
    }
    await _storage.clear();
    state = state.copyWith(
      clearSession: true,
      isSubmitting: false,
      clearError: true,
    );
  }

  Future<void> _authenticate(Future<AuthSessionResult> Function() call) async {
    state = state.copyWith(isSubmitting: true, clearError: true);
    final result = await call();
    switch (result) {
      case AuthSessionOk(:final session):
        await _persist(session);
        state = state.copyWith(
          session: session,
          isSubmitting: false,
          clearError: true,
          pendingDiscoverHint: true,
        );
      case AuthSessionFailure(
        :final message,
        :final errorCode,
        :final statusCode,
      ):
        state = state.copyWith(
          isSubmitting: false,
          errorKey:
              resolveAuthErrorKey(
                errorCode: errorCode,
                statusCode: statusCode,
              ) ??
              message,
        );
    }
  }

  void clearPendingDiscoverHint() {
    if (!state.pendingDiscoverHint) return;
    state = state.copyWith(clearDiscoverHint: true);
  }

  Future<void> _persist(AuthSession session) async {
    await _storage.write(session);
    _scheduleProactiveRefresh();
  }

  /// Called by [GatewayHttpClient] on 401; returns true when session was refreshed.
  Future<bool> refreshOn401() async {
    final current = state.session;
    if (current == null) return false;
    final refreshed = await _authClient.refresh(
      refreshToken: current.refreshToken,
    );
    switch (refreshed) {
      case AuthSessionOk(:final session):
        await _persist(session);
        state = state.copyWith(session: session, clearError: true);
        return true;
      case AuthSessionFailure():
        await _storage.clear();
        state = state.copyWith(clearSession: true, clearError: true);
        return false;
    }
  }

  void _scheduleProactiveRefresh() {
    _refreshTimer?.cancel();
    final session = state.session;
    if (session == null) return;
    final delaySeconds = session.expiresInSeconds - 60;
    if (delaySeconds <= 0) return;
    _refreshTimer = Timer(Duration(seconds: delaySeconds), () async {
      await refreshOn401();
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    super.dispose();
  }
}

final voiceAuthClientProvider = Provider<VoiceAuthClient>((ref) {
  return VoiceAuthClient(
    gateway: GatewayHttpClient(
      httpClient: ref.watch(httpClientProvider),
      config: ref.watch(gatewayConfigProvider),
    ),
  );
});

final StateNotifierProvider<AuthController, AuthState> authControllerProvider =
    StateNotifierProvider<AuthController, AuthState>((ref) {
      return AuthController(
        authClient: ref.watch(voiceAuthClientProvider),
        storage: ref.watch(authSessionStorageProvider),
      );
    });

/// Bearer value for protected Gateway routes, or null when logged out.
final authorizationHeaderProvider = Provider<String?>((ref) {
  return ref.watch(authControllerProvider).session?.authorizationHeader;
});

final Provider<GatewayHttpClient> gatewayHttpClientProvider =
    Provider<GatewayHttpClient>((ref) {
      return GatewayHttpClient(
        httpClient: ref.watch(httpClientProvider),
        config: ref.watch(gatewayConfigProvider),
        authorizationProvider: () =>
            ref.read(authControllerProvider).session?.authorizationHeader,
        onUnauthorized: () =>
            ref.read(authControllerProvider.notifier).refreshOn401(),
        onUpgradeRequired: (error) => ref
            .read(versionPolicyProvider.notifier)
            .onGatewayUpgradeRequired(error),
      );
    });
