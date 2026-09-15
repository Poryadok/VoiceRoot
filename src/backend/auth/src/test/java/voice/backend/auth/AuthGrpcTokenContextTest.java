package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.ConvertGuestRequest;
import app.voice.auth.v1.Enable2FARequest;
import app.voice.auth.v1.GetE2EKeyBackupRequest;
import app.voice.auth.v1.PutE2EKeyBackupRequest;
import app.voice.auth.v1.RegisterRequest;
import app.voice.auth.v1.Verify2FARequest;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.ServerInterceptors;
import io.grpc.Status;
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

@SpringBootTest
@ActiveProfiles("test")
class AuthGrpcTokenContextTest {
  private static final Metadata.Key<String> AUTHORIZATION =
      Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);

  @Autowired AuthGrpcService grpcService;

  @Test
  void protectedCallsRejectMissingMetadataInsteadOfReusingAnotherRpcToken() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName)
        .directExecutor()
        .addService(ServerInterceptors.intercept(grpcService, new AuthorizationServerInterceptor()))
        .build()
        .start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered = client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-token-context@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build()).getSession();

      assertUnauthenticated(() -> client.enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build()));
      var authenticated = withBearer(client, registered.getAccessToken());
      var enrollment = authenticated.enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build());
      assertThat(enrollment.getBackupCodesList()).isNotEmpty();
      authenticated.putE2EKeyBackup(PutE2EKeyBackupRequest.newBuilder()
          .setEncryptedBlob("opaque-backup")
          .build());
      assertUnauthenticated(() -> client.verify2FA(Verify2FARequest.newBuilder()
          .setTotpCode("000000")
          .build()));
      assertUnauthenticated(() -> client.putE2EKeyBackup(PutE2EKeyBackupRequest.newBuilder()
          .setEncryptedBlob("opaque-backup")
          .build()));
      assertUnauthenticated(() -> client.getE2EKeyBackup(GetE2EKeyBackupRequest.getDefaultInstance()));

      var guest = client.register(RegisterRequest.newBuilder()
          .setGuest(true)
          .setPassword("Correct horse battery staple")
          .build()).getSession();
      assertUnauthenticated(() -> client.convertGuest(ConvertGuestRequest.newBuilder()
          .setEmail("converted-grpc-token-context@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build()));
      assertThat(withBearer(client, guest.getAccessToken()).convertGuest(ConvertGuestRequest.newBuilder()
          .setEmail("converted-grpc-token-context@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build()).getSession().getAccessToken()).isNotBlank();
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  private static AuthServiceGrpc.AuthServiceBlockingStub withBearer(
      AuthServiceGrpc.AuthServiceBlockingStub client, String accessToken) {
    Metadata headers = new Metadata();
    headers.put(AUTHORIZATION, "Bearer " + accessToken);
    return client.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers));
  }

  private static void assertUnauthenticated(Runnable call) {
    assertThatThrownBy(call::run)
        .isInstanceOf(StatusRuntimeException.class)
        .satisfies(error -> assertThat(((StatusRuntimeException) error).getStatus().getCode())
            .isEqualTo(Status.Code.UNAUTHENTICATED));
  }
}
