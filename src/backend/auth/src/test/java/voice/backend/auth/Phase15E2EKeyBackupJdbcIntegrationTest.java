package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.GetE2EKeyBackupRequest;
import app.voice.auth.v1.PutE2EKeyBackupRequest;
import app.voice.auth.v1.RegisterRequest;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.service.AuthService;

/**
 * Phase 15 E2E-B red tests: encrypted key backup persisted via JDBC + Flyway V4
 * {@code e2e_key_backups} (docs/features/encryption.md). Not in-memory.
 */
@SpringBootTest
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
class Phase15E2EKeyBackupJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  @Container
  static final GenericContainer<?> redis =
      new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);

  @Container
  static final PostgreSQLContainer<?> userPostgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("user_db")
          .withUsername("voice")
          .withPassword("voice")
          .withInitScript("integration-user-schema.sql");

  @DynamicPropertySource
  static void registerProps(DynamicPropertyRegistry registry) {
    registry.add("voice.auth.jdbc.url", postgres::getJdbcUrl);
    registry.add("spring.datasource.username", postgres::getUsername);
    registry.add("spring.datasource.password", postgres::getPassword);
    registry.add("spring.flyway.user", postgres::getUsername);
    registry.add("spring.flyway.password", postgres::getPassword);
    registry.add("auth.user-db.jdbc-url", userPostgres::getJdbcUrl);
    registry.add("auth.user-db.username", userPostgres::getUsername);
    registry.add("auth.user-db.password", userPostgres::getPassword);
    registry.add("spring.data.redis.host", redis::getHost);
    registry.add("spring.data.redis.port", () -> String.valueOf(redis.getMappedPort(6379)));
  }

  @Autowired AuthGrpcService grpcService;
  @Autowired NamedParameterJdbcTemplate jdbc;

  @Test
  void flywayCreatesE2EKeyBackupsTable() {
    Integer count =
        jdbc.queryForObject(
            """
            SELECT COUNT(*)::int
            FROM information_schema.tables
            WHERE table_schema = 'public' AND table_name = 'e2e_key_backups'
            """,
            Map.of(),
            Integer.class);
    assertThat(count).isEqualTo(1);
  }

  @Test
  void putE2EKeyBackup_rejectsOversizedBlob() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server =
        InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(grpcService)
            .build()
            .start();
    ManagedChannel channel =
        InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered =
          client
              .register(
                  RegisterRequest.newBuilder()
                      .setEmail("e2e-backup-jdbc-oversize@example.com")
                      .setPassword("Correct horse battery staple")
                      .build())
              .getSession();
      assertThat(registered.getAccountId()).isNotBlank();

      String oversized = "x".repeat(AuthService.E2E_KEY_BACKUP_MAX_BLOB_BYTES + 1);
      assertThatThrownBy(
              () ->
                  client.putE2EKeyBackup(
                      PutE2EKeyBackupRequest.newBuilder()
                          .setEncryptedBlob(oversized)
                          .build()))
          .isInstanceOf(StatusRuntimeException.class);
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  @Test
  void putE2EKeyBackupAndGetE2EKeyBackupRoundtripViaJdbc() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server =
        InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(grpcService)
            .build()
            .start();
    ManagedChannel channel =
        InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered =
          client
              .register(
                  RegisterRequest.newBuilder()
                      .setEmail("e2e-backup-jdbc@example.com")
                      .setPassword("Correct horse battery staple")
                      .build())
              .getSession();
      UUID accountId = UUID.fromString(registered.getAccountId());
      assertThat(accountId).isNotNull();

      String encryptedBlob = "phase15-jdbc-encrypted-key-backup-blob";
      client.putE2EKeyBackup(
          PutE2EKeyBackupRequest.newBuilder()
              .setEncryptedBlob(encryptedBlob)
              .setPasswordHint("hint-only")
              .build());

      var restored =
          client.getE2EKeyBackup(GetE2EKeyBackupRequest.getDefaultInstance());
      assertThat(restored.getEncryptedBlob()).isEqualTo(encryptedBlob);

      String storedBlob =
          jdbc.queryForObject(
              """
              SELECT encrypted_blob
              FROM e2e_key_backups
              WHERE account_id = :accountId
              """,
              Map.of("accountId", accountId),
              String.class);
      assertThat(storedBlob).isEqualTo(encryptedBlob);
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }
}
