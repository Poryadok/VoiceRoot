import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/sdk_authorization_client.dart';

void main() {
  const requestId = '11111111-1111-4111-8111-111111111111';
  const profileId = '22222222-2222-4222-8222-222222222222';
  const appId = '33333333-3333-4333-8333-333333333333';
  const environmentId = '44444444-4444-4444-8444-444444444444';
  const code = 'ccccccccccccccccccccccccccccccccccccccccccc';
  const voiceBearer = 'Bearer voice-session';
  const sdkBearer = 'Bearer sdk-session';
  const authKeyId = 'auth-issued-kid-1';
  const redirect = 'https://voice.example/sdk/authorization/callback';
  const scopes = <String>{'game.identity.read', 'game.chat.read'};

  late MemorySdkAuthorizationHandoffStorage handoffs;
  late MemorySdkLinkedSessionStorage sessions;
  late List<http.Request> requests;
  late List<String> signedPayloads;
  late String requestedState;
  var failLinkedResumeOnce = false;
  var failLinkedSigningOnce = false;
  var rejectExchangeOnce = false;
  late SdkAuthorizationClient client;

  setUp(() {
    handoffs = MemorySdkAuthorizationHandoffStorage();
    sessions = MemorySdkLinkedSessionStorage();
    requests = [];
    signedPayloads = [];
    requestedState = '';
    failLinkedResumeOnce = false;
    failLinkedSigningOnce = false;
    rejectExchangeOnce = false;
    final httpClient = MockClient((request) async {
      requests.add(request);
      final path = request.url.path;
      if (request.method == 'POST' && path.endsWith('/authorizations')) {
        requestedState =
            (jsonDecode(request.body) as Map<String, dynamic>)['state']
                as String;
        return http.Response(
          jsonEncode({
            'requestId': requestId,
            'policyRevision': 7,
            'expiresAt': '2026-10-01T00:05:00Z',
            'displayName': 'Example Game',
            'scopes': scopes.toList(),
          }),
          200,
        );
      }
      if (request.method == 'GET' && path.endsWith('/$requestId')) {
        return http.Response(
          jsonEncode({
            'requestId': requestId,
            'applicationId': appId,
            'environmentId': environmentId,
            'displayName': 'Example Game',
            'scopes': scopes.toList(),
            'gameSubject': 'player-42',
            'policyRevision': 7,
            'expiresAt': '2026-10-01T00:05:00Z',
          }),
          200,
        );
      }
      if (request.method == 'POST' && path.endsWith('/$requestId/approve')) {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body, {'profileId': profileId, 'policyRevision': 7});
        return http.Response(
          jsonEncode({
            'code': code,
            'redirectUri': '$redirect?code=$code&state=$requestedState',
            'expiresAt': '2026-10-01T00:01:00Z',
          }),
          200,
        );
      }
      if (request.method == 'POST' && path.endsWith('/$requestId/exchange')) {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body['code'], code);
        expect(body['redirectUri'], redirect);
        expect(body['codeVerifier'], isNotEmpty);
        expect(body['deviceProof'], 'signed-proof');
        if (rejectExchangeOnce) {
          rejectExchangeOnce = false;
          return http.Response('{"error":"invalid_authorization"}', 401);
        }
        return http.Response(
          jsonEncode({
            'sourceAccountId': '55555555-5555-4555-8555-555555555555',
            'accountId': '66666666-6666-4666-8666-666666666666',
            'profileId': profileId,
            'deviceId': '77777777-7777-4777-8777-777777777777',
            'applicationId': appId,
            'environmentId': environmentId,
            'scopes': scopes.toList(),
            'consentRevision': 2,
            'policyRevision': 7,
            'accessToken': 'linked-secret',
            'expiresAt': '2026-10-01T00:05:00Z',
          }),
          200,
        );
      }
      if (request.method == 'POST' && path.endsWith('/linked-session')) {
        expect(request.headers['authorization'], 'Bearer linked-secret');
        expect(jsonDecode(request.body), {'deviceProof': 'signed-proof'});
        if (failLinkedResumeOnce) {
          failLinkedResumeOnce = false;
          return http.Response('{"error":"temporary"}', 503);
        }
        return http.Response(
          jsonEncode({
            'sourceAccountId': '55555555-5555-4555-8555-555555555555',
            'accountId': '66666666-6666-4666-8666-666666666666',
            'profileId': profileId,
            'deviceId': '77777777-7777-4777-8777-777777777777',
            'applicationId': appId,
            'environmentId': environmentId,
            'scopes': scopes.toList(),
            'consentRevision': 2,
            'policyRevision': 7,
            'expiresAt': '2026-10-01T00:05:00Z',
          }),
          200,
        );
      }
      return http.Response('{"error":"not_found"}', 404);
    });
    final gateway = GatewayHttpClient(
      httpClient: httpClient,
      config: GatewayConfig(baseUrl: 'https://voice.example'),
    );
    client = SdkAuthorizationClient(
      gateway: gateway,
      handoffStorage: handoffs,
      linkedSessionStorage: sessions,
      proofSigner: (handle, payload) async {
        expect(handle, authKeyId);
        signedPayloads.add(payload);
        if (payload.startsWith('voice-sdk-linked-v1\n') &&
            failLinkedSigningOnce) {
          failLinkedSigningOnce = false;
          throw const SdkDeviceProofSigningException(isRetryable: true);
        }
        return 'signed-proof';
      },
    );
  });

  test('creates a PKCE request and persists its handoff securely', () async {
    final result = await client.startAuthorization(
      sdkAuthorization: sdkBearer,
      authKeyId: authKeyId,
      redirectUri: redirect,
      scopes: scopes,
    );

    expect(result.requestId, requestId);
    expect(result.displayName, 'Example Game');
    expect(requests.single.url.path, '/api/v1/auth/sdk/authorizations');
    expect(requests.single.headers['authorization'], sdkBearer);
    final body = jsonDecode(requests.single.body) as Map<String, dynamic>;
    expect(body.keys.toSet(), {
      'idempotencyKey',
      'redirectUri',
      'codeChallenge',
      'state',
      'scopes',
      'deviceProof',
    });
    expect(body['redirectUri'], redirect);
    expect(body['scopes'], scopes.toList()..sort());
    expect(body['state'], matches(RegExp(r'^[A-Za-z0-9_-]{43,128}$')));
    expect(body['codeChallenge'], matches(RegExp(r'^[A-Za-z0-9_-]{43}$')));
    final handoff = (await handoffs.readByRequestId(requestId))!;
    expect(
      body['codeChallenge'],
      base64Url
          .encode(sha256.convert(utf8.encode(handoff.codeVerifier)).bytes)
          .replaceAll('=', ''),
    );
    final digest = sha256.convert(
      utf8.encode(
        'voice-sdk-authorization-request-v1\n${body['idempotencyKey']}\n$redirect\n${body['codeChallenge']}\n${body['state']}\ngame.chat.read,game.identity.read',
      ),
    );
    final sourceHash = sha256.convert(utf8.encode('sdk-session'));
    expect(
      signedPayloads.single,
      'voice-sdk-authorize-v1\n$sourceHash\n$digest',
    );
  });

  test(
    'loads authoritative consent and completes one-use callback exchange',
    () async {
      await client.startAuthorization(
        sdkAuthorization: sdkBearer,
        authKeyId: authKeyId,
        redirectUri: redirect,
        scopes: scopes,
      );
      final handoff = (await handoffs.readByRequestId(requestId))!;
      final consent = await client.loadConsentView(
        requestId: requestId,
        voiceAuthorization: voiceBearer,
      );
      expect(consent.applicationId, appId);
      expect(consent.environmentId, environmentId);
      expect(consent.displayName, 'Example Game');
      expect(consent.gameSubject, 'player-42');
      expect(consent.scopes, scopes);

      // The approval redirect is passed directly to the callback consumer. The
      // authorization code never enters persistent handoff storage.
      final approvalResponse = await client.approveAuthorization(
        requestId: requestId,
        voiceAuthorization: voiceBearer,
        profileId: profileId,
        policyRevision: 7,
      );
      final linked = await client.acceptCallbackAndResume(
        Uri.parse(approvalResponse.redirectUri),
      );
      expect(linked.profileId, profileId);
      expect(linked.scopes, scopes);
      expect(await handoffs.readByRequestId(requestId), isNull);
      expect(await sessions.read(), isNull);
      expect(
        requests.map((request) => request.url.path),
        containsAll([
          '/api/v1/auth/sdk/authorizations/$requestId/exchange',
          '/api/v1/auth/sdk/authorizations/linked-session',
        ]),
      );
      expect(
        signedPayloads[1],
        'voice-sdk-code-v1\n$requestId\n${sha256.convert(utf8.encode(code))}\n${sha256.convert(utf8.encode(handoff.codeVerifier))}',
      );
      expect(
        signedPayloads[2],
        'voice-sdk-linked-v1\n${sha256.convert(utf8.encode('linked-secret'))}',
      );

      await expectLater(
        client.acceptCallbackAndResume(Uri.parse(approvalResponse.redirectUri)),
        throwsA(isA<SdkAuthorizationException>()),
      );
    },
  );

  test(
    'keeps only the short-lived secure bootstrap for resume retry',
    () async {
      await client.startAuthorization(
        sdkAuthorization: sdkBearer,
        authKeyId: authKeyId,
        redirectUri: redirect,
        scopes: scopes,
      );
      final approval = await client.approveAuthorization(
        requestId: requestId,
        voiceAuthorization: voiceBearer,
        profileId: profileId,
        policyRevision: 7,
      );
      failLinkedResumeOnce = true;
      await expectLater(
        client.acceptCallbackAndResume(Uri.parse(approval.redirectUri)),
        throwsA(isA<SdkAuthorizationException>()),
      );
      expect(await handoffs.readByRequestId(requestId), isNull);
      expect(await sessions.read(), isNotNull);
      final resumed = await client.resumePendingLinkedSession();
      expect(resumed.profileId, profileId);
      expect(await sessions.read(), isNull);
    },
  );

  test(
    'retains linked credential after transient signer failure and retries resume only',
    () async {
      await client.startAuthorization(
        sdkAuthorization: sdkBearer,
        authKeyId: authKeyId,
        redirectUri: redirect,
        scopes: scopes,
      );
      final approval = await client.approveAuthorization(
        requestId: requestId,
        voiceAuthorization: voiceBearer,
        profileId: profileId,
        policyRevision: 7,
      );
      failLinkedSigningOnce = true;

      await expectLater(
        client.acceptCallbackAndResume(Uri.parse(approval.redirectUri)),
        throwsA(
          isA<SdkAuthorizationException>().having(
            (error) => error.canRetryResume,
            'canRetryResume',
            isTrue,
          ),
        ),
      );
      expect(await sessions.read(), isNotNull);
      expect(
        requests.where((request) => request.url.path.endsWith('/exchange')),
        hasLength(1),
      );
      expect(
        requests.where(
          (request) => request.url.path.endsWith('/linked-session'),
        ),
        isEmpty,
      );

      final resumed = await client.resumePendingLinkedSession();
      expect(resumed.profileId, profileId);
      expect(await sessions.read(), isNull);
      expect(
        requests.where((request) => request.url.path.endsWith('/exchange')),
        hasLength(1),
      );
      expect(
        requests.where(
          (request) => request.url.path.endsWith('/linked-session'),
        ),
        hasLength(1),
      );
    },
  );

  test('clears linked credential after definitive signer failure', () async {
    await sessions.write(
      SdkLinkedSessionCredential(
        token: 'linked-secret',
        authKeyId: authKeyId,
        expiresAt: DateTime.now().toUtc().add(const Duration(minutes: 5)),
      ),
    );
    client = SdkAuthorizationClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          fail('definitive signer failure must not call Auth');
        }),
        config: GatewayConfig(baseUrl: 'https://voice.example'),
      ),
      handoffStorage: handoffs,
      linkedSessionStorage: sessions,
      proofSigner: (_, _) async =>
          throw const SdkDeviceProofSigningException(isRetryable: false),
    );

    await expectLater(
      client.resumePendingLinkedSession(),
      throwsA(
        isA<SdkAuthorizationException>().having(
          (error) => error.canRetryResume,
          'canRetryResume',
          isFalse,
        ),
      ),
    );
    expect(await sessions.read(), isNull);
  });

  test('rejects foreign redirects and wrong state before exchange', () async {
    await client.startAuthorization(
      sdkAuthorization: sdkBearer,
      authKeyId: authKeyId,
      redirectUri: redirect,
      scopes: scopes,
    );
    final record = await handoffs.readByRequestId(requestId);
    final state = record!.state;
    final before = requests.length;

    for (final callback in [
      Uri.parse(
        'https://foreign.example/sdk/authorization/callback?code=$code&state=$state',
      ),
      Uri.parse('$redirect?code=$code&state=wrong'),
      Uri.parse('$redirect?code=$code&state=$state&extra=1'),
    ]) {
      await expectLater(
        client.acceptCallbackAndResume(callback),
        throwsA(isA<SdkAuthorizationException>()),
      );
    }
    expect(requests.length, before);
  });

  test(
    'Auth rejects foreign code without creating linked session state',
    () async {
      await client.startAuthorization(
        sdkAuthorization: sdkBearer,
        authKeyId: authKeyId,
        redirectUri: redirect,
        scopes: scopes,
      );
      final approval = await client.approveAuthorization(
        requestId: requestId,
        voiceAuthorization: voiceBearer,
        profileId: profileId,
        policyRevision: 7,
      );
      final approvedCallback = Uri.parse(approval.redirectUri);
      final foreignCodeCallback = approvedCallback.replace(
        queryParameters: {
          'code': 'ddddddddddddddddddddddddddddddddddddddddddd',
          'state': approvedCallback.queryParameters['state']!,
        },
      );
      rejectExchangeOnce = true; // Auth rejects the foreign code/target proof.
      await expectLater(
        client.acceptCallbackAndResume(foreignCodeCallback),
        throwsA(isA<SdkAuthorizationException>()),
      );
      expect(
        requests.where(
          (request) => request.url.path.endsWith('/linked-session'),
        ),
        isEmpty,
      );
      expect(await sessions.read(), isNull);
    },
  );
}

class MemorySdkAuthorizationHandoffStorage
    implements SdkAuthorizationHandoffStorage {
  final rawValues = <String, String>{};

  @override
  Future<void> pruneExpired() async {}

  @override
  Future<SdkAuthorizationHandoff?> readByRequestId(String requestId) async {
    for (final raw in rawValues.values) {
      final json = jsonDecode(raw) as Map<String, dynamic>;
      if (json['requestId'] == requestId) {
        return SdkAuthorizationHandoff.fromJson(json);
      }
    }
    return null;
  }

  @override
  Future<SdkAuthorizationHandoff?> readByState(String state) async {
    final raw = rawValues[state];
    if (raw == null) return null;
    return SdkAuthorizationHandoff.fromJson(
      jsonDecode(raw) as Map<String, dynamic>,
    );
  }

  @override
  Future<void> write(SdkAuthorizationHandoff handoff) async {
    rawValues[handoff.state] = jsonEncode(handoff.toJson());
  }

  @override
  Future<void> deleteByState(String state) async {
    rawValues.remove(state);
  }
}

class MemorySdkLinkedSessionStorage implements SdkLinkedSessionStorage {
  SdkLinkedSessionCredential? value;

  @override
  Future<void> pruneExpired() async {}

  @override
  Future<SdkLinkedSessionCredential?> read() async => value;

  @override
  Future<void> write(SdkLinkedSessionCredential credential) async {
    value = credential;
  }

  @override
  Future<void> clear() async {
    value = null;
  }
}
