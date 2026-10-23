package voice.backend.auth.roleidentityauthority;

import com.google.protobuf.ByteString;
import java.security.MessageDigest;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.HashSet;
import java.util.ArrayList;
import java.util.List;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Objects;
import java.util.UUID;
import javax.sql.DataSource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.transaction.support.TransactionTemplate;
import voice.identityerasure.v1.AccountLifecycleFact;
import voice.identityerasure.v1.AccountLifecycleState;
import voice.identityerasure.v1.Eligibility;
import voice.identityerasure.v1.ErasureDisposition;
import voice.identityerasure.v1.ObservationWindow;
import voice.identityerasure.v1.OwnerCut;
import voice.identityerasure.v1.RecoveryBinding;

/** Auth-owned parent facts; protected transport and User preflight call this outside caller locks. */
public final class AuthIdentityOwnerStore {
  public record Observed(UUID accountId, AccountLifecycleFact lifecycle, ObservationWindow window) {}
  public record ObservedHandles(List<AccountLifecycleFact> accounts, ObservationWindow window, OwnerCut ownerCut) {
    public ObservedHandles {
      accounts = List.copyOf(accounts);
      Objects.requireNonNull(window);
      Objects.requireNonNull(ownerCut);
    }
  }
  private record Head(UUID stream, UUID generation, long through, byte[] digest,
      long keysetRevision, int activeVersion, boolean enforced) {}
  private record Lifetime(UUID handle, long revision, String state, boolean eligible,
      UUID cycle, Instant eraseAt, boolean selectorComplete) {}
  private final JdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final RoleIdentitySelectors selectors;
  private final AuthIdentityCatalog catalog;

  public AuthIdentityOwnerStore(DataSource source, RoleIdentitySelectors selectors) {
    this(source, selectors, null);
  }

  public AuthIdentityOwnerStore(DataSource source, RoleIdentitySelectors selectors,
      AuthIdentityDatabasePolicy policy) {
    this.catalog = new AuthIdentityCatalog(policy);
    this.jdbc = new JdbcTemplate(Objects.requireNonNull(source));
    this.jdbc.setQueryTimeout(2);
    this.selectors = Objects.requireNonNull(selectors);
    this.transactions = new TransactionTemplate(new DataSourceTransactionManager(source));
    this.transactions.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    this.transactions.setTimeout(3);
  }

  public Observed resolveAccount(UUID accountId) {
    if (TransactionSynchronizationManager.isActualTransactionActive()) {
      throw new IllegalStateException("identity owner observation requires independent transaction");
    }
    if (accountId == null || accountId.equals(new UUID(0, 0))) {
      throw new IllegalArgumentException("invalid identity owner selector");
    }
    // Local immutable keys only; prepare every retained lookup before SQL locks.
    Map<Integer, byte[]> lookups = selectors.allAccounts(accountId);
    Map<Integer, byte[]> fingerprints = selectors.keyFingerprints();
    return transactions.execute(status -> resolve(accountId, lookups, fingerprints));
  }

  /** Online exact-handle observation; a local head is never independent recovery authority. */
  public ObservedHandles observeAccountHandles(List<UUID> handles, OwnerCut requiredCut, RecoveryBinding recovery) {
    if (TransactionSynchronizationManager.isActualTransactionActive()) {
      throw new IllegalStateException("identity owner observation requires independent transaction");
    }
    if (handles == null) throw new IllegalArgumentException("invalid owner handles");
    var seen = new HashSet<UUID>();
    for (UUID handle : handles) {
      if (handle == null || handle.equals(new UUID(0, 0)) || !seen.add(handle)) {
        throw new IllegalArgumentException("invalid owner handles");
      }
    }
    List<UUID> requested = List.copyOf(handles);
    validateRequiredCut(requiredCut);
    if (recovery != null) {
      validateRecoveryBinding(recovery);
      // There is no independently durable CURRENT adapter in this source unit.
      // Echoing a supplied binding or comparing this DB's own head is insufficient.
      throw new IllegalStateException("identity owner recovery authority unavailable");
    }
    Map<Integer, byte[]> fingerprints = selectors.keyFingerprints();
    return transactions.execute(status -> observeHandles(requested, requiredCut, fingerprints));
  }

  private ObservedHandles observeHandles(List<UUID> requested, OwnerCut requiredCut,
      Map<Integer, byte[]> fingerprints) {
    catalog.validate(jdbc);
    validateKeys(head(false), fingerprints);
    List<UUID> ordered = requested.stream().sorted().toList();
    var retained = new HashSet<UUID>();
    // Lock ALL accounts before any lifetime/helper/head. Otherwise one handle's
    // materialization could hold the journal head while waiting on another account.
    for (UUID handle : ordered) {
      var rows = jdbc.query("SELECT role_identity_handle FROM accounts WHERE role_identity_handle=? FOR UPDATE",
          (rs, row) -> rs.getObject(1, UUID.class), handle);
      if (rows.size() > 1 || (!rows.isEmpty() && !handle.equals(rows.getFirst()))) throw unavailable();
      if (!rows.isEmpty()) retained.add(handle);
    }
    var facts = new LinkedHashMap<UUID, Lifetime>();
    for (UUID handle : ordered) facts.put(handle, lifetime(handle, true));
    for (UUID handle : ordered) {
      if (facts.get(handle) == null) {
        if (retained.contains(handle)) throw unavailable();
        continue;
      }
      jdbc.queryForObject("SELECT role_identity_materialize_handle(?)", Long.class, handle);
      Lifetime fact = lifetime(handle, false);
      if (fact == null) throw unavailable();
      facts.put(handle, fact);
    }
    Head cut = head(true);
    validateKeys(cut, fingerprints);
    Instant observedAt = jdbc.queryForObject("SELECT clock_timestamp()", Timestamp.class).toInstant();
    boolean complete = !requested.isEmpty() && requested.stream()
        .allMatch(handle -> complete(facts.get(handle), retained.contains(handle)));
    var ownerCut = OwnerCut.newBuilder().setOwner("auth").setStreamId(cut.stream().toString())
        .setHistoryGeneration(cut.generation().toString()).setThroughRevision(cut.through())
        .setSelectorKeysetRevision(cut.keysetRevision()).setDigest(ByteString.copyFrom(cut.digest()))
        .setFactCount(jdbc.queryForObject("SELECT count(*) FROM role_identity_lifetimes", Long.class))
        .setComplete(complete);
    for (int version : selectors.retainedVersions()) ownerCut.addSelectorKeyVersions(Integer.toString(version));
    OwnerCut actual = ownerCut.build();
    if (requiredCut != null && !requiredCut.toBuilder().setComplete(false).build()
        .equals(actual.toBuilder().setComplete(false).build())) {
      throw new IllegalStateException("identity owner required cut unavailable");
    }
    var accounts = new ArrayList<AccountLifecycleFact>(requested.size());
    for (UUID handle : requested) {
      accounts.add(handleFact(handle, facts.get(handle), retained.contains(handle), actual, observedAt));
    }
    return new ObservedHandles(accounts, ObservationWindow.newBuilder().setObservedAt(timestamp(observedAt))
        .setValidUntil(timestamp(observedAt.plusSeconds(10))).build(), actual);
  }

  private Lifetime lifetime(UUID handle, boolean lock) {
    var rows = jdbc.query("SELECT * FROM role_identity_lifetimes WHERE owner_handle=?" + (lock ? " FOR UPDATE" : ""),
        (rs, row) -> new Lifetime(rs.getObject("owner_handle", UUID.class), rs.getLong("lifecycle_revision"),
            rs.getString("state"), rs.getBoolean("eligible"), rs.getObject("deletion_cycle", UUID.class),
            rs.getTimestamp("erase_at") == null ? null : rs.getTimestamp("erase_at").toInstant(),
            rs.getBoolean("selector_complete")), handle);
    if (rows.size() > 1) throw unavailable();
    return rows.isEmpty() ? null : rows.getFirst();
  }

  private static boolean complete(Lifetime fact, boolean retained) {
    return fact != null && fact.selectorComplete() && (retained || fact.state().equals("TERMINAL"));
  }

  private static AccountLifecycleFact handleFact(UUID handle, Lifetime fact, boolean retained,
      OwnerCut cut, Instant observedAt) {
    var result = AccountLifecycleFact.newBuilder().setOwnerHandle(handle.toString()).setOwnerCut(cut)
        .setEligibility(Eligibility.ELIGIBILITY_UNAVAILABLE)
        .setDisposition(ErasureDisposition.ERASURE_DISPOSITION_INCOMPLETE);
    if (fact == null) return result.build();
    if (!handle.equals(fact.handle()) || fact.revision() <= 0) throw unavailable();
    AccountLifecycleState state = switch (fact.state()) {
      case "ACTIVE" -> AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_ACTIVE;
      case "RECOVERABLE" -> AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_RECOVERABLE;
      case "TERMINAL" -> AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_TERMINAL;
      default -> throw unavailable();
    };
    if (state == AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_ACTIVE) {
      if (fact.cycle() != null || fact.eraseAt() != null) throw unavailable();
    } else {
      if (fact.cycle() == null || fact.eraseAt() == null || fact.eligible()) throw unavailable();
      if (state == AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_TERMINAL
          ? fact.eraseAt().isAfter(observedAt) : !fact.eraseAt().isAfter(observedAt)) throw unavailable();
      result.setDeletionCycle(fact.cycle().toString()).setEraseAt(timestamp(fact.eraseAt()));
    }
    result.setLifecycleRevision(fact.revision()).setState(state);
    if (complete(fact, retained)) result
        .setEligibility(fact.eligible() ? Eligibility.ELIGIBILITY_ELIGIBLE : Eligibility.ELIGIBILITY_INELIGIBLE)
        .setDisposition(state == AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_TERMINAL
            ? ErasureDisposition.ERASURE_DISPOSITION_IRREVERSIBLE
            : ErasureDisposition.ERASURE_DISPOSITION_NOT_IRREVERSIBLE);
    return result.build();
  }

  static void validateRequiredCut(OwnerCut cut) {
    if (cut == null) return;
    try {
      if (!cut.getUnknownFields().asMap().isEmpty() || !cut.getOwner().equals("auth")
          || !cut.getComplete() || cut.getThroughRevision() <= 0 || cut.getSelectorKeysetRevision() <= 0
          || cut.getFactCount() < 0 || cut.getDigest().size() != 32 || cut.getSelectorKeyVersionsCount() == 0) {
        throw new IllegalArgumentException();
      }
      canonicalId(cut.getStreamId()); canonicalId(cut.getHistoryGeneration());
      int prior = 0;
      for (String version : cut.getSelectorKeyVersionsList()) {
        int parsed = Integer.parseInt(version);
        if (parsed <= prior || !Integer.toString(parsed).equals(version)) throw new IllegalArgumentException();
        prior = parsed;
      }
    } catch (RuntimeException invalid) { throw new IllegalArgumentException("invalid required owner cut"); }
  }

  static void validateRecoveryBinding(RecoveryBinding binding) {
    if (!binding.getUnknownFields().asMap().isEmpty() || binding.getRestoreTarget().isBlank()
        || binding.getRestoreGeneration().isBlank() || binding.getChallenge().isEmpty()
        || binding.getRoleStreamId().isBlank() || binding.getRoleThroughRevision() <= 0
        || binding.getRoleCheckpointDigest().size() != 32 || binding.getRoleKeysetRevision().isBlank()
        || binding.getInventoryGeneration() <= 0 || binding.getInventoryDigest().size() != 32) {
      throw new IllegalArgumentException("invalid owner recovery binding");
    }
  }

  private static UUID canonicalId(String text) {
    UUID id = UUID.fromString(text);
    if (!id.toString().equals(text) || id.equals(new UUID(0, 0))) throw new IllegalArgumentException();
    return id;
  }

  private Observed resolve(UUID accountId, Map<Integer, byte[]> lookups,
      Map<Integer, byte[]> fingerprints) {
    catalog.validate(jdbc);
    validateKeys(head(false), fingerprints);
    var rawHandles = jdbc.query("SELECT role_identity_handle FROM accounts WHERE id=? FOR UPDATE",
        (rs, row) -> rs.getObject(1, UUID.class), accountId);
    var matches = new HashSet<UUID>();
    for (var lookup : lookups.entrySet()) {
      matches.addAll(jdbc.query("SELECT owner_handle FROM role_identity_account_selectors "
          + "WHERE key_version=? AND selector=?", (rs, row) -> rs.getObject(1, UUID.class),
          lookup.getKey(), lookup.getValue()));
    }
    if (matches.size() > 1) throw unavailable();
    UUID handle = rawHandles.isEmpty() ? null : rawHandles.getFirst();
    if (handle != null) {
      if (!matches.isEmpty() && !matches.contains(handle)) throw unavailable();
      // A retained exact account can capture its own current selector. No history
      // is inferred for an absent row; trigger/keyset conflicts roll this back.
      jdbc.update("UPDATE accounts SET role_identity_selector_version=?,"
          + "role_identity_account_selector=?,role_identity_keyset_revision=? WHERE id=?",
          selectors.activeVersion(), lookups.get(selectors.activeVersion()), selectors.keysetRevision(), accountId);
    } else if (!matches.isEmpty()) {
      handle = matches.iterator().next();
    }

    Lifetime fact = null;
    if (handle != null) {
      jdbc.queryForObject("SELECT role_identity_materialize_handle(?)", Long.class, handle);
      fact = jdbc.queryForObject("SELECT * FROM role_identity_lifetimes WHERE owner_handle=? FOR UPDATE",
          (rs, row) -> new Lifetime(rs.getObject("owner_handle", UUID.class), rs.getLong("lifecycle_revision"),
              rs.getString("state"), rs.getBoolean("eligible"), rs.getObject("deletion_cycle", UUID.class),
              rs.getTimestamp("erase_at") == null ? null : rs.getTimestamp("erase_at").toInstant(),
              rs.getBoolean("selector_complete")), handle);
    }
    // Account -> lifetime -> head lock order matches domain writers. Taking this
    // lock before an account would deadlock a writer awaiting its journal cut.
    Head cut = head(true);
    validateKeys(cut, fingerprints);
    Instant observedAt = jdbc.queryForObject("SELECT clock_timestamp()", Timestamp.class).toInstant();
    boolean complete = fact != null && fact.selectorComplete()
        && (!rawHandles.isEmpty() || fact.state().equals("TERMINAL"));
    if (fact != null && fact.state().equals("TERMINAL")
        && (fact.cycle() == null || fact.eraseAt() == null || fact.eraseAt().isAfter(observedAt))) {
      throw unavailable();
    }
    var ownerCut = OwnerCut.newBuilder().setOwner("auth").setStreamId(cut.stream().toString())
        .setHistoryGeneration(cut.generation().toString()).setThroughRevision(cut.through())
        .setSelectorKeysetRevision(cut.keysetRevision()).setDigest(ByteString.copyFrom(cut.digest()))
        .setFactCount(jdbc.queryForObject("SELECT count(*) FROM role_identity_lifetimes", Long.class))
        .setComplete(complete);
    for (int version : selectors.retainedVersions()) ownerCut.addSelectorKeyVersions(Integer.toString(version));
    var lifecycle = AccountLifecycleFact.newBuilder().setOwnerCut(ownerCut)
        .setEligibility(Eligibility.ELIGIBILITY_UNAVAILABLE)
        .setDisposition(ErasureDisposition.ERASURE_DISPOSITION_INCOMPLETE);
    if (fact != null) {
      lifecycle.setOwnerHandle(fact.handle().toString()).setLifecycleRevision(fact.revision())
          .setState(switch (fact.state()) {
            case "ACTIVE" -> AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_ACTIVE;
            case "RECOVERABLE" -> AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_RECOVERABLE;
            case "TERMINAL" -> AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_TERMINAL;
            default -> throw unavailable();
          });
      if (fact.cycle() != null) lifecycle.setDeletionCycle(fact.cycle().toString());
      if (fact.eraseAt() != null) lifecycle.setEraseAt(timestamp(fact.eraseAt()));
      if (complete) lifecycle.setEligibility(fact.eligible() ? Eligibility.ELIGIBILITY_ELIGIBLE : Eligibility.ELIGIBILITY_INELIGIBLE)
          .setDisposition(fact.state().equals("TERMINAL") ? ErasureDisposition.ERASURE_DISPOSITION_IRREVERSIBLE
              : ErasureDisposition.ERASURE_DISPOSITION_NOT_IRREVERSIBLE);
    }
    return new Observed(accountId, lifecycle.build(), ObservationWindow.newBuilder()
        .setObservedAt(timestamp(observedAt)).setValidUntil(timestamp(observedAt.plusSeconds(10))).build());
  }

  private Head head(boolean lock) {
    return jdbc.queryForObject("SELECT * FROM role_identity_owner_state" + (lock ? " FOR SHARE" : ""),
        (rs, row) -> new Head(rs.getObject("stream_id", UUID.class), rs.getObject("history_generation", UUID.class),
            rs.getLong("through_revision"), rs.getBytes("head_digest"), rs.getLong("keyset_revision"),
            rs.getInt("active_version"), rs.getBoolean("enforcement_enabled")));
  }

  private void validateKeys(Head head, Map<Integer, byte[]> expected) {
    if (head == null || !head.enforced() || head.keysetRevision() != selectors.keysetRevision()
        || head.activeVersion() != selectors.activeVersion() || head.through() <= 0
        || head.digest() == null || head.digest().length != 32) throw unavailable();
    var actual = new LinkedHashMap<Integer, byte[]>();
    jdbc.query("SELECT version,fingerprint FROM role_identity_key_versions ORDER BY version",
        rs -> { actual.put(rs.getInt(1), rs.getBytes(2)); });
    if (!actual.keySet().equals(expected.keySet())) throw unavailable();
    for (var key : actual.entrySet()) {
      if (!MessageDigest.isEqual(key.getValue(), expected.get(key.getKey()))) throw unavailable();
    }
  }

  private static com.google.protobuf.Timestamp timestamp(Instant value) {
    return com.google.protobuf.Timestamp.newBuilder().setSeconds(value.getEpochSecond()).setNanos(value.getNano()).build();
  }

  private static IllegalStateException unavailable() {
    return new IllegalStateException("identity owner keyset unavailable");
  }
}
