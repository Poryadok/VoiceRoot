package voice.backend.auth.config;

import app.voice.user.v1.UserServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Metadata;
import io.grpc.stub.MetadataUtils;
import org.springframework.boot.autoconfigure.condition.ConditionalOnExpression;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.userdb.GrpcPhoneHashResolver;
import voice.backend.auth.userdb.GrpcPrimaryProfileProvisioner;
import voice.backend.auth.userdb.GrpcProfileSwitchValidator;
import voice.backend.auth.userdb.GrpcUserVerificationSync;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.userdb.UserVerificationSync;

@Configuration
@ConditionalOnExpression("!'${auth.user-grpc.addr:}'.blank")
public class UserGrpcClientConfiguration {
  private static final Metadata.Key<String> INTERNAL_CALLER_HEADER =
      Metadata.Key.of("x-voice-internal-caller", Metadata.ASCII_STRING_MARSHALLER);

  @Bean(destroyMethod = "shutdown")
  ManagedChannel userGrpcChannel(AuthProperties props) {
    String addr = props.getUserGrpc().getAddr().trim();
    return ManagedChannelBuilder.forTarget(normalizeTarget(addr)).usePlaintext().build();
  }

  @Bean
  UserGrpcDeadlineClientInterceptor userGrpcDeadlineClientInterceptor(
      AuthProperties props, Environment environment) {
    String configuredDeadline = environment.getProperty("auth.user-grpc.deadline");
    if (configuredDeadline != null && configuredDeadline.isBlank()) {
      throw new IllegalArgumentException("auth.user-grpc.deadline must be positive");
    }
    return new UserGrpcDeadlineClientInterceptor(props.getUserGrpc().getDeadline());
  }

  @Bean
  UserServiceGrpc.UserServiceBlockingStub userGrpcStub(
      ManagedChannel userGrpcChannel,
      UserGrpcDeadlineClientInterceptor userGrpcDeadlineClientInterceptor) {
    Metadata headers = new Metadata();
    headers.put(INTERNAL_CALLER_HEADER, "auth");
    return UserServiceGrpc.newBlockingStub(userGrpcChannel)
        .withInterceptors(
            MetadataUtils.newAttachHeadersInterceptor(headers),
            userGrpcDeadlineClientInterceptor);
  }

  @Bean
  PrimaryProfileProvisioner grpcPrimaryProfileProvisioner(
      UserServiceGrpc.UserServiceBlockingStub userGrpcStub) {
    return new GrpcPrimaryProfileProvisioner(userGrpcStub);
  }

  @Bean
  PhoneHashResolver grpcPhoneHashResolver(
      UserServiceGrpc.UserServiceBlockingStub userGrpcStub, AccountRepository accounts) {
    return new GrpcPhoneHashResolver(userGrpcStub, accounts);
  }

  @Bean
  ProfileSwitchValidator grpcProfileSwitchValidator(
      UserServiceGrpc.UserServiceBlockingStub userGrpcStub) {
    return new GrpcProfileSwitchValidator(userGrpcStub);
  }

  @Bean
  UserVerificationSync grpcUserVerificationSync(
      UserServiceGrpc.UserServiceBlockingStub userGrpcStub) {
    return new GrpcUserVerificationSync(userGrpcStub);
  }

  private static String normalizeTarget(String addr) {
    if (addr.startsWith(":")) {
      return "localhost" + addr;
    }
    return addr;
  }
}
