package voice.backend.auth.oauth;

public record OAuthTokenResponse(String accessToken, String tokenType, long expiresIn) {}
