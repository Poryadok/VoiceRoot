package voice.backend.auth.moderationauthority;

import app.voice.automod.v1.*;
import com.google.protobuf.InvalidProtocolBufferException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;
import java.util.UUID;

/** The canonical Go/Java wire identity; this class grants no sender authority. */
final class AuthLifecycleCodec {
  static final int MAX_BYTES=1024;
  private AuthLifecycleCodec() {}
  static byte[] canonical(AccountLifecycleFact fact) {
    if(fact==null || !fact.getUnknownFields().asMap().isEmpty() || fact.getProtocolVersion()!=1
        || !identifier(fact.getAuthorityGeneration()) || !identifier(fact.getEventId())
        || !identifier(fact.getSubjectId())) throw invalid();
    boolean valid=switch(fact.getState()) {
      case SUBJECT_STATE_LIVE -> fact.getLifecycleRevision()>0 && !fact.hasDeletionCycleId() && !fact.hasPurgeAt();
      case SUBJECT_STATE_DELETED -> fact.getLifecycleRevision()>0 && identifier(fact.getDeletionCycleId())
          && fact.hasPurgeAt() && fact.getPurgeAt().getUnknownFields().asMap().isEmpty()
          && fact.getPurgeAt().getSeconds()>=-62135596800L && fact.getPurgeAt().getSeconds()<=253402300799L
          && fact.getPurgeAt().getNanos()>=0 && fact.getPurgeAt().getNanos()<1000000000
          && fact.getPurgeAt().getNanos()%1000==0;
      case SUBJECT_STATE_PURGED -> fact.getLifecycleRevision()==0 && !fact.hasDeletionCycleId() && !fact.hasPurgeAt();
      default -> false;
    };
    if(!valid || fact.getSerializedSize()>MAX_BYTES) throw invalid();
    // All fields are scalar/singular and there are no maps or unknown fields.
    return fact.toByteArray();
  }
  static AccountLifecycleFact decode(byte[] body) {
    if(body==null || body.length==0 || body.length>MAX_BYTES) throw invalid();
    try {
      var fact=AccountLifecycleFact.parseFrom(body);
      if(!Arrays.equals(body,canonical(fact))) throw invalid();
      return fact;
    }catch(InvalidProtocolBufferException error){throw invalid();}
  }
  static byte[] hash(AccountLifecycleFact fact) {
    try {
      var hash=MessageDigest.getInstance("SHA-256");
      hash.update("voice.automod.lifecycle.v1\u0000".getBytes(StandardCharsets.UTF_8));
      return hash.digest(canonical(fact));
    }catch(NoSuchAlgorithmException impossible){throw new IllegalStateException(impossible);}
  }
  /** Structural binding only; a higher or terminal receipt still needs owner CAS. */
  static void validateReceipt(AccountLifecycleFact fact,ApplyLifecycleFactResponse response) {
    canonical(fact);
    if(response==null || !response.getUnknownFields().asMap().isEmpty() || response.getSerializedSize()>MAX_BYTES
        || response.getProtocolVersion()!=1 || !response.getAuthorityGeneration().equals(fact.getAuthorityGeneration())
        || !response.getSubjectId().equals(fact.getSubjectId()) || !response.getRequestedEventId().equals(fact.getEventId())) throw invalid();
    if(response.getTerminal()) {
      if(response.getLifecycleRevision()!=0 || !response.getCurrentFactSha256().isEmpty()) throw invalid();
      return;
    }
    if(fact.getState()==SubjectState.SUBJECT_STATE_PURGED || response.getLifecycleRevision()<=0
        || response.getLifecycleRevision()<fact.getLifecycleRevision() || response.getCurrentFactSha256().size()!=32) throw invalid();
    if(response.getLifecycleRevision()==fact.getLifecycleRevision()
        && !MessageDigest.isEqual(hash(fact),response.getCurrentFactSha256().toByteArray())) throw invalid();
  }
  private static boolean identifier(String value) {
    try {var id=UUID.fromString(value);return id.toString().equals(value) && !id.equals(new UUID(0,0));}
    catch(IllegalArgumentException|NullPointerException invalid){return false;}
  }
  private static IllegalArgumentException invalid(){return new IllegalArgumentException("invalid private lifecycle fact or receipt");}
}
