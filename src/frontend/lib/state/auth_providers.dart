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
    this.pendingGuestConversionEmail,
    this.isGuestConversionPromotionPending = false,
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

  /// Email awaiting verification after a guest conversion submission.
  final String? pendingGuestConversionEmail;

  final bool isGuestConversionPromotionPending;

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
    String? pendingGuestConversionEmail,
    bool clearPendingGuestConversionEmail = false,
    bool? isGuestConversionPromotionPending,
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
      pendingGuestConversionEmail: clearPendingGuestConversionEmail
          ? null
          : (pendingGuestConversionEmail ?? this.pendingGuestConversionEmail),
      isGuestConversionPromotionPending:
          isGuestConversionPromotionPending ??
          this.isGuestConversionPromotionPending,
    );
  }
}

final authSessionStorageProvider = Provider<AuthSessionStorage>((ref) {
  throw UnimplementedError(
    'Override authSessionStorageProvider in ProviderScope',
  );
});

final guestCredentialsStorageProvider = Provider<GuestCredentialsStorage>((
  ref,
) {
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
  Future<bool>? _refreshInFlight;
  var _profileSwitchGeneration = 0;
  AuthSession? _latestProfileSession;
  var _latestProfileSessionGeneration = 0;
  int? _terminatedProfileSessionGeneration;
  var _convertingGuest = false;
  static final _random = Random.secure();

  bool _isDefinitiveAuthRejection(AuthSessionFailure failure) {
    final code = failure.errorCode?.trim();
    final status = failure.statusCode;
    if (status == 401 || status == 403) return true;
    const definitiveCodes = {
      'invalid_token',
      'token_revoked',
      'token_expired',
      'invalid_credentials',
      'unauthenticated',
    };
    if (code != null && definitiveCodes.contains(code)) return true;
    if (code == 'network_error' || code == 'auth_unavailable') return false;
    if (status != null && status >= 500) return false;
    return false;
  }

  Future<void> _finishRestoreWithSavedSession(AuthSession saved) async {
    final isGuest = await _resolveIsGuest(saved);
    final needsGuestNickname =
        isGuest &&
        !await _guestCredentialsStorage.isNicknameCompleted(saved.accountId);
    state = state.copyWith(
      session: saved,
      isRestoring: false,
      clearError: true,
      clearDiscoverHint: true,
      isGuest: isGuest,
      needsGuestNickname: needsGuestNickname,
      pendingGuestConversionEmail: isGuest
          ? await _guestCredentialsStorage.readPendingConversionEmail()
          : null,
      isGuestConversionPromotionPending:
          isGuest &&
          await _guestCredentialsStorage.isGuestConversionPromotionPending(),
    );
    if (!needsGuestNickname) {
      await _notifyAuthenticated();
    }
  }

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
        final needsGuestNickname =
            isGuest &&
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
          pendingGuestConversionEmail: isGuest
              ? await _guestCredentialsStorage.readPendingConversionEmail()
              : null,
          isGuestConversionPromotionPending:
              isGuest &&
              await _guestCredentialsStorage
                  .isGuestConversionPromotionPending(),
        );
        if (!needsGuestNickname) {
          await _notifyAuthenticated();
        }
      case AuthSessionFailure(
        :final message,
        :final errorCode,
        :final statusCode,
      ):
        if (_isDefinitiveAuthRejection(
          AuthSessionFailure(
            message: message,
            errorCode: errorCode,
            statusCode: statusCode,
          ),
        )) {
          await _storage.clear();
          state = state.copyWith(
            clearSession: true,
            isRestoring: false,
            clearError: true,
            clearGuest: true,
          );
        } else {
          await _finishRestoreWithSavedSession(saved);
        }
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
                message: message,
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
    _convertingGuest = true;
    final result = await _authClient.convertGuest(
      session: current,
      email: email,
      password: password,
    );
    switch (result) {
      case AuthSessionOk(:final session):
        if (session.accessToken.isEmpty || session.refreshToken.isEmpty) {
          _convertingGuest = false;
          return AuthErrorKeys.validationFailed;
        }
        await _guestCredentialsStorage.writePendingConversionEmail(email);
        await _guestCredentialsStorage.setGuestConversionPromotionPending(
          false,
        );
        state = state.copyWith(
          session: session,
          isGuest: true,
          pendingGuestConversionEmail: email,
          isGuestConversionPromotionPending: false,
          clearError: true,
        );
        await _persist(session);
        final sent = await _authClient.sendGuestConversionEmailOtp(
          session: session,
          email: email,
        );
        _convertingGuest = false;
        return switch (sent) {
          AuthApiOk<void>() => null,
          AuthApiFailure(:final message, :final errorCode, :final statusCode) =>
            resolveAuthErrorKey(
                  errorCode: errorCode,
                  statusCode: statusCode,
                  message: message,
                ) ??
                message,
        };
      case AuthSessionFailure(
        :final message,
        :final errorCode,
        :final statusCode,
      ):
        _convertingGuest = false;
        return resolveAuthErrorKey(
              errorCode: errorCode,
              statusCode: statusCode,
              message: message,
            ) ??
            message;
    }
  }

  Future<String?> resendGuestConversionEmailOtp() async {
    final current = state.session;
    final email = state.pendingGuestConversionEmail;
    if (current == null || email == null || email.isEmpty) {
      return 'not_authenticated';
    }
    final result = await _authClient.sendGuestConversionEmailOtp(
      session: current,
      email: email,
    );
    return switch (result) {
      AuthApiOk<void>() => null,
      AuthApiFailure(:final message, :final errorCode, :final statusCode) =>
        resolveAuthErrorKey(
              errorCode: errorCode,
              statusCode: statusCode,
              message: message,
            ) ??
            message,
    };
  }

  Future<String?> verifyGuestConversionEmail(String code) async {
    final current = state.session;
    final email = state.pendingGuestConversionEmail;
    if (current == null || email == null || email.isEmpty) {
      return 'not_authenticated';
    }
    _convertingGuest = true;
    final credentials = await _guestCredentialsStorage.snapshot();
    final verified = await _authClient.verifyGuestConversionEmailOtp(
      session: current,
      email: email,
      code: code,
    );
    if (verified case GuestConversionOtpFailure(
      :final message,
      :final errorCode,
      :final statusCode,
    )) {
      _convertingGuest = false;
      return resolveAuthErrorKey(
            errorCode: errorCode,
            statusCode: statusCode,
            message: message,
          ) ??
          message;
    }
    if (verified case GuestConversionOtpSession(
      :final session,
    ) when _isRegularSession(session)) {
      final completed = await _finishGuestConversion(
        session: session,
        expected: current,
        generation: _profileSwitchGeneration,
        credentials: credentials,
      );
      if (!completed) {
        _convertingGuest = false;
        return 'not_authenticated';
      }
      _convertingGuest = false;
      await _notifyAuthenticated();
      return null;
    }
    return _refreshGuestConversionUntilRegular(
      current,
      _profileSwitchGeneration,
    );
  }

  /// Rechecks promotion after an accepted OTP without replaying that OTP.
  Future<String?> resumeGuestConversionPromotion() async {
    final current = state.session;
    if (current == null || !state.isGuestConversionPromotionPending) {
      return 'not_authenticated';
    }
    _convertingGuest = true;
    return _refreshGuestConversionUntilRegular(
      current,
      _profileSwitchGeneration,
    );
  }

  Future<String?> _refreshGuestConversionUntilRegular(
    AuthSession initialSession,
    int generation,
  ) async {
    var current = initialSession;
    for (var attempt = 0; attempt < 4; attempt++) {
      final credentials = await _guestCredentialsStorage.snapshot();
      final refreshed = await _authClient.refresh(
        refreshToken: current.refreshToken,
      );
      if (!_isGuestConversionCurrent(current, generation)) {
        _convertingGuest = false;
        return 'not_authenticated';
      }
      switch (refreshed) {
        case AuthSessionOk(:final session) when _isRegularSession(session):
          final completed = await _finishGuestConversion(
            session: session,
            expected: current,
            generation: generation,
            credentials: credentials,
          );
          if (!completed) {
            _convertingGuest = false;
            return 'not_authenticated';
          }
          _convertingGuest = false;
          await _notifyAuthenticated();
          return null;
        case AuthSessionOk(:final session):
          final previous = current;
          if (!_isGuestConversionCurrent(previous, generation)) {
            _convertingGuest = false;
            return 'not_authenticated';
          }
          await _guestCredentialsStorage.setGuestConversionPromotionPending(
            true,
          );
          if (!_isGuestConversionCurrent(previous, generation)) {
            _convertingGuest = false;
            return 'not_authenticated';
          }
          current = session;
          await _commitProfileSession(
            session: current,
            generation: generation,
            nextState: (currentState) => currentState.copyWith(
              session: current,
              isGuest: true,
              isGuestConversionPromotionPending: true,
              clearError: true,
            ),
          );
          if (generation != _profileSwitchGeneration) {
            _convertingGuest = false;
            return 'not_authenticated';
          }
          if (attempt < 3) {
            await Future<void>.delayed(
              Duration(milliseconds: 150 * (attempt + 1)),
            );
            if (!_isGuestConversionCurrent(current, generation)) {
              _convertingGuest = false;
              return 'not_authenticated';
            }
          }
        case AuthSessionFailure(
          :final message,
          :final errorCode,
          :final statusCode,
        ):
          _convertingGuest = false;
          return resolveAuthErrorKey(
                errorCode: errorCode,
                statusCode: statusCode,
                message: message,
              ) ??
              message;
      }
    }
    _convertingGuest = false;
    return 'guest_conversion_pending';
  }

  Future<bool> _finishGuestConversion({
    required AuthSession session,
    required AuthSession expected,
    required int generation,
    required GuestCredentialsSnapshot credentials,
  }) async {
    if (!_isGuestConversionCurrent(expected, generation)) return false;
    if (!await _guestCredentialsStorage.clearIfUnchanged(credentials)) {
      return false;
    }
    if (!_isGuestConversionCurrent(expected, generation)) return false;
    await _commitProfileSession(
      session: session,
      generation: generation,
      nextState: (currentState) => currentState.copyWith(
        session: session,
        clearGuest: true,
        clearGuestNickname: true,
        clearPendingGuestConversionEmail: true,
        isGuestConversionPromotionPending: false,
        clearError: true,
      ),
    );
    return generation == _profileSwitchGeneration &&
        state.session?.refreshToken == session.refreshToken &&
        !state.isGuest;
  }

  Future<void> login({
    required String email,
    required String password,
    String? totpCode,
  }) => _authenticate(
    () =>
        _authClient.login(email: email, password: password, totpCode: totpCode),
  );

  Future<void> applySession(AuthSession session) async {
    await _persist(session);
    state = state.copyWith(session: session, clearError: true);
  }

  Future<String?> switchActiveProfile(String profileId) async {
    final current = state.session;
    if (current == null) return 'not_authenticated';
    if (_terminatedProfileSessionGeneration != null) {
      return 'not_authenticated';
    }
    if (current.activeProfileId == profileId) return null;
    final generation = ++_profileSwitchGeneration;
    _terminatedProfileSessionGeneration = null;

    final result = await _authClient.switchActiveProfile(
      session: current,
      profileId: profileId,
    );
    switch (result) {
      case AuthSessionOk(:final session):
        await _commitProfileSession(
          session: session,
          generation: generation,
          nextState: (currentState) =>
              currentState.copyWith(session: session, clearError: true),
        );
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
    _terminateProfileSession();
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
                message: message,
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
    _terminatedProfileSessionGeneration = null;
    await _storage.write(session);
    _scheduleProactiveRefresh();
  }

  void _terminateProfileSession() {
    _profileSwitchGeneration++;
    _latestProfileSession = null;
    _latestProfileSessionGeneration = -1;
    _terminatedProfileSessionGeneration = _profileSwitchGeneration;
  }

  Future<void> _commitProfileSession({
    required AuthSession session,
    required int generation,
    required AuthState Function(AuthState currentState) nextState,
  }) {
    if (generation != _profileSwitchGeneration) {
      return Future<void>.value();
    }

    _latestProfileSession = session;
    _latestProfileSessionGeneration = generation;
    return _storage.write(session).then((_) async {
      if (generation == _profileSwitchGeneration) {
        state = nextState(state);
        _scheduleProactiveRefresh();
        return;
      }

      if (_terminatedProfileSessionGeneration == _profileSwitchGeneration &&
          generation < _profileSwitchGeneration) {
        await _storage.clear();
        return;
      }

      final repair = _latestProfileSessionGeneration == _profileSwitchGeneration
          ? _latestProfileSession
          : state.session;
      if (repair != null) {
        await _storage.write(repair);
      }
    });
  }

  Future<void> _notifyAuthenticated() async {
    final callback = onAuthenticated;
    if (callback == null) return;
    await callback();
  }

  /// Called by [GatewayHttpClient] on 401; returns true when session was refreshed.
  Future<bool> refreshOn401() {
    final inFlight = _refreshInFlight;
    if (inFlight != null) return inFlight;
    final future = _refreshOn401Once(_profileSwitchGeneration);
    _refreshInFlight = future;
    return future.whenComplete(() {
      if (identical(_refreshInFlight, future)) {
        _refreshInFlight = null;
      }
    });
  }

  Future<bool> _refreshOn401Once(int generation) async {
    final current = state.session;
    if (current == null) return false;
    final refreshed = await _authClient.refresh(
      refreshToken: current.refreshToken,
    );
    switch (refreshed) {
      case AuthSessionOk(:final session):
        final isGuest = await _resolveIsGuest(session);
        await _commitProfileSession(
          session: session,
          generation: generation,
          nextState: (currentState) => currentState.copyWith(
            session: session,
            clearError: true,
            isGuest: isGuest,
          ),
        );
        return true;
      case AuthSessionFailure(
        :final message,
        :final errorCode,
        :final statusCode,
      ):
        if (_convertingGuest) return false;
        if (generation != _profileSwitchGeneration) return false;
        if (_isDefinitiveAuthRejection(
          AuthSessionFailure(
            message: message,
            errorCode: errorCode,
            statusCode: statusCode,
          ),
        )) {
          _terminateProfileSession();
          await _storage.clear();
          state = state.copyWith(clearSession: true, clearError: true);
        }
        return false;
    }
  }

  bool _isRegularSession(AuthSession session) {
    final type =
        session.accountType ?? accountTypeFromAccessToken(session.accessToken);
    return type == 'regular';
  }

  bool _isGuestConversionCurrent(AuthSession expected, int generation) {
    return generation == _profileSwitchGeneration &&
        state.session?.refreshToken == expected.refreshToken &&
        state.isGuest &&
        state.pendingGuestConversionEmail != null;
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
    return List.generate(
      32,
      (_) => chars[_random.nextInt(chars.length)],
    ).join();
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
          await ref
              .read(deepLinkControllerProvider.notifier)
              .flushPendingAfterAuth();
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
