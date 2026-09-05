package voice.backend.auth.service;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.util.Base64;
import java.util.Objects;
import java.util.UUID;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import voice.backend.auth.config.AuthProperties;

/**
 * Derives one opaque restore token per durable deletion operation without storing plaintext.
 *
 * <p>The dedicated secret is deliberately not interchangeable with JWT, TOTP, or analytics keys.
 */
public final class AccountDeletionRestoreTokenCodec {
  private static final String HMAC_ALGORITHM = "HmacSHA256";
  private static final byte[] DOMAIN =
      "voice.auth.account-deletion.restore-token.v1\u0000".getBytes(StandardCharsets.US_ASCII);
  private static final int MIN_SECRET_BYTES = 32;
  private final byte[] secret;

  public AccountDeletionRestoreTokenCodec(AuthProperties properties) {
    Objects.requireNonNull(properties, "properties");
    String configured = properties.getAccountDeletion().getTokenSecret();
    if (configured == null || configured.isBlank()) {
      throw new IllegalStateException(
          "account deletion token secret is required: set ACCOUNT_DELETE_TOKEN_SECRET");
    }
    byte[] configuredBytes = configured.getBytes(StandardCharsets.UTF_8);
    if (configuredBytes.length < MIN_SECRET_BYTES) {
      throw new IllegalStateException(
          "ACCOUNT_DELETE_TOKEN_SECRET must contain at least 32 bytes");
    }
    secret = configuredBytes.clone();
  }

  public String derive(UUID accountId, UUID operationId) {
    Objects.requireNonNull(accountId, "accountId");
    Objects.requireNonNull(operationId, "operationId");
    try {
      Mac mac = Mac.getInstance(HMAC_ALGORITHM);
      mac.init(new SecretKeySpec(secret, HMAC_ALGORITHM));
      mac.update(DOMAIN);
      ByteBuffer ids = ByteBuffer.allocate(32);
      ids.putLong(accountId.getMostSignificantBits());
      ids.putLong(accountId.getLeastSignificantBits());
      ids.putLong(operationId.getMostSignificantBits());
      ids.putLong(operationId.getLeastSignificantBits());
      return Base64.getUrlEncoder().withoutPadding().encodeToString(mac.doFinal(ids.array()));
    } catch (GeneralSecurityException failure) {
      throw new IllegalStateException("derive account deletion restore token", failure);
    }
  }
}
