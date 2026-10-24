package voice.backend.auth.roleidentityauthority;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import java.util.UUID;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/** Dedicated owner-purpose selectors; no existing Auth key family is borrowed. */
public final class RoleIdentitySelectors {
  private static final byte[] DOMAIN =
      "voice-role-identity-selector-v1\0auth\0account\0".getBytes(StandardCharsets.US_ASCII);
  private final long revision;
  private final int activeVersion;
  private final Map<Integer, byte[]> keys;
  private final List<Integer> versions;

  public RoleIdentitySelectors(long revision, int activeVersion, Map<Integer, byte[]> keys) {
    if (revision <= 0 || activeVersion <= 0 || keys == null || !keys.containsKey(activeVersion)) {
      throw invalid();
    }
    var copied = new TreeMap<Integer, byte[]>();
    var seen = new HashSet<String>();
    for (var entry : keys.entrySet()) {
      Integer version = entry.getKey();
      byte[] key = entry.getValue();
      if (version == null || version <= 0 || key == null || key.length != 32
          || Arrays.equals(key, new byte[32])
          || !seen.add(java.util.HexFormat.of().formatHex(key))) {
        throw invalid();
      }
      copied.put(version, key.clone());
    }
    this.revision = revision;
    this.activeVersion = activeVersion;
    this.keys = Map.copyOf(copied);
    this.versions = List.copyOf(copied.keySet());
  }

  public byte[] activeAccount(UUID account) { return account(activeVersion, account); }

  public Map<Integer, byte[]> allAccounts(UUID account) {
    var result = new LinkedHashMap<Integer, byte[]>();
    for (int version : versions) result.put(version, account(version, account));
    return Map.copyOf(result);
  }

  public long keysetRevision() { return revision; }
  public int activeVersion() { return activeVersion; }
  public List<Integer> retainedVersions() { return versions; }

  Map<Integer, byte[]> keyFingerprints() {
    var result = new LinkedHashMap<Integer, byte[]>();
    try {
      var digest = java.security.MessageDigest.getInstance("SHA-256");
      for (int version : versions) result.put(version, digest.digest(keys.get(version)));
      return Map.copyOf(result);
    } catch (java.security.NoSuchAlgorithmException impossible) {
      throw unavailable();
    }
  }

  public void requireRetainedVersions(List<Integer> required) {
    var seen = new HashSet<Integer>();
    if (required == null) throw unavailable();
    for (Integer version : required) {
      if (version == null || version <= 0 || !seen.add(version) || !keys.containsKey(version)) {
        throw unavailable();
      }
    }
  }

  private byte[] account(int version, UUID account) {
    if (account == null || account.equals(new UUID(0, 0))) throw invalid();
    byte[] key = keys.get(version);
    if (key == null) throw unavailable();
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(key, "HmacSHA256"));
      mac.update(DOMAIN);
      return mac.doFinal(ByteBuffer.allocate(16).putLong(account.getMostSignificantBits())
          .putLong(account.getLeastSignificantBits()).array());
    } catch (java.security.GeneralSecurityException unavailable) {
      throw unavailable();
    }
  }

  private static IllegalArgumentException invalid() {
    return new IllegalArgumentException("invalid identity selector configuration or input");
  }

  private static IllegalStateException unavailable() {
    return new IllegalStateException("identity selector keyset unavailable");
  }
}
