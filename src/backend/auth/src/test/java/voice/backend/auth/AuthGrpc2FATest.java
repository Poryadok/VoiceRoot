package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.Enable2FARequest;
import app.voice.auth.v1.LoginRequest;
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
class AuthGrpc2FATest {
  private static final Metadata.Key<String> AUTHORIZATION =
      Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);

  @Autowired AuthGrpcService grpcService;

  @Test
  void enable2FAReturnsTotpUriAndBackupCodesOverGrpc() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor()
        .addService(ServerInterceptors.intercept(grpcService, new AuthorizationServerInterceptor())).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered = client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-2fa@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build()).getSession();

      var enable = withBearer(client, registered.getAccessToken()).enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build());
      assertThat(enable.getTotpUri()).contains("otpauth://");
      assertThat(enable.getBackupCodesList()).isNotEmpty();
      assertThat(enable.getBackupCodesList()).hasSizeGreaterThanOrEqualTo(8);
      assertThat(registered.getAccountId()).isNotBlank();
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  @Test
  void verify2FAActivatesTotpOverGrpc() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor()
        .addService(ServerInterceptors.intercept(grpcService, new AuthorizationServerInterceptor())).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered = client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-2fa-verify@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build());

      var authenticated = withBearer(client, registered.getSession().getAccessToken());
      authenticated.enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build());

      var verified = authenticated.verify2FA(Verify2FARequest.newBuilder().setTotpCode("000000").build());
      assertThat(verified.getSession().getAccessToken()).isNotBlank();
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  @Test
  void loginRequiresTotpWhen2FAEnabledOverGrpc() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor()
        .addService(ServerInterceptors.intercept(grpcService, new AuthorizationServerInterceptor())).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered = client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-2fa-login@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build());
      var authenticated = withBearer(client, registered.getSession().getAccessToken());
      authenticated.enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build());
      authenticated.verify2FA(Verify2FARequest.newBuilder().setTotpCode("000000").build());

      assertThatThrownBy(() -> client.login(LoginRequest.newBuilder()
          .setEmail("grpc-2fa-login@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .setDeviceInfoJson("{}")
          .build()))
          .isInstanceOf(StatusRuntimeException.class)
          .satisfies(ex -> assertThat(((StatusRuntimeException) ex).getStatus().getCode())
              .isEqualTo(Status.Code.UNAUTHENTICATED));
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  @Test
  void loginWithBackupCodeSucceedsOverGrpc() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor()
        .addService(ServerInterceptors.intercept(grpcService, new AuthorizationServerInterceptor())).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered = client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-2fa-backup@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build());
      var authenticated = withBearer(client, registered.getSession().getAccessToken());
      var enable = authenticated.enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build());
      authenticated.verify2FA(Verify2FARequest.newBuilder().setTotpCode("000000").build());

      String backup = enable.getBackupCodesList().getFirst();
      var login = client.login(LoginRequest.newBuilder()
          .setEmail("grpc-2fa-backup@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .setTotpCode(backup)
          .setDeviceInfoJson("{}")
          .build());
      assertThat(login.getSession().getAccessToken()).isNotBlank();
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
}
