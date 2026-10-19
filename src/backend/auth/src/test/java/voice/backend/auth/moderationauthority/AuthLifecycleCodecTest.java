package voice.backend.auth.moderationauthority;

import static org.assertj.core.api.Assertions.*;
import app.voice.automod.v1.*;
import com.google.protobuf.ByteString;
import com.google.protobuf.Timestamp;
import com.google.protobuf.UnknownFieldSet;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;

class AuthLifecycleCodecTest {
  static AccountLifecycleFact live() {
    return AccountLifecycleFact.newBuilder().setProtocolVersion(1)
        .setAuthorityGeneration("00000000-0000-4000-8000-000000000001")
        .setEventId("00000000-0000-4000-8000-000000000002")
        .setSubjectId("80000000-0000-4000-8000-000000000003")
        .setState(SubjectState.SUBJECT_STATE_LIVE).setLifecycleRevision(7).build();
  }
  @Test void matchesIndependentGoGoldenAndRefusesAlternateCarriers() {
    var fact=live();byte[] body=AuthLifecycleCodec.canonical(fact);
    assertThat(body).hasSize(120);
    assertThat(HexFormat.of().formatHex(AuthLifecycleCodec.hash(fact))).isEqualTo("53043311ff246eba73d39ef71db56fba6a1d299e7fae115fb9ccfcec8190655a");
    assertThat(AuthLifecycleCodec.decode(body)).isEqualTo(fact);
    byte[] duplicate=java.util.Arrays.copyOf(body,body.length+2);duplicate[body.length]=8;duplicate[body.length+1]=1;
    assertThatThrownBy(()->AuthLifecycleCodec.decode(duplicate)).isInstanceOf(IllegalArgumentException.class);
    var unknown=UnknownFieldSet.newBuilder().addField(100,UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build();
    assertThatThrownBy(()->AuthLifecycleCodec.canonical(fact.toBuilder().setUnknownFields(unknown).build())).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(()->AuthLifecycleCodec.canonical(fact.toBuilder().setSubjectId("00000000-0000-0000-0000-000000000000").build())).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(()->AuthLifecycleCodec.canonical(fact.toBuilder().setLifecycleRevision(-1L).build())).isInstanceOf(IllegalArgumentException.class);
  }
  @Test void dueDeletedIsNotTerminalAndTerminalInventsNoErasedFields() {
    var deleted=live().toBuilder().setState(SubjectState.SUBJECT_STATE_DELETED)
        .setDeletionCycleId("00000000-0000-4000-8000-000000000004")
        .setPurgeAt(Timestamp.newBuilder().setSeconds(1).setNanos(1000)).build();
    assertThat(AuthLifecycleCodec.decode(AuthLifecycleCodec.canonical(deleted))).isEqualTo(deleted);
    assertThatThrownBy(()->AuthLifecycleCodec.canonical(deleted.toBuilder().setPurgeAt(Timestamp.newBuilder().setSeconds(1).setNanos(1)).build())).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(()->AuthLifecycleCodec.canonical(deleted.toBuilder().setState(SubjectState.SUBJECT_STATE_PURGED).build())).isInstanceOf(IllegalArgumentException.class);
    var terminal=deleted.toBuilder().setState(SubjectState.SUBJECT_STATE_PURGED).setLifecycleRevision(0).clearDeletionCycleId().clearPurgeAt().build();
    assertThat(AuthLifecycleCodec.decode(AuthLifecycleCodec.canonical(terminal))).isEqualTo(terminal);
  }
  @Test void receiptStructureNeverSuppliesOwnerCurrentnessOrTerminalAuthority() {
    var fact=live();var response=ApplyLifecycleFactResponse.newBuilder().setProtocolVersion(1).setAuthorityGeneration(fact.getAuthorityGeneration())
        .setSubjectId(fact.getSubjectId()).setRequestedEventId(fact.getEventId()).setLifecycleRevision(7)
        .setCurrentFactSha256(ByteString.copyFrom(AuthLifecycleCodec.hash(fact))).build();
    AuthLifecycleCodec.validateReceipt(fact,response);
    assertThatThrownBy(()->AuthLifecycleCodec.validateReceipt(fact,response.toBuilder().setRequestedEventId(fact.getSubjectId()).build())).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(()->AuthLifecycleCodec.validateReceipt(fact,response.toBuilder().setCurrentFactSha256(ByteString.copyFrom(new byte[32])).build())).isInstanceOf(IllegalArgumentException.class);
    // These are structurally valid only. The delivery store must compare them
    // with CURRENT local owner state or its own permanent terminal ledger.
    AuthLifecycleCodec.validateReceipt(fact,response.toBuilder().setLifecycleRevision(8).setCurrentFactSha256(ByteString.copyFrom(new byte[32])).build());
    AuthLifecycleCodec.validateReceipt(fact,response.toBuilder().setTerminal(true).setLifecycleRevision(0).clearCurrentFactSha256().build());
  }
}
