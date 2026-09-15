package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.GetE2EKeyBackupRequest;
import app.voice.auth.v1.PutE2EKeyBackupRequest;
import app.voice.auth.v1.RegisterRequest;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.ServerInterceptors;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.MetadataUtils;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.grpc.AuthorizationServerInterceptor;
import voice.backend.auth.service.AuthService;

/**
 * encryption (docs/features/encryption.md) red tests: encrypted key backup roundtrip (docs/features/encryption.md).
 * Server stores opaque blob only; decryption key stays client-side.
 */
@SpringBootTest
@ActiveProfiles("test")
class E2EKeyBackupIntegrationTest {
  @Autowired AuthGrpcService grpcService;

  @Test
  void putE2EKeyBackupAndGetE2EKeyBackupRoundtrip() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server =
        InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(ServerInterceptors.intercept(grpcService, new AuthorizationServerInterceptor()))
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
      var authenticated = withBearer(client, registered.getAccessToken());

      String encryptedBlob = "phase15-encrypted-key-backup-blob-base64";
      authenticated.putE2EKeyBackup(
          PutE2EKeyBackupRequest.newBuilder()
              .setEncryptedBlob(encryptedBlob)
              .setPasswordHint("hint-only")
              .build());

      var restored =
          authenticated.getE2EKeyBackup(GetE2EKeyBackupRequest.getDefaultInstance());
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
            .addService(ServerInterceptors.intercept(grpcService, new AuthorizationServerInterceptor()))
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
      var authenticated = withBearer(client, registered.getAccessToken());

      String oversized = "x".repeat(AuthService.E2E_KEY_BACKUP_MAX_BLOB_BYTES + 1);
      assertThatThrownBy(
              () ->
                  authenticated.putE2EKeyBackup(
                      PutE2EKeyBackupRequest.newBuilder()
                          .setEncryptedBlob(oversized)
                          .build()))
          .isInstanceOf(StatusRuntimeException.class);
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  private static AuthServiceGrpc.AuthServiceBlockingStub withBearer(
      AuthServiceGrpc.AuthServiceBlockingStub client, String accessToken) {
    Metadata headers = new Metadata();
    headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER), "Bearer " + accessToken);
    return client.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers));
  }
}
