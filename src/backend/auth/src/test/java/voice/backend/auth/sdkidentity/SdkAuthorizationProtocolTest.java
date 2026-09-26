package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Arrays;
import java.util.Base64;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.NullAndEmptySource;
import org.junit.jupiter.params.provider.ValueSource;

/** GAME-AUTH-02 canonical request, PKCE S256 and registry policy contracts. */
class SdkAuthorizationProtocolTest {
  private static final UUID KEY = UUID.fromString("55555555-5555-4555-8555-555555555555");
  private static final UUID APP = UUID.fromString("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa");
  private static final UUID ENV = UUID.fromString("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb");
  private static final String REDIRECT = "https://voice.example/sdk/callback?source=game";
  private static final String VERIFIER = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk";
  private static final String CHALLENGE = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM";
  private static final String STATE = "s".repeat(43);
  private static final Set<String> SCOPES = Set.of("game.voice.join", "game.identity.read");
  private static final Set<String> ALL_PLAYER_SCOPES = Set.of("game.identity.read", "game.chat.read",
      "game.chat.send", "game.voice.join", "game.presence.write", "game.invites.create");

  private String hash(UUID key, String redirect, String challenge, String state, Set<String> scopes) {
    return SdkAuthorizationProtocol.requestHash(key, redirect, challenge, state, scopes);
  }

  private SdkAuthorizationPolicy.Policy policy() {
    return new SdkAuthorizationPolicy.Policy(APP, ENV, 7, "Example game", Set.of(REDIRECT), ALL_PLAYER_SCOPES);
  }

  private void denied(org.assertj.core.api.ThrowableAssert.ThrowingCallable action) {
    assertThatThrownBy(action).isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
  }

  @Test
  void requestHashMatchesExactUtf8CanonicalBytesWithSortedScopesAndNoTrailingNewline() throws Exception {
    String canonical = "voice-sdk-authorization-request-v1\n55555555-5555-4555-8555-555555555555\n"
        + REDIRECT + "\n" + CHALLENGE + "\n" + STATE + "\ngame.identity.read,game.voice.join";
    String expected = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
        .digest(canonical.getBytes(StandardCharsets.UTF_8)));
    assertThat(hash(KEY, REDIRECT, CHALLENGE, STATE, SCOPES)).isEqualTo(expected).matches("[a-f0-9]{64}");
  }

  @Test
  void scopeOrderDoesNotChangeHashOrMutateCallerSet() {
    var first = new LinkedHashSet<>(List.of("game.voice.join", "game.identity.read"));
    var second = new LinkedHashSet<>(List.of("game.identity.read", "game.voice.join"));
    assertThat(hash(KEY, REDIRECT, CHALLENGE, STATE, first))
        .isEqualTo(hash(KEY, REDIRECT, CHALLENGE, STATE, second));
    assertThat(first).containsExactly("game.voice.join", "game.identity.read");
  }

  @Test
  void everyRequestFieldIsBoundIntoDigest() {
    String original = hash(KEY, REDIRECT, CHALLENGE, STATE, SCOPES);
    assertThat(List.of(
        hash(UUID.fromString("66666666-6666-4666-8666-666666666666"), REDIRECT, CHALLENGE, STATE, SCOPES),
        hash(KEY, REDIRECT + "2", CHALLENGE, STATE, SCOPES),
        hash(KEY, REDIRECT, "a".repeat(43), STATE, SCOPES),
        hash(KEY, REDIRECT, CHALLENGE, "t".repeat(43), SCOPES),
        hash(KEY, REDIRECT, CHALLENGE, STATE, Set.of("game.identity.read"))))
        .doesNotContain(original).doesNotHaveDuplicates();
  }

  @Test
  void rejectsMissingIdempotencyKeyAndInvalidScopeSets() {
    denied(() -> hash(null, REDIRECT, CHALLENGE, STATE, SCOPES));
    for (Set<String> scopes : Arrays.asList(null, Set.<String>of(),
        new HashSet<>(Arrays.asList("game.identity.read", null)), Set.of(""),
        Set.of("game.identity.read,game.voice.join"), Set.of("game.identity.read\ngame.voice.join"),
        Set.of("game.sessions.manage"), Set.of("game.memberships.sync"), Set.of("game.events.publish"),
        Set.of("game.commands.receive"), Set.of("unknown.scope"), Set.of("GAME.IDENTITY.READ"))) {
      denied(() -> hash(KEY, REDIRECT, CHALLENGE, STATE, scopes));
    }
    Set<String> tooMany = new HashSet<>();
    for (int i = 0; i < 17; i++) tooMany.add("game.scope." + i);
    denied(() -> hash(KEY, REDIRECT, CHALLENGE, STATE, tooMany));
    assertThat(hash(KEY, REDIRECT, CHALLENGE, STATE, ALL_PLAYER_SCOPES)).matches("[a-f0-9]{64}");
  }

  @ParameterizedTest
  @NullAndEmptySource
  @ValueSource(strings = {"/relative", "//voice.example/callback", "https://voice.example/cb#fragment",
      "https://user:password@voice.example/cb", "https://voice.example/cb\r\nX-Test: injected",
      "https://voice.example/cb\n", "https://voice.example/cb with space", ":malformed"})
  void rejectsInvalidRedirectBeforeHashing(String redirect) {
    denied(() -> hash(KEY, redirect, CHALLENGE, STATE, SCOPES));
  }

  @ParameterizedTest
  @NullAndEmptySource
  @ValueSource(strings = {"short", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM=",
      "+aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
  void rejectsInvalidS256ChallengeBeforeHashing(String challenge) {
    denied(() -> hash(KEY, REDIRECT, challenge, STATE, SCOPES));
  }

  @Test
  void stateRequiresBase64UrlAndExactLengthBounds() {
    for (String state : Arrays.asList(null, "", "s".repeat(42), "s".repeat(129),
        "s".repeat(42) + "=", "s".repeat(42) + "+", "s".repeat(42) + "/",
        "s".repeat(42) + "\n", "s".repeat(42) + "é")) {
      denied(() -> hash(KEY, REDIRECT, CHALLENGE, state, SCOPES));
    }
    assertThat(hash(KEY, REDIRECT, CHALLENGE, "_-" + "s".repeat(126), SCOPES)).isNotBlank();
  }

  @Test
  void verifiesRfc7636S256VectorAndNeverAcceptsPlainChallenge() {
    assertThat(SdkAuthorizationProtocol.verifyPkce(VERIFIER, CHALLENGE)).isTrue();
    assertThat(SdkAuthorizationProtocol.verifyPkce(VERIFIER, VERIFIER)).isFalse();
    assertThat(SdkAuthorizationProtocol.verifyPkce("a".repeat(43), CHALLENGE)).isFalse();
    assertThat(SdkAuthorizationProtocol.verifyPkce(VERIFIER, CHALLENGE.toLowerCase())).isFalse();
  }

  @Test
  void pkceAllowsUnreservedAsciiAtVerifierLengthBoundaries() throws Exception {
    for (String verifier : List.of("._~-" + "a".repeat(39), "._~-" + "z".repeat(124))) {
      String challenge = Base64.getUrlEncoder().withoutPadding().encodeToString(
          MessageDigest.getInstance("SHA-256").digest(verifier.getBytes(StandardCharsets.US_ASCII)));
      assertThat(SdkAuthorizationProtocol.verifyPkce(verifier, challenge)).isTrue();
    }
  }

  @Test
  void malformedPkceInputsReturnFalseEvenIfTheirDigestMatches() throws Exception {
    for (String verifier : Arrays.asList(null, "", "a".repeat(42), "a".repeat(129),
        "a".repeat(42) + "+", "a".repeat(42) + "/", "a".repeat(42) + "=",
        "a".repeat(42) + " ", "a".repeat(42) + "\n", "a".repeat(42) + "é")) {
      String digest = verifier == null ? CHALLENGE : Base64.getUrlEncoder().withoutPadding().encodeToString(
          MessageDigest.getInstance("SHA-256").digest(verifier.getBytes(StandardCharsets.UTF_8)));
      assertThat(SdkAuthorizationProtocol.verifyPkce(verifier, digest)).isFalse();
    }
    for (String challenge : Arrays.asList(null, "", CHALLENGE + "=", CHALLENGE.substring(1),
        "+" + CHALLENGE.substring(1), "/" + CHALLENGE.substring(1))) {
      assertThat(SdkAuthorizationProtocol.verifyPkce(VERIFIER, challenge)).isFalse();
    }
  }

  @Test
  void acceptsExactRegistryPolicyWithRequestedPlayerScopes() {
    assertThatCode(() -> SdkAuthorizationProtocol.requirePolicy(policy(), APP, ENV, REDIRECT, SCOPES))
        .doesNotThrowAnyException();
  }

  @Test
  void registryOwnsAllowedRedirectSchemes() {
    for (String redirect : List.of("voicegame://auth/callback", "http://127.0.0.1:8765/callback")) {
      var policy = new SdkAuthorizationPolicy.Policy(APP, ENV, 1, "Game", Set.of(redirect), SCOPES);
      assertThat(hash(KEY, redirect, CHALLENGE, STATE, SCOPES)).isNotBlank();
      assertThatCode(() -> SdkAuthorizationProtocol.requirePolicy(policy, APP, ENV, redirect, SCOPES))
          .doesNotThrowAnyException();
    }
  }

  @Test
  void policyMustExistMatchApplicationEnvironmentAndHavePositiveRevisionAndDisplayName() {
    denied(() -> SdkAuthorizationProtocol.requirePolicy(null, APP, ENV, REDIRECT, SCOPES));
    denied(() -> SdkAuthorizationProtocol.requirePolicy(policy(), null, ENV, REDIRECT, SCOPES));
    denied(() -> SdkAuthorizationProtocol.requirePolicy(policy(), APP, null, REDIRECT, SCOPES));
    for (UUID app : Arrays.asList(null, ENV)) {
      denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
          app, ENV, 1, "Game", Set.of(REDIRECT), SCOPES), APP, ENV, REDIRECT, SCOPES));
    }
    for (UUID env : Arrays.asList(null, APP)) {
      denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
          APP, env, 1, "Game", Set.of(REDIRECT), SCOPES), APP, ENV, REDIRECT, SCOPES));
    }
    for (long revision : new long[] {0, -1}) {
      denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
          APP, ENV, revision, "Game", Set.of(REDIRECT), SCOPES), APP, ENV, REDIRECT, SCOPES));
    }
    for (String name : Arrays.asList(null, "", " \t ")) {
      denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
          APP, ENV, 1, name, Set.of(REDIRECT), SCOPES), APP, ENV, REDIRECT, SCOPES));
    }
  }

  @Test
  void policyRedirectComparisonIsExactWithoutUriNormalizationOrPrefixMatching() {
    for (String redirect : Arrays.asList(null, "", REDIRECT + "/", REDIRECT + "&attacker=1",
        REDIRECT.replace("voice.example", "VOICE.EXAMPLE"),
        REDIRECT.replace("source=game", "source=%67ame"), "https://voice.example/sdk/")) {
      denied(() -> SdkAuthorizationProtocol.requirePolicy(policy(), APP, ENV, redirect, SCOPES));
    }
  }

  @Test
  void missingAllowlistOrScopesAndUnapprovedOrServiceScopesFailClosed() {
    denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
        APP, ENV, 1, "Game", null, SCOPES), APP, ENV, REDIRECT, SCOPES));
    denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
        APP, ENV, 1, "Game", Set.of(), SCOPES), APP, ENV, REDIRECT, SCOPES));
    denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
        APP, ENV, 1, "Game", Set.of(REDIRECT), null), APP, ENV, REDIRECT, SCOPES));
    denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
        APP, ENV, 1, "Game", Set.of(REDIRECT), Set.of("game.identity.read")), APP, ENV, REDIRECT, SCOPES));
    for (Set<String> scopes : Arrays.asList(null, Set.<String>of(), Set.of("game.sessions.manage"),
        Set.of("unknown.scope"))) {
      denied(() -> SdkAuthorizationProtocol.requirePolicy(policy(), APP, ENV, REDIRECT, scopes));
    }
    Set<String> serviceScope = Set.of("game.sessions.manage");
    denied(() -> SdkAuthorizationProtocol.requirePolicy(new SdkAuthorizationPolicy.Policy(
        APP, ENV, 1, "Game", Set.of(REDIRECT), serviceScope), APP, ENV, REDIRECT, serviceScope));
  }
}
