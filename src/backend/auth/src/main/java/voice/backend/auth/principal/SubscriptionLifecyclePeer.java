package voice.backend.auth.principal;

import io.grpc.Attributes;
import io.grpc.Grpc;
import java.net.URI;
import java.security.cert.X509Certificate;

/** Exact peer SAN on the dedicated Subscription capability, in addition to its JWT. */
final class SubscriptionLifecyclePeer {
  private SubscriptionLifecyclePeer() {}
  static String normalized(String value) {
    try {
      if (value == null || value.isBlank() || value.contains("*")) throw new IllegalArgumentException();
      URI uri = URI.create(value);
      if (!uri.isAbsolute() || uri.getRawUserInfo() != null || uri.getRawQuery() != null || uri.getRawFragment() != null
          || !uri.normalize().toASCIIString().equals(value)) throw new IllegalArgumentException();
      return value;
    } catch (RuntimeException invalid) { throw new IllegalArgumentException("invalid Subscription lifecycle peer"); }
  }
  static void require(Attributes attributes, String allowed) {
    var session = attributes.get(Grpc.TRANSPORT_ATTR_SSL_SESSION);
    if (session == null || allowed == null) throw new IllegalArgumentException("Subscription lifecycle peer requires mutual TLS");
    try {
      var chain = session.getPeerCertificates();
      if (chain.length > 0 && chain[0] instanceof X509Certificate certificate) {
        var names = certificate.getSubjectAlternativeNames();
        if (names != null) for (var name : names) {
          if (name.size() == 2 && Integer.valueOf(6).equals(name.get(0)) && allowed.equals(name.get(1))) return;
        }
      }
    } catch (Exception denied) { throw new IllegalArgumentException("Subscription lifecycle peer unavailable"); }
    throw new IllegalArgumentException("Subscription lifecycle peer denied");
  }
}
