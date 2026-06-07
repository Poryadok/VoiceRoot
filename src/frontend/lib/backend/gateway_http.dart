import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:protobuf/protobuf.dart';

import 'client_version.dart';
import 'gateway_api_error.dart';
import 'gateway_request_id.dart';
import 'gateway_config.dart';
import 'gateway_proto_json.dart';

typedef GatewayUnauthorizedHandler = Future<bool> Function();
typedef GatewayUpgradeRequiredHandler = void Function(GatewayApiError error);
typedef AuthorizationProvider = String? Function();

sealed class GatewayHttpResult<T> {
  const GatewayHttpResult();
}

final class GatewayHttpOk<T> extends GatewayHttpResult<T> {
  const GatewayHttpOk(this.data);
  final T data;
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
    GatewayUnauthorizedHandler? onUnauthorized,
    GatewayUpgradeRequiredHandler? onUpgradeRequired,
  }) : _http = httpClient,
       _config = config,
       _authorizationProvider = authorizationProvider,
       _onUnauthorized = onUnauthorized,
       _onUpgradeRequired = onUpgradeRequired;

  final http.Client _http;
  final GatewayConfig _config;
  final AuthorizationProvider? _authorizationProvider;
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
    return Uri.parse(_config.baseUrl).replace(
      path: path,
      queryParameters: queryParameters,
    );
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
    );
  }

  Future<GatewayHttpResult<T>> patchProto<T extends GeneratedMessage>({
    required Uri uri,
    String? authorization,
    required GeneratedMessage body,
    required T Function() createEmpty,
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
      isAuthRoute: _isAuthRoute(uri),
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

  Future<GatewayHttpResult<T>> _send<T extends GeneratedMessage>(
    Future<http.Response> Function(bool preferProvider) send, {
    required T Function() createEmpty,
    bool allowNotFound = false,
    bool allowNoContent = false,
    required bool isAuthRoute,
    bool retried = false,
  }) async {
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
      if (response.statusCode == 401 &&
          !retried &&
          !isAuthRoute &&
          _onUnauthorized != null) {
        final refreshed = await _onUnauthorized!();
        if (refreshed) {
          return _send(
            send,
            createEmpty: createEmpty,
            allowNotFound: allowNotFound,
            allowNoContent: allowNoContent,
            isAuthRoute: isAuthRoute,
            retried: true,
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
      return GatewayHttpOk(decodeGatewayProto(createEmpty, response.body));
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
  }) async {
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
      if (response.statusCode == 401 &&
          !retried &&
          !isAuthRoute &&
          _onUnauthorized != null) {
        final refreshed = await _onUnauthorized!();
        if (refreshed) {
          return _sendVoid(send, isAuthRoute: isAuthRoute, retried: true);
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

  Map<String, String> _headers({
    String? authorization,
    bool json = false,
    bool preferProvider = false,
  }) {
    final auth = preferProvider
        ? (_authorizationProvider?.call() ?? authorization)
        : (authorization ?? _authorizationProvider?.call());
    return {
      ...ClientVersion.headers,
      'X-Request-Id': newGatewayRequestId(),
      if (auth != null) 'Authorization': auth,
      if (json) 'Content-Type': 'application/json',
    };
  }

  bool _isAuthRoute(Uri uri) => uri.path.contains('/api/v1/auth/');
}
