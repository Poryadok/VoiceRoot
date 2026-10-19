package voice.backend.auth.moderationauthority;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.file.*;
import java.nio.file.attribute.PosixFilePermission;
import java.security.MessageDigest;
import java.util.*;

/** Trusted operator file outside the application DB rollback boundary, not an RPC input. */
record AuthModerationStartContext(String mode,String environment,UUID deploymentGeneration,
    UUID storeIncarnation,UUID authorityGeneration,UUID operationId,String sha256) {
  static AuthModerationStartContext read(Path file) {
    try {
      if(file==null || !file.isAbsolute())throw new IllegalArgumentException();
      for(Path part=file;part!=null;part=part.getParent())if(Files.isSymbolicLink(part))throw new IllegalArgumentException();
      if(!Files.isRegularFile(file,LinkOption.NOFOLLOW_LINKS) || Files.size(file)<1 || Files.size(file)>16384)throw new IllegalArgumentException();
      long currentUid=((Number)Files.getAttribute(Path.of("/proc/"+ProcessHandle.current().pid()),"unix:uid")).longValue();
      for(Path part=file;part!=null;part=part.getParent()) {
        long owner=((Number)Files.getAttribute(part,"unix:uid",LinkOption.NOFOLLOW_LINKS)).longValue();
        int mode=((Number)Files.getAttribute(part,"unix:mode",LinkOption.NOFOLLOW_LINKS)).intValue();
        if(owner!=0 && owner!=currentUid)throw new IllegalArgumentException();
        // An untrusted writable ancestor could replace an otherwise private
        // directory. Root-owned sticky ancestors such as /tmp retain ownership.
        if(!part.equals(file)&&(mode&0022)!=0 && !((mode&01000)!=0&&owner==0))throw new IllegalArgumentException();
      }
      var permissions=Files.getPosixFilePermissions(file,LinkOption.NOFOLLOW_LINKS);
      if(!permissions.equals(Set.of(PosixFilePermission.OWNER_READ)))throw new IllegalArgumentException();
      var parent=Files.getPosixFilePermissions(file.getParent(),LinkOption.NOFOLLOW_LINKS);
      if(parent.contains(PosixFilePermission.GROUP_WRITE) || parent.contains(PosixFilePermission.OTHERS_WRITE))throw new IllegalArgumentException();
      byte[] bytes;
      try(var input=Files.newInputStream(file,LinkOption.NOFOLLOW_LINKS)){bytes=input.readNBytes(16385);}
      if(bytes.length<1 || bytes.length>16384)throw new IllegalArgumentException();
      var json=new ObjectMapper().enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
          .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS).readTree(bytes);
      Set<String> required=Set.of("protocol_version","mode","environment","deployment_generation","store_incarnation","authority_generation");
      var fields=new HashSet<String>();json.fieldNames().forEachRemaining(fields::add);
      if(!json.isObject() || !json.path("protocol_version").isIntegralNumber() || !json.path("protocol_version").canConvertToInt() || json.path("protocol_version").intValue()!=1
          || !fields.containsAll(required) || fields.stream().anyMatch(k->!required.contains(k)&&!k.equals("operation_id")))throw new IllegalArgumentException();
      String mode=text(json,"mode"),environment=text(json,"environment");
      if(!Set.of("LIVE_CURRENT","DECLARED_RESTORE").contains(mode) || !environment.matches("[a-z][a-z0-9_-]{0,63}"))throw new IllegalArgumentException();
      UUID operation=json.has("operation_id")?id(text(json,"operation_id")):null;
      if(mode.equals("DECLARED_RESTORE")!=(operation!=null))throw new IllegalArgumentException();
      return new AuthModerationStartContext(mode,environment,id(text(json,"deployment_generation")),id(text(json,"store_incarnation")),
          id(text(json,"authority_generation")),operation,HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes)));
    }catch(Exception invalid){throw new IllegalArgumentException("invalid protected Auth moderation startup context",invalid);}
  }
  private static String text(com.fasterxml.jackson.databind.JsonNode json,String name){var value=json.get(name);if(value==null||!value.isTextual())throw new IllegalArgumentException();return value.textValue();}
  private static UUID id(String raw){UUID value=UUID.fromString(raw);if(!value.toString().equals(raw)||value.equals(new UUID(0,0)))throw new IllegalArgumentException();return value;}
}
