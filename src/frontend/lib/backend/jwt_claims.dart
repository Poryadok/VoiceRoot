import 'dart:convert';

/// Best-effort JWT payload decode for client-side claims (no signature verify).
Map<String, dynamic>? decodeJwtPayload(String token) {
  final parts = token.split('.');
  if (parts.length < 2) return null;
  try {
    final normalized = base64Url.normalize(parts[1]);
    final decoded = utf8.decode(base64Url.decode(normalized));
    final json = jsonDecode(decoded);
    return json is Map<String, dynamic> ? json : null;
  } catch (_) {
    return null;
  }
}

String? subscriptionTierFromAccessToken(String token) {
  final payload = decodeJwtPayload(token);
  final tier = payload?['subscription_tier'];
  if (tier is String && tier.isNotEmpty) return tier;
  return null;
}
