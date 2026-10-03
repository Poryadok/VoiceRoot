LOCK TABLE search_space_chat_manifests IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM search_space_chat_manifests) THEN
  RAISE EXCEPTION 'cannot remove retained Chat source/Space root evidence' USING ERRCODE='55000';
 END IF;
END $$;
ALTER TABLE search_space_chat_manifests
 DROP COLUMN root_manifest_id,
 DROP COLUMN root_manifest_sha256,
 DROP COLUMN root_manifest_item_count,
 DROP CONSTRAINT search_space_chat_manifests_item_count_check,
 ADD CONSTRAINT search_space_chat_manifests_item_count_check CHECK(item_count>0);
