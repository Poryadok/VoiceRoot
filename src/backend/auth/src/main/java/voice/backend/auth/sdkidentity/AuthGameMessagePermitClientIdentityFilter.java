package voice.backend.auth.sdkidentity;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.security.cert.X509Certificate;
import org.springframework.web.filter.OncePerRequestFilter;

/** Restricts T16 public-style HTTP route paths to the registered Messaging mTLS identity. */
final class AuthGameMessagePermitClientIdentityFilter extends OncePerRequestFilter {
  private final boolean enabled;
  private final String messagingSan;

  AuthGameMessagePermitClientIdentityFilter(AuthGameBindingMtlsConfiguration.AuthGameBindingMtlsSettings settings) {
    this.enabled = settings.enabled();
    this.messagingSan = settings.allowedMessagingClientUriSan();
  }

  @Override
  protected boolean shouldNotFilter(HttpServletRequest request) {
    return !request.getRequestURI().startsWith("/api/v1/auth/sdk/game-message/execution-permits");
  }

  @Override
  protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
      throws ServletException, IOException {
    Object attribute = request.getAttribute(AuthGameBindingClientIdentityFilter.CERTIFICATE_ATTRIBUTE);
    if (!enabled || !(attribute instanceof X509Certificate[] certificates) || certificates.length == 0
        || !AuthGameBindingClientIdentityFilter.hasExactUriSan(certificates[0], messagingSan)) {
      response.sendError(HttpServletResponse.SC_UNAUTHORIZED);
      return;
    }
    chain.doFilter(request, response);
  }
}
