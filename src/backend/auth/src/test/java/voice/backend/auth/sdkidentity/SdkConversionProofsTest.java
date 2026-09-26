package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

/** GAME-AUTH-03 proof bytes are independently derived from the frozen wire contract. */
class SdkConversionProofsTest {
  private static final UUID KEY = UUID.fromString("11111111-1111-4111-8111-111111111111");
  private static final UUID BINDING = UUID.fromString("22222222-2222-4222-8222-222222222222");
  private static final UUID ACCOUNT = UUID.fromString("33333333-3333-4333-8333-333333333333");
  private static final UUID PROFILE = UUID.fromString("44444444-4444-4444-8444-444444444444");
  private static final UUID OPERATION = UUID.fromString("55555555-5555-4555-8555-555555555555");
  private static final UUID OTHER = UUID.fromString("66666666-6666-4666-8666-666666666666");
  private static final String TOKEN = "_-" + "a".repeat(41);
  private static final Instant NOW = Instant.parse("2026-09-26T12:00:00Z");
  private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);

  private void denied(org.assertj.core.api.ThrowableAssert.ThrowingCallable action) {
    assertThatThrownBy(action).isInstanceOf(SdkIdentityDeniedException.class)
        .hasMessage("invalid_sdk_identity");
  }

  private String sha256(String input) throws Exception {
    return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
        .digest(input.getBytes(StandardCharsets.UTF_8)));
  }

  @ParameterizedTest
  @ValueSource(strings = {"new", "existing"})
  void preparePayloadUsesExactModeTokenHashAndUuidLines(String mode) throws Exception {
    String expected = "voice-sdk-conversion-" + mode + "-v1\n" + sha256(TOKEN)
        + "\n11111111-1111-4111-8111-111111111111\n22222222-2222-4222-8222-222222222222";
    assertThat(SdkConversionProofs.preparePayload(mode, TOKEN, KEY, BINDING))
        .isEqualTo(expected).doesNotContain(TOKEN).doesNotEndWith("\n");
  }

  @Test
  void everyPrepareFieldChangesSignedBytes() {
    String original = SdkConversionProofs.preparePayload("new", TOKEN, KEY, BINDING);
    assertThat(List.of(
        SdkConversionProofs.preparePayload("existing", TOKEN, KEY, BINDING),
        SdkConversionProofs.preparePayload("new", "b".repeat(43), KEY, BINDING),
        SdkConversionProofs.preparePayload("new", TOKEN, OTHER, BINDING),
        SdkConversionProofs.preparePayload("new", TOKEN, KEY, OTHER)))
        .doesNotContain(original).doesNotHaveDuplicates();
  }

  @Test
  void rejectsInvalidModesWithoutNormalizingOrAcceptingLineInjection() {
    for (String mode : Arrays.asList(null, "", " ", "NEW", "Existing", "new ", " existing",
        "new\nexisting", "new\r", "convert")) {
      denied(() -> SdkConversionProofs.preparePayload(mode, TOKEN, KEY, BINDING));
      denied(() -> SdkConversionProofs.requestHash(mode, KEY, BINDING, null, null));
    }
  }

  @Test
  void prepareRequiresExactFortyThreeCharacterBase64UrlToken() {
    for (String token : Arrays.asList(null, "", " ", "a".repeat(42), "a".repeat(44),
        "a".repeat(42) + "=", "a".repeat(42) + "+", "a".repeat(42) + "/",
        "a".repeat(42) + "\n", "a".repeat(42) + "\r", "a".repeat(42) + "é",
        "Bearer " + TOKEN)) {
      for (String mode : List.of("new", "existing")) {
        denied(() -> SdkConversionProofs.preparePayload(mode, token, KEY, BINDING));
      }
    }
  }

  @ParameterizedTest
  @ValueSource(strings = {"new", "existing"})
  void prepareRequiresIdempotencyAndBindingUuids(String mode) {
    denied(() -> SdkConversionProofs.preparePayload(mode, TOKEN, null, BINDING));
    denied(() -> SdkConversionProofs.preparePayload(mode, TOKEN, KEY, null));
  }

  @Test
  void newRequestHashUsesLiteralDashForBothAbsentTargets() throws Exception {
    String canonical = "new\n11111111-1111-4111-8111-111111111111\n"
        + "22222222-2222-4222-8222-222222222222\n-\n-";
    assertThat(SdkConversionProofs.requestHash("new", KEY, BINDING, null, null))
        .isEqualTo(sha256(canonical)).matches("[a-f0-9]{64}");
  }

  @Test
  void existingRequestHashUsesExactSelectedTargetIdsAndNoTrailingNewline() throws Exception {
    String canonical = "existing\n11111111-1111-4111-8111-111111111111\n"
        + "22222222-2222-4222-8222-222222222222\n33333333-3333-4333-8333-333333333333\n"
        + "44444444-4444-4444-8444-444444444444";
    assertThat(SdkConversionProofs.requestHash("existing", KEY, BINDING, ACCOUNT, PROFILE))
        .isEqualTo(sha256(canonical)).isNotEqualTo(sha256(canonical + "\n")).matches("[a-f0-9]{64}");
  }

  @Test
  void everyRequestHashFieldBindsTheOperationIntent() {
    String original = SdkConversionProofs.requestHash("existing", KEY, BINDING, ACCOUNT, PROFILE);
    assertThat(List.of(
        SdkConversionProofs.requestHash("new", KEY, BINDING, null, null),
        SdkConversionProofs.requestHash("existing", OTHER, BINDING, ACCOUNT, PROFILE),
        SdkConversionProofs.requestHash("existing", KEY, OTHER, ACCOUNT, PROFILE),
        SdkConversionProofs.requestHash("existing", KEY, BINDING, OTHER, PROFILE),
        SdkConversionProofs.requestHash("existing", KEY, BINDING, ACCOUNT, OTHER)))
        .doesNotContain(original).doesNotHaveDuplicates();
  }

  @Test
  void requestHashRejectsMissingKeyOrBindingInBothModes() {
    denied(() -> SdkConversionProofs.requestHash("new", null, BINDING, null, null));
    denied(() -> SdkConversionProofs.requestHash("new", KEY, null, null, null));
    denied(() -> SdkConversionProofs.requestHash("existing", null, BINDING, ACCOUNT, PROFILE));
    denied(() -> SdkConversionProofs.requestHash("existing", KEY, null, ACCOUNT, PROFILE));
  }

  @Test
  void newModeCannotSmuggleAnExistingTargetAndExistingModeCannotOmitOne() {
    denied(() -> SdkConversionProofs.requestHash("new", KEY, BINDING, ACCOUNT, PROFILE));
    denied(() -> SdkConversionProofs.requestHash("new", KEY, BINDING, ACCOUNT, null));
    denied(() -> SdkConversionProofs.requestHash("new", KEY, BINDING, null, PROFILE));
    denied(() -> SdkConversionProofs.requestHash("existing", KEY, BINDING, null, null));
    denied(() -> SdkConversionProofs.requestHash("existing", KEY, BINDING, ACCOUNT, null));
    denied(() -> SdkConversionProofs.requestHash("existing", KEY, BINDING, null, PROFILE));
  }

  @Test
  void statusPayloadUsesExactOperationAndDecimalUnixSeconds() {
    long timestamp = NOW.getEpochSecond();
    assertThat(SdkConversionProofs.statusPayload(OPERATION, timestamp, CLOCK))
        .isEqualTo("voice-sdk-conversion-status-v1\n55555555-5555-4555-8555-555555555555\n" + timestamp);
    assertThat(SdkConversionProofs.statusPayload(OTHER, timestamp, CLOCK))
        .isNotEqualTo(SdkConversionProofs.statusPayload(OPERATION, timestamp, CLOCK));
    assertThat(SdkConversionProofs.statusPayload(OPERATION, timestamp - 1, CLOCK))
        .isNotEqualTo(SdkConversionProofs.statusPayload(OPERATION, timestamp, CLOCK));
  }

  @ParameterizedTest
  @ValueSource(longs = {-60, -1, 0, 1, 30})
  void statusAcceptsInclusiveFreshnessBoundaries(long offset) {
    long timestamp = NOW.getEpochSecond() + offset;
    assertThat(SdkConversionProofs.statusPayload(OPERATION, timestamp, CLOCK))
        .isEqualTo("voice-sdk-conversion-status-v1\n" + OPERATION + "\n" + timestamp);
  }

  @Test
  void statusRejectsOutOfWindowAndOverflowingUnixTimestampsUniformly() {
    for (long timestamp : new long[] {NOW.getEpochSecond() - 61, NOW.getEpochSecond() + 31,
        Long.MIN_VALUE, Long.MAX_VALUE, Instant.MIN.getEpochSecond() - 1,
        Instant.MAX.getEpochSecond() + 1}) {
      denied(() -> SdkConversionProofs.statusPayload(OPERATION, timestamp, CLOCK));
    }
    denied(() -> SdkConversionProofs.statusPayload(null, NOW.getEpochSecond(), CLOCK));
    denied(() -> SdkConversionProofs.statusPayload(OPERATION, NOW.getEpochSecond(), null));
  }

  @Test
  void statusDoesNotTruncateCurrentTimeToExtendTheFreshnessWindow() {
    Clock fractional = Clock.fixed(NOW.plusMillis(500), ZoneOffset.UTC);
    denied(() -> SdkConversionProofs.statusPayload(OPERATION, NOW.getEpochSecond() - 60, fractional));
    assertThat(SdkConversionProofs.statusPayload(OPERATION, NOW.getEpochSecond() - 59, fractional)).isNotBlank();
    assertThat(SdkConversionProofs.statusPayload(OPERATION, NOW.getEpochSecond() + 30, fractional)).isNotBlank();
    denied(() -> SdkConversionProofs.statusPayload(OPERATION, NOW.getEpochSecond() + 31, fractional));
  }
}
