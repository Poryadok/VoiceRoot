import 'dart:math';

/// Correlation id for API Gateway requests (hex, matches backend GenerateRequestID).
String newGatewayRequestId() {
  final random = Random.secure();
  final buffer = StringBuffer();
  for (var i = 0; i < 16; i++) {
    buffer.write(random.nextInt(256).toRadixString(16).padLeft(2, '0'));
  }
  return buffer.toString();
}
