package voice.backend.auth.oauth;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Base64;

public final class PkceVerifier {
  private PkceVerifier() {}

  public static String s256Challenge(String codeVerifier) {
    try {
      byte[] digest = MessageDigest.getInstance("SHA-256").digest(codeVerifier.getBytes(StandardCharsets.US_ASCII));
      return Base64.getUrlEncoder().withoutPadding().encodeToString(digest);
    } catch (NoSuchAlgorithmException ex) {
      throw new IllegalStateException("SHA-256 unavailable", ex);
    }
  }

  public static boolean verifyS256(String codeVerifier, String codeChallenge) {
    if (codeVerifier == null || codeVerifier.isBlank() || codeChallenge == null || codeChallenge.isBlank()) {
      return false;
    }
    return s256Challenge(codeVerifier).equals(codeChallenge);
  }
}
