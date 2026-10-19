package voice.backend.auth.principal;

import io.grpc.Attributes;
import io.grpc.Grpc;
import java.net.URI;
import java.security.cert.X509Certificate;

/** Additional identity binding for the Bot RPC on the existing client-CA TLS listener. */
final class BotSessionFloorPeer {
  private BotSessionFloorPeer() {}

  static String normalized(String value) {
    try {
      if (value == null || value.isBlank() || value.contains("*")) return null;
      URI uri = URI.create(value);
      if (!uri.isAbsolute() || uri.getRawUserInfo() != null || uri.getRawQuery() != null
          || uri.getRawFragment() != null || !uri.normalize().toASCIIString().equals(value)) return null;
      return value;
    } catch (IllegalArgumentException malformed) { return null; }
  }

  static void require(Attributes attributes, String allowed) {
    if (allowed == null) throw new IllegalArgumentException("Bot peer is not configured");
    var session = attributes.get(Grpc.TRANSPORT_ATTR_SSL_SESSION);
    if (session == null) throw new IllegalArgumentException("Bot peer requires TLS");
    try {
      // getPeerCertificates succeeds only for an authenticated peer. The existing floor
      // listener verifies the chain against AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE.
      var chain = session.getPeerCertificates();
      if (chain.length > 0 && chain[0] instanceof X509Certificate certificate) {
        var names = certificate.getSubjectAlternativeNames();
        if (names != null) for (var name : names) {
          if (name.size() == 2 && Integer.valueOf(6).equals(name.get(0)) && allowed.equals(name.get(1))) return;
        }
      }
    } catch (Exception unavailable) {
      throw new IllegalArgumentException("Bot peer is not verified");
    }
    throw new IllegalArgumentException("Bot peer is not permitted");
  }
}
