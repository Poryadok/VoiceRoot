package voice.backend.auth.principal;

import com.fasterxml.jackson.databind.JsonNode;
import java.math.BigInteger;
import java.security.KeyFactory;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.RSAPublicKeySpec;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Base64;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;

/** Complete current+next JWKS snapshots with bounded last-good and unknown-kid refresh. */
public final class AuthPrincipalJwksResolver {
  @FunctionalInterface public interface Fetcher { byte[] fetch(String issuer); }
  private record Snapshot(Map<String, RSAPublicKey> keys, Instant fetched) {}
  private record SuccessfulAbsence(String kid, Snapshot snapshot, Instant observed) {}
  static final class UnapprovedPrincipalKeyException extends IllegalArgumentException {
    UnapprovedPrincipalKeyException() { super("principal key is not approved"); }
  }
  private final Fetcher fetcher;
  private final Clock clock;
  private final Duration refreshAfter, hardExpiry, cooldown;
  private final Map<String, Snapshot> sets = new HashMap<>();
  private final Map<String, Instant> unknownRefresh = new HashMap<>();
  // One proven negative per issuer, never an attacker-controlled unbounded kid cache.
  private final Map<String, SuccessfulAbsence> botFloorAbsence = new HashMap<>();

  public AuthPrincipalJwksResolver(Fetcher fetcher, Clock clock, Duration refreshAfter, Duration hardExpiry, Duration cooldown) {
    this.fetcher = Objects.requireNonNull(fetcher);
    this.clock = Objects.requireNonNull(clock);
    if (refreshAfter.isNegative() || refreshAfter.isZero() || hardExpiry.compareTo(refreshAfter) < 0
        || cooldown.isNegative() || cooldown.isZero()) throw new IllegalArgumentException("invalid JWKS cache policy");
    this.refreshAfter = refreshAfter;
    this.hardExpiry = hardExpiry;
    this.cooldown = cooldown;
  }

  public synchronized RSAPublicKey resolve(String issuer, String kid) {
    if (issuer == null || issuer.isBlank() || kid == null || kid.isBlank()) throw unavailable();
    Instant now = clock.instant();
    Snapshot cached = sets.get(issuer);
    RSAPublicKey known = usable(cached, now) ? cached.keys().get(kid) : null;
    if (known != null && Duration.between(cached.fetched(), now).compareTo(refreshAfter) < 0) return known;
    Instant lastUnknown = unknownRefresh.get(issuer);
    if (lastUnknown != null && Duration.between(lastUnknown, now).compareTo(cooldown) < 0) {
      if (known != null) return known;
      throw unavailable();
    }
    boolean unknown = cached == null || !cached.keys().containsKey(kid);
    if (unknown) unknownRefresh.put(issuer, now);
    try {
      Map<String, RSAPublicKey> keys = parse(fetcher.fetch(issuer));
      Snapshot updated = new Snapshot(keys, clock.instant());
      sets.put(issuer, updated);
      RSAPublicKey key = keys.get(kid);
      if (key != null) return key;
      unknownRefresh.put(issuer, clock.instant());
    } catch (Exception ex) {
      // Failed/incomplete refresh never replaces or extends the last-good snapshot.
      unknownRefresh.put(issuer, clock.instant());
      if (usable(cached, clock.instant()) && cached.keys().containsKey(kid)) return cached.keys().get(kid);
    }
    throw unavailable();
  }

  /** Startup preflight for an explicitly closed endpoint policy, outside a serving request. */
  synchronized void prime(String issuer) {
    try {
      if (issuer == null || issuer.isBlank()) throw new IllegalArgumentException();
      var keys = parse(fetcher.fetch(issuer));
      sets.put(issuer,new Snapshot(keys,clock.instant()));
      unknownRefresh.remove(issuer);
    } catch (Exception unavailable) { throw unavailable(); }
  }

  /** Bot floor only: distinguish a successfully refreshed authoritative absence from an outage. */
  public synchronized RSAPublicKey resolveForBotFloor(String issuer, String kid) {
    if (issuer == null || issuer.isBlank() || kid == null || kid.isBlank()) throw unavailable();
    Instant now = clock.instant();
    Snapshot cached = sets.get(issuer);
    RSAPublicKey known = usable(cached, now) ? cached.keys().get(kid) : null;
    if (known != null && Duration.between(cached.fetched(), now).compareTo(refreshAfter) < 0) return known;
    Instant lastUnknown = unknownRefresh.get(issuer);
    if (lastUnknown != null && Duration.between(lastUnknown, now).compareTo(cooldown) < 0) {
      if (known != null) return known;
      SuccessfulAbsence absent = botFloorAbsence.get(issuer);
      if (absent != null && absent.kid().equals(kid) && absent.snapshot() == cached
          && absent.observed().equals(lastUnknown) && usable(cached, now)) {
        throw new UnapprovedPrincipalKeyException();
      }
      throw unavailable();
    }
    if (cached == null || !cached.keys().containsKey(kid)) unknownRefresh.put(issuer, now);
    final Map<String, RSAPublicKey> keys;
    try {
      keys = parse(fetcher.fetch(issuer));
    } catch (Exception ex) {
      unknownRefresh.put(issuer, clock.instant());
      botFloorAbsence.remove(issuer);
      if (usable(cached, clock.instant()) && cached.keys().containsKey(kid)) return cached.keys().get(kid);
      throw unavailable();
    }
    Snapshot updated = new Snapshot(keys, clock.instant());
    sets.put(issuer, updated);
    RSAPublicKey key = keys.get(kid);
    if (key != null) return key;
    Instant observed = clock.instant();
    unknownRefresh.put(issuer, observed);
    botFloorAbsence.put(issuer, new SuccessfulAbsence(kid, updated, observed));
    // Deliberately outside the fetch/parse catch: authoritative rejection is not an outage.
    throw new UnapprovedPrincipalKeyException();
  }

  private boolean usable(Snapshot cached, Instant now) {
    if (cached == null) return false;
    Duration age = Duration.between(cached.fetched(), now);
    return !age.isNegative() && age.compareTo(hardExpiry) < 0;
  }

  private static Map<String, RSAPublicKey> parse(byte[] document) throws Exception {
    if (document == null || document.length > 65536) throw unavailable();
    JsonNode root = AuthPrincipalVerifier.JSON.readTree(document);
    JsonNode candidates = root == null ? null : root.get("keys");
    if (candidates == null || !candidates.isArray() || candidates.size() < 2 || candidates.size() > 16) throw unavailable();
    Map<String, RSAPublicKey> result = new HashMap<>();
    for (JsonNode candidate : candidates) {
      String kid = AuthPrincipalVerifier.string(candidate, "kid");
      if (!kid.matches("[A-Za-z0-9][A-Za-z0-9._-]{0,127}") || result.containsKey(kid)
          || !"RSA".equals(AuthPrincipalVerifier.string(candidate, "kty"))
          || !"sig".equals(AuthPrincipalVerifier.string(candidate, "use"))
          || !"RS256".equals(AuthPrincipalVerifier.string(candidate, "alg")) || candidate.has("d")) throw unavailable();
      BigInteger n = number(AuthPrincipalVerifier.string(candidate, "n"));
      BigInteger e = number(AuthPrincipalVerifier.string(candidate, "e"));
      if (n.bitLength() < 2048 || n.bitLength() > 8192 || e.bitLength() > 31 || e.compareTo(BigInteger.valueOf(3)) < 0 || !e.testBit(0)) throw unavailable();
      result.put(kid, (RSAPublicKey) KeyFactory.getInstance("RSA").generatePublic(new RSAPublicKeySpec(n, e)));
    }
    return Map.copyOf(result);
  }

  private static BigInteger number(String value) {
    if (!value.matches("[A-Za-z0-9_-]+")) throw unavailable();
    return new BigInteger(1, Base64.getUrlDecoder().decode(value));
  }

  private static IllegalArgumentException unavailable() {
    return new PrincipalDependencyUnavailableException();
  }
}
