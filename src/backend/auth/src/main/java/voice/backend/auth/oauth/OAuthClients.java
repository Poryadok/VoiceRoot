package voice.backend.auth.oauth;

import java.time.Duration;
import java.util.List;
import voice.backend.auth.config.AuthProperties;

public final class OAuthClients {
  private OAuthClients() {}

  public record Resolved(
      String clientId, String clientSecret, List<String> redirectUris, Duration authorizationCodeTtl) {}

  public static Resolved resolve(AuthProperties properties, String clientId) {
    if (clientId == null || clientId.isBlank()) {
      throw new OAuthException("invalid_client", 401);
    }
    var oauth = properties.getOauth();
    if (oauth.getDeveloperPortal().isEnabled()
        && clientId.equals(oauth.getDeveloperPortal().getClientId())) {
      return fromSettings(oauth.getDeveloperPortal());
    }
    if (oauth.getAdmin().isEnabled() && clientId.equals(oauth.getAdmin().getClientId())) {
      return fromSettings(oauth.getAdmin());
    }
    throw new OAuthException("invalid_client", 401);
  }

  public static boolean anyEnabled(AuthProperties properties) {
    return properties.getOauth().getDeveloperPortal().isEnabled()
        || properties.getOauth().getAdmin().isEnabled();
  }

  private static Resolved fromSettings(AuthProperties.OAuthClientSettings settings) {
    return new Resolved(
        settings.getClientId(),
        settings.getClientSecret(),
        settings.getRedirectUris(),
        settings.getAuthorizationCodeTtl());
  }
}
