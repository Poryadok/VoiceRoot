package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import app.voice.auth.v1.*;
import com.google.protobuf.ByteString;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;

class AuthAdmissionSettingsGrpcTest {
  @Test void settingsPairAcceptsBothAccountTypesAndEchoesExactFullRequestHash() throws Exception {
    for (String type : new String[]{"regular", "guest"}) {
      var existing = new AuthAdmissionGrpcTest();
      try (var fixture = existing.new Fixture(type)) {
        var request = existing.request().toBuilder()
            .setPhase(SpaceAdmissionPhase.SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY)
            .setSubjectRole(SpaceAdmissionSubjectRole.SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR).build();
        var result = fixture.call(request, request, "space");
        assertEquals(SpaceAdmissionDecision.SPACE_ADMISSION_DECISION_ELIGIBLE, result.getDecision());
        assertEquals(type, result.getAccountType());
        assertEquals(2, existing.reads.get(), "same account authority is read before and after User");
        assertEquals(ByteString.copyFrom(HexFormat.of().parseHex(
            AuthPrincipalServerInterceptor.requestHash(request).substring(7))), result.getRequestBindingSha256());
      }
    }
  }

  @Test void exhaustivePhaseRoleEpochMatrixPreservesOldPairsAndRejectsBeforeOwnerReads() throws Exception {
    var existing = new AuthAdmissionGrpcTest();
    try (var fixture = existing.new Fixture()) {
      for (int phase = 1; phase <= 6; phase++) for (int role = 1; role <= 3; role++)
          for (Long epoch : new Long[]{null, 0L, 8L, Long.MAX_VALUE, -1L, Long.MIN_VALUE}) {
        var builder = existing.request().toBuilder().setPhaseValue(phase).setSubjectRoleValue(role).clearInteractiveSessionEpoch();
        if (epoch != null) builder.setInteractiveSessionEpoch(epoch);
        var request = builder.build();
        boolean positiveEpoch = epoch != null && epoch > 0;
        boolean allowed = switch (phase) {
          case 1, 3 -> role == 1 && positiveEpoch;
          case 2 -> (role == 1 || role == 2) && positiveEpoch;
          case 4 -> (role == 1 && epoch == null) || (role == 2 && positiveEpoch);
          case 5 -> (role == 1 || role == 2) && epoch == null;
          case 6 -> role == 3 && positiveEpoch;
          default -> false;
        };
        int before = existing.reads.get();
        String label = "phase=" + phase + ", role=" + role + ", epoch=" + epoch;
        if (allowed) {
          assertEquals(SpaceAdmissionDecision.SPACE_ADMISSION_DECISION_ELIGIBLE,
              fixture.call(request, request, "space").getDecision(), label);
          assertEquals(before + 2, existing.reads.get(), label);
        } else {
          assertEquals(Status.Code.INVALID_ARGUMENT, assertThrows(StatusRuntimeException.class,
              () -> fixture.call(request, request, "space"), label).getStatus().getCode(), label);
          assertEquals(before, existing.reads.get(), "invalid purpose must not query owners: " + label);
        }
      }
    }
  }

  @Test void oldPurposeRequestGoldenHashesRemainByteIdentical() {
    int[][] pairs = {{1,1,8},{2,1,8},{2,2,8},{3,1,8},{4,1,0},{4,2,8},{5,1,0},{5,2,0}};
    String[] expected = {
      "3f1fac4ecb3c5ebfa219867a93a943ceabba4e502f0b8fff4fa59b707b36a64f",
      "7d853a1a57256065d1482a831913ef34990b8d8fa15cbb6ed5416f2cf95c3a37",
      "45b9bfccb93198e4c577aa77acc0bb2ed2bd16a0822a2d3d67936c4a7c3b578e",
      "3bbbf24fc989eac81dca01c448958d7f65417bd1ec6c1bbcead232ffb1327e9d",
      "fd90537988172e36d5a0b21a015d85b3b6f8d17f87e7797cce29199de880a189",
      "e39b7cd6ac7aae6fb20bae1e41fefb224af8ebf0eab4f3fece7a1c515078cd63",
      "5d7ec505eb00a6746f871af52c205376158802a39e7a6950ab24afb30c36be05",
      "20a93a2fbd1fcefa1418367c50f6904f897a572d9252b058d60429b684e7d4a0"
    };
    for (int i = 0; i < pairs.length; i++) {
      var request = ObserveSpaceAdmissionSubjectRequest.newBuilder().setProtocolVersion(1)
          .setPhaseValue(pairs[i][0]).setSubjectRoleValue(pairs[i][1])
          .setAccountId("00000000-0000-0000-0000-000000000001")
          .setProfileId("00000000-0000-0000-0000-000000000002")
          .setSpaceId("00000000-0000-0000-0000-000000000003")
          .setAdmissionBindingSha256(ByteString.copyFrom(new byte[32]));
      if (pairs[i][2] != 0) request.setInteractiveSessionEpoch(pairs[i][2]);
      assertEquals("sha256:" + expected[i], AuthPrincipalServerInterceptor.requestHash(request.build()));
    }
  }
}
