package voice.backend.auth.oauth;

import java.net.URI;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.AuthSession;
import voice.backend.auth.service.LoginCommand;

public class OAuth2Service {
  private final AuthProperties properties;
  private final AuthService authService;
  private final OAuthAuthorizationCodeStore codeStore;
  private final Clock clock;
  private final SecureRandom secureRandom = new SecureRandom();

  public OAuth2Service(
      AuthProperties properties,
      AuthService authService,
      OAuthAuthorizationCodeStore codeStore,
      Clock clock) {
    this.properties = properties;
    this.authService = authService;
    this.codeStore = codeStore;
    this.clock = clock;
  }

  public boolean isEnabled() {
    return developerPortal().isEnabled();
  }

  public void ensureEnabled() {
    if (!isEnabled()) {
      throw new OAuthException("oauth_disabled", 404);
    }
  }

  public void validateAuthorizeRequest(OAuthAuthorizeRequest request) {
    ensureEnabled();
    if (!"code".equals(request.responseType())) {
      throw new OAuthException("unsupported_response_type", 400);
    }
    validateClient(request.clientId());
    validateRedirectUri(request.redirectUri());
    if (request.codeChallenge() == null || request.codeChallenge().isBlank()) {
      throw new OAuthException("invalid_request", 400);
    }
    if (!"S256".equalsIgnoreCase(request.codeChallengeMethod())) {
      throw new OAuthException("invalid_request", 400);
    }
  }

  public String loginFormHtml(OAuthAuthorizeRequest request) {
    validateAuthorizeRequest(request);
    return """
        <!DOCTYPE html>
        <html lang="en">
        <head><meta charset="utf-8"><title>Voice Sign In</title></head>
        <body>
        <h1>Sign in to Voice</h1>
        <form method="post" action="/api/v1/auth/oauth2/authorize">
          <input type="hidden" name="response_type" value="%s"/>
          <input type="hidden" name="client_id" value="%s"/>
          <input type="hidden" name="redirect_uri" value="%s"/>
          <input type="hidden" name="state" value="%s"/>
          <input type="hidden" name="code_challenge" value="%s"/>
          <input type="hidden" name="code_challenge_method" value="%s"/>
          <label>Email <input name="email" type="email" required autocomplete="username"/></label><br/>
          <label>Password <input name="password" type="password" required autocomplete="current-password"/></label><br/>
          <label>TOTP (optional) <input name="totp_code" type="text" inputmode="numeric" autocomplete="one-time-code"/></label><br/>
          <button type="submit">Sign in</button>
        </form>
        </body>
        </html>
        """
        .formatted(
            esc(request.responseType()),
            esc(request.clientId()),
            esc(request.redirectUri()),
            esc(request.state()),
            esc(request.codeChallenge()),
            esc(request.codeChallengeMethod()));
  }

  public URI completeAuthorizeAfterLogin(OAuthAuthorizeRequest request, LoginCommand login) {
    validateAuthorizeRequest(request);
    AuthSession session;
    try {
      session = authService.login(login);
    } catch (AuthException ex) {
      throw new OAuthException(ex.getMessage(), 401);
    }
    String code = randomCode();
    Instant expiresAt = Instant.now(clock).plus(developerPortal().getAuthorizationCodeTtl());
    OAuthAuthorizationCode record =
        new OAuthAuthorizationCode(
            code,
            session.accountId(),
            session.profileId(),
            request.clientId(),
            request.redirectUri(),
            request.codeChallenge(),
            request.codeChallengeMethod(),
            expiresAt);
    codeStore.save(record, developerPortal().getAuthorizationCodeTtl());
    return buildRedirect(request.redirectUri(), code, request.state());
  }

  public OAuthTokenResponse exchangeAuthorizationCode(OAuthTokenRequest request) {
    ensureEnabled();
    if (!"authorization_code".equals(request.grantType())) {
      throw new OAuthException("unsupported_grant_type", 400);
    }
    validateClient(request.clientId());
    validateClientSecret(request.clientId(), request.clientSecret());
    OAuthAuthorizationCode record =
        codeStore
            .consume(request.code())
            .orElseThrow(() -> new OAuthException("invalid_grant", 401));
    if (record.isExpired(Instant.now(clock))) {
      throw new OAuthException("invalid_grant", 401);
    }
    if (!record.clientId().equals(request.clientId()) || !record.redirectUri().equals(request.redirectUri())) {
      throw new OAuthException("invalid_grant", 401);
    }
    if (!PkceVerifier.verifyS256(request.codeVerifier(), record.codeChallenge())) {
      throw new OAuthException("invalid_grant", 401);
    }
    String accessToken = authService.issueOAuthAccessToken(record.accountId(), record.profileId());
    return new OAuthTokenResponse(accessToken, "Bearer", authService.accessTokenTtlSeconds());
  }

  public Map<String, String> openIdConfiguration() {
    ensureEnabled();
    String base = properties.getOauth().getPublicApiBaseUrl().replaceAll("/$", "");
    return Map.of(
        "issuer",
        properties.getJwt().getIssuer(),
        "authorization_endpoint",
        base + "/api/v1/auth/oauth2/authorize",
        "token_endpoint",
        base + "/api/v1/auth/oauth2/token",
        "jwks_uri",
        base + "/api/v1/auth/.well-known/jwks.json");
  }

  private AuthProperties.DeveloperPortalOAuth developerPortal() {
    return properties.getOauth().getDeveloperPortal();
  }

  private void validateClient(String clientId) {
    if (clientId == null || !clientId.equals(developerPortal().getClientId())) {
      throw new OAuthException("invalid_client", 401);
    }
  }

  private void validateRedirectUri(String redirectUri) {
    if (redirectUri == null || redirectUri.isBlank()) {
      throw new OAuthException("invalid_redirect_uri", 400);
    }
    List<String> allowed = developerPortal().getRedirectUris();
    if (allowed == null || allowed.stream().noneMatch(redirectUri::equals)) {
      throw new OAuthException("invalid_redirect_uri", 400);
    }
  }

  private void validateClientSecret(String clientId, String clientSecret) {
    String configured = developerPortal().getClientSecret();
    if (configured == null || configured.isBlank()) {
      return;
    }
    if (clientSecret == null || !configured.equals(clientSecret)) {
      throw new OAuthException("invalid_client", 401);
    }
  }

  private static URI buildRedirect(String redirectUri, String code, String state) {
    StringBuilder url = new StringBuilder(redirectUri);
    url.append(redirectUri.contains("?") ? "&" : "?");
    url.append("code=").append(encode(code));
    if (state != null && !state.isBlank()) {
      url.append("&state=").append(encode(state));
    }
    return URI.create(url.toString());
  }

  private String randomCode() {
    byte[] bytes = new byte[32];
    secureRandom.nextBytes(bytes);
    return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
  }

  private static String encode(String value) {
    return URLEncoder.encode(value, StandardCharsets.UTF_8);
  }

  private static String esc(String value) {
    if (value == null) {
      return "";
    }
    return value
        .replace("&", "&amp;")
        .replace("\"", "&quot;")
        .replace("<", "&lt;")
        .replace(">", "&gt;");
  }
}
