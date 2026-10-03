package voice.backend.auth.sdkidentity;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;

/** Publishes the dedicated Auth service-principal keys, separate from client JWT keys. */
@RestController
@ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
@RequestMapping("/api/v1/auth")
public final class AuthUserPrincipalJwksController {
  private final AuthUserPrincipalIssuer issuer;

  public AuthUserPrincipalJwksController(AuthUserPrincipalIssuer issuer) {
    this.issuer = issuer;
  }

  @GetMapping("/.well-known/principal-jwks.json")
  public String jwks() {
    return issuer.jwksJson();
  }
}
