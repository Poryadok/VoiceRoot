package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;
import voice.backend.auth.service.AccountDeletionRestoreTokenCodec;

class AccountDeletionTokenSecretConfigurationTest {
  @Test
  void jdbcRuntimeRejectsMissingOrShortDedicatedDeletionTokenSecret() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.JDBC);

    assertThatThrownBy(() -> new AccountDeletionRestoreTokenCodec(properties))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("ACCOUNT_DELETE_TOKEN_SECRET");

    properties.getAccountDeletion().setTokenSecret("too-short");
    assertThatThrownBy(() -> new AccountDeletionRestoreTokenCodec(properties))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("at least 32 bytes");
  }

  @Test
  void memoryRuntimeUsesOnlyAnExplicitSafeTestFixture() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.MEMORY);
    properties.getAccountDeletion().setTokenSecret("test-only-account-delete-token-secret");

    new AccountDeletionRestoreTokenCodec(properties);
  }
}
