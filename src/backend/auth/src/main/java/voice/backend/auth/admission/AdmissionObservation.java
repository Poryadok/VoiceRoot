package voice.backend.auth.admission;

import java.util.UUID;
import java.util.function.Function;

/** Read-only owner aggregation; never holds an Auth mutation lock across User I/O. */
public final class AdmissionObservation {
  public enum Phase { START, READ, SUBMIT, APPROVE, FIRST_PUBLISH, SET_ENTRY_POLICY }
  public enum Role { APPLICANT, REVIEWER, SETTINGS_ACTOR }
  public record Request(Phase phase, Role role, UUID account, UUID profile, UUID space,
      String binding, String requestHash, Long interactiveEpoch) {}
  public record Account(UUID id, String type, boolean active, long revision, long durableEpoch, long retainedEpoch) {}
  public record Profile(UUID account, UUID id, String binding, long revision, boolean deleted, boolean frozen) {}
  public record Result(String requestHash, UUID account, UUID profile, String type,
      long accountRevision, long floor, long profileRevision) {}
  @FunctionalInterface public interface Profiles { Profile inspect(Request request); }
  private final Function<UUID, Account> accounts;
  private final Profiles profiles;
  public AdmissionObservation(Function<UUID, Account> accounts, Profiles profiles) {
    this.accounts = java.util.Objects.requireNonNull(accounts);
    this.profiles = java.util.Objects.requireNonNull(profiles);
  }
  public Result observe(Request request) {
    validate(request);
    try {
      requireAccount(accounts.apply(request.account()), request);
      Profile profile = profiles.inspect(request);
      if (profile == null || !request.account().equals(profile.account()) || !request.profile().equals(profile.id())
          || !request.requestHash().equals(profile.binding()) || profile.revision() <= 0) throw unavailable();
      if (profile.deleted() || profile.frozen()) throw denied();
      // The remote request is outside any Auth transaction/monitor. Re-read our
      // current facts afterwards; an earlier allow is never a reusable permit.
      Account current = accounts.apply(request.account());
      long floor = requireAccount(current, request);
      return new Result(request.requestHash(), request.account(), request.profile(), current.type(),
          current.revision(), floor, profile.revision());
    } catch (io.grpc.StatusRuntimeException known) {
      throw known;
    } catch (RuntimeException failure) {
      throw unavailable();
    }
  }
  private static void validate(Request r) {
    if (r == null || r.phase() == null || r.role() == null || nil(r.account()) || nil(r.profile()) || nil(r.space())
        || r.binding() == null || !r.binding().matches("[0-9a-f]{64}")
        || r.requestHash() == null || !r.requestHash().matches("sha256:[0-9a-f]{64}")) throw invalid();
    boolean settings = r.phase() == Phase.SET_ENTRY_POLICY;
    if (settings != (r.role() == Role.SETTINGS_ACTOR)) throw invalid();
    boolean interactive = settings || r.phase() == Phase.READ || r.phase() == Phase.START || r.phase() == Phase.SUBMIT
        || (r.phase() == Phase.APPROVE && r.role() == Role.REVIEWER);
    if (((r.phase() == Phase.START || r.phase() == Phase.SUBMIT) && r.role() != Role.APPLICANT)
        || interactive != (r.interactiveEpoch() != null)
        || (r.interactiveEpoch() != null && r.interactiveEpoch() <= 0)) throw invalid();
  }
  private static boolean nil(UUID value) { return value == null || value.equals(new UUID(0,0)); }
  private static long requireAccount(Account a, Request r) {
    if (a == null || !r.account().equals(a.id())) throw denied();
    if (!a.active()) throw denied();
    if ((!"regular".equals(a.type()) && !"guest".equals(a.type())) || a.revision() <= 0
        || a.durableEpoch() <= 0 || a.retainedEpoch() <= 0) throw unavailable();
    long floor = Math.max(a.durableEpoch(), a.retainedEpoch());
    if (r.interactiveEpoch() != null && r.interactiveEpoch() < floor) throw denied();
    return floor;
  }
  private static io.grpc.StatusRuntimeException denied() {
    return io.grpc.Status.PERMISSION_DENIED.withDescription("admission subject ineligible").asRuntimeException();
  }
  private static io.grpc.StatusRuntimeException unavailable() {
    return io.grpc.Status.UNAVAILABLE.withDescription("admission authority unavailable").asRuntimeException();
  }
  private static io.grpc.StatusRuntimeException invalid() {
    return io.grpc.Status.INVALID_ARGUMENT.withDescription("invalid admission observation").asRuntimeException();
  }
}
