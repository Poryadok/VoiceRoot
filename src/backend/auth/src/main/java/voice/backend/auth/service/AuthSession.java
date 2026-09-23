package voice.backend.auth.service;

public record AuthSession(
    String accessToken,
    String refreshToken,
    long expiresInSeconds,
    String accountId,
    String profileId,
    String accountType,
    Boolean emailVerificationRequired) {
  public AuthSession(
      String accessToken,
      String refreshToken,
      long expiresInSeconds,
      String accountId,
      String profileId,
      String accountType) {
    this(accessToken, refreshToken, expiresInSeconds, accountId, profileId, accountType, null);
  }
}
