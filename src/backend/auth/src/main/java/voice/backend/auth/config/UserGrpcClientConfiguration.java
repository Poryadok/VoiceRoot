package voice.backend.auth.config;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import org.springframework.boot.autoconfigure.condition.ConditionalOnExpression;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import voice.backend.auth.userdb.GrpcUserVerificationSync;
import voice.backend.auth.userdb.UserVerificationSync;

@Configuration
@ConditionalOnExpression("!'${auth.user-grpc.addr:}'.blank")
public class UserGrpcClientConfiguration {

  @Bean(destroyMethod = "close")
  UserVerificationSync grpcUserVerificationSync(AuthProperties props) {
    String addr = props.getUserGrpc().getAddr().trim();
    ManagedChannel channel =
        ManagedChannelBuilder.forTarget(normalizeTarget(addr)).usePlaintext().build();
    return new GrpcUserVerificationSync(channel);
  }

  private static String normalizeTarget(String addr) {
    if (addr.startsWith(":")) {
      return "localhost" + addr;
    }
    return addr;
  }
}
