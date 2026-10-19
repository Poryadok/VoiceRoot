package voice.backend.auth.releasepolicy;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.StreamReadFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.Map;
import java.util.Set;
import java.util.function.Function;

/** Immutable owner authority. Projections never grant capabilities. */
public final class ReleasePolicy {
  public static final String ALPHA = "free-invite-alpha";
  private static final Set<String> CAPABILITIES = Set.of("phone_auth", "paid", "mobile_push",
      "public_discovery", "unfinished_rich_types", "federation");
  private static final Set<String> BASELINE = Set.of("text", "photo", "video", "document", "music");
  private static final Set<String> RICH = Set.of("text", "photo", "video", "document", "music",
      "voice", "sticker", "gif", "article", "location", "video_note", "send_silent", "schedule", "when_online");
  private final String profile;
  private final String source;
  private final String sha256;

  private ReleasePolicy(String profile, String source, String sha256) {
    this.profile = profile; this.source = source; this.sha256 = sha256;
  }
  public static ReleasePolicy development() {
    return new ReleasePolicy("development", "", "");
  }
  public static ReleasePolicy load(Map<String,String> environment, String buildSource) {
    return load(environment::get, buildSource);
  }
  public static ReleasePolicy load(Function<String,String> lookup, String buildSource) {
    String profile = lookup.apply("VOICE_RELEASE_PROFILE");
    String file = lookup.apply("VOICE_RELEASE_POLICY_FILE");
    String sha = lookup.apply("VOICE_RELEASE_POLICY_SHA256");
    if ("development".equals(profile)) {
      if (file != null || sha != null || lookup.apply("VOICE_RELEASE_SNAPSHOT_FILE") != null) throw invalid();
      return development();
    }
    if (!ALPHA.equals(profile) || file == null || !Path.of(file).isAbsolute()) throw invalid();
    try (var input = Files.newInputStream(Path.of(file))) {
      return parse(input.readNBytes(65537), sha, buildSource);
    } catch (Exception ex) { throw invalid(); }
  }
  public static ReleasePolicy parse(byte[] data, String expectedDigest, String buildSource) {
    if (data == null || data.length == 0 || data.length > 65536
        || !hex(expectedDigest, 64) || !hex(buildSource, 40)
        || !digest(data).equals(expectedDigest)) throw invalid();
    try {
      ObjectMapper mapper = new ObjectMapper(JsonFactory.builder()
          .enable(StreamReadFeature.STRICT_DUPLICATE_DETECTION).build());
      try (var parser = mapper.createParser(data)) {
        JsonNode root = mapper.readTree(parser);
        if (parser.nextToken() != null) throw invalid();
        keys(root, Set.of("schema_version", "profile", "source_sha", "capabilities", "rich_intents"));
        if (!root.get("schema_version").isIntegralNumber() || !root.get("schema_version").canConvertToInt()
            || root.get("schema_version").intValue() != 2
            || !root.get("profile").isTextual() || !ALPHA.equals(root.get("profile").textValue())
            || !root.get("source_sha").isTextual() || !buildSource.equals(root.get("source_sha").textValue())) throw invalid();
        JsonNode capabilities = root.get("capabilities"); keys(capabilities, CAPABILITIES);
        for (String id : CAPABILITIES) if (!capabilities.get(id).isTextual()
            || !"disabled".equals(capabilities.get(id).textValue())) throw invalid();
        JsonNode rich = root.get("rich_intents"); keys(rich, RICH);
        for (String id : RICH) {
          JsonNode state = rich.get(id);
          if (!state.isTextual() || !("qualified".equals(state.textValue())
              || !BASELINE.contains(id) && "disabled".equals(state.textValue()))) throw invalid();
        }
      }
      return new ReleasePolicy(ALPHA, buildSource, expectedDigest);
    } catch (Exception ex) { throw invalid(); }
  }
  private static void keys(JsonNode object, Set<String> keys) {
    if (object == null || !object.isObject() || object.size() != keys.size()) throw invalid();
    var names = object.fieldNames(); while (names.hasNext()) if (!keys.contains(names.next())) throw invalid();
  }
  private static boolean hex(String value, int size) {
    return value != null && value.matches("[0-9a-f]{" + size + "}") && !value.equals("0".repeat(size));
  }
  public static String digest(byte[] data) {
    try { return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(data)); }
    catch (java.security.NoSuchAlgorithmException impossible) { throw new IllegalStateException(impossible); }
  }
  private static IllegalArgumentException invalid() { return new IllegalArgumentException("invalid or unbound release policy"); }
  public boolean allowsPhoneAuth() { return "development".equals(profile); }
  public String profile() { return profile; }
  public String sourceSHA() { return source; }
  public String sha256() { return sha256; }
  public Map<String,String> capabilityStates() {
    return CAPABILITIES.stream().collect(java.util.stream.Collectors.toUnmodifiableMap(
        Function.identity(), ignored -> allowsPhoneAuth() ? "development" : "disabled"));
  }
}
