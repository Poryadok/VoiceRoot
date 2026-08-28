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

    assertThatThrownBy(() -> new TotpService(properties))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("AUTH_TOTP_ENCRYPTION_KEY");
  }
}
