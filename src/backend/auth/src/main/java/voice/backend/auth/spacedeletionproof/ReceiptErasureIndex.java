package voice.backend.auth.spacedeletionproof;

import java.nio.charset.StandardCharsets;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/** Purpose-separated post-erasure lookup index. */
public final class ReceiptErasureIndex {
  private static final byte[] PURPOSE =
      "voice-auth-space-delete-receipt-v1".getBytes(StandardCharsets.US_ASCII);

  private ReceiptErasureIndex() {}

  public static byte[] hmacSha256(byte[] key, byte[] deterministicBinding) {
    if (key == null || key.length < 32 || deterministicBinding == null) {
      throw new IllegalArgumentException("invalid receipt erasure index input");
    }
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(key, "HmacSHA256"));
      mac.update(PURPOSE);
      mac.update((byte) 0);
      return mac.doFinal(deterministicBinding);
    } catch (java.security.GeneralSecurityException impossible) {
      throw new IllegalStateException("HmacSHA256 unavailable", impossible);
    }
  }
}
