package voice.backend.auth.sdkidentity;

import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.util.Base64;
import java.util.Arrays;
import java.util.UUID;
import javax.crypto.Cipher;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;

/** Auth-only AES-GCM envelope for replaying an approved one-use binding code after a lost response. */
public final class AuthGameBindingApprovalCodeVault {
  public record Sealed(byte[] nonce, byte[] ciphertext) {}
  private final byte[] key;
  private final SecureRandom random = new SecureRandom();

  public AuthGameBindingApprovalCodeVault(byte[] key) {
    if (key == null || key.length != 32 || Arrays.equals(key, new byte[32]))
      throw new IllegalArgumentException("approval code encryption key must be nonzero 32-byte AES-256 key");
    this.key = key.clone();
  }

  public Sealed seal(UUID requestId, String code) {
    if (requestId == null || code == null || !code.matches("[A-Za-z0-9_-]{43}")) throw new IllegalArgumentException("invalid approval code");
    byte[] nonce = new byte[12]; random.nextBytes(nonce);
    return new Sealed(nonce, crypt(Cipher.ENCRYPT_MODE, requestId, nonce, code.getBytes(StandardCharsets.US_ASCII)));
  }

  public String open(UUID requestId, byte[] nonce, byte[] ciphertext) {
    if (requestId == null || nonce == null || nonce.length != 12 || ciphertext == null || ciphertext.length < 32)
      throw new SdkIdentityDeniedException();
    byte[] plaintext = crypt(Cipher.DECRYPT_MODE, requestId, nonce, ciphertext);
    String code = new String(plaintext, StandardCharsets.US_ASCII);
    if (!code.matches("[A-Za-z0-9_-]{43}")) throw new SdkIdentityDeniedException();
    return code;
  }

  static AuthGameBindingApprovalCodeVault configured(org.springframework.core.env.Environment environment) {
    String encoded = environment.getProperty("voice.auth.game-binding.approval-code.key-base64",
        environment.getProperty("AUTH_GAME_BINDING_APPROVAL_CODE_KEY_B64", "")).trim();
    if (encoded.isEmpty()) return null;
    try {
      byte[] decoded = Base64.getDecoder().decode(encoded);
      if (!Base64.getEncoder().encodeToString(decoded).equals(encoded)) throw new IllegalArgumentException();
      return new AuthGameBindingApprovalCodeVault(decoded);
    } catch (RuntimeException invalid) {
      throw new IllegalStateException("Auth game-binding approval code encryption key must be canonical 32-byte Base64", invalid);
    }
  }

  private byte[] crypt(int mode, UUID requestId, byte[] nonce, byte[] input) {
    try {
      Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
      cipher.init(mode, new SecretKeySpec(key, "AES"), new GCMParameterSpec(128, nonce));
      cipher.updateAAD(("voice-game-binding-approval-code-v1\n" + requestId).getBytes(StandardCharsets.US_ASCII));
      return cipher.doFinal(input);
    } catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }
}
