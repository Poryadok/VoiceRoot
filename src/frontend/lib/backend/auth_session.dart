/// Persisted auth session from Auth `SessionResponse` (register / login / refresh).
class AuthSession {
  const AuthSession({
    required this.accessToken,
    required this.refreshToken,
    required this.accountId,
    required this.activeProfileId,
    required this.expiresInSeconds,
  });

  final String accessToken;
  final String refreshToken;
  final String accountId;

  /// Active profile for API calls; matches JWT claim `profile_id` and Auth response field.
  final String activeProfileId;
  final int expiresInSeconds;

  String get authorizationHeader => 'Bearer $accessToken';

  Map<String, dynamic> toJson() => {
        'access_token': accessToken,
        'refresh_token': refreshToken,
        'account_id': accountId,
        'profile_id': activeProfileId,
        'expires_in_seconds': expiresInSeconds,
      };

  factory AuthSession.fromJson(Map<String, dynamic> json) {
    return AuthSession(
      accessToken: json['access_token'] as String,
      refreshToken: json['refresh_token'] as String,
      accountId: json['account_id'] as String,
      activeProfileId: json['profile_id'] as String,
      expiresInSeconds: (json['expires_in_seconds'] as num).toInt(),
    );
  }

  /// Parses Auth REST `SessionResponse` body.
  factory AuthSession.fromAuthResponse(Map<String, dynamic> json) =>
      AuthSession.fromJson(json);
}
