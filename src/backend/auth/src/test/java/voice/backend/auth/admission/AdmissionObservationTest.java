package voice.backend.auth.admission;

import static org.assertj.core.api.Assertions.*;
import io.grpc.Status;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import voice.backend.auth.admission.AdmissionObservation.*;

class AdmissionObservationTest {
  final UUID account = UUID.randomUUID(), profile = UUID.randomUUID(), space = UUID.randomUUID();
  final String hash = "sha256:" + "ab".repeat(32);
  Request request(Phase phase, Role role, Long epoch) {
    return new Request(phase, role, account, profile, space, "cd".repeat(32), hash, epoch);
  }
  AdmissionObservation.Account good(String type, long durable, long retained) {
    return new AdmissionObservation.Account(account, type, true, 9, durable, retained);
  }
  Profile eligible(Request r) { return new Profile(account, profile, r.requestHash(), 13, false, false); }
  @Test void regularAndGuestUseMaximumFloorAndExactBinding() {
    for (String type : new String[]{"regular", "guest"}) {
      var service = new AdmissionObservation(id -> good(type, 7, 8), this::eligible);
      var result = service.observe(request(Phase.SUBMIT, Role.APPLICANT, 8L));
      assertThat(result).isEqualTo(new Result(hash, account, profile, type, 9, 8, 13));
    }
  }
  @Test void retainedAndDurableRevocationBothDenyBeforeUser() {
    for (long[] floor : new long[][]{{8,7},{7,8}}) {
      var service = new AdmissionObservation(id -> good("regular", floor[0], floor[1]), r -> { throw new AssertionError("User called"); });
      assertThatThrownBy(() -> service.observe(request(Phase.START, Role.APPLICANT, 7L)))
          .satisfies(ex -> assertThat(Status.fromThrowable(ex).getCode()).isEqualTo(Status.Code.PERMISSION_DENIED));
    }
  }
  @Test void delayedApplicantAndContinuationDoNotPinOldSession() {
    var service = new AdmissionObservation(id -> good("regular", 88, 89), this::eligible);
    assertThat(service.observe(request(Phase.APPROVE, Role.APPLICANT, null)).floor()).isEqualTo(89);
    for (Role role : new Role[]{Role.APPLICANT, Role.REVIEWER})
      assertThat(service.observe(request(Phase.FIRST_PUBLISH, role, null)).floor()).isEqualTo(89);
  }
  @Test void phaseAndEpochMatrixRejectsMalformedCombinations() {
    var service = new AdmissionObservation(id -> good("regular", 1, 1), this::eligible);
    for (Request bad : new Request[]{request(Phase.START, Role.REVIEWER, 1L), request(Phase.SUBMIT, Role.APPLICANT, null),
        request(Phase.READ, Role.REVIEWER, null), request(Phase.APPROVE, Role.REVIEWER, null),
        request(Phase.APPROVE, Role.APPLICANT, 1L), request(Phase.FIRST_PUBLISH, Role.APPLICANT, 1L)}) {
      assertThatThrownBy(() -> service.observe(bad)).satisfies(ex ->
          assertThat(Status.fromThrowable(ex).getCode()).isEqualTo(Status.Code.INVALID_ARGUMENT));
    }
  }
  @Test void postUserAccountRevocationDeniesAndNoCachedAllowSurvives() {
    var current = new AtomicReference<>(good("regular", 1, 1));
    var service = new AdmissionObservation(id -> current.get(), r -> {
      current.set(new AdmissionObservation.Account(account, "regular", false, 10, 2, 2));
      return eligible(r);
    });
    assertThatThrownBy(() -> service.observe(request(Phase.SUBMIT, Role.APPLICANT, 1L)))
        .satisfies(ex -> assertThat(Status.fromThrowable(ex).getCode()).isEqualTo(Status.Code.PERMISSION_DENIED));
  }
  @Test void malformedOwnerFactsAndForeignResponseFailClosed() {
    for (AdmissionObservation.Account bad : new AdmissionObservation.Account[]{good("unknown",1,1), good("regular",0,1), good("regular",1,0),
        new AdmissionObservation.Account(account,"regular",true,0,1,1)}) {
      var service = new AdmissionObservation(id -> bad, this::eligible);
      assertThatThrownBy(() -> service.observe(request(Phase.START, Role.APPLICANT, 1L)))
          .satisfies(ex -> assertThat(Status.fromThrowable(ex).getCode()).isEqualTo(Status.Code.UNAVAILABLE));
    }
    for (Profile bad : new Profile[]{new Profile(UUID.randomUUID(),profile,hash,1,false,false),
        new Profile(account,profile,"sha256:"+"00".repeat(32),1,false,false), new Profile(account,profile,hash,0,false,false)}) {
      var service = new AdmissionObservation(id -> good("guest",1,1), r -> bad);
      assertThatThrownBy(() -> service.observe(request(Phase.START, Role.APPLICANT, 1L)))
          .satisfies(ex -> assertThat(Status.fromThrowable(ex).getCode()).isEqualTo(Status.Code.UNAVAILABLE));
    }
  }
  @Test void deletedFrozenAndUnavailableProfileNeverBecomeEligible() {
    for (boolean frozen : new boolean[]{true,false}) {
      var service = new AdmissionObservation(id -> good("guest",1,1), r -> new Profile(account,profile,hash,1,!frozen,frozen));
      assertThatThrownBy(() -> service.observe(request(Phase.START, Role.APPLICANT, 1L)))
          .satisfies(ex -> assertThat(Status.fromThrowable(ex).getCode()).isEqualTo(Status.Code.PERMISSION_DENIED));
    }
  }
}
