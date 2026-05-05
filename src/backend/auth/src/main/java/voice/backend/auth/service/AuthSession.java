package voice.backend.auth.service;

public record AuthSession(
    String accessToken, String refreshToken, long expiresInSeconds, String accountId, String profileId) {}
