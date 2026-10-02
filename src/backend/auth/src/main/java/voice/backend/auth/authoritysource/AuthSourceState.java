package voice.backend.auth.authoritysource;

import java.util.*;

/** Raw clocks/leases remain stable at a revision. Consumers evaluate time anew. */
final class AuthSourceState {
  @SuppressWarnings("unchecked")
  static List<Map<String,Object>> group(Map<String,Object> state,String name) {
    return (List<Map<String,Object>>)state.get(name);
  }
  static String text(Map<String,Object> row,String name) {return (String)row.get(name);}
  static long number(Map<String,Object> row,String name) {return ((Number)row.get(name)).longValue();}
  static boolean flag(Map<String,Object> row,String name) {return (Boolean)row.get(name);}
  private static void require(boolean accepted) {if(!accepted)throw new IllegalStateException("invalid Auth source state");}
  private static void uuid(Map<String,Object> row,String... fields) {for(String field:fields)require(AuthAuthorityReader.id(text(row,field)));}
  private static void positive(Map<String,Object> row,String... fields) {for(String field:fields)require(number(row,field)>0);}
  private static void oneOf(Map<String,Object> row,String field,String... values) {require(Set.of(values).contains(text(row,field)));}
  private static void ordered(List<Map<String,Object>> rows,String... fields) {
    String previous="";
    for(var row:rows) {String key=String.join("/",Arrays.stream(fields).map(field->text(row,field)).toList());require(key.compareTo(previous)>0);previous=key;}
  }
  @SuppressWarnings("unchecked")
  private static void scopes(Map<String,Object> row) {
    var scopes=(List<String>)row.get("scopes");require(scopes!=null && !scopes.isEmpty() && scopes.size()<=64);
    String previous="";
    for(String scope:scopes) {require(scope.matches("[a-z][a-z0-9_.-]{0,63}") && scope.compareTo(previous)>0);previous=scope;}
  }
  private static void target(Map<String,Object> row) {
    boolean missing=text(row,"target_account_id").isEmpty();
    require(missing==text(row,"target_profile_id").isEmpty());
    if(missing) {require(number(row,"target_epoch")==0 && number(row,"profile_revision")==0);}
    else {uuid(row,"target_account_id","target_profile_id");positive(row,"target_epoch","profile_revision");}
  }

  static void validate(Map<String,Object> state,List<String> requested) {
    var accounts=group(state,"accounts");var identities=group(state,"sdk_identities");
    require(accounts.size()==requested.size() && identities.size()==requested.size());
    Map<String,Map<String,Object>> sdk=new HashMap<>(),devices=new HashMap<>();
    Set<String> actors=new HashSet<>();
    for(int index=0;index<requested.size();index++) {
      var account=accounts.get(index);var identity=identities.get(index);
      require(requested.get(index).equals(text(account,"account_id")) && requested.get(index).equals(text(identity,"account_id")));
      require(!(flag(account,"exists") && flag(identity,"exists")));
      if(flag(account,"exists")) {oneOf(account,"type","regular","guest");oneOf(account,"status","active","suspended","deleted");positive(account,"session_epoch","security_revision");}
      else {require(text(account,"type").isEmpty() && text(account,"status").isEmpty() && !flag(account,"deleted") && !flag(account,"email_pending") && number(account,"session_epoch")==0 && number(account,"security_revision")==0);}
      if(flag(identity,"exists")) {uuid(identity,"actor_id","application_id","environment_id");oneOf(identity,"status","active","suspended","deleted","retired");positive(identity,"ownership_generation");require(actors.add(text(identity,"actor_id")));sdk.put(requested.get(index),identity);}
      else {require(text(identity,"actor_id").isEmpty() && text(identity,"application_id").isEmpty() && text(identity,"environment_id").isEmpty() && text(identity,"status").isEmpty() && number(identity,"ownership_generation")==0);}
    }
    var deviceRows=group(state,"sdk_devices");ordered(deviceRows,"account_id","device_id");
    for(var device:deviceRows) {uuid(device,"account_id","device_id");positive(device,"authority_revision");require(sdk.containsKey(text(device,"account_id")) && devices.put(text(device,"device_id"),device)==null);}
    var keys=group(state,"sdk_keys");ordered(keys,"account_id","device_id","key_id");
    for(var key:keys) {parent(key,sdk,devices);uuid(key,"key_id");positive(key,"generation","not_before_unix_millis","not_after_unix_millis");require(number(key,"not_after_unix_millis")>number(key,"not_before_unix_millis"));oneOf(key,"status","active","overlap","revoked","expired");}
    for(var session:group(state,"sdk_sessions")) {uuid(session,"account_id","device_id");positive(session,"ownership_generation","until_unix_millis");require(devices.containsKey(text(session,"device_id")) && text(devices.get(text(session,"device_id")),"account_id").equals(text(session,"account_id")));}
    var bindings=group(state,"sdk_bindings");ordered(bindings,"source_account_id","request_id");
    Map<String,String> requests=new HashMap<>();
    for(var binding:bindings) {
      sourceParent(binding,sdk,devices);uuid(binding,"request_id");positive(binding,"source_generation","policy_revision");target(binding);scopes(binding);
      oneOf(binding,"status","unbound","active","revoking","revoked");
      require(number(binding,"authority_revision")>=0 && number(binding,"consent_revision")>=0 && number(binding,"linked_until_unix_millis")>=0);
      if(!text(binding,"status").equals("unbound")) {uuid(binding,"binding_id");positive(binding,"authority_revision");require(flag(binding,"binding_intent"));}
      else require(text(binding,"binding_id").isEmpty() || AuthAuthorityReader.id(text(binding,"binding_id")));
      require(requests.put(text(binding,"request_id"),text(binding,"source_account_id"))==null);
    }
    var conversions=group(state,"sdk_conversions");ordered(conversions,"source_account_id","operation_id");
    for(var conversion:conversions) {sourceParent(conversion,sdk,devices);uuid(conversion,"operation_id","binding_id");positive(conversion,"source_generation","revision");oneOf(conversion,"mode","new","existing");oneOf(conversion,"state","prepared","previewed","confirmed","frozen","owners_ready","activated","retired","cancelled");target(conversion);}
    var grants=group(state,"sdk_message_grants");ordered(grants,"source_account_id","grant_id");
    for(var grant:grants) {
      uuid(grant,"source_account_id","authorization_request_id","grant_id","application_id","environment_id","target_account_id","target_profile_id","binding_id");positive(grant,"target_epoch","consent_revision","policy_revision","profile_revision","authority_revision");scopes(grant);oneOf(grant,"status","active","revoking","revoked");
      var identity=sdk.get(text(grant,"source_account_id"));
      require(identity!=null && text(identity,"application_id").equals(text(grant,"application_id")) && text(identity,"environment_id").equals(text(grant,"environment_id")) && text(grant,"source_account_id").equals(requests.get(text(grant,"authorization_request_id"))));
    }
  }
  private static void parent(Map<String,Object> row,Map<String,Map<String,Object>> sdk,Map<String,Map<String,Object>> devices) {
    uuid(row,"account_id","device_id","application_id","environment_id");
    var device=devices.get(text(row,"device_id"));var identity=sdk.get(text(row,"account_id"));
    require(device!=null && identity!=null && text(device,"account_id").equals(text(row,"account_id")) && text(identity,"application_id").equals(text(row,"application_id")) && text(identity,"environment_id").equals(text(row,"environment_id")));
  }
  private static void sourceParent(Map<String,Object> row,Map<String,Map<String,Object>> sdk,Map<String,Map<String,Object>> devices) {
    uuid(row,"source_account_id","device_id","application_id","environment_id");
    var device=devices.get(text(row,"device_id"));var identity=sdk.get(text(row,"source_account_id"));
    require(device!=null && identity!=null && text(device,"account_id").equals(text(row,"source_account_id")) && text(identity,"application_id").equals(text(row,"application_id")) && text(identity,"environment_id").equals(text(row,"environment_id")));
  }
  static long validUntil(Map<String,Object> state,long now) {
    long earliest=0;
    for(var key:group(state,"sdk_keys")) for(String field:List.of("not_before_unix_millis","not_after_unix_millis")) earliest=earlier(earliest,number(key,field),now);
    for(var session:group(state,"sdk_sessions")) earliest=earlier(earliest,number(session,"until_unix_millis"),now);
    for(var binding:group(state,"sdk_bindings")) earliest=earlier(earliest,number(binding,"linked_until_unix_millis"),now);
    return earliest;
  }
  private static long earlier(long earliest,long candidate,long now) {return candidate>now && (earliest==0 || candidate<earliest)?candidate:earliest;}
}
