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

@SpringBootTest
@ActiveProfiles("test")
class ResolvePhoneHashesIntegrationTest {
  @Autowired AuthGrpcService grpcService;

  @Test
  void resolvePhoneHashesReturnsPrimaryProfileForRegisteredPhoneHash() throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server =
        InProcessServerBuilder.forName(serverName).directExecutor().addService(grpcService).build().start();
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

      var resp =
          client.resolvePhoneHashes(
              ResolvePhoneHashesRequest.newBuilder().addPhoneHashes(phoneHash).build());

      assertThat(resp.getMatchesList()).hasSize(1);
      assertThat(resp.getMatches(0).getPhoneHash()).isEqualTo(phoneHash);
      assertThat(resp.getMatches(0).getProfileId()).isEqualTo(registered.getProfileId());

      var empty =
          client.resolvePhoneHashes(
              ResolvePhoneHashesRequest.newBuilder().addPhoneHashes("sha256-unknown").build());
      assertThat(empty.getMatchesList()).isEmpty();
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }
}
