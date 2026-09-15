package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.RegisterRequest;
import app.voice.auth.v1.ResolvePhoneHashesRequest;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.principal.AuthInternalTestPrincipal;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;

@SpringBootTest
@ActiveProfiles("test")
class ResolvePhoneHashesIntegrationTest {
  @Autowired AuthGrpcService grpcService;

  @Test
  void resolvePhoneHashesReturnsPrimaryProfileForRegisteredPhoneHash() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server =
        InProcessServerBuilder.forName(serverName).directExecutor()
            .intercept(AuthInternalTestPrincipal.interceptor()).addService(grpcService).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      String phoneHash = "sha256-compose-phone-hash-test";
      var registered =
          client.register(
                  RegisterRequest.newBuilder()
                      .setEmail("phone-hash@example.com")
                      .setPhone(phoneHash)
                      .setPassword("Correct horse battery staple")
                      .build())
              .getSession();

      var request = ResolvePhoneHashesRequest.newBuilder().addPhoneHashes(phoneHash).build();
      var resp = AuthInternalTestPrincipal.client(client, "social", AuthPrincipalServerInterceptor.PHONE_RPC, request)
          .resolvePhoneHashes(request);

      assertThat(resp.getMatchesList()).hasSize(1);
      assertThat(resp.getMatches(0).getPhoneHash()).isEqualTo(phoneHash);
      assertThat(resp.getMatches(0).getProfileId()).isEqualTo(registered.getProfileId());

      var missing = ResolvePhoneHashesRequest.newBuilder().addPhoneHashes("sha256-unknown").build();
      var empty = AuthInternalTestPrincipal.client(client, "social", AuthPrincipalServerInterceptor.PHONE_RPC, missing)
          .resolvePhoneHashes(missing);
      assertThat(empty.getMatchesList()).isEmpty();
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }
}
