package voice.backend.auth.config;

import java.time.Clock;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.io.ResourceLoader;
import voice.backend.auth.security.JwtService;

@Configuration
public class JwtServiceConfiguration {
  @Bean
  JwtService jwtService(AuthProperties properties, Clock clock, ResourceLoader resourceLoader) {
    var jwt = properties.getJwt();
    if (properties.getPersistence() == AuthProperties.PersistenceMode.MEMORY) {
      return JwtService.forTests(
          jwt.getIssuer(), jwt.getAudience(), jwt.getKeyId(), jwt.getAccessTtl(), clock);
    }
    String pem = JwtPrivateKeyLoader.loadPem(jwt, resourceLoader);
    if (pem.isBlank()) {
      throw new IllegalStateException(
          "JWT signing key is required when auth.persistence=jdbc: set auth.jwt.private-key-pem or auth.jwt.private-key-location");
    }
    return JwtService.fromPkcs8PrivateKeyPem(
        jwt.getIssuer(), jwt.getAudience(), jwt.getKeyId(), jwt.getAccessTtl(), clock, pem);
  }
}
