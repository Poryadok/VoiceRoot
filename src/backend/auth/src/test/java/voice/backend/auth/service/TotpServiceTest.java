package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;
import voice.backend.auth.config.AuthProperties;

class TotpServiceTest {
  @Test
  void jdbcWithoutEncryptionKeyFailsClosed() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.JDBC);
    properties.getTotp().setTestBypass(false);
    properties.getTotp().setEncryptionKey("");

    assertThatThrownBy(() -> new TotpService(properties))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("AUTH_TOTP_ENCRYPTION_KEY");
  }

  @Test
  void memoryAllowsDevKeyWhenEncryptionKeyUnset() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.MEMORY);
    properties.getTotp().setEncryptionKey("");

    TotpService totp = new TotpService(properties);
    byte[] encrypted = totp.encryptSecret("JBSWY3DPEHPK3PXP");
    assertThat(totp.decryptSecret(encrypted)).isEqualTo("JBSWY3DPEHPK3PXP");
  }

  @Test
  void jdbcWithConfiguredKeyEncryptsRoundTrip() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.JDBC);
    properties.getTotp().setTestBypass(false);
    properties.getTotp().setEncryptionKey("staging-totp-encryption-key-32b!!");

    TotpService totp = new TotpService(properties);
    byte[] encrypted = totp.encryptSecret("JBSWY3DPEHPK3PXP");
    assertThat(totp.decryptSecret(encrypted)).isEqualTo("JBSWY3DPEHPK3PXP");
  }

  @Test
  void jdbcTestBypassAllowsDevKeyWhenEncryptionKeyUnset() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.JDBC);
    properties.getTotp().setTestBypass(true);
    properties.getTotp().setEncryptionKey("");

    TotpService totp = new TotpService(properties);
    byte[] encrypted = totp.encryptSecret("JBSWY3DPEHPK3PXP");
    assertThat(totp.decryptSecret(encrypted)).isEqualTo("JBSWY3DPEHPK3PXP");
  }
}
