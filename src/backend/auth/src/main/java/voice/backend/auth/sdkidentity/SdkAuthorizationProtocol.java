package voice.backend.auth.sdkidentity;

import java.net.URI;
import java.net.URISyntaxException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Base64;
import java.util.HexFormat;
import java.util.Set;
import java.util.TreeSet;
import java.util.UUID;

/** Canonical request binding and strict S256 PKCE for browser SDK authorization. */
public final class SdkAuthorizationProtocol {
  private static final Set<String> PLAYER_SCOPES = Set.of("game.identity.read", "game.chat.read",
      "game.chat.send", "game.voice.join", "game.presence.write", "game.invites.create");

  private SdkAuthorizationProtocol() {}

  public static String requestHash(UUID idempotencyKey, String redirectUri, String codeChallenge,
                                   String state, Set<String> scopes) {
    require(idempotencyKey != null);
    requireRedirect(redirectUri);
    require(base64Url(codeChallenge, 43, 43));
    require(base64Url(state, 43, 128));
    requireScopes(scopes);
    String canonical = "voice-sdk-authorization-request-v1\n" + idempotencyKey + "\n"
        + redirectUri + "\n" + codeChallenge + "\n" + state + "\n"
        + String.join(",", new TreeSet<>(scopes));
    return HexFormat.of().formatHex(sha256(canonical));
  }

  public static boolean verifyPkce(String verifier, String challenge) {
    if (verifier == null || verifier.length() < 43 || verifier.length() > 128
        || !verifier.matches("[A-Za-z0-9._~-]+") || !base64Url(challenge, 43, 43)) return false;
    String expected = Base64.getUrlEncoder().withoutPadding().encodeToString(sha256(verifier));
    return MessageDigest.isEqual(expected.getBytes(StandardCharsets.US_ASCII),
        challenge.getBytes(StandardCharsets.US_ASCII));
  }

  public static void requirePolicy(SdkAuthorizationPolicy.Policy policy, UUID applicationId,
                                   UUID environmentId, String redirectUri, Set<String> scopes) {
    require(policy != null && applicationId != null && environmentId != null);
    require(applicationId.equals(policy.applicationId()) && environmentId.equals(policy.environmentId())
        && policy.revision() > 0 && policy.displayName() != null && !policy.displayName().isBlank());
    requireRedirect(redirectUri);
    requireScopes(scopes);
    requireScopes(policy.playerScopes());
    require(policy.redirectUris() != null && !policy.redirectUris().isEmpty());
    for (String allowed : policy.redirectUris()) requireRedirect(allowed);
    require(policy.redirectUris().contains(redirectUri) && policy.playerScopes().containsAll(scopes));
  }

  private static void requireRedirect(String redirect) {
    require(redirect != null && !redirect.isBlank());
    try {
      URI uri = new URI(redirect);
      require(uri.isAbsolute() && uri.getRawFragment() == null && uri.getRawUserInfo() == null
          && (uri.getRawAuthority() == null || !uri.getRawAuthority().contains("@"))
          && redirect.indexOf('\r') < 0 && redirect.indexOf('\n') < 0);
    } catch (URISyntaxException malformed) {
      throw new SdkIdentityDeniedException();
    }
  }

  private static void requireScopes(Set<String> scopes) {
    require(scopes != null && !scopes.isEmpty() && scopes.size() <= 16);
    for (String scope : scopes) require(scope != null && PLAYER_SCOPES.contains(scope));
  }

  private static boolean base64Url(String value, int minimum, int maximum) {
    return value != null && value.length() >= minimum && value.length() <= maximum
        && value.matches("[A-Za-z0-9_-]+");
  }

  private static byte[] sha256(String value) {
    try {
      return MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8));
    } catch (NoSuchAlgorithmException impossible) {
      throw new IllegalStateException(impossible);
    }
  }

  private static void require(boolean valid) {
    if (!valid) throw new SdkIdentityDeniedException();
  }
}
