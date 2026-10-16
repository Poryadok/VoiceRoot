import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:uuid/uuid.dart';

import 'gateway_http.dart';

typedef SdkDeviceProofSigner =
    Future<String> Function(String authIssuedKid, String payload);

/// A signer must classify key-store and key-authority failures explicitly.
/// Only transient failures may keep a linked bootstrap credential retryable.
class SdkDeviceProofSigningException implements Exception {
  const SdkDeviceProofSigningException({required this.isRetryable});

  final bool isRetryable;
}

enum SdkAuthorizationErrorPresentation {
  temporary,
  authorizationDenied,
  invalidResponse,
}

class SdkAuthorizationException implements Exception {
  const SdkAuthorizationException(
    this.message, {
    this.canRetryResume = false,
    this.presentation,
  });
  final String message;
  final bool canRetryResume;
  final SdkAuthorizationErrorPresentation? presentation;
  @override
  String toString() => 'SdkAuthorizationException: $message';
}

class SdkAuthorizationStart {
  const SdkAuthorizationStart({
    required this.requestId,
    required this.displayName,
    required this.scopes,
    required this.expiresAt,
  });
  final String requestId;
  final String displayName;
  final Set<String> scopes;
  final DateTime expiresAt;
}

class SdkConsentView {
  const SdkConsentView({
    required this.requestId,
    required this.applicationId,
    required this.environmentId,
    required this.displayName,
    required this.scopes,
    required this.gameSubject,
    required this.policyRevision,
    required this.expiresAt,
  });
  final String requestId;
  final String applicationId;
  final String environmentId;
  final String displayName;
  final Set<String> scopes;
  final String gameSubject;
  final int policyRevision;
  final DateTime expiresAt;

  factory SdkConsentView.fromJson(Map<String, dynamic> json) => SdkConsentView(
    requestId: _requiredString(json, 'requestId'),
    applicationId: _requiredString(json, 'applicationId'),
    environmentId: _requiredString(json, 'environmentId'),
    displayName: _requiredString(json, 'displayName'),
    scopes: _stringSet(json, 'scopes'),
    gameSubject: _requiredString(json, 'gameSubject'),
    policyRevision: _requiredInt(json, 'policyRevision'),
    expiresAt: _date(json, 'expiresAt'),
  );
}

class SdkAuthorizationApproval {
  const SdkAuthorizationApproval({
    required this.redirectUri,
    required this.expiresAt,
  });

  /// Contains the one-use code only in transient memory for callback handoff.
  final String redirectUri;
  final DateTime expiresAt;
}

class SdkLinkedSession {
  const SdkLinkedSession({
    required this.sourceAccountId,
    required this.accountId,
    required this.profileId,
    required this.deviceId,
    required this.applicationId,
    required this.environmentId,
    required this.scopes,
    required this.consentRevision,
    required this.policyRevision,
    required this.expiresAt,
  });
  final String sourceAccountId;
  final String accountId;
  final String profileId;
  final String deviceId;
  final String applicationId;
  final String environmentId;
  final Set<String> scopes;
  final int consentRevision;
  final int policyRevision;
  final DateTime expiresAt;

  factory SdkLinkedSession.fromJson(Map<String, dynamic> json) =>
      SdkLinkedSession(
        sourceAccountId: _requiredString(json, 'sourceAccountId'),
        accountId: _requiredString(json, 'accountId'),
        profileId: _requiredString(json, 'profileId'),
        deviceId: _requiredString(json, 'deviceId'),
        applicationId: _requiredString(json, 'applicationId'),
        environmentId: _requiredString(json, 'environmentId'),
        scopes: _stringSet(json, 'scopes'),
        consentRevision: _requiredInt(json, 'consentRevision'),
        policyRevision: _requiredInt(json, 'policyRevision'),
        expiresAt: _date(json, 'expiresAt'),
      );
}

class SdkAuthorizationHandoff {
  const SdkAuthorizationHandoff({
    required this.requestId,
    required this.redirectUri,
    required this.state,
    required this.codeVerifier,
    required this.authKeyId,
    required this.expiresAt,
  });
  final String requestId;
  final String redirectUri;
  final String state;
  final String codeVerifier;
  final String authKeyId;

  /// Secure-storage records are short-lived and discarded after five minutes.
  final DateTime expiresAt;

  Map<String, dynamic> toJson() => {
    'requestId': requestId,
    'redirectUri': redirectUri,
    'state': state,
    'codeVerifier': codeVerifier,
    'authKeyId': authKeyId,
    'expiresAt': expiresAt.toUtc().toIso8601String(),
  };

  factory SdkAuthorizationHandoff.fromJson(Map<String, dynamic> json) =>
      SdkAuthorizationHandoff(
        requestId: _requiredString(json, 'requestId'),
        redirectUri: _requiredString(json, 'redirectUri'),
        state: _requiredString(json, 'state'),
        codeVerifier: _requiredString(json, 'codeVerifier'),
        authKeyId: _requiredString(json, 'authKeyId'),
        expiresAt: _date(json, 'expiresAt'),
      );
}

abstract interface class SdkAuthorizationHandoffStorage {
  Future<void> pruneExpired();
  Future<SdkAuthorizationHandoff?> readByRequestId(String requestId);
  Future<SdkAuthorizationHandoff?> readByState(String state);
  Future<void> write(SdkAuthorizationHandoff handoff);
  Future<void> deleteByState(String state);
}

class SecureSdkAuthorizationHandoffStorage
    implements SdkAuthorizationHandoffStorage {
  SecureSdkAuthorizationHandoffStorage(this._storage);
  final FlutterSecureStorage _storage;
  static const _prefix = 'sdk_authorization_handoff_';

  @override
  Future<void> pruneExpired() async {
    final all = await _storage.readAll();
    for (final entry in all.entries.where(
      (entry) => entry.key.startsWith(_prefix),
    )) {
      final handoff = _decode(entry.value);
      if (handoff == null ||
          !handoff.expiresAt.isAfter(DateTime.now().toUtc())) {
        await _storage.delete(key: entry.key);
      }
    }
  }

  @override
  Future<SdkAuthorizationHandoff?> readByRequestId(String requestId) async {
    final all = await _storage.readAll();
    for (final entry in all.entries.where(
      (entry) => entry.key.startsWith(_prefix),
    )) {
      final handoff = _decode(entry.value);
      if (handoff == null ||
          !handoff.expiresAt.isAfter(DateTime.now().toUtc())) {
        await _storage.delete(key: entry.key);
        continue;
      }
      if (handoff.requestId == requestId) return handoff;
    }
    return null;
  }

  @override
  Future<SdkAuthorizationHandoff?> readByState(String state) async {
    final raw = await _storage.read(key: '$_prefix$state');
    final handoff = _decode(raw);
    if (handoff == null || !handoff.expiresAt.isAfter(DateTime.now().toUtc())) {
      if (raw != null) await deleteByState(state);
      return null;
    }
    return handoff;
  }

  @override
  Future<void> write(SdkAuthorizationHandoff handoff) async {
    await _storage.write(
      key: '$_prefix${handoff.state}',
      value: jsonEncode(handoff.toJson()),
    );
  }

  @override
  Future<void> deleteByState(String state) =>
      _storage.delete(key: '$_prefix$state');

  SdkAuthorizationHandoff? _decode(String? raw) {
    if (raw == null) return null;
    try {
      final handoff = SdkAuthorizationHandoff.fromJson(
        jsonDecode(raw) as Map<String, dynamic>,
      );
      return handoff;
    } on Object {
      return null;
    }
  }
}

class SdkLinkedSessionCredential {
  const SdkLinkedSessionCredential({
    required this.token,
    required this.authKeyId,
    required this.expiresAt,
  });
  final String token;
  final String authKeyId;
  final DateTime expiresAt;
}

abstract interface class SdkLinkedSessionStorage {
  Future<void> pruneExpired();
  Future<SdkLinkedSessionCredential?> read();
  Future<void> write(SdkLinkedSessionCredential credential);
  Future<void> clear();
}

class SecureSdkLinkedSessionStorage implements SdkLinkedSessionStorage {
  SecureSdkLinkedSessionStorage(this._storage);
  final FlutterSecureStorage _storage;
  static const _key = 'sdk_linked_session_credential';

  @override
  Future<void> pruneExpired() async {
    await read();
  }

  @override
  Future<SdkLinkedSessionCredential?> read() async {
    final raw = await _storage.read(key: _key);
    if (raw == null) return null;
    try {
      final json = jsonDecode(raw) as Map<String, dynamic>;
      final credential = SdkLinkedSessionCredential(
        token: _requiredString(json, 'token'),
        authKeyId: _requiredString(json, 'authKeyId'),
        expiresAt: _date(json, 'expiresAt'),
      );
      if (!credential.expiresAt.isAfter(DateTime.now().toUtc())) {
        await clear();
        return null;
      }
      return credential;
    } on Object {
      await clear();
      return null;
    }
  }

  @override
  Future<void> write(SdkLinkedSessionCredential credential) => _storage.write(
    key: _key,
    value: jsonEncode({
      'token': credential.token,
      'authKeyId': credential.authKeyId,
      'expiresAt': credential.expiresAt.toUtc().toIso8601String(),
    }),
  );

  @override
  Future<void> clear() => _storage.delete(key: _key);
}

class SdkAuthorizationClient {
  SdkAuthorizationClient({
    required GatewayHttpClient gateway,
    required SdkAuthorizationHandoffStorage handoffStorage,
    required SdkLinkedSessionStorage linkedSessionStorage,
    required SdkDeviceProofSigner proofSigner,
    DateTime Function()? clock,
    Random? random,
  }) : _gateway = gateway,
       _handoffs = handoffStorage,
       _sessions = linkedSessionStorage,
       _proofSigner = proofSigner,
       _clock = clock ?? DateTime.now,
       _random = random ?? Random.secure();

  final GatewayHttpClient _gateway;
  final SdkAuthorizationHandoffStorage _handoffs;
  final SdkLinkedSessionStorage _sessions;
  final SdkDeviceProofSigner _proofSigner;
  final DateTime Function() _clock;
  final Random _random;
  static const _base = '/api/v1/auth/sdk/authorizations';
  static const _allowedScopes = <String>{
    'game.identity.read',
    'game.chat.read',
    'game.chat.send',
    'game.voice.join',
    'game.presence.write',
    'game.invites.create',
  };

  Future<SdkAuthorizationStart> startAuthorization({
    required String sdkAuthorization,
    required String authKeyId,
    required String redirectUri,
    required Set<String> scopes,
  }) async {
    await _handoffs.pruneExpired();
    _validateRedirect(redirectUri);
    if (sdkAuthorization.isEmpty || authKeyId.isEmpty) {
      throw const SdkAuthorizationException(
        'missing authorization or device key',
      );
    }
    if (scopes.isEmpty ||
        scopes.length > 16 ||
        !_allowedScopes.containsAll(scopes)) {
      throw const SdkAuthorizationException('invalid requested scopes');
    }
    final verifier = _randomToken(64);
    final challenge = _base64Url(sha256.convert(utf8.encode(verifier)).bytes);
    final state = _randomToken(48);
    final idempotencyKey = const Uuid().v4();
    final digest = _requestDigest(
      idempotencyKey,
      redirectUri,
      challenge,
      state,
      scopes,
    );
    final sourceToken = sdkAuthorization.replaceFirst(
      RegExp(r'^Bearer\s+'),
      '',
    );
    final proof = await _proofSigner(
      authKeyId,
      'voice-sdk-authorize-v1\n${_sha(sourceToken)}\n$digest',
    );
    final result = await _post(_base, sdkAuthorization, {
      'idempotencyKey': idempotencyKey,
      'redirectUri': redirectUri,
      'codeChallenge': challenge,
      'state': state,
      'scopes': scopes.toList()..sort(),
      'deviceProof': proof,
    });
    final handoff = SdkAuthorizationHandoff(
      requestId: _requiredString(result, 'requestId'),
      redirectUri: redirectUri,
      state: state,
      codeVerifier: verifier,
      authKeyId: authKeyId,
      expiresAt: _clock().toUtc().add(const Duration(minutes: 5)),
    );
    await _handoffs.write(handoff);
    return SdkAuthorizationStart(
      requestId: handoff.requestId,
      displayName: _requiredString(result, 'displayName'),
      scopes: _stringSet(result, 'scopes'),
      expiresAt: _date(result, 'expiresAt'),
    );
  }

  Future<SdkConsentView> loadConsentView({
    required String requestId,
    required String voiceAuthorization,
  }) async => SdkConsentView.fromJson(
    await _get('$_base/${Uri.encodeComponent(requestId)}', voiceAuthorization),
  );

  Future<SdkAuthorizationApproval> approveAuthorization({
    required String requestId,
    required String voiceAuthorization,
    required String profileId,
    required int policyRevision,
  }) async {
    if (profileId.isEmpty || policyRevision < 1) {
      throw const SdkAuthorizationException(
        'profile and current policy revision are required',
      );
    }
    final response = await _post(
      '$_base/${Uri.encodeComponent(requestId)}/approve',
      voiceAuthorization,
      {'profileId': profileId, 'policyRevision': policyRevision},
    );
    final callback = Uri.tryParse(_requiredString(response, 'redirectUri'));
    final code = callback?.queryParametersAll['code'];
    if (callback == null ||
        code == null ||
        code.length != 1 ||
        !_validCode(code.single)) {
      throw const SdkAuthorizationException(
        'Auth returned an invalid callback',
        presentation: SdkAuthorizationErrorPresentation.invalidResponse,
      );
    }
    return SdkAuthorizationApproval(
      redirectUri: callback.toString(),
      expiresAt: _date(response, 'expiresAt'),
    );
  }

  /// Accepts the callback URI as data, independently from the browser/WebView.
  /// The code exists only in this stack frame and the handoff is deleted before
  /// network exchange, so a replay cannot recover the verifier or code.
  Future<SdkLinkedSession> acceptCallbackAndResume(Uri callback) async {
    final params = callback.queryParametersAll;
    if (params.keys.toSet().difference({'code', 'state'}).isNotEmpty ||
        params['code']?.length != 1 ||
        params['state']?.length != 1 ||
        callback.hasFragment ||
        callback.userInfo.isNotEmpty) {
      throw const SdkAuthorizationException('invalid callback parameters');
    }
    final state = params['state']!.single;
    final handoff = await _handoffs.readByState(state);
    if (handoff == null ||
        !_sameCallbackEndpoint(callback, handoff.redirectUri)) {
      throw const SdkAuthorizationException(
        'callback origin, path, or state did not match',
      );
    }
    final code = params['code']!.single;
    if (!_validCode(code)) {
      throw const SdkAuthorizationException('invalid authorization code');
    }
    await _handoffs.deleteByState(state);
    final codeHash = _sha(code);
    final verifierHash = _sha(handoff.codeVerifier);
    final proof = await _proofSigner(
      handoff.authKeyId,
      'voice-sdk-code-v1\n${handoff.requestId}\n$codeHash\n$verifierHash',
    );
    late final Map<String, dynamic> exchange;
    try {
      exchange = await _post(
        '$_base/${Uri.encodeComponent(handoff.requestId)}/exchange',
        null,
        {
          'code': code,
          'redirectUri': handoff.redirectUri,
          'codeVerifier': handoff.codeVerifier,
          'deviceProof': proof,
        },
      );
    } on SdkAuthorizationException {
      throw const SdkAuthorizationException(
        'Authorization could not be completed. Start again from the game.',
      );
    }
    final token = _requiredString(exchange, 'accessToken');
    final expiresAt = _date(exchange, 'expiresAt');
    await _sessions.write(
      SdkLinkedSessionCredential(
        token: token,
        authKeyId: handoff.authKeyId,
        expiresAt: expiresAt,
      ),
    );
    return resumePendingLinkedSession();
  }

  /// Retry linked-session resume after a transport failure. The short-lived
  /// bootstrap bearer remains only in secure storage until resume succeeds or
  /// its server-provided expiry is reached.
  Future<SdkLinkedSession> resumePendingLinkedSession() async {
    final credential = await _sessions.read();
    if (credential == null) {
      throw const SdkAuthorizationException('no resumable linked session');
    }
    late final String linkedProof;
    try {
      linkedProof = await _proofSigner(
        credential.authKeyId,
        'voice-sdk-linked-v1\n${_sha(credential.token)}',
      );
    } on SdkDeviceProofSigningException catch (error) {
      if (!error.isRetryable) await _sessions.clear();
      throw SdkAuthorizationException(
        error.isRetryable
            ? 'Voice is temporarily unavailable. You can retry the session resume.'
            : 'The device key is no longer available. Start again from the game.',
        canRetryResume: error.isRetryable,
      );
    } on Object {
      await _sessions.clear();
      throw const SdkAuthorizationException(
        'The device key is no longer available. Start again from the game.',
      );
    }
    late final Map<String, dynamic> result;
    try {
      result = await _post(
        '$_base/linked-session',
        'Bearer ${credential.token}',
        {'deviceProof': linkedProof},
      );
    } on SdkAuthorizationException catch (error) {
      if (!error.canRetryResume) await _sessions.clear();
      rethrow;
    }
    final session = SdkLinkedSession.fromJson(result);
    if (session.expiresAt.isAfter(credential.expiresAt)) {
      throw const SdkAuthorizationException(
        'linked session outlived bootstrap credential',
      );
    }
    await _sessions.clear();
    return session;
  }

  Future<Map<String, dynamic>> _get(String path, String? auth) async {
    final result = await _gateway.getJson(
      _gateway.resolve(path),
      authorization: auth,
    );
    if (result is GatewayHttpOk<Map<String, dynamic>>) return result.data;
    if (result is GatewayHttpFailure) {
      final retryable =
          result.error.statusCode == 0 ||
          result.error.statusCode == 429 ||
          result.error.statusCode >= 500;
      throw SdkAuthorizationException(
        retryable
            ? 'Voice is temporarily unavailable. You can retry the session resume.'
            : 'The linked session is no longer valid. Start again from the game.',
        canRetryResume: retryable,
      );
    }
    throw const SdkAuthorizationException('unexpected Auth response');
  }

  Future<Map<String, dynamic>> _post(
    String path,
    String? auth,
    Map<String, dynamic> body,
  ) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve(path),
      authorization: auth,
      body: body,
    );
    if (result is GatewayHttpOk<Map<String, dynamic>>) return result.data;
    if (result is GatewayHttpFailure) {
      final retryable =
          result.error.statusCode == 0 ||
          result.error.statusCode == 429 ||
          result.error.statusCode >= 500;
      throw SdkAuthorizationException(
        retryable
            ? 'Voice is temporarily unavailable.'
            : 'The authorization was denied. Start again from the game.',
        canRetryResume: retryable,
        presentation: retryable
            ? SdkAuthorizationErrorPresentation.temporary
            : SdkAuthorizationErrorPresentation.authorizationDenied,
      );
    }
    throw const SdkAuthorizationException(
      'unexpected Auth response',
      presentation: SdkAuthorizationErrorPresentation.invalidResponse,
    );
  }

  String _randomToken(int length) {
    const alphabet =
        'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';
    return List.generate(
      length,
      (_) => alphabet[_random.nextInt(alphabet.length)],
    ).join();
  }

  static String _base64Url(List<int> bytes) =>
      base64Url.encode(bytes).replaceAll('=', '');
  static String _sha(String value) =>
      sha256.convert(utf8.encode(value)).toString();
  static String _requestDigest(
    String key,
    String redirect,
    String challenge,
    String state,
    Set<String> scopes,
  ) => _sha(
    'voice-sdk-authorization-request-v1\n$key\n$redirect\n$challenge\n$state\n${(scopes.toList()..sort()).join(',')}',
  );

  static bool _validCode(String code) =>
      RegExp(r'^[A-Za-z0-9_-]{43}$').hasMatch(code);

  static void _validateRedirect(String redirect) {
    final uri = Uri.tryParse(redirect);
    if (uri == null ||
        uri.scheme != 'https' ||
        uri.host.isEmpty ||
        uri.userInfo.isNotEmpty ||
        uri.hasQuery ||
        uri.hasFragment ||
        uri.path != '/sdk/authorization/callback' ||
        uri.port != 443) {
      throw const SdkAuthorizationException(
        'redirect URI must be a canonical HTTPS callback',
      );
    }
  }

  static bool _sameCallbackEndpoint(Uri callback, String expected) {
    final redirect = Uri.parse(expected);
    return callback.scheme == redirect.scheme &&
        callback.host.toLowerCase() == redirect.host.toLowerCase() &&
        callback.port == redirect.port &&
        callback.path == redirect.path;
  }
}

String _requiredString(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is! String || value.isEmpty) {
    throw SdkAuthorizationException(
      'Auth response missing $key',
      presentation: SdkAuthorizationErrorPresentation.invalidResponse,
    );
  }
  return value;
}

int _requiredInt(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is! int || value < 1) {
    throw SdkAuthorizationException(
      'Auth response missing $key',
      presentation: SdkAuthorizationErrorPresentation.invalidResponse,
    );
  }
  return value;
}

Set<String> _stringSet(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is! List || value.any((item) => item is! String)) {
    throw SdkAuthorizationException(
      'Auth response has invalid $key',
      presentation: SdkAuthorizationErrorPresentation.invalidResponse,
    );
  }
  return value.cast<String>().toSet();
}

DateTime _date(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is! String) {
    throw SdkAuthorizationException(
      'Auth response missing $key',
      presentation: SdkAuthorizationErrorPresentation.invalidResponse,
    );
  }
  final parsed = DateTime.tryParse(value)?.toUtc();
  if (parsed == null) {
    throw SdkAuthorizationException(
      'Auth response has invalid $key',
      presentation: SdkAuthorizationErrorPresentation.invalidResponse,
    );
  }
  return parsed;
}
