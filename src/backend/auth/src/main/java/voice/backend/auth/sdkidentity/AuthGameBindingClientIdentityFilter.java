package voice.backend.auth.sdkidentity;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.security.cert.X509Certificate;
import java.util.Collection;
import java.util.List;
import org.springframework.web.filter.OncePerRequestFilter;

/** Checks the verified client certificate URI SAN on the private binding routes. */
final class AuthGameBindingClientIdentityFilter extends OncePerRequestFilter {
  static final String CERTIFICATE_ATTRIBUTE = "jakarta.servlet.request.X509Certificate";
  private final boolean enabled;
  private final String allowedUriSan;

  AuthGameBindingClientIdentityFilter(AuthGameBindingMtlsConfiguration.AuthGameBindingMtlsSettings settings) {
    this.enabled = settings.enabled();
    this.allowedUriSan = settings.allowedClientUriSan();
  }

  @Override
  protected boolean shouldNotFilter(HttpServletRequest request) {
    return !request.getRequestURI().startsWith("/internal/v1/auth/game-bindings/handoffs/");
  }

  @Override
  protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
      throws ServletException, IOException {
    Object attribute = request.getAttribute(CERTIFICATE_ATTRIBUTE);
    if (!enabled || !(attribute instanceof X509Certificate[] chainCertificates) || chainCertificates.length == 0
        || !hasExactUriSan(chainCertificates[0], allowedUriSan)) {
      response.sendError(HttpServletResponse.SC_UNAUTHORIZED);
      return;
    }
    chain.doFilter(request, response);
  }

  static boolean hasExactUriSan(X509Certificate certificate, String expected) {
    if (certificate == null || expected == null || expected.isBlank()) return false;
    try {
      Collection<List<?>> names = certificate.getSubjectAlternativeNames();
      if (names == null) return false;
      for (List<?> name : names) {
        if (name.size() == 2 && Integer.valueOf(6).equals(name.get(0)) && expected.equals(name.get(1))) return true;
      }
    } catch (Exception malformed) { return false; }
    return false;
  }
}
