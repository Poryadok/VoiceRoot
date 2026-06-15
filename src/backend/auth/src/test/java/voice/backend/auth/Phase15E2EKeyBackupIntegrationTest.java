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
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.service.AuthService;

/**
 * Phase 15 red tests: encrypted key backup roundtrip (docs/features/encryption.md).
 * Server stores opaque blob only; decryption key stays client-side.
 */
@SpringBootTest
@ActiveProfiles("test")
class Phase15E2EKeyBackupIntegrationTest {
  @Autowired AuthGrpcService grpcService;

  @Test
  void putE2EKeyBackupAndGetE2EKeyBackupRoundtrip() throws Exception {
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
                      .setEmail("e2e-backup@example.com")
                      .setPassword("Correct horse battery staple")
                      .build())
              .getSession();
      assertThat(registered.getAccountId()).isNotBlank();

      String encryptedBlob = "phase15-encrypted-key-backup-blob-base64";
      client.putE2EKeyBackup(
          PutE2EKeyBackupRequest.newBuilder()
              .setEncryptedBlob(encryptedBlob)
              .setPasswordHint("hint-only")
              .build());

      var restored =
          client.getE2EKeyBackup(GetE2EKeyBackupRequest.getDefaultInstance());
      assertThat(restored.getEncryptedBlob()).isEqualTo(encryptedBlob);
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
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
                      .setEmail("e2e-backup-oversize@example.com")
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
}
