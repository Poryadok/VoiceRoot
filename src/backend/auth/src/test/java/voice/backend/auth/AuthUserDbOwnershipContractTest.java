package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.function.Executable;

/**
 * RED ownership guard for A1: Auth may own account credentials, but never User profiles or user_db.
 *
 * <p>The list is deliberately explicit: it protects the known direct JDBC implementation and the
 * configuration/deploy credentials that keep it live. Do not turn this into a broad text grep.
 */
class AuthUserDbOwnershipContractTest {
  @Test
  void authHasNoUserDbJdbcClassesOrCredentialsAndConsumesUserRpcPortsInstead() throws Exception {
    Path root = repositoryRoot();
    List<Executable> requirements = List.of(
        () -> assertThat(root.resolve("src/backend/auth/src/main/java/voice/backend/auth/config/UserDbJdbcConfiguration.java"))
            .doesNotExist(),
        () -> assertThat(root.resolve("src/backend/auth/src/main/java/voice/backend/auth/config/JdbcPersistenceWithUserDbUrlCondition.java"))
            .doesNotExist(),
        () -> assertThat(root.resolve("src/backend/auth/src/main/java/voice/backend/auth/userdb/JdbcPrimaryProfileProvisioner.java"))
            .doesNotExist(),
        () -> assertThat(root.resolve("src/backend/auth/src/main/java/voice/backend/auth/userdb/JdbcProfileSwitchValidator.java"))
            .doesNotExist(),
        () -> assertThat(root.resolve("src/backend/auth/src/main/java/voice/backend/auth/userdb/JdbcPhoneHashResolver.java"))
            .doesNotExist(),
        () -> assertThat(root.resolve("src/backend/auth/src/main/java/voice/backend/auth/userdb/JdbcUserVerificationSync.java"))
            .doesNotExist(),
        () -> assertThat(read(root, "src/backend/auth/src/main/java/voice/backend/auth/config/AuthProperties.java"))
            .doesNotContain("class UserDb", "getUserDb"),
        () -> assertThat(read(root, "src/backend/auth/src/main/java/voice/backend/auth/config/PrimaryProfileBeansConfiguration.java"))
            .doesNotContain("userJdbc", "JdbcPrimaryProfileProvisioner", "JdbcProfileSwitchValidator", "JdbcPhoneHashResolver", "JdbcUserVerificationSync"),
        () -> assertThat(read(root, "src/backend/auth/src/main/resources/application.yml"))
            .doesNotContain("auth.user-db", "user-db:", "AUTH_USER_DB_"),
        () -> assertThat(read(root, "deploy/staging/services.yaml")).doesNotContain("AUTH_USER_DB_"),
        () -> assertThat(read(root, "deploy/prod/services.yaml")).doesNotContain("AUTH_USER_DB_"),
        () -> assertThat(read(root, "docker-compose.yml")).doesNotContain("AUTH_USER_DB_"),
        () -> assertThat(read(root, "scripts/ci/auth-container-smoke.sh")).doesNotContain("AUTH_USER_DB_"),
        () -> assertThat(read(root, "src/backend/auth/src/main/java/voice/backend/auth/config/UserGrpcClientConfiguration.java"))
            .contains(
                "GrpcPrimaryProfileProvisioner",
                "GrpcPhoneHashResolver",
                "GrpcProfileSwitchValidator",
                "GrpcUserVerificationSync"));

    org.junit.jupiter.api.Assertions.assertAll(requirements);
  }

  @Test
  void authContractNamesTheOnlyAllowedUserRpcOperationsForSessionPhoneAndPromotion() throws Exception {
    String proto = read(repositoryRoot(), "protos/voice/user/v1/user.proto");

    assertThat(proto).contains(
        "rpc EnsurePrimaryProfile", "rpc ResolvePrimaryProfileIDs", "rpc MarkAccountRegular",
        "rpc SetVerification", "rpc ClearVerification");
    assertThat(proto).doesNotContain("ValidateProfileForSession");
  }

  private static String read(Path root, String relative) throws IOException {
    return Files.readString(root.resolve(relative));
  }

  private static Path repositoryRoot() {
    Path current = Path.of("").toAbsolutePath();
    while (current != null && !Files.exists(current.resolve("protos/voice/user/v1/user.proto"))) {
      current = current.getParent();
    }
    if (current == null) throw new IllegalStateException("Voice repository root was not found");
    return current;
  }
}
