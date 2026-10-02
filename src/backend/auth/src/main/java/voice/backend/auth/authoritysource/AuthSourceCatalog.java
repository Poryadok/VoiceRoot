// Generated from the owning source contract and migration bodies; DO NOT EDIT.
package voice.backend.auth.authoritysource;
import java.util.*;
final class AuthSourceCatalog {
 record Trigger(String table,String name,String function,int type) {}
 static final List<String> TABLES=List.of("accounts","sdk_identities","sdk_devices","sdk_device_keys","sdk_sessions","sdk_linked_sessions","sdk_authorizations","sdk_conversion_operations","sdk_game_message_grants","sdk_game_binding_handoff_claims","auth_authority_revision");
 static final List<Trigger> TRIGGERS=List.of(
new Trigger("auth_authority_revision","auth_authority_floor_change","auth_authority_floor_guard",31),
new Trigger("auth_authority_revision","auth_authority_floor_truncate","auth_authority_floor_guard",34),
new Trigger("accounts","auth_source_accounts","auth_authority_changed",28),
new Trigger("accounts","auth_source_accounts_truncate","auth_authority_changed",32),
new Trigger("sdk_identities","auth_source_sdk_identities","auth_authority_changed",28),
new Trigger("sdk_identities","auth_source_sdk_identities_truncate","auth_authority_changed",32),
new Trigger("sdk_devices","auth_source_sdk_devices","auth_authority_changed",28),
new Trigger("sdk_devices","auth_source_sdk_devices_truncate","auth_authority_changed",32),
new Trigger("sdk_device_keys","auth_source_sdk_device_keys","auth_authority_changed",28),
new Trigger("sdk_device_keys","auth_source_sdk_device_keys_truncate","auth_authority_changed",32),
new Trigger("sdk_sessions","auth_source_sdk_sessions","auth_authority_changed",28),
new Trigger("sdk_sessions","auth_source_sdk_sessions_truncate","auth_authority_changed",32),
new Trigger("sdk_linked_sessions","auth_source_sdk_linked_sessions","auth_authority_changed",28),
new Trigger("sdk_linked_sessions","auth_source_sdk_linked_sessions_truncate","auth_authority_changed",32),
new Trigger("sdk_authorizations","auth_source_sdk_authorizations","auth_authority_changed",28),
new Trigger("sdk_authorizations","auth_source_sdk_authorizations_truncate","auth_authority_changed",32),
new Trigger("sdk_conversion_operations","auth_source_sdk_conversion_operations","auth_authority_changed",28),
new Trigger("sdk_conversion_operations","auth_source_sdk_conversion_operations_truncate","auth_authority_changed",32),
new Trigger("sdk_game_message_grants","auth_source_sdk_game_message_grants","auth_authority_changed",28),
new Trigger("sdk_game_message_grants","auth_source_sdk_game_message_grants_truncate","auth_authority_changed",32),
new Trigger("sdk_game_binding_handoff_claims","auth_source_sdk_game_binding_handoff_claims","auth_authority_changed",28),
new Trigger("sdk_game_binding_handoff_claims","auth_source_sdk_game_binding_handoff_claims_truncate","auth_authority_changed",32));
 static final Map<String,String> FUNCTIONS=Map.of(
"auth_authority_floor_guard","2bd6a523329a2cc42661a79074cf4ed1586a80539f003325d2f3239b7a9e24c5",
"auth_authority_changed","516323852ae51eae4928b1eeaec443186931f2116a6921fb3985b2e86640153f");
 static final Map<String,String> QUERIES;
 static {var queries=new LinkedHashMap<String,String>();
queries.put("accounts","""
SELECT r.id::text AS account_id,a.id IS NOT NULL AS exists,COALESCE(a.type,'') AS type,COALESCE(a.status,'') AS status,a.deleted_at IS NOT NULL AS deleted,COALESCE(a.regular_email_verification_pending,false) AS email_pending,COALESCE(a.session_epoch,0) AS session_epoch,COALESCE(a.security_revision,0) AS security_revision FROM unnest(?::uuid[]) r(id) LEFT JOIN public.accounts a ON a.id=r.id ORDER BY r.id LIMIT 10001
""");
queries.put("sdk_identities","""
SELECT r.id::text AS account_id,i.account_id IS NOT NULL AS exists,COALESCE(i.actor_id::text,'') AS actor_id,COALESCE(i.application_id::text,'') AS application_id,COALESCE(i.environment_id::text,'') AS environment_id,COALESCE(i.ownership_generation,0) AS ownership_generation,COALESCE(i.status,'') AS status FROM unnest(?::uuid[]) r(id) LEFT JOIN public.sdk_identities i ON i.account_id=r.id ORDER BY r.id LIMIT 10001
""");
queries.put("sdk_devices","""
SELECT account_id::text,device_id::text,authority_revision,revoked_at IS NOT NULL AS revoked FROM public.sdk_devices WHERE account_id=ANY(?::uuid[]) ORDER BY account_id,device_id LIMIT 10001
""");
queries.put("sdk_keys","""
SELECT d.account_id::text,k.device_id::text,k.key_id::text,k.application_id::text,k.environment_id::text,k.generation,k.status,k.revoked_at IS NOT NULL AS revoked,floor(extract(epoch FROM k.not_before)*1000)::bigint AS not_before_unix_millis,floor(extract(epoch FROM k.not_after)*1000)::bigint AS not_after_unix_millis FROM public.sdk_device_keys k JOIN public.sdk_devices d ON d.device_id=k.device_id WHERE d.account_id=ANY(?::uuid[]) ORDER BY d.account_id,k.device_id,k.key_id LIMIT 10001
""");
queries.put("sdk_sessions","""
SELECT DISTINCT account_id::text,device_id::text,ownership_generation,floor(extract(epoch FROM expires_at)*1000)::bigint AS until_unix_millis FROM public.sdk_sessions WHERE account_id=ANY(?::uuid[]) ORDER BY account_id,device_id,ownership_generation,until_unix_millis LIMIT 10001
""");
queries.put("sdk_bindings","""
SELECT a.request_id::text,a.source_account_id::text,a.device_id::text,a.source_generation,a.application_id::text,a.environment_id::text,a.game_binding_intent AS binding_intent,COALESCE(a.game_binding_id::text,'') AS binding_id,a.game_binding_status AS status,a.game_binding_authority_revision AS authority_revision,COALESCE(a.target_account_id::text,'') AS target_account_id,COALESCE(a.target_profile_id::text,'') AS target_profile_id,COALESCE(a.target_epoch,0) AS target_epoch,COALESCE(a.profile_revision,0) AS profile_revision,string_to_array(a.scopes,',') AS scopes,a.policy_revision,COALESCE(s.consent_revision,a.game_binding_consent_revision,0) AS consent_revision,COALESCE(floor(extract(epoch FROM s.expires_at)*1000)::bigint,0) AS linked_until_unix_millis FROM public.sdk_authorizations a LEFT JOIN public.sdk_linked_sessions s ON s.request_id=a.request_id WHERE a.source_account_id=ANY(?::uuid[]) ORDER BY a.source_account_id,a.request_id LIMIT 10001
""");
queries.put("sdk_conversions","""
SELECT operation_id::text,source_account_id::text,device_id::text,application_id::text,environment_id::text,source_generation,binding_id::text,mode,state,revision,COALESCE(target_account_id::text,'') AS target_account_id,COALESCE(target_profile_id::text,'') AS target_profile_id,COALESCE(target_epoch,0) AS target_epoch,COALESCE(profile_revision,0) AS profile_revision FROM public.sdk_conversion_operations WHERE source_account_id=ANY(?::uuid[]) ORDER BY source_account_id,operation_id LIMIT 10001
""");
queries.put("sdk_message_grants","""
SELECT a.source_account_id::text,g.authorization_request_id::text,g.grant_id::text,g.application_id::text,g.environment_id::text,g.target_account_id::text,g.target_profile_id::text,g.target_epoch,g.binding_id::text,g.consent_revision,string_to_array(g.scopes,',') AS scopes,g.policy_revision,g.profile_revision,g.status,g.authority_revision FROM public.sdk_game_message_grants g JOIN public.sdk_authorizations a ON a.request_id=g.authorization_request_id WHERE a.source_account_id=ANY(?::uuid[]) ORDER BY a.source_account_id,g.grant_id LIMIT 10001
""");
QUERIES=Collections.unmodifiableMap(queries);}
}
