package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;
import voice.backend.auth.service.TotpService;

class TotpEncryptionKeyConfigurationTest {
  @Test
  void jdbcWithoutTotpKeyFailsWhenTestBypassDisabled() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.JDBC);
    properties.getTotp().setTestBypass(false);
    properties.getTotp().setEncryptionKey("");

    assertThatThrownBy(() -> createTotpService(properties))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("AUTH_TOTP_ENCRYPTION_KEY");
  }

  private static TotpService createTotpService(AuthProperties properties) {
    if (properties.getPersistence() == AuthProperties.PersistenceMode.JDBC
        && !properties.getTotp().isTestBypass()) {
      String key = properties.getTotp().getEncryptionKey();
      if (key == null || key.isBlank()) {
        throw new IllegalStateException(
            "TOTP encryption key is required when auth.persistence=jdbc and auth.totp.test-bypass=false:"
                + " set AUTH_TOTP_ENCRYPTION_KEY");
      }
    }
    return new TotpService(properties);
  }
}
