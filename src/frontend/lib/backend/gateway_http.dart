import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:protobuf/protobuf.dart';

import 'client_version.dart';
import 'gateway_api_error.dart';
import 'gateway_request_id.dart';
import 'gateway_config.dart';
import 'gateway_proto_json.dart';

typedef GatewayUnauthorizedHandler =
    Future<bool> Function(GatewayRequestIdentity identity);
typedef GatewayUpgradeRequiredHandler = void Function(GatewayApiError error);
typedef AuthorizationProvider = String? Function();
typedef GatewayRequestIdentityProvider = GatewayRequestIdentity? Function();

final class GatewayRequestIdentity {
  const GatewayRequestIdentity({
    required this.generation,
    required this.accountId,
    required this.profileId,
  });

  final int generation;
  final String accountId;
  final String profileId;

  @override
  bool operator ==(Object other) =>
      other is GatewayRequestIdentity &&
      other.generation == generation &&
      other.accountId == accountId &&
      other.profileId == profileId;

  @override
  int get hashCode => Object.hash(generation, accountId, profileId);
}

sealed class GatewayHttpResult<T> {
  const GatewayHttpResult();
}

final class GatewayHttpOk<T> extends GatewayHttpResult<T> {
  const GatewayHttpOk(this.data, {this.statusCode});
  final T data;
  final int? statusCode;
}

final class GatewayHttpFailure extends GatewayHttpResult<Never> {
  const GatewayHttpFailure(this.error);
  final GatewayApiError error;
}

/// Shared HTTP client for API Gateway with unified errors and optional 401/426 hooks.
class GatewayHttpClient {
  GatewayHttpClient({
    required http.Client httpClient,
    required GatewayConfig config,
    AuthorizationProvider? authorizationProvider,
    GatewayRequestIdentityProvider? requestIdentityProvider,
    GatewayUnauthorizedHandler? onUnauthorized,
    GatewayUpgradeRequiredHandler? onUpgradeRequired,
  }) : _http = httpClient,
       _config = config,
       _authorizationProvider = authorizationProvider,
       _requestIdentityProvider = requestIdentityProvider,
       _onUnauthorized = onUnauthorized,
       _onUpgradeRequired = onUpgradeRequired;

  final http.Client _http;
  final GatewayConfig _config;
  final AuthorizationProvider? _authorizationProvider;
  final GatewayRequestIdentityProvider? _requestIdentityProvider;
  final GatewayUnauthorizedHandler? _onUnauthorized;
  final GatewayUpgradeRequiredHandler? _onUpgradeRequired;

  static const missingBaseUrl = GatewayApiError(
    errorCode: 'missing_base_url',
    message: 'missing base URL',
    statusCode: 0,
  );

  bool get hasBaseUrl => _config.hasBaseUrl;

  Uri resolve(String path) => Uri.parse(_config.baseUrl).resolve(path);

  Uri replace({required String path, Map<String, String>? queryParameters}) {
    return Uri.parse(
      _config.baseUrl,
    ).replace(path: path, queryParameters: queryParameters);
  }

  Future<GatewayHttpResult<T>> getProto<T extends GeneratedMessage>(
    Uri uri, {
    String? authorization,
    required T Function() createEmpty,
    bool allowNotFound = false,
    bool allowNoContent = false,
  }) {
    return _send(
      (preferProvider) => _http.get(
        uri,
        headers: _headers(
          authorization: authorization,
          preferProvider: preferProvider,
        ),
      ),
      createEmpty: createEmpty,
      allowNotFound: allowNotFound,
      allowNoContent: allowNoContent,
      isAuthRoute: _isAuthRoute(uri),
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<T>> postProto<T extends GeneratedMessage>({
    required Uri uri,
    String? authorization,
    GeneratedMessage? body,
    required T Function() createEmpty,
    bool allowNoContent = false,
  }) {
    return _send(
      (preferProvider) => _http.post(
        uri,
        headers: _headers(
          authorization: authorization,
          json: body != null,
          preferProvider: preferProvider,
        ),
        body: body == null ? null : encodeGatewayProto(body),
      ),
      createEmpty: createEmpty,
      allowNoContent: allowNoContent,
      isAuthRoute: _isAuthRoute(uri),
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<T>> patchProto<T extends GeneratedMessage>({
    required Uri uri,
    String? authorization,
    required GeneratedMessage body,
    required T Function() createEmpty,
    bool allowNoContent = false,
  }) {
    return _send(
      (preferProvider) => _http.patch(
        uri,
        headers: _headers(
          authorization: authorization,
          json: true,
          preferProvider: preferProvider,
        ),
        body: encodeGatewayProto(body),
      ),
      createEmpty: createEmpty,
      allowNoContent: allowNoContent,
      isAuthRoute: _isAuthRoute(uri),
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<void>> deleteEmpty({
    required Uri uri,
    String? authorization,
  }) {
    return _sendVoid(
      (preferProvider) => _http.delete(
        uri,
        headers: _headers(
          authorization: authorization,
          preferProvider: preferProvider,
        ),
      ),
      isAuthRoute: _isAuthRoute(uri),
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<void>> postEmpty({
    required Uri uri,
    String? authorization,
    Map<String, dynamic>? jsonBody,
  }) {
    return _sendVoid(
      (preferProvider) => _http.post(
        uri,
        headers: _headers(
          authorization: authorization,
          json: jsonBody != null,
          preferProvider: preferProvider,
        ),
        body: jsonBody == null ? null : jsonEncode(jsonBody),
      ),
      isAuthRoute: _isAuthRoute(uri),
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<Map<String, dynamic>>> getJson(
    Uri uri, {
    String? authorization,
  }) {
    return _sendJsonMap(
      (preferProvider) => _http.get(
        uri,
        headers: _headers(
          authorization: authorization,
          preferProvider: preferProvider,
        ),
      ),
      isAuthRoute: _isAuthRoute(uri),
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<Map<String, dynamic>>> postJson({
    required Uri uri,
    String? authorization,
    required Map<String, dynamic> body,
    bool allowNoContent = false,
  }) {
    return _sendJsonMap(
      (preferProvider) => _http.post(
        uri,
        headers: _headers(
          authorization: authorization,
          json: true,
          preferProvider: preferProvider,
        ),
        body: jsonEncode(body),
      ),
      isAuthRoute: _isAuthRoute(uri),
      allowNoContent: allowNoContent,
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<Map<String, dynamic>>> putJson({
    required Uri uri,
    String? authorization,
    required Map<String, dynamic> body,
    bool allowNoContent = false,
  }) {
    return _sendJsonMap(
      (preferProvider) => _http.put(
        uri,
        headers: _headers(
          authorization: authorization,
          json: true,
          preferProvider: preferProvider,
        ),
        body: jsonEncode(body),
      ),
      isAuthRoute: _isAuthRoute(uri),
      allowNoContent: allowNoContent,
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<Map<String, dynamic>>> patchJson({
    required Uri uri,
    String? authorization,
    required Map<String, dynamic> body,
    bool allowNoContent = false,
  }) {
    return _sendJsonMap(
      (preferProvider) => _http.patch(
        uri,
        headers: _headers(
          authorization: authorization,
          json: true,
          preferProvider: preferProvider,
        ),
        body: jsonEncode(body),
      ),
      isAuthRoute: _isAuthRoute(uri),
      allowNoContent: allowNoContent,
      authorization: authorization,
    );
  }

  Future<GatewayHttpResult<void>> putBytes({
    required Uri uri,
    required Map<String, String> headers,
    required List<int> bytes,
  }) async {
    try {
      final res = await _http.put(uri, headers: headers, body: bytes);
      final err = GatewayApiError.fromResponse(res);
      if (err == null) return const GatewayHttpOk(null);
      return GatewayHttpFailure(err);
    } catch (e) {
      return GatewayHttpFailure(
        GatewayApiError(
          errorCode: 'network_error',
          message: '$e',
          statusCode: 0,
        ),
      );
    }
  }

  Future<GatewayHttpResult<Uint8List>> getBytes({
    required Uri uri,
    int? maxBytes,
  }) async {
    Completer<void>? abort;
    try {
      if (maxBytes != null) {
        if (maxBytes <= 0 || maxBytes > 10 * 1024 * 1024) {
          return const GatewayHttpFailure(
            GatewayApiError(
              errorCode: 'download_size_invalid',
              message: 'Invalid download limit',
              statusCode: 0,
            ),
          );
        }
        abort = Completer<void>();
        final cancellation = abort;
        final request = http.AbortableRequest(
          'GET',
          uri,
          abortTrigger: cancellation.future,
        )..followRedirects = false;
        final response = await _http
            .send(request)
            .then((response) async {
              // Also retire a late response from an injected/custom client that
              // does not implement Abortable, without retaining signed URL errors.
              if (cancellation.isCompleted) {
                await response.stream.listen((_) {}).cancel();
                throw http.RequestAbortedException();
              }
              return response;
            })
            .timeout(
              const Duration(seconds: 30),
              onTimeout: () {
                if (!cancellation.isCompleted) cancellation.complete();
                throw TimeoutException('download_headers_timeout');
              },
            );
        if (response.statusCode < 200 || response.statusCode >= 300) {
          await response.stream.listen((_) {}).cancel();
          return GatewayHttpFailure(
            GatewayApiError(
              errorCode: 'download_failed',
              message: 'Download failed',
              statusCode: response.statusCode,
            ),
          );
        }
        if ((response.contentLength ?? 0) > maxBytes) {
          await response.stream.listen((_) {}).cancel();
          return const GatewayHttpFailure(
            GatewayApiError(
              errorCode: 'download_too_large',
              message: 'Download exceeds limit',
              statusCode: 0,
            ),
          );
        }
        final bytes = BytesBuilder(copy: false);
        final elapsed = Stopwatch()..start();
        await for (final chunk in response.stream.timeout(
          const Duration(seconds: 10),
        )) {
          if (elapsed.elapsed > const Duration(seconds: 30)) {
            return const GatewayHttpFailure(
              GatewayApiError(
                errorCode: 'download_timeout',
                message: 'Download timed out',
                statusCode: 0,
              ),
            );
          }
          if (bytes.length + chunk.length > maxBytes) {
            return const GatewayHttpFailure(
              GatewayApiError(
                errorCode: 'download_too_large',
                message: 'Download exceeds limit',
                statusCode: 0,
              ),
            );
          }
          bytes.add(chunk);
        }
        if ((response.headers['content-encoding'] ?? 'identity') ==
                'identity' &&
            response.contentLength != null &&
            response.contentLength != bytes.length) {
          return const GatewayHttpFailure(
            GatewayApiError(
              errorCode: 'download_incomplete',
              message: 'Incomplete download',
              statusCode: 0,
            ),
          );
        }
        return GatewayHttpOk(bytes.takeBytes());
      }
      final res = await _http.get(uri);
      if (res.statusCode >= 200 && res.statusCode < 300) {
        return GatewayHttpOk(Uint8List.fromList(res.bodyBytes));
      }
      final err = GatewayApiError.fromResponse(res);
      return GatewayHttpFailure(
        err ??
            GatewayApiError(
              errorCode: 'download_failed',
              message: 'HTTP ${res.statusCode}',
              statusCode: res.statusCode,
            ),
      );
    } catch (e) {
      return GatewayHttpFailure(
        GatewayApiError(
          errorCode: 'network_error',
          message: maxBytes == null ? '$e' : 'Download unavailable',
          statusCode: 0,
        ),
      );
    } finally {
      if (abort != null && !abort.isCompleted) abort.complete();
    }
  }

  Future<GatewayHttpResult<T>> _send<T extends GeneratedMessage>(
    Future<http.Response> Function(bool preferProvider) send, {
    required T Function() createEmpty,
    bool allowNotFound = false,
    bool allowNoContent = false,
    required bool isAuthRoute,
    bool retried = false,
    GatewayRequestIdentity? requestIdentity,
    String? authorization,
    bool? retryAllowed,
  }) async {
    requestIdentity ??= _requestIdentityProvider?.call();
    retryAllowed ??= _canRetryWithProvider(authorization);
    if (!_config.hasBaseUrl) {
      return const GatewayHttpFailure(missingBaseUrl);
    }
    try {
      var response = await send(retried);
      if (response.statusCode == 426) {
        final err = GatewayApiError.fromResponse(response)!;
        _onUpgradeRequired?.call(err);
        return GatewayHttpFailure(err);
      }
      if (response.statusCode == 401 && !retried && !isAuthRoute) {
        if (await _refreshForRequest(requestIdentity, retryAllowed)) {
          return _send(
            send,
            createEmpty: createEmpty,
            allowNotFound: allowNotFound,
            allowNoContent: allowNoContent,
            isAuthRoute: isAuthRoute,
            retried: true,
            requestIdentity: requestIdentity,
            authorization: authorization,
            retryAllowed: retryAllowed,
          );
        }
      }
      if (response.statusCode == 404 && allowNotFound) {
        return GatewayHttpOk(createEmpty());
      }
      if (response.statusCode == 204 && allowNoContent) {
        return GatewayHttpOk(createEmpty());
      }
      final err = GatewayApiError.fromResponse(response);
      if (err != null) {
        return GatewayHttpFailure(err);
      }
      return GatewayHttpOk(
        decodeGatewayProto(createEmpty, _responseBodyUtf8(response)),
      );
    } catch (e) {
      return GatewayHttpFailure(
        GatewayApiError(
          errorCode: 'network_error',
          message: '$e',
          statusCode: 0,
        ),
      );
    }
  }

  Future<GatewayHttpResult<Map<String, dynamic>>> _sendJsonMap(
    Future<http.Response> Function(bool preferProvider) send, {
    required bool isAuthRoute,
    bool allowNoContent = false,
    bool retried = false,
    GatewayRequestIdentity? requestIdentity,
    String? authorization,
    bool? retryAllowed,
  }) async {
    requestIdentity ??= _requestIdentityProvider?.call();
    retryAllowed ??= _canRetryWithProvider(authorization);
    if (!_config.hasBaseUrl) {
      return const GatewayHttpFailure(missingBaseUrl);
    }
    try {
      final response = await send(retried);
      if (response.statusCode == 426) {
        final err = GatewayApiError.fromResponse(response)!;
        _onUpgradeRequired?.call(err);
        return GatewayHttpFailure(err);
      }
      if (response.statusCode == 401 && !retried && !isAuthRoute) {
        if (await _refreshForRequest(requestIdentity, retryAllowed)) {
          return _sendJsonMap(
            send,
            isAuthRoute: isAuthRoute,
            allowNoContent: allowNoContent,
            retried: true,
            requestIdentity: requestIdentity,
            authorization: authorization,
            retryAllowed: retryAllowed,
          );
        }
      }
      if (response.statusCode == 204 && allowNoContent) {
        return GatewayHttpOk(
          const <String, dynamic>{},
          statusCode: response.statusCode,
        );
      }
      if (response.statusCode >= 200 && response.statusCode < 300) {
        final raw = _responseBodyUtf8(response);
        if (raw.trim().isEmpty) {
          return GatewayHttpOk(
            const <String, dynamic>{},
            statusCode: response.statusCode,
          );
        }
        final decoded = jsonDecode(raw);
        if (decoded is Map<String, dynamic>) {
          return GatewayHttpOk(decoded, statusCode: response.statusCode);
        }
        return GatewayHttpFailure(
          GatewayApiError(
            errorCode: 'invalid_response',
            message: 'expected JSON object',
            statusCode: response.statusCode,
          ),
        );
      }
      return GatewayHttpFailure(GatewayApiError.fromResponse(response)!);
    } catch (e) {
      return GatewayHttpFailure(
        GatewayApiError(
          errorCode: 'network_error',
          message: '$e',
          statusCode: 0,
        ),
      );
    }
  }

  Future<GatewayHttpResult<void>> _sendVoid(
    Future<http.Response> Function(bool preferProvider) send, {
    required bool isAuthRoute,
    bool retried = false,
    GatewayRequestIdentity? requestIdentity,
    String? authorization,
    bool? retryAllowed,
  }) async {
    requestIdentity ??= _requestIdentityProvider?.call();
    retryAllowed ??= _canRetryWithProvider(authorization);
    if (!_config.hasBaseUrl) {
      return const GatewayHttpFailure(missingBaseUrl);
    }
    try {
      final response = await send(retried);
      if (response.statusCode == 426) {
        final err = GatewayApiError.fromResponse(response)!;
        _onUpgradeRequired?.call(err);
        return GatewayHttpFailure(err);
      }
      if (response.statusCode == 401 && !retried && !isAuthRoute) {
        if (await _refreshForRequest(requestIdentity, retryAllowed)) {
          return _sendVoid(
            send,
            isAuthRoute: isAuthRoute,
            retried: true,
            requestIdentity: requestIdentity,
            authorization: authorization,
            retryAllowed: retryAllowed,
          );
        }
      }
      if (response.statusCode >= 200 && response.statusCode < 300) {
        return const GatewayHttpOk(null);
      }
      return GatewayHttpFailure(GatewayApiError.fromResponse(response)!);
    } catch (e) {
      return GatewayHttpFailure(
        GatewayApiError(
          errorCode: 'network_error',
          message: '$e',
          statusCode: 0,
        ),
      );
    }
  }

  bool _canRetryWithProvider(String? requestAuthorization) {
    final currentAuthorization = _authorizationProvider?.call();
    return currentAuthorization != null &&
        (requestAuthorization == null ||
            requestAuthorization == currentAuthorization);
  }

  Future<bool> _refreshForRequest(
    GatewayRequestIdentity? identity,
    bool? retryAllowed,
  ) async {
    final currentIdentity = _requestIdentityProvider?.call();
    final onUnauthorized = _onUnauthorized;
    if (retryAllowed != true ||
        identity == null ||
        currentIdentity != identity ||
        onUnauthorized == null) {
      return false;
    }
    final refreshed = await onUnauthorized(identity);
    return refreshed && _requestIdentityProvider?.call() == identity;
  }

  Map<String, String> _headers({
    String? authorization,
    bool json = false,
    bool preferProvider = false,
  }) {
    final auth = preferProvider
        ? _authorizationProvider?.call()
        : (authorization ?? _authorizationProvider?.call());
    return {
      ...ClientVersion.headers,
      'X-Request-Id': newGatewayRequestId(),
      'Authorization': ?auth,
      if (json) 'Content-Type': 'application/json',
    };
  }

  bool _isAuthRoute(Uri uri) {
    final path = uri.path;
    if (!path.startsWith('/api/v1/auth/')) return false;
    // Session-bearing auth endpoints should refresh the access token on 401.
    const sessionRoutes = <String>[
      '/api/v1/auth/convert-guest',
      '/api/v1/auth/switch-profile',
      '/api/v1/auth/logout',
      '/api/v1/auth/e2e-key-backup',
      '/api/v1/auth/2fa/enable',
      '/api/v1/auth/2fa/verify',
      '/api/v1/auth/2fa/disable',
    ];
    if (sessionRoutes.contains(path)) return false;
    if (path.startsWith('/api/v1/auth/linked-accounts')) return false;
    return true;
  }

  static String _responseBodyUtf8(http.Response response) {
    if (response.bodyBytes.isEmpty) return '';
    return utf8.decode(response.bodyBytes, allowMalformed: true);
  }
}
