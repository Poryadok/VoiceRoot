package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.LoginRequest;
import app.voice.auth.v1.LogoutRequest;
import app.voice.auth.v1.RefreshTokenRequest;
import app.voice.auth.v1.RegisterRequest;
import app.voice.auth.v1.ValidateTokenRequest;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import voice.backend.auth.grpc.AuthGrpcService;

@SpringBootTest
@ActiveProfiles("test")
class AuthGrpcIntegrationTest {
  @Autowired AuthGrpcService grpcService;

  @org.junit.jupiter.api.Test
  void registerLoginRefreshValidateLogoutAndJwksWorkOverGrpc() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor().addService(grpcService).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      var registered = client.register(RegisterRequest.newBuilder()
          .setEmail("grpc@example.com")
          .setPassword("Correct horse battery staple")
          .build()).getSession();
      assertThat(registered.getAccessToken()).contains(".");
      assertThat(registered.getRefreshToken()).isNotBlank().doesNotContain(".");
      assertThat(registered.getExpiresInSeconds()).isEqualTo(900);
      assertThat(registered.getProfileId())
          .matches("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}");

      var claims = client.validateToken(ValidateTokenRequest.newBuilder()
          .setAccessToken(registered.getAccessToken())
          .build()).getClaims();
      assertThat(claims.getUserId()).isEqualTo(registered.getAccountId());
      assertThat(claims.getProfileId()).isEqualTo(registered.getProfileId());

      var login = client.login(LoginRequest.newBuilder()
          .setEmail("grpc@example.com")
          .setPassword("Correct horse battery staple")
          .build()).getSession();
      assertThat(login.getRefreshToken()).isNotEqualTo(registered.getRefreshToken());

      var refreshed = client.refreshToken(RefreshTokenRequest.newBuilder()
          .setRefreshToken(registered.getRefreshToken())
          .build()).getSession();
      assertThat(refreshed.getRefreshToken()).isNotEqualTo(registered.getRefreshToken());

      client.logout(LogoutRequest.newBuilder().setRefreshToken(refreshed.getRefreshToken()).build());
      assertThatThrownBy(() -> client.validateToken(ValidateTokenRequest.newBuilder()
          .setAccessToken(refreshed.getAccessToken())
          .build()))
          .isInstanceOf(StatusRuntimeException.class)
          .hasMessageContaining("token_revoked");

      assertThat(client.getJWKS(app.voice.auth.v1.GetJWKSRequest.getDefaultInstance()).getKeysJson())
          .contains("\"kid\":\"test-key\"");
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }
}
