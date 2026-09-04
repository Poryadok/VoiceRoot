package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

/**
 * T-049b RED catalog contract. Schema behavior is exercised against PostgreSQL in
 * {@link GuestConversionDurabilityJdbcIntegrationTest}.
 */
class GuestConversionDurabilityMigrationContractTest {
  static final String FLYWAY_MIGRATION = "V8__guest_conversion_operations.sql";
  static final String GOLANG_UP_MIGRATION = "000009_guest_conversion_operations.up.sql";
  static final String GOLANG_DOWN_MIGRATION = "000009_guest_conversion_operations.down.sql";

  @Test
  void bothSupportedAuthMigrationCatalogsContainTheDurableOperationRevision() {
    assertThat(flywayMigration())
        .as("Flyway must retain Auth-owned guest conversion work across restart")
        .exists();
    assertThat(golangMigration(GOLANG_UP_MIGRATION))
        .as("golang-migrate must apply the same Auth-owned durable operation")
        .exists();
    assertThat(golangMigration(GOLANG_DOWN_MIGRATION))
        .as("the operational migration path needs an explicit pending-safe rollback")
        .exists();
  }

  static Path flywayMigration() {
    return authProjectRoot().resolve("src/main/resources/db/migration").resolve(FLYWAY_MIGRATION);
  }

  static Path golangMigration(String name) {
    return repositoryRoot().resolve("src/backend/migrations/auth_db").resolve(name);
  }

  static Path authProjectRoot() {
    return repositoryRoot().resolve("src/backend/auth");
  }

  static Path repositoryRoot() {
    for (Path candidate = Path.of("").toAbsolutePath();
        candidate != null;
        candidate = candidate.getParent()) {
      if (Files.isDirectory(candidate.resolve("src/backend/auth"))
          && Files.isDirectory(candidate.resolve("src/backend/migrations/auth_db"))) {
        return candidate;
      }
    }
    throw new IllegalStateException(
        "Voice repository root was not found from the Maven working directory");
  }
}
