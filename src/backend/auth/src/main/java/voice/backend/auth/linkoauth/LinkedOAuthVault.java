package voice.backend.auth.linkoauth;

import java.nio.*;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.util.UUID;
import javax.crypto.Cipher;
import javax.crypto.spec.*;

/** AES-GCM envelope; payloads cannot be transplanted between attempts or purposes. */
final class LinkedOAuthVault {
  private final byte[] key;
  private final SecureRandom random = new SecureRandom();
  LinkedOAuthVault(byte[] key) { this.key = key.clone(); }
  byte[] seal(UUID id, String purpose, byte[] plaintext) {
    byte[] nonce = new byte[12]; random.nextBytes(nonce);
    byte[] encrypted = crypt(Cipher.ENCRYPT_MODE, id, purpose, nonce, plaintext);
    return ByteBuffer.allocate(12 + encrypted.length).put(nonce).put(encrypted).array();
  }
  byte[] open(UUID id, String purpose, byte[] envelope) {
    if (envelope == null || envelope.length < 28) throw new LinkedOAuthException("invalid_link_attempt");
    ByteBuffer b = ByteBuffer.wrap(envelope); byte[] nonce = new byte[12]; b.get(nonce);
    byte[] encrypted = new byte[b.remaining()]; b.get(encrypted);
    return crypt(Cipher.DECRYPT_MODE, id, purpose, nonce, encrypted);
  }
  private byte[] crypt(int mode, UUID id, String purpose, byte[] nonce, byte[] input) {
    try {
      Cipher c = Cipher.getInstance("AES/GCM/NoPadding");
      c.init(mode, new SecretKeySpec(key, "AES"), new GCMParameterSpec(128, nonce));
      c.updateAAD(("voice-linked-oauth-v1\n" + purpose + "\n" + id).getBytes(StandardCharsets.US_ASCII));
      return c.doFinal(input);
    } catch (Exception e) { throw new LinkedOAuthException("invalid_link_attempt"); }
  }
}
