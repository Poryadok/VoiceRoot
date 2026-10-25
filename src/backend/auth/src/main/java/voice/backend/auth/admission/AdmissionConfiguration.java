package voice.backend.auth.admission;

import app.voice.user.v1.UserServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import java.io.File;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

@Configuration(proxyBeanMethods=false)
public class AdmissionConfiguration {
  record Settings(boolean enabled) {}
  @Bean Settings admissionSettings(Environment env) {
    boolean enabled=env.getProperty("auth.space-admission.enabled",Boolean.class,false);
    var names=java.util.List.of("GRPC_ADDR","TLS_CA_FILE","TLS_CERT_FILE","TLS_KEY_FILE","TLS_SERVER_NAME");
    boolean any=names.stream().anyMatch(suffix->env.containsProperty("AUTH_SPACE_ADMISSION_USER_"+suffix));
    if(any&&!enabled) throw new IllegalStateException("admission client settings require explicit enablement");
    if(enabled) names.forEach(suffix->required(env,suffix));
    return new Settings(enabled);
  }
  @Bean(destroyMethod="shutdownNow")
  @ConditionalOnProperty(prefix="auth.space-admission",name="enabled",havingValue="true")
  ManagedChannel admissionUserChannel(Environment env) {
    try {
      var tls=GrpcSslContexts.forClient().trustManager(new File(required(env,"TLS_CA_FILE")))
          .keyManager(new File(required(env,"TLS_CERT_FILE")),new File(required(env,"TLS_KEY_FILE"))).build();
      return NettyChannelBuilder.forTarget(required(env,"GRPC_ADDR")).sslContext(tls)
          .overrideAuthority(required(env,"TLS_SERVER_NAME")).build();
    } catch (Exception error) { throw new IllegalStateException("invalid admission User mutual TLS configuration",error); }
  }
  @Bean
  @ConditionalOnProperty(prefix="auth.space-admission",name="enabled",havingValue="true")
  AdmissionObservation admissionObservation(NamedParameterJdbcTemplate jdbc,SessionEpochFloorStore floors,
      AuthUserPrincipalIssuer issuer,@Qualifier("admissionUserChannel") ManagedChannel channel) {
    return new AdmissionObservation(new JdbcAdmissionAccountReader(jdbc,floors),
        new AdmissionProfileClient(UserServiceGrpc.newBlockingStub(channel),issuer));
  }
  private static String required(Environment env,String suffix) {
    String value=env.getProperty("AUTH_SPACE_ADMISSION_USER_"+suffix,"");
    if(value.isBlank()) throw new IllegalArgumentException("admission User configuration incomplete");
    return value.trim();
  }
}
