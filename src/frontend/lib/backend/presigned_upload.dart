import 'gateway_http.dart';

/// PUT bytes to a presigned URL (R2 / avatar upload).
Future<GatewayHttpResult<void>> putPresigned({
  required GatewayHttpClient gateway,
  required Uri uploadUrl,
  required Map<String, String> requiredHeaders,
  required List<int> bytes,
}) {
  return gateway.putBytes(
    uri: uploadUrl,
    headers: requiredHeaders,
    bytes: bytes,
  );
}
