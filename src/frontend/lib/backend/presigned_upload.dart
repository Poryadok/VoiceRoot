import 'gateway_http.dart';

/// Maps compose object-storage hosts so host-side clients can PUT/GET MinIO.
///
/// File/User sign URLs with docker-compose endpoints (`host.docker.internal` or
/// `minio`). Host Flutter must use `127.0.0.1` (see docs/TESTING.md).
Uri rewritePresignedUrlForHost(Uri uri) {
  if (uri.host == 'host.docker.internal' || uri.host == 'minio') {
    return uri.replace(host: '127.0.0.1');
  }
  return uri;
}

/// PUT bytes to a presigned URL (R2 / avatar upload).
Future<GatewayHttpResult<void>> putPresigned({
  required GatewayHttpClient gateway,
  required Uri uploadUrl,
  required Map<String, String> requiredHeaders,
  required List<int> bytes,
}) {
  return gateway.putBytes(
    uri: rewritePresignedUrlForHost(uploadUrl),
    headers: requiredHeaders,
    bytes: bytes,
  );
}
