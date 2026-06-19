import 'dart:async';
import 'dart:math';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend/auth_client.dart';
import '../backend/auth_session.dart';
import '../backend/auth_session_storage.dart';
import '../backend/discover_hint_storage.dart';
import '../backend/guest_credentials_storage.dart';
import '../backend/jwt_claims.dart';
import '../backend/gateway_http.dart';
import '../ui/auth/auth_errors.dart';
import '../routing/deep_link_controller.dart';
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
    this.isGuest = false,
    this.needsGuestNickname = false,
  });

  final AuthSession? session;
  final bool isRestoring;
  final bool isSubmitting;

  /// API or client [AuthErrorKeys] value for localized UI message.
  final String? errorKey;

  /// True after login/register; cleared after discover snackbar is shown.
  final bool pendingDiscoverHint;

  /// Guest account created via auto-register (no email).
  final bool isGuest;

  /// Guest must set a nickname before entering the main shell.
  final bool needsGuestNickname;

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
    bool? isGuest,
    bool clearGuest = false,
    bool? needsGuestNickname,
    bool clearGuestNickname = false,
  }) {
    return AuthState(
      session: clearSession ? null : (session ?? this.session),
      isRestoring: isRestoring ?? this.isRestoring,
      isSubmitting: isSubmitting ?? this.isSubmitting,
      errorKey: clearError ? null : (errorKey ?? this.errorKey),
      pendingDiscoverHint: clearDiscoverHint
          ? false
          : (pendingDiscoverHint ?? this.pendingDiscoverHint),
      isGuest: clearGuest ? false : (isGuest ?? this.isGuest),
      needsGuestNickname: clearGuestNickname
          ? false
          : (needsGuestNickname ?? this.needsGuestNickname),
    );
  }
}

final authSessionStorageProvider = Provider<AuthSessionStorage>((ref) {
  throw UnimplementedError(
    'Override authSessionStorageProvider in ProviderScope',
  );
});

final guestCredentialsStorageProvider = Provider<GuestCredentialsStorage>((ref) {
  return InMemoryGuestCredentialsStorage();
});


class AuthController extends StateNotifier<AuthState> {
  AuthController({
    required VoiceAuthClient authClient,
    required AuthSessionStorage storage,
    required GuestCredentialsStorage guestCredentialsStorage,
    this.onAuthenticated,
  }) : _authClient = authClient,
       _storage = storage,
       _guestCredentialsStorage = guestCredentialsStorage,
       super(const AuthState()) {
    _scheduleProactiveRefresh();
  }

  final VoiceAuthClient _authClient;
  final AuthSessionStorage _storage;
  final GuestCredentialsStorage _guestCredentialsStorage;
  final Future<void> Function()? onAuthenticated;
  Timer? _refreshTimer;
  static final _random = Random.secure();

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
        final isGuest = await _resolveIsGuest(session);
        final needsGuestNickname = isGuest &&
            !await _guestCredentialsStorage.isNicknameCompleted(
              session.accountId,
            );
        state = state.copyWith(
          session: session,
          isRestoring: false,
          clearError: true,
          clearDiscoverHint: true,
          isGuest: isGuest,
          needsGuestNickname: needsGuestNickname,
        );
        if (!needsGuestNickname) {
          await _notifyAuthenticated();
        }
      case AuthSessionFailure():
        await _storage.clear();
        state = state.copyWith(
          clearSession: true,
          isRestoring: false,
          clearError: true,
          clearGuest: true,
        );
    }
  }

  Future<void> register({required String email, required String password}) =>
      _authenticate(
        () => _authClient.register(email: email, password: password),
      );

  Future<void> registerGuest() async {
    if (state.session != null) return;
    final password = _generateGuestPassword();
    await _guestCredentialsStorage.writePassword(password);
    state = state.copyWith(isRestoring: true, clearError: true);
    final result = await _authClient.registerGuest(password: password);
    switch (result) {
      case AuthSessionOk(:final session):
        await _persist(session);
        state = state.copyWith(
          session: session,
          isRestoring: false,
          clearError: true,
          isGuest: await _resolveIsGuest(session),
          needsGuestNickname: true,
        );
      case AuthSessionFailure(
        :final message,
        :final errorCode,
        :final statusCode,
      ):
        state = state.copyWith(
          isRestoring: false,
          errorKey:
              resolveAuthErrorKey(
                errorCode: errorCode,
                statusCode: statusCode,
              ) ??
              message,
        );
    }
  }

  void requireGuestNickname() {
    if (!state.isGuest || state.session == null) return;
    state = state.copyWith(needsGuestNickname: true);
  }

  Future<void> completeGuestNickname() async {
    final accountId = state.session?.accountId;
    if (accountId == null) return;
    await _guestCredentialsStorage.markNicknameCompleted(accountId);
    state = state.copyWith(clearGuestNickname: true);
    await _notifyAuthenticated();
  }

  Future<String?> convertGuest({
    required String email,
    required String password,
  }) async {
    final current = state.session;
    if (current == null) return 'not_authenticated';
    final result = await _authClient.convertGuest(
      session: current,
      email: email,
      password: password,
    );
    switch (result) {
      case AuthSessionOk(:final session):
        await _guestCredentialsStorage.clear();
        await _persist(session);
        state = state.copyWith(
          session: session,
          clearGuest: true,
          clearGuestNickname: true,
          clearError: true,
        );
        return null;
      case AuthSessionFailure(:final message):
        return message;
    }
  }

  Future<void> login({
    required String email,
    required String password,
    String? totpCode,
  }) => _authenticate(
    () => _authClient.login(
      email: email,
      password: password,
      totpCode: totpCode,
    ),
  );

  Future<void> applySession(AuthSession session) async {
    await _persist(session);
    state = state.copyWith(session: session, clearError: true);
  }

  Future<String?> switchActiveProfile(String profileId) async {
    final current = state.session;
    if (current == null) return 'not_authenticated';
    if (current.activeProfileId == profileId) return null;

    final result = await _authClient.switchActiveProfile(
      session: current,
      profileId: profileId,
    );
    switch (result) {
      case AuthSessionOk(:final session):
        await _persist(session);
        state = state.copyWith(session: session, clearError: true);
        return null;
      case AuthSessionFailure(:final message):
        return message;
    }
  }

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
    await _guestCredentialsStorage.clear();
    state = state.copyWith(
      clearSession: true,
      isSubmitting: false,
      clearError: true,
      clearGuest: true,
      clearGuestNickname: true,
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
          isGuest: await _resolveIsGuest(session),
        );
        await _notifyAuthenticated();
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

  Future<void> _notifyAuthenticated() async {
    final callback = onAuthenticated;
    if (callback == null) return;
    await callback();
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
        state = state.copyWith(
          session: session,
          clearError: true,
          isGuest: await _resolveIsGuest(session),
        );
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

  String _generateGuestPassword() {
    const chars =
        'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
    return List.generate(32, (_) => chars[_random.nextInt(chars.length)]).join();
  }

  Future<bool> _resolveIsGuest(AuthSession session) async {
    if (isGuestAccountType(session.accountType)) return true;
    if (isGuestAccountType(accountTypeFromAccessToken(session.accessToken))) {
      return true;
    }
    final guestPassword = await _guestCredentialsStorage.readPassword();
    return guestPassword != null && guestPassword.isNotEmpty;
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

final profileSwitchInProgressProvider = StateProvider<bool>((ref) => false);

final StateNotifierProvider<AuthController, AuthState> authControllerProvider =
    StateNotifierProvider<AuthController, AuthState>((ref) {
      return AuthController(
        authClient: ref.watch(voiceAuthClientProvider),
        storage: ref.watch(authSessionStorageProvider),
        guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
        onAuthenticated: () async {
          await ref.read(deepLinkControllerProvider.notifier).flushPendingAfterAuth();
        },
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
