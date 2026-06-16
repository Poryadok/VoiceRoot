package voice.backend.auth.oauth;

public record OAuthTokenRequest(
    String grantType,
    String code,
    String redirectUri,
    String clientId,
    String codeVerifier,
    String clientSecret) {}
