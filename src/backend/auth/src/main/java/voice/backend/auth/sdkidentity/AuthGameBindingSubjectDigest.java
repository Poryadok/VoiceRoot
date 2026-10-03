package voice.backend.auth.sdkidentity;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.util.Base64;
import java.util.HexFormat;
import java.util.UUID;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/** Dedicated, versioned namespace HMAC for provider subjects; never uses token/signing keys. */
public final class AuthGameBindingSubjectDigest {
  private final String keyId;
  private final byte[] key;

  public AuthGameBindingSubjectDigest(String keyId, byte[] key) {
    if (keyId == null || !keyId.matches("[A-Za-z0-9_-]{1,32}") || key == null || key.length != 32
        || java.util.Arrays.equals(key, new byte[32])) throw new IllegalArgumentException("invalid game-binding subject digest key");
    this.keyId = keyId;
    this.key = key.clone();
  }

  public String digest(String provider, String issuer, UUID applicationId, UUID environmentId, String subject) {
    if (provider == null || !provider.matches("[a-z][a-z0-9_-]{0,31}") || issuer == null || issuer.isBlank()
        || applicationId == null || environmentId == null || subject == null || subject.isBlank()) {
      throw new IllegalArgumentException("invalid game-binding subject namespace");
    }
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(key, "HmacSHA256"));
      mac.update("voice-game-binding-provider-subject-v1\n".getBytes(StandardCharsets.US_ASCII));
      update(mac, provider);
      update(mac, issuer);
      update(mac, applicationId.toString());
      update(mac, environmentId.toString());
      update(mac, subject);
      return "hmac-sha256-v1:" + keyId + ":" + HexFormat.of().formatHex(mac.doFinal());
    } catch (GeneralSecurityException impossible) {
      throw new IllegalStateException("HMAC-SHA-256 unavailable", impossible);
    }
  }

  static AuthGameBindingSubjectDigest configured(org.springframework.core.env.Environment environment) {
    String keyId = setting(environment, "voice.auth.game-binding.subject-digest.kid",
        "AUTH_GAME_BINDING_SUBJECT_DIGEST_KID");
    String encoded = setting(environment, "voice.auth.game-binding.subject-digest.key-base64",
        "AUTH_GAME_BINDING_SUBJECT_DIGEST_KEY_B64");
    if (keyId.isBlank() && encoded.isBlank()) return null;
    try {
      byte[] decoded = Base64.getDecoder().decode(encoded);
      if (!Base64.getEncoder().encodeToString(decoded).equals(encoded)) throw new IllegalArgumentException();
      return new AuthGameBindingSubjectDigest(keyId, decoded);
    } catch (RuntimeException invalid) {
      throw new IllegalStateException("Auth game-binding subject digest requires a key ID and canonical 32-byte key", invalid);
    }
  }

  private static String setting(org.springframework.core.env.Environment environment, String property, String env) {
    String value = environment.getProperty(property);
    if (value == null) value = environment.getProperty(env);
    return value == null ? "" : value.trim();
  }

  private static void update(Mac mac, String value) {
    byte[] bytes = value.getBytes(StandardCharsets.UTF_8);
    mac.update(ByteBuffer.allocate(Integer.BYTES).putInt(bytes.length).array());
    mac.update(bytes);
  }
}
