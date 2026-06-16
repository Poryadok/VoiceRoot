package voice.backend.auth.oauth;

public record OAuthAuthorizeRequest(
    String responseType,
    String clientId,
    String redirectUri,
    String state,
    String codeChallenge,
    String codeChallengeMethod) {}
