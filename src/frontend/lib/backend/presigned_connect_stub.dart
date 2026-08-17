import 'dart:typed_data';

import 'gateway_api_error.dart';
import 'gateway_http.dart';

/// Web/stub: cannot set a signed Host header distinct from the connect URI.
Future<GatewayHttpResult<void>> putBytesPreservingHost({
  required Uri connectUri,
  required String signedHostHeader,
  required Map<String, String> headers,
  required List<int> bytes,
}) async {
  return GatewayHttpFailure(
    GatewayApiError(
      errorCode: 'presign_host_unsupported',
      message:
          'Cannot PUT presigned URL: signed host $signedHostHeader differs from ${connectUri.host}',
      statusCode: 0,
    ),
  );
}

Future<GatewayHttpResult<Uint8List>> getBytesPreservingHost({
  required Uri connectUri,
  required String signedHostHeader,
}) async {
  return GatewayHttpFailure(
    GatewayApiError(
      errorCode: 'presign_host_unsupported',
      message:
          'Cannot GET presigned URL: signed host $signedHostHeader differs from ${connectUri.host}',
      statusCode: 0,
    ),
  );
}
