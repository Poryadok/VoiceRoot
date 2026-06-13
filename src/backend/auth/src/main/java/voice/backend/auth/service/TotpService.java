package voice.backend.auth.service;

import com.warrenstrange.googleauth.GoogleAuthenticator;
import com.warrenstrange.googleauth.GoogleAuthenticatorKey;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.SecureRandom;
import java.util.Arrays;
import javax.crypto.Cipher;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import voice.backend.auth.config.AuthProperties;

public class TotpService {
  private static final int GCM_NONCE_BYTES = 12;
  private static final int GCM_TAG_BITS = 128;
  private static final byte[] DEFAULT_DEV_KEY = "voice-auth-dev-key-32-byte-secret!".getBytes(StandardCharsets.UTF_8);

  private final GoogleAuthenticator authenticator = new GoogleAuthenticator();
  private final SecureRandom secureRandom = new SecureRandom();
  private final AuthProperties properties;

  public TotpService(AuthProperties properties) {
    this.properties = properties;
  }

  public String generateSecret() {
    GoogleAuthenticatorKey key = authenticator.createCredentials();
    return key.getKey();
  }

  public String buildTotpUriFromSecret(String accountLabel, String secret) {
    String issuer = properties.getJwt().getIssuer() == null || properties.getJwt().getIssuer().isBlank()
        ? "voice-auth"
        : properties.getJwt().getIssuer();
    String safeIssuer = issuer.replace(" ", "%20");
    String safeLabel = accountLabel.replace(" ", "%20");
    return "otpauth://totp/" + safeIssuer + ":" + safeLabel + "?secret=" + secret + "&issuer=" + safeIssuer;
  }

  public byte[] encryptSecret(String secret) {
    byte[] nonce = new byte[GCM_NONCE_BYTES];
    secureRandom.nextBytes(nonce);
    byte[] plain = secret.getBytes(StandardCharsets.UTF_8);
    byte[] cipher = crypt(Cipher.ENCRYPT_MODE, nonce, plain);
    ByteBuffer out = ByteBuffer.allocate(nonce.length + cipher.length);
    out.put(nonce);
    out.put(cipher);
    return out.array();
  }

  public String decryptSecret(byte[] encrypted) {
    if (encrypted == null || encrypted.length <= GCM_NONCE_BYTES) {
      throw new AuthException("totp_not_enrolled");
    }
    byte[] nonce = Arrays.copyOfRange(encrypted, 0, GCM_NONCE_BYTES);
    byte[] cipher = Arrays.copyOfRange(encrypted, GCM_NONCE_BYTES, encrypted.length);
    byte[] plain = crypt(Cipher.DECRYPT_MODE, nonce, cipher);
    return new String(plain, StandardCharsets.UTF_8);
  }

  public boolean verifyEncrypted(byte[] encryptedSecret, String code) {
    if (properties.getTotp().isTestBypass() && "000000".equals(code)) {
      return true;
    }
    String secret = decryptSecret(encryptedSecret);
    int value;
    try {
      value = Integer.parseInt(code);
    } catch (RuntimeException ex) {
      return false;
    }
    return authenticator.authorize(secret, value);
  }

  private byte[] crypt(int mode, byte[] nonce, byte[] payload) {
    try {
      Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
      cipher.init(mode, new SecretKeySpec(resolveKey(), "AES"), new GCMParameterSpec(GCM_TAG_BITS, nonce));
      return cipher.doFinal(payload);
    } catch (GeneralSecurityException ex) {
      throw new IllegalStateException("totp crypto failure", ex);
    }
  }

  private byte[] resolveKey() {
    String key = properties.getTotp().getEncryptionKey();
    if (key == null || key.isBlank()) {
      return Arrays.copyOf(DEFAULT_DEV_KEY, 32);
    }
    byte[] bytes = key.getBytes(StandardCharsets.UTF_8);
    return Arrays.copyOf(bytes, 32);
  }
}
