import 'dart:typed_data';

import 'gateway_http.dart';
import 'presigned_connect_stub.dart'
    if (dart.library.io) 'presigned_connect_io.dart' as connect;

/// Maps compose object-storage hosts so host-side clients can PUT/GET MinIO.
///
/// File/User sign URLs with docker-compose endpoints (`host.docker.internal` or
/// `minio`). Host Flutter must connect via `127.0.0.1` (see docs/TESTING.md)
/// while keeping the signed `Host` header.
Uri rewritePresignedUrlForHost(Uri uri) {
  if (uri.host == 'host.docker.internal' || uri.host == 'minio') {
    return uri.replace(host: '127.0.0.1');
  }
  return uri;
}

/// Connect URI plus optional signed Host when it differs from the connect host.
class PresignConnectTarget {
  const PresignConnectTarget({
    required this.connectUri,
    this.signedHostHeader,
  });

  final Uri connectUri;
  final String? signedHostHeader;
}

String signedAuthority(Uri uri) {
  if (!uri.hasPort) {
    return uri.host;
  }
  final defaultPort = (uri.scheme == 'https' && uri.port == 443) ||
      (uri.scheme == 'http' && uri.port == 80);
  if (defaultPort) {
    return uri.host;
  }
  return '${uri.host}:${uri.port}';
}

PresignConnectTarget presignConnectTarget(Uri signedUrl) {
  final connectUri = rewritePresignedUrlForHost(signedUrl);
  if (connectUri.host == signedUrl.host && connectUri.port == signedUrl.port) {
    return PresignConnectTarget(connectUri: connectUri);
  }
  return PresignConnectTarget(
    connectUri: connectUri,
    signedHostHeader: signedAuthority(signedUrl),
  );
}

/// PUT bytes to a presigned URL (R2 / MinIO / avatar upload).
///
/// When the signed host is a compose-internal name, connect to localhost but
/// send the original `Host` so SigV4 still matches.
Future<GatewayHttpResult<void>> putPresigned({
  required GatewayHttpClient gateway,
  required Uri uploadUrl,
  required Map<String, String> requiredHeaders,
  required List<int> bytes,
}) {
  final target = presignConnectTarget(uploadUrl);
  final signedHost = target.signedHostHeader;
  if (signedHost == null) {
    return gateway.putBytes(
      uri: target.connectUri,
      headers: requiredHeaders,
      bytes: bytes,
    );
  }
  return connect.putBytesPreservingHost(
    connectUri: target.connectUri,
    signedHostHeader: signedHost,
    headers: requiredHeaders,
    bytes: bytes,
  );
}

/// GET bytes from a presigned URL, preserving signed Host when rewritten.
Future<GatewayHttpResult<Uint8List>> getPresignedBytes({
  required GatewayHttpClient gateway,
  required Uri downloadUrl,
}) {
  final target = presignConnectTarget(downloadUrl);
  final signedHost = target.signedHostHeader;
  if (signedHost == null) {
    return gateway.getBytes(uri: target.connectUri);
  }
  return connect.getBytesPreservingHost(
    connectUri: target.connectUri,
    signedHostHeader: signedHost,
  );
}
