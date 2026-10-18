BEGIN;
LOCK TABLE voice_party_resource_retirements,voice_party_resource_components,voice_party_resource_component_links IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM voice_party_resource_retirements)
 OR EXISTS(SELECT 1 FROM voice_party_resource_components)
 OR EXISTS(SELECT 1 FROM voice_party_resource_component_links) THEN
  RAISE EXCEPTION 'retained resource retirement evidence prevents rollback';
 END IF;
END $$;
DROP TABLE voice_party_resource_component_links,voice_party_resource_retirements,voice_party_resource_components;
COMMIT;
