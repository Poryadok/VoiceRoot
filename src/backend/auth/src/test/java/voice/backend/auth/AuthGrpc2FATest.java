package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.Enable2FARequest;
import app.voice.auth.v1.LoginRequest;
import app.voice.auth.v1.RegisterRequest;
import app.voice.auth.v1.Verify2FARequest;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import voice.backend.auth.grpc.AuthGrpcService;

@SpringBootTest
@ActiveProfiles("test")
class AuthGrpc2FATest {
  @Autowired AuthGrpcService grpcService;

  @Test
  void enable2FAReturnsTotpUriAndBackupCodesOverGrpc() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor().addService(grpcService).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered = client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-2fa@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build()).getSession();

      var enable = client.enable2FA(Enable2FARequest.newBuilder()
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
    Server server = InProcessServerBuilder.forName(serverName).directExecutor().addService(grpcService).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-2fa-verify@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build());

      client.enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build());

      var verified = client.verify2FA(Verify2FARequest.newBuilder().setTotpCode("000000").build());
      assertThat(verified.getSession().getAccessToken()).isNotBlank();
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  @Test
  void loginRequiresTotpWhen2FAEnabledOverGrpc() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor().addService(grpcService).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-2fa-login@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build());
      client.enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build());
      client.verify2FA(Verify2FARequest.newBuilder().setTotpCode("000000").build());

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
    Server server = InProcessServerBuilder.forName(serverName).directExecutor().addService(grpcService).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      client.register(RegisterRequest.newBuilder()
          .setEmail("grpc-2fa-backup@voice-qa.test")
          .setPassword("Correct horse battery staple")
          .build());
      var enable = client.enable2FA(Enable2FARequest.newBuilder()
          .setPassword("Correct horse battery staple")
          .build());
      client.verify2FA(Verify2FARequest.newBuilder().setTotpCode("000000").build());

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
}
