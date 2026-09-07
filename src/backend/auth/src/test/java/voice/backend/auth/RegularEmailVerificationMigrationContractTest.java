package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

class RegularEmailVerificationMigrationContractTest {
  @Test
  void bothSupportedAuthMigrationCatalogsContainFreshEmailPendingRevision() throws Exception {
    Path flyway =
        GuestConversionDurabilityMigrationContractTest.authProjectRoot()
            .resolve("src/main/resources/db/migration/V11__regular_email_verification_pending.sql");
    assertThat(Files.exists(flyway)).isTrue();
    assertThat(Files.readString(flyway)).contains("DEFAULT false");
    assertThat(
            Files.exists(
                GuestConversionDurabilityMigrationContractTest.repositoryRoot()
                    .resolve("src/backend/migrations/auth_db/000012_regular_email_verification_pending.up.sql")))
        .isTrue();
    assertThat(
            Files.exists(
                GuestConversionDurabilityMigrationContractTest.repositoryRoot()
                    .resolve("src/backend/migrations/auth_db/000012_regular_email_verification_pending.down.sql")))
        .isTrue();
  }
}
