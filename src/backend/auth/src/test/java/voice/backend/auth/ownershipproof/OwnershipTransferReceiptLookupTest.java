package voice.backend.auth.ownershipproof;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;

import java.time.*;
import java.util.*;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class OwnershipTransferReceiptLookupTest {
  final OwnershipTransferProofStore store = mock(OwnershipTransferProofStore.class);
  final BCryptPasswordHasher passwords = mock(BCryptPasswordHasher.class);
  final TotpService totp = mock(TotpService.class);
  final BackupCodeService backup = mock(BackupCodeService.class);
  final SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
  final ProofBinding binding = new ProofBinding(UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), 7);
  final String digest = "0123456789abcdef".repeat(4);
  final Instant consumedAt = Instant.parse("2026-09-10T10:00:00.123456Z");
  final UUID receiptId = UUID.randomUUID();
  final StoredProof consumed = new StoredProof(binding, digest, 1, List.of("password", "totp"),
      consumedAt.plusSeconds(300), receiptId, consumedAt);
  OwnershipTransferProofService service;

  @BeforeEach void setUp() {
    service = new OwnershipTransferProofService(store, passwords, totp, backup, floors,
        Clock.fixed(consumedAt.plusSeconds(86400), ZoneOffset.UTC));
    when(store.findConsumed(any())).thenReturn(Optional.of(consumed));
    when(floors.requireFloor(any())).thenThrow(new IllegalStateException("floor unavailable"));
  }

  @Test void historicalReceiptReturnsExactCommittedFieldsWithoutAccountFloorFactorsOrWrites() {
    var expected = new OwnershipTransferProofService.Receipt(receiptId, binding, consumedAt, List.of("password", "totp"));
    assertThat(service.lookup(binding, digest)).isEqualTo(expected);
    assertThat(service.lookup(binding, digest)).isEqualTo(expected);
    assertThat(expected.toString()).doesNotContain(digest);
    verify(store, times(2)).findConsumed(binding.operationId());
    verifyNoMoreInteractions(store);
    verifyNoInteractions(passwords, totp, backup, floors);
  }

  @Test void eachOriginalBindingAndWellFormedDigestMustMatchEvenWhenOperationExists() {
    var mismatches = List.of(
        new ProofBinding(UUID.randomUUID(), binding.profileId(), binding.spaceId(), binding.newOwnerProfileId(), binding.operationId(), 7),
        new ProofBinding(binding.accountId(), UUID.randomUUID(), binding.spaceId(), binding.newOwnerProfileId(), binding.operationId(), 7),
        new ProofBinding(binding.accountId(), binding.profileId(), UUID.randomUUID(), binding.newOwnerProfileId(), binding.operationId(), 7),
        new ProofBinding(binding.accountId(), binding.profileId(), binding.spaceId(), UUID.randomUUID(), binding.operationId(), 7),
        new ProofBinding(binding.accountId(), binding.profileId(), binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7),
        new ProofBinding(binding.accountId(), binding.profileId(), binding.spaceId(), binding.newOwnerProfileId(), binding.operationId(), 8));
    for (var requested : mismatches) {
      assertThatThrownBy(() -> service.lookup(requested, digest)).isInstanceOf(ProofDeniedException.class)
          .hasMessage("ownership proof denied");
    }
    assertThatThrownBy(() -> service.lookup(binding, "f".repeat(64))).isInstanceOf(ProofDeniedException.class)
        .hasMessage("ownership proof denied");
    verify(store, times(6)).findConsumed(binding.operationId());
    verify(store).findConsumed(mismatches.get(4).operationId());
    verifyNoMoreInteractions(store);
    verifyNoInteractions(passwords, totp, backup, floors);
  }

  @Test void missingAndUnconsumedIncludingExpiredProofAreUniformlyDeniedWithoutMutation() {
    when(store.findConsumed(binding.operationId())).thenReturn(Optional.empty());
    assertThatThrownBy(() -> service.lookup(binding, digest)).isInstanceOf(ProofDeniedException.class)
        .hasMessage("ownership proof denied");
    for (Instant expiry : List.of(consumedAt.minusSeconds(1), consumedAt.plusSeconds(172800))) {
      when(store.findConsumed(binding.operationId())).thenReturn(Optional.of(
          new StoredProof(binding, digest, 1, List.of("password"), expiry, receiptId, null)));
      assertThatThrownBy(() -> service.lookup(binding, digest)).isInstanceOf(ProofDeniedException.class)
          .hasMessage("ownership proof denied");
    }
    verify(store, times(3)).findConsumed(binding.operationId());
    verifyNoMoreInteractions(store);
    verifyNoInteractions(passwords, totp, backup, floors);
  }

  @Test void digestShapeIsExactLowercaseHexWithoutTrimmingOrPrefix() {
    clearInvocations(store);
    for (String bad : Arrays.asList(null, "", "a".repeat(63), "a".repeat(65), digest.toUpperCase(Locale.ROOT),
        "g".repeat(64), " " + digest, digest + " ", "sha256:" + digest)) {
      assertThatThrownBy(() -> service.lookup(binding, bad)).isInstanceOf(IllegalArgumentException.class);
    }
    verifyNoInteractions(store, passwords, totp, backup, floors);
  }
}
